package syncml

import (
	"crypto/md5" //nolint:gosec // OMA DM Security 1.2.1 mandates MD5 for syncml:auth-md5
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

// CredentialHash is H(username:password), the value a server stores so it
// never needs the password (OMA DM Security 5.3.2).
func CredentialHash(username, password string) []byte {
	sum := md5.Sum([]byte(username + ":" + password)) //nolint:gosec // protocol-mandated
	return sum[:]
}

// MD5Digest computes B64(H(B64(H(username:password)):nonce)). The nonce is
// the raw bytes that were sent base64-encoded in NextNonce; MS-MDM 1.3.1
// and OMA DM Security 5.3.2 both require the binary form in the hash.
func MD5Digest(username, password string, nonce []byte) string {
	return MD5DigestFromHash(CredentialHash(username, password), nonce)
}

// MD5DigestFromHash is MD5Digest for a stored CredentialHash.
func MD5DigestFromHash(credHash, nonce []byte) string {
	inner := base64.StdEncoding.EncodeToString(credHash)
	buf := make([]byte, 0, len(inner)+1+len(nonce))
	buf = append(buf, inner...)
	buf = append(buf, ':')
	buf = append(buf, nonce...)
	sum := md5.Sum(buf) //nolint:gosec // protocol-mandated
	return base64.StdEncoding.EncodeToString(sum[:])
}

// VerifyMD5 compares a received Cred/Data with the expected digest in
// constant time.
func VerifyMD5(received string, credHash, nonce []byte) bool {
	want := MD5DigestFromHash(credHash, nonce)
	return subtle.ConstantTimeCompare([]byte(received), []byte(want)) == 1
}

// BasicCredential encodes username:password for syncml:auth-basic.
func BasicCredential(username, password string) string {
	return base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
}

// ParseBasicCredential decodes a syncml:auth-basic Cred/Data.
func ParseBasicCredential(data string) (username, password string, err error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(data))
	if err != nil {
		return "", "", fmt.Errorf("%w: basic credential: %w", ErrAuth, err)
	}
	u, p, ok := strings.Cut(string(raw), ":")
	if !ok {
		return "", "", fmt.Errorf("%w: basic credential has no ':'", ErrAuth)
	}
	return u, p, nil
}

// NewNonce returns n random bytes; OMA DM Security 5.3.3 recommends at
// least 16.
func NewNonce(n int) ([]byte, error) {
	if n < 1 {
		return nil, fmt.Errorf("%w: nonce length %d", ErrAuth, n)
	}
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrAuth, err)
	}
	return b, nil
}

// NewMD5Cred builds the Cred for a computed digest.
func NewMD5Cred(digest string) *Cred {
	return &Cred{Meta: Meta{Type: AuthMD5, Format: FormatB64}, Data: digest}
}

// NewBasicCred builds the Cred for syncml:auth-basic.
func NewBasicCred(username, password string) *Cred {
	return &Cred{Meta: Meta{Type: AuthBasic, Format: FormatB64}, Data: BasicCredential(username, password)}
}

// NewMD5Chal builds the challenge that carries the next nonce, base64 on
// the wire.
func NewMD5Chal(nonce []byte) *Chal {
	return &Chal{Meta: Meta{Type: AuthMD5, Format: FormatB64, NextNonce: base64.StdEncoding.EncodeToString(nonce)}}
}

// NewBasicChal builds a challenge for syncml:auth-basic.
func NewBasicChal() *Chal {
	return &Chal{Meta: Meta{Type: AuthBasic, Format: FormatB64}}
}

// Nonce decodes the challenge's NextNonce.
func (c *Chal) Nonce() ([]byte, error) {
	if c == nil || c.Meta.NextNonce == "" {
		return nil, fmt.Errorf("%w: challenge has no NextNonce", ErrAuth)
	}
	b, err := base64.StdEncoding.DecodeString(strings.TrimSpace(c.Meta.NextNonce))
	if err != nil {
		return nil, fmt.Errorf("%w: NextNonce: %w", ErrAuth, err)
	}
	if len(b) == 0 {
		return nil, errors.New("syncml: auth: empty NextNonce")
	}
	return b, nil
}
