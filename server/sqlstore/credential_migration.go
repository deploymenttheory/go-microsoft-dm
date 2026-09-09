package sqlstore

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/mdm"
)

// migrateBasicCredentials replaces legacy plaintext Basic credentials and clears
// their old columns atomically. The columns remain for schema compatibility;
// current writes leave them empty. Run with all old server instances stopped.
func (s *conn) migrateBasicCredentials(ctx context.Context) error {
	return s.tx(ctx, func(tx *sql.Tx) error {
		credentials, err := s.legacyBasicCredentials(ctx, tx)
		if err != nil {
			return err
		}
		for _, c := range credentials {
			_, err := tx.ExecContext(ctx, s.d.rebind(`UPDATE mdm_credentials
				SET credential_hash = ?, basic_username = '', basic_password = '' WHERE device_id = ?`), c.hash, c.deviceID)
			if err != nil {
				return fmt.Errorf("sqlstore: migrate basic credential: %w", err)
			}
		}
		return nil
	})
}

type migratedBasicCredential struct {
	deviceID string
	hash     []byte
}

// Close the result set before updating rows; SQLite uses one connection and
// MySQL cannot execute another statement while rows are still being streamed.
func (s *conn) legacyBasicCredentials(ctx context.Context, tx *sql.Tx) ([]migratedBasicCredential, error) {
	rows, err := tx.QueryContext(ctx, s.d.rebind(`SELECT device_id, basic_username, basic_password
		FROM mdm_credentials WHERE auth_type = ? AND (basic_username <> '' OR basic_password <> '')`), string(mdm.AuthBasic))
	if err != nil {
		return nil, fmt.Errorf("sqlstore: read legacy basic credentials: %w", err)
	}
	defer rows.Close()
	var credentials []migratedBasicCredential
	for rows.Next() {
		var deviceID, username, password string
		if err := rows.Scan(&deviceID, &username, &password); err != nil {
			return nil, fmt.Errorf("sqlstore: scan legacy basic credential: %w", err)
		}
		hash, err := mdm.HashBasicCredential(username, password)
		if err != nil {
			return nil, fmt.Errorf("sqlstore: hash legacy basic credential: %w", err)
		}
		credentials = append(credentials, migratedBasicCredential{deviceID: deviceID, hash: hash})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlstore: iterate legacy basic credentials: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("sqlstore: close legacy basic credentials: %w", err)
	}
	return credentials, nil
}
