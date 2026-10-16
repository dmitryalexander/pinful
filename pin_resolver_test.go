package main

import (
	"encoding/json"
	"html/template"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

const resolverSig = "16efcf4348bb1d011bae594d7949ee84"

func resolverFixture(t *testing.T) Pin {
	t.Helper()
	oldTransport, oldBoards, oldMappings := http.DefaultTransport, boardEmbeds, resolvedOpaquePins
	boardEmbeds = newResolverCache[[]embedPin](256)
	resolvedOpaquePins = newResolverCache[string](4096)
	t.Cleanup(func() { http.DefaultTransport, boardEmbeds, resolvedOpaquePins = oldTransport, oldBoards, oldMappings })
	pin := Pin{ID: "e1_resolver", ImageURL: "/image?url=fixture"}
	pin.ImageSignature = resolverSig
	pin.CreatedAt = "Tue, 19 Jun 2018 07:15:41 +0000"
	pin.SourceBoard.ID, pin.SourceBoard.URL = "12345", "/artist/art/"
	rememberPinPreview(pin)
	return pin
}

func resolverReply(status int, value any) *http.Response {
	body, _ := json.Marshal(value)
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(string(body)))}
}

func resolverDetail(pin Pin) map[string]any {
	return map[string]any{"id": "789", "image_signature": pin.ImageSignature, "created_at": pin.CreatedAt,
		"board":  map[string]any{"id": pin.SourceBoard.ID, "url": pin.SourceBoard.URL},
		"images": map[string]any{"orig": map[string]any{"url": "https://i.pinimg.com/originals/" + resolverSig + ".jpg", "width": 600, "height": 800}}}
}

func resolverWidget(ids ...string) map[string]any {
	pins := []any{}
	for _, id := range ids {
		pins = append(pins, map[string]any{"id": id, "images": map[string]any{"236x": map[string]any{"url": "https://i.pinimg.com/236x/" + resolverSig + ".jpg"}}})
	}
	return map[string]any{"status": "success", "data": map[string]any{"pins": pins}}
}

func TestAutomaticResolverRedirectAndPagination(t *testing.T) {
	pin := resolverFixture(t)
	widgets, details, pages := 0, 0, 0
	http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == "widgets.pinterest.com" {
			widgets++
			if r.Header.Get("Cookie") != "" || r.Header.Get("X-CSRFToken") != "" {
				t.Fatal("widget request must stay anonymous")
			}
			return resolverReply(200, resolverWidget("789")), nil
		}
		var data struct {
			Options struct {
				ID        string   `json:"id"`
				PinID     string   `json:"pin_id"`
				Bookmarks []string `json:"bookmarks"`
			} `json:"options"`
		}
		_ = json.Unmarshal([]byte(r.URL.Query().Get("data")), &data)
		if strings.Contains(r.URL.Path, "PinResource") {
			details++
			if data.Options.ID != "789" {
				t.Fatal("unresolved ID sent for verified details")
			}
			return resolverReply(200, map[string]any{"resource_response": map[string]any{"data": resolverDetail(pin)}}), nil
		}
		if data.Options.PinID != "789" {
			t.Fatal("pagination changed recommendation seed")
		}
		if pages == 1 && (len(data.Options.Bookmarks) != 1 || data.Options.Bookmarks[0] != "page+two/==") {
			t.Fatal("cursor lost")
		}
		pages++
		return resolverReply(200, map[string]any{"resource_response": map[string]any{"data": []any{map[string]any{"id": "e1_child", "type": "pin", "images": resolverDetail(pin)["images"], "image_signature": resolverSig, "created_at": pin.CreatedAt, "board": resolverDetail(pin)["board"]}}, "bookmark": "page+two/=="}}), nil
	})
	router := gin.New()
	router.SetHTMLTemplate(template.Must(template.ParseFS(templatesFS, "templates/*")))
	router.GET("/pin/:id", pinHandler)
	query := "q=art%20work&from=123&next&cursor=page%2Btwo%2F%3D%3D"
	out := httptest.NewRecorder()
	router.ServeHTTP(out, httptest.NewRequest("GET", "/pin/e1_resolver?"+query, nil))
	if out.Code != 302 || out.Header().Get("Location") != "/pin/789?"+query || out.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("bad canonical redirect: %d %s", out.Code, out.Header().Get("Location"))
	}
	// Canonical pagination stays functional after all opaque/board evidence expires.
	pinPreviews.Lock()
	delete(pinPreviews.items, pin.ID)
	pinPreviews.Unlock()
	if resolveOpaquePin(pin.ID) != "789" || widgets != 1 {
		t.Fatal("verified mapping did not survive preview expiry")
	}
	for _, target := range []string{"/pin/789", "/pin/789?next&cursor=page%2Btwo%2F%3D%3D"} {
		req := httptest.NewRequest("GET", target, nil)
		req.AddCookie(&http.Cookie{Name: "csrftoken", Value: "fixture"})
		out = httptest.NewRecorder()
		router.ServeHTTP(out, req)
		if out.Code != 200 || strings.Contains(out.Body.String(), "Could not load related pins") {
			t.Fatal("resolved wall failed")
		}
	}
	child, ok := recalledPinPreview("e1_child")
	if !ok || child.SourceBoard.ID != "12345" || child.CreatedAt != pin.CreatedAt || child.ImageSignature != resolverSig {
		t.Fatal("next-click evidence lost")
	}
	if widgets != 1 || details != 3 || pages != 2 {
		t.Fatalf("unexpected requests %d %d %d", widgets, details, pages)
	}
}

