package main

import (
	"crypto/sha256"
	"fmt"
	"html/template"
	"io/fs"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
)

// Hash embedded bytes, rather than timestamps or manually maintained query tags.
func interfaceBuild(assets fs.FS) (string, error) {
	hash := sha256.New()
	err := fs.WalkDir(assets, ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := fs.ReadFile(assets, name)
		if err != nil {
			return err
		}
		fmt.Fprintf(hash, "%s\x00", name)
		hash.Write(data)
		hash.Write([]byte{0})
		return nil
	})
	return fmt.Sprintf("%x", hash.Sum(nil)), err
}

func loadInterfaceTemplates(pages, assets fs.FS) (*template.Template, error) {
	build, err := interfaceBuild(assets)
	if err != nil {
		return nil, err
	}
	entries, err := fs.ReadDir(pages, "templates")
	if err != nil {
		return nil, err
	}
	refs := regexp.MustCompile(`(src|href)="(/static/[^"?]+)(?:\?[^\"]*)?"`)
	result := template.New("")
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		content, err := fs.ReadFile(pages, "templates/"+entry.Name())
		if err != nil {
			return nil, err
		}
		text := refs.ReplaceAllString(string(content), `${1}="${2}?v=`+build+`"`)
		if _, err := result.New(entry.Name()).Parse(text); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func interfaceCachePolicy() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		switch {
		case strings.HasPrefix(path, "/static/"):
			c.Header("Cache-Control", "no-cache")
		case path != "/image" && path != "/video" && path != "/hls":
			c.Header("Cache-Control", "no-store")
		}
		c.Next()
	}
}
