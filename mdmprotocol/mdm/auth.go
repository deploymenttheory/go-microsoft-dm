package mdm

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"strings"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/syncml"
)

// Errors the authentication step reports.
var (
	// ErrUnenrolled reports a device that is not enrolled.
	ErrUnenrolled = errors.New("mdm: device not enrolled")
	// ErrAuth reports an authentication failure the engine turns into a
	// challenge or a 401.
	ErrAuth = errors.New("mdm: authentication failed")
)

// AuthType names the application-layer credential the account uses.
type AuthType string

// The credential types. Certificate means the device is trusted by its
// verified TLS client certificate on each request and needs no application-layer
// credential.
const (
	AuthCertificate AuthType = "certificate"
	AuthDigest      AuthType = "md5"
	AuthBasic       AuthType = "basic"
)

// Identity is what an Authenticator knows about an enrolled device.
type Identity struct {
	// DeviceID is the OMA DM source, the enrollment's DeviceID.
	DeviceID string
	// EnrollmentKey is the certificate serial that keys the enrollment.
	EnrollmentKey string
	// CertificateThumbprint is the enrollment certificate's SHA-1 thumbprint,
	// uppercase hex, used with the serial to bind a verified TLS leaf.
	CertificateThumbprint string
	// AuthType is how the client authenticates at the application layer when
	// the TLS certificate is not matched.
	AuthType AuthType
	// CredentialHash is H(username:password) for AuthDigest, or the salted
	// verifier returned by HashBasicCredential for AuthBasic.
	CredentialHash []byte
}

// Authenticator resolves a device to its identity and supplies what the
// engine needs to verify a credential. It is implemented over the storage
// tier.
type Authenticator interface {
	// Lookup returns the identity for an OMA DM source (DeviceID), or
	// ErrUnenrolled.
	Lookup(ctx context.Context, deviceID string) (*Identity, error)
	// TrustCertificate binds a verified TLS leaf to the enrollment. The
	// engine supplies only chains from Transport.VerifiedChains whose leaf
	// is the current peer. The TLS listener must verify against enrollment
	// CA roots. A false result is final; the engine has no fallback match.
	TrustCertificate(ctx context.Context, id *Identity, verifiedChains [][]*x509.Certificate) bool
}

// authOutcome is the result of the engine's per-message authentication.
type authOutcome int

const (
	// authTrusted means the message may be acted on.
	authTrusted authOutcome = iota
	// authChallenge means send 407 with a fresh nonce and no commands.
	authChallenge
	// authRejected means send 401 with a fresh nonce; the credential was
	// wrong.
	authRejected
)

// verifyCredential checks the SyncHdr Cred against the identity and the
// nonce the engine issued. It returns whether the credential matched.
func verifyCredential(id *Identity, cred *syncml.Cred, nonce []byte) bool {
	if cred == nil {
		return false
	}
	switch id.AuthType {
	case AuthDigest:
		if cred.Meta.Type != syncml.AuthMD5 || len(nonce) == 0 || len(id.CredentialHash) == 0 {
			return false
		}
		return syncml.VerifyMD5(strings.TrimSpace(cred.Data), id.CredentialHash, nonce)
	case AuthBasic:
		if cred.Meta.Type != syncml.AuthBasic {
			return false
		}
		u, p, err := syncml.ParseBasicCredential(cred.Data)
		if err != nil {
			return false
		}
		return VerifyBasicCredential(id.CredentialHash, u, p)
	}
	return false
}

// nonceKey is the state key holding the current MD5 nonce for a device.
func nonceKey(deviceID string) string {
	return "mdm/nonce/" + strings.Map(func(r rune) rune {
		if r <= 32 || r > 126 {
			return '_'
		}
		return r
	}, deviceID)
}

func fmtNonceErr(err error) error { return fmt.Errorf("%w: %w", ErrAuth, err) }
