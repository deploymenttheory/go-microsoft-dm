package sqlstore

import (
	"testing"

	"github.com/go-sql-driver/mysql"
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
