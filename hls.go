package main

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
)

var hlsURIAttribute = regexp.MustCompile(`\bURI="([^"]*)"`)

// Resolve every playlist reference against the final upstream URL. Nested
// playlists, audio, initialization segments, and keys stay behind this proxy.
func rewriteHLS(body string, base *url.URL) (string, error) {
	if !strings.HasPrefix(strings.TrimSpace(body), "#EXTM3U") {
		return "", fmt.Errorf("invalid HLS playlist")
	}
	resolve := func(value string) (string, error) {
		u, err := url.Parse(value)
		if err != nil || value == "" {
			return "", fmt.Errorf("invalid HLS reference")
		}
		target := base.ResolveReference(u).String()
		if !isAllowedDomain(target) {
			return "", fmt.Errorf("disallowed HLS reference")
		}
		return "/hls?url=" + url.QueryEscape(target), nil
	}
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "#") {
			rewritten, err := resolve(line)
			if err != nil {
				return "", err
			}
			lines[i] = rewritten
			continue
		}
		var refErr error
		lines[i] = hlsURIAttribute.ReplaceAllStringFunc(line, func(match string) string {
			value := hlsURIAttribute.FindStringSubmatch(match)[1]
			rewritten, err := resolve(value)
			if err != nil {
				refErr = err
				return match
			}
			return `URI="` + rewritten + `"`
		})
		if refErr != nil {
			return "", refErr
		}
	}
	return strings.Join(lines, "\n"), nil
}

func proxyHLSHandler(c *gin.Context) {
	target := c.Query("url")
	if !isAllowedDomain(target) {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}
	u, _ := url.Parse(target)
	isPlaylist := strings.EqualFold(path.Ext(u.Path), ".m3u8")
	method := c.Request.Method
	// A playlist HEAD needs the rewritten representation's length, not the origin's.
	if isPlaylist {
		method = http.MethodGet
	}
	req, err := http.NewRequestWithContext(c.Request.Context(), method, target, nil)
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	if !isPlaylist {
		for _, key := range []string{"Range", "If-Range"} {
			if value := c.GetHeader(key); value != "" {
				req.Header.Set(key, value)
			}
		}
	}
	resp, err := mediaClient.Do(req)
	if err != nil {
		c.AbortWithStatus(http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusRequestedRangeNotSatisfiable {
		c.Header("Content-Range", resp.Header.Get("Content-Range"))
		c.AbortWithStatus(resp.StatusCode)
		return
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		c.AbortWithStatus(http.StatusBadGateway)
		return
	}
	contentType, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	isPlaylist = isPlaylist || contentType == "application/vnd.apple.mpegurl" || contentType == "application/x-mpegurl"
	if isPlaylist {
		const maxPlaylist = 2 * 1024 * 1024
		body, err := io.ReadAll(io.LimitReader(resp.Body, maxPlaylist+1))
		if err != nil || len(body) > maxPlaylist {
			c.AbortWithStatus(http.StatusBadGateway)
			return
		}
		base := u
		if resp.Request != nil {
			base = resp.Request.URL
		}
		rewritten, err := rewriteHLS(string(body), base)
		if err != nil {
			c.AbortWithStatus(http.StatusBadGateway)
			return
		}
		// The download worker uses a private loopback proxy for every HLS resource.
		if prefix := c.GetString("downloadProxyBase"); prefix != "" {
			rewritten = bestDownloadVariant(rewritten)
			rewritten = strings.ReplaceAll(rewritten, "/hls?url=", prefix+"/hls?url=")
		}
		c.Header("Cache-Control", "no-cache")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("Content-Length", fmt.Sprint(len(rewritten)))
		if c.Request.Method == http.MethodHead {
			c.Header("Content-Type", "application/vnd.apple.mpegurl")
			c.Status(http.StatusOK)
			return
		}
		c.Data(http.StatusOK, "application/vnd.apple.mpegurl", []byte(rewritten))
		return
	}
	// Restrict non-playlist responses to segment, audio, subtitle and key formats.
	switch contentType {
	case "video/mp2t", "video/mp4", "audio/mp4", "audio/aac", "audio/mpeg", "application/octet-stream", "binary/octet-stream", "text/vtt":
	default:
		c.AbortWithStatus(http.StatusBadGateway)
		return
	}
	c.Header("Content-Type", contentType)
	c.Header("X-Content-Type-Options", "nosniff")
	for _, key := range []string{"Content-Length", "Content-Range", "Accept-Ranges", "Cache-Control", "ETag", "Last-Modified"} {
		if value := resp.Header.Get(key); value != "" {
			c.Header(key, value)
		}
	}
	c.Status(resp.StatusCode)
	if c.Request.Method != http.MethodHead {
		_, _ = io.Copy(c.Writer, resp.Body)
	}
}
