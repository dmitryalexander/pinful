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
	if r := appDataPost(t, s, `{"items":{"pinless-history-v1":null},"base":{"pinless-history-v1":"`+itemRevision(s.items, "pinless-history-v1")+`"}}`, "http://localhost:3001"); r.Code != 200 {
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
	broken := newAppDataStore(dir)
	if broken.err == nil {
		t.Fatal("corrupt file accepted")
	}
	if w := appDataPost(t, broken, `{"items":{"pinless-playlists-v1":"defaults"}}`, "http://localhost:3001"); w.Code != 503 {
		t.Fatal("write accepted during recovery")
	}
	raw, _ := os.ReadFile(filepath.Join(dir, "store.json"))
	if string(raw) != "{broken" {
		t.Fatal("damaged bytes changed")
	}
	names, _ := filepath.Glob(filepath.Join(dir, "first-sync", "*.json"))
	id, _ := filepath.Rel(dir, names[0])
	w := management(t, broken, "/app-data/recover", map[string]any{"id": filepath.ToSlash(id), "confirm": "RECOVER"})
	if w.Code != 200 || *broken.items["pinless-playlists-v1"] != "mine" {
		t.Fatal(w.Code, w.Body)
	}
}

func TestConcurrentCollectionsAndInterruptedReset(t *testing.T) {
	dir := t.TempDir()
	s := newAppDataStore(dir)
	if w := appDataPost(t, s, `{"items":{"pinless-playlists-v1":"first"}}`, "http://localhost:3001"); w.Code != 200 {
		t.Fatal(w.Code)
	}
	old := itemRevision(s.items, "pinless-playlists-v1")
	write := func(value string) int {
		raw, _ := json.Marshal(map[string]any{"items": map[string]string{"pinless-playlists-v1": value}, "base": map[string]string{"pinless-playlists-v1": old}})
		return appDataPost(t, s, string(raw), "http://localhost:3001").Code
	}
	if write("browser A") != 200 || write("browser B") != 409 {
		t.Fatal("lost update not rejected")
	}
	if *s.items["pinless-playlists-v1"] != "browser A" {
		t.Fatal("conflict changed saved data")
	}
	if w := appDataPost(t, s, `{"items":{"pinless-other":"independent"}}`, "http://localhost:3001"); w.Code != 200 {
		t.Fatal("unrelated key conflicted")
	}
	journal, _ := json.Marshal(dataOperation{Items: map[string]*string{}, Epoch: "recovered", Reset: true})
	if err := appDataWriteFile(filepath.Join(dir, "operation.json"), journal); err != nil {
		t.Fatal(err)
	}
	restarted := newAppDataStore(dir)
	if restarted.err != nil || restarted.epoch != "recovered" || len(restarted.items) != 0 {
		t.Fatal("interrupted reset not finished", restarted.err)
	}
	if _, err := os.Stat(filepath.Join(dir, "operation.json")); !os.IsNotExist(err) {
		t.Fatal("journal not finalized")
	}
}

func TestFirstSyncRetention(t *testing.T) {
	s := newAppDataStore(t.TempDir())
	v := "test"
	for i := 0; i < 25; i++ {
		if err := s.writeFirstSync(map[string]*string{"pinless-test": &v}); err != nil {
			t.Fatal(err)
		}
	}
	names, _ := filepath.Glob(filepath.Join(s.dir, "first-sync", "*.json"))
	if len(names) != 20 {
		t.Fatal(len(names))
	}
}
