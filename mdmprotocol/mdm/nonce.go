package mdm

import (
	"context"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/syncml"
	"github.com/deploymenttheory/go-microsoft-dm/state"
)

// currentNonce returns the device's issued MD5 nonce, or nil when none is
// stored or it expired.
func (s *Service) currentNonce(ctx context.Context, deviceID string) []byte {
	rec, err := s.cfg.Nonce.Get(ctx, nonceKey(deviceID))
	if err != nil {
		return nil
	}
	if !rec.ExpiresAt.IsZero() && !rec.ExpiresAt.After(s.cfg.Clock.Now()) {
		return nil
	}
	return rec.Value
}

// issueNonce generates a fresh nonce, stores it with the configured life,
// and returns it. The MD5 nonce is renewed each session (MS-MDM 1.3.1).
func (s *Service) issueNonce(ctx context.Context, deviceID string) ([]byte, error) {
	nonce, err := syncml.NewNonce(16)
	if err != nil {
		return nil, fmtNonceErr(err)
	}
	key := nonceKey(deviceID)
	err = s.cfg.Nonce.Update(ctx, []string{key}, func(tx state.Tx) error {
		return tx.Put(ctx, state.Record{Key: key, Value: nonce, ExpiresAt: tx.Now().Add(s.cfg.NonceTTL)})
	})
	if err != nil {
		return nil, fmtNonceErr(err)
	}
	return nonce, nil
}
