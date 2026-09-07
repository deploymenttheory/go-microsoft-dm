package testpki

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"sync/atomic"
	"time"
)

var (
	// ErrNilKey is returned when no key is supplied.
	ErrNilKey = errors.New("testpki: nil key")
	// ErrEmptyName is returned when a subject or host name is empty.
	ErrEmptyName = errors.New("testpki: empty name")
)

// oidCommonName is the X.520 commonName attribute (2.5.4.3).
var oidCommonName = asn1.ObjectIdentifier{2, 5, 4, 3}

// Identity is a certificate with its private key.
type Identity struct {
	Cert *x509.Certificate
	Key  crypto.Signer
}

// PEM encodes the certificate as a CERTIFICATE block and the key as a
// PKCS#8 PRIVATE KEY block.
func (i *Identity) PEM() (certPEM, keyPEM []byte, err error) {
	if i == nil || i.Cert == nil || i.Key == nil {
		return nil, nil, ErrNilKey
	}
	der, err := x509.MarshalPKCS8PrivateKey(i.Key)
	if err != nil {
		return nil, nil, fmt.Errorf("testpki: %w", err)
	}
	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: i.Cert.Raw})
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	return certPEM, keyPEM, nil
}

// CA is a test certificate authority.
type CA struct {
	Identity
	serial atomic.Int64
}

// NewCA creates a self-signed RSA CA valid for one day.
func NewCA(name string) (*CA, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("testpki: %w", err)
	}
	now := time.Now()
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: name},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(24 * time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, fmt.Errorf("testpki: %w", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, fmt.Errorf("testpki: %w", err)
	}
	ca := &CA{Identity: Identity{Cert: cert, Key: key}}
	ca.serial.Store(1)
	return ca, nil
}

// Pool returns a pool containing only this CA.
func (ca *CA) Pool() *x509.CertPool {
	p := x509.NewCertPool()
	p.AddCert(ca.Cert)
	return p
}

// Issue signs an MDM client identity (ECDSA P-256) with the given common
// name, valid from notBefore for one day. Windows puts the enrollment's
// device identifier in the common name; the caller chooses it.
func (ca *CA) Issue(commonName string, notBefore time.Time) (*Identity, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("testpki: %w", err)
	}
	return ca.IssueWithKey(commonName, notBefore, key)
}

// IssueWithKey signs an MDM client identity for an existing key.
func (ca *CA) IssueWithKey(
	commonName string,
	notBefore time.Time,
	key crypto.Signer,
) (*Identity, error) {
	return ca.issue(&x509.Certificate{
		Subject:     pkix.Name{CommonName: commonName},
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}, notBefore, key)
}

// IssueServer signs a TLS server identity (ECDSA P-256) for host, which may
// be a DNS name or an IP address, valid from notBefore for one day. It is what
// an httptest server for the enrollment or management endpoints presents.
func (ca *CA) IssueServer(host string, notBefore time.Time) (*Identity, error) {
	if host == "" {
		return nil, ErrEmptyName
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("testpki: %w", err)
	}
	tmpl := &x509.Certificate{
		Subject:     pkix.Name{CommonName: host},
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	if ip := net.ParseIP(host); ip != nil {
		tmpl.IPAddresses = []net.IP{ip}
	} else {
		tmpl.DNSNames = []string{host}
	}
	return ca.issue(tmpl, notBefore, key)
}

func (ca *CA) issue(tmpl *x509.Certificate, notBefore time.Time, key crypto.Signer) (*Identity, error) {
	if key == nil {
		return nil, ErrNilKey
	}
	tmpl.SerialNumber = big.NewInt(ca.serial.Add(1))
	tmpl.NotBefore = notBefore
	tmpl.NotAfter = notBefore.Add(24 * time.Hour)
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca.Cert, key.Public(), ca.Key)
	if err != nil {
		return nil, fmt.Errorf("testpki: %w", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, fmt.Errorf("testpki: %w", err)
	}
	return &Identity{Cert: cert, Key: key}, nil
}

// CSR returns a PKCS#10 request in DER for key with the given common name,
// the way a well-behaved client would encode it.
func CSR(commonName string, key crypto.Signer) ([]byte, error) {
	if key == nil {
		return nil, ErrNilKey
	}
	if commonName == "" {
		return nil, ErrEmptyName
	}
	der, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		Subject: pkix.Name{CommonName: commonName},
	}, key)
	if err != nil {
		return nil, fmt.Errorf("testpki: %w", err)
	}
	return der, nil
}

// WindowsCSRSubject is the common name a Windows 11 enrollment client was
// observed sending: an enrollment identifier with an exclamation mark in it,
// tagged as a PrintableString even though '!' is not in the PrintableString
// alphabet (ITU-T X.680 section 41.4). crypto/x509 rejects it as an invalid
// PrintableString.
const WindowsCSRSubject = "F717C0F0-5F68-4AC3-A341-01B254!4219DFB0A902F747A9C4FD43C8CE36CE"

// WindowsCSR returns a PKCS#10 request in DER for key whose subject common
// name is encoded as a PrintableString containing the bytes of subject
// verbatim, however invalid. Pass WindowsCSRSubject to reproduce the observed
// client behavior, or any string to probe a parser with other characters.
// The request is otherwise well formed and its signature verifies, so a
// parser that fails on it fails only for the reason Windows makes it fail.
func WindowsCSR(subject string, key crypto.Signer) ([]byte, error) {
	if key == nil {
		return nil, ErrNilKey
	}
	if subject == "" {
		return nil, ErrEmptyName
	}
	// Build the RDNSequence by hand so the string type is under our control:
	// pkix.Name would pick UTF8String for anything outside PrintableString.
	rdn := pkix.RDNSequence{{pkix.AttributeTypeAndValue{
		Type:  oidCommonName,
		Value: asn1.RawValue{Tag: asn1.TagPrintableString, Bytes: []byte(subject)},
	}}}
	raw, err := asn1.Marshal(rdn)
	if err != nil {
		return nil, fmt.Errorf("testpki: %w", err)
	}
	der, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{RawSubject: raw}, key)
	if err != nil {
		return nil, fmt.Errorf("testpki: %w", err)
	}
	return der, nil
}
