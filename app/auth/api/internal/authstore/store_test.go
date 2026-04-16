package authstore

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"flash-mall/app/auth/api/internal/sessionstate"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type recordingStateStore struct {
	mu       sync.Mutex
	saved    []sessionstate.SessionSnapshot
	deleted  []string
	versions map[int64]int64
}

type refreshLockDriver struct {
	t            *testing.T
	sessionID    string
	familySecret string
	currentHash  string
	currentGen   int64
	querySeen    bool
	updateSeen   bool
	committed    bool
}

type refreshLockConn struct {
	driver *refreshLockDriver
	inTx   bool
}

type refreshLockRows struct {
	columns []string
	values  []driver.Value
	done    bool
}

func (d *refreshLockDriver) Open(string) (driver.Conn, error) {
	return &refreshLockConn{driver: d}, nil
}

func (c *refreshLockConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepare not supported")
}

func (c *refreshLockConn) Close() error {
	return nil
}

func (c *refreshLockConn) Begin() (driver.Tx, error) {
	c.inTx = true
	return c, nil
}

func (c *refreshLockConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.inTx = true
	return c, nil
}

func (c *refreshLockConn) Commit() error {
	c.inTx = false
	c.driver.committed = true
	if !c.driver.querySeen {
		c.driver.t.Fatalf("expected refresh to lock session row before updating")
	}
	if !c.driver.updateSeen {
		c.driver.t.Fatalf("expected refresh to update the session row")
	}
	return nil
}

func (c *refreshLockConn) Rollback() error {
	c.inTx = false
	return nil
}

func (c *refreshLockConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if !c.inTx {
		c.driver.t.Fatalf("expected refresh query to run inside a transaction")
	}
	if !strings.Contains(query, "FROM user_sessions") || !strings.Contains(query, "WHERE id = ?") || !strings.Contains(query, "FOR UPDATE") {
		c.driver.t.Fatalf("expected locked session-row query, got %q", query)
	}
	if len(args) != 1 {
		c.driver.t.Fatalf("expected one query arg, got %d", len(args))
	}
	if got := args[0].Value; got != c.driver.sessionID {
		c.driver.t.Fatalf("expected session id %q, got %v", c.driver.sessionID, got)
	}
	c.driver.querySeen = true
	return &refreshLockRows{
		columns: []string{"id", "user_id", "device_type", "session_version", "refresh_token_hash", "previous_refresh_token_hash", "refresh_family_secret", "refresh_generation", "expires_at", "status"},
		values: []driver.Value{
			c.driver.sessionID,
			int64(1001),
			"web",
			int64(1),
			c.driver.currentHash,
			"",
			c.driver.familySecret,
			c.driver.currentGen,
			time.Now().Add(time.Hour),
			statusActive,
		},
	}, nil
}

func (c *refreshLockConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if !c.inTx {
		c.driver.t.Fatalf("expected refresh update to run inside a transaction")
	}
	if !strings.Contains(query, "refresh_generation = ?") || !strings.Contains(query, "refresh_token_hash = ?") || !strings.Contains(query, "WHERE id = ? AND refresh_generation = ? AND refresh_token_hash = ? AND status = ?") {
		c.driver.t.Fatalf("expected conditional refresh update, got %q", query)
	}
	if len(args) != 8 {
		c.driver.t.Fatalf("expected 8 exec args, got %d", len(args))
	}
	if got := args[0].Value; got != c.driver.currentGen+1 {
		c.driver.t.Fatalf("expected next generation %d, got %v", c.driver.currentGen+1, got)
	}
	newHash, ok := args[1].Value.(string)
	if !ok || newHash == "" {
		c.driver.t.Fatalf("expected new refresh hash, got %v", args[0].Value)
	}
	if newHash == c.driver.currentHash {
		c.driver.t.Fatalf("expected new refresh hash to differ from current hash")
	}
	if got := args[2].Value; got != c.driver.currentHash {
		c.driver.t.Fatalf("expected previous hash %q, got %v", c.driver.currentHash, got)
	}
	if got := args[3].Value; got == nil {
		c.driver.t.Fatalf("expected expiration value")
	}
	if got := args[4].Value; got != c.driver.sessionID {
		c.driver.t.Fatalf("expected session id %q, got %v", c.driver.sessionID, got)
	}
	if got := args[5].Value; got != c.driver.currentGen {
		c.driver.t.Fatalf("expected refresh generation predicate %d, got %v", c.driver.currentGen, got)
	}
	if got := args[6].Value; got != c.driver.currentHash {
		c.driver.t.Fatalf("expected refresh hash predicate %q, got %v", c.driver.currentHash, got)
	}
	if got := fmt.Sprint(args[7].Value); got != fmt.Sprint(statusActive) {
		c.driver.t.Fatalf("expected active status predicate, got %v", args[7].Value)
	}
	c.driver.updateSeen = true
	c.driver.currentHash = newHash
	c.driver.currentGen++
	return driver.RowsAffected(1), nil
}

