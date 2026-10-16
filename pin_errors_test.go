package main

import (
	"bytes"
	"compress/gzip"
	"errors"
	"html/template"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestPinRequestsDistinguishFailureFromEmpty(t *testing.T) {
	old := http.DefaultTransport
	defer func() { http.DefaultTransport = old }()
	for _, tc := range []struct {
		name, body, encoding string
		status int
		networkError bool
	}{
		{name: "HTTP error with plausible data", status: 503, body: `{"resource_response":{"data":[]}}`},
		{name: "API error", status: 200, body: `{"resource_response":{"error":{"message":"denied"},"data":[]}}`},
		{name: "failed status", status: 200, body: `{"resource_response":{"status":"failure","data":[]}}`},
		{name: "missing resource", status: 200, body: `{}`},
		{name: "null data", status: 200, body: `{"resource_response":{"data":null}}`},
		{name: "HTML response", status: 200, body: `<html>Unavailable</html>`},
		{name: "wrong data shape", status: 200, body: `{"resource_response":{"data":"unexpected"}}`},
		{name: "broken gzip", status: 200, body: "bad gzip", encoding: "gzip"},
		{name: "network error", networkError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pinPreviews.Lock(); pinPreviews.items = make(map[string]pinPreview); pinPreviews.Unlock()
			http.DefaultTransport = profileTransport(func(r *http.Request) (*http.Response, error) {
				if tc.networkError { return nil, errors.New("offline") }
				return &http.Response{StatusCode: tc.status, Header: http.Header{"Content-Encoding": {tc.encoding}}, Body: io.NopCloser(strings.NewReader(tc.body))}, nil
			})
			pins, cursor := fetchRelatedPins("e1_example", "token", "cursor")
			if pins != nil || cursor != "" { t.Fatal("failure was presented as successful results") }
			pin := fetchPinDetails("e1_example", "token")
			if pin.ImageURL != "" || pin.VideoURL != "" { t.Fatal("failed pin exposed media") }
		})
	}
	for _, compressed := range []bool{false, true} {
		http.DefaultTransport = profileTransport(func(r *http.Request) (*http.Response, error) {
			body := `{"resource_response":{"status":"success","data":[]}}`
			header := make(http.Header)
			if compressed {
				var b bytes.Buffer
				w := gzip.NewWriter(&b); _, _ = w.Write([]byte(body)); _ = w.Close()
				body = b.String(); header.Set("Content-Encoding", "gzip")
			}
			return &http.Response{StatusCode: 200, Header: header, Body: io.NopCloser(strings.NewReader(body))}, nil
		})
		pins, cursor := fetchRelatedPins("123", "token", "")
		if pins == nil || len(pins) != 0 || cursor != "" { t.Fatal("valid empty results marked failed") }
	}
}

func TestPinPageReportsIndependentFailures(t *testing.T) {
	old := http.DefaultTransport
	defer func() { http.DefaultTransport = old }()
	tpl := template.Must(template.ParseFS(templatesFS, "templates/*"))
	for _, tc := range []struct { name string; pinOK, relatedOK bool }{
		{"both failed", false, false}, {"only related failed", true, false},
		{"only pin failed", false, true}, {"successful empty related", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pinPreviews.Lock(); pinPreviews.items = make(map[string]pinPreview); pinPreviews.Unlock()
			http.DefaultTransport = profileTransport(func(r *http.Request) (*http.Response, error) {
				body := `{"resource_response":{"error":{"message":"unavailable"}}}`
				if strings.Contains(r.URL.Path, "PinResource") && tc.pinOK {
					body = `{"resource_response":{"data":{"id":"123","images":{"orig":{"url":"https://i.pinimg.com/originals/example.jpg","width":600,"height":800}}}}}`
				} else if strings.Contains(r.URL.Path, "RelatedModulesResource") && tc.relatedOK {
					body = `{"resource_response":{"data":[]}}`
				}
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
			})
			router := gin.New(); router.SetHTMLTemplate(tpl); router.GET("/pin/:id", pinHandler)
			req := httptest.NewRequest("GET", "/pin/123?q=example&from=456&next&cursor=retry-cursor", nil)
			req.AddCookie(&http.Cookie{Name: "csrftoken", Value: "test-token"})
			out := httptest.NewRecorder(); router.ServeHTTP(out, req)
			html := out.Body.String()
			if out.Code != 200 { t.Fatalf("status %d", out.Code) }
			if !tc.pinOK {
				for _, part := range []string{"/static/warning.png", "Couldn’t load this pin", "This does not mean the pin was deleted.", "Back to the Feed"} {
					if !strings.Contains(html, part) { t.Fatalf("missing end-of-line content %q", part) }
				}
				if strings.Contains(html, "Could not load related pins") || strings.Contains(html, "grid-container pin-detail-wall") || strings.Contains(html, "No related pins found") {
					t.Fatal("unavailable pin should show a single illustrated state without an empty wall")
				}
			} else {
				if strings.Contains(html, "pin-end-title") { t.Fatal("loaded pin marked unavailable") }
				if strings.Contains(html, "Could not load related pins") == tc.relatedOK { t.Fatal("incorrect related error state") }
				if strings.Contains(html, "No related pins found") != tc.relatedOK { t.Fatal("empty message conceals failure") }
			}
			if strings.Contains(html, `class="pin-featured"`) != tc.pinOK { t.Fatal("missing pin created an empty featured tile") }
			if tc.pinOK && !tc.relatedOK {
				if !strings.Contains(html, `class="pin-retry" href=""`) { t.Fatal("retry must preserve current URL and continuation") }
			}
		})
	}
}

