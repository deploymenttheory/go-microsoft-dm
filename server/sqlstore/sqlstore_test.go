package sqlstore_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/mdm"
	"github.com/deploymenttheory/go-microsoft-dm/paging"
	"github.com/deploymenttheory/go-microsoft-dm/server/sqlstore"
	"github.com/deploymenttheory/go-microsoft-dm/storage"
	"github.com/deploymenttheory/go-microsoft-dm/storage/storagetest"
)

// newSQLite returns a fresh SQLite store backed by a temp file (one file per
// call, so suites do not share state).
func newSQLite(t *testing.T) *sqlstore.Store {
	t.Helper()
	dsn := "file:" + filepath.Join(t.TempDir(), "test.db") + "?_pragma=busy_timeout(5000)"
	s, err := sqlstore.Open(context.Background(), sqlstore.SQLite, dsn, sqlstore.Options{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestSQLiteStoreContract(t *testing.T) {
	t.Parallel()
	storagetest.RunAll(t, func(t *testing.T) storage.Store { return newSQLite(t) })
}

func TestSQLiteQueueContract(t *testing.T) {
	t.Parallel()
	storagetest.RunQueueSuite(t, func(t *testing.T) mdm.CommandQueue { return newSQLite(t).Queue() })
}

func TestSQLiteCredentials(t *testing.T) {
	t.Parallel()
	storagetest.RunCredentialSuite(t, func(t *testing.T) storage.CredentialStore { return newSQLite(t) })
}

func TestSQLiteFactsAndEvents(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newSQLite(t)
	// Facts not found.
	if _, err := s.Facts(ctx, "d"); err == nil {
		t.Error("missing facts not reported")
	}
	if err := s.PutFacts(ctx, sqlstore.Facts{DeviceID: "d", DevInfo: map[string]string{"Man": "VMware"}, LoginStatus: "user", ChannelURI: "https://wns/x", UpdatedAt: t0}); err != nil {
		t.Fatal(err)
	}
	// Merge: a second write updates the row.
	if err := s.PutFacts(ctx, sqlstore.Facts{DeviceID: "d", DevInfo: map[string]string{"Man": "VMware", "Mod": "7,1"}, LoginStatus: "none", UpdatedAt: t0}); err != nil {
		t.Fatal(err)
	}
	f, err := s.Facts(ctx, "d")
	if err != nil || f.LoginStatus != "none" || f.DevInfo["Mod"] != "7,1" {
		t.Errorf("facts = %+v, %v", f, err)
	}
	if err := s.PutFacts(ctx, sqlstore.Facts{}); err == nil {
		t.Error("empty device id accepted")
	}
	// Events.
	if err := s.LogEvent(ctx, "d", "enrolled", map[string]string{"upn": "u@c"}); err != nil {
		t.Fatal(err)
	}
	if err := s.LogEvent(ctx, "d", "hwdevid_conflict", nil); err != nil {
		t.Fatal(err)
	}
	evs, err := s.Events(ctx, "d", 0)
	if err != nil || len(evs) != 2 || evs[0].Kind != "enrolled" || evs[1].Kind != "hwdevid_conflict" {
		t.Errorf("events = %+v, %v", evs, err)
	}
	if evs, _ := s.Events(ctx, "d", 1); len(evs) != 1 {
		t.Errorf("limited events = %d", len(evs))
	}
}

func TestOpenErrors(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	if _, err := sqlstore.Open(ctx, "oracle", "x", sqlstore.Options{}); err == nil {
		t.Error("unknown dialect accepted")
	}
	if _, err := sqlstore.Open(ctx, sqlstore.SQLite, "", sqlstore.Options{}); err == nil {
		t.Error("empty dsn accepted")
	}
	if _, err := sqlstore.Open(ctx, sqlstore.SQLite, "file:/nonexistent-dir/x/y/z.db?mode=rw", sqlstore.Options{}); err == nil {
		t.Error("unopenable dsn accepted")
	}
}

var t0 = timeValue()

func TestStoreEdges(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newSQLite(t)
	if s.DB() == nil {
		t.Error("DB() nil")
	}
	// A bad paging cursor is rejected by List.
	if _, err := s.List(ctx, storage.EnrollmentQuery{}, paging.Page{Cursor: "x"}); err == nil {
		t.Error("bad cursor accepted (enrollments)")
	}
	if _, err := s.ListCertificates(ctx, storage.CertificateQuery{}, paging.Page{Cursor: "-1"}); err == nil {
		t.Error("bad cursor accepted (certificates)")
	}
	if _, err := s.Queue().List(ctx, "d", mdm.Query{}, paging.Page{Cursor: "x"}); err == nil {
		t.Error("bad cursor accepted (queue)")
	}
	// Prune with nothing to prune returns 0.
	if n, err := s.Queue().Prune(ctx, t0, 10); err != nil || n != 0 {
		t.Errorf("prune empty = %d, %v", n, err)
	}
}

func TestClosedStoreErrors(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newSQLite(t)
	// Seed one row so lookups have something before we break the DB, then
	// close so every subsequent call hits the driver error path.
	yes := true
	_ = s.Close()

	enr := storagetest.Enrollment(1, "1", t0)
	cert := storagetest.Certificate("1", "DEVICE-01", t0)
	q := s.Queue()
	cmd, _ := mdm.NewReplace("./A", "1", mdm.WithID("c1"))

	checks := []func() error{
		func() error { return s.Create(ctx, enr) },
		func() error { _, e := s.Get(ctx, "d"); return e },
		func() error { _, e := s.GetBySerial(ctx, "1"); return e },
		func() error { _, e := s.GetByThumbprint(ctx, "TP-1"); return e },
		func() error { _, e := s.ListByHWDevID(ctx, "HW"); return e },
		func() error { return s.SetState(ctx, "1", storage.StateUnenrolled, t0) },
		func() error { return s.TouchLastSeen(ctx, "1", t0) },
		func() error { _, e := s.List(ctx, storage.EnrollmentQuery{}, paging.Page{}); return e },
		func() error { return s.PutCertificate(ctx, cert) },
		func() error { _, e := s.Certificate(ctx, "1"); return e },
		func() error { _, e := s.CertificateByThumbprint(ctx, "TP-1"); return e },
		func() error { return s.Revoke(ctx, "1", t0) },
		func() error {
			_, e := s.ListCertificates(ctx, storage.CertificateQuery{Revoked: &yes}, paging.Page{})
			return e
		},
		func() error {
			return s.PutMDMCredential(ctx, storage.MDMCredential{DeviceID: "d", CredentialHash: []byte{1}})
		},
		func() error { _, e := s.MDMCredential(ctx, "d"); return e },
		func() error { return s.PutFacts(ctx, sqlstore.Facts{DeviceID: "d"}) },
		func() error { _, e := s.Facts(ctx, "d"); return e },
		func() error { return s.LogEvent(ctx, "d", "k", nil) },
		func() error { _, e := s.Events(ctx, "d", 0); return e },
		func() error { _, e := q.Enqueue(ctx, "d", cmd, t0); return e },
		func() error { _, e := q.Deliverable(ctx, "d", 10); return e },
		func() error { return q.MarkSent(ctx, "d", []mdm.Delivery{{CommandID: "c1", MsgID: "2", At: t0}}) },
		func() error { return q.StoreResult(ctx, "d", "c1", mdm.Result{}) },
		func() error { _, e := q.Get(ctx, "d", "c1"); return e },
		func() error {
			_, e := q.List(ctx, "d", mdm.Query{States: []mdm.State{mdm.StatePending}}, paging.Page{})
			return e
		},
		func() error { _, e := q.Cancel(ctx, "d", []string{"c1"}, t0); return e },
		func() error { _, e := q.Prune(ctx, t0, 10); return e },
	}
	for i, fn := range checks {
		if err := fn(); err == nil {
			t.Errorf("check %d returned nil on a closed DB", i)
		}
	}
}

func TestCorruptRows(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newSQLite(t)
	// A command row with an unparseable body and result exercises the decode
	// error branches in scanCommand.
	_, err := s.DB().Exec(`INSERT INTO commands (device_id, id, seq, scope, internal, state, body, enqueued_at, last_sent_at, sent_msg_id, attempts, completed_at, result)
		VALUES ('d', 'bad', 1, 'device', 0, 'acknowledged', 'not-xml', 1, 0, '', 0, 0, 'not-json')`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Queue().Get(ctx, "d", "bad"); err == nil {
		t.Error("corrupt command body accepted")
	}
	// A good body but corrupt result JSON.
	_, err = s.DB().Exec(`INSERT INTO commands (device_id, id, seq, scope, internal, state, body, enqueued_at, last_sent_at, sent_msg_id, attempts, completed_at, result)
		VALUES ('d', 'badresult', 2, 'device', 0, 'acknowledged', '<Get><CmdID>1</CmdID><Item><Target><LocURI>./A</LocURI></Target></Item></Get>', 1, 0, '', 0, 0, 'not-json')`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Queue().Get(ctx, "d", "badresult"); err == nil {
		t.Error("corrupt result accepted")
	}
	// A corrupt enrollment context.
	_, err = s.DB().Exec(`INSERT INTO enrollments (serial, thumbprint, device_id, hwdevid, enrollment_type, upn, context, state, enrolled_at, updated_at, last_seen_at)
		VALUES ('9', 'TP-9', 'dc', '', 'Full', '', 'not-json', 'active', 1, 1, 0)`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(ctx, "dc"); err == nil {
		t.Error("corrupt enrollment context accepted")
	}
	// A corrupt facts devinfo.
	_, err = s.DB().Exec(`INSERT INTO device_facts (device_id, devinfo, login_status, sync_type, device_prep_sync, channel_uri, updated_at)
		VALUES ('df', 'not-json', '', '', '', '', 1)`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Facts(ctx, "df"); err == nil {
		t.Error("corrupt facts accepted")
	}
	// A corrupt body in a deliverable (pending) state, so Deliverable's filter
	// does not skip it.
	if _, err := s.DB().Exec(`INSERT INTO commands (device_id, id, seq, scope, internal, state, body, enqueued_at, last_sent_at, sent_msg_id, attempts, completed_at, result)
		VALUES ('d', 'badpending', 3, 'device', 0, 'pending', 'not-xml', 1, 0, '', 0, 0, '')`); err != nil {
		t.Fatal(err)
	}
	// The list-path methods hit the same decode branches through their
	// row-iteration loop, a distinct code path from the single-row Get.
	if _, err := s.Queue().Deliverable(ctx, "d", 10); err == nil {
		t.Error("Deliverable over a corrupt body accepted")
	}
	if _, err := s.Queue().List(ctx, "d", mdm.Query{}, paging.Page{}); err == nil {
		t.Error("queue List over a corrupt body accepted")
	}
	if _, err := s.ListByHWDevID(ctx, ""); err == nil {
		t.Error("ListByHWDevID over a corrupt context accepted")
	}
	if _, err := s.List(ctx, storage.EnrollmentQuery{}, paging.Page{}); err == nil {
		t.Error("enrollment List over a corrupt context accepted")
	}
	// A scope filter exercises the scope clause of the queue List builder.
	if _, err := s.Queue().List(ctx, "clean", mdm.Query{Scope: mdm.ScopeDevice}, paging.Page{}); err != nil {
		t.Errorf("scoped list on a clean device: %v", err)
	}
	// An event row whose at column holds text fails the int64 Scan mid-iteration.
	if _, err := s.DB().Exec(`INSERT INTO events (device_id, kind, detail, at) VALUES ('de', 'k', 'd', 'not-a-number')`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Events(ctx, "de", 0); err == nil {
		t.Error("event with NULL detail accepted")
	}
}

func TestOptionsMaxConns(t *testing.T) {
	t.Parallel()
	// MaxOpenConns is honoured for non-sqlite; for sqlite it is forced to 1.
	s := newSQLite(t)
	if s.DB().Stats().MaxOpenConnections != 1 {
		t.Errorf("sqlite max conns = %d", s.DB().Stats().MaxOpenConnections)
	}
}
