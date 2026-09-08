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
