package main

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestPreviousRecommendationsExcludeSelectedAndDuplicateImages(t *testing.T) {
	a := "https://i.pinimg.com/originals/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.jpg"
	b := "https://i.pinimg.com/originals/bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb.jpg"
	pins := []Pin{{ID: "1", ImageURL: a}, {ID: "2", ImageURL: "/image?url=" + url.QueryEscape(a)}, {ID: "3", ImageURL: b}, {ID: "4", ImageURL: "https://i.pinimg.com/236x/bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb.jpg"}}
	got := previousRecommendations(pins, Pin{ID: "1", ImageURL: a}, "1")
	if len(got) != 1 || got[0].ID != "3" {
		t.Fatalf("unexpected filtered pins: %+v", got)
	}
}
func TestPreviousWallRenderingAndPagination(t *testing.T) {
	old := http.DefaultTransport
	defer func() { http.DefaultTransport = old }()
	source := Pin{ID: "456", ImageURL: "/image?url=https%3A%2F%2Fi.pinimg.com%2Foriginals%2Faaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.jpg"}
	other := Pin{ID: "789", ImageURL: "/image?url=https%3A%2F%2Fi.pinimg.com%2Foriginals%2Fbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb.jpg"}
	token := rememberPreviousWall([]Pin{source, other}, "123", "next-parent-page")
	http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if strings.Contains(r.URL.Path, "PinResource") {
			return resolverReply(200, map[string]any{"resource_response": map[string]any{"data": map[string]any{"id": "456", "images": map[string]any{"orig": map[string]any{"url": "https://i.pinimg.com/originals/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.jpg", "width": 600, "height": 800}}}}}), nil
		}
		var data struct {
			Options struct {
				ID        string   `json:"pin_id"`
				Bookmarks []string `json:"bookmarks"`
			} `json:"options"`
		}
		_ = json.Unmarshal([]byte(r.URL.Query().Get("data")), &data)
		rows := []any{}
		if data.Options.ID == "123" {
			if len(data.Options.Bookmarks) != 1 || data.Options.Bookmarks[0] != "next-parent-page" {
				t.Fatal("parent cursor lost")
			}
			rows = []any{map[string]any{"type": "pin", "id": "999", "images": map[string]any{"orig": map[string]any{"url": "https://i.pinimg.com/originals/cccccccccccccccccccccccccccccccc.jpg", "width": 600, "height": 800}}}}
		}
		return resolverReply(200, map[string]any{"resource_response": map[string]any{"data": rows}}), nil
	})
	router := gin.New()
	router.SetHTMLTemplate(template.Must(template.ParseFS(templatesFS, "templates/*")))
	router.GET("/pin/:id", pinHandler)
	render := func(path string) string {
		req := httptest.NewRequest("GET", path, nil)
		req.AddCookie(&http.Cookie{Name: "csrftoken", Value: "synthetic"})
		out := httptest.NewRecorder()
		router.ServeHTTP(out, req)
		if out.Code != 200 {
			t.Fatal(out.Code)
		}
		return out.Body.String()
	}
	html := render("/pin/456?wall_context=" + token)
	if !strings.Contains(html, `<a href="/pin/456?wall_context=`+token+`"><img`) {
		t.Fatal("featured link must stay relative for saved hover controls while retaining wall context")
	}
	for _, part := range []string{`data-previous-wall="true"`, `data-lens-page="true"`, `data-featured-pin="456"`, `/pin/789?`, `wall_fallback=1`, `next-parent-page`} {
		if !strings.Contains(html, part) {
			t.Fatalf("missing %s", part)
		}
	}
	for _, part := range []string{`class="no-related"`, `class="pin-preview-note"`, `class="source-error pin-compact-notice"`, `data-single-pin="true"`} {
		if strings.Contains(html, part) {
			t.Fatalf("unexpected fallback text/layout %s", part)
		}
	}
	next := render("/pin/456?wall_context=" + token + "&wall_fallback=1&next&cursor=next-parent-page")
	if !strings.Contains(next, "/pin/999?") {
		t.Fatal("inherited pagination did not fetch parent seed")
	}
	direct := render("/pin/456")
	if strings.Contains(direct, `data-previous-wall="true"`) {
		t.Fatal("unrelated direct visit inherited another wall")
	}
}
func TestPreviousWallContextsAreBoundedAndIndependent(t *testing.T) {
	previousWalls.Lock()
	oldItems, oldOrder := previousWalls.items, previousWalls.order
	previousWalls.items = make(map[string]previousWall)
	previousWalls.order = nil
	previousWalls.Unlock()
	defer func() {
		previousWalls.Lock()
		previousWalls.items, previousWalls.order = oldItems, oldOrder
		previousWalls.Unlock()
	}()
	first := rememberPreviousWall([]Pin{{ID: "1"}}, "11", "")
	second := rememberPreviousWall([]Pin{{ID: "2"}}, "22", "")
	a, _ := recalledPreviousWall(first)
	b, _ := recalledPreviousWall(second)
	if a.Pins[0].ID == b.Pins[0].ID {
		t.Fatal("tabs shared unrelated recommendations")
	}
	for i := 0; i < 256; i++ {
		rememberPreviousWall([]Pin{{ID: "3"}}, "33", "")
	}
	if _, ok := recalledPreviousWall(first); ok {
		t.Fatal("wall context bound not enforced")
	}
}

