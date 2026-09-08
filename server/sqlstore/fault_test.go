package sqlstore

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"time"
	"sync"
	"sync/atomic"
)

// faultDriver wraps another database/sql driver and fails the Nth statement
// (Exec or Query, in or out of a transaction), so tests can exercise the
// error branches that a healthy database never takes.
type faultDriver struct{ base driver.Driver }

// faultState is the shared injection counter. When armed is true, each
// statement decrements remaining; the one that brings it to zero fails.
var faultState struct {
	mu        sync.Mutex
	armed     bool
	remaining int64
}

var errInjected = errors.New("sqlstore: injected fault")

// armFault makes the countdown-th statement fail (1 = the next one).
func armFault(countdown int64) {
	faultState.mu.Lock()
	faultState.armed = true
	atomic.StoreInt64(&faultState.remaining, countdown)
	faultState.mu.Unlock()
}

func disarmFault() {
	faultState.mu.Lock()
	faultState.armed = false
	faultState.mu.Unlock()
}

// trip reports whether this statement should fail.
func trip() bool {
	faultState.mu.Lock()
	armed := faultState.armed
	faultState.mu.Unlock()
	if !armed {
		return false
	}
	return atomic.AddInt64(&faultState.remaining, -1) == 0
}

var registerFaultOnce sync.Once

// registerFaultDriver registers "sqlite-fault" wrapping the sqlite driver.
func registerFaultDriver() {
	registerFaultOnce.Do(func() {
		db, err := sql.Open("sqlite", "file::memory:")
		if err != nil {
			panic(err)
		}
		base := db.Driver()
		_ = db.Close()
		sql.Register("sqlite-fault", faultDriver{base: base})
	})
}

func (d faultDriver) Open(name string) (driver.Conn, error) {
	c, err := d.base.Open(name)
	if err != nil {
		return nil, err
	}
	return &faultConn{Conn: c}, nil
}

type faultConn struct{ driver.Conn }

func (c *faultConn) Prepare(q string) (driver.Stmt, error) { return c.Conn.Prepare(q) }
func (c *faultConn) Begin() (driver.Tx, error)             { return c.Conn.Begin() } //nolint:staticcheck // delegate

func (c *faultConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	if bt, ok := c.Conn.(driver.ConnBeginTx); ok {
		return bt.BeginTx(ctx, opts)
	}
	return c.Conn.Begin() //nolint:staticcheck // fallback
}

func (c *faultConn) ExecContext(ctx context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	if trip() {
		return nil, errInjected
	}
	if e, ok := c.Conn.(driver.ExecerContext); ok {
		return e.ExecContext(ctx, q, args)
	}
	return nil, driver.ErrSkip
}

func (c *faultConn) QueryContext(ctx context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	if trip() {
		return nil, errInjected
	}
	if qr, ok := c.Conn.(driver.QueryerContext); ok {
		return qr.QueryContext(ctx, q, args)
	}
	return nil, driver.ErrSkip
}

func (c *faultConn) Ping(ctx context.Context) error {
	if p, ok := c.Conn.(driver.Pinger); ok {
		return p.Ping(ctx)
	}
	return nil
}

var _ io.Closer = (*faultConn)(nil)

// openRaw opens a *sql.DB for a dialect without running migrations (the
// fault-injection tests seed the schema through a separate healthy handle).
func openRaw(d dialect, dsn string) (*sql.DB, error) {
	db, err := sql.Open(d.driver, dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	return db, nil
}

// clockZero is a fixed clock for the fault tests.
type clockZero struct{}

func (clockZero) Now() time.Time                              { return time.Unix(0, 0).UTC() }
func (clockZero) Since(t time.Time) time.Duration             { return 0 }
func (clockZero) After(d time.Duration) <-chan time.Time      { ch := make(chan time.Time); return ch }
