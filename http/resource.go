package http

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v3/disk"
	"github.com/spf13/afero"

	fbErrors "github.com/filebrowser/filebrowser/v2/errors"
	"github.com/filebrowser/filebrowser/v2/files"
	"github.com/filebrowser/filebrowser/v2/fileutils"
)

var logFilesVirtualPath = "/virtual/logs"
var dirSize = 3488

var logFiles = []string{
	"/rcade/share/.emulationstation/es_log.txt",
	"/rcade/share/.emulationstation/es_log.txt.bak",
	"/rcade/share/.emulationstation/upgrade.log",
	"/tmp/last_game_launch.log",
	"/tmp/rcade-usbmount.log",
	"/var/log/messages",
}

// getLogFile returns a FileInfo for a log file
func getLogFile(path string) (*files.FileInfo, error) {
	logFileName := filepath.Base(path)
	for _, logFile := range logFiles {
		if filepath.Base(logFile) == logFileName {
			logFileStat, err := os.Stat(logFile)
			if err != nil {
				if os.IsNotExist(err) {
					return &files.FileInfo{}, nil
				}

				return &files.FileInfo{}, err
			}
			content, err := os.ReadFile(logFile)
			if err != nil {
				return &files.FileInfo{}, err
			}
			return &files.FileInfo{
				Path:      filepath.Join("/virtual", path),
				Name:      logFileName,
				Size:      logFileStat.Size(),
				Extension: filepath.Ext(logFileName),
				ModTime:   logFileStat.ModTime(),
				Mode:      fs.FileMode(0444),
				IsDir:     false,
				IsSymlink: false,
				Type:      "text",
				Content:   string(content),
			}, nil
		}
	}
	return &files.FileInfo{}, nil
}

// getLogFiles returns a Listing of log files
func getLogFiles() (*files.FileInfo, error) {
	var items []*files.FileInfo
	for _, path := range logFiles {
		logFileName := filepath.Base(path)
		virtPath := filepath.Join(logFilesVirtualPath, logFileName)
		logFileStat, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}

			return &files.FileInfo{}, err
		}
		items = append(items, &files.FileInfo{
			Path:      virtPath,
			Name:      logFileName,
			Type:      "text",
			IsDir:     false,
			IsSymlink: false,
			Mode:      fs.FileMode(0444),
			Size:      logFileStat.Size(),
			ModTime:   logFileStat.ModTime(),
			Extension: filepath.Ext(logFileName),
		})
	}
	i := files.FileInfo{
		Listing: &files.Listing{
			Items:    items,
			NumDirs:  0,
			NumFiles: len(items),
			Sorting:  files.Sorting{By: "name", Asc: true},
		},
		Path:      logFilesVirtualPath,
		Name:      "logs",
		IsDir:     true,
		IsSymlink: false,
		Extension: "",
	}

	return &i, nil
}

func getVirtualRoot() (*files.FileInfo, error) {
	var items []*files.FileInfo

	currentTime := time.Now()
	modTime := currentTime.Add(-5 * time.Second)

	items = append(items, &files.FileInfo{
		Path:    "/virtual/logs",
		Name:    "logs",
		IsDir:   true,
		Mode:    fs.FileMode(0555) | fs.ModeDir,
		Size:    int64(dirSize),
		ModTime: modTime,
	})
	i := files.FileInfo{
		Listing: &files.Listing{
			Items:    items,
			NumDirs:  len(items),
			NumFiles: 0,
			Sorting:  files.Sorting{By: "name", Asc: true},
		},
		Path:    "/virtual",
		Name:    "virtual",
		Size:    int64(dirSize),
		Mode:    fs.FileMode(0555) | fs.ModeDir,
		ModTime: modTime,
		IsDir:   true,
	}

	return &i, nil
}

var resourceVirtualGetHandler = withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	if r.URL.Path == "/" {
		return virtualRootHandler(w, r, d)
	} else if r.URL.Path == "/logs" || r.URL.Path == "/logs/" {
		return logFilesHandler(w, r, d)
	} else if strings.HasPrefix(r.URL.Path, "/logs/") {
		return logFileHandler(w, r, d)
	}

	return http.StatusNotFound, nil
})

var virtualRootHandler = withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	file, err := getVirtualRoot()
	if err != nil {
		return errToStatus(err), err
	}
	return renderJSON(w, r, file)
})

var logFileHandler = withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	file, err := getLogFile(r.URL.Path)
	if err != nil {
		return errToStatus(err), err
	}
	return renderJSON(w, r, file)
})

var logFilesHandler = withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	file, err := getLogFiles()
	if err != nil {
		return errToStatus(err), err
	}
	return renderJSON(w, r, file)
})

