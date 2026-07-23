package handler

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func HomeUIHandler() http.HandlerFunc {
	return serveHTML("shop.html")
}

func ShopUIHandler() http.HandlerFunc {
	return serveHTML("shop.html")
}

func DebugUIHandler() http.HandlerFunc {
	return serveHTML("shop.html")
}

func AdminUIHandler() http.HandlerFunc {
	return serveHTML("admin.html")
}

func StaticWebAssetHandler(prefix string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rel := strings.TrimPrefix(r.URL.Path, prefix)
		rel = strings.TrimPrefix(filepath.Clean("/"+rel), string(filepath.Separator))
		if rel == "." || strings.HasPrefix(rel, "..") {
			http.NotFound(w, r)
			return
		}
		path := filepath.Join(resolveWebRoot(), strings.Trim(prefix, "/"), rel)
		if _, err := os.Stat(path); err != nil {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, path)
	}
}

func serveHTML(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		content, err := os.ReadFile(filepath.Join(resolveWebRoot(), filepath.Base(name)))
		if err != nil {
			http.Error(w, "ui not found", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(content)
	}
}

func resolveWebRoot() string {
	if configured := strings.TrimSpace(os.Getenv("FLASH_MALL_WEB_ROOT")); configured != "" {
		return configured
	}
	for _, candidate := range []string{"/app/web", "artifacts/web", "../../../../../artifacts/web"} {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}
	return "/app/web"
}
