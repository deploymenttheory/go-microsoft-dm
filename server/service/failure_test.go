package service

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/deploymenttheory/go-microsoft-dm/clock"
	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/enroll"
	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/mdm"
	"github.com/deploymenttheory/go-microsoft-dm/paging"
	"github.com/deploymenttheory/go-microsoft-dm/server/sqlstore"
	"github.com/deploymenttheory/go-microsoft-dm/storage"
)

var errBoom = errors.New("boom")

// fakeStore implements service.Store with per-method injectable errors and
// a settable active enrollment for Get.
type fakeStore struct {
	fail        map[string]error
	active      *storage.Enrollment
	setStateErr error
	revokeErr   error
	events      int
	creds       int
}

func (f *fakeStore) e(m string) error { return f.fail[m] }

// EnrollmentStore
func (f *fakeStore) Create(context.Context, *storage.Enrollment) error { return f.e("Create") }
func (f *fakeStore) Get(context.Context, string) (*storage.Enrollment, error) {
	if err := f.e("Get"); err != nil {
		return nil, err
	}
	if f.active == nil {
		return nil, storage.ErrNotFound
	}
	return f.active, nil
}
func (f *fakeStore) GetBySerial(context.Context, string) (*storage.Enrollment, error) {
	return nil, storage.ErrNotFound
}
func (f *fakeStore) GetByThumbprint(context.Context, string) (*storage.Enrollment, error) {
	return nil, storage.ErrNotFound
}
func (f *fakeStore) ListByHWDevID(context.Context, string) ([]storage.Enrollment, error) {
	return nil, nil
}
func (f *fakeStore) SetState(context.Context, string, storage.State, time.Time) error {
	return f.setStateErr
}
func (f *fakeStore) TouchLastSeen(context.Context, string, time.Time) error { return nil }
func (f *fakeStore) List(context.Context, storage.EnrollmentQuery, paging.Page) (paging.Result[storage.Enrollment], error) {
	return paging.Result[storage.Enrollment]{}, nil
}

// CertificateStore
func (f *fakeStore) PutCertificate(context.Context, *storage.Certificate) error { return nil }
func (f *fakeStore) Certificate(context.Context, string) (*storage.Certificate, error) {
	return nil, storage.ErrNotFound
}
func (f *fakeStore) CertificateByThumbprint(context.Context, string) (*storage.Certificate, error) {
	return nil, storage.ErrNotFound
}
func (f *fakeStore) Revoke(context.Context, string, time.Time) error { return f.revokeErr }
func (f *fakeStore) ListCertificates(context.Context, storage.CertificateQuery, paging.Page) (paging.Result[storage.Certificate], error) {
	return paging.Result[storage.Certificate]{}, nil
}

// CredentialStore
func (f *fakeStore) PutMDMCredential(context.Context, storage.MDMCredential) error {
	if err := f.e("PutMDMCredential"); err != nil {
		return err
	}
	f.creds++
	return nil
}
func (f *fakeStore) MDMCredential(context.Context, string) (storage.MDMCredential, error) {
	return storage.MDMCredential{}, storage.ErrNotFound
}

// FactsStore + EventLog
func (f *fakeStore) PutFacts(context.Context, sqlstore.Facts) error { return f.e("PutFacts") }
func (f *fakeStore) LogEvent(context.Context, string, string, any) error {
	if err := f.e("LogEvent"); err != nil {
		return err
	}
	f.events++
	return nil
}

var _ Store = (*fakeStore)(nil)

func enrollmentFor(deviceID string) *enroll.Enrollment {
	return &enroll.Enrollment{Request: &enroll.Request{Context: enroll.AdditionalContext{DeviceID: deviceID}}}
}

func TestCredentialSourceStoreFailure(t *testing.T) {
	t.Parallel()
	cs := &credentialSource{store: &fakeStore{fail: map[string]error{"PutMDMCredential": errBoom}}, clock: clock.Real{}}
	if _, _, err := cs.Credentials(context.Background(), enrollmentFor("d")); !errors.Is(err, errBoom) {
		t.Errorf("err = %v", err)
	}
}

