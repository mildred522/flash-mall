package handler

import (
	"embed"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Rebuild this package when embedded storefront copy changes.
//
//go:embed web/*.html
var webUIFS embed.FS

func HomeUIHandler() http.HandlerFunc {
	return serveHTML("web/index.html", "web/home.html")
}

func ShopUIHandler() http.HandlerFunc {
	return serveHTML("web/shop.html", "web/shop.html")
}

func DebugUIHandler() http.HandlerFunc {
	return serveHTML("web/debug.html", "web/debug.html")
}

func AdminUIHandler() http.HandlerFunc {
	return serveHTML("web/admin.html", "web/admin.html")
}

func StaticWebAssetHandler(prefix string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rel := strings.TrimPrefix(r.URL.Path, prefix)
		rel = strings.TrimPrefix(filepath.Clean("/"+rel), string(filepath.Separator))
		if rel == "." || strings.HasPrefix(rel, "..") {
			http.NotFound(w, r)
			return
		}
		path := filepath.Join("web", strings.Trim(prefix, "/"), rel)
		if _, err := os.Stat(path); err != nil {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, path)
	}
}

func serveHTML(diskPath string, embeddedPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if content, err := os.ReadFile(diskPath); err == nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(content)
			return
		}
		content, err := webUIFS.ReadFile(embeddedPath)
		if err != nil {
			http.Error(w, "ui not found", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(content)
	}
}
