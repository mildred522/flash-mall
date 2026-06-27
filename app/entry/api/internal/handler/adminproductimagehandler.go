package handler

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"flash-mall/app/entry/api/internal/svc"

	"github.com/zeromicro/go-zero/rest/httpx"
)

const maxProductImageBytes = 5 << 20

func AdminProductImageUploadHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxProductImageBytes+1024)
		if err := r.ParseMultipartForm(maxProductImageBytes); err != nil {
			writeBadRequest(w, "image upload must be <= 5MB")
			return
		}
		file, header, err := r.FormFile("image")
		if err != nil {
			writeBadRequest(w, "image file required")
			return
		}
		defer func() { _ = file.Close() }()

		head := make([]byte, 512)
		n, _ := io.ReadFull(file, head)
		head = head[:n]
		if seeker, ok := file.(io.Seeker); ok {
			_, _ = seeker.Seek(0, io.SeekStart)
		} else {
			writeBadRequest(w, "image file cannot be read")
			return
		}

		contentType := http.DetectContentType(head)
		ext := productImageExt(contentType, filepath.Ext(header.Filename))
		if ext == "" {
			writeBadRequest(w, "only jpg, png, webp and gif images are allowed")
			return
		}

		dir := filepath.Join(productUploadDir(svcCtx), "products")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		name := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)
		dstPath := filepath.Join(dir, name)
		dst, err := os.OpenFile(dstPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		defer func() { _ = dst.Close() }()

		if _, err := io.Copy(dst, io.LimitReader(file, maxProductImageBytes+1)); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, map[string]any{
			"image_url": "/uploads/products/" + name,
		})
	}
}

func ProductUploadStaticHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	root := http.Dir(productUploadDir(svcCtx))
	fs := http.StripPrefix("/uploads/", http.FileServer(root))
	return func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/uploads/products/") {
			http.NotFound(w, r)
			return
		}
		fs.ServeHTTP(w, r)
	}
}

func productImageExt(contentType, originalExt string) string {
	switch contentType {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	}
	switch strings.ToLower(originalExt) {
	case ".jpg", ".jpeg":
		return ".jpg"
	case ".png", ".webp", ".gif":
		return strings.ToLower(originalExt)
	default:
		return ""
	}
}
