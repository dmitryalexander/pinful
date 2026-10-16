package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRewriteHLS(t *testing.T) {
	base, _ := url.Parse("https://v1.pinimg.com/video/master.m3u8")
	input := "#EXTM3U\n#EXT-X-MEDIA:TYPE=AUDIO,URI=\"audio.m3u8\"\n#EXT-X-KEY:METHOD=AES-128,URI=\"../key.bin?x=1&y=2\"\n#EXT-X-MAP:URI=\"init.mp4\"\n#EXT-X-BYTERANGE:40@0\npart.ts\nhttps://v2.pinimg.com/other.m3u8\n"
	output, err := rewriteHLS(input, base)
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"https://v1.pinimg.com/video/audio.m3u8", "https://v1.pinimg.com/key.bin?x=1&y=2", "https://v1.pinimg.com/video/init.mp4", "https://v1.pinimg.com/video/part.ts", "https://v2.pinimg.com/other.m3u8"} {
		if !strings.Contains(output, "/hls?url="+url.QueryEscape(target)) {
			t.Fatalf("reference not proxied: %s", target)
		}
	}
	if !strings.Contains(output, "#EXT-X-BYTERANGE:40@0") {
		t.Fatal("byte range lost")
	}
	for _, invalid := range []string{"<html>error</html>", "#EXTM3U\nhttp://localhost/private", "#EXTM3U\n#EXT-X-KEY:URI=\"https://evil.example/key\"", "#EXTM3U\nfile:///etc/passwd"} {
		if _, err := rewriteHLS(invalid, base); err == nil {
			t.Fatalf("accepted invalid playlist: %s", invalid)
		}
	}
}

func TestHLSProxy(t *testing.T) {
	original := mediaClient
	t.Cleanup(func() { mediaClient = original })
	for _, method := range []string{"GET", "HEAD"} {
		mediaClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.Method != "GET" || r.Header.Get("Range") != "" {
				t.Fatal("playlist must be fetched in full")
			}
			final, _ := url.Parse("https://v2.pinimg.com/redirected/master.m3u8")
			return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"application/vnd.apple.mpegurl"}, "Content-Length": {"1"}}, Body: io.NopCloser(strings.NewReader("#EXTM3U\nchild.m3u8\n")), Request: &http.Request{URL: final}}, nil
		})}
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(method, "/hls?url="+url.QueryEscape("https://v1.pinimg.com/master.m3u8"), nil)
		c.Request.Header.Set("Range", "bytes=0-1")
		proxyHLSHandler(c)
		c.Writer.WriteHeaderNow()
		if w.Code != 200 || w.Header().Get("Content-Length") == "1" {
			t.Fatal("incorrect rewritten response")
		}
		if method == "HEAD" && w.Body.Len() != 0 {
			t.Fatal("HEAD body must be empty")
		}
		if method == "GET" && !strings.Contains(w.Body.String(), url.QueryEscape("https://v2.pinimg.com/redirected/child.m3u8")) {
			t.Fatal("redirect base not respected")
		}
	}
	mediaClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Range") != "bytes=0-3" {
			t.Fatal("segment range lost")
		}
		return &http.Response{StatusCode: 206, Header: http.Header{"Content-Type": {"video/mp2t"}, "Content-Range": {"bytes 0-3/100"}}, Body: io.NopCloser(strings.NewReader("data"))}, nil
	})}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/hls?url="+url.QueryEscape("https://v1.pinimg.com/part.ts"), nil)
	c.Request.Header.Set("Range", "bytes=0-3")
	proxyHLSHandler(c)
	if w.Code != 206 || w.Body.String() != "data" || w.Header().Get("Content-Range") != "bytes 0-3/100" {
		t.Fatal("segment corrupted")
	}
}

func TestLiveHLSPin(t *testing.T) {
	if os.Getenv("PINLESS_LIVE_TEST") != "1" {
		t.Skip("optional Pinterest live test")
	}
	pin := fetchPinDetails("3659243441431987", "")
	if !strings.HasPrefix(pin.VideoURL, "/hls?") {
		t.Fatalf("HLS story not selected: %+v", pin)
	}
	var visit func(string, int)
	segments := 0
	visit = func(local string, depth int) {
		if depth > 3 {
			t.Fatal("unexpected playlist nesting")
		}
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", local, nil)
		proxyHLSHandler(c)
		if w.Code != 200 {
			t.Fatalf("HLS fetch %d: %s", w.Code, local)
		}
		if w.Header().Get("Content-Type") != "application/vnd.apple.mpegurl" {
			if w.Body.Len() == 0 {
				t.Fatal("empty segment")
			}
			segments++
			return
		}
		body := w.Body.String()
		// Follow the alternate audio playlist and the first video rendition/segment.
		for _, line := range strings.Split(body, "\n") {
			if strings.HasPrefix(line, "#EXT-X-MEDIA:") {
				match := hlsURIAttribute.FindStringSubmatch(line)
				if len(match) > 1 {
					visit(match[1], depth+1)
				}
			}
		}
		for _, line := range strings.Split(body, "\n") {
			if strings.HasPrefix(line, "/hls?") {
				visit(line, depth+1)
				break
			}
		}
	}
	visit(pin.VideoURL, 0)
	if segments < 2 {
		t.Fatal("audio/video segments not both fetched")
	}
	t.Logf("HLS story selected; audio and video segments streamed successfully")
}
