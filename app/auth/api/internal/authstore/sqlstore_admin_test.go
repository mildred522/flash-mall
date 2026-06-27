package authstore

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type demoSeedDriver struct {
	mu    sync.Mutex
	execs []demoSeedExec
}

type demoSeedExec struct {
	query string
	args  []driver.Value
}

type demoSeedConn struct {
	driver *demoSeedDriver
}

func (d *demoSeedDriver) Open(string) (driver.Conn, error) {
	return &demoSeedConn{driver: d}, nil
}

func (c *demoSeedConn) Prepare(string) (driver.Stmt, error) {
	return nil, fmt.Errorf("prepare not supported")
}

func (c *demoSeedConn) Close() error { return nil }

func (c *demoSeedConn) Begin() (driver.Tx, error) { return c, nil }

func (c *demoSeedConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	return c, nil
}

func (c *demoSeedConn) Commit() error { return nil }

func (c *demoSeedConn) Rollback() error { return nil }

func (c *demoSeedConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	values := make([]driver.Value, 0, len(args))
	for _, arg := range args {
		values = append(values, arg.Value)
	}
	c.driver.mu.Lock()
	defer c.driver.mu.Unlock()
	c.driver.execs = append(c.driver.execs, demoSeedExec{query: query, args: values})
	return driver.RowsAffected(1), nil
}

func TestSQLStoreEnsureDemoUserSeedsAdminUser(t *testing.T) {
	driverName := fmt.Sprintf("demo-seed-%s", strings.ReplaceAll(t.Name(), "/", "-"))
	recorder := &demoSeedDriver{}
	sql.Register(driverName, recorder)
	db, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatalf("open recording db: %v", err)
	}
	defer func() { _ = db.Close() }()

	store := NewSQLStore(sqlx.NewSqlConnFromDB(db), "flashmall123", nil)
	if err := store.ensureDemoUser(); err != nil {
		t.Fatalf("ensure demo user failed: %v", err)
	}

	if !recorder.hasExecArg(int64(1002)) {
		t.Fatal("expected admin user id 1002 to be seeded")
	}
	if !recorder.hasExecArg("13800000002") {
		t.Fatal("expected admin phone to be seeded")
	}
	if !recorder.hasExecArg("admin") {
		t.Fatal("expected admin role to be seeded")
	}
}

func (d *demoSeedDriver) hasExecArg(want driver.Value) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, exec := range d.execs {
		for _, arg := range exec.args {
			if arg == want {
				return true
			}
		}
	}
	return false
}
