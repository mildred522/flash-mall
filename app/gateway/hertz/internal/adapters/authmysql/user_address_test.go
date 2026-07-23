package authmysql

import (
	"context"
	"testing"

	"flash-mall/app/gateway/hertz/internal/application/useraddress"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestUserAddressListScopesByUser(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	mock.ExpectQuery("SELECT id, receiver_name, receiver_phone").WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "phone", "province", "city", "district", "detail", "is_default",
		}).AddRow(8, "收件人", "13800000003", "浙江", "杭州", "西湖", "文三路 1 号", 1))

	items, err := NewUserAddressRepository(db).List(context.Background(), 7)
	if err != nil || len(items) != 1 || !items[0].IsDefault || items[0].AddressID != 8 {
		t.Fatalf("items = %+v, err = %v", items, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestUserAddressUpsertChangesDefaultAtomically(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE mall_auth.user_address SET is_default = 0 WHERE user_id = \\?").
		WithArgs(int64(7)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE mall_auth.user_address SET receiver_name=\\?").
		WithArgs("收件人", "13800000003", "浙江", "杭州", "西湖", "文三路 1 号", int64(1), int64(8), int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	result, err := NewUserAddressRepository(db).Upsert(context.Background(), useraddress.UpsertInput{
		AddressID: 8, UserID: 7, ReceiverName: "收件人", ReceiverPhone: "13800000003",
		Province: "浙江", City: "杭州", District: "西湖", Detail: "文三路 1 号", IsDefault: true,
	})
	if err != nil || !result.Found || result.AddressID != 8 {
		t.Fatalf("result = %+v, err = %v", result, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestUserAddressUpsertRollsBackWhenOwnedAddressMissing(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE mall_auth.user_address SET is_default = 0 WHERE user_id = \\?").
		WithArgs(int64(7)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE mall_auth.user_address SET receiver_name=\\?").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	result, err := NewUserAddressRepository(db).Upsert(context.Background(), useraddress.UpsertInput{
		AddressID: 99, UserID: 7, ReceiverName: "收件人", ReceiverPhone: "13800000003", Detail: "文三路 1 号", IsDefault: true,
	})
	if err != nil || result.Found {
		t.Fatalf("result = %+v, err = %v", result, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
