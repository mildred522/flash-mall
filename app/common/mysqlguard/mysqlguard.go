package mysqlguard

import (
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
