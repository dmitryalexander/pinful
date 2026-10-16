package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

// Each link carries the particular wall it came from, keeping tabs independent.
type previousWall struct {
	Pins           []Pin
	Seed, Bookmark string
}

var previousWalls = struct {
	sync.Mutex
	items map[string]previousWall
	order []string
}{items: make(map[string]previousWall)}

func rememberPreviousWall(pins []Pin, seed, bookmark string) string {
	if len(pins) == 0 {
		return ""
	}
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return ""
	}
	token := hex.EncodeToString(bytes[:])
	previousWalls.Lock()
	defer previousWalls.Unlock()
	previousWalls.items[token] = previousWall{append([]Pin(nil), pins...), seed, bookmark}
	previousWalls.order = append(previousWalls.order, token)
	if len(previousWalls.order) > 256 {
		delete(previousWalls.items, previousWalls.order[0])
		previousWalls.order = previousWalls.order[1:]
	}
	return token
}
func recalledPreviousWall(token string) (previousWall, bool) {
	previousWalls.Lock()
	defer previousWalls.Unlock()
	wall, ok := previousWalls.items[token]
	return wall, ok
}
func wallImageSignature(raw string) string {
	if u, err := url.Parse(raw); err == nil && u.Path == "/image" {
		raw = u.Query().Get("url")
	}
	return publicImageSignature(raw)
}
func previousRecommendations(pins []Pin, selected Pin, id string) []Pin {
	result := make([]Pin, 0, len(pins))
	seen := make(map[string]bool)
	signature := wallImageSignature(selected.ImageURL)
	// Pin media commonly uses the local /image proxy.
	for _, pin := range pins {
		key := wallImageSignature(pin.ImageURL)
		if key == "" {
			key = pin.ImageURL
		}
		if key == "" {
			key = pin.ID
		}
		if pin.ID == id || (selected.ImageURL != "" && pin.ImageURL == selected.ImageURL) || (signature != "" && wallImageSignature(pin.ImageURL) == signature) || seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, pin)
	}
	return result
}

func searchWallPins(results []searchResult) []Pin {
	pins := make([]Pin, 0, len(results))
	for _, result := range results {
		pin, ok := recalledPinPreview(result.ID)
		if !ok {
			pin = Pin{ID: result.ID, ImageURL: result.Image, VideoURL: result.VideoURL, ImageWidth: result.ImageWidth, ImageHeight: result.ImageHeight}
		}
		pins = append(pins, pin)
	}
	return pins
}
func fetchPreviousWallPage(c *gin.Context, previous previousWall, cursor string, csrf *string) ([]Pin, string) {
	if !strings.HasPrefix(previous.Seed, "search:") {
		return fetchRelatedPins(previous.Seed, *csrf, cursor)
	}
	tags := parseSearchTags(strings.TrimPrefix(previous.Seed, "search:"))
	var pins []Pin
	for page := 0; page < 4; page++ {
		results, next, failure := fetchSearchPage(c, tags.UpstreamQuery(), tags.Scope(), cursor, csrf)
		if failure != nil {
			return pins, cursor
		}
		filtered := make([]searchResult, 0, len(results))
		for _, result := range results {
			if tags.Keep(result.Kind, result.ImageWidth, result.ImageHeight) {
				filtered = append(filtered, result)
			}
		}
		pins = append(pins, searchWallPins(filtered)...)
		cursor = next
		if len(pins) > 0 || cursor == "" {
			break
		}
	}
	return pins, cursor
}

// Saved walls live in the browser; pass only their public media to navigation.
func savedWallContextHandler(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	if !localPinterestHost(c.Request.Host) || c.GetHeader("Origin") != "http://"+c.Request.Host || !strings.HasPrefix(c.GetHeader("Content-Type"), "application/json") {
		c.Status(http.StatusForbidden)
		return
	}
	var body struct {
		Pins []struct {
			ID     string `json:"id"`
			Image  string `json:"image"`
			Video  string `json:"video"`
			Title  string `json:"title"`
			Width  int    `json:"width"`
			Height int    `json:"height"`
		} `json:"pins"`
	}
	if json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, 256<<10)).Decode(&body) != nil || len(body.Pins) > 80 {
		c.Status(http.StatusBadRequest)
		return
	}
	safe := func(raw string) string {
		if raw == "" || len(raw) > 4096 {
			return ""
		}
		u, err := url.Parse(raw)
		if err != nil {
			return ""
		}
		if u.Host == "" && (u.Path == "/image" || u.Path == "/video" || u.Path == "/hls") {
			u, err = url.Parse(u.Query().Get("url"))
			if err != nil {
				return ""
			}
		}
		if u.Scheme != "https" || u.User != nil || u.Port() != "" || !(u.Hostname() == "pinimg.com" || strings.HasSuffix(u.Hostname(), ".pinimg.com")) {
			return ""
		}
		return raw
	}
	pins := make([]Pin, 0, len(body.Pins))
	for _, item := range body.Pins {
		if !numericPinID.MatchString(item.ID) && !opaqueStatusID.MatchString(item.ID) {
			continue
		}
		image, video := safe(item.Image), safe(item.Video)
		if image == "" && video == "" {
			continue
		}
		if len(item.Title) > 500 {
			item.Title = item.Title[:500]
		}
		if item.Width < 1 || item.Width > 20000 {
			item.Width = 500
		}
		if item.Height < 1 || item.Height > 20000 {
			item.Height = 500
		}
		pin := Pin{ID: item.ID, ImageURL: image, VideoURL: video, Title: item.Title, ImageWidth: item.Width, ImageHeight: item.Height}
		rememberPinPreview(pin)
		pins = append(pins, pin)
	}
	token := rememberPreviousWall(pins, "", "")
	if token == "" {
		c.Status(http.StatusBadRequest)
		return
	}
	c.JSON(http.StatusOK, gin.H{"context": token})
}
