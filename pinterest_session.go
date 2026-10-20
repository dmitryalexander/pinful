package main

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// This optional connection belongs to a local, single-user server. Credentials
// never enter app-data or browser storage. Only an explicit same-origin export
// returns them to the user for downloading. A private file in
// the installation's data volume remembers the last verified connection.
type pinterestCredentials struct {
	Session string `json:"session"`
	CSRF    string `json:"csrf"`
}

type pinterestSessionStore struct {
	sync.RWMutex
	credentials pinterestCredentials
	generation  uint64
	file        string
}

var pinterestSession pinterestSessionStore

func (s *pinterestSessionStore) load(dir string) error {
	s.file = filepath.Join(dir, "pinterest-session.json")
	raw, err := os.ReadFile(s.file)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return errors.New("saved Pinterest connection unavailable")
	}
	var credentials pinterestCredentials
	if json.Unmarshal(raw, &credentials) != nil || !validPinterestCookie(credentials.Session, true) || !validPinterestCookie(credentials.CSRF, false) {
		return errors.New("saved Pinterest connection invalid")
	}
	s.credentials = credentials
	return nil
}

// Caller holds the store lock. Atomic replacement never overwrites a good
// saved connection with a partial write. This file is excluded from app backups.
func (s *pinterestSessionStore) persist(credentials pinterestCredentials) error {
	if s.file == "" {
		return nil
	}
	if credentials.Session == "" {
		err := os.Remove(s.file)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		return syncDirectory(filepath.Dir(s.file))
	}
	raw, err := json.Marshal(credentials)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(s.file), ".pinterest-session-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(raw); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), s.file)
}

func (s *pinterestSessionStore) snapshot() (pinterestCredentials, uint64) {
	s.RLock()
	defer s.RUnlock()
	return s.credentials, s.generation
}

func localPinterestHost(host string) bool {
	u, err := url.Parse("http://" + host)
	if err != nil || u.User != nil || u.Path != "" {
		return false
	}
	switch u.Hostname() {
	case "localhost", "127.0.0.1", "::1":
		return true
	}
	return false
}

func validPinterestCookie(value string, required bool) bool {
	if value == "" {
		return !required
	}
	if len(value) > 8192 {
		return false
	}
	for _, ch := range value {
		if ch < 0x21 || ch > 0x7e || strings.ContainsRune("\";,\\", ch) {
			return false
		}
	}
	return true
}

type pinterestTransport struct {
	base        http.RoundTripper
	credentials pinterestCredentials
}

func (t pinterestTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	copy := req.Clone(req.Context())
	// Auth may go only to the website's read-resource endpoints, never proxies,
	// arbitrary URLs, alternate hosts, ports or redirect targets.
	allowed := copy.URL.Scheme == "https" && copy.URL.Host == "www.pinterest.com" &&
		copy.URL.User == nil && strings.HasPrefix(copy.URL.Path, "/resource/") && strings.HasSuffix(copy.URL.Path, "/get/")
	if t.credentials.Session != "" {
		if !allowed {
			return nil, errors.New("Pinterest session destination refused")
		}
		copy.Header.Del("Cookie")
		copy.Header.Del("X-CSRFToken")
		copy.AddCookie(&http.Cookie{Name: "_pinterest_sess", Value: t.credentials.Session})
		copy.AddCookie(&http.Cookie{Name: "_auth", Value: "1"})
		if t.credentials.CSRF != "" {
			copy.AddCookie(&http.Cookie{Name: "csrftoken", Value: t.credentials.CSRF})
			copy.Header.Set("X-CSRFToken", t.credentials.CSRF)
		}
	}
	return t.base.RoundTrip(copy)
}

func clientWithPinterestCredentials(credentials pinterestCredentials) *http.Client {
	return &http.Client{
		Timeout:       20 * time.Second,
		Transport:     pinterestTransport{base: http.DefaultTransport, credentials: credentials},
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
	}
}

func newPinterestClient() *http.Client {
	credentials, _ := pinterestSession.snapshot()
	return clientWithPinterestCredentials(credentials)
}

