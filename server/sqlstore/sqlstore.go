package sqlstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/deploymenttheory/go-microsoft-dm/clock"
	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/mdm"
	"github.com/deploymenttheory/go-microsoft-dm/storage"

	"github.com/go-sql-driver/mysql"   // mysql driver and error type
	"github.com/jackc/pgx/v5/pgconn"   // postgres error type
	_ "github.com/jackc/pgx/v5/stdlib" // postgres driver (pgx stdlib)
	_ "modernc.org/sqlite"             // pure-Go sqlite driver
)

// Errors returned by this package.
var (
	// ErrDialect reports an unknown SQL dialect.
	ErrDialect = errors.New("sqlstore: unknown dialect")
	// ErrConfig reports an unusable configuration.
	ErrConfig = errors.New("sqlstore: invalid configuration")
)

// conn holds the database handle and dialect shared by the stores.
type conn struct {
	db    *sql.DB
	d     dialect
	clock clock.Clock
}

// Store is the SQL-backed durable store: enrollments, certificates,
// credentials, device facts and the event log. The command queue is a
// separate type (Queue) over the same connection, because storage.Store and
// mdm.CommandQueue both define Get and List with different signatures.
type Store struct {
	*conn
}

// Queue is the SQL-backed mdm.CommandQueue over the same connection.
type Queue struct {
	*conn
}

var (
	_ storage.Store           = (*Store)(nil)
	_ storage.CredentialStore = (*Store)(nil)
	_ mdm.CommandQueue        = (*Queue)(nil)
)

// Queue returns the command queue over this store's connection.
func (s *Store) Queue() *Queue { return &Queue{conn: s.conn} }

// Options configure Open.
type Options struct {
	// Clock stamps server-side times (event log); default real time.
	Clock clock.Clock
	// MaxOpenConns caps the connection pool; 0 leaves the driver default,
	// except SQLite where it is forced to 1 (a file database is not safe for
	// concurrent writers).
	MaxOpenConns int
}

// Open connects to the database, applies the schema and returns a Store.
func Open(ctx context.Context, kind Kind, dsn string, opts Options) (*Store, error) {
	d, err := dialectFor(kind)
	if err != nil {
		return nil, err
	}
	if dsn == "" {
		return nil, fmt.Errorf("%w: empty DSN", ErrConfig)
	}
	db, err := sql.Open(d.driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("sqlstore: open: %w", err)
	}
	if kind == SQLite {
		db.SetMaxOpenConns(1)
	} else if opts.MaxOpenConns > 0 {
		db.SetMaxOpenConns(opts.MaxOpenConns)
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("sqlstore: ping: %w", err)
	}
	clk := opts.Clock
	if clk == nil {
		clk = clock.Real{}
	}
	s := &Store{conn: &conn{db: db, d: d, clock: clk}}
	if err := s.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// DB exposes the underlying handle for health checks.
func (s *conn) DB() *sql.DB { return s.db }

// Close closes the database.
func (s *conn) Close() error { return s.db.Close() }

// exec runs a statement with the dialect's placeholders.
func (s *conn) exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return s.db.ExecContext(ctx, s.d.rebind(query), args...)
}

// query runs a query with the dialect's placeholders.
func (s *conn) query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return s.db.QueryContext(ctx, s.d.rebind(query), args...)
}

// queryRow runs a single-row query with the dialect's placeholders.
func (s *conn) queryRow(ctx context.Context, query string, args ...any) *sql.Row {
	return s.db.QueryRowContext(ctx, s.d.rebind(query), args...)
}

// nanos converts a time to Unix nanoseconds, with zero for the zero time.
func nanos(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UTC().UnixNano()
}

// fromNanos converts stored Unix nanoseconds back to a UTC time, with the
// zero time for 0.
func fromNanos(n int64) time.Time {
	if n == 0 {
		return time.Time{}
	}
	return time.Unix(0, n).UTC()
}

// boolInt encodes a bool as 0/1.
func boolInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

// tx runs fn in a transaction, committing on success and rolling back on
// error.
// tx runs fn in a transaction, retrying a bounded number of times when the
// database reports a transient deadlock or serialization failure. PostgreSQL
// and MySQL can abort one of two concurrent write transactions this way (the
// per-device sequence counter is a hot row); the caller's work is idempotent
// on retry because a failed transaction rolls back entirely. Non-transient
// errors, including ErrConflict, are returned on the first attempt.
func (s *conn) tx(ctx context.Context, fn func(*sql.Tx) error) error {
	const maxAttempts = 10
	var err error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if err = s.txOnce(ctx, fn); err == nil || !isRetriable(err) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(attempt+1) * time.Millisecond):
		}
	}
	return err
}

func (s *conn) txOnce(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("sqlstore: begin: %w", err)
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// isRetriable reports a transient deadlock or serialization failure that a
// transaction can safely retry: MySQL 1213 (deadlock) and 1205 (lock wait
// timeout), PostgreSQL 40001 (serialization_failure) and 40P01 (deadlock).
func isRetriable(err error) bool {
	var me *mysql.MySQLError
	if errors.As(err, &me) {
		return me.Number == 1213 || me.Number == 1205
	}
	var pe *pgconn.PgError
	if errors.As(err, &pe) {
		return pe.Code == "40001" || pe.Code == "40P01"
	}
	return false
}

// rebindRow runs a single-row query inside a transaction with the dialect's
// placeholders.
func (s *conn) rebindRow(ctx context.Context, tx *sql.Tx, query string, args ...any) *sql.Row {
	return tx.QueryRowContext(ctx, s.d.rebind(query), args...)
}
