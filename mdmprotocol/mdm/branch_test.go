package mdm

import (
	"context"
	"errors"
	"testing"

	"github.com/deploymenttheory/go-microsoft-dm/clock"
	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/syncml"
)

func TestBuilderLocURIErrors(t *testing.T) {
	t.Parallel()
	if _, err := NewReplace("/bad", "1"); !errors.Is(err, ErrCommand) {
		t.Errorf("replace: %v", err)
	}
	if _, err := NewDelete("/bad"); !errors.Is(err, ErrCommand) {
		t.Errorf("delete: %v", err)
	}
	if _, err := NewExec("/bad", "x"); !errors.Is(err, ErrCommand) {
		t.Errorf("exec: %v", err)
	}
	if _, err := NewExec("/bad", ""); !errors.Is(err, ErrCommand) {
		t.Errorf("exec no data: %v", err)
	}
	if _, err := NewGet([]string{"/bad"}); !errors.Is(err, ErrCommand) {
		t.Errorf("get: %v", err)
	}
	// A Sequence carrying a nested Atomic is refused inside an Atomic.
	repl, _ := NewReplace("./A", "1")
	atomic, _ := NewAtomic([]*Command{repl})
	seq := &Command{Scope: ScopeDevice, Body: &syncml.Sequence{Commands: []syncml.Command{atomic.Body}}}
	if _, err := NewAtomic([]*Command{seq}); !errors.Is(err, ErrCommand) {
		t.Errorf("atomic through sequence: %v", err)
	}
}

func TestValidateGroupMembers(t *testing.T) {
	t.Parallel()
	// An Atomic whose members break a rule (Add then Replace) fails Validate.
	bad := &Command{Scope: ScopeDevice, Body: &syncml.Atomic{Commands: []syncml.Command{
		&syncml.Add{Items: []syncml.Item{{Target: "./A"}}},
		&syncml.Replace{Items: []syncml.Item{{Target: "./A"}}},
	}}}
	if err := Validate(bad); !errors.Is(err, ErrCommand) {
		t.Errorf("validate group: %v", err)
	}
	// An empty group.
	if err := Validate(&Command{Scope: ScopeDevice, Body: &syncml.Atomic{}}); !errors.Is(err, ErrCommand) {
		t.Errorf("empty atomic: %v", err)
	}
	// A leaf without items.
	if err := Validate(&Command{Scope: ScopeDevice, Body: &syncml.Add{}}); !errors.Is(err, ErrCommand) {
		t.Errorf("no items: %v", err)
	}
}

func TestFileResultsChunked(t *testing.T) {
	t.Parallel()
	s := &Service{cfg: Config{MaxRequestSize: syncml.DefaultMaxSize, Clock: clock.NewFake(t0)}}
	sess := &Session{Sent: map[string]string{"1/5": "getcmd"}, Children: map[string]string{}}
	// First chunk with MoreData: buffered, no result yet.
	res1 := &syncml.Results{MsgRef: "1", CmdRef: "5", Items: []syncml.Item{
		{Source: "./A", Meta: &syncml.Meta{Size: 6}, Data: &syncml.Data{Value: "abc"}, MoreData: true}}}
	if err := s.fileResults(sess, res1); err != nil {
		t.Fatal(err)
	}
	if sess.AssemblingCommand != "getcmd" {
		t.Errorf("assembling = %q", sess.AssemblingCommand)
	}
	// Last chunk: completes and stashes.
	res2 := &syncml.Results{MsgRef: "1", CmdRef: "5", Items: []syncml.Item{
		{Source: "./A", Data: &syncml.Data{Value: "def"}}}}
	if err := s.fileResults(sess, res2); err != nil {
		t.Fatal(err)
	}
	if sess.AssemblingCommand != "" || sess.results["getcmd"] == nil {
		t.Errorf("not reassembled: %+v", sess.results)
	}
	// A Results referencing an unknown command is ignored.
	if err := s.fileResults(sess, &syncml.Results{MsgRef: "9", CmdRef: "9"}); err != nil {
		t.Errorf("unknown results: %v", err)
	}
	// A Results referencing a group child resolves to the parent.
	sess.Children["2/7"] = "parent"
	if err := s.fileResults(sess, &syncml.Results{MsgRef: "2", CmdRef: "7", Items: []syncml.Item{{Source: "./B", Data: &syncml.Data{Value: "x"}}}}); err != nil {
		t.Fatal(err)
	}
	if sess.results["parent"] == nil {
		t.Error("child results not filed to parent")
	}
}

