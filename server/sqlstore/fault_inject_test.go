package sqlstore

import (
	"context"
	"testing"
	"time"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/mdm"
	"github.com/deploymenttheory/go-microsoft-dm/paging"
	"github.com/deploymenttheory/go-microsoft-dm/storage"
)

var ft0 = time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)

// faultStore opens a store on the fault driver over a shared in-memory db.
func faultStore(t *testing.T) *Store {
	t.Helper()
	registerFaultDriver()
	// Open with a plain sqlite store to create the schema, then reopen the
	// same in-memory database through the fault driver.
	dsn := "file:faulttest" + t.Name() + "?mode=memory&cache=shared"
	seed, err := Open(context.Background(), SQLite, dsn, Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = seed.Close() })
	// A second handle on the same shared cache, through the fault driver.
	d, _ := dialectFor(SQLite)
	d.driver = "sqlite-fault"
	fdb, err := openRaw(d, dsn)
	if err != nil {
		t.Fatal(err)
	}
	s := &Store{conn: &conn{db: fdb, d: d, clock: clockZero{}}}
	t.Cleanup(func() { _ = s.Close(); disarmFault() })
	return s
}

// injectAt runs fn with the countdown-th statement failing, asserting an error.
func injectAt(t *testing.T, s *Store, countdown int64, name string, fn func() error) {
	t.Helper()
	armFault(countdown)
	err := fn()
	disarmFault()
	if err == nil {
		t.Errorf("%s: no error with a fault at statement %d", name, countdown)
	}
}

// TestFaultInjection walks each store method, failing statements at
// successive positions to reach the in-transaction and iteration error arms.
func TestFaultInjection(t *testing.T) {
	ctx := context.Background()
	s := faultStore(t)
	enr := storage.Enrollment{Serial: "1", Thumbprint: "TP-1", DeviceID: "d", HWDevID: "HW", EnrollmentType: "Full", EnrolledAt: ft0}
	cert := storage.Certificate{Serial: "1", Thumbprint: "TP-1", Subject: "CN=d", DeviceID: "d", NotBefore: ft0, NotAfter: ft0.Add(time.Hour), Raw: []byte{1}}
	yes := true

	// Create: fail at the begin, the first select, the supersede update, the insert.
	for i := int64(1); i <= 4; i++ {
		injectAt(t, s, i, "Create", func() error { return s.Create(ctx, &enr) })
	}
	// Get / GetBySerial / GetByThumbprint.
	injectAt(t, s, 1, "Get", func() error { _, e := s.Get(ctx, "d"); return e })
	injectAt(t, s, 1, "GetBySerial", func() error { _, e := s.GetBySerial(ctx, "1"); return e })
	injectAt(t, s, 1, "GetByThumbprint", func() error { _, e := s.GetByThumbprint(ctx, "TP-1"); return e })
	injectAt(t, s, 1, "ListByHWDevID", func() error { _, e := s.ListByHWDevID(ctx, "HW"); return e })
	// SetState: begin, select device, update.
	for i := int64(1); i <= 3; i++ {
		injectAt(t, s, i, "SetState", func() error { return s.SetState(ctx, "1", storage.StateActive, ft0) })
	}
	injectAt(t, s, 1, "TouchLastSeen", func() error { return s.TouchLastSeen(ctx, "1", ft0) })
	injectAt(t, s, 1, "List", func() error { _, e := s.List(ctx, storage.EnrollmentQuery{}, paging.Page{}); return e })

	// Certificates.
	for i := int64(1); i <= 3; i++ {
		injectAt(t, s, i, "PutCertificate", func() error { return s.PutCertificate(ctx, &cert) })
	}
	injectAt(t, s, 1, "Certificate", func() error { _, e := s.Certificate(ctx, "1"); return e })
	injectAt(t, s, 1, "CertificateByThumbprint", func() error { _, e := s.CertificateByThumbprint(ctx, "TP-1"); return e })
	injectAt(t, s, 1, "Revoke", func() error { return s.Revoke(ctx, "1", ft0) })
	injectAt(t, s, 1, "ListCertificates", func() error { _, e := s.ListCertificates(ctx, storage.CertificateQuery{Revoked: &yes}, paging.Page{}); return e })

	// Credentials, facts, events.
	injectAt(t, s, 1, "PutMDMCredential", func() error { return s.PutMDMCredential(ctx, storage.MDMCredential{DeviceID: "d", CredentialHash: []byte{1}}) })
	injectAt(t, s, 1, "MDMCredential", func() error { _, e := s.MDMCredential(ctx, "d"); return e })
	injectAt(t, s, 1, "PutFacts", func() error { return s.PutFacts(ctx, Facts{DeviceID: "d"}) })
	injectAt(t, s, 1, "Facts", func() error { _, e := s.Facts(ctx, "d"); return e })
	injectAt(t, s, 1, "LogEvent", func() error { return s.LogEvent(ctx, "d", "k", nil) })
	injectAt(t, s, 1, "Events", func() error { _, e := s.Events(ctx, "d", 0); return e })

	// Queue.
	q := s.Queue()
	cmd, _ := mdm.NewReplace("./A", "1", mdm.WithID("c1"))
	for i := int64(1); i <= 3; i++ {
		injectAt(t, s, i, "Enqueue", func() error { _, e := q.Enqueue(ctx, "d", cmd, ft0); return e })
	}
	injectAt(t, s, 1, "Deliverable", func() error { _, e := q.Deliverable(ctx, "d", 10); return e })
	injectAt(t, s, 1, "MarkSent", func() error { return q.MarkSent(ctx, "d", []mdm.Delivery{{CommandID: "c1", MsgID: "2", At: ft0}}) })
	injectAt(t, s, 1, "StoreResult", func() error { return q.StoreResult(ctx, "d", "c1", mdm.Result{}) })
	injectAt(t, s, 1, "QueueGet", func() error { _, e := q.Get(ctx, "d", "c1"); return e })
	injectAt(t, s, 1, "QueueList", func() error { _, e := q.List(ctx, "d", mdm.Query{States: []mdm.State{mdm.StatePending}}, paging.Page{}); return e })
	injectAt(t, s, 1, "Cancel", func() error { _, e := q.Cancel(ctx, "d", []string{"c1"}, ft0); return e })
	injectAt(t, s, 1, "Prune", func() error { _, e := q.Prune(ctx, ft0.Add(time.Hour), 10); return e })
}

