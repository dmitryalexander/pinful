package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func downloadTestSetup(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	oldRoot, oldClient := downloadRoot, mediaClient
	downloadRoot = t.TempDir()
	t.Cleanup(func() { downloadRoot, mediaClient = oldRoot, oldClient })
	r := gin.New()
	r.GET("/download", downloadVideoHandler)
	return r
}

func assertNoDownloadFiles(t *testing.T) {
	t.Helper()
	entries, err := os.ReadDir(downloadRoot)
	if err != nil || len(entries) != 0 {
		t.Fatalf("temporary files left behind: %v, %v", entries, err)
	}
}

func TestDownloadRestartCleanup(t *testing.T) {
	downloadTestSetup(t)
	stale := filepath.Join(downloadRoot, "job-interrupted")
	os.Mkdir(stale, 0700)
	os.WriteFile(filepath.Join(stale, "video.mp4"), []byte("partial"), 0600)
	os.WriteFile(filepath.Join(downloadRoot, "unrelated"), []byte("keep"), 0600)
	if err := cleanDownloadFiles(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatal("stale job remains")
	}
	if _, err := os.Stat(filepath.Join(downloadRoot, "unrelated")); err != nil {
		t.Fatal("unrelated file removed")
	}
}

func TestDirectVideoDownload(t *testing.T) {
	r := downloadTestSetup(t)
	mediaClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"video/mp4"}}, Body: io.NopCloser(strings.NewReader("video bytes"))}, nil
	})}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/download?url="+url.QueryEscape("https://v.pinimg.com/clip.mp4"), nil))
	if w.Code != 200 || w.Body.String() != "video bytes" || !strings.Contains(w.Header().Get("Content-Disposition"), "pinless-clip.mp4") {
		t.Fatalf("bad download: %d %v", w.Code, w.Header())
	}
	assertNoDownloadFiles(t)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/download?url=http://127.0.0.1/private.m3u8", nil))
	if w.Code != 403 {
		t.Fatal("accepted untrusted video source")
	}
}

func TestImageDownloadsPreserveBytes(t *testing.T) {
	r := downloadTestSetup(t)
	for _, kind := range []string{"image/gif", "image/jpeg", "image/png", "image/webp"} {
		payload := "original media bytes for " + kind
		mediaClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {kind}}, Body: io.NopCloser(strings.NewReader(payload))}, nil
		})}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/download?kind=image&url="+url.QueryEscape("https://i.pinimg.com/originals/picture.gif"), nil))
		if w.Code != 200 || w.Body.String() != payload || w.Header().Get("Content-Type") != kind { t.Fatalf("image bytes changed: %s %d", kind, w.Code) }
		if !strings.Contains(w.Header().Get("Content-Disposition"), "attachment;") { t.Fatal("missing download disposition") }
		if kind == "image/gif" && !strings.Contains(w.Header().Get("Content-Disposition"), ".gif") { t.Fatal("lost GIF extension") }
		assertNoDownloadFiles(t)
	}
}

func TestHLSDownloadAndCleanup(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("FFmpeg integration test")
	}
	r := downloadTestSetup(t)
	fixture := t.TempDir()
	cmd := exec.Command("ffmpeg", "-nostdin", "-v", "error", "-f", "lavfi", "-i", "color=c=blue:s=64x64:r=25", "-f", "lavfi", "-i", "sine=frequency=440", "-t", "2", "-c:v", "libx264", "-c:a", "aac", "-g", "25", "-hls_time", "1", "-hls_list_size", "0", filepath.Join(fixture, "media.m3u8"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("fixture: %v %s", err, out)
	}
	mediaClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		var body []byte
		var err error
		kind := "video/mp2t"
		if req.URL.Path == "/master.m3u8" {
			body = []byte("#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=1000\nunused-low-quality.m3u8\n#EXT-X-STREAM-INF:BANDWIDTH=200000\nmedia.m3u8\n")
		} else {
			body, err = os.ReadFile(filepath.Join(fixture, filepath.Base(req.URL.Path)))
		}
		if strings.HasSuffix(req.URL.Path, ".m3u8") {
			kind = "application/vnd.apple.mpegurl"
		}
		if err != nil {
			return nil, err
		}
		return &http.Response{StatusCode: 200, Request: req, Header: http.Header{"Content-Type": {kind}}, Body: io.NopCloser(bytes.NewReader(body))}, nil
	})}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/download?url="+url.QueryEscape("https://v.pinimg.com/master.m3u8"), nil))
	if w.Code != 200 || !bytes.Contains(w.Body.Bytes()[:min(40, w.Body.Len())], []byte("ftyp")) {
		t.Fatalf("conversion: %d %s", w.Code, w.Body.String())
	}
	assertNoDownloadFiles(t)
	out := filepath.Join(t.TempDir(), "download.mp4")
	os.WriteFile(out, w.Body.Bytes(), 0600)
	probe, err := exec.Command("ffprobe", "-v", "error", "-show_entries", "stream=codec_type:format=duration", "-of", "default=noprint_wrappers=1", out).CombinedOutput()
	if err != nil || !bytes.Contains(probe, []byte("codec_type=video")) || !bytes.Contains(probe, []byte("codec_type=audio")) || !bytes.Contains(probe, []byte("duration=2.")) {
		t.Fatalf("incomplete MP4: %s %v", probe, err)
	}
	// A failed upstream request must not retain a partial output.
	mediaClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) { return nil, io.ErrUnexpectedEOF })}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/download?url="+url.QueryEscape("https://v.pinimg.com/broken.m3u8"), nil))
	if w.Code != 502 {
		t.Fatalf("expected download failure: %d", w.Code)
	}
	assertNoDownloadFiles(t)
}

func TestCancelledDownloadCleanup(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("FFmpeg integration test")
	}
	r := downloadTestSetup(t)
	started := make(chan struct{}, 1)
	mediaClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		started <- struct{}{}
		<-req.Context().Done()
		return nil, req.Context().Err()
	})}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/download?url="+url.QueryEscape("https://v.pinimg.com/slow.m3u8"), nil).WithContext(ctx))
	}()
	select {
	case <-started:
	case <-time.After(10 * time.Second):
		t.Fatal("worker never started")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("cancel did not stop worker")
	}
	assertNoDownloadFiles(t)
}
