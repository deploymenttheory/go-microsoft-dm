package testpki

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/asn1"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestIssue(t *testing.T) {
	t.Parallel()
	ca, err := NewCA("ca")
	if err != nil {
		t.Fatal(err)
	}
	a, err := ca.Issue("a", time.Now().Add(-time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	b, err := ca.Issue("b", time.Now().Add(-time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if a.Cert.SerialNumber.Cmp(b.Cert.SerialNumber) == 0 {
		t.Fatal("serials must differ")
	}
	if _, err := a.Cert.Verify(x509.VerifyOptions{Roots: ca.Pool(), KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}); err != nil {
		t.Fatalf("chain: %v", err)
	}
	if _, err := ca.IssueWithKey("c", time.Now(), nil); !errors.Is(err, ErrNilKey) {
		t.Fatalf("nil key: err = %v", err)
	}
}

func TestIssueServerAndPEM(t *testing.T) {
	t.Parallel()
	ca, err := NewCA("ca")
	if err != nil {
		t.Fatal(err)
	}
	for _, host := range []string{"enterpriseenrollment.example.test", "127.0.0.1"} {
		id, err := ca.IssueServer(host, time.Now().Add(-time.Minute))
		if err != nil {
			t.Fatal(err)
		}
		if err := id.Cert.VerifyHostname(host); err != nil {
			t.Fatalf("%s: %v", host, err)
		}
		if _, err := id.Cert.Verify(x509.VerifyOptions{Roots: ca.Pool(), DNSName: host}); err != nil {
			t.Fatalf("%s chain: %v", host, err)
		}
		certPEM, keyPEM, err := id.PEM()
		if err != nil {
			t.Fatal(err)
		}
		pair, err := tls.X509KeyPair(certPEM, keyPEM)
		if err != nil {
			t.Fatalf("round trip: %v", err)
		}
		if !bytes.Equal(pair.Certificate[0], id.Cert.Raw) {
			t.Fatal("PEM certificate does not match")
		}
	}
	if _, err := ca.IssueServer("", time.Now()); !errors.Is(err, ErrEmptyName) {
		t.Fatalf("empty host: err = %v", err)
	}
	if _, _, err := (&Identity{}).PEM(); !errors.Is(err, ErrNilKey) {
		t.Fatalf("empty identity: err = %v", err)
	}
}

func TestCSRParsesWithTheStandardLibrary(t *testing.T) {
	t.Parallel()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := CSR("device-1", key)
	if err != nil {
		t.Fatal(err)
	}
	req, err := x509.ParseCertificateRequest(der)
	if err != nil {
		t.Fatal(err)
	}
	if req.Subject.CommonName != "device-1" || req.CheckSignature() != nil {
		t.Fatalf("subject %q, signature %v", req.Subject.CommonName, req.CheckSignature())
	}
	if _, err := CSR("", key); !errors.Is(err, ErrEmptyName) {
		t.Fatalf("empty name: err = %v", err)
	}
	if _, err := CSR("x", nil); !errors.Is(err, ErrNilKey) {
		t.Fatalf("nil key: err = %v", err)
	}
}

// TestWindowsCSRIsRejectedByTheStandardLibrary pins the pitfall from research
// section 4: the request a Windows client sends cannot be parsed by
// crypto/x509, which is why every open Windows MDM server vendors or patches
// the parser. The Phase 4 parser in pki/wstep must accept exactly this input.
func TestWindowsCSRIsRejectedByTheStandardLibrary(t *testing.T) {
	t.Parallel()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := WindowsCSR(WindowsCSRSubject, key)
	if err != nil {
		t.Fatal(err)
	}
	_, err = x509.ParseCertificateRequest(der)
	if err == nil || !strings.Contains(err.Error(), "PrintableString") {
		t.Fatalf("standard parser: err = %v, want a PrintableString error", err)
	}
	// The request is otherwise well formed: the subject bytes are the ones we
	// asked for and the public key is the caller's.
	var raw struct {
		Info struct {
			Version   int
			Subject   asn1.RawValue
			PublicKey asn1.RawValue
			Rest      asn1.RawValue `asn1:"optional"`
		}
		Rest asn1.RawValue `asn1:"optional"`
	}
	if _, err := asn1.Unmarshal(der, &raw); err != nil {
		t.Fatalf("outer structure: %v", err)
	}
	if !bytes.Contains(raw.Info.Subject.FullBytes, []byte(WindowsCSRSubject)) {
		t.Fatalf("subject bytes not carried verbatim: %x", raw.Info.Subject.FullBytes)
	}
	pub, err := x509.ParsePKIXPublicKey(raw.Info.PublicKey.FullBytes)
	if err != nil {
		t.Fatal(err)
	}
	if !key.PublicKey.Equal(pub) {
		t.Fatal("public key does not match the signing key")
	}
	// A conforming subject through the same encoder parses, which shows the
	// rejection above is about the character, not the hand-built encoding.
	ok, err := WindowsCSR("F717C0F0-5F68-4AC3-A341-01B2544219DF", key)
	if err != nil {
		t.Fatal(err)
	}
	req, err := x509.ParseCertificateRequest(ok)
	if err != nil {
		t.Fatalf("printable subject: %v", err)
	}
	if req.Subject.CommonName != "F717C0F0-5F68-4AC3-A341-01B2544219DF" || req.CheckSignature() != nil {
		t.Fatalf("subject %q, signature %v", req.Subject.CommonName, req.CheckSignature())
	}
	if _, err := WindowsCSR("", key); !errors.Is(err, ErrEmptyName) {
		t.Fatalf("empty subject: err = %v", err)
	}
	if _, err := WindowsCSR(WindowsCSRSubject, nil); !errors.Is(err, ErrNilKey) {
		t.Fatalf("nil key: err = %v", err)
	}
}
