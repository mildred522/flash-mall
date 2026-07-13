package handler

import (
	"context"
	"regexp"
	"testing"

	"flash-mall/app/common/authctx"
	"flash-mall/app/gateway/hertz/internal/svc"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func TestSelectedMerchantIDFromServiceUsesOrderDatasource(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(`
SELECT merchant_id
FROM merchant_user
WHERE user_id = ? AND status = 1
ORDER BY id ASC
LIMIT 1`)).
		WithArgs(int64(1001)).
		WillReturnRows(sqlmock.NewRows([]string{"merchant_id"}).AddRow(int64(1000)))

	svcCtx := &svc.ServiceContext{OrderSqlConn: sqlx.NewSqlConnFromDB(db)}
	merchantID, err := selectedMerchantIDFromService(
		context.Background(),
		svcCtx,
		authctx.Identity{UserID: 1001, Role: authctx.RoleUser},
	)

	if err != nil {
		t.Fatalf("selectedMerchantIDFromService: %v", err)
	}
	if merchantID != 1000 {
		t.Fatalf("merchant id = %d, want 1000", merchantID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("order datasource expectations: %v", err)
	}
}
