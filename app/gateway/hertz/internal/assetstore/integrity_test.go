package assetstore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

type integrityAuditStub struct {
	report IntegrityReport
	err    error
	calls  int
}

type referenceSourceStub struct {
	references []string
	err        error
	calls      *int
}

func (s referenceSourceStub) References(context.Context) ([]string, error) {
	if s.calls != nil {
		*s.calls++
	}
	return s.references, s.err
}

func (s *integrityAuditStub) Check(context.Context) (IntegrityReport, error) {
	s.calls++
	return s.report, s.err
}

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

func TestIntegrityCheckerReportsInvalidAndCorruptReferences(t *testing.T) {
	t.Parallel()
	productDB, productMock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer productDB.Close()

	root := t.TempDir()
	content := []byte("expected image")
	sum := sha256.Sum256(content)
	digest := hex.EncodeToString(sum[:])
	if err := os.MkdirAll(filepath.Join(root, "products"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "products", digest+".png"), []byte("corrupt image"), 0o644); err != nil {
		t.Fatal(err)
	}
	productMock.ExpectQuery("SELECT image_url FROM product").
		WillReturnRows(sqlmock.NewRows([]string{"image_url"}).
			AddRow("/uploads/products/" + digest + ".png").
			AddRow("/uploads/../secret"))

	report, err := NewIntegrityChecker(root, productDB, nil).Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if report.CorruptFiles != 1 || report.InvalidReferences != 1 {
		t.Fatalf("unexpected report: %+v", report)
	}
	if report.Healthy() {
		t.Fatal("corrupt and invalid references must degrade integrity")
	}
}

func TestIntegrityMonitorCachesAuditResult(t *testing.T) {
	t.Parallel()
	audit := &integrityAuditStub{report: IntegrityReport{
		Configured:   true,
		Writable:     true,
		MissingFiles: 1,
	}}
	monitor := NewIntegrityMonitor(audit, time.Hour)

	monitor.RunOnce(context.Background())
	first := monitor.Snapshot()
	second := monitor.Snapshot()

	if audit.calls != 1 {
		t.Fatalf("audit calls=%d, want 1", audit.calls)
	}
	if first.CheckedAt.IsZero() || second.Report.MissingFiles != 1 || !second.Ready() {
		t.Fatalf("unexpected snapshot: %+v", second)
	}
	if !second.Degraded() {
		t.Fatal("missing historical assets should degrade integrity without failing readiness")
	}
}

func TestIntegrityCheckerAcceptsIndependentReferenceSource(t *testing.T) {
	t.Parallel()
	checker := NewIntegrityCheckerWithSource(t.TempDir(), referenceSourceStub{
		references: []string{"/uploads/products/missing.png"},
	})
	report, err := checker.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if report.ReferencedFiles != 1 || report.MissingFiles != 1 {
		t.Fatalf("unexpected report: %+v", report)
	}
}

func TestIntegrityMonitorPrimeChecksStorageWithoutScanningReferences(t *testing.T) {
	t.Parallel()
	referenceCalls := 0
	checker := NewIntegrityCheckerWithSource(t.TempDir(), referenceSourceStub{
		references: []string{"/uploads/products/missing.png"},
		calls:      &referenceCalls,
	})
	monitor := NewIntegrityMonitor(checker, time.Hour)

	monitor.Prime(context.Background())

	if referenceCalls != 0 {
		t.Fatalf("prime scanned %d reference batches", referenceCalls)
	}
	if snapshot := monitor.Snapshot(); !snapshot.Ready() || snapshot.Degraded() {
		t.Fatalf("unexpected primed snapshot: %+v", snapshot)
	}
}
