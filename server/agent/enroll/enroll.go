// Package enroll provisions a credential for the optional Windows agent.
package enroll

import "context"

// TokenStore owns credential generation and durable enrollment binding.
type TokenStore interface {
	IssueAgentToken(context.Context, string) (string, error)
}

// Issue rotates the token for the current active device enrollment. The caller
// must deliver the returned value securely; the store retains only its digest.
func Issue(ctx context.Context, store TokenStore, deviceID string) (string, error) {
	return store.IssueAgentToken(ctx, deviceID)
}
