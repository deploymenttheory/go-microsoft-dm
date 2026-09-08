package sqlstore

import (
	"fmt"
	"strconv"
	"strings"
)

// Kind names a supported SQL dialect.
type Kind string

// The supported dialects.
const (
	SQLite   Kind = "sqlite"
	Postgres Kind = "postgres"
	MySQL    Kind = "mysql"
)

// dialect captures the differences between the three databases.
type dialect struct {
	kind Kind
	// driver is the database/sql driver name.
	driver string
	// blob, ts and boolCol are the column types for DER/JSON, Unix-nano
	// timestamps and 0/1 booleans.
	blob string
	// autoID is the column definition for an auto-incrementing primary key.
	autoID string
	// postgresPlaceholders selects $1-style bind markers over '?'.
	postgresPlaceholders bool
}

func dialectFor(k Kind) (dialect, error) {
	switch k {
	case SQLite:
		return dialect{kind: SQLite, driver: "sqlite", blob: "BLOB", autoID: "INTEGER PRIMARY KEY AUTOINCREMENT"}, nil
	case Postgres:
		return dialect{kind: Postgres, driver: "pgx", blob: "BYTEA", autoID: "BIGSERIAL PRIMARY KEY", postgresPlaceholders: true}, nil
	case MySQL:
		return dialect{kind: MySQL, driver: "mysql", blob: "LONGBLOB", autoID: "BIGINT AUTO_INCREMENT PRIMARY KEY"}, nil
	}
	return dialect{}, fmt.Errorf("%w: %q", ErrDialect, k)
}

// rebind turns the '?' placeholders in a query into the dialect's markers.
// SQLite and MySQL use '?'; PostgreSQL uses $1, $2, ...
func (d dialect) rebind(query string) string {
	if !d.postgresPlaceholders {
		return query
	}
	var b strings.Builder
	n := 0
	for i := 0; i < len(query); i++ {
		if query[i] == '?' {
			n++
			b.WriteByte('$')
			b.WriteString(strconv.Itoa(n))
			continue
		}
		b.WriteByte(query[i])
	}
	return b.String()
}

// incrementSeq returns the conflict clause for the per-device sequence
// counter: on a first insert the row keeps its VALUES seq of 1, and on a
// collision next_seq is incremented. The INSERT holds a row lock until the
// transaction commits, so concurrent enqueues for one device serialise and
// never share a sequence, without a dialect-specific FOR UPDATE.
func (d dialect) incrementSeq() string {
	if d.kind == MySQL {
		return " ON DUPLICATE KEY UPDATE next_seq = next_seq + 1"
	}
	return " ON CONFLICT (device_id) DO UPDATE SET next_seq = command_seq.next_seq + 1"
}

// createIndex returns a CREATE INDEX statement. SQLite and PostgreSQL accept
// IF NOT EXISTS; MySQL does not, so its statement omits the clause and migrate
// tolerates the duplicate-index error on a re-run.
func (d dialect) createIndex(name, table, cols string) string {
	if d.kind == MySQL {
		return "CREATE INDEX " + name + " ON " + table + " (" + cols + ")"
	}
	return "CREATE INDEX IF NOT EXISTS " + name + " ON " + table + " (" + cols + ")"
}

// upsert returns the conflict clause for an INSERT that updates on a key
// collision. cols are the non-key columns to overwrite.
func (d dialect) upsert(keyCols, cols []string) string {
	switch d.kind {
	case MySQL:
		sets := make([]string, len(cols))
		for i, c := range cols {
			sets[i] = c + " = VALUES(" + c + ")"
		}
		return " ON DUPLICATE KEY UPDATE " + strings.Join(sets, ", ")
	default: // sqlite, postgres
		sets := make([]string, len(cols))
		for i, c := range cols {
			sets[i] = c + " = EXCLUDED." + c
		}
		return " ON CONFLICT (" + strings.Join(keyCols, ", ") + ") DO UPDATE SET " + strings.Join(sets, ", ")
	}
}
