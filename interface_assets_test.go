package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/gin-gonic/gin"
)

func TestInterfaceBuildTracksBytes(t *testing.T) {
	assets := fstest.MapFS{"static/app.js": {Data: []byte("old")}, "static/art.png": {Data: []byte("art")}}
	first, err := interfaceBuild(assets)
	if err != nil {
		t.Fatal(err)
	}
	same, _ := interfaceBuild(assets)
	if first != same {
		t.Fatal("unchanged files changed build")
	}
	assets["static/art.png"].Data = []byte("new art")
	next, _ := interfaceBuild(assets)
	if first == next {
		t.Fatal("artwork edit did not change build")
	}
}
func TestEveryTemplateAssetGetsCurrentBuild(t *testing.T) {
	build, err := interfaceBuild(staticFS)
	if err != nil {
		t.Fatal(err)
	}
	pages, err := loadInterfaceTemplates(templatesFS, staticFS)
	if err != nil {
		t.Fatal(err)
	}
	// Inspect parsed template text: covers every page without requiring upstream data.
	for _, page := range pages.Templates() {
		if page.Tree == nil {
			continue
		}
		text := page.Tree.Root.String()
		for _, part := range strings.Split(text, `/static/`)[1:] {
			ref := strings.SplitN(part, `"`, 2)[0]
			if !strings.HasSuffix(ref, "?v="+build) {
				t.Fatalf("%s has stale asset: %s", page.Name(), ref)
			}
		}
	}
	fixture := fstest.MapFS{"templates/test.html": {Data: []byte(`<script src="/static/app.js?v=old"></script><img src="/static/art.png">`)}}
	tmpl, err := loadInterfaceTemplates(fixture, staticFS)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := tmpl.ExecuteTemplate(&out, "test.html", nil); err != nil {
		t.Fatal(err)
	}
	if strings.Count(out.String(), "?v="+build) != 2 {
		t.Fatal(out.String())
	}
}
func TestInterfaceCachePolicies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(interfaceCachePolicy())
	for _, path := range []string{"/", "/about", "/app-data/items", "/pinterest-session", "/static/app.js", "/static/art.png", "/video", "/hls"} {
		router.GET(path, func(c *gin.Context) { c.String(http.StatusOK, "test") })
		r := httptest.NewRecorder()
		router.ServeHTTP(r, httptest.NewRequest("GET", path, nil))
		want := "no-store"
		if strings.HasPrefix(path, "/static/") {
			want = "no-cache"
		}
		if path == "/video" || path == "/hls" {
			want = ""
		}
		if got := r.Header().Get("Cache-Control"); got != want {
			t.Errorf("%s: %q, want %q", path, got, want)
		}
	}
}
