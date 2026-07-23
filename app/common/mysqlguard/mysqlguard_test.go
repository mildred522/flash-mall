package mysqlguard

import "testing"

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
