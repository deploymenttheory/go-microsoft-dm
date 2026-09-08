package storagetest

import (
	"context"
	"errors"
	"testing"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/mdm"
	"github.com/deploymenttheory/go-microsoft-dm/storage"
)

// CredentialFactory returns a fresh, empty credential store for one test.
type CredentialFactory func(t *testing.T) storage.CredentialStore

// RunCredentialSuite exercises storage.CredentialStore.
func RunCredentialSuite(t *testing.T, newStore CredentialFactory) {
	t.Helper()
	ctx := context.Background()
	s := newStore(t)

	if _, err := s.MDMCredential(ctx, "d"); !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("missing = %v", err)
	}
	if err := s.PutMDMCredential(ctx, storage.MDMCredential{}); !errors.Is(err, storage.ErrInvalid) {
		t.Errorf("empty device id = %v", err)
	}
	c := storage.MDMCredential{DeviceID: "d", AuthType: mdm.AuthDigest, CredentialHash: []byte{1, 2, 3}}
	if err := s.PutMDMCredential(ctx, c); err != nil {
		t.Fatal(err)
	}
	got, err := s.MDMCredential(ctx, "d")
	if err != nil || got.AuthType != mdm.AuthDigest || string(got.CredentialHash) != "\x01\x02\x03" {
		t.Errorf("get = %+v, %v", got, err)
	}
	// Replace.
	if err := s.PutMDMCredential(ctx, storage.MDMCredential{DeviceID: "d", AuthType: mdm.AuthBasic, BasicUsername: "u", BasicPassword: "p"}); err != nil {
		t.Fatal(err)
	}
	got, err = s.MDMCredential(ctx, "d")
	if err != nil || got.AuthType != mdm.AuthBasic || got.BasicUsername != "u" || got.BasicPassword != "p" {
		t.Errorf("replaced = %+v, %v", got, err)
	}
	// A returned credential is independent of the store.
	if len(got.CredentialHash) > 0 {
		got.CredentialHash[0] = 9
		again, _ := s.MDMCredential(ctx, "d")
		if len(again.CredentialHash) > 0 && again.CredentialHash[0] == 9 {
			t.Error("credential store shares memory")
		}
	}
}
