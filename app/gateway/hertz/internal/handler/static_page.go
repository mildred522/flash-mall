package handler

import (
	"context"
	"errors"
	"os"
	"path"
	"path/filepath"
	"strings"

	"flash-mall/app/common/apperror"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

const staticWebRoot = "/app/web"

func StaticPageHandler(name string) app.HandlerFunc {
	return func(_ context.Context, c *app.RequestContext) {
		c.File(filepath.Join(staticWebRoot, name))
	}
}

func StaticAssetHandler(directory string) app.HandlerFunc {
	root := filepath.Join(staticWebRoot, directory)
	return func(ctx context.Context, c *app.RequestContext) {
		assetPath, err := staticAssetPath(root, c.Param("any"))
		if err != nil {
			fail(ctx, c, consts.StatusNotFound, apperror.New(apperror.CodeNotFound, "asset not found"))
			return
		}
		if _, err := os.Stat(assetPath); err != nil {
			fail(ctx, c, consts.StatusNotFound, apperror.New(apperror.CodeNotFound, "asset not found"))
			return
		}
		c.File(assetPath)
	}
}

func staticAssetPath(root, raw string) (string, error) {
	raw = strings.TrimSpace(strings.ReplaceAll(raw, "\\", "/"))
	if raw == "" {
		return "", errors.New("empty asset path")
	}
	for _, segment := range strings.Split(raw, "/") {
		if segment == ".." {
			return "", errors.New("asset traversal is forbidden")
		}
	}
	relative := strings.TrimPrefix(path.Clean("/"+raw), "/")
	if relative == "" || relative == "." {
		return "", errors.New("empty asset path")
	}
	cleanRoot := filepath.Clean(root)
	candidate := filepath.Clean(filepath.Join(cleanRoot, filepath.FromSlash(relative)))
	if !strings.HasPrefix(candidate, cleanRoot+string(filepath.Separator)) {
		return "", errors.New("asset path escapes root")
	}
	return candidate, nil
}
