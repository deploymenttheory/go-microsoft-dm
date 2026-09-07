package ca

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/deploymenttheory/go-microsoft-dm/clock"
)

var t0 = time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)

func newRoot(t *testing.T, opts ...Option) *Local {
	t.Helper()
	l, err := NewSelfSigned(SelfSignedOptions{Clock: clock.NewFake(t0)}, opts...)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func rsaKey(t *testing.T, bits int) *rsa.PrivateKey {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func ecKey(t *testing.T, curve elliptic.Curve) *ecdsa.PrivateKey {
	t.Helper()
	k, err := ecdsa.GenerateKey(curve, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func TestSelfSignedDefaults(t *testing.T) {
	t.Parallel()
	l := newRoot(t)
	c := l.Certificate()
	if !c.IsCA || c.Subject.CommonName != "go-microsoft-dm CA" || !c.NotBefore.Equal(t0) || !c.NotAfter.Equal(t0.Add(10*365*24*time.Hour)) {
		t.Errorf("root = %+v", c.Subject)
	}
	if _, ok := l.key.(*rsa.PrivateKey); !ok {
		t.Errorf("key type %T", l.key)
	}
	if len(l.Chain()) != 1 || l.Chain()[0] != c {
		t.Error("Chain of a root is itself")
	}
	if got := Thumbprint(c); len(got) != 40 || got != Thumbprint(c) {
		t.Errorf("thumbprint %q", got)
	}
	// A supplied key is certified as is.
	key := ecKey(t, elliptic.P256())
	l, err := NewSelfSigned(SelfSignedOptions{Key: key, Subject: pkix.Name{CommonName: "EC Root"}, Validity: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	if l.Certificate().Subject.CommonName != "EC Root" || l.Certificate().PublicKey.(*ecdsa.PublicKey).X.Cmp(key.X) != 0 {
		t.Error("supplied key not used")
	}
}

func TestSelfSignedRandomFailure(t *testing.T) {
	t.Parallel()
	if _, err := NewSelfSigned(SelfSignedOptions{Random: failingReader{}}); err == nil {
		t.Error("generate with failing random succeeded")
	}
	key := rsaKey(t, 2048)
	if _, err := NewSelfSigned(SelfSignedOptions{Key: key, Random: failingReader{}}); err == nil {
		t.Error("serial with failing random succeeded")
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

func TestNewLocalRejects(t *testing.T) {
	t.Parallel()
	root := newRoot(t)
	if _, err := NewLocal(nil, root.key); !errors.Is(err, ErrConfig) {
		t.Error("nil cert")
	}
	if _, err := NewLocal(root.Certificate(), nil); !errors.Is(err, ErrConfig) {
		t.Error("nil key")
	}
	leaf, err := root.Issue(context.Background(), Request{PublicKey: rsaKey(t, 2048).Public(), Subject: pkix.Name{CommonName: "leaf"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewLocal(leaf, root.key); !errors.Is(err, ErrConfig) {
		t.Error("non-CA accepted")
	}
	if _, err := NewLocal(root.Certificate(), root.key, WithChain(nil)); !errors.Is(err, ErrConfig) {
		t.Error("nil in chain accepted")
	}
}

func TestIssueDefaults(t *testing.T) {
	t.Parallel()
	root := newRoot(t)
	key := rsaKey(t, 2048)
	cert, err := root.Issue(context.Background(), Request{PublicKey: key.Public(), Subject: pkix.Name{CommonName: "DEVICE-1"}})
	if err != nil {
		t.Fatal(err)
	}
	if !cert.NotBefore.Equal(t0) {
		t.Errorf("NotBefore = %s, want now (never back-dated)", cert.NotBefore)
	}
	if !cert.NotAfter.Equal(t0.Add(365 * 24 * time.Hour)) {
		t.Errorf("NotAfter = %s", cert.NotAfter)
	}
	if cert.KeyUsage != x509.KeyUsageDigitalSignature|x509.KeyUsageKeyEncipherment || len(cert.ExtKeyUsage) != 1 || cert.ExtKeyUsage[0] != x509.ExtKeyUsageClientAuth {
		t.Errorf("usage = %v %v", cert.KeyUsage, cert.ExtKeyUsage)
	}
	if cert.Subject.CommonName != "DEVICE-1" || cert.SerialNumber.Sign() <= 0 || cert.IsCA {
		t.Errorf("cert = %+v", cert.Subject)
	}
	if err := cert.CheckSignatureFrom(root.Certificate()); err != nil {
		t.Error(err)
	}
	ec, err := root.Issue(context.Background(), Request{PublicKey: ecKey(t, elliptic.P384()).Public()})
	if err != nil {
		t.Fatal(err)
	}
	if ec.KeyUsage != x509.KeyUsageDigitalSignature {
		t.Errorf("EC usage = %v", ec.KeyUsage)
	}
}

func TestIssuePolicy(t *testing.T) {
	t.Parallel()
	root := newRoot(t, WithPolicy(Policy{Validity: 48 * time.Hour, KeyUsage: x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageEmailProtection},
		Subject:     &pkix.Name{CommonName: "override"}, CRLDistributionPoints: []string{"http://crl.example/r.crl"}, OCSPServer: []string{"http://ocsp.example"},
		ExtraExtensions: []pkix.Extension{{Id: []int{1, 3, 6, 1, 4, 1, 311, 66, 1, 0}, Value: []byte{0x0c, 0x01, 'x'}}}}))
	key := rsaKey(t, 2048)
	cert, err := root.Issue(context.Background(), Request{PublicKey: key.Public(), Subject: pkix.Name{CommonName: "requested"}})
	if err != nil {
		t.Fatal(err)
	}
	if cert.Subject.CommonName != "override" || !cert.NotAfter.Equal(t0.Add(48*time.Hour)) || cert.KeyUsage != x509.KeyUsageDigitalSignature ||
		len(cert.ExtKeyUsage) != 2 || len(cert.CRLDistributionPoints) != 1 || len(cert.OCSPServer) != 1 {
		t.Errorf("cert = %+v", cert)
	}
	found := false
	for _, e := range cert.Extensions {
		if e.Id.String() == "1.3.6.1.4.1.311.66.1.0" {
			found = true
		}
	}
	if !found {
		t.Error("extra extension missing")
	}
	if root.Policy().Validity != 48*time.Hour {
		t.Error("Policy()")
	}
	// A per-request policy overrides the default; NotAfter caps.
	cap := t0.Add(time.Hour)
	cert, err = root.Issue(context.Background(), Request{PublicKey: key.Public(), Policy: &Policy{NotAfter: cap}})
	if err != nil {
		t.Fatal(err)
	}
	if !cert.NotAfter.Equal(cap) || cert.Subject.CommonName != "" {
		t.Errorf("capped cert = %s %q", cert.NotAfter, cert.Subject.CommonName)
	}
}

func TestIssueRejects(t *testing.T) {
	t.Parallel()
	root := newRoot(t)
	small := rsaKey(t, 1024)
	cases := map[string]struct {
		req  Request
		want error
	}{
		"no key":        {req: Request{}, want: ErrRequest},
		"small rsa":     {req: Request{PublicKey: small.Public()}, want: ErrPolicy},
		"unknown key":   {req: Request{PublicKey: "not a key"}, want: ErrPolicy},
		"not allowed":   {req: Request{PublicKey: ecKey(t, elliptic.P256()).Public(), Policy: &Policy{AllowedKeys: []KeyKind{KeyRSA2048}}}, want: ErrPolicy},
		"unknown kind":  {req: Request{PublicKey: ecKey(t, elliptic.P224()).Public(), Policy: &Policy{AllowedKeys: []KeyKind{KeyECP256}}}, want: ErrPolicy},
		"expired cap":   {req: Request{PublicKey: ecKey(t, elliptic.P256()).Public(), Policy: &Policy{NotAfter: t0.Add(-time.Second)}}, want: ErrPolicy},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := root.Issue(context.Background(), tc.req); !errors.Is(err, tc.want) {
				t.Errorf("err = %v, want %v", err, tc.want)
			}
		})
	}
	// Allowed kinds admit exactly what they list.
	allowed := &Policy{AllowedKeys: []KeyKind{KeyRSA2048, KeyECP256}}
	if _, err := root.Issue(context.Background(), Request{PublicKey: ecKey(t, elliptic.P256()).Public(), Policy: allowed}); err != nil {
		t.Error(err)
	}
	failing, err := NewLocal(root.Certificate(), root.key, WithRandom(failingReader{}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := failing.Issue(context.Background(), Request{PublicKey: rsaKey(t, 2048).Public()}); err == nil {
		t.Error("issue with failing random succeeded")
	}
	// A CA whose key does not match its certificate cannot sign.
	mismatch, err := NewLocal(root.Certificate(), rsaKey(t, 2048))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mismatch.Issue(context.Background(), Request{PublicKey: rsaKey(t, 2048).Public()}); !errors.Is(err, ErrRequest) {
		t.Errorf("mismatched key: %v", err)
	}
}

func TestKindOf(t *testing.T) {
	t.Parallel()
	cases := map[KeyKind]any{
		KeyRSA2048: rsaKey(t, 2048).Public(), KeyRSA3072: rsaKey(t, 3072).Public(), KeyRSA4096: rsaKey(t, 4096).Public(),
		KeyECP256: ecKey(t, elliptic.P256()).Public(), KeyECP384: ecKey(t, elliptic.P384()).Public(), KeyECP521: ecKey(t, elliptic.P521()).Public(),
	}
	for want, pub := range cases {
		if got, ok := KindOf(pub); !ok || got != want {
			t.Errorf("KindOf = %q %v, want %q", got, ok, want)
		}
	}
	if _, ok := KindOf(rsaKey(t, 1024).Public()); ok {
		t.Error("rsa-1024 has a kind")
	}
	if _, ok := KindOf("x"); ok {
		t.Error("string has a kind")
	}
	if s, err := Serial(); err != nil || s.Sign() <= 0 || s.BitLen() > 128 {
		t.Errorf("Serial = %v %v", s, err)
	}
}

func TestChainAndIntermediate(t *testing.T) {
	t.Parallel()
	root := newRoot(t)
	interKey := rsaKey(t, 2048)
	interCert, err := root.Issue(context.Background(), Request{PublicKey: interKey.Public(), Subject: pkix.Name{CommonName: "Intermediate"}, Policy: &Policy{
		KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}})
	if err != nil {
		t.Fatal(err)
	}
	// Issue does not mint CAs; build one by hand to test chains.
	interCert.IsCA = true
	tmpl := &x509.Certificate{SerialNumber: interCert.SerialNumber, Subject: interCert.Subject, NotBefore: t0, NotAfter: t0.Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, root.Certificate(), interKey.Public(), root.key)
	if err != nil {
		t.Fatal(err)
	}
	interCert, err = x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	inter, err := NewLocal(interCert, interKey, WithChain(root.Certificate()), WithClock(clock.NewFake(t0)))
	if err != nil {
		t.Fatal(err)
	}
	chain := inter.Chain()
	if len(chain) != 2 || chain[0] != interCert || chain[1] != root.Certificate() {
		t.Errorf("chain = %v", chain)
	}
	leaf, err := inter.Issue(context.Background(), Request{PublicKey: ecKey(t, elliptic.P256()).Public(), Subject: pkix.Name{CommonName: "leaf"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := leaf.CheckSignatureFrom(interCert); err != nil {
		t.Error(err)
	}
}

func TestLoadPEMAndFiles(t *testing.T) {
	t.Parallel()
	root := newRoot(t)
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: root.Certificate().Raw})
	pkcs8, err := x509.MarshalPKCS8PrivateKey(root.key)
	if err != nil {
		t.Fatal(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8})
	l, err := LoadPEM(certPEM, keyPEM, certPEM)
	if err != nil {
		t.Fatal(err)
	}
	if len(l.Chain()) != 2 || !l.Certificate().Equal(root.Certificate()) {
		t.Error("loaded chain")
	}
	pkcs1 := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(root.key.(*rsa.PrivateKey))})
	if _, err := LoadPEM(certPEM, pkcs1); err != nil {
		t.Errorf("pkcs1: %v", err)
	}
	ec := ecKey(t, elliptic.P256())
	ecRoot, err := NewSelfSigned(SelfSignedOptions{Key: ec})
	if err != nil {
		t.Fatal(err)
	}
	ecDER, err := x509.MarshalECPrivateKey(ec)
	if err != nil {
		t.Fatal(err)
	}
	ecCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: ecRoot.Certificate().Raw})
	if _, err := LoadPEM(ecCertPEM, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: ecDER})); err != nil {
		t.Errorf("ec: %v", err)
	}
	bad := map[string][3][]byte{
		"no cert block":   {[]byte("junk"), keyPEM, nil},
		"wrong cert type": {keyPEM, keyPEM, nil},
		"bad cert der":    {pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte{1}}), keyPEM, nil},
		"no key block":    {certPEM, []byte("junk"), nil},
		"unknown key":     {certPEM, pem.EncodeToMemory(&pem.Block{Type: "DSA PRIVATE KEY", Bytes: []byte{1}}), nil},
		"bad key der":     {certPEM, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte{1}}), nil},
		"bad chain":       {certPEM, keyPEM, []byte("junk")},
	}
	for name, tc := range bad {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var err error
			if tc[2] != nil {
				_, err = LoadPEM(tc[0], tc[1], tc[2])
			} else {
				_, err = LoadPEM(tc[0], tc[1])
			}
			if !errors.Is(err, ErrPEM) {
				t.Errorf("err = %v", err)
			}
		})
	}
	dir := t.TempDir()
	certPath, keyPath, chainPath := filepath.Join(dir, "ca.crt"), filepath.Join(dir, "ca.key"), filepath.Join(dir, "chain.crt")
	for p, data := range map[string][]byte{certPath: certPEM, keyPath: keyPEM, chainPath: certPEM} {
		if err := os.WriteFile(p, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if l, err := LoadFiles(certPath, keyPath, chainPath); err != nil || len(l.Chain()) != 2 {
		t.Errorf("LoadFiles: %v", err)
	}
	for _, args := range [][3]string{{filepath.Join(dir, "missing"), keyPath, ""}, {certPath, filepath.Join(dir, "missing"), ""}, {certPath, keyPath, filepath.Join(dir, "missing")}} {
		var err error
		if args[2] != "" {
			_, err = LoadFiles(args[0], args[1], args[2])
		} else {
			_, err = LoadFiles(args[0], args[1])
		}
		if err == nil {
			t.Errorf("LoadFiles(%v) succeeded", args)
		}
	}
}
