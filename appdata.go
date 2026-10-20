package main

// Server-side copy of Pinless's own data: playlists, history, followed creators, hidden pins and
// search groups. The pages keep using browser storage, and static/app-data.js mirrors every
// change here, so clearing the browser (or switching browsers) no longer loses anything.
//
//   GET  /app-data/items  -> {"items": {"key": "value" or null}}   (null = deleted on purpose)
//   POST /app-data/items  <- {"items": {...}, "backup": {...}}      (merge; same origin only)
//
// Files live in PINLESS_DATA (default /data, a Docker volume):
//   store.json                    the current data
//   daily/store-YYYY-MM-DD.json   one copy per day, the last 14 days
//   first-sync/<time>.json        a browser's own copy whenever it differed from the server on
//                                 its first connection (bounded retention; see pruneFirstSync)

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	appDataMaxBody  = 32 << 20 // one request
	appDataMaxTotal = 96 << 20 // everything saved
	appDataKeepDays = 14
)

var appDataKeyPattern = regexp.MustCompile(`^pinless-[A-Za-z0-9._-]{1,120}$`)

type appDataStore struct {
	mu    sync.Mutex
	dir   string
	items map[string]*string
	err   error  // set when the data folder is unusable; the app then keeps browser-only storage
	epoch string // changes on import/reset so old tabs cannot restore discarded data
}

type appDataRequest struct {
	Items  map[string]*string `json:"items"`
	Backup map[string]*string `json:"backup"`
	Epoch  string             `json:"epoch"`
	Base   map[string]string  `json:"base"`
}

func newAppDataStore(dir string) *appDataStore {
	s := &appDataStore{dir: dir, items: map[string]*string{}}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		s.err = err
		return s
	}
	raw, err := os.ReadFile(filepath.Join(dir, "store.json"))
	switch {
	case errors.Is(err, os.ErrNotExist):
	case err != nil:
		s.err = err
	default:
		var saved struct {
			Items map[string]*string `json:"items"`
			Epoch string             `json:"epoch"`
		}
		if json.Unmarshal(raw, &saved) != nil || saved.Items == nil {
			s.err = errors.New("saved data needs recovery")
		} else {
			s.epoch = saved.Epoch
			for key, value := range saved.Items {
				if appDataKeyPattern.MatchString(key) {
					s.items[key] = value
				} else {
					s.err = errors.New("saved data contains unsupported keys")
				}
			}
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "operation.json")); err == nil {
		s.err = s.finishOperation()
	}
	return s
}

func (s *appDataStore) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	if r.URL.Path == "/app-data/recovery" || r.URL.Path == "/app-data/recover" {
		s.recovery(w, r)
		return
	}
	s.mu.Lock()
	unavailable := s.err != nil
	s.mu.Unlock()
	if unavailable {
		appDataReply(w, http.StatusServiceUnavailable, map[string]string{"message": "Saved data needs recovery. Open Menu → Recover saved data. Saving is paused.", "code": "recovery"})
		return
	}
	if r.URL.Path != "/app-data/items" {
		s.manage(w, r)
		return
	}
	switch r.Method {
	case http.MethodGet:
		s.mu.Lock()
		if s.err != nil {
			s.mu.Unlock()
			appDataReply(w, 503, map[string]string{"message": "Storage recovery is pending."})
			return
		}
		body, err := json.Marshal(s.snapshot())
		s.mu.Unlock()
		if err != nil {
			appDataReply(w, http.StatusInternalServerError, map[string]string{"message": "Could not read saved data."})
			return
		}
		_, _ = w.Write(body)
	case http.MethodPost:
		s.post(w, r)
	default:
		w.Header().Set("Allow", "GET, POST")
		appDataReply(w, http.StatusMethodNotAllowed, map[string]string{"message": "GET or POST only."})
	}
}

