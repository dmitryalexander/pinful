package main

import (
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Downloads hold a read lock until their temporary directory has been removed.
// Cleanup never interrupts an active conversion or response stream.
var downloadFilesMu sync.RWMutex

func temporaryDownloadUsage() (int64, error) {
	var size int64
	entries, err := os.ReadDir(downloadRoot)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), "job-") {
			continue
		}
		err = filepath.WalkDir(filepath.Join(downloadRoot, entry.Name()), func(path string, d fs.DirEntry, err error) error {
			if os.IsNotExist(err) {
				return nil
			}
			if err != nil {
				return err
			}
			if d.Type().IsRegular() {
				info, err := d.Info()
				if os.IsNotExist(err) {
					return nil
				}
				if err != nil {
					return err
				}
				size += info.Size()
			}
			return nil
		})
		if err != nil {
			return 0, err
		}
	}
	return size, nil
}
func temporaryStorage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || !localPinterestHost(r.Host) || !appDataSameOrigin(r) || r.Header.Get("Content-Type") != "application/json" {
		appDataReply(w, 403, map[string]string{"message": "Use Pinful's Menu."})
		return
	}
	var request struct {
		Clear bool   `json:"clear"`
		Epoch string `json:"epoch"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
	decoder.DisallowUnknownFields()
	var extra any
	if decoder.Decode(&request) != nil || decoder.Decode(&extra) != io.EOF {
		appDataReply(w, 400, map[string]string{"message": "Invalid storage request."})
		return
	}
	if request.Clear {
		if !downloadFilesMu.TryLock() {
			appDataReply(w, 409, map[string]string{"message": "A download is active. Wait for it to finish, then clear temporary data."})
			return
		}
		defer downloadFilesMu.Unlock()
		if err := cleanDownloadFiles(); err != nil {
			appDataReply(w, 500, map[string]string{"message": "Could not clear temporary download files."})
			return
		}
	} else {
		downloadFilesMu.RLock()
		defer downloadFilesMu.RUnlock()
	}
	size, err := temporaryDownloadUsage()
	if err != nil {
		appDataReply(w, 500, map[string]string{"message": "Could not measure temporary download files."})
		return
	}
	appDataReply(w, 200, map[string]any{"bytes": size, "label": "Temporary video downloads", "activeDownloads": len(downloadSlots)})
}
