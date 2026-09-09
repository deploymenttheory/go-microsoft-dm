package storage_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/enroll"
	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/mdm"
	"github.com/deploymenttheory/go-microsoft-dm/storage"
	"github.com/deploymenttheory/go-microsoft-dm/storage/inmem"
)

var tt0 = time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)

func TestMDMAuthenticator(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := inmem.New()
	if err := store.Create(ctx, &storage.Enrollment{
		Serial: "42", Thumbprint: "TP-42", DeviceID: "DEVICE-A", EnrollmentType: enroll.EnrollmentTypeFull, EnrolledAt: tt0,
	}); err != nil {
		t.Fatal(err)
	}
	auth := &storage.MDMAuthenticator{Enrollments: store, Credentials: store}

	// No credential stored: certificate identity.
	id, err := auth.Lookup(ctx, "DEVICE-A")
	if err != nil || id.EnrollmentKey != "42" || id.AuthType != mdm.AuthCertificate {
		t.Fatalf("lookup = %+v, %v", id, err)
	}
	// With an MD5 credential.
	if err := store.PutMDMCredential(ctx, storage.MDMCredential{DeviceID: "DEVICE-A", AuthType: mdm.AuthDigest, CredentialHash: []byte{1, 2, 3}}); err != nil {
		t.Fatal(err)
	}
	id, err = auth.Lookup(ctx, "DEVICE-A")
	if err != nil || id.AuthType != mdm.AuthDigest || len(id.CredentialHash) != 3 {
		t.Errorf("md5 lookup = %+v, %v", id, err)
	}
	// Unknown device.
	if _, err := auth.Lookup(ctx, "NOPE"); !errors.Is(err, mdm.ErrUnenrolled) {
		t.Errorf("unknown = %v", err)
	}
	// Credential store not found is not fatal.
	auth2 := &storage.MDMAuthenticator{Enrollments: store, Credentials: store}
	if _, err := store.MDMCredential(ctx, "DEVICE-B"); !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("missing credential = %v", err)
	}
	if err := store.Create(ctx, &storage.Enrollment{Serial: "43", Thumbprint: "TP-43", DeviceID: "DEVICE-B", EnrollmentType: enroll.EnrollmentTypeDevice, EnrolledAt: tt0}); err != nil {
		t.Fatal(err)
	}
	if id, err := auth2.Lookup(ctx, "DEVICE-B"); err != nil || id.AuthType != mdm.AuthCertificate {
		t.Errorf("no credential lookup = %+v, %v", id, err)
	}
	// Empty device id credential.
	if err := store.PutMDMCredential(ctx, storage.MDMCredential{}); !errors.Is(err, storage.ErrInvalid) {
		t.Errorf("empty credential = %v", err)
	}
	// An authenticator without a credential store uses certificate auth.
	bare := &storage.MDMAuthenticator{Enrollments: store}
	if id, err := bare.Lookup(ctx, "DEVICE-A"); err != nil || id.AuthType != mdm.AuthCertificate {
		t.Errorf("bare lookup = %+v, %v", id, err)
	}
}
