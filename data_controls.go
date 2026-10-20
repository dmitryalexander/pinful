package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type infoBackup struct {
	Format  string             `json:"format"`
	Version int                `json:"version"`
	App     string             `json:"app"`
	Saved   string             `json:"saved,omitempty"`
	Items   map[string]*string `json:"items"`
}

func (s *appDataStore) manage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || !localPinterestHost(r.Host) || !appDataSameOrigin(r) ||
		r.Header.Get("Content-Type") != "application/json" {
		appDataReply(w, http.StatusForbidden, map[string]string{"message": "Use Pinful's Menu."})
		return
	}
	var request struct {
		Epoch   string     `json:"epoch"`
		Confirm string     `json:"confirm"`
		Backup  infoBackup `json:"backup"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 100<<20))
	decoder.DisallowUnknownFields()
	var extra any
	if decoder.Decode(&request) != nil || decoder.Decode(&extra) != io.EOF {
		appDataReply(w, http.StatusBadRequest, map[string]string{"message": "Invalid backup request."})
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		appDataReply(w, 503, map[string]string{"message": "Storage recovery is pending."})
		return
	}
	if request.Epoch != s.epoch {
		appDataReply(w, http.StatusConflict, map[string]string{"message": "Data changed. Reload before continuing."})
		return
	}
	if r.URL.Path == "/app-data/export" {
		appDataReply(w, http.StatusOK, infoBackup{"myapps-info", 1, "pinful", time.Now().UTC().Format(time.RFC3339), s.items})
		return
	}
	next := map[string]*string{}
	reset := r.URL.Path == "/app-data/reset"
	if reset {
		if request.Confirm != "CLEAR EVERYTHING" {
			appDataReply(w, http.StatusBadRequest, map[string]string{"message": "Confirm Clear everything in the Menu."})
			return
		}
	} else if r.URL.Path == "/app-data/import" {
		b := request.Backup
		if b.Format != "myapps-info" || b.Version != 1 || b.App != "pinful" || b.Items == nil {
			appDataReply(w, http.StatusBadRequest, map[string]string{"message": "Choose a Pinful info export (version 1)."})
			return
		}
		for key, value := range b.Items {
			if !appDataKeyPattern.MatchString(key) || key == "pinless-wall-snapshot-v1" {
				appDataReply(w, http.StatusBadRequest, map[string]string{"message": "The backup contains an unsupported item."})
				return
			}
			next[key] = value
		}
	} else {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	epoch := hex.EncodeToString(random)
	body, err := json.Marshal(map[string]any{"version": 1, "items": next, "epoch": epoch})
	if err != nil || len(body) > appDataMaxTotal {
		appDataReply(w, http.StatusRequestEntityTooLarge, map[string]string{"message": "Backup is too large."})
		return
	}
	journal, _ := json.Marshal(dataOperation{Items: next, Epoch: epoch, Reset: reset})
	if err := appDataWriteFile(filepath.Join(s.dir, "operation.json"), journal); err != nil {
		appDataReply(w, 503, map[string]string{"message": "Could not start the operation. Nothing changed."})
		return
	}
	s.err = s.finishOperation()
	if s.err != nil {
		appDataReply(w, 503, map[string]string{"message": "Operation saved for recovery. Restart the app to finish it; saving is paused."})
		return
	}
	appDataReply(w, 200, s.snapshot())
}

type dataOperation struct {
	Items map[string]*string `json:"items"`
	Epoch string             `json:"epoch"`
	Reset bool               `json:"reset"`
}

// A durable intent precedes every destructive step. Replaying is idempotent.
// Startup finishes an interrupted operation before accepting any requests.
func (s *appDataStore) finishOperation() error {
	raw, err := os.ReadFile(filepath.Join(s.dir, "operation.json"))
	if err != nil {
		return err
	}
	var op dataOperation
	if json.Unmarshal(raw, &op) != nil || op.Items == nil || op.Epoch == "" {
		return fmt.Errorf("invalid recovery operation")
	}
	if op.Reset {
		pinterestSession.Lock()
		defer pinterestSession.Unlock()
		if err := pinterestSession.persist(pinterestCredentials{}); err != nil {
			return err
		}
		pinterestSession.credentials = pinterestCredentials{}
		pinterestSession.generation++
	}
	body, _ := json.Marshal(map[string]any{"version": 1, "items": op.Items, "epoch": op.Epoch})
	if err := appDataWriteFile(filepath.Join(s.dir, "store.json"), body); err != nil {
		return err
	}
	if op.Reset {
		for _, pattern := range []string{"daily/store-*.json", "first-sync/*.json", "store.unreadable-*.json"} {
			names, _ := filepath.Glob(filepath.Join(s.dir, pattern))
			for _, name := range names {
				if err := os.Remove(name); err != nil {
					return err
				}
			}
		}
		for _, dir := range []string{"daily", "first-sync"} {
			if _, err := os.Stat(filepath.Join(s.dir, dir)); err == nil {
				if err := syncDirectory(filepath.Join(s.dir, dir)); err != nil {
					return err
				}
			}
		}
	}
	if err := os.Remove(filepath.Join(s.dir, "operation.json")); err != nil {
		return err
	}
	if err := syncDirectory(s.dir); err != nil {
		return err
	}
	s.items, s.epoch = op.Items, op.Epoch
	previousWalls.Lock()
	clear(previousWalls.items)
	previousWalls.order = nil
	previousWalls.Unlock()
	pinPreviews.Lock()
	clear(pinPreviews.items)
	pinPreviews.Unlock()
	return nil
}

func (s *appDataStore) recovery(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !localPinterestHost(r.Host) {
		w.WriteHeader(403)
		return
	}
	// A pending operation must be completed, never replaced by a recovery selection.
	if _, err := os.Stat(filepath.Join(s.dir, "operation.json")); err == nil {
		s.err = s.finishOperation()
		if s.err != nil {
			appDataReply(w, 503, map[string]string{"message": "Operation recovery is pending. Restart the app and retry."})
			return
		}
	}
	candidates := map[string]map[string]*string{}
	for _, pattern := range []string{"daily/store-*.json", "first-sync/*.json"} {
		names, _ := filepath.Glob(filepath.Join(s.dir, pattern))
		for _, name := range names {
			raw, err := os.ReadFile(name)
			if err != nil {
				continue
			}
			var saved dataOperation
			if json.Unmarshal(raw, &saved) != nil || saved.Items == nil {
				continue
			}
			valid := true
			for key := range saved.Items {
				if !appDataKeyPattern.MatchString(key) {
					valid = false
				}
			}
			if valid {
				id, _ := filepath.Rel(s.dir, name)
				candidates[filepath.ToSlash(id)] = saved.Items
			}
		}
	}
	if r.Method == "GET" && r.URL.Path == "/app-data/recovery" {
		choices := []map[string]any{}
		for id, data := range candidates {
			choices = append(choices, map[string]any{"id": id, "count": len(data)})
		}
		appDataReply(w, 200, map[string]any{"required": s.err != nil, "copies": choices})
		return
	}
	if r.Method != "POST" || !appDataSameOrigin(r) || r.Header.Get("Content-Type") != "application/json" {
		w.WriteHeader(403)
		return
	}
	var request struct {
		ID      string `json:"id"`
		Confirm string `json:"confirm"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&request) != nil || request.Confirm != "RECOVER" {
		w.WriteHeader(400)
		return
	}
	data, ok := candidates[request.ID]
	if !ok {
		w.WriteHeader(400)
		return
	}
	// Preserve damaged bytes for inspection; do not expose them or guess at their contents.
	if raw, err := os.ReadFile(filepath.Join(s.dir, "store.json")); err == nil {
		if err := appDataWriteFile(filepath.Join(s.dir, "store.unreadable-"+time.Now().Format("20060102-150405")+".json"), raw); err != nil {
			w.WriteHeader(503)
			return
		}
	}
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		w.WriteHeader(503)
		return
	}
	raw, _ := json.Marshal(dataOperation{Items: data, Epoch: hex.EncodeToString(random)})
	if err := appDataWriteFile(filepath.Join(s.dir, "operation.json"), raw); err != nil {
		w.WriteHeader(503)
		return
	}
	s.err = s.finishOperation()
	if s.err != nil {
		w.WriteHeader(503)
		return
	}
	appDataReply(w, 200, s.snapshot())
}
