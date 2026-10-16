package main

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type PinterestVideo struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type PinterestVideoList struct {
	VideoList map[string]PinterestVideo `json:"video_list"`
}

type PinterestMedia struct {
	Embed struct {
		Type string `json:"type"`
		Src  string `json:"src"`
	} `json:"embed"`
	Videos       PinterestVideoList `json:"videos"`
	StoryPinData struct {
		Pages []struct {
			Blocks []struct {
				Video PinterestVideoList `json:"video"`
			} `json:"blocks"`
		} `json:"pages"`
	} `json:"story_pin_data"`
}

func bestPinImage(images PinterestImages, media PinterestMedia) *PinterestImage {
	best := extractBestImageInfo(images)
	// GIF embeds are the animation; resized image variants may be still JPEGs.
	if strings.EqualFold(media.Embed.Type, "gif") && isAllowedDomain(media.Embed.Src) {
		animated := PinterestImage{URL: media.Embed.Src}
		if best != nil {
			animated.Width, animated.Height = best.Width, best.Height
		}
		return &animated
	}
	return best
}

func videoProxyURL(media PinterestMedia) string {
	var candidates []PinterestVideo
	for _, v := range media.Videos.VideoList {
		candidates = append(candidates, v)
	}
	for _, page := range media.StoryPinData.Pages {
		for _, block := range page.Blocks {
			for _, v := range block.Video.VideoList {
				candidates = append(candidates, v)
			}
		}
		// A multi-page story should start with its first playable page.
		if len(candidates) > 0 {
			break
		}
	}
	if media.Embed.Type == "video" {
		candidates = append(candidates, PinterestVideo{URL: media.Embed.Src})
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]
		if a.Width*a.Height != b.Width*b.Height {
			return a.Width*a.Height > b.Width*b.Height
		}
		return a.URL < b.URL
	})
	for _, v := range candidates {
		u, err := url.Parse(v.URL)
		if err != nil || !isAllowedDomain(v.URL) {
			continue
		}
		ext := strings.ToLower(path.Ext(u.Path))
		if ext == ".mp4" || ext == ".webm" {
			return "/video?url=" + url.QueryEscape(v.URL)
		}
	}
	for _, v := range candidates {
		u, err := url.Parse(v.URL)
		if err == nil && isAllowedDomain(v.URL) && strings.EqualFold(path.Ext(u.Path), ".m3u8") {
			return "/hls?url=" + url.QueryEscape(v.URL)
		}
	}
	return ""
}

var mediaClient = &http.Client{
	Timeout: 2 * time.Minute,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 || !isAllowedDomain(req.URL.String()) {
			return fmt.Errorf("invalid media redirect")
		}
		return nil
	},
}

func proxyVideoHandler(c *gin.Context) { proxyMedia(c, true) }

func proxyMedia(c *gin.Context, video bool) {
	mediaURL := c.Query("url")
	if !isAllowedDomain(mediaURL) {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}
	req, err := http.NewRequestWithContext(c.Request.Context(), c.Request.Method, mediaURL, nil)
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	if video {
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
	if video && resp.StatusCode == http.StatusRequestedRangeNotSatisfiable {
		c.Header("Content-Range", resp.Header.Get("Content-Range"))
		c.Status(resp.StatusCode)
		return
	}
	if resp.StatusCode != http.StatusOK && !(video && resp.StatusCode == http.StatusPartialContent) {
		c.AbortWithStatus(http.StatusBadGateway)
		return
	}
	contentType, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if contentType == "" || contentType == "application/octet-stream" {
		parsed, _ := url.Parse(mediaURL)
		switch strings.ToLower(path.Ext(parsed.Path)) {
		case ".gif":
			contentType = "image/gif"
		case ".jpg", ".jpeg":
			contentType = "image/jpeg"
		case ".png":
			contentType = "image/png"
		case ".webp":
			contentType = "image/webp"
		case ".mp4":
			contentType = "video/mp4"
		case ".webm":
			contentType = "video/webm"
		}
	}
	valid := strings.HasPrefix(contentType, "image/") && contentType != "image/svg+xml"
	if video {
		valid = contentType == "video/mp4" || contentType == "video/webm"
	}
	if !valid {
		c.AbortWithStatus(http.StatusBadGateway)
		return
	}
	c.Header("Content-Type", contentType)
	if c.FullPath() == "/download" {
		ext := ".mp4"
		if contentType == "video/webm" {
			ext = ".webm"
		}
		if !video {
			ext = ".img"
			if extensions, err := mime.ExtensionsByType(contentType); err == nil && len(extensions) > 0 { ext = extensions[0] }
			if contentType == "image/jpeg" { ext = ".jpg" }
		}
		c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": downloadFilename(mediaURL, ext)}))
	}
	c.Header("X-Content-Type-Options", "nosniff")
	for _, key := range []string{"Content-Length", "Cache-Control", "ETag", "Last-Modified", "Accept-Ranges", "Content-Range"} {
		if value := resp.Header.Get(key); value != "" {
			c.Header(key, value)
		}
	}
	c.Status(resp.StatusCode)
	if c.Request.Method != http.MethodHead {
		_, _ = io.Copy(c.Writer, resp.Body)
	}
}
