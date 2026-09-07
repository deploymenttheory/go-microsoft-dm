package wstep

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"errors"
	"testing"
	"time"

	"github.com/deploymenttheory/go-microsoft-dm/clock"
	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/enroll"
	"github.com/deploymenttheory/go-microsoft-dm/pki/ca"
	"github.com/deploymenttheory/go-microsoft-dm/testpki"
)

var t0 = time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)

func key(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

// TestParseWindowsCSR is the research section 4 pitfall: the standard
// parser refuses the subject Windows sends; ParseCSR reads it and the
// signature still verifies over the original bytes.
func TestParseWindowsCSR(t *testing.T) {
	t.Parallel()
	k := key(t)
	der, err := testpki.WindowsCSR(testpki.WindowsCSRSubject, k)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := x509.ParseCertificateRequest(der); err == nil {
		t.Fatal("crypto/x509 accepted the Windows subject; the relaxation is no longer exercised")
	}
	csr, err := ParseCSR(der)
	if err != nil {
		t.Fatal(err)
	}
	if csr.Subject.CommonName != testpki.WindowsCSRSubject {
		t.Errorf("CommonName = %q", csr.Subject.CommonName)
	}
	if string(csr.Raw) != string(der) {
		t.Error("Raw is not the original DER")
	}
	if err := csr.CheckSignature(); err != nil {
		t.Errorf("signature over restored bytes: %v", err)
	}
	pub, ok := csr.PublicKey.(*ecdsa.PublicKey)
	if !ok || pub.X.Cmp(k.X) != 0 {
		t.Error("public key lost")
	}
	// The original subject bytes are kept, PrintableString tag and all, so
	// the standard decoder still refuses them.
	idx := bytes.Index(csr.RawSubject, []byte(testpki.WindowsCSRSubject))
	if idx < 2 || csr.RawSubject[idx-2] != asn1.TagPrintableString {
		t.Errorf("RawSubject does not carry the original PrintableString: % x", csr.RawSubject)
	}
	var rdn pkix.RDNSequence
	if _, err := asn1.Unmarshal(csr.RawSubject, &rdn); err == nil {
		t.Error("RawSubject was rewritten")
	}
	if !bytes.Equal(csr.RawTBSCertificateRequest, der[4:4+len(csr.RawTBSCertificateRequest)]) {
		t.Error("RawTBSCertificateRequest is not the original")
	}
}

func TestParseCSRPlain(t *testing.T) {
	t.Parallel()
	k := key(t)
	der, err := testpki.CSR("7BA748C8-703E-4DF2-A74A-92984117346A", k)
	if err != nil {
		t.Fatal(err)
	}
	csr, err := ParseCSR(der)
	if err != nil || csr.Subject.CommonName != "7BA748C8-703E-4DF2-A74A-92984117346A" {
		t.Errorf("plain CSR: %v", err)
	}
	// A NUL is inside the relaxation too.
	der, err = testpki.WindowsCSR("DEV\x00ID", k)
	if err != nil {
		t.Fatal(err)
	}
	if csr, err := ParseCSR(der); err != nil || csr.Subject.CommonName != "DEV\x00ID" {
		t.Errorf("NUL subject: %v", err)
	}
}

func TestParseCSRRejects(t *testing.T) {
	t.Parallel()
	k := key(t)
	outside, err := testpki.WindowsCSR("DEVICE#1", k)
	if err != nil {
		t.Fatal(err)
	}
	good, err := testpki.WindowsCSR(testpki.WindowsCSRSubject, k)
	if err != nil {
		t.Fatal(err)
	}
	tampered := append([]byte(nil), good...)
	tampered[len(tampered)-1] ^= 0xff
	plain, err := testpki.CSR("x", k)
	if err != nil {
		t.Fatal(err)
	}
	plainTampered := append([]byte(nil), plain...)
	plainTampered[len(plainTampered)-1] ^= 0xff
	pkcs7, err := asn1.Marshal(struct {
		ContentType asn1.ObjectIdentifier
		Content     asn1.RawValue `asn1:"explicit,tag:0"`
	}{ContentType: asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 7, 2}, Content: asn1.RawValue{Class: asn1.ClassUniversal, Tag: asn1.TagSequence, IsCompound: true}})
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]struct {
		der  []byte
		want error
	}{
		"empty":            {der: nil, want: ErrCSR},
		"garbage":          {der: []byte{0x30, 0x03, 0x02, 0x01}, want: ErrCSR},
		"pkcs7":            {der: pkcs7, want: ErrPKCS7},
		"outside relax":    {der: outside, want: ErrSubject},
		"relaxed tampered": {der: tampered, want: ErrSignature},
		"plain tampered":   {der: plainTampered, want: ErrSignature},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := ParseCSR(tc.der); !errors.Is(err, tc.want) {
				t.Errorf("err = %v, want %v", err, tc.want)
			}
		})
	}
	if !IsPKCS7(pkcs7) || IsPKCS7(plain) || IsPKCS7(nil) {
		t.Error("IsPKCS7")
	}
	// An OID that is not PKCS#7 is not mistaken for one.
	other, _ := asn1.Marshal(struct {
		ContentType asn1.ObjectIdentifier
	}{ContentType: asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 1}})
	if IsPKCS7(other) {
		t.Error("RSA OID taken for PKCS#7")
	}
}