func (r *refreshLockRows) Columns() []string { return r.columns }

func (r *refreshLockRows) Close() error { return nil }

func (r *refreshLockRows) Next(dest []driver.Value) error {
	if r.done {
		return io.EOF
	}
	copy(dest, r.values)
	r.done = true
	return nil
}

func (r *recordingStateStore) SaveSession(_ context.Context, session sessionstate.SessionSnapshot, _ time.Duration) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.saved = append(r.saved, session)
	return nil
}

func (r *recordingStateStore) DeleteSession(_ context.Context, sessionID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.deleted = append(r.deleted, sessionID)
	return nil
}

func (r *recordingStateStore) SetUserVersion(_ context.Context, userID int64, version int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.versions == nil {
		r.versions = make(map[int64]int64)
	}
	r.versions[userID] = version
	return nil
}

func TestStoreCreateSessionForDevice_SameDeviceReplacesOldSession(t *testing.T) {
	store := NewStore("pwd")

	_, firstRefresh, err := store.CreateSessionForDevice(1001, "web", 3600)
	if err != nil {
		t.Fatalf("create first session: %v", err)
	}
	_, secondRefresh, err := store.CreateSessionForDevice(1001, "web", 3600)
	if err != nil {
		t.Fatalf("create second session: %v", err)
	}

	if _, _, err := store.RefreshSession(firstRefresh, 3600); err == nil {
		t.Fatalf("expected first same-device session to be invalidated")
	}
	if _, _, err := store.RefreshSession(secondRefresh, 3600); err != nil {
		t.Fatalf("expected latest same-device session to stay active: %v", err)
	}
}

func TestStoreCreateSessionForDevice_DifferentDevicesCanCoexist(t *testing.T) {
	store := NewStore("pwd")

	_, webRefresh, err := store.CreateSessionForDevice(1001, "web", 3600)
	if err != nil {
		t.Fatalf("create web session: %v", err)
	}
	_, iosRefresh, err := store.CreateSessionForDevice(1001, "ios", 3600)
	if err != nil {
		t.Fatalf("create ios session: %v", err)
	}

	if _, _, err := store.RefreshSession(webRefresh, 3600); err != nil {
		t.Fatalf("expected web session to remain active: %v", err)
	}
	if _, _, err := store.RefreshSession(iosRefresh, 3600); err != nil {
		t.Fatalf("expected ios session to remain active: %v", err)
	}
}

func TestStoreRefreshSession_ReplayedPreviousTokenRevokesSession(t *testing.T) {
	state := &recordingStateStore{}
	store := NewStoreWithState("pwd", state)

	sessionID, firstRefresh, err := store.CreateSessionForDevice(1001, "web", 3600)
	if err != nil {
		t.Fatalf("create first session: %v", err)
	}
	_, secondRefresh, err := store.RefreshSession(firstRefresh, 3600)
	if err != nil {
		t.Fatalf("rotate first session: %v", err)
	}

	if _, _, err := store.RefreshSession(firstRefresh, 3600); !errors.Is(err, ErrRefreshTokenReplayed) {
		t.Fatalf("expected replayed token to revoke session, got %v", err)
	}
	if _, ok := store.GetActiveSession(sessionID); ok {
		t.Fatalf("expected replayed session to be revoked")
	}
	if _, _, err := store.RefreshSession(secondRefresh, 3600); !errors.Is(err, ErrRefreshTokenInvalid) {
		t.Fatalf("expected latest refresh token to die after replay kill-switch, got %v", err)
	}

	state.mu.Lock()
	defer state.mu.Unlock()
	foundDelete := false
	for _, deletedID := range state.deleted {
		if deletedID == sessionID {
			foundDelete = true
			break
		}
	}
	if !foundDelete {
		t.Fatalf("expected session state to be deleted on replay")
	}
}

func TestStoreRefreshSession_ForgedTokenWithSameSessionIDDoesNotRevokeSession(t *testing.T) {
	store := NewStore("pwd")

	sessionID, firstRefresh, err := store.CreateSessionForDevice(1001, "web", 3600)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	parts := strings.SplitN(firstRefresh, ".", 3)
	if len(parts) != 3 {
		t.Fatalf("expected refresh token parts, got %q", firstRefresh)
	}
	forged := sessionID + "." + parts[1] + ".forged"

	if _, _, err := store.RefreshSession(forged, 3600); !errors.Is(err, ErrRefreshTokenInvalid) {
		t.Fatalf("expected forged token to be invalid, got %v", err)
	}
	if _, _, err := store.RefreshSession(firstRefresh, 3600); err != nil {
		t.Fatalf("expected valid token to remain usable after forged attempt: %v", err)
	}
}

