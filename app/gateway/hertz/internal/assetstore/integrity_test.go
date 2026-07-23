package assetstore

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestIntegrityCheckerReportsMissingReferences(t *testing.T) {
	t.Parallel()
	productDB, productMock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer productDB.Close()
	orderDB, orderMock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer orderDB.Close()

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "products"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "products", "present.webp"), []byte("image"), 0o644); err != nil {
		t.Fatal(err)
	}
	productMock.ExpectQuery("SELECT image_url FROM product").
		WillReturnRows(sqlmock.NewRows([]string{"image_url"}).
			AddRow("/uploads/products/present.webp").
			AddRow("/uploads/products/missing.webp"))
	orderMock.ExpectQuery("SELECT product_image_url FROM order_price_snapshot").
		WillReturnRows(sqlmock.NewRows([]string{"product_image_url"}).
			AddRow("/uploads/products/present.webp"))

	report, err := NewIntegrityChecker(root, productDB, orderDB).Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if report.ReferencedFiles != 2 || report.MissingFiles != 1 {
		t.Fatalf("unexpected report: %+v", report)
	}
	if report.Healthy() {
		t.Fatal("report with a missing file must not be healthy")
	}
}

func TestIntegrityCheckerRejectsTraversalReference(t *testing.T) {
	t.Parallel()
	checker := NewIntegrityChecker(t.TempDir(), nil, nil)
	if _, ok := checker.referencePath("/uploads/../secret"); ok {
		t.Fatal("traversal reference must not resolve")
	}
}
