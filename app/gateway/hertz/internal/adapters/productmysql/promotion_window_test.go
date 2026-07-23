package productmysql

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestPromotionWindowAffectedProductIDs(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Date(2026, 7, 22, 12, 0, 0, 0, time.Local)
	mock.ExpectQuery("SELECT DISTINCT product_id").WithArgs(
		now.Add(-2*time.Hour), now.Add(2*time.Hour), now.Add(-2*time.Hour), now.Add(2*time.Hour), int64(100),
	).WillReturnRows(sqlmock.NewRows([]string{"product_id"}).AddRow(8).AddRow(9))
	ids, err := NewPromotionRepository(db).WindowAffectedProductIDs(context.Background(), now, 120, 100)
	if err != nil || len(ids) != 2 || ids[1] != 9 {
		t.Fatalf("ids=%v err=%v", ids, err)
	}
}
