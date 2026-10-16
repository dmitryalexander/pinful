package main

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestRelatedPaginationKeepsGETAndCursor(t *testing.T) {
	old := http.DefaultTransport
	defer func() { http.DefaultTransport = old }()
	requests := 0
	cursors := []string{"", "page+two/==", "page+three/=="}
	http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodGet || r.Body != nil { t.Fatal("continuation must stay GET") }
		if r.URL.Query().Get("source_url") != "/pin/123/" { t.Fatal("source URL lost") }
		var data struct { Options struct { ID string `json:"pin_id"`; Bookmarks []string `json:"bookmarks"` } `json:"options"` }
		if err := json.Unmarshal([]byte(r.URL.Query().Get("data")), &data); err != nil { t.Fatal(err) }
		if data.Options.ID != "123" { t.Fatal("pin changed") }
		if requests == 0 && len(data.Options.Bookmarks) != 0 { t.Fatal("unexpected first-page cursor") }
		if requests > 0 && (len(data.Options.Bookmarks) != 1 || data.Options.Bookmarks[0] != cursors[requests]) { t.Fatal("cursor was changed or lost") }
		next := "-end-"
		if requests < 2 { next = cursors[requests+1] }
		requests++
		payload := map[string]any{"resource_response":map[string]any{"data":[]any{map[string]any{"type":"pin","id":string(rune('0'+requests)),"images":map[string]any{"orig":map[string]any{"url":"https://i.pinimg.com/fixture.jpg","width":10,"height":10}}}},"bookmark":next}}
		body, _ := json.Marshal(payload)
		return &http.Response{StatusCode:200, Header:make(http.Header), Body:io.NopCloser(strings.NewReader(string(body)))}, nil
	})
	cursor := ""
	for page := 0; page < 3; page++ {
		pins, next := fetchRelatedPins("123", "", cursor)
		if len(pins) != 1 { t.Fatal("missing page") }
		cursor = next
	}
	if requests != 3 || cursor != "" { t.Fatal("terminal cursor not handled") }
}
