package storage

import (
	"context"
	"crypto/x509"
	"errors"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/mdm"
	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/wapprov"
)

// MDMCredential is the application-layer credential a device uses in an OMA
// DM session, derived at enrollment from the w7 APPSRV credential the
// provisioning document set.
type MDMCredential struct {
	DeviceID string
	AuthType mdm.AuthType
	// CredentialHash is H(username:password) for AuthDigest, or the salted
	// verifier returned by mdm.HashBasicCredential for AuthBasic.
	CredentialHash []byte
}

// CredentialStore keeps the OMA DM session credential per device.
type CredentialStore interface {
	// PutMDMCredential stores or replaces the device's credential.
	PutMDMCredential(ctx context.Context, c MDMCredential) error
	// MDMCredential returns the device's credential, or ErrNotFound.
	MDMCredential(ctx context.Context, deviceID string) (MDMCredential, error)
}

// MDMAuthenticator implements mdm.Authenticator over the enrollment and
// credential stores. A device is looked up by its OMA DM source (DeviceID),
// the enrollment supplies the key, and the credential store supplies how the
// device authenticates at the application layer.
type MDMAuthenticator struct {
	Enrollments EnrollmentStore
	Credentials CredentialStore
	// TrustCert optionally replaces the enrollment binding policy for
	// verified TLS chains. Nil requires both the stored serial and thumbprint
	// to match the leaf. The engine enforces TLS verification before calling
	// this policy; it cannot opt unverified certificates into authentication.
	TrustCert func(id *mdm.Identity, verifiedChains [][]*x509.Certificate) bool
}

var _ mdm.Authenticator = (*MDMAuthenticator)(nil)

// Lookup implements mdm.Authenticator.
func (a *MDMAuthenticator) Lookup(ctx context.Context, deviceID string) (*mdm.Identity, error) {
	e, err := a.Enrollments.Get(ctx, deviceID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, mdm.ErrUnenrolled
		}
		return nil, err
	}
	id := &mdm.Identity{
		DeviceID: e.DeviceID, EnrollmentKey: e.Serial, CertificateThumbprint: e.Thumbprint, AuthType: mdm.AuthCertificate,
	}
	if a.Credentials != nil {
		if c, err := a.Credentials.MDMCredential(ctx, deviceID); err == nil {
			id.AuthType = c.AuthType
			id.CredentialHash = c.CredentialHash
		} else if !errors.Is(err, ErrNotFound) {
			return nil, err
		}
	}
	return id, nil
}

// TrustCertificate implements mdm.Authenticator.
func (a *MDMAuthenticator) TrustCertificate(_ context.Context, id *mdm.Identity, verifiedChains [][]*x509.Certificate) bool {
	if id == nil || len(verifiedChains) == 0 {
		return false
	}
	if a.TrustCert != nil {
		return a.TrustCert(id, verifiedChains)
	}
	if id.EnrollmentKey == "" || id.CertificateThumbprint == "" {
		return false
	}
	for _, chain := range verifiedChains {
		if len(chain) == 0 || chain[0] == nil {
			continue
		}
		c := chain[0]
		if c.SerialNumber != nil && len(c.Raw) > 0 && c.SerialNumber.String() == id.EnrollmentKey &&
			wapprov.Thumbprint(c.Raw) == id.CertificateThumbprint {
			return true
		}
	}
	return false
}
