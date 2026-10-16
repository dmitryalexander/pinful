package main

import "testing"

func TestPinlessLinkTarget(t *testing.T) {
	cases := map[string]string{
		"https://www.pinterest.com/pin/919226973945552146/":                 "/pin/919226973945552146",
		"https://www.pinterest.com/pin/919226973945552146":                  "/pin/919226973945552146",
		"https://www.pinterest.com/pin/919226973945552146/?utm_source=x":    "/pin/919226973945552146",
		"pinterest.com/pin/919226973945552146/":                             "/pin/919226973945552146",
		"https://uk.pinterest.com/pin/cute-cat--919226973945552146/":        "/pin/919226973945552146",
		"https://www.pinterest.co.uk/pin/919226973945552146/":               "/pin/919226973945552146",
		"  https://www.pinterest.ca/pin/919226973945552146/  ":              "/pin/919226973945552146",
		"cute cats":                                                         "",
		"https://example.com/pin/919226973945552146/":                       "",
		"https://pinterest.com.evil.example/pin/919226973945552146/":        "",
		"https://www.pinterest.com/ideas/cats/123/":                         "",
		"https://user:pw@www.pinterest.com/pin/919226973945552146/":         "",
	}
	for input, want := range cases {
		got, ok := pinlessLinkTarget(input)
		if want == "" && ok {
			t.Errorf("%q: expected no match, got %q", input, got)
		}
		if want != "" && (!ok || got != want) {
			t.Errorf("%q: got %q (%v), want %q", input, got, ok, want)
		}
	}
}
