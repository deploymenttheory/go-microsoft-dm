// Command setup creates disposable certificates and credentials for desktop enrollment.
// Invoke through host.ps1; runtime files belong in the ignored tmp directory.
package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/deploymenttheory/go-microsoft-dm/testpki"
)

func main() {
	dir := flag.String("dir", "", "output directory")
	upn := flag.String("upn", "host-validation@example.com", "enrollment username")
	flag.Parse()
	if err := run(*dir, *upn); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(dir, upn string) error {
	if dir == "" {
		return fmt.Errorf("output directory is required")
	}
	authority, err := testpki.NewCA("go-microsoft-dm local enrollment validation")
	if err != nil {
		return err
	}
	server, err := authority.IssueServer("localhost", time.Now().Add(-time.Minute))
	if err != nil {
		return err
	}
	root, rootKey, err := authority.PEM()
	if err != nil {
		return err
	}
	cert, key, err := server.PEM()
	if err != nil {
		return err
	}
	secret := make([]byte, 24)
	if _, err := rand.Read(secret); err != nil {
		return err
	}
	credentials, err := json.Marshal(map[string]string{
		"upn": upn, "password": hex.EncodeToString(secret), "provider": "go-microsoft-dm-local-test",
	})
	if err != nil {
		return err
	}
	for name, data := range map[string][]byte{
		"root.pem": root, "root-key.pem": rootKey, "root.cer": authority.Cert.Raw,
		"tls.pem": cert, "tls-key.pem": key, "credentials.json": credentials,
	} {
		f, err := os.OpenFile(filepath.Join(dir, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return err
		}
		_, writeErr := f.Write(data)
		closeErr := f.Close()
		if writeErr != nil {
			return writeErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}