var resourceGetHandler = withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	file, err := files.NewFileInfo(&files.FileOptions{
		Fs:         d.user.Fs,
		Path:       r.URL.Path,
		Modify:     d.user.Perm.Modify,
		Expand:     true,
		ReadHeader: d.server.TypeDetectionByHeader,
		Checker:    d,
		Content:    true,
	})
	if err != nil {
		return errToStatus(err), err
	}

	if file.IsDir {
		file.Listing.Sorting = d.user.Sorting
		file.Listing.ApplySort()
		return renderJSON(w, r, file)
	}

	if checksum := r.URL.Query().Get("checksum"); checksum != "" {
		err := file.Checksum(checksum)
		if errors.Is(err, fbErrors.ErrInvalidOption) {
			return http.StatusBadRequest, nil
		} else if err != nil {
			return http.StatusInternalServerError, err
		}

		// do not waste bandwidth if we just want the checksum
		file.Content = ""
	}

	return renderJSON(w, r, file)
})

func resourceDeleteHandler(fileCache FileCache) handleFunc {
	return withUser(func(_ http.ResponseWriter, r *http.Request, d *data) (int, error) {
		if r.URL.Path == "/" || !d.user.Perm.Delete {
			return http.StatusForbidden, nil
		}

		file, err := files.NewFileInfo(&files.FileOptions{
			Fs:         d.user.Fs,
			Path:       r.URL.Path,
			Modify:     d.user.Perm.Modify,
			Expand:     false,
			ReadHeader: d.server.TypeDetectionByHeader,
			Checker:    d,
		})
		if err != nil {
			return errToStatus(err), err
		}

		// delete thumbnails
		err = delThumbs(r.Context(), fileCache, file)
		if err != nil {
			return errToStatus(err), err
		}

		err = d.RunHook(func() error {
			return d.user.Fs.RemoveAll(r.URL.Path)
		}, "delete", r.URL.Path, "", d.user)

		if err != nil {
			return errToStatus(err), err
		}

		return http.StatusNoContent, nil
	})
}

func resourcePostHandler(fileCache FileCache) handleFunc {
	return withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
		if !d.user.Perm.Create || !d.Check(r.URL.Path) {
			return http.StatusForbidden, nil
		}

		// Directories creation on POST.
		if strings.HasSuffix(r.URL.Path, "/") {
			err := d.user.Fs.MkdirAll(r.URL.Path, files.PermDir)
			return errToStatus(err), err
		}

		file, err := files.NewFileInfo(&files.FileOptions{
			Fs:         d.user.Fs,
			Path:       r.URL.Path,
			Modify:     d.user.Perm.Modify,
			Expand:     false,
			ReadHeader: d.server.TypeDetectionByHeader,
			Checker:    d,
		})
		if err == nil {
			if r.URL.Query().Get("override") != "true" {
				return http.StatusConflict, nil
			}

			// Permission for overwriting the file
			if !d.user.Perm.Modify {
				return http.StatusForbidden, nil
			}

			err = delThumbs(r.Context(), fileCache, file)
			if err != nil {
				return errToStatus(err), err
			}
		}

		err = d.RunHook(func() error {
			info, writeErr := writeFile(d.user.Fs, r.URL.Path, r.Body)
			if writeErr != nil {
				return writeErr
			}

			etag := fmt.Sprintf(`"%x%x"`, info.ModTime().UnixNano(), info.Size())
			w.Header().Set("ETag", etag)
			return nil
		}, "upload", r.URL.Path, "", d.user)

		if err != nil {
			_ = d.user.Fs.RemoveAll(r.URL.Path)
		}

		return errToStatus(err), err
	})
}

var resourcePutHandler = withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	if !d.user.Perm.Modify || !d.Check(r.URL.Path) {
		return http.StatusForbidden, nil
	}

	// Only allow PUT for files.
	if strings.HasSuffix(r.URL.Path, "/") {
		return http.StatusMethodNotAllowed, nil
	}

	exists, err := afero.Exists(d.user.Fs, r.URL.Path)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	if !exists {
		return http.StatusNotFound, nil
	}

	err = d.RunHook(func() error {
		info, writeErr := writeFile(d.user.Fs, r.URL.Path, r.Body)
		if writeErr != nil {
			return writeErr
		}

		etag := fmt.Sprintf(`"%x%x"`, info.ModTime().UnixNano(), info.Size())
		w.Header().Set("ETag", etag)
		return nil
	}, "save", r.URL.Path, "", d.user)

	return errToStatus(err), err
})

