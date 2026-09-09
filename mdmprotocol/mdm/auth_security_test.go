package mdm

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/syncml"
	"github.com/deploymenttheory/go-microsoft-dm/state"
)

func TestBasicVerifier(t *testing.T) {
	t.Parallel()
	hash, err := HashBasicCredential("user", "secret:with:colons")
	if err != nil {
		t.Fatal(err)
	}
	again, err := HashBasicCredential("user", "secret:with:colons")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(hash, again) {
		t.Fatal("verifiers reused a salt")
	}
	if !VerifyBasicCredential(hash, "user", "secret:with:colons") {
		t.Error("valid credential rejected")
	}
	for _, pair := range [][2]string{{"wrong", "secret:with:colons"}, {"user", "wrong"}, {"", ""}, {"user:secret", "with:colons"}} {
		if VerifyBasicCredential(hash, pair[0], pair[1]) {
			t.Errorf("incorrect credential accepted: %q", pair[0])
		}
	}
	unknown := bytes.Clone(hash)
	unknown[0]++
	for _, invalid := range [][]byte{nil, {}, hash[:len(hash)-1], append(bytes.Clone(hash), 0), unknown, []byte("user:secret:with:colons")} {
		if VerifyBasicCredential(invalid, "user", "secret:with:colons") {
			t.Error("invalid verifier accepted")
		}
	}
	if _, err := HashBasicCredential("invalid:user", "secret"); !errors.Is(err, ErrAuth) {
		t.Errorf("invalid username: %v", err)
	}
	if verifyCredential(&Identity{AuthType: AuthBasic, CredentialHash: hash}, syncml.NewMD5Cred("digest"), nil) {
		t.Error("Basic accepted MD5 credential")
	}
}

type certificateAuth struct{ calls int }

func (*certificateAuth) Lookup(context.Context, string) (*Identity, error) {
	return &Identity{AuthType: AuthCertificate}, nil
}
func (a *certificateAuth) TrustCertificate(context.Context, *Identity, [][]*x509.Certificate) bool {
	a.calls++
	return true
}

func TestCertificateSessionRequiresVerifiedPeerOnEveryRequest(t *testing.T) {
	t.Parallel()
	leaf := &x509.Certificate{Raw: []byte("leaf")}
	other := &x509.Certificate{Raw: []byte("other")}
	for _, tc := range []struct {
		name      string
		transport *Transport
		trusted   bool
	}{
		{"nil transport", nil, false},
		{"no certificate", &Transport{}, false},
		{"unverified", &Transport{Certificates: []*x509.Certificate{leaf}}, false},
		{"nil peer", &Transport{Certificates: []*x509.Certificate{nil}, VerifiedChains: [][]*x509.Certificate{{leaf}}}, false},
		{"empty raw peer", &Transport{Certificates: []*x509.Certificate{{}}, VerifiedChains: [][]*x509.Certificate{{{}}}}, false},
		{"missing peer", &Transport{VerifiedChains: [][]*x509.Certificate{{leaf}}}, false},
		{"malformed chains", &Transport{Certificates: []*x509.Certificate{leaf}, VerifiedChains: [][]*x509.Certificate{nil, {nil}}}, false},
		{"different verified leaf", &Transport{Certificates: []*x509.Certificate{leaf}, VerifiedChains: [][]*x509.Certificate{{other, leaf}}}, false},
		{"verified leaf", &Transport{Certificates: []*x509.Certificate{leaf}, VerifiedChains: [][]*x509.Certificate{{leaf}}}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			auth := &certificateAuth{}
			svc := &Service{cfg: Config{Auth: auth, Nonce: state.NewMemory()}}
			sess := &Session{Authenticated: true, AuthType: string(AuthCertificate)}
			outcome := svc.authenticate(context.Background(), &syncml.Message{}, &Identity{AuthType: AuthCertificate}, sess, tc.transport)
			if (outcome == authTrusted) != tc.trusted || sess.Authenticated != tc.trusted {
				t.Errorf("outcome = %v, authenticated = %v", outcome, sess.Authenticated)
			}
			if (auth.calls == 1) != tc.trusted {
				t.Errorf("trust callback calls = %d", auth.calls)
			}
		})
	}
}

func TestParseTransportPreservesVerificationEvidence(t *testing.T) {
	t.Parallel()
	leaf := &x509.Certificate{Raw: []byte("leaf")}
	for _, verified := range []bool{false, true} {
		r := httptest.NewRequest("POST", "https://mdm.example.test/svc", nil)
		r.TLS = &tls.ConnectionState{PeerCertificates: []*x509.Certificate{leaf}}
		if verified {
			r.TLS.VerifiedChains = [][]*x509.Certificate{{leaf}}
		}
		tr, err := ParseTransport(r)
		if err != nil {
			t.Fatal(err)
		}
		if len(tr.Certificates) != 1 || (len(tr.verifiedPeerChains()) == 1) != verified {
			t.Fatal("transport lost or invented verification evidence")
		}
	}
}
