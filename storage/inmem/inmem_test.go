package inmem_test

import (
	"context"
	"errors"
	"testing"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/mdm"
	"github.com/deploymenttheory/go-microsoft-dm/storage"
	"github.com/deploymenttheory/go-microsoft-dm/storage/inmem"
	"github.com/deploymenttheory/go-microsoft-dm/storage/storagetest"
)

func TestContract(t *testing.T) {
	t.Parallel()
	storagetest.RunAll(t, func(*testing.T) storage.Store { return inmem.New() })
}

func TestQueueContract(t *testing.T) {
	t.Parallel()
	storagetest.RunQueueSuite(t, func(*testing.T) mdm.CommandQueue { return inmem.NewQueue() })
}

func TestCredentialStore(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := inmem.New()
	if _, err := s.MDMCredential(ctx, "d"); !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("missing = %v", err)
	}
	if err := s.PutMDMCredential(ctx, storage.MDMCredential{}); !errors.Is(err, storage.ErrInvalid) {
		t.Errorf("empty = %v", err)
	}
	c := storage.MDMCredential{DeviceID: "d", AuthType: mdm.AuthDigest, CredentialHash: []byte{1, 2}}
	if err := s.PutMDMCredential(ctx, c); err != nil {
		t.Fatal(err)
	}
	got, err := s.MDMCredential(ctx, "d")
	if err != nil || got.AuthType != mdm.AuthDigest || len(got.CredentialHash) != 2 {
		t.Errorf("got %+v, %v", got, err)
	}
	// Stored copy is independent.
	got.CredentialHash[0] = 9
	again, _ := s.MDMCredential(ctx, "d")
	if again.CredentialHash[0] != 1 {
		t.Error("credential store shares memory")
	}
}
