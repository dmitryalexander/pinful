package main

import (
	"compress/gzip"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

//go:embed templates/*
var templatesFS embed.FS

//go:embed static/*
var staticFS embed.FS

type SearchResult struct {
	Images   []string `json:"images"`
	Bookmark string   `json:"bookmark,omitempty"`
}

type Pin struct {
	pinOrigin
	UpstreamID string `json:"-"`
	VideoURL    string `json:"video_url,omitempty"`
	ID          string `json:"id"`
	ImageWidth  int    `json:"width"`
	ImageHeight int    `json:"height"`
	ImageURL    string `json:"image_url"`
	Title       string `json:"title"`
	Description string `json:"description"`
	PinnerName  string `json:"pinner_name"`
	PinnerUser  string `json:"pinner_user"`
}

type PinData struct {
	Pin     Pin
	Related []Pin
}

type Profile struct {
	CoverURL      string `json:"cover_url"`
	ID            string `json:"id"`
	Username      string `json:"username"`
	FullName      string `json:"full_name"`
	FirstName     string `json:"first_name"`
	About         string `json:"about"`
	PinCount      int    `json:"pin_count"`
	BoardCount    int    `json:"board_count"`
	FollowerCount int    `json:"follower_count"`
	ImageURL      string `json:"image_url"`
	ImageLargeURL string `json:"image_large_url"`
	WebsiteURL    string `json:"website_url"`
}

type Board struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	URL       string `json:"url"`
	PinCount  int    `json:"pin_count"`
	ImageURL  string `json:"image_url"`
	OwnerName string `json:"owner_name"`
	OwnerUser string `json:"owner_user"`
}

var allowedDomains = []string{"pinimg.com", "i.pinimg.com", "pinterest.com"}

func absURL(c *gin.Context, path string) string {
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s%s", scheme, c.Request.Host, path)
}

type PinterestImage struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type PinterestImages struct {
	Orig     PinterestImage `json:"orig"`
	Size736x PinterestImage `json:"736x"`
	Size474x PinterestImage `json:"474x"`
	Size564x PinterestImage `json:"564x"`
	Size236x PinterestImage `json:"236x"`
}

func extractBestImageInfo(pi PinterestImages) *PinterestImage {
	if pi.Orig.URL != "" {
		return &pi.Orig
	} else if pi.Size736x.URL != "" {
		return &pi.Size736x
	} else if pi.Size564x.URL != "" {
		return &pi.Size564x
	} else if pi.Size474x.URL != "" {
		return &pi.Size474x
	} else if pi.Size236x.URL != "" {
		return &pi.Size236x
	}
	return nil
}

func main() {
	if err := cleanDownloadFiles(); err != nil {
		fmt.Fprintln(os.Stderr, "Download cleanup:", err)
	}
	gin.SetMode(gin.ReleaseMode)
	gin.DefaultWriter = io.Discard
	gin.DefaultErrorWriter = io.Discard

	router := gin.Default()
	router.Use(func(c *gin.Context) {
		if !localPinterestHost(c.Request.Host) {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		c.Next()
	})

	router.Use(func(c *gin.Context) {
		c.Header("Referrer-Policy", "no-referrer")
		c.Header("Content-Security-Policy", "default-src 'none'; img-src 'self' data:; media-src 'self' blob:; script-src 'self'; connect-src 'self'; style-src 'self' 'unsafe-inline'; form-action 'self'; base-uri 'self'; frame-ancestors 'none'")
		c.SetSameSite(http.SameSiteStrictMode)
		c.Next()
	})

	// Playlists, history, follows and search groups are also saved on the server (appdata.go).
	// Handled before routing so it never clashes with the /:username/:slug routes.
	appData := newAppDataStore(appDataDir())
	if appData.err != nil {
		fmt.Fprintln(os.Stderr, "App data storage unavailable:", appData.err)
	}
	router.Use(func(c *gin.Context) {
		if c.Request.URL.Path == "/app-data/items" {
			appData.ServeHTTP(c.Writer, c.Request)
			c.Abort()
			return
		}
		c.Next()
	})

	// Register explicitly so Gin cannot redirect GET to the /:username/ route.
	router.Any("/pinterest-session", func(c *gin.Context) {
		pinterestSession.ServeHTTP(c.Writer, c.Request)
	})

	router.POST("/saved-wall-context", savedWallContextHandler)

	// Serve embedded static files
	staticSubFS, _ := fs.Sub(staticFS, "static")
	router.StaticFS("/static", http.FS(staticSubFS))

	// Load embedded templates
	tmpl := template.Must(template.ParseFS(templatesFS, "templates/*"))
	router.SetHTMLTemplate(tmpl)

	router.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", nil)
	})

	router.GET("/search/pins/", searchHandler)
	router.GET("/search-tags.json", searchTagsHandler)
	router.GET("/pin-status", pinStatusHandler)
	router.Any("/lens-jobs", func(c *gin.Context) { c.Status(http.StatusGone) })
	router.Any("/lens-jobs/:job", func(c *gin.Context) { c.Status(http.StatusGone) })
	router.GET("/lens-return", func(c *gin.Context) { c.Status(http.StatusGone) })
	router.GET("/pin/:id", pinHandler)
	router.GET("/image", proxyImageHandler)
	router.GET("/video", proxyVideoHandler)
	router.HEAD("/video", proxyVideoHandler)
	router.GET("/hls", proxyHLSHandler)
	router.HEAD("/hls", proxyHLSHandler)
	router.GET("/download", downloadVideoHandler)
	router.GET("/about", func(c *gin.Context) {
		c.HTML(http.StatusOK, "about.html", nil)
	})
	router.GET("/:username/", profileRedirectHandler)
	router.GET("/:username/_saved/", savedHandler)
	router.GET("/:username/_created/", createdHandler)
	router.GET("/:username/:slug", boardHandler)

	fmt.Print(` _____ _     _
|  _  |_|___| |___ ___ ___
|   __| |   | | -_|_ -|_ -|
|__|  |_|_|_|_|___|___|___|
`)
	address := os.Getenv("PINLESS_LISTEN")
	if address == "" {
		address = "0.0.0.0:3000"
	}
	fmt.Println("Server running at http://" + address)
	if err := router.Run(address); err != nil {
		panic(err)
	}
}

// searchResult is one pin on a search results page.
type searchResult struct {
	VideoURL    string
	ID          string
	Image       string
	ImageWidth  int
	ImageHeight int
	Kind        string // videos, gifs or images (search tags)
}

// searchFailure is how a failed Pinterest request is shown: JSON for transport errors and the
// results page with details for upstream errors, as before search tags.
type searchFailure struct {
	Status int
	JSON   bool
	Body   gin.H
}

