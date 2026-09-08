package sqlstore

import "testing"

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
