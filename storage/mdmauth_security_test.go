package storage_test

import (
	"context"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"testing"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/mdm"
	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/wapprov"
	"github.com/deploymenttheory/go-microsoft-dm/storage"
	"github.com/deploymenttheory/go-microsoft-dm/storage/inmem"
)

func TestCertificateEnrollmentBinding(t *testing.T) {
	t.Parallel()
	leaf := &x509.Certificate{SerialNumber: big.NewInt(42), Raw: []byte("enrolled certificate"), Subject: pkix.Name{CommonName: "arbitrary CN"}}
	id := &mdm.Identity{DeviceID: "device", EnrollmentKey: "42", CertificateThumbprint: wapprov.Thumbprint(leaf.Raw)}
	other := &x509.Certificate{SerialNumber: big.NewInt(43), Raw: []byte("other certificate"), Subject: pkix.Name{CommonName: "device"}}
	sameSerial := *other
	sameSerial.SerialNumber = leaf.SerialNumber
	wrongSerial := *leaf
	wrongSerial.SerialNumber = other.SerialNumber
	for _, tc := range []struct {
		name   string
		id     *mdm.Identity
		chains [][]*x509.Certificate
		want   bool
	}{
		{"exact leaf ignores CN", id, [][]*x509.Certificate{{leaf}}, true},
		{"no chains", id, nil, false},
		{"nil identity", nil, [][]*x509.Certificate{{leaf}}, false},
		{"empty identity", &mdm.Identity{}, [][]*x509.Certificate{{leaf}}, false},
		{"missing thumbprint", &mdm.Identity{EnrollmentKey: "42"}, [][]*x509.Certificate{{leaf}}, false},
		{"empty chain", id, [][]*x509.Certificate{nil}, false},
		{"nil leaf", id, [][]*x509.Certificate{{nil}}, false},
		{"missing serial and DER", id, [][]*x509.Certificate{{{}}}, false},
		{"matching CN only", id, [][]*x509.Certificate{{other}}, false},
		{"matching serial only", id, [][]*x509.Certificate{{&sameSerial}}, false},
		{"matching thumbprint only", id, [][]*x509.Certificate{{&wrongSerial}}, false},
		{"enrollment in intermediate position", id, [][]*x509.Certificate{{other, leaf}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			auth := &storage.MDMAuthenticator{}
			if got := auth.TrustCertificate(context.Background(), tc.id, tc.chains); got != tc.want {
				t.Errorf("trusted = %v, want %v", got, tc.want)
			}
		})
	}
	auth := &storage.MDMAuthenticator{TrustCert: func(*mdm.Identity, [][]*x509.Certificate) bool { return true }}
	if auth.TrustCertificate(context.Background(), id, nil) {
		t.Error("custom policy accepted absent chains")
	}
	if !auth.TrustCertificate(context.Background(), id, [][]*x509.Certificate{{other}}) {
		t.Error("custom binding policy ignored")
	}
}

func TestBasicIdentityContainsOnlyVerifier(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := inmem.New()
	if err := s.Create(ctx, &storage.Enrollment{DeviceID: "device", Serial: "42", Thumbprint: "TP-42", EnrollmentType: "Full", EnrolledAt: tt0}); err != nil {
		t.Fatal(err)
	}
	hash, err := mdm.HashBasicCredential("user", "secret")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.PutMDMCredential(ctx, storage.MDMCredential{DeviceID: "device", AuthType: mdm.AuthBasic, CredentialHash: hash}); err != nil {
		t.Fatal(err)
	}
	auth := &storage.MDMAuthenticator{Enrollments: s, Credentials: s}
	id, err := auth.Lookup(ctx, "device")
	if err != nil {
		t.Fatal(err)
	}
	if id.CertificateThumbprint != "TP-42" || id.AuthType != mdm.AuthBasic || !mdm.VerifyBasicCredential(id.CredentialHash, "user", "secret") {
		t.Error("identity lost certificate binding or Basic verifier")
	}
}
