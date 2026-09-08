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
// type differs between dialects; everything else is portable (TEXT, INTEGER).
func (s *conn) schema() []string {
	blob := s.d.blob
	return []string{
		`CREATE TABLE IF NOT EXISTS enrollments (
			serial TEXT PRIMARY KEY,
			thumbprint TEXT NOT NULL UNIQUE,
			device_id TEXT NOT NULL,
			hwdevid TEXT NOT NULL,
			enrollment_type TEXT NOT NULL,
			upn TEXT NOT NULL,
			context ` + blob + ` NOT NULL,
			state TEXT NOT NULL,
			enrolled_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL,
			last_seen_at INTEGER NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_enrollments_device ON enrollments (device_id)`,
		`CREATE INDEX IF NOT EXISTS idx_enrollments_hwdevid ON enrollments (hwdevid)`,
		`CREATE INDEX IF NOT EXISTS idx_enrollments_order ON enrollments (enrolled_at, serial)`,
		`CREATE TABLE IF NOT EXISTS certificates (
			serial TEXT PRIMARY KEY,
			thumbprint TEXT NOT NULL UNIQUE,
			subject TEXT NOT NULL,
			device_id TEXT NOT NULL,
			not_before INTEGER NOT NULL,
			not_after INTEGER NOT NULL,
			raw ` + blob + ` NOT NULL,
			revoked INTEGER NOT NULL,
			revoked_at INTEGER NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_certificates_device ON certificates (device_id)`,
		`CREATE INDEX IF NOT EXISTS idx_certificates_order ON certificates (not_before, serial)`,
		`CREATE TABLE IF NOT EXISTS mdm_credentials (
			device_id TEXT PRIMARY KEY,
			auth_type TEXT NOT NULL,
			credential_hash ` + blob + ` NOT NULL,
			basic_username TEXT NOT NULL,
			basic_password TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS commands (
			device_id TEXT NOT NULL,
			id TEXT NOT NULL,
			seq INTEGER NOT NULL,
			scope TEXT NOT NULL,
			internal INTEGER NOT NULL,
			state TEXT NOT NULL,
			body ` + blob + ` NOT NULL,
			enqueued_at INTEGER NOT NULL,
			last_sent_at INTEGER NOT NULL,
			sent_msg_id TEXT NOT NULL,
			attempts INTEGER NOT NULL,
			completed_at INTEGER NOT NULL,
			result ` + blob + `,
			PRIMARY KEY (device_id, id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_commands_seq ON commands (device_id, seq)`,
		`CREATE TABLE IF NOT EXISTS device_facts (
			device_id TEXT PRIMARY KEY,
			devinfo ` + blob + ` NOT NULL,
			login_status TEXT NOT NULL,
			sync_type TEXT NOT NULL,
			device_prep_sync TEXT NOT NULL,
			channel_uri TEXT NOT NULL,
			updated_at INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS events (
			id ` + s.d.autoID + `,
			device_id TEXT NOT NULL,
			kind TEXT NOT NULL,
			detail ` + blob + ` NOT NULL,
			at INTEGER NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_events_device ON events (device_id, id)`,
	}
}