// Verify an explicit upstream authentication flag, not a 200 HTML shell or the
// presence of the imported cookie. This does not assert recommendations work.
func verifyPinterestSession(credentials pinterestCredentials) bool {
	data := url.QueryEscape(`{"options":{"query":"sunflower"},"context":{}}`)
	req, _ := http.NewRequest(http.MethodGet, "https://www.pinterest.com/resource/BaseSearchResource/get/?source_url=%2Fsearch%2Fpins%2F%3Fq%3Dsunflower&data="+data, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("x-pinterest-pws-handler", "www/search/[scope].js")
	resp, err := clientWithPinterestCredentials(credentials).Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false
	}
	var result struct {
		Context struct {
			Authenticated bool `json:"is_authenticated"`
		} `json:"client_context"`
		Resource struct {
			Error json.RawMessage `json:"error"`
		} `json:"resource_response"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&result) != nil {
		return false
	}
	return result.Context.Authenticated && (len(result.Resource.Error) == 0 || string(result.Resource.Error) == "null")
}

func (s *pinterestSessionStore) checkStatus(w http.ResponseWriter) {
	credentials, generation := s.snapshot()
	verified := credentials.Session != "" && verifyPinterestSession(credentials)
	s.RLock()
	defer s.RUnlock()
	if s.generation != generation {
		appDataReply(w, http.StatusConflict, map[string]string{"message": "Connection changed. Check again."})
		return
	}
	appDataReply(w, http.StatusOK, map[string]any{"connected": verified, "authenticated": verified, "has_session": credentials.Session != "", "recommendations": "unverified"})
}

func (s *pinterestSessionStore) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	if !localPinterestHost(r.Host) {
		appDataReply(w, http.StatusForbidden, map[string]string{"message": "Open Pinful on localhost to connect."})
		return
	}
	if r.Method == http.MethodGet {
		s.checkStatus(w)
		return
	}
	mediaType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	expectedScheme := "http"
	if r.TLS != nil {
		expectedScheme = "https"
	}
	if r.Header.Get("Origin") != expectedScheme+"://"+r.Host || mediaType != "application/json" {
		appDataReply(w, http.StatusForbidden, map[string]string{"message": "Use the connection controls in Pinful's Menu."})
		return
	}
	if r.Method == http.MethodDelete {
		s.Lock()
		if s.persist(pinterestCredentials{}) != nil {
			s.Unlock()
			appDataReply(w, http.StatusInternalServerError, map[string]string{"message": "Could not remove the saved connection. Try again."})
			return
		}
		s.credentials = pinterestCredentials{}
		s.generation++
		s.Unlock()
		appDataReply(w, http.StatusOK, map[string]bool{"connected": false, "authenticated": false, "has_session": false})
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, POST, DELETE")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if r.URL.Path == "/pinterest-session/check" {
		s.checkStatus(w)
		return
	}
	if r.URL.Path == "/pinterest-session/export" {
		credentials, _ := s.snapshot()
		if credentials.Session == "" {
			appDataReply(w, http.StatusNotFound, map[string]string{"message": "No hash is saved."})
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		appDataReply(w, http.StatusOK, map[string]any{"format": "myapps-hash", "version": 1, "app": "pinful", "credentials": credentials})
		return
	}
	var credentials pinterestCredentials
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 18000))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&credentials) != nil || !validPinterestCookie(credentials.Session, true) || !validPinterestCookie(credentials.CSRF, false) {
		appDataReply(w, http.StatusBadRequest, map[string]string{"message": "Enter the cookie values only, without names or a Cookie header."})
		return
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	_, generation := s.snapshot()
	restoring := r.URL.Path == "/pinterest-session/restore"
	if !restoring && !verifyPinterestSession(credentials) {
		appDataReply(w, http.StatusUnauthorized, map[string]string{"message": "Pinterest did not confirm this session. Check the cookie values and try again. An existing connection is unchanged."})
		return
	}
	s.Lock()
	defer s.Unlock()
	// A disconnect or successful replacement during verification wins.
	if s.generation != generation {
		appDataReply(w, http.StatusConflict, map[string]string{"message": "Connection changed. Try again."})
		return
	}
	if s.persist(credentials) != nil {
		appDataReply(w, http.StatusInternalServerError, map[string]string{"message": "Could not save the connection locally. Existing connection unchanged."})
		return
	}
	s.credentials = credentials
	s.generation++
	appDataReply(w, http.StatusOK, map[string]any{"connected": !restoring, "authenticated": !restoring, "has_session": true, "recommendations": "unverified"})
}