func TestHooks(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	// ClientEvent, GenericAlert and PackageOne happy paths.
	fs := &fakeStore{}
	h := &hooks{store: fs, clock: clock.NewFake(time.Now())}
	if err := h.PackageOne(ctx, "d", "1", mdm.Facts{DevInfo: map[string]string{"Man": "x"}, LoginStatus: "user"}); err != nil {
		t.Fatal(err)
	}
	if err := h.ClientEvent(ctx, mdm.Event{DeviceID: "d", Type: "t", Data: "v"}); err != nil {
		t.Fatal(err)
	}
	if err := h.GenericAlert(ctx, mdm.Event{DeviceID: "d", Type: "t"}); err != nil {
		t.Fatal(err)
	}
	// PackageOne surfaces a facts-store failure and a log failure.
	if err := (&hooks{store: &fakeStore{fail: map[string]error{"PutFacts": errBoom}}, clock: clock.Real{}}).PackageOne(ctx, "d", "1", mdm.Facts{}); !errors.Is(err, errBoom) {
		t.Errorf("facts fail = %v", err)
	}
	if err := (&hooks{store: &fakeStore{fail: map[string]error{"LogEvent": errBoom}}, clock: clock.Real{}}).PackageOne(ctx, "d", "1", mdm.Facts{}); !errors.Is(err, errBoom) {
		t.Errorf("log fail = %v", err)
	}
	// Unenrolled: no active enrollment logs and returns.
	if err := (&hooks{store: &fakeStore{}, clock: clock.Real{}}).Unenrolled(ctx, "d"); err != nil {
		t.Errorf("unenroll no-enrollment = %v", err)
	}
	// Unenrolled: Get error propagates.
	if err := (&hooks{store: &fakeStore{fail: map[string]error{"Get": errBoom}}, clock: clock.Real{}}).Unenrolled(ctx, "d"); !errors.Is(err, errBoom) {
		t.Errorf("unenroll get fail = %v", err)
	}
	// Unenrolled: active enrollment is unenrolled and revoked.
	fs = &fakeStore{active: &storage.Enrollment{Serial: "7", DeviceID: "d", State: storage.StateActive}}
	if err := (&hooks{store: fs, clock: clock.Real{}}).Unenrolled(ctx, "d"); err != nil {
		t.Fatalf("unenroll active = %v", err)
	}
	// Unenrolled: a SetState failure surfaces.
	if err := (&hooks{store: &fakeStore{active: &storage.Enrollment{Serial: "7"}, setStateErr: errBoom}, clock: clock.Real{}}).Unenrolled(ctx, "d"); !errors.Is(err, errBoom) {
		t.Errorf("unenroll setstate fail = %v", err)
	}
}

func TestConflictSink(t *testing.T) {
	t.Parallel()
	fs := &fakeStore{}
	sink := conflictSink{log: fs}
	c := storage.Conflict{HWDevID: "HW", New: storage.Enrollment{DeviceID: "new"}, Existing: []storage.Enrollment{{DeviceID: "old"}}}
	if err := sink.HWDevIDConflict(context.Background(), c); err != nil || fs.events != 1 {
		t.Errorf("conflict = %v, events %d", err, fs.events)
	}
}

func TestCredentialSourceSuccess(t *testing.T) {
	t.Parallel()
	fs := &fakeStore{}
	cs := &credentialSource{store: fs, clock: clock.Real{}}
	server, client, err := cs.Credentials(context.Background(), enrollmentFor("dev-1"))
	if err != nil {
		t.Fatalf("Credentials: %v", err)
	}
	if server.Name != "dev-1" || server.Secret == "" {
		t.Errorf("server cred = %+v", server)
	}
	if client.Secret == "" || client.Name != "" {
		t.Errorf("client cred = %+v", client)
	}
	if len(server.Nonce) != 16 || len(client.Nonce) != 16 || bytes.Equal(server.Nonce, client.Nonce) {
		t.Fatal("credentials need independent 16-byte initial digest nonces")
	}
	if fs.creds != 1 {
		t.Errorf("stored %d credentials, want 1", fs.creds)
	}
}

func TestRandomSecretFailure(t *testing.T) {
	orig := randRead
	randRead = func([]byte) (int, error) { return 0, errBoom }
	t.Cleanup(func() { randRead = orig })
	// The server secret is drawn first, so its failure surfaces.
	cs := &credentialSource{store: &fakeStore{}, clock: clock.Real{}}
	if _, _, err := cs.Credentials(context.Background(), enrollmentFor("d")); !errors.Is(err, errBoom) {
		t.Errorf("server secret err = %v", err)
	}
	// A source that yields the server secret then fails exercises the client
	// secret branch.
	calls := 0
	randRead = func(b []byte) (int, error) {
		calls++
		if calls == 1 {
			return len(b), nil
		}
		return 0, errBoom
	}
	if _, _, err := cs.Credentials(context.Background(), enrollmentFor("d")); !errors.Is(err, errBoom) {
		t.Errorf("client secret err = %v", err)
	}
}

func TestCredentialNonceFailureDoesNotPersist(t *testing.T) {
	orig := randRead
	t.Cleanup(func() { randRead = orig })
	calls := 0
	randRead = func(b []byte) (int, error) {
		calls++
		if calls <= 2 {
			return len(b), nil
		}
		return 0, errBoom
	}
	fs := &fakeStore{}
	cs := &credentialSource{store: fs, clock: clock.Real{}}
	server, client, err := cs.Credentials(context.Background(), enrollmentFor("d"))
	if !errors.Is(err, errBoom) {
		t.Fatalf("nonce error = %v", err)
	}
	if fs.creds != 0 || server.Secret != "" || client.Secret != "" {
		t.Fatal("nonce failure must not persist or return partial credentials")
	}
}
