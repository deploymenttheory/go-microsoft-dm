package storage

import (
	"context"
	"crypto/x509"
	"errors"
	"strings"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/mdm"
)

// MDMCredential is the application-layer credential a device uses in an OMA
// DM session, derived at enrollment from the w7 APPSRV credential the
// provisioning document set.
type MDMCredential struct {
	DeviceID string
	AuthType mdm.AuthType
	// CredentialHash is H(username:password) for AuthDigest.
	CredentialHash []byte
	// BasicUsername and BasicPassword back AuthBasic.
	BasicUsername string
	BasicPassword string
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
	// TrustCert reports whether a peer certificate belongs to the
	// enrollment; nil uses the default (the subject common name contains
	// the DeviceID, as the enrollment CSR named it).
	TrustCert func(id *mdm.Identity, certs []*x509.Certificate) bool
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
	id := &mdm.Identity{DeviceID: e.DeviceID, EnrollmentKey: e.Serial, AuthType: mdm.AuthCertificate}
	if a.Credentials != nil {
		if c, err := a.Credentials.MDMCredential(ctx, deviceID); err == nil {
			id.AuthType = c.AuthType
			id.CredentialHash = c.CredentialHash
			id.BasicUsername, id.BasicPassword = c.BasicUsername, c.BasicPassword
		} else if !errors.Is(err, ErrNotFound) {
			return nil, err
		}
	}
	return id, nil
}

// TrustCertificate implements mdm.Authenticator.
func (a *MDMAuthenticator) TrustCertificate(_ context.Context, id *mdm.Identity, certs []*x509.Certificate) bool {
	if a.TrustCert != nil {
		return a.TrustCert(id, certs)
	}
	for _, c := range certs {
		if id.DeviceID != "" && strings.Contains(c.Subject.CommonName, id.DeviceID) {
			return true
		}
	}
	return false
}
