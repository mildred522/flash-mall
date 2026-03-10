package handler

import (
	"embed"
	"net/http"
)

//go:embed web/*.html
var webUIFS embed.FS

func HomeUIHandler() http.HandlerFunc {
	return serveEmbeddedHTML("web/home.html")
}

func ShopUIHandler() http.HandlerFunc {
	return serveEmbeddedHTML("web/shop.html")
}

func DebugUIHandler() http.HandlerFunc {
	return serveEmbeddedHTML("web/debug.html")
}

func serveEmbeddedHTML(path string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		content, err := webUIFS.ReadFile(path)
		if err != nil {
			http.Error(w, "ui not found", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(content)
	}
}
