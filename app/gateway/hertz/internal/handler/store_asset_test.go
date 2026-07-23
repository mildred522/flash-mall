package handler

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"mime/multipart"
	"path/filepath"
	"strings"
	"testing"

	"flash-mall/app/common/authctx"
	"flash-mall/app/gateway/hertz/internal/adapters/ordermysql"
	"flash-mall/app/gateway/hertz/internal/application/merchantquery"
	"flash-mall/app/gateway/hertz/internal/assetstore"
	"flash-mall/app/gateway/hertz/internal/config"
	"flash-mall/app/gateway/hertz/internal/svc"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func TestProductImageExtRequiresMatchingDetectedType(t *testing.T) {
	cases := []struct {
		contentType string
		filename    string
		want        string
	}{
		{"image/jpeg", "photo.jpeg", ".jpg"},
		{"image/png", "photo.png", ".png"},
		{"image/webp", "photo.webp", ".webp"},
		{"image/gif", "photo.gif", ".gif"},
		{"image/svg+xml", "fake.png", ""},
		{"text/plain", "fake.webp", ""},
		{"image/png", "fake.jpg", ""},
	}
	for _, tc := range cases {
		if got := productImageExt(tc.contentType, filepath.Ext(tc.filename)); got != tc.want {
			t.Errorf("productImageExt(%q, %q)=%q want %q", tc.contentType, tc.filename, got, tc.want)
		}
	}
}

func TestValidateStoreAssetType(t *testing.T) {
	for _, assetType := range []string{"logo", "banner"} {
		if err := validateStoreAssetType(assetType); err != nil {
			t.Fatalf("%s rejected: %v", assetType, err)
		}
	}
	for _, assetType := range []string{"", "icon", "../logo"} {
		if err := validateStoreAssetType(assetType); err == nil {
			t.Fatalf("%q should be rejected", assetType)
		}
	}
}

func TestSafeUploadedAssetPathKeepsRequestsInsideRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "stores")
	path, err := safeUploadedAssetPath(root, "1000/logo-a1.png")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(path, filepath.Clean(root)+string(filepath.Separator)) {
		t.Fatalf("path escaped root: %s", path)
	}
	for _, requested := range []string{"../secret", "1000/../../secret", "/absolute/file.png", "1000"} {
		if _, err := safeUploadedAssetPath(root, requested); err == nil {
			t.Fatalf("%q should be rejected", requested)
		}
	}
}

func TestMerchantStoreAssetUploadRequiresAuthentication(t *testing.T) {
	h := server.Default()
	registerMerchantRoutes(h, &svc.ServiceContext{Config: config.Config{JwtAuthSecret: "jwt-secret"}})

	resp := ut.PerformRequest(h.Engine, "POST", "/api/merchant/store/assets", nil).Result()
	if resp.StatusCode() != consts.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", resp.StatusCode(), resp.Body())
	}
}

func TestMerchantStoreAssetUploadStoresImageUnderMerchantDirectory(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery("SELECT COUNT\\(\\*\\).*merchant_user").
		WithArgs(int64(1001), int64(1000)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("asset_type", "logo"); err != nil {
		t.Fatal(err)
	}
	part, err := writer.CreateFormFile("image", "logo.png")
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte("\x89PNG\r\n\x1a\n")
	if _, err := part.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	svcCtx := &svc.ServiceContext{
		Config:          config.Config{UploadDir: t.TempDir()},
		AssetStore:      assetstore.NewFilesystem(t.TempDir()),
		MerchantQueries: merchantquery.NewService(ordermysql.NewMerchantProfileRepository(db)),
	}
	h := server.Default()
	h.POST("/upload", func(ctx context.Context, c *app.RequestContext) {
		identityCtx := authctx.WithIdentity(ctx, authctx.Identity{
			UserID: 1001, Role: authctx.RoleMerchant, MerchantID: 1000,
		})
		MerchantStoreAssetUploadHandler(svcCtx)(identityCtx, c)
	})
	resp := ut.PerformRequest(h.Engine, "POST", "/upload", &ut.Body{Body: &body, Len: body.Len()},
		ut.Header{Key: "Content-Type", Value: writer.FormDataContentType()}).Result()
	if resp.StatusCode() != consts.StatusOK {
		t.Fatalf("status=%d body=%s", resp.StatusCode(), resp.Body())
	}
	expectedURL := fmt.Sprintf("/uploads/stores/1000/%x.png", sha256.Sum256(payload))
	if !strings.Contains(string(resp.Body()), expectedURL) {
		t.Fatalf("unexpected body: %s", resp.Body())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestStoreAssetRoutesAreRegistered(t *testing.T) {
	h := server.Default()
	registerShopRoutes(h, &svc.ServiceContext{})
	registerMerchantRoutes(h, &svc.ServiceContext{})
	want := map[string]bool{
		"GET /uploads/stores/*any":         false,
		"GET /api/merchant/store/profile":  false,
		"POST /api/merchant/store/profile": false,
		"POST /api/merchant/store/assets":  false,
	}
	for _, route := range h.Routes() {
		if _, ok := want[route.Method+" "+route.Path]; ok {
			want[route.Method+" "+route.Path] = true
		}
	}
	for route, found := range want {
		if !found {
			t.Errorf("route missing: %s", route)
		}
	}
}
