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

func appDataPost(t *testing.T, s *appDataStore, body, origin string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "http://localhost:3001/app-data/items", strings.NewReader(body))
	request.Host = "localhost:3001"
	request.Header.Set("Content-Type", "application/json")
	if origin != "" {
		request.Header.Set("Origin", origin)
	}
	recorder := httptest.NewRecorder()
	s.ServeHTTP(recorder, request)
	return recorder
}

func appDataGet(t *testing.T, s *appDataStore) map[string]*string {
	t.Helper()
	recorder := httptest.NewRecorder()
	s.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "http://localhost:3001/app-data/items", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET status %d", recorder.Code)
	}
	var data struct {
		Items map[string]*string `json:"items"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &data); err != nil {
		t.Fatal(err)
	}
	return data.Items
}

func TestAppDataSavesMergesAndReloads(t *testing.T) {
	dir := t.TempDir()
	s := newAppDataStore(dir)
	if got := appDataGet(t, s); len(got) != 0 {
		t.Fatalf("new store not empty: %v", got)
	}
	if r := appDataPost(t, s, `{"items":{"pinless-playlists-v1":"[1]","pinless-history-v1":"[]"}}`, "http://localhost:3001"); r.Code != 200 {
		t.Fatalf("POST status %d %s", r.Code, r.Body)
	}
	if r := appDataPost(t, s, `{"items":{"pinless-history-v1":null}}`, "http://localhost:3001"); r.Code != 200 {
		t.Fatalf("POST delete status %d", r.Code)
	}
	reloaded := appDataGet(t, newAppDataStore(dir))
	if v := reloaded["pinless-playlists-v1"]; v == nil || *v != "[1]" {
		t.Fatalf("playlist not kept: %v", reloaded)
	}
	if v, ok := reloaded["pinless-history-v1"]; !ok || v != nil {
		t.Fatalf("deletion should be kept as null: %v", reloaded)
	}
	if names, _ := filepath.Glob(filepath.Join(dir, "daily", "store-*.json")); len(names) != 1 {
		t.Fatalf("expected one daily copy, got %v", names)
	}
}

func TestAppDataRejectsOtherSitesAndBadKeys(t *testing.T) {
	s := newAppDataStore(t.TempDir())
	for _, origin := range []string{"", "http://evil.example", "http://localhost:3012"} {
		if r := appDataPost(t, s, `{"items":{"pinless-playlists-v1":"x"}}`, origin); r.Code != http.StatusForbidden {
			t.Fatalf("origin %q: status %d", origin, r.Code)
		}
	}
	if r := appDataPost(t, s, `{"items":{"other-key":"x"}}`, "http://localhost:3001"); r.Code != http.StatusBadRequest {
		t.Fatalf("bad key: status %d", r.Code)
	}
	if len(appDataGet(t, s)) != 0 {
		t.Fatal("rejected writes were saved")
	}
}

func TestAppDataKeepsFirstSyncBackupAndSurvivesBadFile(t *testing.T) {
	dir := t.TempDir()
	s := newAppDataStore(dir)
	if r := appDataPost(t, s, `{"items":{},"backup":{"pinless-playlists-v1":"mine"}}`, "http://localhost:3001"); r.Code != 200 {
		t.Fatalf("backup status %d", r.Code)
	}
	if names, _ := filepath.Glob(filepath.Join(dir, "first-sync", "*.json")); len(names) != 1 {
		t.Fatalf("expected a first-sync backup, got %v", names)
	}
	if err := os.WriteFile(filepath.Join(dir, "store.json"), []byte("{broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := appDataGet(t, newAppDataStore(dir)); len(got) != 0 {
		t.Fatalf("unreadable file should start empty, got %v", got)
	}
	if names, _ := filepath.Glob(filepath.Join(dir, "store.unreadable-*.json")); len(names) != 1 {
		t.Fatalf("unreadable file should be kept aside, got %v", names)
	}
}
