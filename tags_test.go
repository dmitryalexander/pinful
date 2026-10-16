package main

import "testing"

func TestSearchTagsParseAndCombine(t *testing.T) {
	tags := parseSearchTags("tiktok dance #Videos #TALL")
	if tags.Text != "tiktok dance" || !tags.Media["videos"] || !tags.Shape["tall"] {
		t.Fatalf("parse: %+v", tags)
	}
	if tags.Scope() != "videos" || tags.UpstreamQuery() != "tiktok dance" || !tags.Filtering() {
		t.Fatalf("videos only: scope=%q query=%q", tags.Scope(), tags.UpstreamQuery())
	}
	if !tags.Keep("videos", 500, 900) || tags.Keep("videos", 900, 500) || tags.Keep("images", 500, 900) {
		t.Fatal("videos and tall narrow each other")
	}
	either := parseSearchTags("#gif #vids cats")
	if either.Scope() != "" || either.UpstreamQuery() != "cats gif" {
		t.Fatalf("gifs or videos: scope=%q query=%q", either.Scope(), either.UpstreamQuery())
	}
	if !either.Keep("gifs", 1, 1) || !either.Keep("videos", 1, 1) || either.Keep("images", 1, 1) {
		t.Fatal("alternatives within a group")
	}
	stills := parseSearchTags("mountains #pics #square #wide")
	if stills.UpstreamQuery() != "mountains" || stills.Keep("gifs", 100, 100) || !stills.Keep("images", 100, 100) || !stills.Keep("images", 200, 100) || stills.Keep("images", 100, 200) {
		t.Fatal("images with two shapes")
	}
	if !stills.Keep("images", 0, 0) {
		t.Fatal("unknown size is kept")
	}
	literal := parseSearchTags(`"#gifs" #unknown history #tall,`)
	if literal.Text != "#gifs #unknown history" || len(literal.Media) != 0 || !literal.Shape["tall"] {
		t.Fatalf("literal and unknown tags stay text: %+v", literal)
	}
	only := parseSearchTags("#gifs")
	if only.UpstreamQuery() != "gif" {
		t.Fatalf("tag-only search: %q", only.UpstreamQuery())
	}
	all := parseSearchTags("art #images #gifs #videos")
	if all.Filtering() || all.Scope() != "" {
		t.Fatal("all three kinds is no filter")
	}
	plain := parseSearchTags("art #fanart")
	if plain.Filtering() || plain.UpstreamQuery() != "art #fanart" {
		t.Fatal("plain search unchanged")
	}
}

func TestPinShape(t *testing.T) {
	for _, c := range []struct{ w, h int; want string }{{100, 200, "tall"}, {200, 100, "wide"}, {100, 105, "square"}, {0, 10, ""}} {
		if got := pinShape(c.w, c.h); got != c.want {
			t.Fatalf("%dx%d: %q", c.w, c.h, got)
		}
	}
}

func TestPinKind(t *testing.T) {
	var gif PinterestMedia
	gif.Embed.Type = "gif"
	if pinKind("/video?url=x", "https://i.pinimg.com/a.jpg", PinterestMedia{}) != "videos" || pinKind("", "https://i.pinimg.com/a.jpg", gif) != "gifs" ||
		pinKind("", "https://i.pinimg.com/originals/a.GIF", PinterestMedia{}) != "gifs" || pinKind("", "https://i.pinimg.com/a.jpg", PinterestMedia{}) != "images" {
		t.Fatal("kinds")
	}
}
