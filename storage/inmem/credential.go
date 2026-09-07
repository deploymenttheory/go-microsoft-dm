package inmem

import (
	"context"
	"fmt"

	"github.com/deploymenttheory/go-microsoft-dm/storage"
)

var _ storage.CredentialStore = (*Store)(nil)

// PutMDMCredential implements storage.CredentialStore.
func (s *Store) PutMDMCredential(_ context.Context, c storage.MDMCredential) error {
	if c.DeviceID == "" {
		return fmt.Errorf("%w: empty device id", storage.ErrInvalid)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rec := c
	rec.CredentialHash = append([]byte(nil), c.CredentialHash...)
	s.creds[c.DeviceID] = rec
	return nil
}

// MDMCredential implements storage.CredentialStore.
func (s *Store) MDMCredential(_ context.Context, deviceID string) (storage.MDMCredential, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.creds[deviceID]
	if !ok {
		return storage.MDMCredential{}, fmt.Errorf("%w: credential for %q", storage.ErrNotFound, deviceID)
	}
	out := c
	out.CredentialHash = append([]byte(nil), c.CredentialHash...)
	return out, nil
}
