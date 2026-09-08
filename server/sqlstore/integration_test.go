//go:build integration

package sqlstore_test

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/mdm"
	"github.com/deploymenttheory/go-microsoft-dm/server/sqlstore"
	"github.com/deploymenttheory/go-microsoft-dm/storage"
	"github.com/deploymenttheory/go-microsoft-dm/storage/storagetest"
)

// The integration suite runs the storage contract against a real PostgreSQL or
// MySQL server. It is gated behind the "integration" build tag and skips unless
// the relevant DSN is set, so ordinary `go test` never needs a database.
//
// Isolation is per store: every Factory call creates a uniquely named database
// on the target server and drops it on cleanup, so the contract's parallel
// subtests never share tables.

var dbCounter atomic.Uint64

func TestPostgresContract(t *testing.T) {
	dsn := requireDSN(t, "TEST_POSTGRES_DSN")
	newStore := freshDBFactory(t, sqlstore.Postgres, "pgx", dsn)
	t.Run("PushChannel", func(t *testing.T) { runPushChannelLifecycle(t, newStore(t)) })
	t.Run("Store", func(t *testing.T) {
		storagetest.RunAll(t, func(t *testing.T) storage.Store { return newStore(t) })
	})
	t.Run("Queue", func(t *testing.T) {
		storagetest.RunQueueSuite(t, func(t *testing.T) mdm.CommandQueue { return newStore(t).Queue() })
	})
	t.Run("Credentials", func(t *testing.T) {
		storagetest.RunCredentialSuite(t, func(t *testing.T) storage.CredentialStore { return newStore(t) })
	})
}

func TestMySQLContract(t *testing.T) {
	dsn := requireDSN(t, "TEST_MYSQL_DSN")
	newStore := freshDBFactory(t, sqlstore.MySQL, "mysql", dsn)
	t.Run("PushChannel", func(t *testing.T) { runPushChannelLifecycle(t, newStore(t)) })
	t.Run("Store", func(t *testing.T) {
		storagetest.RunAll(t, func(t *testing.T) storage.Store { return newStore(t) })
	})
	t.Run("Queue", func(t *testing.T) {
		storagetest.RunQueueSuite(t, func(t *testing.T) mdm.CommandQueue { return newStore(t).Queue() })
	})
	t.Run("Credentials", func(t *testing.T) {
		storagetest.RunCredentialSuite(t, func(t *testing.T) storage.CredentialStore { return newStore(t) })
	})
}

// requireDSN returns the DSN in env, skipping the test if it is unset.
func requireDSN(t *testing.T, key string) string {
	t.Helper()
	dsn := os.Getenv(key)
	if dsn == "" {
		t.Skipf("%s not set", key)
	}
	return dsn
}

// freshDBFactory returns a store factory that provisions a new database on the
// server per call and opens a store against it. Cleanup drops the database.
func freshDBFactory(t *testing.T, kind sqlstore.Kind, driver, adminDSN string) func(*testing.T) *sqlstore.Store {
	t.Helper()
	return func(t *testing.T) *sqlstore.Store {
		t.Helper()
		name := fmt.Sprintf("dm_it_%d", dbCounter.Add(1))
		admin, err := sql.Open(driver, adminDSN)
		if err != nil {
			t.Fatalf("open admin: %v", err)
		}
		ctx := context.Background()
		if _, err := admin.ExecContext(ctx, "CREATE DATABASE "+name); err != nil {
			_ = admin.Close()
			t.Fatalf("create database %s: %v", name, err)
		}
		storeDSN := withDatabase(kind, adminDSN, name)
		s, err := sqlstore.Open(ctx, kind, storeDSN, sqlstore.Options{})
		if err != nil {
			_, _ = admin.ExecContext(ctx, "DROP DATABASE IF EXISTS "+name)
			_ = admin.Close()
			t.Fatalf("open store on %s: %v", name, err)
		}
		t.Cleanup(func() {
			_ = s.Close()
			_, _ = admin.ExecContext(context.Background(), "DROP DATABASE IF EXISTS "+name)
			_ = admin.Close()
		})
		return s
	}
}

// withDatabase rewrites a DSN to point at a different database name. PostgreSQL
// DSNs are URLs (postgres://.../db); MySQL DSNs are user:pass@tcp(host)/db.
func withDatabase(kind sqlstore.Kind, dsn, name string) string {
	switch kind {
	case sqlstore.Postgres:
		u, err := url.Parse(dsn)
		if err != nil {
			return dsn
		}
		u.Path = "/" + name
		return u.String()
	case sqlstore.MySQL:
		slash := strings.LastIndexByte(dsn, '/')
		if slash < 0 {
			return dsn
		}
		rest := ""
		if q := strings.IndexByte(dsn[slash:], '?'); q >= 0 {
			rest = dsn[slash+q:]
		}
		return dsn[:slash+1] + name + rest
	default:
		return dsn
	}
}
