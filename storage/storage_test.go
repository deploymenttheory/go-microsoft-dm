package storage_test

import (
	"context"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/enroll"
	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/wapprov"
	"github.com/deploymenttheory/go-microsoft-dm/paging"
	"github.com/deploymenttheory/go-microsoft-dm/storage"
	"github.com/deploymenttheory/go-microsoft-dm/storage/inmem"
	"github.com/deploymenttheory/go-microsoft-dm/storage/storagetest"
	"github.com/deploymenttheory/go-microsoft-dm/testpki"
)

var t0 = time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)

func issued(t *testing.T, ca *testpki.CA, serial int64, cn string) *x509.Certificate {
	t.Helper()
	id, err := ca.Issue(cn, t0)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: cn}, NotBefore: t0, NotAfter: t0.Add(time.Hour)}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca.Cert, id.Key.Public(), ca.Key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return cert
}

func enrollment(cert *x509.Certificate, deviceID, hwDevID string, et enroll.EnrollmentType) *enroll.Enrollment {
	return &enroll.Enrollment{
		Request: &enroll.Request{Context: enroll.AdditionalContext{DeviceID: deviceID, HWDevID: hwDevID, EnrollmentType: et, DeviceName: "PC"},
			Principal: enroll.Principal{UPN: "user@contoso.com"}},
		Certificate: cert, EnrolledAt: t0, Store: wapprov.StoreUser,
	}
}

func TestRecorderRecordsAndDetectsConflicts(t *testing.T) {
	t.Parallel()
	ca, err := testpki.NewCA("Test CA")
	if err != nil {
		t.Fatal(err)
	}
	store := inmem.New()
	var conflicts []storage.Conflict
	rec := &storage.Recorder{Store: store, Conflicts: storage.ConflictSinkFunc(func(_ context.Context, c storage.Conflict) error {
		conflicts = append(conflicts, c)
		return nil
	})}
	ctx := context.Background()
	c1 := issued(t, ca, 1, "DEVICE-A")
	if err := rec.Record(ctx, enrollment(c1, "DEVICE-A", "HW-1", enroll.EnrollmentTypeFull), nil); err != nil {
		t.Fatal(err)
	}
	got, err := store.Get(ctx, "DEVICE-A")
	if err != nil || got.Serial != "1" || got.Thumbprint != wapprov.Thumbprint(c1.Raw) || got.UPN != "user@contoso.com" || got.HWDevID != "HW-1" || got.Context.DeviceName != "PC" {
		t.Errorf("enrollment = %+v, %v", got, err)
	}
	cert, err := store.Certificate(ctx, "1")
	if err != nil || cert.DeviceID != "DEVICE-A" || cert.Subject != "CN=DEVICE-A" || string(cert.Raw) != string(c1.Raw) || !cert.NotBefore.Equal(t0) {
		t.Errorf("certificate = %+v, %v", cert, err)
	}
	// The same device re-enrolling with the same HWDevID is not a conflict.
	c2 := issued(t, ca, 2, "DEVICE-A")
	if err := rec.Record(ctx, enrollment(c2, "DEVICE-A", "HW-1", enroll.EnrollmentTypeFull), nil); err != nil {
		t.Fatal(err)
	}
	if len(conflicts) != 0 {
		t.Errorf("re-enrollment reported a conflict: %+v", conflicts)
	}
	if cur, _ := store.Get(ctx, "DEVICE-A"); cur.Serial != "2" {
		t.Errorf("current = %s", cur.Serial)
	}
	// Another DeviceID on the same hardware is the "Duplicate HWDevID" pitfall:
	// recorded, and reported.
	c3 := issued(t, ca, 3, "DEVICE-B")
	if err := rec.Record(ctx, enrollment(c3, "DEVICE-B", "HW-1", enroll.EnrollmentTypeDevice), nil); err != nil {
		t.Fatal(err)
	}
	if len(conflicts) != 1 || conflicts[0].HWDevID != "HW-1" || len(conflicts[0].Existing) != 2 || conflicts[0].New.DeviceID != "DEVICE-B" {
		t.Fatalf("conflicts = %+v", conflicts)
	}
	if _, err := store.Get(ctx, "DEVICE-B"); err != nil {
		t.Errorf("conflicting enrollment was not recorded: %v", err)
	}
	if a, _ := store.Get(ctx, "DEVICE-A"); a.State != storage.StateActive {
		t.Error("the earlier device lost its enrollment")
	}
	// No HWDevID: nothing to compare.
	c4 := issued(t, ca, 4, "DEVICE-C")
	if err := rec.Record(ctx, enrollment(c4, "DEVICE-C", "", enroll.EnrollmentTypeFull), nil); err != nil || len(conflicts) != 1 {
		t.Errorf("no HWDevID: %v, %d conflicts", err, len(conflicts))
	}
	// A recorder without a sink still records.
	quiet := &storage.Recorder{Store: store}
	c5 := issued(t, ca, 5, "DEVICE-D")
	if err := quiet.Record(ctx, enrollment(c5, "DEVICE-D", "HW-1", enroll.EnrollmentTypeFull), nil); err != nil {
		t.Error(err)
	}
	// Unenroll marks the state and revokes.
	if err := storage.Unenroll(ctx, store, "5", t0.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	e5, _ := store.GetBySerial(ctx, "5")
	cert5, _ := store.Certificate(ctx, "5")
	if e5.State != storage.StateUnenrolled || !cert5.Revoked {
		t.Errorf("unenroll: %s revoked=%v", e5.State, cert5.Revoked)
	}
	if err := storage.Unenroll(ctx, store, "nope", t0); !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("unenroll unknown: %v", err)
	}
}

