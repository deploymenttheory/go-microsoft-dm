package sqlstore

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/mdm"
	"github.com/deploymenttheory/go-microsoft-dm/storage"
)

func TestOpenRefusesUnmigratableBasicCredential(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dsn := "file:" + filepath.Join(t.TempDir(), "invalid-legacy.db")
	s, err := Open(ctx, SQLite, dsn, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.exec(ctx, `INSERT INTO mdm_credentials VALUES ('legacy', 'basic', ?, 'invalid:user', 'secret')`, []byte{}); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	opened, err := Open(ctx, SQLite, dsn, Options{})
	if opened != nil {
		opened.Close()
		t.Fatal("startup exposed a store with an unmigrated Basic credential")
	}
	if !errors.Is(err, mdm.ErrAuth) {
		t.Fatalf("startup did not report migration failure: %v", err)
	}
}

func TestBasicCredentialMigrationOnOpen(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dsn := "file:" + filepath.Join(t.TempDir(), "migration.db")
	s, err := Open(ctx, SQLite, dsn, Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct{ device, username, password string }{
		{"legacy", "user", "secret"}, {"empty-password", "user", ""}, {"empty-user", "", "secret"},
	} {
		if _, err := s.exec(ctx, `INSERT INTO mdm_credentials VALUES (?, 'basic', ?, ?, ?)`, row.device, []byte{}, row.username, row.password); err != nil {
			t.Fatal(err)
		}
	}
	digest := storage.MDMCredential{DeviceID: "digest", AuthType: mdm.AuthDigest, CredentialHash: []byte("digest verifier")}
	if err := s.PutMDMCredential(ctx, digest); err != nil {
		t.Fatal(err)
	}
	// An empty verifier must remain invalid after startup, never become the
	// verifier for an empty username/password pair.
	if err := s.PutMDMCredential(ctx, storage.MDMCredential{DeviceID: "missing", AuthType: mdm.AuthBasic}); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(ctx, SQLite, dsn, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, row := range []struct{ device, username, password string }{
		{"legacy", "user", "secret"}, {"empty-password", "user", ""}, {"empty-user", "", "secret"},
	} {
		got, err := s.MDMCredential(ctx, row.device)
		if err != nil {
			t.Fatal(err)
		}
		if !mdm.VerifyBasicCredential(got.CredentialHash, row.username, row.password) {
			t.Fatalf("migration did not preserve %s", row.device)
		}
		var username, password string
		if err := s.queryRow(ctx, `SELECT basic_username, basic_password FROM mdm_credentials WHERE device_id = ?`, row.device).Scan(&username, &password); err != nil {
			t.Fatal(err)
		}
		if username != "" || password != "" {
			t.Fatal("plaintext remains in active row")
		}
		if err := s.migrate(ctx); err != nil {
			t.Fatal(err)
		}
		again, err := s.MDMCredential(ctx, row.device)
		if err != nil || !bytes.Equal(got.CredentialHash, again.CredentialHash) {
			t.Fatal("migration is not idempotent")
		}
	}
	got, err := s.MDMCredential(ctx, "digest")
	if err != nil || !bytes.Equal(got.CredentialHash, digest.CredentialHash) {
		t.Fatal("migration changed digest credential")
	}
	missing, err := s.MDMCredential(ctx, "missing")
	if err != nil || mdm.VerifyBasicCredential(missing.CredentialHash, "", "") {
		t.Fatal("migration activated an empty credential")
	}
	// Replacing a migrated credential keeps the legacy columns empty.
	hash, err := mdm.HashBasicCredential("new-user", "new-secret")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.PutMDMCredential(ctx, storage.MDMCredential{DeviceID: "legacy", AuthType: mdm.AuthBasic, CredentialHash: hash}); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := s.queryRow(ctx, `SELECT COUNT(*) FROM mdm_credentials WHERE basic_username <> '' OR basic_password <> ''`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("plaintext column count = %d, %v", count, err)
	}
}

func TestBasicCredentialMigrationRollsBackOnFailure(t *testing.T) {
	ctx := context.Background()
	s := faultStore(t)
	if _, err := s.exec(ctx, `INSERT INTO mdm_credentials VALUES ('legacy', 'basic', ?, 'user', 'secret')`, []byte{}); err != nil {
		t.Fatal(err)
	}
	// Legacy query and credential update failures must
	// preserve the old credential so a later startup can retry migration.
	for _, statement := range []int64{1, 2} {
		injectAt(t, s, statement, "migrateBasicCredentials", func() error { return s.migrateBasicCredentials(ctx) })
		var password string
		if err := s.queryRow(ctx, `SELECT basic_password FROM mdm_credentials WHERE device_id = 'legacy'`).Scan(&password); err != nil || password != "secret" {
			t.Fatal("failed migration changed the credential")
		}
	}
	if _, err := s.exec(ctx, `UPDATE mdm_credentials SET basic_username = 'invalid:user' WHERE device_id = 'legacy'`); err != nil {
		t.Fatal(err)
	}
	if err := s.migrateBasicCredentials(ctx); err == nil {
		t.Fatal("unrepresentable legacy username migrated")
	}
}
