package http

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"syscall"
	"time"

	"github.com/google/go-github/github"
)

type SystemInfo struct {
	ReturnCode int    `json:"returnCode"`
	Output     string `json:"output"`
}

type Cloudflared struct {
	ProxyURL    string    `json:"proxyURL"`
	Pid         int       `json:"pid"`
	TimeStarted time.Time `json:"timeStarted"`
	SessionCode string    `json:"sessionCode"`
	Started     bool      `json:"started"`
}

var cloudflaredCmd *exec.Cmd
var cloudflaredPid int
var cloudflaredStartTime time.Time
var cloudflaredProxyURL string
var cloudflaredConnected bool
var supportSessionCode string

func downloadCloudflared(filePath string) error {
	if _, err := os.Stat(filePath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	ctx := context.Background()

	client := github.NewClient(nil)
	owner := "cloudflare"
	repo := "cloudflared"

	release, _, err := client.Repositories.GetLatestRelease(ctx, owner, repo)
	if err != nil {
		log.Fatalf("Error getting latest release: %v", err)
	}

	releaseTag := release.GetTagName()

	desiredAssetName := "cloudflared-linux-arm"
	var foundAsset github.ReleaseAsset

	for _, asset := range release.Assets {
		if asset.GetName() == desiredAssetName {
			foundAsset = asset
			break
		}
	}

	downloadURL := foundAsset.GetBrowserDownloadURL()
	log.Printf("Downloading cloudflared %s\n", releaseTag)

	outFile, err := os.Create(filePath)
	if err != nil {
		log.Fatalf("Error creating file: %v", err)
	}
	defer outFile.Close()

	resp, err := http.Get(downloadURL)
	if err != nil {
		log.Fatalf("Error downloading the file: %v", err)
	}
	defer resp.Body.Close()

	_, err = io.Copy(outFile, resp.Body)
	if err != nil {
		log.Fatalf("Error writing to file: %v", err)
	}

	log.Printf("cloudflared downloaded to %s\n", filePath)
	log.Printf("Changing permissions of %s to 0755", filePath)

	err = os.Chmod(filePath, 0755)
	if err != nil {
		log.Fatalf("Error setting file permissions: %v", err)
	}

	return nil
}

func generateSupportFile() {
	log.Println("Generating support file")
	cmd := exec.Command("/rcade/scripts/rcade-commands.sh", "supportfiles")

	// Run the command and wait for it to complete
	err := cmd.Run()
	if err != nil {
		log.Fatalf("Command finished with error: %v", err)
	}

	log.Println("Support file generated")
}

func generateSupportSessionCode() {
	supportSessionCode = fmt.Sprintf("%04d-%04d-%04d", rand.Intn(10000), rand.Intn(10000), rand.Intn(10000))
}

var supportFileHandler = withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	generateSupportFile()

	return http.StatusNoContent, nil
})

var supportRemountHandler = withAdmin(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	log.Printf("Remounting /boot as read-write")
	cmd := exec.Command("mount", "/boot", "-o", "remount,rw")

	err := cmd.Run()
	if err != nil {
		log.Printf("Error remounting /boot: %v", err)
		return http.StatusInternalServerError, err
	}

	log.Printf("Remounted /boot as read-write")
	return http.StatusNoContent, nil
})

var supportSessionStatusHandler = withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	cloudflared := Cloudflared{ProxyURL: cloudflaredProxyURL, Pid: cloudflaredPid, TimeStarted: cloudflaredStartTime, Started: cloudflaredConnected, SessionCode: supportSessionCode}
	return renderJSON(w, r, cloudflared)
})

