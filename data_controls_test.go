package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func management(t *testing.T, s *appDataStore, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("POST", "http://localhost:3001"+path, strings.NewReader(string(raw)))
	r.Header.Set("Origin", "http://localhost:3001")
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	return w
}

func TestInfoRoundTripResetAndStaleClients(t *testing.T) {
	s := newAppDataStore(t.TempDir())
	if w := appDataPost(t, s, `{"items":{"pinless-playlists-v1":"{\"lists\":[]}","pinless-loop-video":"on"}}`, "http://localhost:3001"); w.Code != 200 {
		t.Fatal(w.Body)
	}
	w := management(t, s, "/app-data/export", map[string]any{})
	var backup infoBackup
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &backup) != nil || backup.App != "pinful" {
		t.Fatal(w.Code, w.Body)
	}
	if strings.Contains(w.Body.String(), "session") {
		t.Fatal("Export included session data")
	}
	bad := backup
	bad.App = "individuous"
	if w := management(t, s, "/app-data/import", map[string]any{"backup": bad}); w.Code != 400 {
		t.Fatal(w.Code)
	}
	if len(s.items) != 2 {
		t.Fatal("Invalid import changed data")
	}
	if w := management(t, s, "/app-data/reset", map[string]any{}); w.Code != 400 {
		t.Fatal(w.Code)
	}
	if w := management(t, s, "/app-data/reset", map[string]any{"confirm": "CLEAR EVERYTHING"}); w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	if len(s.items) != 0 || s.epoch == "" {
		t.Fatal("Reset failed")
	}
	if files, _ := filepath.Glob(filepath.Join(s.dir, "daily", "*.json")); len(files) != 0 {
		t.Fatal("Reset kept backups")
	}
	if w := appDataPost(t, s, `{"items":{"pinless-loop-video":"on"}}`, "http://localhost:3001"); w.Code != 409 {
		t.Fatal("Stale write accepted", w.Code)
	}
	oldEpoch := s.epoch
	wallToken := rememberPreviousWall([]Pin{{ID: "123456", ImageURL: "https://i.pinimg.com/test.jpg"}}, "", "")
	if w := management(t, s, "/app-data/import", map[string]any{"epoch": s.epoch, "backup": backup}); w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	if _, ok := recalledPreviousWall(wallToken); ok {
		t.Fatal("Import kept the discarded wall context")
	}
	if s.epoch == oldEpoch || len(s.items) != 2 {
		t.Fatal("Import failed")
	}
	restarted := newAppDataStore(s.dir)
	if restarted.epoch != s.epoch || *restarted.items["pinless-loop-video"] != "on" {
		t.Fatal("Restart lost import")
	}
}

func TestHashSaveRestoreNeverNeedsUpstreamAndProtectsExport(t *testing.T) {
	s := &pinterestSessionStore{}
	if err := s.load(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	send := func(path, method, origin, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "http://localhost:3001"+path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Origin", origin)
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		return w
	}
	if w := send("/pinterest-session/restore", "POST", "http://localhost:3001", `{"session":"synthetic-only","csrf":"test"}`); w.Code != 200 || strings.Contains(w.Body.String(), `"authenticated":true`) {
		t.Fatal(w.Code, w.Body)
	}
	if w := send("/pinterest-session/export", "POST", "https://evil.example", `{}`); w.Code != 403 {
		t.Fatal("Cross-origin export allowed")
	}
	if w := send("/pinterest-session/export", "POST", "http://localhost:3001", `{}`); w.Code != 200 || !strings.Contains(w.Body.String(), `"session":"synthetic-only"`) {
		t.Fatal("Export failed", w.Code)
	}
	if w := send("/pinterest-session/restore", "POST", "http://localhost:3001", `{"session":"bad;value"}`); w.Code != 400 {
		t.Fatal(w.Code)
	}
	c, _ := s.snapshot()
	if c.Session != "synthetic-only" {
		t.Fatal("Invalid restore replaced hash")
	}
	if w := send("/pinterest-session", "DELETE", "http://localhost:3001", ""); w.Code != 200 {
		t.Fatal(w.Code)
	}
	if _, err := os.Stat(s.file); !os.IsNotExist(err) {
		t.Fatal("Clear hash kept credential")
	}
}

func TestInfoRejectsInvalidBackupAndTrailingInput(t *testing.T) {
	s := newAppDataStore(t.TempDir())
	for _, body := range []string{`{"items":{}} {}`, `{"backup":{"password":"secret"}}`, `{"items":{"pinless-x":{}}}`} {
		if w := appDataPost(t, s, body, "http://localhost:3001"); w.Code != http.StatusBadRequest {
			t.Fatal(w.Code)
		}
	}
}
