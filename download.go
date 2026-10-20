package main

import (
	"context"
	"fmt"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const maxDownloadBytes = 512 * 1024 * 1024

var downloadSlots = make(chan struct{}, 2)
var downloadRoot = filepath.Join(os.TempDir(), "pinless-downloads")
var unsafeFilename = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)
var hlsBandwidth = regexp.MustCompile(`(?:^|[:,])BANDWIDTH=(\d+)`)

// Keep the highest-bandwidth variant and its associated audio declarations.
// Playback still uses the full adaptive playlist; this is only for downloads.
func bestDownloadVariant(playlist string) string {
	lines := strings.Split(playlist, "\n")
	best, bestRate := -1, int64(-1)
	variants := make(map[int]int)
	for i, line := range lines {
		if !strings.HasPrefix(line, "#EXT-X-STREAM-INF:") {
			continue
		}
		for j := i + 1; j < len(lines); j++ {
			if strings.TrimSpace(lines[j]) == "" {
				continue
			}
			if strings.HasPrefix(lines[j], "#") {
				break
			}
			variants[i] = j
			rate := int64(0)
			if match := hlsBandwidth.FindStringSubmatch(line); len(match) > 1 {
				rate, _ = strconv.ParseInt(match[1], 10, 64)
			}
			if rate > bestRate {
				best, bestRate = i, rate
			}
			break
		}
	}
	for i, uri := range variants {
		if i != best {
			lines[i], lines[uri] = "", ""
		}
	}
	return strings.Join(lines, "\n")
}

func downloadFilename(source, ext string) string {
	u, _ := url.Parse(source)
	stem := unsafeFilename.ReplaceAllString(strings.TrimSuffix(path.Base(u.Path), path.Ext(u.Path)), "-")
	if len(stem) > 80 {
		stem = stem[:80]
	}
	if stem == "" || stem == "-" {
		stem = "video"
	}
	return "pinless-" + stem + ext
}

// Only this application's job directories are removed. Run before serving requests.
func cleanDownloadFiles() error {
	if err := os.MkdirAll(downloadRoot, 0700); err != nil {
		return err
	}
	entries, err := os.ReadDir(downloadRoot)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "job-") {
			if err := os.RemoveAll(filepath.Join(downloadRoot, entry.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}

func downloadVideoHandler(c *gin.Context) {
	target := c.Query("url")
	if !isAllowedDomain(target) {
		c.String(403, "This video source is not supported.")
		return
	}
	u, _ := url.Parse(target)
	if c.Query("kind") == "image" {
		proxyMedia(c, false)
		return
	}
	ext := strings.ToLower(path.Ext(u.Path))
	if ext == ".mp4" || ext == ".webm" {
		proxyMedia(c, true)
		return
	}
	if ext != ".m3u8" {
		c.String(400, "This video format is not supported.")
		return
	}
	select {
	case downloadSlots <- struct{}{}:
		defer func() { <-downloadSlots }()
	default:
		c.String(429, "Two videos are already being prepared. Please try again shortly.")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Minute)
	defer cancel()
	downloadFilesMu.RLock()
	defer downloadFilesMu.RUnlock()
	dir, err := os.MkdirTemp(downloadRoot, "job-")
	if err != nil {
		c.String(500, "There is not enough temporary storage to prepare this video.")
		return
	}
	defer os.RemoveAll(dir)

	// FFmpeg can only request playlists and pieces through the existing validated
	// proxy. It never receives an upstream playlist with unchecked URLs.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		c.String(500, "The download could not start. Please try again.")
		return
	}
	base := "http://" + listener.Addr().String()
	proxy := gin.New()
	proxy.GET("/hls", func(p *gin.Context) {
		p.Request = p.Request.WithContext(ctx)
		p.Set("downloadProxyBase", base)
		proxyHLSHandler(p)
	})
	server := &http.Server{Handler: proxy, ReadHeaderTimeout: 10 * time.Second}
	go server.Serve(listener)
	defer server.Close()
	output := filepath.Join(dir, "video.mp4")
	cmd := exec.CommandContext(ctx, "ffmpeg", "-nostdin", "-hide_banner", "-loglevel", "error",
		"-protocol_whitelist", "http,tcp,crypto", "-allowed_extensions", "ALL",
		"-i", base+"/hls?url="+url.QueryEscape(target),
		"-map", "0:v:0", "-map", "0:a:0?", "-c", "copy", "-movflags", "+faststart",
		"-fs", fmt.Sprint(maxDownloadBytes), "-y", output)
	if err := cmd.Run(); err != nil {
		if c.Request.Context().Err() != nil {
			return
		}
		if ctx.Err() != nil {
			c.String(504, "Preparing this video took too long. Please try again.")
			return
		}
		c.String(502, "This video could not be prepared for download. Please try another video or retry later.")
		return
	}
	info, err := os.Stat(output)
	if err != nil || info.Size() == 0 {
		c.String(502, "The video download was empty. Please try again.")
		return
	}
	if info.Size() >= maxDownloadBytes {
		c.String(413, "This video is too large to download here (512 MB limit).")
		return
	}
	c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": downloadFilename(target, ".mp4")}))
	c.Header("Content-Type", "video/mp4")
	c.Header("Cache-Control", "no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.File(output)
}
