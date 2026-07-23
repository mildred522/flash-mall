package mysqlguard

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"

	mysql "github.com/go-sql-driver/mysql"
)

// RequireUTF8MB4 prevents a service from silently reconnecting with a
// character set that can turn valid product names into literal question marks.
func RequireUTF8MB4(name, dsn string) error {
	if strings.TrimSpace(dsn) == "" {
		return nil
	}
	_, err := mysql.ParseDSN(dsn)
	if err != nil {
		return fmt.Errorf("%s datasource is invalid: %w", name, err)
	}
	queryStart := strings.LastIndexByte(dsn, '?')
	if queryStart < 0 {
		return fmt.Errorf("%s datasource must explicitly set charset=utf8mb4", name)
	}
	params, err := url.ParseQuery(dsn[queryStart+1:])
	if err != nil {
		return fmt.Errorf("%s datasource parameters are invalid: %w", name, err)
	}
	if !strings.EqualFold(params.Get("charset"), "utf8mb4") {
		return fmt.Errorf("%s datasource must explicitly set charset=utf8mb4", name)
	}
	return nil
}

func MustUTF8MB4(name, dsn string) {
	if err := RequireUTF8MB4(name, dsn); err != nil {
		panic(err)
	}
}

// VerifySessionUTF8MB4 checks the effective character sets negotiated with
// MySQL instead of trusting the DSN alone.
func VerifySessionUTF8MB4(ctx context.Context, name string, db *sql.DB) error {
	if db == nil {
		return fmt.Errorf("%s database is not initialized", name)
	}
	var client, connection, results string
	if err := db.QueryRowContext(ctx, `
SELECT @@character_set_client, @@character_set_connection, @@character_set_results`).
		Scan(&client, &connection, &results); err != nil {
		return fmt.Errorf("%s datasource session charset query failed: %w", name, err)
	}
	if !strings.EqualFold(client, "utf8mb4") ||
		!strings.EqualFold(connection, "utf8mb4") ||
		!strings.EqualFold(results, "utf8mb4") {
		return fmt.Errorf(
			"%s datasource session must use utf8mb4: client=%s connection=%s results=%s",
			name,
			client,
			connection,
			results,
		)
	}
	return nil
}

func MustSessionUTF8MB4(ctx context.Context, name string, db *sql.DB) {
	if err := VerifySessionUTF8MB4(ctx, name, db); err != nil {
		panic(err)
	}
}