var supportStopSessionHandler = withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	if cloudflaredPid == 0 {
		return http.StatusNoContent, nil
	}

	if cloudflaredCmd != nil && cloudflaredCmd.Process != nil {
		log.Printf("Stopping cloudflared (pid: %d)", cloudflaredPid)
		err := cloudflaredCmd.Process.Signal(syscall.SIGTERM)
		if err != nil {
			log.Printf("Error stopping cloudflared: %v", err)
			return http.StatusInternalServerError, err
		}
		err = cloudflaredCmd.Wait()
		if err != nil {
			log.Printf("Error waiting for cloudflared to exit: %v", err)
			return http.StatusInternalServerError, err
		}
		log.Printf("cloudflared process stopped successfully.")
	}

	cloudflaredCmd = nil
	cloudflaredPid = 0
	cloudflaredStartTime = time.Time{}
	cloudflaredProxyURL = ""
	cloudflaredConnected = false
	supportSessionCode = ""

	return http.StatusNoContent, nil
})

var supportStartSessionHandler = withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	var cloudflared Cloudflared
	if cloudflaredPid != 0 {
		cloudflared = Cloudflared{ProxyURL: cloudflaredProxyURL, Pid: cloudflaredPid, TimeStarted: cloudflaredStartTime, Started: cloudflaredConnected, SessionCode: supportSessionCode}
		return renderJSON(w, r, cloudflared)
	}

	cloudflaredPath := "/tmp/cloudflared"
	err := downloadCloudflared(cloudflaredPath)
	if err != nil {
		log.Printf("Error downloading cloudflared: %v", err)
		return http.StatusInternalServerError, err
	}

	port := d.server.Port
	proxiedURL := "http://localhost:" + port

	cloudflaredCmd = exec.Command(cloudflaredPath, "tunnel", "--url", proxiedURL)

	urlRe := regexp.MustCompile(`INF\s+\|\s+(https:\/\/\S+)\s+\|$`)
	connStartRe := regexp.MustCompile(`INF Registered tunnel connection`)

	output, err := cloudflaredCmd.StderrPipe()
	if err != nil {
		log.Printf("Error getting stderr pipe: %v", err)
		return http.StatusInternalServerError, err
	}

	err = cloudflaredCmd.Start()
	if err != nil {
		log.Printf("Error starting cloudflared: %v", err)
		return http.StatusInternalServerError, err
	}
	cloudflaredPid = cloudflaredCmd.Process.Pid

	log.Printf("Starting cloudflared (pid: %d) with URL: %s", cloudflaredPid, proxiedURL)

	scanner := bufio.NewScanner(output)
	for scanner.Scan() {
		line := scanner.Text()
		if matches := urlRe.FindStringSubmatch(line); matches != nil {
			cloudflaredProxyURL = matches[1]
		}
		if matches := connStartRe.FindStringSubmatch(line); matches != nil {
			break
		}
	}

	cloudflaredStartTime = time.Now()
	cloudflaredConnected = true
	generateSupportSessionCode()

	log.Printf("Cloudflared proxy URL: %s", cloudflaredProxyURL)
	resp := Cloudflared{ProxyURL: cloudflaredProxyURL, Pid: cloudflaredPid, TimeStarted: cloudflaredStartTime, Started: cloudflaredConnected, SessionCode: supportSessionCode}
	return renderJSON(w, r, resp)
})

var supportSysinfoHandler = withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	cmd := exec.Command("/rcade/scripts/rcade-commands.sh", "sysinfo")

	// Get combined stdout and stderr
	output, err := cmd.CombinedOutput()
	sysinfo := SystemInfo{ReturnCode: 0, Output: string(output)}

	if err != nil {
		// Check if the error is an ExitError to extract the exit code
		if exitError, ok := err.(*exec.ExitError); ok {
			// This works on Unix-like systems
			if status, ok := exitError.Sys().(syscall.WaitStatus); ok {
				sysinfo.ReturnCode = status.ExitStatus()
				outputString := fmt.Sprintf("ERROR: %s\n\n%s", err, output)
				sysinfo.Output = outputString
			}
		} else {
			sysinfo.ReturnCode = 999
			sysinfo.Output = fmt.Sprintf("Command error: %s", err)
		}
	}

	return renderJSON(w, r, sysinfo)
})
