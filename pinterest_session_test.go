package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPinterestSessionLifecycleAndIsolation(t *testing.T) {
	old := http.DefaultTransport
	defer func() { http.DefaultTransport = old }()
	s := &pinterestSessionStore{}
	upstreamBody := `{"client_context":{"is_authenticated":true},"resource_response":{"data":{}}}`
	requests := 0
	http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		requests++
		cookie, err := r.Cookie("_pinterest_sess")
		if err != nil || cookie.Value != "synthetic-session" {
			t.Fatal("missing session")
		}
		if r.Header.Get("X-CSRFToken") != "synthetic-csrf" {
			t.Fatal("missing CSRF")
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(upstreamBody))}, nil
	})
	call := func(method, origin, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "http://localhost:3002/pinterest-session", strings.NewReader(body))
		r.Header.Set("Origin", origin)
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		return w
	}
	const body = `{"session":"synthetic-session","csrf":"synthetic-csrf"}`
	for _, origin := range []string{"", "https://evil.test", "https://localhost:3002", "http://localhost:3003"} {
		if call("POST", origin, body).Code != 403 {
			t.Fatal("foreign origin accepted")
		}
	}
	if requests != 0 {
		t.Fatal("blocked request reached upstream")
	}
	for _, invalid := range []string{`{"session":"x; other=y"}`, `{"session":"x\r\nInjected:y"}`, `{"session":""}`, body + `{}`, `{"session":"x","password":"unused"}`} {
		if call("POST", "http://localhost:3002", invalid).Code != 400 {
			t.Fatal("invalid payload accepted")
		}
	}
	if requests != 0 {
		t.Fatal("invalid request reached upstream")
	}
	if w := call("POST", "http://localhost:3002", body); w.Code != 200 {
		t.Fatal(w.Code)
	}
	status := call("GET", "", "")
	if !strings.Contains(status.Body.String(), `"connected":true`) || strings.Contains(status.Body.String(), "synthetic") || status.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("unsafe status")
	}
	for _, bad := range []string{`<html>Login</html>`, `{"client_context":{"is_authenticated":false}}`, `{"client_context":{"is_authenticated":true},"resource_response":{"error":{"message":"no"}}}`} {
		upstreamBody = bad
		if call("POST", "http://localhost:3002", body).Code != 401 {
			t.Fatal("unverified session accepted")
		}
		credentials, _ := s.snapshot()
		if credentials.Session == "" {
			t.Fatal("failed replacement lost connection")
		}
	}
	if call("DELETE", "http://localhost:3002", "{}").Code != 200 {
		t.Fatal("disconnect failed")
	}
	credentials, _ := s.snapshot()
	if credentials.Session != "" {
		t.Fatal("session retained")
	}
}

func TestPinterestCredentialsNeverFollowRedirectOrLeaveResourceHost(t *testing.T) {
	old := http.DefaultTransport
	defer func() { http.DefaultTransport = old }()
	requests := 0
	http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		requests++
		return &http.Response{StatusCode: 302, Header: http.Header{"Location": []string{"https://evil.test/"}}, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
	})
	client := clientWithPinterestCredentials(pinterestCredentials{Session: "synthetic-session"})
	for _, target := range []string{"https://evil.test/resource/PinResource/get/", "http://www.pinterest.com/resource/PinResource/get/", "https://www.pinterest.com:444/resource/PinResource/get/", "https://www.pinterest.com/login/", "https://i.pinimg.com/resource/PinResource/get/"} {
		if _, err := client.Get(target); err == nil {
			t.Fatal("credential destination accepted", target)
		}
	}
	if requests != 0 {
		t.Fatal("unexpected transport request")
	}
	resp, err := client.Get("https://www.pinterest.com/resource/PinResource/get/")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if requests != 1 || resp.StatusCode != 302 {
		t.Fatal("redirect followed")
	}
}

func TestPinterestLocalHostAndDisconnectWins(t *testing.T) {
	for _, host := range []string{"evil.test", "localhost.evil.test", "localhost@evil.test", "localhost/path", ""} {
		if localPinterestHost(host) {
			t.Fatal("nonlocal host accepted", host)
		}
	}
	for _, host := range []string{"localhost:3002", "127.0.0.1:3001", "[::1]:3002"} {
		if !localPinterestHost(host) {
			t.Fatal("local host refused", host)
		}
	}
	old := http.DefaultTransport
	defer func() { http.DefaultTransport = old }()
	s := &pinterestSessionStore{}
	http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		d := httptest.NewRequest("DELETE", "http://localhost:3002/pinterest-session", strings.NewReader("{}"))
		d.Header.Set("Origin", "http://localhost:3002")
		d.Header.Set("Content-Type", "application/json")
		s.ServeHTTP(httptest.NewRecorder(), d)
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"client_context":{"is_authenticated":true}}`))}, nil
	})
	r := httptest.NewRequest("POST", "http://localhost:3002/pinterest-session", strings.NewReader(`{"session":"synthetic-session"}`))
	r.Header.Set("Origin", "http://localhost:3002")
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != 409 {
		t.Fatal("disconnect did not win", w.Code)
	}
	c, _ := s.snapshot()
	if c.Session != "" {
		t.Fatal("session resurrected")
	}
}

func TestPinterestRequestsUseConnectedSession(t *testing.T) {
	old := http.DefaultTransport
	defer func() { http.DefaultTransport = old }()
	pinterestSession.Lock()
	pinterestSession.credentials = pinterestCredentials{Session: "synthetic-session", CSRF: "new-csrf"}
	pinterestSession.Unlock()
	defer func() {
		pinterestSession.Lock()
		pinterestSession.credentials = pinterestCredentials{}
		pinterestSession.Unlock()
	}()
	requests := 0
	http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		requests++
		if !strings.Contains(r.Header.Get("Cookie"), "_pinterest_sess=synthetic-session") || strings.Contains(r.Header.Get("Cookie"), "old-csrf") || r.Header.Get("X-CSRFToken") != "new-csrf" {
			t.Fatal("wrong session headers")
		}
		body := `{"resource_response":{"data":{"id":"123","images":{"orig":{"url":"https://i.pinimg.com/image.jpg","width":10,"height":10}}}}}`
		if strings.Contains(r.URL.Path, "RelatedModules") {
			body = `{"resource_response":{"data":[]}}`
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	pin, status := fetchPinDetailsResult("123", "old-csrf")
	if status != 200 || pin.ImageURL == "" {
		t.Fatal("detail request failed")
	}
	related, _ := fetchRelatedPins("123", "old-csrf", "")
	if related == nil || requests != 2 {
		t.Fatal("related request failed")
	}
}
