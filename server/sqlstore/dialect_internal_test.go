package sqlstore

import (
	"testing"

	"github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestDialectFor(t *testing.T) {
	t.Parallel()
	for _, k := range []Kind{SQLite, Postgres, MySQL} {
		d, err := dialectFor(k)
		if err != nil || d.kind != k || d.driver == "" || d.blob == "" || d.autoID == "" {
			t.Errorf("dialectFor(%s) = %+v, %v", k, d, err)
		}
	}
	if _, err := dialectFor("oracle"); err == nil {
		t.Error("unknown dialect accepted")
	}
}

func TestRebind(t *testing.T) {
	t.Parallel()
	sqlite, _ := dialectFor(SQLite)
	mysql, _ := dialectFor(MySQL)
	pg, _ := dialectFor(Postgres)
	q := `SELECT a FROM t WHERE b = ? AND c = ?`
	if got := sqlite.rebind(q); got != q {
		t.Errorf("sqlite rebind = %q", got)
	}
	if got := mysql.rebind(q); got != q {
		t.Errorf("mysql rebind = %q", got)
	}
	if got := pg.rebind(q); got != `SELECT a FROM t WHERE b = $1 AND c = $2` {
		t.Errorf("pg rebind = %q", got)
	}
}

func TestUpsert(t *testing.T) {
	t.Parallel()
	pg, _ := dialectFor(Postgres)
	mysql, _ := dialectFor(MySQL)
	if got := pg.upsert([]string{"id"}, []string{"a", "b"}); got != ` ON CONFLICT (id) DO UPDATE SET a = EXCLUDED.a, b = EXCLUDED.b` {
		t.Errorf("pg upsert = %q", got)
	}
	if got := mysql.upsert([]string{"id"}, []string{"a", "b"}); got != ` ON DUPLICATE KEY UPDATE a = VALUES(a), b = VALUES(b)` {
		t.Errorf("mysql upsert = %q", got)
	}
}

func TestIncrementSeq(t *testing.T) {
	t.Parallel()
	pg, _ := dialectFor(Postgres)
	sqlite, _ := dialectFor(SQLite)
	mysql, _ := dialectFor(MySQL)
	want := ` ON CONFLICT (device_id) DO UPDATE SET next_seq = command_seq.next_seq + 1`
	if got := pg.incrementSeq(); got != want {
		t.Errorf("pg incrementSeq = %q", got)
	}
	if got := sqlite.incrementSeq(); got != want {
		t.Errorf("sqlite incrementSeq = %q", got)
	}
	if got := mysql.incrementSeq(); got != ` ON DUPLICATE KEY UPDATE next_seq = next_seq + 1` {
		t.Errorf("mysql incrementSeq = %q", got)
	}
}

func TestCreateIndex(t *testing.T) {
	t.Parallel()
	pg, _ := dialectFor(Postgres)
	mysql, _ := dialectFor(MySQL)
	if got := pg.createIndex("i", "t", "a, b"); got != `CREATE INDEX IF NOT EXISTS i ON t (a, b)` {
		t.Errorf("pg createIndex = %q", got)
	}
	if got := mysql.createIndex("i", "t", "a, b"); got != `CREATE INDEX i ON t (a, b)` {
		t.Errorf("mysql createIndex = %q", got)
	}
}

func TestIsDuplicateIndex(t *testing.T) {
	t.Parallel()
	if isDuplicateIndex(nil) {
		t.Error("nil reported as duplicate index")
	}
	if isDuplicateIndex(ErrDialect) {
		t.Error("a non-mysql error reported as duplicate index")
	}
	if !isDuplicateIndex(&mysql.MySQLError{Number: 1061, Message: "Duplicate key name 'i'"}) {
		t.Error("1061 not reported as duplicate index")
	}
	if isDuplicateIndex(&mysql.MySQLError{Number: 1146, Message: "table doesn't exist"}) {
		t.Error("1146 reported as duplicate index")
	}
}

func TestIsRetriable(t *testing.T) {
	t.Parallel()
	if isRetriable(nil) {
		t.Error("nil is not retriable")
	}
	if isRetriable(ErrDialect) {
		t.Error("a plain error is not retriable")
	}
	for _, n := range []uint16{1213, 1205} {
		if !isRetriable(&mysql.MySQLError{Number: n}) {
			t.Errorf("mysql %d should be retriable", n)
		}
	}
	if isRetriable(&mysql.MySQLError{Number: 1062}) {
		t.Error("mysql 1062 (duplicate entry) is not retriable")
	}
	for _, c := range []string{"40001", "40P01"} {
		if !isRetriable(&pgconn.PgError{Code: c}) {
			t.Errorf("postgres %s should be retriable", c)
		}
	}
	if isRetriable(&pgconn.PgError{Code: "23505"}) {
		t.Error("postgres 23505 (unique violation) is not retriable")
	}
}
