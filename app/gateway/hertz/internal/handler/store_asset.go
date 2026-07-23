package handler

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"flash-mall/app/common/apperror"
	"flash-mall/app/common/authctx"
	"flash-mall/app/gateway/hertz/internal/assetstore"
	"flash-mall/app/gateway/hertz/internal/svc"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func MerchantStoreAssetUploadHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		identity, hasIdentity := authctx.IdentityFrom(ctx)
		if !hasIdentity || identity.UserID <= 0 {
			fail(ctx, c, consts.StatusUnauthorized, apperror.New(apperror.CodeUnauthorized, "merchant login required"))
			return
		}
		assetType := strings.TrimSpace(string(c.FormValue("asset_type")))
		if err := validateStoreAssetType(assetType); err != nil {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, err.Error()))
			return
		}
		merchantID, err := selectedMerchantIDFromService(ctx, svcCtx, identity)
		if err != nil {
			fail(ctx, c, consts.StatusForbidden, err)
			return
		}
		fileHeader, err := c.FormFile("image")
		if err != nil || fileHeader.Size <= 0 {
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
		ext := productImageExt(http.DetectContentType(head[:n]), filepath.Ext(fileHeader.Filename))
		if ext == "" {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "only matching jpg, png, webp and gif images are allowed"))
			return
		}
		if _, err = file.Seek(0, io.SeekStart); err != nil {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "image file cannot be read"))
			return
		}
		namespace := "stores/" + strconv.FormatInt(merchantID, 10)
		asset, err := assetstore.NewFilesystem(productUploadDir(svcCtx)).Save(
			namespace,
			ext,
			file,
			maxGatewayProductImageBytes,
		)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "store image upload failed", err))
			return
		}
		ok(ctx, c, map[string]any{"image_url": asset.URL, "asset_type": assetType})
	}
}

func validateStoreAssetType(assetType string) error {
	if assetType != "logo" && assetType != "banner" {
		return errors.New("asset_type must be logo or banner")
	}
	return nil
}

func safeUploadedAssetPath(root, requested string) (string, error) {
	if requested == "" || filepath.IsAbs(requested) || strings.Contains(requested, "\\") {
		return "", errors.New("invalid asset path")
	}
	parts := strings.Split(requested, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" || filepath.Base(parts[1]) != parts[1] {
		return "", errors.New("invalid asset path")
	}
	if merchantID, err := strconv.ParseInt(parts[0], 10, 64); err != nil || merchantID <= 0 {
		return "", errors.New("invalid merchant path")
	}
	cleanRoot := filepath.Clean(root)
	path := filepath.Clean(filepath.Join(cleanRoot, filepath.FromSlash(requested)))
	if !strings.HasPrefix(path, cleanRoot+string(filepath.Separator)) {
		return "", errors.New("asset path escapes root")
	}
	return path, nil
}

func StoreUploadStaticHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	root := filepath.Join(productUploadDir(svcCtx), "stores")
	return func(ctx context.Context, c *app.RequestContext) {
		requested := strings.TrimPrefix(strings.TrimSpace(c.Param("any")), "/")
		path, err := safeUploadedAssetPath(root, requested)
		if err != nil {
			fail(ctx, c, consts.StatusForbidden, apperror.New(apperror.CodeForbidden, "invalid image path"))
			return
		}
		if _, err = os.Stat(path); err != nil {
			fail(ctx, c, consts.StatusNotFound, apperror.New(apperror.CodeNotFound, "image not found"))
			return
		}
		c.Response.Header.Set("Cache-Control", "public, max-age=31536000, immutable")
		c.File(path)
	}
}
