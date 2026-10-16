package main

// Search tags (October 14, 2026): #words in the search box choose what a search shows, like
// ScratchPad's file-type tags. Tags ignore capitalization and can be combined: tags from the
// same group are alternatives ("#gifs #videos" shows GIFs or videos), and each group narrows
// the others ("#videos #tall" shows tall videos). Unknown #words stay part of the search text,
// and a quoted tag ("#gifs") is searched literally.

import (
	"strings"
)

type searchTagGroup struct {
	Name  string      `json:"name"`
	Help  string      `json:"help"`
	Tags  []searchTag `json:"tags"`
	Media bool        `json:"-"`
}

type searchTag struct {
	Name    string   `json:"name"`
	Aliases []string `json:"aliases"`
	Help    string   `json:"help"`
}

var searchTagGroups = []searchTagGroup{
	{Name: "What to show", Help: "Pick one or more. Several of these show any of them.", Media: true, Tags: []searchTag{
		{Name: "videos", Aliases: []string{"video", "vid", "vids", "clip", "clips"}, Help: "Video pins only. Pinterest's video search is used when this is the only kind."},
		{Name: "gifs", Aliases: []string{"gif", "animated", "animation", "animations"}, Help: "Animated GIFs only."},
		{Name: "images", Aliases: []string{"image", "img", "imgs", "pic", "pics", "picture", "pictures", "photo", "photos", "still", "stills"}, Help: "Still pictures only: no videos or GIFs."},
	}},
	{Name: "Shape", Help: "Pick one or more. Narrows whatever else you picked.", Tags: []searchTag{
		{Name: "tall", Aliases: []string{"vertical", "portrait"}, Help: "Taller than wide."},
		{Name: "wide", Aliases: []string{"horizontal", "landscape"}, Help: "Wider than tall."},
		{Name: "square", Aliases: []string{"squares"}, Help: "About as wide as tall."},
	}},
}

// searchTagLookup maps every name and alias (lower case) to its canonical tag name.
var searchTagLookup = func() map[string]string {
	lookup := map[string]string{}
	for _, group := range searchTagGroups {
		for _, tag := range group.Tags {
			lookup[tag.Name] = tag.Name
			for _, alias := range tag.Aliases {
				lookup[alias] = tag.Name
			}
		}
	}
	return lookup
}()

type searchTags struct {
	Text  string          // the search words, without the recognized tags
	Media map[string]bool // videos, gifs, images
	Shape map[string]bool // tall, wide, square
	Names []string        // recognized tags in the order typed, canonical names
}

func parseSearchTags(query string) searchTags {
	tags := searchTags{Media: map[string]bool{}, Shape: map[string]bool{}}
	var words []string
	for _, word := range strings.Fields(query) {
		if strings.HasPrefix(word, "#") && len(word) > 1 {
			if name, ok := searchTagLookup[strings.ToLower(strings.TrimRight(word[1:], ",;"))]; ok {
				if name == "tall" || name == "wide" || name == "square" {
					tags.Shape[name] = true
				} else {
					tags.Media[name] = true
				}
				seen := false
				for _, existing := range tags.Names {
					seen = seen || existing == name
				}
				if !seen {
					tags.Names = append(tags.Names, name)
				}
				continue
			}
		}
		// A quoted tag ("#gifs") is ordinary search text; Pinterest gets it without quotes.
		if len(word) > 2 && strings.HasPrefix(word, "\"#") && strings.HasSuffix(word, "\"") {
			word = strings.Trim(word, "\"")
		}
		words = append(words, word)
	}
	tags.Text = strings.Join(words, " ")
	return tags
}

// Filtering reports whether results are narrowed after Pinterest returns them.
func (t searchTags) Filtering() bool {
	return (len(t.Media) > 0 && len(t.Media) < 3) || (len(t.Shape) > 0 && len(t.Shape) < 3)
}

// Scope is Pinterest's own video search, used when videos are the only kind asked for.
func (t searchTags) Scope() string {
	if len(t.Media) == 1 && t.Media["videos"] {
		return "videos"
	}
	return ""
}

// UpstreamQuery is what Pinterest is asked for. GIFs are rare in ordinary results, so a GIF
// search also asks Pinterest for "gif". A search of tags alone uses the tag words.
func (t searchTags) UpstreamQuery() string {
	text := strings.TrimSpace(t.Text)
	if t.Media["gifs"] && !t.Media["images"] && !strings.Contains(strings.ToLower(text), "gif") {
		text = strings.TrimSpace(text + " gif")
	}
	if text == "" {
		text = strings.Join(t.Names, " ")
	}
	return text
}

// pinShape is "tall", "wide" or "square" for a picture of the given size ("" when unknown).
func pinShape(width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	ratio := float64(height) / float64(width)
	switch {
	case ratio > 1.12:
		return "tall"
	case ratio < 0.89:
		return "wide"
	default:
		return "square"
	}
}

// Keep reports whether a pin of this kind ("videos", "gifs" or "images") and size matches.
func (t searchTags) Keep(kind string, width, height int) bool {
	if len(t.Media) > 0 && !t.Media[kind] {
		return false
	}
	if len(t.Shape) > 0 {
		shape := pinShape(width, height)
		if shape != "" && !t.Shape[shape] {
			return false
		}
	}
	return true
}
