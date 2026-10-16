package main

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// Pasting a Pinterest pin link into the search box opens that pin in Pinless.

var (
	pinterestHostPattern = regexp.MustCompile(`^(?:[a-z0-9-]+\.)*pinterest\.[a-z]{2,3}(?:\.[a-z]{2})?$`)
	pinPathPattern       = regexp.MustCompile(`^/pin/(?:[^/]*--)?([0-9]{5,25})/?$`)
	bareLinkPattern      = regexp.MustCompile(`^(?:[a-z0-9-]+\.)*(?:pinterest\.[a-z.]{2,6}|pin\.it)(?:/|$)`)
	shortPathPattern     = regexp.MustCompile(`^/[A-Za-z0-9]{3,20}/?$`)
)

func isPinterestHost(host string) bool {
	return pinterestHostPattern.MatchString(host)
}

// pinTarget returns "/pin/<id>" for a Pinterest pin URL.
func pinTarget(u *url.URL) (string, bool) {
	if !isPinterestHost(strings.ToLower(u.Hostname())) {
		return "", false
	}
	m := pinPathPattern.FindStringSubmatch(u.Path)
	if m == nil {
		return "", false
	}
	return "/pin/" + m[1], true
}

var shortLinkClient = &http.Client{
	Timeout: 5 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

// resolveShortLink follows a pin.it link by hand, only through pin.it and Pinterest hosts.
func resolveShortLink(u *url.URL) (string, bool) {
	current := u
	for hop := 0; hop < 4; hop++ {
		if target, ok := pinTarget(current); ok {
			return target, true
		}
		host := strings.ToLower(current.Hostname())
		if host != "pin.it" && !isPinterestHost(host) {
			return "", false
		}
		req, err := http.NewRequest(http.MethodGet, current.String(), nil)
		if err != nil {
			return "", false
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36")
		res, err := shortLinkClient.Do(req)
		if err != nil {
			return "", false
		}
		location := res.Header.Get("Location")
		res.Body.Close()
		if location == "" {
			return "", false
		}
		next, err := current.Parse(location)
		if err != nil || next.Scheme != "https" {
			return "", false
		}
		current = next
	}
	return "", false
}

// pinlessLinkTarget turns pasted Pinterest pin text into a Pinless path.
func pinlessLinkTarget(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > 500 || strings.ContainsAny(raw, " \t\r\n") {
		return "", false
	}
	lower := strings.ToLower(raw)
	if !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") {
		if !bareLinkPattern.MatchString(lower) {
			return "", false
		}
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.User != nil || u.Port() != "" {
		return "", false
	}
	if target, ok := pinTarget(u); ok {
		return target, true
	}
	if strings.ToLower(u.Hostname()) == "pin.it" && shortPathPattern.MatchString(u.Path) {
		u.Scheme = "https"
		return resolveShortLink(u)
	}
	return "", false
}
