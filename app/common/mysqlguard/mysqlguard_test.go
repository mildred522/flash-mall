package mysqlguard

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestRequireUTF8MB4(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		dsn     string
		wantErr bool
	}{
		{name: "utf8mb4", dsn: "root:pass@tcp(mysql:3306)/mall?charset=utf8mb4&parseTime=true"},
		{name: "empty optional datasource", dsn: ""},
		{name: "missing charset", dsn: "root:pass@tcp(mysql:3306)/mall?parseTime=true", wantErr: true},
		{name: "legacy utf8", dsn: "root:pass@tcp(mysql:3306)/mall?charset=utf8", wantErr: true},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := RequireUTF8MB4("test", test.dsn)
			if (err != nil) != test.wantErr {
				t.Fatalf("RequireUTF8MB4() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}

func TestVerifySessionUTF8MB4ReadsEffectiveConnectionCharsets(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		values  []string
		wantErr bool
	}{
		{name: "utf8mb4 session", values: []string{"utf8mb4", "utf8mb4", "utf8mb4"}},
		{name: "legacy connection charset", values: []string{"utf8mb4", "utf8", "utf8mb4"}, wantErr: true},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			mock.ExpectQuery("SELECT @@character_set_client").
				WillReturnRows(sqlmock.NewRows([]string{
					"character_set_client",
					"character_set_connection",
					"character_set_results",
				}).AddRow(test.values[0], test.values[1], test.values[2]))

			err = VerifySessionUTF8MB4(context.Background(), "test", db)
			if (err != nil) != test.wantErr {
				t.Fatalf("VerifySessionUTF8MB4() error=%v wantErr=%t", err, test.wantErr)
			}
			if err = mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
