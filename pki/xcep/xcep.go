package xcep

import (
	"crypto"
	"errors"
	"fmt"
	"time"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/enroll"
	"github.com/deploymenttheory/go-microsoft-dm/pki/ca"
)

// ErrOptions reports options FromPolicy cannot honour.
var ErrOptions = errors.New("xcep: invalid options")

// Options tune the derived response.
type Options struct {
	// CommonName names the policy; default "go-microsoft-dm".
	CommonName string
	// Renewal is the renewal window before expiry; default 60 days.
	Renewal time.Duration
	// Hash is the signing hash the client is told to use; default SHA-256.
	Hash crypto.Hash
	// CryptoProviders; default the two Windows providers.
	CryptoProviders []string
}

// HashOID returns the MS-XCEP oID entry for a hash.
func HashOID(h crypto.Hash) (enroll.OID, bool) {
	switch h {
	case crypto.SHA256:
		return enroll.SHA256, true
	case crypto.SHA384:
		return enroll.OID{Value: "2.16.840.1.101.3.4.2.2", Group: 4, DefaultName: "szOID_NIST_sha384"}, true
	case crypto.SHA512:
		return enroll.OID{Value: "2.16.840.1.101.3.4.2.3", Group: 4, DefaultName: "szOID_NIST_sha512"}, true
	}
	return enroll.OID{}, false
}

// FromPolicy derives the GetPolicies response from a CA policy.
func FromPolicy(p ca.Policy, o Options) (*enroll.PolicyResponse, error) {
	resp := enroll.DefaultPolicy()
	if o.CommonName != "" {
		resp.CommonName = o.CommonName
	}
	if p.Validity != 0 {
		resp.Validity = p.Validity
	}
	if o.Renewal != 0 {
		resp.Renewal = o.Renewal
	}
	if p.MinRSABits != 0 {
		resp.MinimalKeyLength = p.MinRSABits
	}
	if o.Hash != 0 {
		oid, ok := HashOID(o.Hash)
		if !ok {
			return nil, fmt.Errorf("%w: hash %s has no XCEP OID", ErrOptions, o.Hash)
		}
		resp.HashAlgorithm = oid
	}
	if o.CryptoProviders != nil {
		resp.CryptoProviders = o.CryptoProviders
	}
	if err := resp.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrOptions, err)
	}
	return resp, nil
}