// TestFaultInjectionPopulated seeds rows so the in-transaction supersede and
// delete branches are reached before a fault is injected.
func TestFaultInjectionPopulated(t *testing.T) {
	ctx := context.Background()
	s := faultStore(t)
	// Seed with faults disarmed.
	first := storage.Enrollment{Serial: "1", Thumbprint: "TP-1", DeviceID: "d", EnrollmentType: "Full", EnrolledAt: ft0}
	if err := s.Create(ctx, &first); err != nil {
		t.Fatal(err)
	}
	// Create a second enrollment: begin, select-serial, select-thumbprint,
	// supersede UPDATE (statement 4), INSERT. Fail the supersede.
	second := storage.Enrollment{Serial: "2", Thumbprint: "TP-2", DeviceID: "d", EnrollmentType: "Full", EnrolledAt: ft0}
	injectAt(t, s, 4, "Create supersede", func() error { return s.Create(ctx, &second) })

	// Now actually create the second so two enrollments exist.
	if err := s.Create(ctx, &second); err != nil {
		t.Fatal(err)
	}
	// SetState the first back to active: begin, select-device, supersede
	// UPDATE (statement 3 fails).
	injectAt(t, s, 3, "SetState supersede", func() error { return s.SetState(ctx, "1", storage.StateActive, ft0) })

	// Seed a terminal command and prune it: select (1), then the delete tx.
	q := s.Queue()
	cmd, _ := mdm.NewReplace("./A", "1", mdm.WithID("done"))
	if _, err := q.Enqueue(ctx, "d", cmd, ft0); err != nil {
		t.Fatal(err)
	}
	if err := q.StoreResult(ctx, "d", "done", mdm.Result{Status: 200, ReceivedAt: ft0}); err != nil {
		t.Fatal(err)
	}
	// Prune: select (statement 1), begin, delete (statement 2 fails).
	injectAt(t, s, 2, "Prune delete", func() error { _, e := q.Prune(ctx, ft0.Add(time.Hour), 10); return e })
}

func TestMigrateAndOpenErrors(t *testing.T) {
	ctx := context.Background()
	// A fault during migrate surfaces.
	s := faultStore(t)
	armFault(1)
	err := s.migrate(ctx)
	disarmFault()
	if err == nil {
		t.Error("migrate with a fault succeeded")
	}
	// Open a non-sqlite dialect with MaxOpenConns set: the pool option runs,
	// then the ping fails against an unreachable database.
	if _, err := Open(ctx, MySQL, "user:pass@tcp(127.0.0.1:0)/db?timeout=200ms", Options{MaxOpenConns: 5}); err == nil {
		t.Error("unreachable mysql accepted")
	}
	// LogEvent with an unmarshalable detail fails at json.Marshal.
	live := newStoreT(t)
	if err := live.LogEvent(ctx, "d", "k", make(chan int)); err == nil {
		t.Error("unmarshalable event detail accepted")
	}
}

// newStoreT opens a healthy sqlite store for a single test.
func newStoreT(t *testing.T) *Store {
	t.Helper()
	s, err := Open(context.Background(), SQLite, "file:live"+t.Name()+"?mode=memory&cache=shared", Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}
