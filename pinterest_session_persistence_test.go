package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPinterestRememberCheckAndForget(t *testing.T) {
	old := http.DefaultTransport
	defer func() { http.DefaultTransport = old }()
	authenticated := true
	http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		cookie, err := r.Cookie("_pinterest_sess")
		if err != nil || cookie.Value != "synthetic-session" {
			t.Fatal("saved cookie missing")
		}
		body := `{"client_context":{"is_authenticated":false}}`
		if authenticated {
			body = `{"client_context":{"is_authenticated":true}}`
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	dir := t.TempDir()
	s := &pinterestSessionStore{}
	if err := s.load(dir); err != nil {
		t.Fatal(err)
	}
	call := func(method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "http://localhost:3002"+path, strings.NewReader(body))
		r.Header.Set("Origin", "http://localhost:3002")
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		return w
	}
	if w := call("POST", "/pinterest-session", `{"session":"synthetic-session"}`); w.Code != 200 {
		t.Fatal(w.Code)
	}
	file := filepath.Join(dir, "pinterest-session.json")
	info, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatal("private file permissions", info.Mode())
	}
	s = &pinterestSessionStore{}
	if err := s.load(dir); err != nil {
		t.Fatal(err)
	}
	if w := call("GET", "/pinterest-session", ""); !strings.Contains(w.Body.String(), `"connected":true`) || strings.Contains(w.Body.String(), "synthetic") {
		t.Fatal("restart or isolation failed")
	}
	if w := call("POST", "/pinterest-session/check", `{}`); !strings.Contains(w.Body.String(), `"authenticated":true`) {
		t.Fatal("saved connection not checked")
	}
	authenticated = false
	if w := call("GET", "/pinterest-session", ""); !strings.Contains(w.Body.String(), `"connected":false`) || !strings.Contains(w.Body.String(), `"has_session":true`) {
		t.Fatal("saved invalid credentials were labeled connected")
	}
	if w := call("POST", "/pinterest-session/check", `{}`); !strings.Contains(w.Body.String(), `"authenticated":false`) {
		t.Fatal("invalid session not detected")
	}
	if w := call("POST", "/pinterest-session", `{"session":"synthetic-session"}`); w.Code != 401 {
		t.Fatal("invalid replacement accepted")
	}
	if _, err := os.Stat(file); err != nil {
		t.Fatal("check or failed replacement erased saved session")
	}
	if w := call("DELETE", "/pinterest-session", `{}`); w.Code != 200 {
		t.Fatal(w.Code)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatal("disconnect retained saved session")
	}
	s = &pinterestSessionStore{}
	if err := s.load(dir); err != nil {
		t.Fatal(err)
	}
	credentials, _ := s.snapshot()
	if credentials.Session != "" {
		t.Fatal("session restored after disconnect")
	}
}

func TestPinterestSaveFailureKeepsCurrentConnection(t *testing.T) {
	old := http.DefaultTransport
	defer func() { http.DefaultTransport = old }()
	http.DefaultTransport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"client_context":{"is_authenticated":true}}`))}, nil
	})
	s := &pinterestSessionStore{file: filepath.Join(t.TempDir(), "missing", "session.json"), credentials: pinterestCredentials{Session: "previous-session"}}
	r := httptest.NewRequest("POST", "http://localhost:3002/pinterest-session", strings.NewReader(`{"session":"replacement"}`))
	r.Header.Set("Origin", "http://localhost:3002")
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	credentials, _ := s.snapshot()
	if w.Code != 500 || credentials.Session != "previous-session" {
		t.Fatal("failed disk write replaced current connection")
	}
}
