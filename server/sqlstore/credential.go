package sqlstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/mdm"
	"github.com/deploymenttheory/go-microsoft-dm/storage"
)

// PutMDMCredential implements storage.CredentialStore.
func (s *Store) PutMDMCredential(ctx context.Context, c storage.MDMCredential) error {
	if c.DeviceID == "" {
		return fmt.Errorf("%w: empty device id", storage.ErrInvalid)
	}
	hash := c.CredentialHash
	if hash == nil {
		hash = []byte{}
	}
	cols := []string{"auth_type", "credential_hash", "basic_username", "basic_password"}
	_, err := s.exec(ctx,
		`INSERT INTO mdm_credentials (device_id, auth_type, credential_hash, basic_username, basic_password) VALUES (?, ?, ?, ?, ?)`+
			s.d.upsert([]string{"device_id"}, cols),
		c.DeviceID, string(c.AuthType), hash, "", "")
	return err
}

// MDMCredential implements storage.CredentialStore.
func (s *Store) MDMCredential(ctx context.Context, deviceID string) (storage.MDMCredential, error) {
	var (
		c        storage.MDMCredential
		authType string
	)
	c.DeviceID = deviceID
	err := s.queryRow(ctx, `SELECT auth_type, credential_hash FROM mdm_credentials WHERE device_id = ?`, deviceID).
		Scan(&authType, &c.CredentialHash)
	if errors.Is(err, sql.ErrNoRows) {
		return storage.MDMCredential{}, fmt.Errorf("%w: credential for %q", storage.ErrNotFound, deviceID)
	}
	if err != nil {
		return storage.MDMCredential{}, err
	}
	c.AuthType = mdm.AuthType(authType)
	return c, nil
}