func TestFileStatusChild(t *testing.T) {
	t.Parallel()
	s := &Service{}
	sess := &Session{Sent: map[string]string{"1/5": "cmd"}, Children: map[string]string{"1/6": "parent"}}
	// A status on a group child records a ChildResult.
	s.fileStatus(sess, &syncml.Status{MsgRef: "1", CmdRef: "6", Cmd: "Replace", Data: syncml.Data{Value: "200"}})
	if sess.results["parent"] == nil || len(sess.results["parent"].children) != 1 {
		t.Errorf("child status = %+v", sess.results["parent"])
	}
	// A status on CmdRef 0 (SyncHdr) is ignored.
	s.fileStatus(sess, &syncml.Status{CmdRef: "0"})
	// A status on an unknown command is ignored.
	s.fileStatus(sess, &syncml.Status{MsgRef: "9", CmdRef: "9"})
	// A status on a top-level command records it.
	s.fileStatus(sess, &syncml.Status{MsgRef: "1", CmdRef: "5", Data: syncml.Data{Value: "200", OriginalError: "0x1"}})
	if sess.results["cmd"] == nil || sess.results["cmd"].origErr != "0x1" {
		t.Errorf("top status = %+v", sess.results["cmd"])
	}
}

func TestFlushResultsError(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	s := &Service{cfg: Config{Clock: clock.NewFake(t0), Queue: failQueue{err: boom}}}
	code := syncml.StatusOK
	sess := &Session{results: map[string]*pendingResult{"c": {status: &code}}}
	if err := s.flushResults(context.Background(), sess); !errors.Is(err, boom) {
		t.Errorf("flush error = %v", err)
	}
	// A result without a status is left pending.
	sess = &Session{results: map[string]*pendingResult{"c": {}}}
	if err := s.flushResults(context.Background(), sess); err != nil {
		t.Fatal(err)
	}
	if sess.results["c"] == nil {
		t.Error("status-less result was flushed")
	}
}

// failQueue fails StoreResult.
type failQueue struct {
	CommandQueue
	err error
}

func (f failQueue) StoreResult(context.Context, string, string, Result) error { return f.err }

func TestSmallBranches(t *testing.T) {
	t.Parallel()
	// commandSize returns 0 for a command that cannot encode (Data with both
	// Value and XML).
	bad := &syncml.Replace{Items: []syncml.Item{{Target: "./A", Data: &syncml.Data{Value: "v", XML: "<x/>"}}}}
	if commandSize(bad) != 0 {
		t.Error("commandSize of an unencodable command")
	}
	// A Sequence with a nil first member.
	if _, err := NewSequence([]*Command{nil}); !errors.Is(err, ErrCommand) {
		t.Errorf("nil sequence member: %v", err)
	}
	// isConsumedEvent with no Meta.
	if isConsumedEvent(syncml.Item{}) {
		t.Error("item without meta consumed")
	}
	// isUnenrollAlert with no Meta.
	if isUnenrollAlert(syncml.Item{}) {
		t.Error("item without meta is an unenroll")
	}
}

type failReadBody struct{}

func (failReadBody) Read([]byte) (int, error) { return 0, errors.New("read fail") }
func (failReadBody) Close() error             { return nil }

func TestServeHTTPUnreadable(t *testing.T) {
	t.Parallel()
	svc := &Service{cfg: Config{MaxRequestSize: 1024}}
	h := NewHandler(svc)
	rec := newRecorder()
	req := newPost(failReadBody{})
	h.ServeHTTP(rec, req)
	if rec.code != 400 {
		t.Errorf("unreadable body = %d", rec.code)
	}
}