func TestRecorderErrors(t *testing.T) {
	t.Parallel()
	ca, err := testpki.NewCA("Test CA")
	if err != nil {
		t.Fatal(err)
	}
	boom := errors.New("boom")
	ctx := context.Background()
	var nilRec *storage.Recorder
	if err := nilRec.Record(ctx, nil, nil); err == nil {
		t.Error("nil recorder")
	}
	if err := (&storage.Recorder{}).Record(ctx, nil, nil); err == nil {
		t.Error("no store")
	}
	rec := &storage.Recorder{Store: inmem.New()}
	if err := rec.Record(ctx, nil, nil); !errors.Is(err, storage.ErrInvalid) {
		t.Errorf("nil enrollment: %v", err)
	}
	if err := rec.Record(ctx, &enroll.Enrollment{Certificate: issued(t, ca, 1, "x")}, nil); !errors.Is(err, storage.ErrInvalid) {
		t.Errorf("no request: %v", err)
	}
	for _, method := range []string{"PutCertificate", "ListByHWDevID", "Create"} {
		failing := &storagetest.Failing{Store: inmem.New(), Fail: map[string]error{method: boom}}
		rec := &storage.Recorder{Store: failing}
		if err := rec.Record(ctx, enrollment(issued(t, ca, 1, "d"), "DEVICE-A", "HW-1", enroll.EnrollmentTypeFull), nil); !errors.Is(err, boom) {
			t.Errorf("%s failing: %v", method, err)
		}
	}
	// A sink error aborts.
	store := inmem.New()
	rec = &storage.Recorder{Store: store, Conflicts: storage.ConflictSinkFunc(func(context.Context, storage.Conflict) error { return boom })}
	if err := rec.Record(ctx, enrollment(issued(t, ca, 1, "a"), "DEVICE-A", "HW-1", enroll.EnrollmentTypeFull), nil); err != nil {
		t.Fatal(err)
	}
	if err := rec.Record(ctx, enrollment(issued(t, ca, 2, "b"), "DEVICE-B", "HW-1", enroll.EnrollmentTypeFull), nil); !errors.Is(err, boom) {
		t.Errorf("sink error: %v", err)
	}
	// An invalid context (no DeviceID) is refused by the store.
	if err := rec.Record(ctx, enrollment(issued(t, ca, 3, "c"), "", "", enroll.EnrollmentTypeFull), nil); !errors.Is(err, storage.ErrInvalid) {
		t.Errorf("no device id: %v", err)
	}
}

