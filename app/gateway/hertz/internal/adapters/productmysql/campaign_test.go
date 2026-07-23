package productmysql

import (
	"context"
	"regexp"
	"testing"

	"flash-mall/app/gateway/hertz/internal/application/campaign"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestCampaignListScansRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery("SELECT c.id").WillReturnRows(sqlmock.NewRows([]string{
		"id", "product_id", "product_name", "name", "campaign_stock", "per_user_limit", "starts_at", "ends_at", "status",
	}).AddRow(3, 8, "咖啡", "闪购", 20, 2, "2026-07-22 10:00:00", "2026-07-22 12:00:00", 1))

	items, err := NewCampaignRepository(db).List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].CampaignID != 3 || items[0].ProductName != "咖啡" {
		t.Fatalf("items=%+v", items)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCampaignUpsertInsertsAndReturnsID(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO mall_product.seckill_campaign")).
		WithArgs(int64(8), "闪购", int64(20), int64(2), "", "", int64(1)).
		WillReturnResult(sqlmock.NewResult(13, 1))

	id, err := NewCampaignRepository(db).Upsert(context.Background(), campaign.UpsertInput{
		ProductID: 8, Name: "闪购", CampaignStock: 20, PerUserLimit: 2, Status: 1,
	})
	if err != nil || id != 13 {
		t.Fatalf("id=%d err=%v", id, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCampaignUpsertUpdatesExistingID(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectExec(regexp.QuoteMeta("UPDATE mall_product.seckill_campaign")).
		WithArgs(int64(8), "闪购", int64(20), int64(2), "", "", int64(1), int64(9)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	id, err := NewCampaignRepository(db).Upsert(context.Background(), campaign.UpsertInput{
		CampaignID: 9, ProductID: 8, Name: "闪购", CampaignStock: 20, PerUserLimit: 2, Status: 1,
	})
	if err != nil || id != 9 {
		t.Fatalf("id=%d err=%v", id, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
