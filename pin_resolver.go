package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Public source evidence travels with server previews, never with saved-list edits.
type pinOrigin struct {
	PinJoin struct {
		CanonicalPin struct {
			ID string `json:"id"`
		} `json:"canonical_pin"`
		SEOCanonicalURL string `json:"seo_canonical_url"`
	} `json:"pin_join"`
	ImageSignature string `json:"image_signature"`
	CreatedAt      string `json:"created_at"`
	SourceBoard    struct {
		ID  string `json:"id"`
		URL string `json:"url"`
	} `json:"board"`
}

var resolverPinFields = []string{"pin.pin_join()", "pinjoin.canonical_pin", "pinjoin.seo_canonical_url"}

type resolverEntry[T any] struct {
	value   T
	expires time.Time
}
type resolverCache[T any] struct {
	sync.Mutex
	items   map[string]resolverEntry[T]
	flights map[string]chan struct{}
	limit   int
}

func newResolverCache[T any](limit int) *resolverCache[T] {
	return &resolverCache[T]{items: make(map[string]resolverEntry[T]), flights: make(map[string]chan struct{}), limit: limit}
}

// Share concurrent work, expire failures quickly, and bound both data and jobs.
// Loaders never recurse into this cache. Busy requests retain ordinary recovery.
func (c *resolverCache[T]) load(key string, loader func() (T, bool)) T {
	var zero T
	c.Lock()
	if entry, ok := c.items[key]; ok && time.Now().Before(entry.expires) {
		c.Unlock()
		return entry.value
	}
	if done, ok := c.flights[key]; ok {
		c.Unlock()
		select {
		case <-done:
		case <-time.After(15 * time.Second):
			return zero
		}
		c.Lock()
		entry := c.items[key]
		c.Unlock()
		return entry.value
	}
	if len(c.flights) >= 8 {
		c.Unlock()
		return zero
	}
	done := make(chan struct{})
	c.flights[key] = done
	c.Unlock()
	value, success := loader()
	ttl := 2 * time.Minute
	if success {
		ttl = time.Hour
	}
	c.Lock()
	if len(c.items) >= c.limit {
		var oldest string
		var expiry time.Time
		for k, entry := range c.items {
			if oldest == "" || entry.expires.Before(expiry) {
				oldest, expiry = k, entry.expires
			}
		}
		delete(c.items, oldest)
	}
	c.items[key] = resolverEntry[T]{value, time.Now().Add(ttl)}
	delete(c.flights, key)
	close(done)
	c.Unlock()
	return value
}

var boardEmbeds = newResolverCache[[]embedPin](256)
var resolvedOpaquePins = newResolverCache[string](4096)
var numericPinID = regexp.MustCompile(`^[0-9]{1,24}$`)
var imageSignature = regexp.MustCompile(`^[a-f0-9]{32}$`)
var canonicalPinPath = regexp.MustCompile(`^/pin/(?:[^/?#]+--)?([0-9]{1,24})/$`)

type embedPin struct {
	ID     string          `json:"id"`
	Images PinterestImages `json:"images"`
}

// Accept only a public two-component board path; the destination is fixed.
func publicBoardPath(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.IsAbs() || u.Host != "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return ""
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) != 2 {
		return ""
	}
	for _, part := range parts {
		if part == "" || part == "." || part == ".." || strings.ContainsAny(part, "\\\x00\r\n") {
			return ""
		}
	}
	return "/" + url.PathEscape(parts[0]) + "/" + url.PathEscape(parts[1]) + "/"
}

func publicImageSignature(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host != "i.pinimg.com" || u.User != nil {
		return ""
	}
	name := path.Base(u.Path)
	sig := strings.TrimSuffix(name, path.Ext(name))
	if !imageSignature.MatchString(sig) {
		return ""
	}
	return sig
}

func fetchBoardEmbed(boardPath string) []embedPin {
	req, _ := http.NewRequest(http.MethodGet, "https://widgets.pinterest.com/v3/pidgets/boards"+boardPath+"pins/", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Accept", "application/json")
	// Always anonymous: do not use the connected-session client for widget hosts.
	client := &http.Client{Timeout: 4 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	var result struct {
		Status string `json:"status"`
		Data   struct {
			Board struct {
				ID string `json:"id"`
			} `json:"board"`
			Pins []embedPin `json:"pins"`
		} `json:"data"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&result) != nil || result.Status != "success" || len(result.Data.Pins) > 50 {
		return nil
	}
	return result.Data.Pins
}

func resolveOpaquePin(id string) string {
	if !strings.HasPrefix(id, "e1_") {
		return ""
	}
	resolvedOpaquePins.Lock()
	entry, cached := resolvedOpaquePins.items[id]
	resolvedOpaquePins.Unlock()
	if cached && time.Now().Before(entry.expires) {
		return entry.value
	}
	preview, ok := recalledPinPreview(id)
	boardPath := publicBoardPath(preview.SourceBoard.URL)
	if !ok || boardPath == "" || !numericPinID.MatchString(preview.SourceBoard.ID) || preview.CreatedAt == "" || !imageSignature.MatchString(preview.ImageSignature) {
		return ""
	}
	return resolvedOpaquePins.load(id, func() (string, bool) {
		// The SEO URL can name a different canonical/near-duplicate post. Only
		// accept it automatically when Pinterest identifies it as this exact pin.
		if preview.PinJoin.CanonicalPin.ID == id {
			match := canonicalPinPath.FindStringSubmatch(preview.PinJoin.SEOCanonicalURL)
			if len(match) == 2 && verifiedOriginalPin(preview, match[1]) {
				return match[1], true
			}
		}
		pins := boardEmbeds.load(boardPath, func() ([]embedPin, bool) { pins := fetchBoardEmbed(boardPath); return pins, pins != nil })
		candidate := ""
		for _, pin := range pins {
			image := extractBestImageInfo(pin.Images)
			if image == nil || !numericPinID.MatchString(pin.ID) || publicImageSignature(image.URL) != preview.ImageSignature {
				continue
			}
			if candidate != "" && candidate != pin.ID {
				return "", false
			}
			candidate = pin.ID
		}
		if candidate == "" {
			return "", false
		}
		if !verifiedOriginalPin(preview, candidate) {
			return "", false
		}
		return candidate, true
	})
}

func verifiedOriginalPin(preview Pin, candidate string) bool {
	client := clientWithPinterestCredentials(pinterestCredentials{})
	client.Timeout = 4 * time.Second
	detail, status := fetchPinDetailsWithClient(candidate, "", client)
	image, _ := url.Parse(detail.ImageURL)
	return status == http.StatusOK && detail.UpstreamID == candidate && detail.ImageSignature == preview.ImageSignature && detail.SourceBoard.ID == preview.SourceBoard.ID && detail.CreatedAt == preview.CreatedAt && image != nil && publicImageSignature(image.Query().Get("url")) == preview.ImageSignature
}