func TestPin404RequiresNumericIDAndUpstreamNotFound(t *testing.T) {
	connectedPinFixture(t)
	old := http.DefaultTransport; defer func() { http.DefaultTransport = old }()
	tpl := template.Must(template.ParseFS(templatesFS, "templates/*"))
	for _, tc := range []struct { id string; status int; missing bool }{
		{"987654321", 404, true}, {"987654321", 503, false}, {"987654321", 403, false}, {"e1_example", 404, false},
	} {
		pinPreviews.Lock(); pinPreviews.items = make(map[string]pinPreview); pinPreviews.Unlock()
		http.DefaultTransport = profileTransport(func(r *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: tc.status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{}`))}, nil
		})
		router := gin.New(); router.SetHTMLTemplate(tpl); router.GET("/pin/:id", pinHandler)
		req := httptest.NewRequest("GET", "/pin/"+tc.id, nil); req.AddCookie(&http.Cookie{Name:"csrftoken", Value:"test"})
		out := httptest.NewRecorder(); router.ServeHTTP(out, req)
		if strings.Contains(out.Body.String(), "data-confirmed-missing") != tc.missing { t.Fatalf("incorrect missing state for %s HTTP %d", tc.id, tc.status) }
	}
}

func TestRejectedEncryptedPinRetainsSearchMedia(t *testing.T) {
	old := http.DefaultTransport; defer func() { http.DefaultTransport = old }()
	http.DefaultTransport = profileTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 404, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{}`))}, nil
	})
	rememberPinPreview(Pin{ID:"e1_retained", ImageURL:"/image?url=https%3A%2F%2Fi.pinimg.com%2Foriginals%2Fexample.jpg", ImageWidth:600, ImageHeight:800})
	router := gin.New(); router.SetHTMLTemplate(template.Must(template.ParseFS(templatesFS, "templates/*"))); router.GET("/pin/:id", pinHandler)
	req := httptest.NewRequest("GET", "/pin/e1_retained", nil); req.AddCookie(&http.Cookie{Name:"csrftoken", Value:"test"})
	out := httptest.NewRecorder(); router.ServeHTTP(out, req)
	html := out.Body.String()
	for _, part := range []string{`class="pin-featured"`, "example.jpg", "Showing the image received from search", "Could not load related pins"} {
		if !strings.Contains(html, part) { t.Fatalf("missing recovered media/state %s", part) }
	}
	if strings.Contains(html, "data-confirmed-missing") { t.Fatal("encrypted pin marked deleted") }
 if out.Code != http.StatusOK || out.Header().Get("Location") != "" { t.Fatal("opaque pin must remain in Pinful without signing in") }
}

func connectedPinFixture(t *testing.T) {
 old, _ := pinterestSession.snapshot()
 pinterestSession.Lock(); pinterestSession.credentials = pinterestCredentials{Session: "synthetic-session"}; pinterestSession.Unlock()
 t.Cleanup(func() { pinterestSession.Lock(); pinterestSession.credentials = old; pinterestSession.Unlock() })
}
