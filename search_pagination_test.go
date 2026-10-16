package main

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSearchPaginationKeepsGETQueryScopeAndCursor(t *testing.T) {
	old := http.DefaultTransport
	defer func() { http.DefaultTransport = old }()
	cursors := []string{"", "page+two/==", "page+three/=="}
	requests := 0
	http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodGet || r.Body != nil {
			t.Fatal("search continuation must stay GET")
		}
		var data struct {
			Options struct {
				Query     string   `json:"query"`
				Scope     string   `json:"scope"`
				Bookmarks []string `json:"bookmarks"`
			} `json:"options"`
		}
		if err := json.Unmarshal([]byte(r.URL.Query().Get("data")), &data); err != nil {
			t.Fatal(err)
		}
		if data.Options.Query != "Titanfall & mecha" || data.Options.Scope != "videos" {
			t.Fatal("search context changed")
		}
		if requests == 0 && len(data.Options.Bookmarks) != 0 {
			t.Fatal("unexpected initial cursor")
		}
		if requests > 0 && (len(data.Options.Bookmarks) != 1 || data.Options.Bookmarks[0] != cursors[requests]) {
			t.Fatal("cursor corrupted")
		}
		next := "-end-"
		if requests < 2 {
			next = cursors[requests+1]
		}
		requests++
		payload := map[string]any{"resource_response": map[string]any{"data": map[string]any{"results": []any{map[string]any{"id": string(rune('0' + requests)), "images": map[string]any{"orig": map[string]any{"url": "https://i.pinimg.com/fixture.jpg", "width": 10, "height": 10}}}}}, "bookmark": next}}
		body, _ := json.Marshal(payload)
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(string(body)))}, nil
	})
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "http://localhost/search/pins/", nil)
	cursor, csrf := "", ""
	for page := 0; page < 3; page++ {
		pins, next, failure := fetchSearchPage(c, "Titanfall & mecha", "videos", cursor, &csrf)
		if failure != nil || len(pins) != 1 {
			t.Fatal("search page failed")
		}
		cursor = next
	}
	if requests != 3 || cursor != "" {
		t.Fatal("terminal cursor not handled")
	}
}