// fetchSearchPage asks Pinterest for one page of results. It returns the pins, the bookmark for
// the next page ("" at the end) or a failure.
func fetchSearchPage(c *gin.Context, upstreamQuery, scope, bookmark string, csrftoken *string) ([]searchResult, string, *searchFailure) {
	apiURL := "https://www.pinterest.com/resource/BaseSearchResource/get/"
	options := map[string]interface{}{
		"query": upstreamQuery,
		"additional_fields": resolverPinFields,
	}
	if scope != "" {
		options["scope"] = scope
	}
	if bookmark != "" {
		options["bookmarks"] = []string{bookmark}
	}
	dataParamObj := map[string]interface{}{"options": options}

	dataParam, err := json.Marshal(dataParamObj)
	if err != nil {
		return nil, "", &searchFailure{Status: http.StatusInternalServerError, JSON: true, Body: gin.H{"error": "Failed to encode data"}}
	}

	dataParamEscaped := url.QueryEscape(string(dataParam))
	finalURL := fmt.Sprintf("%s?data=%s", apiURL, dataParamEscaped)

	// Continue searches with GET: Pinterest rejects the legacy page-two POST.
	req, err := http.NewRequest(http.MethodGet, finalURL, nil)
	if err != nil {
		return nil, "", &searchFailure{Status: http.StatusInternalServerError, JSON: true, Body: gin.H{"error": "Failed to create request"}}
	}
	req.Header.Set("x-pinterest-pws-handler", "www/search/[scope].js")
	req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")

	if *csrftoken != "" {
		req.Header.Set("x-csrftoken", *csrftoken)
		req.Header.Set("cookie", fmt.Sprintf("csrftoken=%s", *csrftoken))
	}

	client := newPinterestClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", &searchFailure{Status: http.StatusInternalServerError, JSON: true, Body: gin.H{"error": "Request failed"}}
	}
	defer resp.Body.Close()

	if newToken := resp.Cookies(); len(newToken) > 0 {
		for _, ck := range newToken {
			if ck != nil && ck.Name == "csrftoken" && ck.Value != "" {
				*csrftoken = ck.Value
				c.SetCookie("csrftoken", *csrftoken, 0, "/", "", c.Request.TLS != nil, true)
				break
			}
		}
	}

	var reader io.Reader = resp.Body
	contentEncoding := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Encoding")))
	if strings.Contains(contentEncoding, "gzip") {
		gzr, gzErr := gzip.NewReader(resp.Body)
		if gzErr != nil {
			return nil, "", &searchFailure{Status: http.StatusBadGateway, Body: gin.H{
				"error":            "Failed to init gzip reader",
				"upstream_status":  resp.Status,
				"content_encoding": resp.Header.Get("Content-Encoding"),
				"content_type":     resp.Header.Get("Content-Type"),
				"details":          gzErr.Error(),
			}}
		}
		defer gzr.Close()
		reader = gzr
	}

	bodyBytes, err := io.ReadAll(reader)
	if err != nil {
		return nil, "", &searchFailure{Status: http.StatusInternalServerError, JSON: true, Body: gin.H{"error": "Failed to read response"}}
	}
	if resp.StatusCode != http.StatusOK {
		snippet := string(bodyBytes)
		if len(snippet) > 500 {
			snippet = snippet[:500]
		}
		return nil, "", &searchFailure{Status: http.StatusBadGateway, Body: gin.H{
			"error":            "Upstream error",
			"upstream_status":  resp.Status,
			"content_encoding": resp.Header.Get("Content-Encoding"),
			"content_type":     resp.Header.Get("Content-Type"),
			"body":             snippet,
		}}
	}

	var responseData struct {
		ResourceResponse struct {
			Data struct {
				Results []struct {
					pinOrigin
					ID     string          `json:"id"`
					Title json.RawMessage `json:"title"`
					Description string `json:"description"`
					Pinner struct { FullName string `json:"full_name"`; Username string `json:"username"` } `json:"pinner"`
					Images PinterestImages `json:"images"`
					PinterestMedia
				} `json:"results"`
			} `json:"data"`
			Bookmark string `json:"bookmark,omitempty"`
		} `json:"resource_response"`
	}

	if err := json.Unmarshal(bodyBytes, &responseData); err != nil {
		snippet := string(bodyBytes)
		if len(snippet) > 500 {
			snippet = snippet[:500]
		}
		return nil, "", &searchFailure{Status: http.StatusBadGateway, Body: gin.H{
			"error":            "Failed to decode response",
			"upstream_status":  resp.Status,
			"content_encoding": resp.Header.Get("Content-Encoding"),
			"content_type":     resp.Header.Get("Content-Type"),
			"decode_error":     err.Error(),
			"body":             snippet,
		}}
	}

	var results []searchResult
	for _, result := range responseData.ResourceResponse.Data.Results {
		imageInfo := bestPinImage(result.Images, result.PinterestMedia)
		if imageInfo == nil {
			continue
		}
		imageUrl := imageInfo.URL
		if imageUrl != "" && isAllowedDomain(imageUrl) {
			proxyImageUrl := fmt.Sprintf("/image?url=%s", url.QueryEscape(imageUrl))
			videoURL := videoProxyURL(result.PinterestMedia)
			rememberPinPreview(Pin{pinOrigin: result.pinOrigin, ID: result.ID, Title: previewTitle(result.Title), Description: result.Description, PinnerName: result.Pinner.FullName, PinnerUser: result.Pinner.Username, ImageURL: proxyImageUrl, VideoURL: videoURL, ImageWidth: imageInfo.Width, ImageHeight: imageInfo.Height})
			results = append(results, searchResult{
				ID:          result.ID,
				VideoURL:    videoURL,
				Image:       proxyImageUrl,
				ImageWidth:  imageInfo.Width,
				ImageHeight: imageInfo.Height,
				Kind:        pinKind(videoURL, imageUrl, result.PinterestMedia),
			})
		}
	}
	return results, cleanBookmark(responseData.ResourceResponse.Bookmark), nil
}

// pinKind sorts a pin for the #videos, #gifs and #images tags.
func pinKind(videoURL, imageURL string, media PinterestMedia) string {
	if videoURL != "" {
		return "videos"
	}
	if strings.EqualFold(media.Embed.Type, "gif") {
		return "gifs"
	}
	if parsed, err := url.Parse(imageURL); err == nil && strings.EqualFold(path.Ext(parsed.Path), ".gif") {
		return "gifs"
	}
	return "images"
}

func searchHandler(c *gin.Context) {
	query := c.Query("q")

	// A pasted Pinterest pin link opens that pin directly.
	if _, paging := c.GetQuery("next"); !paging {
		if target, ok := pinlessLinkTarget(query); ok {
			c.Redirect(http.StatusFound, target)
			return
		}
	}

	// get bookmark from cookie for privacy
	bookmark := ""
	if cookie, err := c.Cookie("bookmark"); err == nil && cookie != "" {
		bookmark = cookie
	}

	// clear bookmark if new search
	if _, nextExists := c.GetQuery("next"); !nextExists {
		c.SetCookie("bookmark", "", -1, "/", "", c.Request.TLS != nil, true)
		bookmark = ""
	}
	if cursor, exists := c.GetQuery("cursor"); exists { bookmark = cursor }

	csrftoken := ""
	if cookie, err := c.Cookie("csrftoken"); err == nil && cookie != "" {
		csrftoken = cookie
	}
	if csrftoken == "" {
		csrftoken = fetchCSRFToken()
		if csrftoken != "" {
			c.SetCookie("csrftoken", csrftoken, 0, "/", "", c.Request.TLS != nil, true)
		}
	}

	// Search tags (#videos, #gifs, #tall...): Pinterest gets the words, the tags narrow the pins.
	// A narrowed page keeps asking Pinterest for more (up to four pages) so it is not left empty.
	tags := parseSearchTags(query)
	var results []searchResult
	for pages := 0; ; pages++ {
		page, next, failure := fetchSearchPage(c, tags.UpstreamQuery(), tags.Scope(), bookmark, &csrftoken)
		if failure != nil {
			if pages > 0 {
				break // show what was found; the next page retries from the same bookmark
			}
			if failure.JSON {
				c.JSON(failure.Status, failure.Body)
			} else {
				c.HTML(failure.Status, "results.html", gin.H{"Results": nil, "Query": query, "Error": failure.Body})
			}
			return
		}
		for _, item := range page {
			if tags.Keep(item.Kind, item.ImageWidth, item.ImageHeight) {
				results = append(results, item)
			}
		}
		bookmark = next
		if !tags.Filtering() || len(results) >= 12 || bookmark == "" || pages >= 3 {
			break
		}
	}

	// store bookmark for pagination
	if bookmark != "" {
		c.SetCookie("bookmark", bookmark, 0, "/", "", c.Request.TLS != nil, true)
	} else {
		// clear cookie when no pages
		c.SetCookie("bookmark", "", -1, "/", "", c.Request.TLS != nil, true)
	}

	c.HTML(http.StatusOK, "results.html", gin.H{
		"Results":  results,
        "WallContext": rememberPreviousWall(searchWallPins(results),"search:"+query,bookmark),
		"Bookmark": bookmark,
		"Query":    query,
	})
}

