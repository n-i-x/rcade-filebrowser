package http

import (
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"syscall"
)

type SystemInfo struct {
	ReturnCode int    `json:"returnCode"`
	Output     string `json:"output"`
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

var supportFileHandler = withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	generateSupportFile()

	return http.StatusNoContent, nil
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