// TestParseRelaxedStructuralErrors feeds the relaxed path DER that has the
// PrintableString problem but is otherwise broken.
func TestParseRelaxedStructuralErrors(t *testing.T) {
	t.Parallel()
	// A CertificationRequest whose TBS is not a proper sequence of the four
	// fields: the outer unmarshal succeeds (RawValues), the inner fails.
	badTBS, err := asn1.Marshal(struct {
		TBS       asn1.RawValue
		SigAlg    asn1.RawValue
		Signature asn1.BitString
	}{TBS: asn1.RawValue{Class: asn1.ClassUniversal, Tag: asn1.TagSequence, IsCompound: true, Bytes: []byte{0x13, 0x01, '!'}},
		SigAlg: asn1.RawValue{Class: asn1.ClassUniversal, Tag: asn1.TagSequence, IsCompound: true}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parseRelaxed(badTBS); !errors.Is(err, ErrCSR) {
		t.Errorf("bad TBS: %v", err)
	}
	if _, err := parseRelaxed([]byte{0x30, 0x00}); !errors.Is(err, ErrCSR) {
		t.Errorf("empty sequence: %v", err)
	}
	raw := func(der []byte) asn1.RawValue {
		var v asn1.RawValue
		if _, err := asn1.Unmarshal(der, &v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	// A subject that is not an RDNSequence.
	if _, err := relaxSubject(raw([]byte{0x02, 0x01, 0x01})); !errors.Is(err, ErrCSR) {
		t.Errorf("subject not sequence: %v", err)
	}
	// A set whose member is not an AttributeTypeAndValue.
	set, _ := asn1.Marshal(asn1.RawValue{Class: asn1.ClassUniversal, Tag: asn1.TagSet, IsCompound: true, Bytes: []byte{0x02, 0x01, 0x01}})
	seq, _ := asn1.Marshal(asn1.RawValue{Class: asn1.ClassUniversal, Tag: asn1.TagSequence, IsCompound: true, Bytes: set})
	if _, err := relaxSubject(raw(seq)); !errors.Is(err, ErrCSR) {
		t.Errorf("bad atv: %v", err)
	}
	// A set that is not a set.
	seq, _ = asn1.Marshal(asn1.RawValue{Class: asn1.ClassUniversal, Tag: asn1.TagSequence, IsCompound: true, Bytes: []byte{0x02, 0x01, 0x01}})
	if _, err := relaxSubject(raw(seq)); !errors.Is(err, ErrCSR) {
		t.Errorf("bad set: %v", err)
	}
	// An AttributeTypeAndValue with one part.
	one, _ := asn1.Marshal(asn1.RawValue{Class: asn1.ClassUniversal, Tag: asn1.TagSequence, IsCompound: true, Bytes: []byte{0x02, 0x01, 0x01}})
	set, _ = asn1.Marshal(asn1.RawValue{Class: asn1.ClassUniversal, Tag: asn1.TagSet, IsCompound: true, Bytes: one})
	seq, _ = asn1.Marshal(asn1.RawValue{Class: asn1.ClassUniversal, Tag: asn1.TagSequence, IsCompound: true, Bytes: set})
	if _, err := relaxSubject(raw(seq)); !errors.Is(err, ErrCSR) {
		t.Errorf("one part: %v", err)
	}
	// Long-form lengths survive re-encoding.
	long := make([]byte, 300)
	if got := tlv(0x04, long); len(got) != 304 || got[1] != 0x82 || got[2] != 0x01 || got[3] != 0x2c {
		t.Errorf("tlv long form = % x", got[:4])
	}
	// Truncated children.
	seq, _ = asn1.Marshal(asn1.RawValue{Class: asn1.ClassUniversal, Tag: asn1.TagSequence, IsCompound: true, Bytes: []byte{0x31, 0x05, 0x02}})
	if _, err := relaxSubject(raw(seq)); !errors.Is(err, ErrCSR) {
		t.Errorf("truncated: %v", err)
	}
	// A subject that needs no relaxation passes through unchanged.
	rdn, _ := asn1.Marshal(pkix.RDNSequence{{pkix.AttributeTypeAndValue{Type: asn1.ObjectIdentifier{2, 5, 4, 3}, Value: "plain"}}})
	out, err := relaxSubject(raw(rdn))
	if err != nil || string(out) != string(rdn) {
		t.Errorf("unchanged subject: %v", err)
	}
}

func newIssuer(t *testing.T, opts ...Option) (*Issuer, *ca.Local) {
	t.Helper()
	root, err := ca.NewSelfSigned(ca.SelfSignedOptions{Clock: clock.NewFake(t0)})
	if err != nil {
		t.Fatal(err)
	}
	iss, err := NewIssuer(root, opts...)
	if err != nil {
		t.Fatal(err)
	}
	return iss, root
}

func TestIssuerIssuesWindowsCSR(t *testing.T) {
	t.Parallel()
	iss, root := newIssuer(t)
	k := key(t)
	der, err := testpki.WindowsCSR(testpki.WindowsCSRSubject, k)
	if err != nil {
		t.Fatal(err)
	}
	got, err := iss.Issue(context.Background(), &enroll.Request{CSR: der, Context: enroll.AdditionalContext{DeviceID: "d1"}})
	if err != nil {
		t.Fatal(err)
	}
	cert := got.Certificate
	if cert.Subject.CommonName != testpki.WindowsCSRSubject || !cert.NotBefore.Equal(t0) {
		t.Errorf("cert = %+v %s", cert.Subject, cert.NotBefore)
	}
	// The issued certificate is one crypto/x509 reads without help.
	if _, err := x509.ParseCertificate(cert.Raw); err != nil {
		t.Error(err)
	}
	if err := cert.CheckSignatureFrom(root.Certificate()); err != nil {
		t.Error(err)
	}
	if len(got.Chain) != 1 || got.Chain[0] != root.Certificate() {
		t.Error("chain")
	}
}

func TestIssuerOptionsAndErrors(t *testing.T) {
	t.Parallel()
	if _, err := NewIssuer(nil); !errors.Is(err, ErrConfig) {
		t.Error("nil CA accepted")
	}
	iss, _ := newIssuer(t,
		WithPolicy(ca.Policy{Validity: time.Hour}),
		WithSubject(func(req *enroll.Request, _ *x509.CertificateRequest) pkix.Name {
			return pkix.Name{CommonName: req.Context.DeviceID}
		}))
	k := key(t)
	der, err := testpki.CSR("csr-name", k)
	if err != nil {
		t.Fatal(err)
	}
	got, err := iss.Issue(context.Background(), &enroll.Request{CSR: der, Context: enroll.AdditionalContext{DeviceID: "DEVICE-ID"}})
	if err != nil {
		t.Fatal(err)
	}
	if got.Certificate.Subject.CommonName != "DEVICE-ID" || !got.Certificate.NotAfter.Equal(t0.Add(time.Hour)) {
		t.Errorf("cert = %+v", got.Certificate.Subject)
	}
	if _, err := iss.Issue(context.Background(), nil); !errors.Is(err, ErrCSR) {
		t.Error("nil request")
	}
	if _, err := iss.Issue(context.Background(), &enroll.Request{CSR: []byte("junk")}); !errors.Is(err, ErrCSR) {
		t.Error("junk CSR")
	}
	// The CA's policy refuses what the CSR asks for.
	strict, _ := newIssuer(t, WithPolicy(ca.Policy{AllowedKeys: []ca.KeyKind{ca.KeyRSA2048}}))
	if _, err := strict.Issue(context.Background(), &enroll.Request{CSR: der}); !errors.Is(err, ca.ErrPolicy) {
		t.Errorf("policy: %v", err)
	}
}
