package main

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

var opaqueStatusID = regexp.MustCompile(`^e1_[A-Za-z0-9_-]{1,240}$`)
var pinStatuses = newResolverCache[string](2048)
var statusAdmission = struct {
	sync.Mutex
	next   time.Time
	active int
}{}

func inspectPinStatus(id string) string {
	if strings.HasPrefix(id, "e1_") {
		id = resolveOpaquePin(id)
		if id == "" {
			return "unresolved"
		}
	}
	client := newPinterestClient()
	client.Timeout = 5 * time.Second
	pins, next, count := fetchRelatedPinsWithClient(id, "", "", client, 1)
	if pins == nil {
		return "unknown"
	}
	if count > 0 {
		return "available"
	}
	if next == "" {
		return "empty"
	}
	return "unknown"
}

func pinStatusHandler(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	id := c.Query("id")
	if !numericPinID.MatchString(id) && !opaqueStatusID.MatchString(id) {
		c.JSON(http.StatusBadRequest, gin.H{"state": "unknown"})
		return
	}
	_, generation := pinterestSession.snapshot()
	key := fmt.Sprintf("%d:%s", generation, id)
	pinStatuses.Lock()
	cached, ok := pinStatuses.items[key]
	pinStatuses.Unlock()
	if ok && time.Now().Before(cached.expires) {
		c.JSON(http.StatusOK, gin.H{"state": cached.value})
		return
	}
	// Background wall checks must not flood Pinterest across tabs/windows.
	statusAdmission.Lock()
	if statusAdmission.active >= 2 || time.Now().Before(statusAdmission.next) {
		statusAdmission.Unlock()
		c.Header("Retry-After", "5")
		c.JSON(http.StatusTooManyRequests, gin.H{"state": "unknown"})
		return
	}
	statusAdmission.active++
	statusAdmission.next = time.Now().Add(time.Second)
	statusAdmission.Unlock()
	defer func() { statusAdmission.Lock(); statusAdmission.active--; statusAdmission.Unlock() }()
	state := pinStatuses.load(key, func() (string, bool) { return inspectPinStatus(id), false })
	if state == "" {
		state = "unknown"
	}
	c.JSON(http.StatusOK, gin.H{"state": state})
}
