package ordermysql

import (
	"context"
	"database/sql"
	"regexp"
	"testing"

	"flash-mall/app/gateway/hertz/internal/application/merchantonboarding"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestMerchantOnboardingSubmitReturnsExistingPendingApplication(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id, status FROM merchant_apply").WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "status"}).AddRow(33, merchantonboarding.StatusPending))
	mock.ExpectQuery("SELECT mu.merchant_id").WithArgs(int64(9)).WillReturnError(sql.ErrNoRows)
	mock.ExpectCommit()

	result, err := NewMerchantOnboardingRepository(db).Submit(context.Background(), merchantonboarding.SubmitInput{
		UserID: 9, MerchantName: "不会覆盖", ContactPhone: "13800000003",
	})
	if err != nil || result.ApplyID != 33 || result.Status != merchantonboarding.StatusPending {
		t.Fatalf("result = %+v, err = %v", result, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMerchantOnboardingAuditApprovalIsAtomic(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT user_id, merchant_name, contact_phone, status, merchant_id FROM merchant_apply").
		WithArgs(int64(12)).WillReturnRows(sqlmock.NewRows([]string{
		"user_id", "merchant_name", "contact_phone", "status", "merchant_id",
	}).AddRow(9, "山岚商店", "13800000003", merchantonboarding.StatusPending, 0))
	mock.ExpectExec("INSERT INTO merchant").WithArgs("山岚商店", int64(9), "13800000003").
		WillReturnResult(sqlmock.NewResult(71, 1))
	mock.ExpectExec("INSERT INTO merchant_user").WithArgs(int64(71), int64(9)).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("UPDATE merchant_apply SET status=\\?,merchant_id=\\?,audit_remark=\\?,operator_id=\\?,audit_time=NOW\\(\\) WHERE id=\\? AND status=0").
		WithArgs(merchantonboarding.StatusApproved, int64(71), "资料齐全", int64(2), int64(12)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	result, err := NewMerchantOnboardingRepository(db).Audit(context.Background(), merchantonboarding.AuditInput{
		ApplyID: 12, Approve: true, Remark: "资料齐全", OperatorID: 2,
	})
	if err != nil || result.MerchantID != 71 || result.Status != merchantonboarding.StatusApproved || result.Idempotent {
		t.Fatalf("result = %+v, err = %v", result, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMerchantOnboardingAuditRepeatsSameDecisionIdempotently(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT user_id, merchant_name, contact_phone, status, merchant_id FROM merchant_apply").
		WithArgs(int64(12)).WillReturnRows(sqlmock.NewRows([]string{
		"user_id", "merchant_name", "contact_phone", "status", "merchant_id",
	}).AddRow(9, "山岚商店", "13800000003", merchantonboarding.StatusApproved, 71))
	mock.ExpectCommit()

	result, err := NewMerchantOnboardingRepository(db).Audit(context.Background(), merchantonboarding.AuditInput{
		ApplyID: 12, Approve: true, OperatorID: 2,
	})
	if err != nil || result.MerchantID != 71 || !result.Idempotent || result.Rejection != "" {
		t.Fatalf("result = %+v, err = %v", result, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMerchantOnboardingAuditRejectsOppositeDecision(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT user_id, merchant_name, contact_phone, status, merchant_id FROM merchant_apply").
		WithArgs(int64(12)).WillReturnRows(sqlmock.NewRows([]string{
		"user_id", "merchant_name", "contact_phone", "status", "merchant_id",
	}).AddRow(9, "山岚商店", "13800000003", merchantonboarding.StatusRejected, 0))
	mock.ExpectRollback()

	result, err := NewMerchantOnboardingRepository(db).Audit(context.Background(), merchantonboarding.AuditInput{
		ApplyID: 12, Approve: true, OperatorID: 2,
	})
	if err != nil || result.Rejection != merchantonboarding.RejectAlreadyAudited {
		t.Fatalf("result = %+v, err = %v", result, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMerchantOnboardingListFiltersAndPaginates(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM merchant_apply WHERE 1=1 AND status = ?")).
		WithArgs(merchantonboarding.StatusPending).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery("SELECT id, user_id, merchant_name").
		WithArgs(merchantonboarding.StatusPending, int64(20), int64(20)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "merchant_name", "contact_phone", "status", "merchant_id",
			"audit_remark", "operator_id", "create_time", "audit_time",
		}).AddRow(12, 1001, "测试商店", "13800000003", 0, 0, "", 0, "2026-07-13 15:00:00", ""))

	result, err := NewMerchantOnboardingRepository(db).List(context.Background(), merchantonboarding.ListQuery{
		Status: merchantonboarding.StatusPending, Page: 2, PageSize: 20,
	})
	if err != nil || result.Total != 1 || len(result.Items) != 1 || result.Items[0].ApplyID != 12 {
		t.Fatalf("result = %+v, err = %v", result, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
