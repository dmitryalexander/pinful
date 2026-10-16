package main

import (
	"encoding/json"
	"sync"
	"time"
)

func previewTitle(raw json.RawMessage) string {
	var title string
	if json.Unmarshal(raw, &title) == nil {
		return title
	}
	var structured struct {
		Format string `json:"format"`
	}
	_ = json.Unmarshal(raw, &structured)
	return structured.Format
}

// Search can return encrypted e1_ IDs that Pinterest's detail API rejects.
// Keep the public media we actually received instead of discarding it on click.
// This bounded, short-lived cache is not user storage or a replacement for details.
var pinPreviews = struct {
	sync.Mutex
	items map[string]pinPreview
}{items: make(map[string]pinPreview)}

type pinPreview struct {
	pin  Pin
	seen time.Time
}

func rememberPinPreview(pin Pin) {
	if pin.ID == "" || (pin.ImageURL == "" && pin.VideoURL == "") {
		return
	}
	pinPreviews.Lock()
	defer pinPreviews.Unlock()
	if len(pinPreviews.items) >= 4096 {
		var oldest string
		var age time.Time
		for id, item := range pinPreviews.items {
			if oldest == "" || item.seen.Before(age) {
				oldest, age = id, item.seen
			}
		}
		delete(pinPreviews.items, oldest)
	}
	pinPreviews.items[pin.ID] = pinPreview{pin, time.Now()}
}

func recalledPinPreview(id string) (Pin, bool) {
	pinPreviews.Lock()
	defer pinPreviews.Unlock()
	item, ok := pinPreviews.items[id]
	if !ok || time.Since(item.seen) > time.Hour {
		delete(pinPreviews.items, id)
		return Pin{}, false
	}
	return item.pin, true
}
