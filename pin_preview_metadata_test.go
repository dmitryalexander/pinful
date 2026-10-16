package main

import (
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSearchRetainsPreviewMetadata(t *testing.T) {
	old := http.DefaultTransport
	defer func() { http.DefaultTransport = old }()
	for _, title := range []string{`"Garden"`, `{"format":"Garden","args":[]}`} {
		http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
			body := `{"resource_response":{"data":{"results":[{"id":"e1_metadata","title":` + title + `,"description":"Flowers","pinner":{"full_name":"Artist","username":"artist"},"images":{"orig":{"url":"https://i.pinimg.com/a.jpg","width":10,"height":20}}}]}}}`
			return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
		})
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest("GET", "http://localhost/search/pins/", nil)
		csrf := ""
		_, _, failure := fetchSearchPage(c, "flowers", "pins", "", &csrf)
		if failure != nil {
			t.Fatal("search failed")
		}
		pin, ok := recalledPinPreview("e1_metadata")
		if !ok || pin.Title != "Garden" || pin.Description != "Flowers" || pin.PinnerUser != "artist" || pin.PinnerName != "Artist" {
			t.Fatalf("metadata lost: %+v", pin)
		}
	}
}
