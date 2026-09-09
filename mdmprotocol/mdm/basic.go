package mdm

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"strings"
)

const (
	basicVerifierVersion = 1
	basicSaltSize        = 16
	basicIterations      = 600_000
	basicVerifierSize    = 1 + basicSaltSize + sha256.Size
)

// HashBasicCredential returns a versioned, salted PBKDF2-HMAC-SHA256 verifier
// for storage in Identity.CredentialHash. Version 1 uses 600,000 iterations,
// a random 16-byte salt and a 32-byte key. It binds both username and password;
// usernames containing ':' cannot be represented by the Basic wire format.
func HashBasicCredential(username, password string) ([]byte, error) {
	if strings.Contains(username, ":") {
		return nil, fmt.Errorf("%w: basic username contains ':'", ErrAuth)
	}
	verifier := make([]byte, basicVerifierSize)
	verifier[0] = basicVerifierVersion
	salt := verifier[1 : 1+basicSaltSize]
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("%w: basic salt: %w", ErrAuth, err)
	}
	key, err := pbkdf2.Key(sha256.New, username+":"+password, salt, basicIterations, sha256.Size)
	if err != nil {
		return nil, fmt.Errorf("%w: basic verifier: %w", ErrAuth, err)
	}
	copy(verifier[1+basicSaltSize:], key)
	return verifier, nil
}

// VerifyBasicCredential checks both credential components with a constant-time
// comparison of fixed-size derived keys. Missing, malformed and unknown-version
// verifiers fail closed; no plaintext or digest-credential fallback is accepted.
func VerifyBasicCredential(verifier []byte, username, password string) bool {
	if len(verifier) != basicVerifierSize || verifier[0] != basicVerifierVersion || strings.Contains(username, ":") {
		return false
	}
	key, err := pbkdf2.Key(sha256.New, username+":"+password, verifier[1:1+basicSaltSize], basicIterations, sha256.Size)
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(key, verifier[1+basicSaltSize:]) == 1
}