func TestStoreRefreshSession_OlderValidTokenAfterMultipleRotationsKillsSession(t *testing.T) {
	store := NewStore("pwd")

	sessionID, tokenA, err := store.CreateSessionForDevice(1001, "web", 3600)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	tokenBSession, tokenB, err := store.RefreshSession(tokenA, 3600)
	if err != nil {
		t.Fatalf("first refresh: %v", err)
	}
	if tokenBSession == nil || tokenBSession.ID != sessionID {
		t.Fatalf("expected refreshed session to keep the same session id")
	}
	tokenCSession, tokenC, err := store.RefreshSession(tokenB, 3600)
	if err != nil {
		t.Fatalf("second refresh: %v", err)
	}
	if tokenCSession == nil || tokenCSession.ID != sessionID {
		t.Fatalf("expected second refresh to keep the same session id")
	}

	if _, _, err := store.RefreshSession(tokenA, 3600); !errors.Is(err, ErrRefreshTokenReplayed) {
		t.Fatalf("expected oldest token to kill the session, got %v", err)
	}
	if _, _, err := store.RefreshSession(tokenC, 3600); !errors.Is(err, ErrRefreshTokenInvalid) {
		t.Fatalf("expected latest token to die after replay kill-switch, got %v", err)
	}
}

func TestSQLStoreRefreshSession_LocksSessionRowBeforeRotating(t *testing.T) {
	driverName := fmt.Sprintf("refresh-lock-%d", time.Now().UnixNano())
	sql.Register(driverName, &refreshLockDriver{
		t:            t,
		sessionID:    "sess-123",
		familySecret: "family-secret",
		currentHash:  hashToken(buildRefreshToken("sess-123", 1, "family-secret")),
		currentGen:   1,
	})

	db, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatalf("open driver db: %v", err)
	}
	defer db.Close()

	store := NewSQLStore(sqlx.NewSqlConnFromDB(db), "pwd", nil)
	store.demoOnce.Do(func() {})

	session, tokenB, err := store.RefreshSession(buildRefreshToken("sess-123", 1, "family-secret"), 3600)
	if err != nil {
		t.Fatalf("refresh failed: %v", err)
	}
	if session == nil || session.ID != "sess-123" {
		t.Fatalf("expected refreshed session to keep the same session id")
	}
	if tokenB == "sess-123.secret-a" {
		t.Fatalf("expected token rotation")
	}
}

func TestStoreConsumeCode_BlocksAfterMaxAttempts(t *testing.T) {
	store := NewStore("pwd")

	code, _, err := store.IssueCode("13800138000", "reset-password", 300)
	if err != nil {
		t.Fatalf("issue code: %v", err)
	}
	if err := store.ConsumeCode("13800138000", "reset-password", "000000", 2); err == nil {
		t.Fatalf("expected first wrong attempt to fail")
	}
	if err := store.ConsumeCode("13800138000", "reset-password", "111111", 2); err == nil {
		t.Fatalf("expected second wrong attempt to fail")
	}
	if err := store.ConsumeCode("13800138000", "reset-password", code, 2); err == nil {
		t.Fatalf("expected code to be revoked after max attempts")
	}
}

func TestStoreResetCode_ClearsActiveCode(t *testing.T) {
	store := NewStore("pwd")

	code, _, err := store.IssueCode("13800138000", "register", 300)
	if err != nil {
		t.Fatalf("issue code: %v", err)
	}
	if err := store.ResetCode("13800138000", "register"); err != nil {
		t.Fatalf("reset code: %v", err)
	}
	if err := store.ConsumeCode("13800138000", "register", code, 5); err == nil {
		t.Fatalf("expected reset code to invalidate the active code")
	}
}

func TestStoreUpdatePassword_RevokesRefreshedSessions(t *testing.T) {
	store := NewStore("pwd")

	_, firstRefresh, err := store.CreateSessionForDevice(1001, "web", 3600)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	_, secondRefresh, err := store.RefreshSession(firstRefresh, 3600)
	if err != nil {
		t.Fatalf("refresh session: %v", err)
	}

	if _, err := store.UpdatePassword("13800000001", "new-pass-456"); err != nil {
		t.Fatalf("update password: %v", err)
	}
	if _, _, err := store.RefreshSession(firstRefresh, 3600); err == nil {
		t.Fatalf("expected old refresh token to be invalid after password reset")
	}
	if _, _, err := store.RefreshSession(secondRefresh, 3600); err == nil {
		t.Fatalf("expected rotated refresh token to be invalid after password reset")
	}
}
