package xcep

import (
	"crypto"
	"errors"
	"testing"
	"time"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/enroll"
	"github.com/deploymenttheory/go-microsoft-dm/pki/ca"
)

func TestFromPolicyDefaults(t *testing.T) {
	t.Parallel()
	got, err := FromPolicy(ca.Policy{}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	want := enroll.DefaultPolicy()
	if got.CommonName != want.CommonName || got.Validity != want.Validity || got.Renewal != want.Renewal || got.MinimalKeyLength != 2048 || got.HashAlgorithm != enroll.SHA256 || len(got.CryptoProviders) != 2 {
		t.Errorf("got %+v", got)
	}
}

func TestFromPolicyDerives(t *testing.T) {
	t.Parallel()
	got, err := FromPolicy(ca.Policy{Validity: 90 * 24 * time.Hour, MinRSABits: 3072},
		Options{CommonName: "Contoso MDM", Renewal: 30 * 24 * time.Hour, Hash: crypto.SHA384, CryptoProviders: []string{"Microsoft Software Key Storage Provider"}})
	if err != nil {
		t.Fatal(err)
	}
	if got.CommonName != "Contoso MDM" || got.Validity != 90*24*time.Hour || got.Renewal != 30*24*time.Hour || got.MinimalKeyLength != 3072 ||
		got.HashAlgorithm.Value != "2.16.840.1.101.3.4.2.2" || len(got.CryptoProviders) != 1 {
		t.Errorf("got %+v", got)
	}
	if _, err := enroll.EncodePolicyResponse(got, "r"); err != nil {
		t.Error(err)
	}
}

func TestFromPolicyRejects(t *testing.T) {
	t.Parallel()
	if _, err := FromPolicy(ca.Policy{}, Options{Hash: crypto.MD5}); !errors.Is(err, ErrOptions) {
		t.Errorf("md5: %v", err)
	}
	if _, err := FromPolicy(ca.Policy{MinRSABits: 1024}, Options{}); !errors.Is(err, ErrOptions) {
		t.Errorf("1024: %v", err)
	}
	if _, err := FromPolicy(ca.Policy{Validity: time.Hour}, Options{}); !errors.Is(err, ErrOptions) {
		t.Errorf("renewal longer than validity: %v", err)
	}
	for _, h := range []crypto.Hash{crypto.SHA256, crypto.SHA384, crypto.SHA512} {
		if _, ok := HashOID(h); !ok {
			t.Errorf("no OID for %s", h)
		}
	}
	if _, ok := HashOID(crypto.SHA1); ok {
		t.Error("SHA-1 has an OID")
	}
}
