package wstep

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"testing"

	"github.com/deploymenttheory/go-microsoft-dm/testpki"
)

// FuzzParseCSR mutates real requests, including the Windows-shaped one, and
// checks that the parser never panics and never accepts a request whose
// signature does not verify.
func FuzzParseCSR(f *testing.F) {
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		f.Fatal(err)
	}
	plain, _ := testpki.CSR("DEVICE", k)
	windows, _ := testpki.WindowsCSR(testpki.WindowsCSRSubject, k)
	f.Add(plain)
	f.Add(windows)
	f.Fuzz(func(t *testing.T, data []byte) {
		csr, err := ParseCSR(data)
		if err != nil {
			return
		}
		if err := csr.CheckSignature(); err != nil {
			t.Fatalf("accepted request with bad signature: %v", err)
		}
	})
}