func TestFailingPassesThrough(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	ctx := context.Background()
	f := &storagetest.Failing{Store: inmem.New()}
	if err := f.Create(ctx, storagetest.Enrollment(1, "1", t0)); err != nil {
		t.Fatal(err)
	}
	if err := f.PutCertificate(ctx, storagetest.Certificate("1", "DEVICE-01", t0)); err != nil {
		t.Fatal(err)
	}
	calls := map[string]func() error{
		"Get":                     func() error { _, err := f.Get(ctx, "DEVICE-01"); return err },
		"GetBySerial":             func() error { _, err := f.GetBySerial(ctx, "1"); return err },
		"GetByThumbprint":         func() error { _, err := f.GetByThumbprint(ctx, "TP-1"); return err },
		"ListByHWDevID":           func() error { _, err := f.ListByHWDevID(ctx, "HW-01"); return err },
		"SetState":                func() error { return f.SetState(ctx, "1", storage.StateActive, t0) },
		"TouchLastSeen":           func() error { return f.TouchLastSeen(ctx, "1", t0) },
		"List":                    func() error { _, err := f.List(ctx, storage.EnrollmentQuery{}, paging.Page{}); return err },
		"Certificate":             func() error { _, err := f.Certificate(ctx, "1"); return err },
		"CertificateByThumbprint": func() error { _, err := f.CertificateByThumbprint(ctx, "TP-1"); return err },
		"Revoke":                  func() error { return f.Revoke(ctx, "1", t0) },
		"ListCertificates":        func() error { _, err := f.ListCertificates(ctx, storage.CertificateQuery{}, paging.Page{}); return err },
		"Create":                  func() error { return f.Create(ctx, storagetest.Enrollment(2, "2", t0)) },
		"PutCertificate":          func() error { return f.PutCertificate(ctx, storagetest.Certificate("2", "DEVICE-02", t0)) },
	}
	for name, call := range calls {
		f.Fail = nil
		if err := call(); err != nil {
			t.Errorf("%s passthrough: %v", name, err)
		}
		f.Fail = map[string]error{name: boom}
		if err := call(); !errors.Is(err, boom) {
			t.Errorf("%s failing: %v", name, err)
		}
	}
}

func TestStateAndValidate(t *testing.T) {
	t.Parallel()
	for _, s := range []storage.State{storage.StateActive, storage.StateSuperseded, storage.StateUnenrolled} {
		if !s.Valid() {
			t.Errorf("%s invalid", s)
		}
	}
	if storage.State("x").Valid() {
		t.Error("x valid")
	}
	if err := storagetest.Enrollment(1, "1", t0).Validate(); err != nil {
		t.Error(err)
	}
	if err := storagetest.Certificate("1", "d", t0).Validate(); err != nil {
		t.Error(err)
	}
}

func TestValidateBranches(t *testing.T) {
	t.Parallel()
	var nilE *storage.Enrollment
	if nilE.Validate() == nil {
		t.Error("nil enrollment valid")
	}
	good := storagetest.Enrollment(1, "1", t0)
	mutations := []func(e *storage.Enrollment){
		func(e *storage.Enrollment) { e.Serial = "" },
		func(e *storage.Enrollment) { e.Thumbprint = "" },
		func(e *storage.Enrollment) { e.DeviceID = "" },
		func(e *storage.Enrollment) { e.EnrollmentType = "Half" },
		func(e *storage.Enrollment) { e.EnrolledAt = time.Time{} },
	}
	for i, m := range mutations {
		e := *good
		m(&e)
		if e.Validate() == nil {
			t.Errorf("mutation %d valid", i)
		}
	}
	var nilC *storage.Certificate
	if nilC.Validate() == nil {
		t.Error("nil certificate valid")
	}
	goodC := storagetest.Certificate("1", "d", t0)
	certMutations := []func(c *storage.Certificate){
		func(c *storage.Certificate) { c.Serial = "" },
		func(c *storage.Certificate) { c.Thumbprint = "" },
		func(c *storage.Certificate) { c.Raw = nil },
		func(c *storage.Certificate) { c.NotBefore = time.Time{} },
		func(c *storage.Certificate) { c.NotAfter = c.NotBefore },
	}
	for i, m := range certMutations {
		c := *goodC
		m(&c)
		if c.Validate() == nil {
			t.Errorf("certificate mutation %d valid", i)
		}
	}
}
