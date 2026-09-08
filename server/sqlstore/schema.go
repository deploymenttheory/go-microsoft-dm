package sqlstore

import (
	"context"
	"fmt"
)

// migrate creates the schema. The statements are idempotent
// (CREATE TABLE IF NOT EXISTS), so applying them repeatedly is safe; a
// versioned migration table is a later concern.
func (s *conn) migrate(ctx context.Context) error {
	for _, stmt := range s.schema() {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("sqlstore: migrate: %w", err)
		}
	}
	return nil
}

// schema returns the CREATE statements for the dialect. Only the blob column
// type and the events auto-increment id differ between dialects; the rest is
// portable. BIGINT stores Unix-nanosecond times and the per-device sequence as
// int64: on SQLite it takes INTEGER affinity, and on PostgreSQL and MySQL it is
// the 64-bit type the 32-bit INTEGER (int4) is too small for. Columns that take
// part in a key are VARCHAR(255) rather than TEXT, because MySQL cannot index a
// TEXT column without a prefix length; free-text columns stay TEXT.
func (s *conn) schema() []string {
	blob := s.d.blob
	return []string{
		`CREATE TABLE IF NOT EXISTS enrollments (
			serial VARCHAR(255) PRIMARY KEY,
			thumbprint VARCHAR(255) NOT NULL UNIQUE,
			device_id VARCHAR(255) NOT NULL,
			hwdevid VARCHAR(255) NOT NULL,
			enrollment_type TEXT NOT NULL,
			upn TEXT NOT NULL,
			context ` + blob + ` NOT NULL,
			state TEXT NOT NULL,
			enrolled_at BIGINT NOT NULL,
			updated_at BIGINT NOT NULL,
			last_seen_at BIGINT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_enrollments_device ON enrollments (device_id)`,
		`CREATE INDEX IF NOT EXISTS idx_enrollments_hwdevid ON enrollments (hwdevid)`,
		`CREATE INDEX IF NOT EXISTS idx_enrollments_order ON enrollments (enrolled_at, serial)`,
		`CREATE TABLE IF NOT EXISTS certificates (
			serial VARCHAR(255) PRIMARY KEY,
			thumbprint VARCHAR(255) NOT NULL UNIQUE,
			subject TEXT NOT NULL,
			device_id VARCHAR(255) NOT NULL,
			not_before BIGINT NOT NULL,
			not_after BIGINT NOT NULL,
			raw ` + blob + ` NOT NULL,
			revoked BIGINT NOT NULL,
			revoked_at BIGINT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_certificates_device ON certificates (device_id)`,
		`CREATE INDEX IF NOT EXISTS idx_certificates_order ON certificates (not_before, serial)`,
		`CREATE TABLE IF NOT EXISTS mdm_credentials (
			device_id VARCHAR(255) PRIMARY KEY,
			auth_type TEXT NOT NULL,
			credential_hash ` + blob + ` NOT NULL,
			basic_username TEXT NOT NULL,
			basic_password TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS commands (
			device_id VARCHAR(255) NOT NULL,
			id VARCHAR(255) NOT NULL,
			seq BIGINT NOT NULL,
			scope TEXT NOT NULL,
			internal BIGINT NOT NULL,
			state TEXT NOT NULL,
			body ` + blob + ` NOT NULL,
			enqueued_at BIGINT NOT NULL,
			last_sent_at BIGINT NOT NULL,
			sent_msg_id TEXT NOT NULL,
			attempts BIGINT NOT NULL,
			completed_at BIGINT NOT NULL,
			result ` + blob + `,
			PRIMARY KEY (device_id, id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_commands_seq ON commands (device_id, seq)`,
		`CREATE TABLE IF NOT EXISTS device_facts (
			device_id VARCHAR(255) PRIMARY KEY,
			devinfo ` + blob + ` NOT NULL,
			login_status TEXT NOT NULL,
			sync_type TEXT NOT NULL,
			device_prep_sync TEXT NOT NULL,
			channel_uri TEXT NOT NULL,
			updated_at BIGINT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS events (
			id ` + s.d.autoID + `,
			device_id VARCHAR(255) NOT NULL,
			kind TEXT NOT NULL,
			detail ` + blob + ` NOT NULL,
			at BIGINT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_events_device ON events (device_id, id)`,
	}
}
