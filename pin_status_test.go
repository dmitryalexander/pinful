package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestPinStatusDistinguishesEmptyFailureAndUnresolved(t *testing.T) {
	old := http.DefaultTransport
	defer func() { http.DefaultTransport = old }()
	for _, tc := range []struct {
		name, state string
		status      int
		data        any
		bookmark    string
	}{
		{"available", "available", 200, []any{map[string]any{"id": "321", "type": "pin", "images": resolverDetail(Pin{})["images"]}}, ""},
		{"associated pin without renderable media", "available", 200, []any{map[string]any{"id": "321", "type": "pin"}}, ""},
		{"confirmed empty", "empty", 200, []any{}, ""},
		{"HTTP failure", "unknown", 503, []any{}, ""},
		{"bad shape", "unknown", 200, "unexpected", ""},
		{"empty nonterminal page", "unknown", 200, []any{}, "more"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				var data struct {
					Options struct {
						PageSize int `json:"page_size"`
					} `json:"options"`
				}
				_ = json.Unmarshal([]byte(r.URL.Query().Get("data")), &data)
				if data.Options.PageSize != 1 {
					t.Fatal("background probe should request a single related pin")
				}
				return resolverReply(tc.status, map[string]any{"resource_response": map[string]any{"data": tc.data, "bookmark": tc.bookmark}}), nil
			})
			if got := inspectPinStatus("123"); got != tc.state {
				t.Fatalf("%s, want %s", got, tc.state)
			}
		})
	}
	t.Run("unresolved opaque", func(t *testing.T) {
		resolverFixture(t)
		http.DefaultTransport = roundTripFunc(func(*http.Request) (*http.Response, error) {
			t.Fatal("no source evidence must not trigger arbitrary requests")
			return nil, nil
		})
		if inspectPinStatus("e1_status_missing") != "unresolved" {
			t.Fatal("missing evidence is unresolved, not an empty related wall")
		}
	})
}

func TestPinStatusEndpointValidationCacheAndThrottle(t *testing.T) {
	oldCache := pinStatuses
	pinStatuses = newResolverCache[string](2048)
	defer func() { pinStatuses = oldCache }()
	statusAdmission.Lock()
	statusAdmission.next = time.Time{}
	statusAdmission.active = 0
	statusAdmission.Unlock()
	old := http.DefaultTransport
	defer func() { http.DefaultTransport = old }()
	count := 0
	http.DefaultTransport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		count++
		return resolverReply(200, map[string]any{"resource_response": map[string]any{"data": []any{}}}), nil
	})
	router := gin.New()
	router.GET("/pin-status", pinStatusHandler)
	request := func(id string) *httptest.ResponseRecorder {
		out := httptest.NewRecorder()
		router.ServeHTTP(out, httptest.NewRequest("GET", "/pin-status?id="+id, nil))
		return out
	}
	if out := request("bad"); out.Code != 400 {
		t.Fatal("malformed pin accepted")
	}
	for i := 0; i < 2; i++ {
		out := request("123")
		if out.Code != 200 || !strings.Contains(out.Body.String(), `"empty"`) || out.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("incorrect status response")
		}
	}
	if count != 1 {
		t.Fatal("cached request repeated upstream lookup")
	}
	if out := request("456"); out.Code != 429 || out.Header().Get("Retry-After") == "" {
		t.Fatal("background rate limit missing")
	}
}
