package handler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"flash-mall/app/common/apperror"
	"flash-mall/app/common/authctx"
	"flash-mall/app/gateway/hertz/internal/svc"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

const maxGatewayProductImageBytes = 5 << 20

func AdminProductImageUploadHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return productImageUploadHandler(svcCtx)
}

func MerchantProductImageUploadHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		identity, ok := authctx.IdentityFrom(ctx)
		if !ok || identity.UserID <= 0 {
			fail(ctx, c, consts.StatusUnauthorized, apperror.New(apperror.CodeUnauthorized, "merchant login required"))
			return
		}
		if _, err := selectedMerchantIDFromService(ctx, svcCtx, identity); err != nil {
			fail(ctx, c, consts.StatusForbidden, err)
			return
		}
		productImageUploadHandler(svcCtx)(ctx, c)
	}
}

func productImageUploadHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		fileHeader, err := c.FormFile("image")
		if err != nil {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "image file required"))
			return
		}
		if fileHeader.Size > maxGatewayProductImageBytes {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "image upload must be <= 5MB"))
			return
		}

		file, err := fileHeader.Open()
		if err != nil {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "image file cannot be read"))
			return
		}
		defer func() { _ = file.Close() }()

		head := make([]byte, 512)
		n, _ := io.ReadFull(file, head)
		head = head[:n]
		if seeker, ok := file.(io.Seeker); ok {
			_, _ = seeker.Seek(0, io.SeekStart)
		} else {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "image file cannot be read"))
			return
		}

		ext := productImageExt(http.DetectContentType(head), filepath.Ext(fileHeader.Filename))
		if ext == "" {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "only jpg, png, webp and gif images are allowed"))
			return
		}

		dir := filepath.Join(productUploadDir(svcCtx), "products")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "product image upload failed", err))
			return
		}

		name := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)
		dstPath := filepath.Join(dir, name)
		dst, err := os.OpenFile(dstPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "product image upload failed", err))
			return
		}
		defer func() { _ = dst.Close() }()

		if _, err := io.Copy(dst, io.LimitReader(file, maxGatewayProductImageBytes+1)); err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "product image upload failed", err))
			return
		}
		ok(ctx, c, map[string]any{"image_url": "/uploads/products/" + name})
	}
}

func ProductUploadStaticHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	root := filepath.Join(productUploadDir(svcCtx), "products")
	return func(ctx context.Context, c *app.RequestContext) {
		name := filepath.Base(strings.TrimSpace(c.Param("any")))
		if name == "." || name == string(filepath.Separator) || name == "" {
			fail(ctx, c, consts.StatusNotFound, apperror.New(apperror.CodeNotFound, "image not found"))
			return
		}
		path := filepath.Join(root, name)
		if !strings.HasPrefix(filepath.Clean(path), filepath.Clean(root)+string(filepath.Separator)) {
			fail(ctx, c, consts.StatusForbidden, apperror.New(apperror.CodeForbidden, "invalid image path"))
			return
		}
		if _, err := os.Stat(path); err != nil {
			fail(ctx, c, consts.StatusNotFound, apperror.New(apperror.CodeNotFound, "image not found"))
			return
		}
		c.File(path)
	}
}

func productUploadDir(svcCtx *svc.ServiceContext) string {
	if dir := strings.TrimSpace(svcCtx.Config.UploadDir); dir != "" {
		return dir
	}
	return filepath.Join(".runtime", "uploads")
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
