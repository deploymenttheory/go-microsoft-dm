package syncml_test

import (
	"encoding/base64"
	"errors"
	"testing"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/syncml"
)

// The Bruce2/OhBehave/Nonce digest is the worked example in OMA DM Protocol
// 1.2.1 section 9.4.2; the others were computed independently with Python's
// hashlib as b64(md5(b64(md5("user:pass")) + ":" + nonce)).
func TestMD5DigestKnownAnswers(t *testing.T) {
	t.Parallel()
	if got := syncml.MD5Digest("Bruce2", "OhBehave", []byte("Nonce")); got != "Zz6EivR3yeaaENcRN6lpAQ==" {
		t.Fatalf("digest = %q", got)
	}
	raw := make([]byte, 16)
	for i := range raw {
		raw[i] = byte(i)
	}
	if got := syncml.MD5Digest("user@example.test", "s3cret", raw); got != "5cvSZnmJL4pMBKyOWYaRaA==" {
		t.Fatalf("digest = %q", got)
	}
	if got := base64.StdEncoding.EncodeToString(syncml.CredentialHash("Bruce2", "OhBehave")); got != "PtEdr8lBQ45IbT1bZIkrOQ==" {
		t.Fatalf("hash = %q", got)
	}
	hash := syncml.CredentialHash("Bruce2", "OhBehave")
	if !syncml.VerifyMD5("Zz6EivR3yeaaENcRN6lpAQ==", hash, []byte("Nonce")) {
		t.Fatal("verify rejected the right digest")
	}
	if syncml.VerifyMD5("Zz6EivR3yeaaENcRN6lpAQ==", hash, []byte("Stale")) || syncml.VerifyMD5("", hash, []byte("Nonce")) || syncml.VerifyMD5("Zz6EivR3yeaaENcRN6lpAQ==", syncml.CredentialHash("Bruce2", "wrong"), []byte("Nonce")) {
		t.Fatal("verify accepted a wrong digest")
	}
}

func TestMD5ChalAndCredRoundTrip(t *testing.T) {
	t.Parallel()
	nonce, err := syncml.NewNonce(16)
	if err != nil || len(nonce) != 16 {
		t.Fatal(err)
	}
	other, _ := syncml.NewNonce(16)
	if string(other) == string(nonce) {
		t.Fatal("nonces repeat")
	}
	chal := syncml.NewMD5Chal(nonce)
	got, err := chal.Nonce()
	if err != nil || string(got) != string(nonce) || chal.Meta.Type != syncml.AuthMD5 || chal.Meta.Format != syncml.FormatB64 {
		t.Fatalf("chal %+v err %v", chal, err)
	}
	// The wire form is base64 and the hash uses the raw bytes (MS-MDM 1.3.1).
	if chal.Meta.NextNonce != base64.StdEncoding.EncodeToString(nonce) {
		t.Fatal("NextNonce is not base64")
	}
	cred := syncml.NewMD5Cred(syncml.MD5Digest("u", "p", nonce))
	if cred.Meta.Type != syncml.AuthMD5 || !syncml.VerifyMD5(cred.Data, syncml.CredentialHash("u", "p"), nonce) {
		t.Fatal("cred does not verify")
	}
	for name, c := range map[string]*syncml.Chal{
		"nil":            nil,
		"empty":          {},
		"not base64":     {Meta: syncml.Meta{NextNonce: "%%%"}},
		"decodes to nil": {Meta: syncml.Meta{NextNonce: ""}},
	} {
		if _, err := c.Nonce(); !errors.Is(err, syncml.ErrAuth) && err == nil {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	if _, err := (&syncml.Chal{Meta: syncml.Meta{NextNonce: "%%%"}}).Nonce(); !errors.Is(err, syncml.ErrAuth) {
		t.Fatal("bad base64 must be ErrAuth")
	}
	if _, err := (&syncml.Chal{Meta: syncml.Meta{NextNonce: "===="}}).Nonce(); err == nil {
		t.Fatal("empty decode must fail")
	}
	if _, err := syncml.NewNonce(0); !errors.Is(err, syncml.ErrAuth) {
		t.Fatal("NewNonce(0)")
	}
	if b := syncml.NewBasicChal(); b.Meta.Type != syncml.AuthBasic {
		t.Fatal("basic chal")
	}
}

func TestBasicCredential(t *testing.T) {
	t.Parallel()
	// OMA DM Protocol 9.4.1: Bruce2:OhBehave is QnJ1Y2UyOk9oQmVoYXZl.
	if syncml.BasicCredential("Bruce2", "OhBehave") != "QnJ1Y2UyOk9oQmVoYXZl" {
		t.Fatal("basic credential differs from the OMA example")
	}
	c := syncml.NewBasicCred("domain\\user", "p:w")
	if c.Meta.Type != syncml.AuthBasic || c.Data != base64.StdEncoding.EncodeToString([]byte("domain\\user:p:w")) {
		t.Fatalf("cred %+v", c)
	}
	u, p, err := syncml.ParseBasicCredential(c.Data)
	if err != nil || u != "domain\\user" || p != "p:w" {
		t.Fatalf("parsed %q %q %v", u, p, err)
	}
	if _, _, err := syncml.ParseBasicCredential("!!!"); !errors.Is(err, syncml.ErrAuth) {
		t.Fatal("bad base64")
	}
	if _, _, err := syncml.ParseBasicCredential(base64.StdEncoding.EncodeToString([]byte("nocolon"))); !errors.Is(err, syncml.ErrAuth) {
		t.Fatal("no colon")
	}
}