func TestResolverRejectsUnverifiedMatches(t *testing.T) {
	for _, mode := range []string{"no match", "ambiguous", "wrong board", "wrong date", "wrong image", "wrong ID", "detail failure", "bad widget", "widget redirect"} {
		t.Run(mode, func(t *testing.T) {
			pin := resolverFixture(t)
			http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.URL.Host == "widgets.pinterest.com" {
					switch mode {
					case "no match":
						return resolverReply(200, resolverWidget()), nil
					case "ambiguous":
						return resolverReply(200, resolverWidget("789", "790")), nil
					case "bad widget":
						return resolverReply(200, map[string]any{"status": "failure", "data": resolverWidget("789")["data"]}), nil
					case "widget redirect":
						resp := resolverReply(302, nil)
						resp.Header.Set("Location", "https://example.com/")
						return resp, nil
					}
					return resolverReply(200, resolverWidget("789")), nil
				}
				detail := resolverDetail(pin)
				switch mode {
				case "wrong board":
					detail["board"] = map[string]any{"id": "999"}
				case "wrong date":
					detail["created_at"] = "different"
				case "wrong image":
					detail["image_signature"] = "ffffffffffffffffffffffffffffffff"
				case "wrong ID":
					detail["id"] = "111"
				case "detail failure":
					return resolverReply(503, nil), nil
				}
				return resolverReply(200, map[string]any{"resource_response": map[string]any{"data": detail}}), nil
			})
			if got := resolveOpaquePin(pin.ID); got != "" {
				t.Fatalf("unverified mapping accepted: %s", got)
			}
		})
	}
}

func TestResolverRequiresCompleteSourceEvidence(t *testing.T) {
	for _, mode := range []string{"missing time", "missing signature", "missing board", "external board", "encoded traversal"} {
		t.Run(mode, func(t *testing.T) {
			pin := resolverFixture(t)
			switch mode {
			case "missing time":
				pin.CreatedAt = ""
			case "missing signature":
				pin.ImageSignature = ""
			case "missing board":
				pin.SourceBoard.ID = ""
			case "external board":
				pin.SourceBoard.URL = "https://example.com/user/board/"
			case "encoded traversal":
				pin.SourceBoard.URL = "/user/%2e%2e/"
			}
			rememberPinPreview(pin)
			http.DefaultTransport = roundTripFunc(func(*http.Request) (*http.Response, error) {
				t.Fatal("incomplete evidence must not fetch")
				return nil, nil
			})
			if resolveOpaquePin(pin.ID) != "" {
				t.Fatal("unexpected mapping")
			}
		})
	}
}

func TestCanonicalOriginalBypassesEmbedWindow(t *testing.T) {
	for _, mode := range []string{"original", "different canonical pin", "external URL", "changed time", "changed image bytes"} {
		t.Run(mode, func(t *testing.T) {
			pin := resolverFixture(t)
			pin.PinJoin.CanonicalPin.ID = pin.ID
			pin.PinJoin.SEOCanonicalURL = "/pin/artwork--789/"
			if mode == "different canonical pin" {
				pin.PinJoin.CanonicalPin.ID = "e1_other"
			}
			if mode == "external URL" {
				pin.PinJoin.SEOCanonicalURL = "https://example.com/pin/789/"
			}
			rememberPinPreview(pin)
			widgets, details := 0, 0
			http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.URL.Host == "widgets.pinterest.com" {
					widgets++
					return resolverReply(200, resolverWidget()), nil
				}
				details++
				if r.Header.Get("Cookie") != "" {
					t.Fatal("canonical verification must be anonymous")
				}
				detail := resolverDetail(pin)
				if mode == "changed time" {
					detail["created_at"] = "different"
				}
				if mode == "changed image bytes" {
					detail["images"] = map[string]any{"orig": map[string]any{"url": "https://i.pinimg.com/originals/ffffffffffffffffffffffffffffffff.jpg"}}
				}
				return resolverReply(200, map[string]any{"resource_response": map[string]any{"data": detail}}), nil
			})
			got := resolveOpaquePin(pin.ID)
			if mode == "original" {
				if got != "789" || widgets != 0 || details != 1 {
					t.Fatal("original canonical ID should resolve without board embed")
				}
			} else if got != "" {
				t.Fatal("canonical substitution or metadata mismatch accepted")
			}
			if (mode == "different canonical pin" || mode == "external URL") && details != 0 {
				t.Fatal("untrusted candidate fetched")
			}
		})
	}
}

func TestResolverCacheDeduplicatesBoundsAndExpires(t *testing.T) {
	cache := newResolverCache[string](2)
	var calls atomic.Int32
	start := make(chan struct{})
	release := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if cache.load("same", func() (string, bool) { calls.Add(1); <-release; return "789", true }) != "789" {
				t.Error("concurrent result lost")
			}
		}()
	}
	close(start)
	for calls.Load() == 0 {
		time.Sleep(time.Millisecond)
	}
	close(release)
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatal("duplicate network work")
	}
	cache.items["same"] = resolverEntry[string]{"stale", time.Now().Add(-time.Second)}
	if cache.load("same", func() (string, bool) { return "fresh", true }) != "fresh" {
		t.Fatal("expired cache reused")
	}
	cache.load("two", func() (string, bool) { return "", false })
	cache.load("three", func() (string, bool) { return "", false })
	if len(cache.items) != 2 {
		t.Fatal("cache unbounded")
	}
}
