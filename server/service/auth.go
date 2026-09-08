package service

import (
	"context"
	"crypto/subtle"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/enroll"
)

// StaticAuthenticator accepts the on-premise enrollment credentials in Users
// (username to password). An empty map with AllowAny set accepts any
// credential, for simulated-device testing.
type StaticAuthenticator struct {
	Users    map[string]string
	AllowAny bool
}

var _ enroll.Authenticator = (*StaticAuthenticator)(nil)

// Authenticate implements enroll.Authenticator.
func (a *StaticAuthenticator) Authenticate(_ context.Context, creds enroll.Credentials) (enroll.Principal, error) {
	if a.AllowAny {
		return enroll.Principal{UPN: creds.Username}, nil
	}
	want, ok := a.Users[creds.Username]
	if !ok {
		return enroll.Principal{}, enroll.ErrUnauthenticated
	}
	if subtle.ConstantTimeCompare([]byte(want), []byte(creds.Password)) != 1 {
		return enroll.Principal{}, enroll.ErrUnauthenticated
	}
	return enroll.Principal{UPN: creds.Username}, nil
}