func resourcePatchHandler(fileCache FileCache) handleFunc {
	return withUser(func(_ http.ResponseWriter, r *http.Request, d *data) (int, error) {
		src := r.URL.Path
		dst := r.URL.Query().Get("destination")
		action := r.URL.Query().Get("action")
		dst, err := url.QueryUnescape(dst)
		if !d.Check(src) || !d.Check(dst) {
			return http.StatusForbidden, nil
		}
		if err != nil {
			return errToStatus(err), err
		}
		if dst == "/" || src == "/" {
			return http.StatusForbidden, nil
		}

		err = checkParent(src, dst)
		if err != nil {
			return http.StatusBadRequest, err
		}

		override := r.URL.Query().Get("override") == "true"
		rename := r.URL.Query().Get("rename") == "true"
		if !override && !rename {
			if _, err = d.user.Fs.Stat(dst); err == nil {
				return http.StatusConflict, nil
			}
		}
		if rename {
			dst = addVersionSuffix(dst, d.user.Fs)
		}

		// Permission for overwriting the file
		if override && !d.user.Perm.Modify {
			return http.StatusForbidden, nil
		}

		err = d.RunHook(func() error {
			return patchAction(r.Context(), action, src, dst, d, fileCache)
		}, action, src, dst, d.user)

		return errToStatus(err), err
	})
}

func checkParent(src, dst string) error {
	rel, err := filepath.Rel(src, dst)
	if err != nil {
		return err
	}

	rel = filepath.ToSlash(rel)
	if !strings.HasPrefix(rel, "../") && rel != ".." && rel != "." {
		return fbErrors.ErrSourceIsParent
	}

	return nil
}

func addVersionSuffix(source string, fs afero.Fs) string {
	counter := 1
	dir, name := path.Split(source)
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)

	for {
		if _, err := fs.Stat(source); err != nil {
			break
		}
		renamed := fmt.Sprintf("%s(%d)%s", base, counter, ext)
		source = path.Join(dir, renamed)
		counter++
	}

	return source
}

func writeFile(fs afero.Fs, dst string, in io.Reader) (os.FileInfo, error) {
	dir, _ := path.Split(dst)
	err := fs.MkdirAll(dir, files.PermDir)
	if err != nil {
		return nil, err
	}

	file, err := fs.OpenFile(dst, os.O_RDWR|os.O_CREATE|os.O_TRUNC, files.PermFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	_, err = io.Copy(file, in)
	if err != nil {
		return nil, err
	}

	// Gets the info about the file.
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}

	return info, nil
}

func delThumbs(ctx context.Context, fileCache FileCache, file *files.FileInfo) error {
	for _, previewSizeName := range PreviewSizeNames() {
		size, _ := ParsePreviewSize(previewSizeName)
		if err := fileCache.Delete(ctx, previewCacheKey(file, size)); err != nil {
			return err
		}
	}

	return nil
}

func patchAction(ctx context.Context, action, src, dst string, d *data, fileCache FileCache) error {
	switch action {
	case "copy":
		if !d.user.Perm.Create {
			return fbErrors.ErrPermissionDenied
		}

		return fileutils.Copy(d.user.Fs, src, dst)
	case "rename":
		if !d.user.Perm.Rename {
			return fbErrors.ErrPermissionDenied
		}
		src = path.Clean("/" + src)
		dst = path.Clean("/" + dst)

		file, err := files.NewFileInfo(&files.FileOptions{
			Fs:         d.user.Fs,
			Path:       src,
			Modify:     d.user.Perm.Modify,
			Expand:     false,
			ReadHeader: false,
			Checker:    d,
		})
		if err != nil {
			return err
		}

		// delete thumbnails
		err = delThumbs(ctx, fileCache, file)
		if err != nil {
			return err
		}

		return fileutils.MoveFile(d.user.Fs, src, dst)
	default:
		return fmt.Errorf("unsupported action %s: %w", action, fbErrors.ErrInvalidRequestParams)
	}
}

type DiskUsageResponse struct {
	Total uint64 `json:"total"`
	Used  uint64 `json:"used"`
}

var diskUsage = withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	if strings.HasPrefix(r.URL.Path, "/virtual") {
		return renderJSON(w, r, &DiskUsageResponse{
			Total: 0,
			Used:  0,
		})
	}

	file, err := files.NewFileInfo(&files.FileOptions{
		Fs:         d.user.Fs,
		Path:       r.URL.Path,
		Modify:     d.user.Perm.Modify,
		Expand:     false,
		ReadHeader: false,
		Checker:    d,
		Content:    false,
	})
	if err != nil {
		return errToStatus(err), err
	}
	fPath := file.RealPath()
	if !file.IsDir {
		return renderJSON(w, r, &DiskUsageResponse{
			Total: 0,
			Used:  0,
		})
	}

	usage, err := disk.UsageWithContext(r.Context(), fPath)
	if err != nil {
		return errToStatus(err), err
	}
	return renderJSON(w, r, &DiskUsageResponse{
		Total: usage.Total,
		Used:  usage.Used,
	})
})