func TestSearchWallContextAndContinuation(t *testing.T) {
	old := http.DefaultTransport
	defer func() { http.DefaultTransport = old }()
	http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if !strings.Contains(r.URL.Path, "BaseSearchResource") {
			t.Fatal("search continuation used the pin recommendations endpoint")
		}
		var data struct {
			Options struct {
				Query     string   `json:"query"`
				Bookmarks []string `json:"bookmarks"`
			} `json:"options"`
		}
		_ = json.Unmarshal([]byte(r.URL.Query().Get("data")), &data)
		if data.Options.Query != "art" {
			t.Fatalf("lost search query: %s", data.Options.Query)
		}
		return resolverReply(200, map[string]any{"resource_response": map[string]any{"data": map[string]any{"results": []any{map[string]any{"id": "789", "images": map[string]any{"orig": map[string]any{"url": "https://i.pinimg.com/originals/bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb.jpg", "width": 600, "height": 800}}}}}, "bookmark": "search-next"}}), nil
	})
	router := gin.New()
	router.SetHTMLTemplate(template.Must(template.ParseFS(templatesFS, "templates/*")))
	router.GET("/search/pins/", searchHandler)
	req := httptest.NewRequest("GET", "/search/pins/?q=art", nil)
	req.AddCookie(&http.Cookie{Name: "csrftoken", Value: "synthetic"})
	out := httptest.NewRecorder()
	router.ServeHTTP(out, req)
	html := out.Body.String()
	if !strings.Contains(html, "wall_context=") {
		t.Fatal("search cards lack originating wall context")
	}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/pin/456", nil)
	csrf := "synthetic"
	pins, next := fetchPreviousWallPage(c, previousWall{Seed: "search:art"}, "search-cursor", &csrf)
	if len(pins) != 1 || pins[0].ID != "789" || next != "search-next" {
		t.Fatal("search continuation lost images or cursor")
	}
	if strings.Contains(html, "pin-status.js") || strings.Contains(html, "pin-status.css") {
		t.Fatal("purple-status assets still loaded")
	}
}

func TestSavedWallContextImportsOnlyPublicMedia(t *testing.T) {
	router := gin.New()
	router.POST("/saved-wall-context", savedWallContextHandler)
	body := `{"pins":[{"id":"e1_saved_wall","image":"/image?url=https%3A%2F%2Fi.pinimg.com%2Foriginals%2Faaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.jpg","title":"Saved hero"},{"id":"789","image":"https://i.pinimg.com/originals/bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb.jpg"},{"id":"999","image":"/image?url=http%3A%2F%2F127.0.0.1%2Fprivate"}]}`
	request := func(origin string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "http://localhost:3001/saved-wall-context", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", origin)
		out := httptest.NewRecorder()
		router.ServeHTTP(out, req)
		return out
	}
	if out := request("https://foreign.example"); out.Code != 403 {
		t.Fatal("cross-origin import accepted")
	}
	out := request("http://localhost:3001")
	if out.Code != 200 {
		t.Fatal(out.Code, out.Body.String())
	}
	var data struct {
		Context string `json:"context"`
	}
	_ = json.Unmarshal(out.Body.Bytes(), &data)
	wall, ok := recalledPreviousWall(data.Context)
	if !ok || len(wall.Pins) != 2 || wall.Seed != "" || wall.Bookmark != "" {
		t.Fatal("saved context invalid")
	}
	hero, ok := recalledPinPreview("e1_saved_wall")
	if !ok || hero.Title != "Saved hero" {
		t.Fatal("selected saved image not available to pin handler")
	}
}