// searchTagsHandler lists the search tags for the Menu's tag guide.
func searchTagsHandler(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"groups": searchTagGroups})
}

func pinHandler(c *gin.Context) {
	pinID := c.Param("id")
	query := c.Query("q")
	from := c.Query("from")
	previous, hasPrevious := recalledPreviousWall(c.Query("wall_context"))
	inherited := false
	wallSeed := pinID

	// Move to the verified numeric page before reading pagination state. Its URL
	// keeps subsequent requests on the same recommendation seed after cache expiry.
	if strings.HasPrefix(pinID, "e1_") {
		if resolved := resolveOpaquePin(pinID); resolved != "" {
			target := "/pin/" + resolved
			if c.Request.URL.RawQuery != "" {
				target += "?" + c.Request.URL.RawQuery
			}
			c.Header("Cache-Control", "no-store")
			c.Redirect(http.StatusFound, target)
			return
		}
	}

	bookmark := ""
	if cookie, err := c.Cookie("bookmark"); err == nil && cookie != "" {
		bookmark = cookie
	}

	if _, nextExists := c.GetQuery("next"); !nextExists {
		c.SetCookie("bookmark", "", -1, "/", "", c.Request.TLS != nil, true)
		bookmark = ""
	}
	if cursor, exists := c.GetQuery("cursor"); exists {
		bookmark = cursor
	}

	csrftoken := ""
	if cookie, err := c.Cookie("csrftoken"); err == nil && cookie != "" {
		csrftoken = cookie
	}
	if csrftoken == "" {
		csrftoken = fetchCSRFToken()
		if csrftoken != "" {
			c.SetCookie("csrftoken", csrftoken, 0, "/", "", c.Request.TLS != nil, true)
		}
	}

	var pin Pin
	pinStatus := http.StatusOK
	previewOnly := false
	var related []Pin
	nextBookmark := ""
	pin, pinStatus = fetchPinDetailsResult(pinID, csrftoken)
	if pin.ImageURL == "" && pin.VideoURL == "" {
		if preview, ok := recalledPinPreview(pinID); ok {
			pin, previewOnly = preview, true
		}
	}
	if hasPrevious && c.Query("wall_fallback") == "1" && bookmark != "" {
		related, nextBookmark = fetchPreviousWallPage(c, previous, bookmark, &csrftoken)
		related = previousRecommendations(related, pin, pinID)
		inherited, wallSeed = true, previous.Seed
	} else {
		related, nextBookmark = fetchRelatedPins(pinID, csrftoken, bookmark)
		if len(related) == 0 && bookmark == "" && hasPrevious {
			candidates := previousRecommendations(previous.Pins, pin, pinID)
			if len(candidates) > 0 {
				related, nextBookmark = candidates, previous.Bookmark
				inherited, wallSeed = true, previous.Seed
			}
		}
	}
	for _, item := range related {
		rememberPinPreview(item)
	}

	if nextBookmark != "" {
		c.SetCookie("bookmark", nextBookmark, 0, "/", "", c.Request.TLS != nil, true)
	} else {
		c.SetCookie("bookmark", "", -1, "/", "", c.Request.TLS != nil, true)
	}

	ogImage := ""
	if pin.ImageURL != "" {
		ogImage = absURL(c, pin.ImageURL)
	}

	c.HTML(http.StatusOK, "pin.html", gin.H{
		"Pin":                      pin,
		"Related":                  related,
		"InheritedRecommendations": inherited,
		"WallContext":              rememberPreviousWall(related, wallSeed, nextBookmark),
		"RelatedBookmark":          nextBookmark,
		"PinLoadError":             pin.ImageURL == "" && pin.VideoURL == "",
		"RelatedLoadError":         related == nil,
		"PinNotFound":              pinStatus == http.StatusNotFound && !strings.HasPrefix(pinID, "e1_"),
		"PinPreviewOnly":           previewOnly,
		"Query":   query,
		"From":    from,
		"PinSelfURL": c.Request.URL.RequestURI(),
        "PageURL": absURL(c, c.Request.URL.RequestURI()),
		"OGImage": ogImage,
	})
}

// readPinResource distinguishes an unavailable/malformed upstream response from
// a successful resource whose data happens to be empty.
type pinResourceFailure struct { status int }
func (e *pinResourceFailure) Error() string { return fmt.Sprintf("Pinterest resource HTTP %d", e.status) }

