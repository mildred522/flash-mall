package handler

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/cloudwego/hertz/pkg/app"
)

func TestLoadLatestMerchantApplication(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(`
SELECT id, merchant_name, contact_phone, status, merchant_id, audit_remark,
       COALESCE(DATE_FORMAT(create_time, '%Y-%m-%d %H:%i:%s'), ''),
       COALESCE(DATE_FORMAT(audit_time, '%Y-%m-%d %H:%i:%s'), '')
FROM merchant_apply
WHERE user_id = ?
ORDER BY id DESC
LIMIT 1`)).
		WithArgs(int64(1001)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "merchant_name", "contact_phone", "status", "merchant_id",
			"audit_remark", "create_time", "audit_time",
		}).AddRow(12, "测试商店", "13800000003", 2, 0, "名称不清晰", "2026-07-13 15:00:00", "2026-07-13 16:00:00"))

	got, err := loadLatestMerchantApplication(context.Background(), db, 1001)
	if err != nil {
		t.Fatal(err)
	}
	if got.Application == nil || got.Application.ApplyID != 12 || got.Application.StatusText != "rejected" {
		t.Fatalf("unexpected application: %#v", got.Application)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestLoadLatestMerchantApplicationReturnsNullWhenMissing(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery("SELECT id, merchant_name, contact_phone, status").
		WithArgs(int64(1001)).
		WillReturnError(sql.ErrNoRows)

	got, err := loadLatestMerchantApplication(context.Background(), db, 1001)
	if err != nil {
		t.Fatal(err)
	}
	if got.Application != nil {
		t.Fatalf("expected null application, got %#v", got.Application)
	}
}

func TestCreateMerchantApplyRejectsAlreadyActiveMerchant(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT COUNT\\(\\*\\)").
		WithArgs(int64(1001)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectRollback()

	_, err = createMerchantApply(context.Background(), db, 1001, MerchantApplyReq{
		MerchantName: "测试商店",
		ContactPhone: "13800000003",
	})
	if !errors.Is(err, errMerchantAlreadyActive) {
		t.Fatalf("expected active merchant conflict, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCreateMerchantApplyReturnsExistingPendingApplication(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT COUNT\\(\\*\\)").
		WithArgs(int64(1001)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery("SELECT id").
		WithArgs(int64(1001)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(33))
	mock.ExpectCommit()

	got, err := createMerchantApply(context.Background(), db, 1001, MerchantApplyReq{
		MerchantName: "新名称不会覆盖待审核记录",
		ContactPhone: "13800000003",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.ApplyID != 33 || got.Status != "pending" {
		t.Fatalf("unexpected response: %#v", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCreateMerchantApplyInsertsAfterRejectedHistory(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT COUNT\\(\\*\\)").
		WithArgs(int64(1001)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery("SELECT id").
		WithArgs(int64(1001)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("INSERT INTO merchant_apply").
		WithArgs(int64(1001), "重新申请商店", "13800000003").
		WillReturnResult(sqlmock.NewResult(44, 1))
	mock.ExpectCommit()

	got, err := createMerchantApply(context.Background(), db, 1001, MerchantApplyReq{
		MerchantName: "重新申请商店",
		ContactPhone: "13800000003",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.ApplyID != 44 || got.Status != "pending" {
		t.Fatalf("unexpected response: %#v", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestAdminMerchantApplicationQueryNormalizesPagination(t *testing.T) {
	c := app.NewContext(0)
	c.Request.SetRequestURI("/?status=0&page=2&page_size=500")
	req, err := adminMerchantApplicationQueryFromRequest(c)
	if err != nil {
		t.Fatal(err)
	}
	if req.Status != 0 || req.Page != 2 || req.PageSize != 100 {
		t.Fatalf("unexpected query: %#v", req)
	}
}

func TestValidateAdminMerchantAuditRequestRequiresRejectionRemark(t *testing.T) {
	err := validateAdminMerchantAuditRequest(adminMerchantApplyAuditReq{ApplyID: 12, Approve: false})
	if err == nil {
		t.Fatal("expected rejection without remark to fail")
	}
	if err := validateAdminMerchantAuditRequest(adminMerchantApplyAuditReq{ApplyID: 12, Approve: true}); err != nil {
		t.Fatalf("approval remark must be optional: %v", err)
	}
}

func TestLoadAdminMerchantApplicationsFiltersAndPaginates(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM merchant_apply WHERE 1=1 AND status = ?")).
		WithArgs(int64(0)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery("SELECT id, user_id, merchant_name").
		WithArgs(int64(0), int64(20), int64(20)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "merchant_name", "contact_phone", "status", "merchant_id",
			"audit_remark", "operator_id", "create_time", "audit_time",
		}).AddRow(12, 1001, "测试商店", "13800000003", 0, 0, "", 0, "2026-07-13 15:00:00", ""))

	got, err := loadAdminMerchantApplications(context.Background(), db, AdminMerchantApplicationListReq{
		Status: 0, Page: 2, PageSize: 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Total != 1 || got.Page != 2 || len(got.Items) != 1 || got.Items[0].StatusText != "pending" {
		t.Fatalf("unexpected response: %#v", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
