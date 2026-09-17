package sqlstore

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/deploymenttheory/go-microsoft-dm/storage"
)

// IssueAgentToken rotates the agent credential for the current enrollment.
// Only its SHA-256 digest is stored. Re-enrollment also invalidates it because
// the stored serial must match the currently active enrollment.
func (s *Store) IssueAgentToken(ctx context.Context, deviceID string) (string, error) {
	e, err := s.Get(ctx, deviceID)
	if err != nil {
		return "", err
	}
	if e.State != storage.StateActive {
		return "", storage.ErrNotFound
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("sqlstore: agent token entropy: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	digest := sha256.Sum256([]byte(token))
	hash := hex.EncodeToString(digest[:])
	err = s.tx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, s.d.rebind(`DELETE FROM agent_tokens WHERE device_id = ?`), deviceID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, s.d.rebind(`INSERT INTO agent_tokens (device_id, serial, token_hash, created_at) VALUES (?, ?, ?, ?)`), deviceID, e.Serial, hash, nanos(s.clock.Now()))
		return err
	})
	if err != nil {
		return "", err
	}
	return token, nil
}

// AgentWake authenticates a device agent and returns the newest outstanding
// user command ID. Empty means that no wake is needed.
func (s *Store) AgentWake(ctx context.Context, deviceID, token string) (string, error) {
	if token == "" || len(token) > 128 {
		return "", storage.ErrNotFound
	}
	e, err := s.Get(ctx, deviceID)
	if err != nil {
		return "", err
	}
	var serial, storedHash string
	err = s.queryRow(ctx, `SELECT serial, token_hash FROM agent_tokens WHERE device_id = ?`, deviceID).Scan(&serial, &storedHash)
	if errors.Is(err, sql.ErrNoRows) {
		return "", storage.ErrNotFound
	}
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256([]byte(token))
	actual := hex.EncodeToString(digest[:])
	if e.State != storage.StateActive || serial != e.Serial || subtle.ConstantTimeCompare([]byte(actual), []byte(storedHash)) != 1 {
		return "", storage.ErrNotFound
	}
	var id string
	err = s.queryRow(ctx, `SELECT id FROM commands WHERE device_id = ? AND internal = 0 AND state IN ('pending', 'sent') ORDER BY seq DESC LIMIT 1`, deviceID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return id, err
}