func readPinResource(resp *http.Response) ([]byte, error) {
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &pinResourceFailure{resp.StatusCode}
	}
	var reader io.Reader = resp.Body
	if strings.Contains(strings.ToLower(resp.Header.Get("Content-Encoding")), "gzip") {
		gzr, err := gzip.NewReader(resp.Body)
		if err != nil { return nil, err }
		defer gzr.Close()
		reader = gzr
	}
	body, err := io.ReadAll(reader)
	if err != nil { return nil, err }
	var envelope struct {
		Response struct {
			Data json.RawMessage `json:"data"`
			Error json.RawMessage `json:"error"`
			Status string `json:"status"`
		} `json:"resource_response"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil { return nil, err }
	r := envelope.Response
	if (len(r.Error) > 0 && string(r.Error) != "null") ||
		(r.Status != "" && r.Status != "success") ||
		len(r.Data) == 0 || string(r.Data) == "null" {
		return nil, fmt.Errorf("Pinterest resource unavailable")
	}
	return body, nil
}

func fetchPinDetails(pinID string, csrftoken string) Pin {
	pin, _ := fetchPinDetailsResult(pinID, csrftoken)
	return pin
}

func fetchPinDetailsResult(pinID string, csrftoken string) (Pin, int) {
	return fetchPinDetailsWithClient(pinID, csrftoken, newPinterestClient())
}

func fetchPinDetailsWithClient(pinID string, csrftoken string, client *http.Client) (Pin, int) {
	apiURL := "https://www.pinterest.com/resource/PinResource/get/"
	sourceURL := fmt.Sprintf("/pin/%s/", pinID)
	options := map[string]interface{}{
		"id":            pinID,
		"field_set_key": "detailed",
	}
	dataParamObj := map[string]interface{}{"options": options}
	dataParam, _ := json.Marshal(dataParamObj)
	dataParamEscaped := url.QueryEscape(string(dataParam))
	sourceURLEscaped := url.QueryEscape(sourceURL)
	finalURL := fmt.Sprintf("%s?source_url=%s&data=%s", apiURL, sourceURLEscaped, dataParamEscaped)

	req, _ := http.NewRequest(http.MethodGet, finalURL, nil)
	req.Header.Set("Accept", "application/json, text/javascript, */*, q=0.01")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("x-pinterest-pws-handler", fmt.Sprintf("www/pin/%s.js", pinID))
	req.Header.Set("x-pinterest-source-url", sourceURL)
	req.Header.Set("Referer", "https://www.pinterest.com/")

	if csrftoken != "" {
		req.Header.Set("x-csrftoken", csrftoken)
		req.Header.Set("cookie", fmt.Sprintf("csrftoken=%s", csrftoken))
	}

	resp, err := client.Do(req)
	if err != nil {
		return Pin{ID: pinID}, http.StatusBadGateway
	}
	defer resp.Body.Close()

	bodyBytes, err := readPinResource(resp)
	if err != nil {
		var failure *pinResourceFailure
		if errors.As(err, &failure) { return Pin{ID: pinID}, failure.status }
		return Pin{ID: pinID}, http.StatusBadGateway
	}

	var singlePinResponse struct {
		ResourceResponse struct {
			Data struct {
				pinOrigin
				Title       string          `json:"title"`
				Description string          `json:"description"`
				Images      PinterestImages `json:"images"`
				PinterestMedia
				Pinner struct {
					FullName string `json:"full_name"`
					Username string `json:"username"`
				} `json:"pinner"`
				ID   string `json:"id"`
				Type string `json:"type"`
			} `json:"data"`
		} `json:"resource_response"`
	}

	if err := json.Unmarshal(bodyBytes, &singlePinResponse); err != nil {
		return Pin{ID: pinID}, http.StatusBadGateway
	}

	data := singlePinResponse.ResourceResponse.Data

	pin := Pin{
		pinOrigin: data.pinOrigin,
		UpstreamID: data.ID,
		ID:          pinID,
		VideoURL:    videoProxyURL(data.PinterestMedia),
		Title:       strings.TrimSpace(data.Title),
		Description: strings.TrimSpace(data.Description),
		PinnerName:  strings.TrimSpace(data.Pinner.FullName),
		PinnerUser:  strings.TrimSpace(data.Pinner.Username),
	}

	bestImage := bestPinImage(data.Images, data.PinterestMedia)
	if bestImage != nil {
		pin.ImageURL = fmt.Sprintf("/image?url=%s", url.QueryEscape(bestImage.URL))
		pin.ImageHeight = bestImage.Height
		pin.ImageWidth = bestImage.Width
	}

	if pin.ImageURL == "" && pin.VideoURL == "" { return pin, http.StatusBadGateway }
	rememberPinPreview(pin)
	return pin, http.StatusOK
}

func fetchRelatedPins(pinID string, csrftoken string, bookmark string) ([]Pin, string) {
	pins, next, _ := fetchRelatedPinsWithClient(pinID, csrftoken, bookmark, newPinterestClient(), 12)
	return pins, next
}

func fetchRelatedPinsWithClient(pinID string, csrftoken string, bookmark string, client *http.Client, pageSize int) ([]Pin, string, int) {
	apiURL := "https://www.pinterest.com/resource/RelatedModulesResource/get/"
	sourceURL := fmt.Sprintf("/pin/%s/", pinID)
	options := map[string]interface{}{
		"pin_id":    pinID,
		"page_size": pageSize,
		"source":    "pin",
		"additional_fields": resolverPinFields,
	}
	if bookmark != "" {
		options["bookmarks"] = []string{bookmark}
	}
	dataParamObj := map[string]interface{}{"options": options}
	dataParam, _ := json.Marshal(dataParamObj)
	dataParamEscaped := url.QueryEscape(string(dataParam))
	sourceURLEscaped := url.QueryEscape(sourceURL)

	finalURL := fmt.Sprintf("%s?source_url=%s&data=%s", apiURL, sourceURLEscaped, dataParamEscaped)

	// The read-resource endpoint accepts continuation bookmarks over GET too.
	// Switching to POST on page two triggers upstream 403s and stops the wall.
	// Keep source_url and the bookmark together on every page.
	req, _ := http.NewRequest(http.MethodGet, finalURL, nil)
	req.Header.Set("Accept", "application/json, text/javascript, */*, q=0.01")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("x-pinterest-pws-handler", fmt.Sprintf("www/pin/%s.js", pinID))
	req.Header.Set("x-pinterest-source-url", sourceURL)
	req.Header.Set("Referer", "https://www.pinterest.com/")

	if csrftoken != "" {
		req.Header.Set("x-csrftoken", csrftoken)
		req.Header.Set("cookie", fmt.Sprintf("csrftoken=%s", csrftoken))
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", 0
	}
	defer resp.Body.Close()

	bodyBytes, err := readPinResource(resp)
	if err != nil {
		return nil, "", 0
	}

	var responseData struct {
		ResourceResponse struct {
			Data []struct {
				pinOrigin
				ID          string          `json:"id"`
				Type        string          `json:"type"`
				StoryType   string          `json:"story_type"`
				TitleRaw    json.RawMessage `json:"title"`
				GridTitle   string          `json:"grid_title"`
				Description string          `json:"description"`
				Images      PinterestImages `json:"images"`
				PinterestMedia
				Pinner struct {
					FullName string `json:"full_name"`
					Username string `json:"username"`
				} `json:"pinner"`
				AggregatedPinData struct {
					AggregatedStats struct {
						Saves int `json:"saves"`
					} `json:"aggregated_stats"`
				} `json:"aggregated_pin_data"`
			} `json:"data"`
			Bookmark string `json:"bookmark,omitempty"`
		} `json:"resource_response"`
	}

	if err := json.Unmarshal(bodyBytes, &responseData); err != nil {
		return nil, "", 0
	}

	related := make([]Pin, 0)
	pinCount := 0
	for _, result := range responseData.ResourceResponse.Data {
		if result.Type != "pin" || result.ID == "" {
			continue
		}
		pinCount++

		title := ""
		if len(result.TitleRaw) > 0 {
			var titleObj struct {
				Format string   `json:"format"`
				Args   []string `json:"args"`
			}
			if err := json.Unmarshal(result.TitleRaw, &titleObj); err == nil && titleObj.Format != "" {
				title = titleObj.Format
			} else {
				var titleStr string
				if err := json.Unmarshal(result.TitleRaw, &titleStr); err == nil {
					title = titleStr
				}
			}
		}
		if title == "" {
			title = result.GridTitle
		}
		if title == "" {
			title = result.Description
		}

		bestImage := bestPinImage(result.Images, result.PinterestMedia)
		if bestImage != nil {
			related = append(related, Pin{
				pinOrigin: result.pinOrigin,
				ID:          result.ID,
				VideoURL:    videoProxyURL(result.PinterestMedia),
				ImageURL:    fmt.Sprintf("/image?url=%s", url.QueryEscape(bestImage.URL)),
				ImageWidth:  bestImage.Width,
				ImageHeight: bestImage.Height,
				Title:       strings.TrimSpace(title),
				Description: strings.TrimSpace(result.Description),
				PinnerName:  strings.TrimSpace(result.Pinner.FullName),
				PinnerUser:  strings.TrimSpace(result.Pinner.Username),
			})
		}
	}

	return related, cleanBookmark(responseData.ResourceResponse.Bookmark), pinCount
}

func profileRedirectHandler(c *gin.Context) {
	username := c.Param("username")
	c.Redirect(http.StatusFound, "/"+username+"/_saved/")
}

func savedHandler(c *gin.Context) {
	username := c.Param("username")

	csrftoken := ""
	if cookie, err := c.Cookie("csrftoken"); err == nil && cookie != "" {
		csrftoken = cookie
	}
	if csrftoken == "" {
		csrftoken = fetchCSRFToken()
		if csrftoken != "" {
			c.SetCookie("csrftoken", csrftoken, 0, "/", "", c.Request.TLS != nil, true)
		}
	}

	bookmark := ""
	if cookie, err := c.Cookie("bookmark"); err == nil && cookie != "" {
		bookmark = cookie
	}
	if _, nextExists := c.GetQuery("next"); !nextExists {
		c.SetCookie("bookmark", "", -1, "/", "", c.Request.TLS != nil, true)
		bookmark = ""
	}
	if cursor, exists := c.GetQuery("cursor"); exists { bookmark = cursor }

	pinsBookmark := ""
	if cookie, err := c.Cookie("pins_bookmark"); err == nil && cookie != "" {
		pinsBookmark = cookie
	}
	if _, nextExists := c.GetQuery("pins_next"); !nextExists {
		c.SetCookie("pins_bookmark", "", -1, "/", "", c.Request.TLS != nil, true)
		pinsBookmark = ""
	}
	if cursor, exists := c.GetQuery("pins_cursor"); exists { pinsBookmark = cursor }

	profile, profileToken := fetchUserProfile(username, csrftoken)
    if profile.Username == "" { profile.Username = username; profile.FullName = username }
	if profileToken != "" {
		csrftoken = profileToken
		c.SetCookie("csrftoken", csrftoken, 0, "/", "", c.Request.TLS != nil, true)
	}

	boards, nextBookmark, boardToken := fetchBoards(username, csrftoken, bookmark)
	if boardToken != "" {
		csrftoken = boardToken
		c.SetCookie("csrftoken", csrftoken, 0, "/", "", c.Request.TLS != nil, true)
	}

	if nextBookmark != "" {
		c.SetCookie("bookmark", nextBookmark, 0, "/", "", c.Request.TLS != nil, true)
	} else {
		c.SetCookie("bookmark", "", -1, "/", "", c.Request.TLS != nil, true)
	}

	profilePins, pinsNextBookmark := fetchUserProfilePins(username, csrftoken, pinsBookmark)

	if pinsNextBookmark != "" {
		c.SetCookie("pins_bookmark", pinsNextBookmark, 0, "/", "", c.Request.TLS != nil, true)
	} else {
		c.SetCookie("pins_bookmark", "", -1, "/", "", c.Request.TLS != nil, true)
	}

	ogImage := ""
	if profile.ImageURL != "" {
		ogImage = absURL(c, profile.ImageURL)
	}

	c.HTML(http.StatusOK, "profile.html", gin.H{
		"Profile":      profile,
		"Boards":       boards,
        "SourceError": profileSourceError(boards == nil && profilePins == nil),
		"ProfilePins":  profilePins,
		"Bookmark":     nextBookmark,
		"PinsBookmark": pinsNextBookmark,
		"PageURL":      absURL(c, c.Request.URL.RequestURI()),
		"OGImage":      ogImage,
	})
}

func createdHandler(c *gin.Context) {
	username := c.Param("username")

	csrftoken := ""
	if cookie, err := c.Cookie("csrftoken"); err == nil && cookie != "" {
		csrftoken = cookie
	}
	if csrftoken == "" {
		csrftoken = fetchCSRFToken()
		if csrftoken != "" {
			c.SetCookie("csrftoken", csrftoken, 0, "/", "", c.Request.TLS != nil, true)
		}
	}

	bookmark := ""
	if cookie, err := c.Cookie("bookmark"); err == nil && cookie != "" {
		bookmark = cookie
	}
	if _, nextExists := c.GetQuery("next"); !nextExists {
		c.SetCookie("bookmark", "", -1, "/", "", c.Request.TLS != nil, true)
		bookmark = ""
	}
	if cursor, exists := c.GetQuery("cursor"); exists { bookmark = cursor }

	profile, profileToken := fetchUserProfile(username, csrftoken)
    if profile.Username == "" { profile.Username = username; profile.FullName = username }
	if profileToken != "" {
		csrftoken = profileToken
		c.SetCookie("csrftoken", csrftoken, 0, "/", "", c.Request.TLS != nil, true)
	}

	pins, nextBookmark := fetchUserCreatedPins(profile.ID, username, csrftoken, bookmark)

	if nextBookmark != "" {
		c.SetCookie("bookmark", nextBookmark, 0, "/", "", c.Request.TLS != nil, true)
	} else {
		c.SetCookie("bookmark", "", -1, "/", "", c.Request.TLS != nil, true)
	}

	ogImage := ""
	if profile.ImageURL != "" {
		ogImage = absURL(c, profile.ImageURL)
	}

	c.HTML(http.StatusOK, "profile.html", gin.H{
		"Profile":  profile,
		"Pins":     pins,
        "Created": true,
        "SourceError": profileSourceError(pins == nil),
		"Bookmark": nextBookmark,
		"PageURL":  absURL(c, c.Request.URL.RequestURI()),
		"OGImage":  ogImage,
	})
}

func boardHandler(c *gin.Context) {
	username := c.Param("username")
	slug := c.Param("slug")

	csrftoken := ""
	if cookie, err := c.Cookie("csrftoken"); err == nil && cookie != "" {
		csrftoken = cookie
	}
	if csrftoken == "" {
		csrftoken = fetchCSRFToken()
		if csrftoken != "" {
			c.SetCookie("csrftoken", csrftoken, 0, "/", "", c.Request.TLS != nil, true)
		}
	}

	bookmark := ""
	if cookie, err := c.Cookie("bookmark"); err == nil && cookie != "" {
		bookmark = cookie
	}
	if _, nextExists := c.GetQuery("next"); !nextExists {
		c.SetCookie("bookmark", "", -1, "/", "", c.Request.TLS != nil, true)
		bookmark = ""
	}
	if cursor, exists := c.GetQuery("cursor"); exists { bookmark = cursor }

	boardInfo := fetchBoardInfo(username, slug, csrftoken)
	var pins []Pin
	var nextBookmark string
	var feedToken string
	if boardInfo.ID != "" {
		sourceURL := fmt.Sprintf("/%s/%s/", username, slug)
		pins, nextBookmark, feedToken = fetchBoardPins(boardInfo.ID, sourceURL, csrftoken, bookmark)
		if feedToken != "" {
			csrftoken = feedToken
			c.SetCookie("csrftoken", csrftoken, 0, "/", "", c.Request.TLS != nil, true)
		}
	}

	if nextBookmark != "" {
		c.SetCookie("bookmark", nextBookmark, 0, "/", "", c.Request.TLS != nil, true)
	} else {
		c.SetCookie("bookmark", "", -1, "/", "", c.Request.TLS != nil, true)
	}

	ogImage := ""
	if boardInfo.ImageURL != "" {
		ogImage = absURL(c, boardInfo.ImageURL)
	}

	c.HTML(http.StatusOK, "board.html", gin.H{
		"Board":    boardInfo,
        "SourceError": profileSourceError(pins == nil),
		"Pins":     pins,
		"Bookmark": nextBookmark,
		"PageURL":  absURL(c, c.Request.URL.RequestURI()),
		"OGImage":  ogImage,
	})
}

func fetchUserProfile(username string, csrftoken string) (Profile, string) {
	apiURL := "https://www.pinterest.com/resource/UserResource/get/"
	sourceURL := fmt.Sprintf("/%s/", username)
	options := map[string]interface{}{
		"username":      username,
		"field_set_key": "profile",
	}
	dataParamObj := map[string]interface{}{"options": options}
	dataParam, _ := json.Marshal(dataParamObj)
	dataParamEscaped := url.QueryEscape(string(dataParam))
	sourceURLEscaped := url.QueryEscape(sourceURL)
	finalURL := fmt.Sprintf("%s?source_url=%s&data=%s", apiURL, sourceURLEscaped, dataParamEscaped)

	req, _ := http.NewRequest(http.MethodGet, finalURL, nil)
	req.Header.Set("Accept", "application/json, text/javascript, */*, q=0.01")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("x-pinterest-pws-handler", fmt.Sprintf("www/%s.js", username))
	req.Header.Set("x-pinterest-source-url", sourceURL)
	req.Header.Set("Referer", "https://www.pinterest.com/")

	if csrftoken != "" {
		req.Header.Set("x-csrftoken", csrftoken)
		req.Header.Set("cookie", fmt.Sprintf("csrftoken=%s", csrftoken))
	}

	client := newPinterestClient()
	resp, err := client.Do(req)
	if err != nil {
		return Profile{}, ""
	}
	defer resp.Body.Close()

	newToken := ""
	for _, ck := range resp.Cookies() {
		if ck != nil && ck.Name == "csrftoken" && ck.Value != "" {
			newToken = ck.Value
			break
		}
	}

	var reader io.Reader = resp.Body
	contentEncoding := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Encoding")))
	if strings.Contains(contentEncoding, "gzip") {
		gzr, err := gzip.NewReader(resp.Body)
		if err != nil {
			return Profile{}, newToken
		}
		defer gzr.Close()
		reader = gzr
	}

	bodyBytes, _ := io.ReadAll(reader)

	var responseData struct {
		ResourceResponse struct {
			Data struct {
				ID             string `json:"id"`
				Username       string `json:"username"`
				FullName       string `json:"full_name"`
				FirstName      string `json:"first_name"`
				About          string `json:"about"`
				PinCount       int    `json:"pin_count"`
				BoardCount     int    `json:"board_count"`
				FollowerCount  int    `json:"follower_count"`
				ImageLargeURL  string `json:"image_large_url"`
				ImageMediumURL string `json:"image_medium_url"`
				WebsiteURL     string `json:"website_url"`
				ProfileCover struct {
					Images map[string]struct { URL string `json:"url"`; Width int `json:"width"` } `json:"images"`
				} `json:"profile_cover"`
			} `json:"data"`
		} `json:"resource_response"`
	}

	if err := json.Unmarshal(bodyBytes, &responseData); err != nil {
		return Profile{}, newToken
	}

	d := responseData.ResourceResponse.Data
	coverURL := ""
	coverWidth := 0
	for _, image := range d.ProfileCover.Images {
		parsed, err := url.Parse(image.URL)
		if err == nil && parsed.Scheme == "https" && strings.HasSuffix(parsed.Hostname(), ".pinimg.com") && image.Width > coverWidth {
			coverURL = "/image?url=" + url.QueryEscape(image.URL)
			coverWidth = image.Width
		}
	}
	imageURL := ""
	if d.ImageLargeURL != "" {
		imageURL = fmt.Sprintf("/image?url=%s", url.QueryEscape(d.ImageLargeURL))
	} else if d.ImageMediumURL != "" {
		imageURL = fmt.Sprintf("/image?url=%s", url.QueryEscape(d.ImageMediumURL))
	}

	return Profile{
		CoverURL:      coverURL,
		ID:            d.ID,
		Username:      d.Username,
		FullName:      d.FullName,
		FirstName:     d.FirstName,
		About:         d.About,
		PinCount:      d.PinCount,
		BoardCount:    d.BoardCount,
		FollowerCount: d.FollowerCount,
		ImageURL:      imageURL,
		WebsiteURL:    d.WebsiteURL,
	}, newToken
}

func fetchBoards(username string, csrftoken string, bookmark string) ([]Board, string, string) {
	apiURL := "https://www.pinterest.com/resource/BoardsFeedResource/get/"
	sourceURL := fmt.Sprintf("/%s/", username)
	options := map[string]interface{}{
		"field_set_key":  "profile_grid_item",
		"filter_stories": false,
		"sort":           "last_pinned_to",
		"username":       username,
	}
	if bookmark != "" {
		options["bookmarks"] = []string{bookmark}
	}
	dataParamObj := map[string]interface{}{"options": options}
	dataParam, _ := json.Marshal(dataParamObj)
	dataParamEscaped := url.QueryEscape(string(dataParam))
	sourceURLEscaped := url.QueryEscape(sourceURL)

	finalURL := fmt.Sprintf("%s?source_url=%s&data=%s", apiURL, sourceURLEscaped, dataParamEscaped)

	method := http.MethodGet
	var body io.Reader
	if bookmark != "" {
		method = http.MethodPost
		finalURL = apiURL
		body = strings.NewReader("data=" + dataParamEscaped)
	}

	req, _ := http.NewRequest(method, finalURL, body)
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	req.Header.Set("Accept", "application/json, text/javascript, */*, q=0.01")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("x-pinterest-pws-handler", fmt.Sprintf("www/%s.js", username))
	req.Header.Set("x-pinterest-source-url", sourceURL)
	req.Header.Set("Referer", "https://www.pinterest.com/")

	if csrftoken != "" {
		req.Header.Set("x-csrftoken", csrftoken)
		req.Header.Set("cookie", fmt.Sprintf("csrftoken=%s", csrftoken))
	}

	client := newPinterestClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK { return nil, "", "" }

	newToken := ""
	for _, ck := range resp.Cookies() {
		if ck != nil && ck.Name == "csrftoken" && ck.Value != "" {
			newToken = ck.Value
			break
		}
	}

	var reader io.Reader = resp.Body
	contentEncoding := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Encoding")))
	if strings.Contains(contentEncoding, "gzip") {
		gzr, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, "", newToken
		}
		defer gzr.Close()
		reader = gzr
	}

	bodyBytes, _ := io.ReadAll(reader)

	var responseData struct {
		ResourceResponse struct {
			Data []struct {
				ID          string `json:"id"`
				Type        string `json:"type"`
				Name        string `json:"name"`
				URL         string `json:"url"`
				PinCount    int    `json:"pin_count"`
				CoverImages struct {
					Cover200x150 *struct {
						URL string `json:"url"`
					} `json:"200x150"`
				} `json:"cover_images"`
				ImageCoverURL string `json:"image_cover_url"`
			} `json:"data"`
			Bookmark string `json:"bookmark,omitempty"`
		} `json:"resource_response"`
	}

	if err := json.Unmarshal(bodyBytes, &responseData); err != nil {
		return nil, "", newToken
	}

	if responseData.ResourceResponse.Data == nil { return nil, "", newToken }
	boards := make([]Board, 0)
	for _, r := range responseData.ResourceResponse.Data {
		if r.Type != "board" || r.ID == "" {
			continue
		}
		imageURL := ""
		if r.CoverImages.Cover200x150 != nil {
			imageURL = fmt.Sprintf("/image?url=%s", url.QueryEscape(r.CoverImages.Cover200x150.URL))
		} else if r.ImageCoverURL != "" {
			imageURL = fmt.Sprintf("/image?url=%s", url.QueryEscape(r.ImageCoverURL))
		}
		boards = append(boards, Board{
			ID:       r.ID,
			Name:     r.Name,
			URL:      r.URL,
			PinCount: r.PinCount,
			ImageURL: imageURL,
		})
	}

	return boards, cleanBookmark(responseData.ResourceResponse.Bookmark), newToken
}

func fetchUserCreatedPins(userID string, username string, csrftoken string, bookmark string) ([]Pin, string) {
	apiURL := "https://www.pinterest.com/resource/UserActivityPinsResource/get/"
	sourceURL := fmt.Sprintf("/%s/_created/", username)
	options := map[string]interface{}{
		"exclude_add_pin_rep": true,
		"field_set_key":       "profile_created_grid_item",
		"is_own_profile_pins": false,
		"user_id":             userID,
		"username":            username,
	}
	if bookmark != "" {
		options["bookmarks"] = []string{bookmark}
	}
	dataParamObj := map[string]interface{}{"options": options}
	dataParam, _ := json.Marshal(dataParamObj)
	dataParamEscaped := url.QueryEscape(string(dataParam))
	sourceURLEscaped := url.QueryEscape(sourceURL)

	finalURL := fmt.Sprintf("%s?source_url=%s&data=%s&_=%d", apiURL, sourceURLEscaped, dataParamEscaped, time.Now().UnixMilli())

	method := http.MethodGet
	var body io.Reader
	if bookmark != "" {
		method = http.MethodPost
		finalURL = apiURL
		body = strings.NewReader("data=" + dataParamEscaped)
	}

	req, _ := http.NewRequest(method, finalURL, body)
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	req.Header.Set("Accept", "application/json, text/javascript, */*, q=0.01")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("x-pinterest-source-url", sourceURL)
	req.Header.Set("x-pinterest-pws-handler", fmt.Sprintf("www/%s/_created.js", username))
	req.Header.Set("Referer", "https://www.pinterest.com/")

	if csrftoken != "" {
		req.Header.Set("x-csrftoken", csrftoken)
		req.Header.Set("cookie", fmt.Sprintf("csrftoken=%s", csrftoken))
	}

	client := newPinterestClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK { return nil, "" }

	var reader io.Reader = resp.Body
	contentEncoding := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Encoding")))
	if strings.Contains(contentEncoding, "gzip") {
		gzr, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, ""
		}
		defer gzr.Close()
		reader = gzr
	}

	bodyBytes, _ := io.ReadAll(reader)

	var responseData struct {
		ResourceResponse struct {
			Data []struct {
				ID     string          `json:"id"`
				Type   string          `json:"type"`
				Images PinterestImages `json:"images"`
				PinterestMedia
				Pinner struct {
					FullName string `json:"full_name"`
					Username string `json:"username"`
				} `json:"pinner"`
			} `json:"data"`
			Bookmark string `json:"bookmark,omitempty"`
		} `json:"resource_response"`
	}

	if err := json.Unmarshal(bodyBytes, &responseData); err != nil {
		return nil, ""
	}

	if responseData.ResourceResponse.Data == nil { return nil, "" }
	pins := make([]Pin, 0)
	for _, result := range responseData.ResourceResponse.Data {
		if result.Type != "pin" || result.ID == "" {
			continue
		}

		bestImage := bestPinImage(result.Images, result.PinterestMedia)
		if bestImage != nil {
			pins = append(pins, Pin{
				ID:          result.ID,
				VideoURL:    videoProxyURL(result.PinterestMedia),
				ImageURL:    fmt.Sprintf("/image?url=%s", url.QueryEscape(bestImage.URL)),
				ImageWidth:  bestImage.Width,
				ImageHeight: bestImage.Height,
				PinnerName:  strings.TrimSpace(result.Pinner.FullName),
			})
		}
	}

	return pins, cleanBookmark(responseData.ResourceResponse.Bookmark)
}

func fetchUserProfilePins(username string, csrftoken string, bookmark string) ([]Pin, string) {
	apiURL := "https://www.pinterest.com/resource/UserPinsResource/get/"
	sourceURL := fmt.Sprintf("/%s/", username)
	options := map[string]interface{}{
		"add_vase":            true,
		"field_set_key":       "mobile_grid_item",
		"is_own_profile_pins": false,
		"username":            username,
	}
	if bookmark != "" {
		options["bookmarks"] = []string{bookmark}
	}
	dataParamObj := map[string]interface{}{
		"options": options,
		"context": map[string]interface{}{},
	}
	dataParam, _ := json.Marshal(dataParamObj)
	dataParamEscaped := url.QueryEscape(string(dataParam))
	sourceURLEscaped := url.QueryEscape(sourceURL)

	finalURL := fmt.Sprintf("%s?source_url=%s&data=%s&_=%d", apiURL, sourceURLEscaped, dataParamEscaped, time.Now().UnixMilli())

	method := http.MethodGet
	var body io.Reader
	if bookmark != "" {
		method = http.MethodPost
		finalURL = apiURL
		body = strings.NewReader("data=" + dataParamEscaped)
	}

	req, _ := http.NewRequest(method, finalURL, body)
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	req.Header.Set("Accept", "application/json, text/javascript, */*, q=0.01")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("x-pinterest-source-url", sourceURL)
	req.Header.Set("x-pinterest-pws-handler", fmt.Sprintf("www/%s.js", username))
	req.Header.Set("Referer", "https://www.pinterest.com/")

	if csrftoken != "" {
		req.Header.Set("x-csrftoken", csrftoken)
		req.Header.Set("cookie", fmt.Sprintf("csrftoken=%s", csrftoken))
	}

	client := newPinterestClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK { return nil, "" }

	var reader io.Reader = resp.Body
	contentEncoding := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Encoding")))
	if strings.Contains(contentEncoding, "gzip") {
		gzr, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, ""
		}
		defer gzr.Close()
		reader = gzr
	}

	bodyBytes, _ := io.ReadAll(reader)

	var responseData struct {
		ResourceResponse struct {
			Data []struct {
				ID     string          `json:"id"`
				Type   string          `json:"type"`
				Images PinterestImages `json:"images"`
				PinterestMedia
				Pinner struct {
					FullName string `json:"full_name"`
					Username string `json:"username"`
				} `json:"pinner"`
			} `json:"data"`
			Bookmark string `json:"bookmark,omitempty"`
		} `json:"resource_response"`
	}

	if err := json.Unmarshal(bodyBytes, &responseData); err != nil {
		return nil, ""
	}

	if responseData.ResourceResponse.Data == nil { return nil, "" }
	pins := make([]Pin, 0)
	for _, result := range responseData.ResourceResponse.Data {
		if result.Type != "pin" || result.ID == "" {
			continue
		}

		bestImage := bestPinImage(result.Images, result.PinterestMedia)
		if bestImage != nil {
			pins = append(pins, Pin{
				ID:          result.ID,
				VideoURL:    videoProxyURL(result.PinterestMedia),
				ImageURL:    fmt.Sprintf("/image?url=%s", url.QueryEscape(bestImage.URL)),
				ImageWidth:  bestImage.Width,
				ImageHeight: bestImage.Height,
				PinnerName:  strings.TrimSpace(result.Pinner.FullName),
			})
		}
	}

	return pins, cleanBookmark(responseData.ResourceResponse.Bookmark)
}

func fetchBoardInfo(username string, slug string, csrftoken string) Board {
	apiURL := "https://www.pinterest.com/resource/BoardResource/get/"
	sourceURL := fmt.Sprintf("/%s/%s/", username, slug)
	options := map[string]interface{}{
		"field_set_key":  "profile_grid_item",
		"is_mobile_fork": true,
		"username":       username,
		"slug":           slug,
	}
	dataParamObj := map[string]interface{}{"options": options}
	dataParam, _ := json.Marshal(dataParamObj)
	dataParamEscaped := url.QueryEscape(string(dataParam))
	sourceURLEscaped := url.QueryEscape(sourceURL)
	finalURL := fmt.Sprintf("%s?source_url=%s&data=%s", apiURL, sourceURLEscaped, dataParamEscaped)

	req, _ := http.NewRequest(http.MethodGet, finalURL, nil)
	req.Header.Set("Accept", "application/json, text/javascript, */*, q=0.01")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("x-pinterest-pws-handler", fmt.Sprintf("www/%s/%s.js", username, slug))
	req.Header.Set("x-pinterest-source-url", sourceURL)
	req.Header.Set("Referer", "https://www.pinterest.com/")

	if csrftoken != "" {
		req.Header.Set("x-csrftoken", csrftoken)
		req.Header.Set("cookie", fmt.Sprintf("csrftoken=%s", csrftoken))
	}

	client := newPinterestClient()
	resp, err := client.Do(req)
	if err != nil {
		return Board{}
	}
	defer resp.Body.Close()

	var reader io.Reader = resp.Body
	contentEncoding := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Encoding")))
	if strings.Contains(contentEncoding, "gzip") {
		gzr, err := gzip.NewReader(resp.Body)
		if err != nil {
			return Board{}
		}
		defer gzr.Close()
		reader = gzr
	}

	bodyBytes, _ := io.ReadAll(reader)

	var responseData struct {
		ResourceResponse struct {
			Data struct {
				ID            string `json:"id"`
				Name          string `json:"name"`
				URL           string `json:"url"`
				PinCount      int    `json:"pin_count"`
				ImageCoverURL string `json:"image_cover_url"`
				Description   string `json:"description"`
				Owner         struct {
					Username string `json:"username"`
					FullName string `json:"full_name"`
				} `json:"owner"`
			} `json:"data"`
		} `json:"resource_response"`
	}

	if err := json.Unmarshal(bodyBytes, &responseData); err != nil {
		return Board{}
	}

	d := responseData.ResourceResponse.Data
	imageURL := ""
	if d.ImageCoverURL != "" {
		imageURL = fmt.Sprintf("/image?url=%s", url.QueryEscape(d.ImageCoverURL))
	}

	return Board{
		ID:        d.ID,
		Name:      d.Name,
		URL:       d.URL,
		PinCount:  d.PinCount,
		ImageURL:  imageURL,
		OwnerName: d.Owner.FullName,
		OwnerUser: d.Owner.Username,
	}
}

func fetchBoardPins(boardID string, sourceURL string, csrftoken string, bookmark string) ([]Pin, string, string) {
	apiURL := "https://www.pinterest.com/resource/BoardFeedResource/get/"
	options := map[string]interface{}{
		"add_vase":             true,
		"board_id":             boardID,
		"field_set_key":        "react_grid_pin",
		"filter_section_pins":  false,
		"is_react":             true,
		"prepend":              false,
		"page_size":            15,
		"redux_normalize_feed": true,
	}
	if bookmark != "" {
		options["bookmarks"] = []string{bookmark}
	} else {
		options["bookmarks"] = nil
	}
	dataParamObj := map[string]interface{}{
		"options": options,
		"context": map[string]interface{}{},
	}
	dataParam, _ := json.Marshal(dataParamObj)
	dataParamEscaped := url.QueryEscape(string(dataParam))
	sourceURLEscaped := url.QueryEscape(sourceURL)
	finalURL := fmt.Sprintf("%s?source_url=%s&data=%s&_=%d", apiURL, sourceURLEscaped, dataParamEscaped, time.Now().UnixMilli())

	method := http.MethodGet
	var body io.Reader
	if bookmark != "" {
		method = http.MethodPost
		finalURL = apiURL
		body = strings.NewReader("data=" + dataParamEscaped)
	}

	req, _ := http.NewRequest(method, finalURL, body)
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	req.Header.Set("Accept", "application/json, text/javascript, */*, q=0.01")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("x-pinterest-source-url", sourceURL)
	req.Header.Set("x-pinterest-pws-handler", "www/[username]/[slug].js")
	req.Header.Set("Referer", "https://www.pinterest.com/")

	if csrftoken != "" {
		req.Header.Set("x-csrftoken", csrftoken)
		req.Header.Set("cookie", fmt.Sprintf("csrftoken=%s", csrftoken))
	}

	client := newPinterestClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK { return nil, "", "" }

	newToken := ""
	for _, ck := range resp.Cookies() {
		if ck != nil && ck.Name == "csrftoken" && ck.Value != "" {
			newToken = ck.Value
			break
		}
	}

	var reader io.Reader = resp.Body
	contentEncoding := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Encoding")))
	if strings.Contains(contentEncoding, "gzip") {
		gzr, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, "", newToken
		}
		defer gzr.Close()
		reader = gzr
	}

	bodyBytes, _ := io.ReadAll(reader)

	var responseData struct {
		ResourceResponse struct {
			Data []struct {
				ID     string          `json:"id"`
				Type   string          `json:"type"`
				Images PinterestImages `json:"images"`
				PinterestMedia
				Pinner struct {
					FullName string `json:"full_name"`
					Username string `json:"username"`
				} `json:"pinner"`
			} `json:"data"`
			Bookmark string `json:"bookmark,omitempty"`
		} `json:"resource_response"`
	}

	if err := json.Unmarshal(bodyBytes, &responseData); err != nil {
		return nil, "", newToken
	}

	if responseData.ResourceResponse.Data == nil { return nil, "", newToken }
	pins := make([]Pin, 0)
	for _, result := range responseData.ResourceResponse.Data {
		if result.Type != "pin" || result.ID == "" {
			continue
		}

		bestImage := bestPinImage(result.Images, result.PinterestMedia)
		if bestImage != nil {
			pins = append(pins, Pin{
				ID:          result.ID,
				VideoURL:    videoProxyURL(result.PinterestMedia),
				ImageURL:    fmt.Sprintf("/image?url=%s", url.QueryEscape(bestImage.URL)),
				ImageWidth:  bestImage.Width,
				ImageHeight: bestImage.Height,
				PinnerName:  strings.TrimSpace(result.Pinner.FullName),
			})
		}
	}

	return pins, cleanBookmark(responseData.ResourceResponse.Bookmark), newToken
}

func fetchCSRFToken() string {
	client := &http.Client{Timeout: 10 * time.Second}
	req, _ := http.NewRequest(http.MethodGet, "https://www.pinterest.com/", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	for _, ck := range resp.Cookies() {
		if ck.Name == "csrftoken" && ck.Value != "" {
			return ck.Value
		}
	}
	return ""
}

func proxyImageHandler(c *gin.Context) {
	proxyMedia(c, false)
}

func isAllowedDomain(urlStr string) bool {
	parsedUrl, err := url.Parse(urlStr)
	if err != nil || parsedUrl.Host == "" || (parsedUrl.Scheme != "https" && parsedUrl.Scheme != "http") || parsedUrl.User != nil {
		return false
	}

	for _, domain := range allowedDomains {
		if parsedUrl.Host == domain || strings.HasSuffix(parsedUrl.Host, "."+domain) {
			return true
		}
	}

	return false
}

var safeClient = &http.Client{
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if !isAllowedDomain(req.URL.String()) {
			return fmt.Errorf("redirect to disallowed domain: %s", req.URL.Host)
		}
		return nil
	},
	Timeout: 10 * time.Second,
}

func cleanBookmark(value string) string {
 if value == "-end-" { return "" }; return value
}

func profileSourceError(failed bool) string { if failed { return "Pinterest could not provide this public feed. Retry to check again; this does not mean the account has no pins." }; return "" }
