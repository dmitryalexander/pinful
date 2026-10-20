package main

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTemporaryCleanupPreservesOtherFilesAndActiveDownloads(t *testing.T) {
	old := downloadRoot
	downloadRoot = t.TempDir()
	defer func() { downloadRoot = old }()
	os.Mkdir(filepath.Join(downloadRoot, "job-stale"), 0700)
	os.WriteFile(filepath.Join(downloadRoot, "job-stale", "partial.mp4"), []byte("partial"), 0600)
	keep := filepath.Join(downloadRoot, "unrelated")
	os.WriteFile(keep, []byte("saved"), 0600)
	request := func(origin string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "http://localhost:3001/app-data/temporary", strings.NewReader(`{"clear":true}`))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Origin", origin)
		w := httptest.NewRecorder()
		temporaryStorage(w, r)
		return w
	}
	if n, err := temporaryDownloadUsage(); err != nil || n != 7 {
		t.Fatalf("size %d %v", n, err)
	}
	if request("http://foreign.example").Code != 403 {
		t.Fatal("foreign cleanup accepted")
	}
	downloadFilesMu.RLock()
	w := request("http://localhost:3001")
	downloadFilesMu.RUnlock()
	if w.Code != 409 {
		t.Fatal("active download not protected")
	}
	if _, err := os.Stat(filepath.Join(downloadRoot, "job-stale", "partial.mp4")); err != nil {
		t.Fatal("active file removed")
	}
	if w := request("http://localhost:3001"); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if n, err := temporaryDownloadUsage(); err != nil || n != 0 {
		t.Fatal("temp data not cleared")
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatal("unrelated file removed")
	}
}
