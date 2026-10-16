package main

import (
	"bytes"
	"encoding/json"
	"html/template"
	"image/gif"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

const sampleGIF = "https://i.pinimg.com/originals/5b/78/b2/5b78b2cb7bd1a2f057d2b0fab7ddaf47.gif"

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestAnimationSelection(t *testing.T) {
	var data struct {
		Images PinterestImages `json:"images"`
		PinterestMedia
	}
	input := `{"images":{"736x":{"url":"https://i.pinimg.com/736x/still.jpg","width":370,"height":324}},"embed":{"type":"gif","src":"` + sampleGIF + `"}}`
	if err := json.Unmarshal([]byte(input), &data); err != nil {
		t.Fatal(err)
	}
	image := bestPinImage(data.Images, data.PinterestMedia)
	if image.URL != sampleGIF || image.Width != 370 || image.Height != 324 {
		t.Fatalf("lost animation or dimensions: %+v", image)
	}
	data.Embed.Src = "https://evil.example/animation.gif"
	if bestPinImage(data.Images, data.PinterestMedia).URL != data.Images.Size736x.URL {
		t.Fatal("accepted an untrusted GIF")
	}
}

func TestVideoSelection(t *testing.T) {
	for _, ext := range []string{"mp4", "webm"} {
		var media PinterestMedia
		json.Unmarshal([]byte(`{"videos":{"video_list":{"hls":{"url":"https://v.pinimg.com/a.m3u8","width":2000},"file":{"url":"https://v.pinimg.com/a.`+ext+`","width":720,"height":1280}}}}`), &media)
		want := "/video?url=" + url.QueryEscape("https://v.pinimg.com/a."+ext)
		if got := videoProxyURL(media); got != want {
			t.Fatalf("%s: %s", ext, got)
		}
	}
}

func TestMediaProxy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	original := mediaClient
	t.Cleanup(func() { mediaClient = original })
	for _, tc := range []struct {
		name, method, kind, body string
		video                    bool
		status                   int
	}{
		{"gif", "GET", "image/gif", "GIF89a animation bytes", false, 200},
		{"webm seek", "GET", "video/webm", "partial video", true, 206},
		{"mp4 head", "HEAD", "video/mp4", "", true, 200},
		{"range error", "GET", "video/mp4", "", true, 416},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mediaClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if tc.video && r.Header.Get("Range") != "bytes=0-12" {
					t.Fatal("Range not forwarded")
				}
				return &http.Response{StatusCode: tc.status, Header: http.Header{"Content-Type": {tc.kind}, "Content-Range": {"bytes 0-12/100"}}, Body: io.NopCloser(strings.NewReader(tc.body))}, nil
			})}
			r := httptest.NewRequest(tc.method, "/image?url="+url.QueryEscape(sampleGIF), nil)
			r.Header.Set("Range", "bytes=0-12")
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = r
			proxyMedia(c, tc.video)
			c.Writer.WriteHeaderNow()
			if w.Code != tc.status || w.Body.String() != tc.body {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
			if tc.status != 416 && w.Header().Get("Content-Type") != tc.kind {
				t.Fatal("wrong media type")
			}
		})
	}
}

func TestTemplates(t *testing.T) {
	tmpl := template.Must(template.ParseFS(templatesFS, "templates/*"))
	for _, video := range []string{"", "/video?url=sample"} {
		pin := Pin{ID: "123", ImageURL: "/image?url=sample", VideoURL: video}
		var b bytes.Buffer
		if err := tmpl.ExecuteTemplate(&b, "pin.html", gin.H{"Pin": pin, "Related": []Pin{pin}}); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(b.String(), "<video") != (video != "") {
			t.Fatal("wrong player rendering")
		}
	}
}

func TestLiveSampleGIF(t *testing.T) {
	if os.Getenv("PINLESS_LIVE_TEST") != "1" {
		t.Skip("optional live Pinterest check")
	}
	pin := fetchPinDetails("373024781621410291", "")
	if !strings.Contains(pin.ImageURL, url.QueryEscape(sampleGIF)) {
		t.Fatalf("sample still uses preview: %+v", pin)
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", pin.ImageURL, nil)
	proxyImageHandler(c)
	if w.Code != 200 || w.Header().Get("Content-Type") != "image/gif" {
		t.Fatalf("GIF response: %d %s", w.Code, w.Header().Get("Content-Type"))
	}
	animation, err := gif.DecodeAll(bytes.NewReader(w.Body.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if len(animation.Image) < 2 {
		t.Fatal("animation flattened")
	}
	t.Logf("sample GIF retains %d frames", len(animation.Image))
}