func (s *appDataStore) post(w http.ResponseWriter, r *http.Request) {
	// Only this app's own pages may write: other websites cannot forge a same-origin JSON POST.
	if !appDataSameOrigin(r) || !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		appDataReply(w, http.StatusForbidden, map[string]string{"message": "Local app only."})
		return
	}
	var request appDataRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, appDataMaxBody))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		appDataReply(w, http.StatusBadRequest, map[string]string{"message": "Invalid data."})
		return
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		appDataReply(w, http.StatusBadRequest, map[string]string{"message": "Invalid data."})
		return
	}
	for key := range request.Items {
		if !appDataKeyPattern.MatchString(key) {
			appDataReply(w, http.StatusBadRequest, map[string]string{"message": "Invalid key: " + key})
			return
		}
	}
	for key := range request.Backup {
		if !appDataKeyPattern.MatchString(key) {
			appDataReply(w, http.StatusBadRequest, map[string]string{"message": "Invalid backup key."})
			return
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		appDataReply(w, 503, map[string]string{"message": "Storage recovery is pending."})
		return
	}
	if request.Epoch != s.epoch {
		appDataReply(w, http.StatusConflict, map[string]string{"message": "Data was imported or cleared. Reload this page.", "epoch": s.epoch})
		return
	}
	conflicts := []string{}
	for key := range request.Items {
		if request.Base[key] != itemRevision(s.items, key) {
			conflicts = append(conflicts, key)
		}
	}
	if len(conflicts) > 0 {
		reply := s.snapshot()
		reply["code"], reply["conflicts"] = "conflict", conflicts
		reply["message"] = "Another browser changed this collection. Choose which version to keep."
		appDataReply(w, http.StatusConflict, reply)
		return
	}
	if len(request.Backup) > 0 {
		if err := s.writeFirstSync(request.Backup); err != nil {
			appDataReply(w, http.StatusInternalServerError, map[string]string{"message": "Could not keep a backup."})
			return
		}
	}
	if len(request.Items) == 0 {
		appDataReply(w, http.StatusOK, map[string]bool{"ok": true})
		return
	}
	next := make(map[string]*string, len(s.items)+len(request.Items))
	for key, value := range s.items {
		next[key] = value
	}
	for key, value := range request.Items {
		next[key] = value
	}
	body, err := json.Marshal(map[string]any{"version": 1, "items": next, "epoch": s.epoch})
	if err != nil || len(body) > appDataMaxTotal {
		appDataReply(w, http.StatusRequestEntityTooLarge, map[string]string{"message": "Saved data is too large."})
		return
	}
	s.keepDailyCopy()
	if err := appDataWriteFile(filepath.Join(s.dir, "store.json"), body); err != nil {
		appDataReply(w, http.StatusInternalServerError, map[string]string{"message": "Could not save."})
		return
	}
	s.items = next
	appDataReply(w, http.StatusOK, s.snapshot())
}

// keepDailyCopy saves the current file once per day before it changes, and keeps the last 14 days.
func (s *appDataStore) keepDailyCopy() {
	current := filepath.Join(s.dir, "store.json")
	raw, err := os.ReadFile(current)
	if err != nil {
		return
	}
	daily := filepath.Join(s.dir, "daily")
	if os.MkdirAll(daily, 0o755) != nil {
		return
	}
	today := filepath.Join(daily, "store-"+time.Now().Format("2006-01-02")+".json")
	if _, err := os.Stat(today); err == nil {
		return
	}
	if appDataWriteFile(today, raw) != nil {
		return
	}
	names, _ := filepath.Glob(filepath.Join(daily, "store-*.json"))
	sort.Strings(names)
	for len(names) > appDataKeepDays {
		_ = os.Remove(names[0])
		names = names[1:]
	}
}

func (s *appDataStore) writeFirstSync(backup map[string]*string) error {
	folder := filepath.Join(s.dir, "first-sync")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		return err
	}
	body, err := json.MarshalIndent(map[string]any{"saved": time.Now().Format(time.RFC3339), "items": backup}, "", " ")
	if err != nil {
		return err
	}
	name := fmt.Sprintf("%s-%d.json", time.Now().Format("20060102-150405"), time.Now().UnixNano())
	if err := appDataWriteFile(filepath.Join(folder, name), body); err != nil {
		return err
	}
	return pruneFirstSync(folder)
}

func appDataSameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return false
	}
	parsed, err := url.Parse(origin)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host == r.Host
}

// appDataWriteFile replaces a file atomically, so a crash never leaves half-written data.
func appDataWriteFile(path string, body []byte) error {
	temp, err := os.CreateTemp(filepath.Dir(path), ".saving-*")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	if _, err := temp.Write(body); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(temp.Name(), path); err != nil {
		return err
	}
	return syncDirectory(filepath.Dir(path))
}

func syncDirectory(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

// A digest of the exact stored representation is an opaque per-item revision.
// Missing and explicitly deleted entries have different revisions.
func itemRevision(items map[string]*string, key string) string {
	value, exists := items[key]
	if !exists {
		return ""
	}
	raw, _ := json.Marshal(value)
	return fmt.Sprintf("%x", sha256.Sum256(raw))
}

func (s *appDataStore) snapshot() map[string]any {
	revisions := map[string]string{}
	for key := range s.items {
		revisions[key] = itemRevision(s.items, key)
	}
	return map[string]any{"items": s.items, "epoch": s.epoch, "revisions": revisions}
}

// At most 20 snapshots, 128 MiB total, and 30 days old. The newest copy survives.
func pruneFirstSync(folder string) error {
	names, _ := filepath.Glob(filepath.Join(folder, "*.json"))
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	var size int64
	for i, name := range names {
		info, err := os.Stat(name)
		if err != nil {
			return err
		}
		size += info.Size()
		if i > 0 && (i >= 20 || size > 128<<20 || time.Since(info.ModTime()) > 30*24*time.Hour) {
			if err := os.Remove(name); err != nil {
				return err
			}
		}
	}
	return syncDirectory(folder)
}

func appDataReply(w http.ResponseWriter, status int, data any) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func appDataDir() string {
	if dir := os.Getenv("PINLESS_DATA"); dir != "" {
		return dir
	}
	return "/data"
}
