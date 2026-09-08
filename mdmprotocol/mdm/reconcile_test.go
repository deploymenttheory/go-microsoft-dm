package mdm_test

import (
	"context"
	"testing"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/mdm"
	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/syncml"
	"github.com/deploymenttheory/go-microsoft-dm/paging"
	"github.com/deploymenttheory/go-microsoft-dm/storage/inmem"
)

func TestDiff(t *testing.T) {
	t.Parallel()
	desired := []mdm.Setting{
		{URI: "./Vendor/MSFT/Policy/Config/B/Two", Value: "2", Format: syncml.FormatInt},
		{URI: "./Vendor/MSFT/Policy/Config/A/One", Value: "1", Format: syncml.FormatInt},
		{URI: "./Vendor/MSFT/Policy/Config/C/Three", Value: "3"},
	}
	acknowledged := map[string]string{
		"./Vendor/MSFT/Policy/Config/A/One":   "1", // unchanged -> no command
		"./Vendor/MSFT/Policy/Config/B/Two":   "9", // changed   -> Replace
		// C/Three absent -> Replace
	}
	got := mdm.Diff(desired, acknowledged)
	if len(got) != 2 {
		t.Fatalf("want 2 commands, got %d", len(got))
	}
	// Deterministic URI order: B before C.
	if got[0].Body.(*syncml.Replace).Items[0].Target != "./Vendor/MSFT/Policy/Config/B/Two" {
		t.Errorf("first = %s", got[0].Body.(*syncml.Replace).Items[0].Target)
	}
	if got[1].Body.(*syncml.Replace).Items[0].Target != "./Vendor/MSFT/Policy/Config/C/Three" {
		t.Errorf("second = %s", got[1].Body.(*syncml.Replace).Items[0].Target)
	}
	// The changed one carries the desired value and format.
	it := got[0].Body.(*syncml.Replace).Items[0]
	if it.Data.Value != "2" || it.Meta == nil || it.Meta.Format != syncml.FormatInt {
		t.Errorf("changed item = %+v", it)
	}
	// A setting with no format omits Meta.
	if c := got[1].Body.(*syncml.Replace).Items[0]; c.Meta != nil {
		t.Errorf("unformatted setting carried Meta: %+v", c.Meta)
	}
	// Nothing desired -> nothing to do; everything matching -> nothing.
	if len(mdm.Diff(nil, acknowledged)) != 0 {
		t.Error("empty desired produced commands")
	}
	if len(mdm.Diff([]mdm.Setting{{URI: "./A", Value: "x"}}, map[string]string{"./A": "x"})) != 0 {
		t.Error("matching setting produced a command")
	}
	// A malformed URI is skipped, not panicked on.
	if len(mdm.Diff([]mdm.Setting{{URI: "/bad", Value: "x"}}, nil)) != 0 {
		t.Error("malformed URI produced a command")
	}
}

func TestAcknowledgedValues(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	q := inmem.NewQueue()
	const dev = "DEVICE-01"

	// An acknowledged Replace contributes its value.
	repl, _ := mdm.NewReplace("./Vendor/MSFT/Policy/Config/A/One", "1", mdm.WithID("r1"), mdm.WithFormat(syncml.FormatInt))
	if _, err := q.Enqueue(ctx, dev, repl, t0); err != nil {
		t.Fatal(err)
	}
	if err := q.MarkSent(ctx, dev, []mdm.Delivery{{CommandID: "r1", MsgID: "2", At: t0}}); err != nil {
		t.Fatal(err)
	}
	if err := q.StoreResult(ctx, dev, "r1", mdm.Result{Status: syncml.StatusOK, ReceivedAt: t0}); err != nil {
		t.Fatal(err)
	}
	// A pending (not yet acknowledged) Replace contributes nothing.
	pending, _ := mdm.NewReplace("./Vendor/MSFT/Policy/Config/P/Pending", "9", mdm.WithID("r2"))
	if _, err := q.Enqueue(ctx, dev, pending, t0); err != nil {
		t.Fatal(err)
	}
	// An acknowledged Get with a result contributes the read-back value.
	get, _ := mdm.NewGet([]string{"./DevDetail/SwV"}, mdm.WithID("g1"))
	if _, err := q.Enqueue(ctx, dev, get, t0); err != nil {
		t.Fatal(err)
	}
	if err := q.MarkSent(ctx, dev, []mdm.Delivery{{CommandID: "g1", MsgID: "2", At: t0}}); err != nil {
		t.Fatal(err)
	}
	if err := q.StoreResult(ctx, dev, "g1", mdm.Result{Status: syncml.StatusOK, ReceivedAt: t0,
		Items: []syncml.Item{{Source: "./DevDetail/SwV", Data: &syncml.Data{Value: "10.0.26100.1"}}}}); err != nil {
		t.Fatal(err)
	}

	got, err := mdm.AcknowledgedValues(ctx, q, dev)
	if err != nil {
		t.Fatal(err)
	}
	if got["./Vendor/MSFT/Policy/Config/A/One"] != "1" {
		t.Errorf("replace value = %q", got["./Vendor/MSFT/Policy/Config/A/One"])
	}
	if got["./DevDetail/SwV"] != "10.0.26100.1" {
		t.Errorf("get value = %q", got["./DevDetail/SwV"])
	}
	if _, ok := got["./Vendor/MSFT/Policy/Config/P/Pending"]; ok {
		t.Error("pending command contributed a value")
	}

	// Diff over the derived state re-sends only what changed.
	desired := []mdm.Setting{
		{URI: "./Vendor/MSFT/Policy/Config/A/One", Value: "1", Format: syncml.FormatInt}, // matches -> skip
		{URI: "./Vendor/MSFT/Policy/Config/A/One", Value: "1"},                            // duplicate, still matches
		{URI: "./Vendor/MSFT/Policy/Config/D/Four", Value: "4"},                           // new -> Replace
	}
	cmds := mdm.Diff(desired, got)
	if len(cmds) != 1 || cmds[0].Body.(*syncml.Replace).Items[0].Target != "./Vendor/MSFT/Policy/Config/D/Four" {
		t.Fatalf("diff produced %d commands: %+v", len(cmds), cmds)
	}
}

// pagedQueue returns two pages then stops, and can fail, to exercise the
// AcknowledgedValues paging and error branches.
type pagedQueue struct {
	mdm.CommandQueue
	pages []paging.Result[mdm.QueuedCommand]
	err   error
	calls int
}

func (q *pagedQueue) List(context.Context, string, mdm.Query, paging.Page) (paging.Result[mdm.QueuedCommand], error) {
	if q.err != nil {
		return paging.Result[mdm.QueuedCommand]{}, q.err
	}
	p := q.pages[q.calls]
	q.calls++
	return p, nil
}

func TestAcknowledgedValuesPaging(t *testing.T) {
	t.Parallel()
	repl, _ := mdm.NewReplace("./A", "1", mdm.WithID("r1"))
	repl2, _ := mdm.NewReplace("./B", "2", mdm.WithID("r2"))
	q := &pagedQueue{pages: []paging.Result[mdm.QueuedCommand]{
		{Items: []mdm.QueuedCommand{{Command: *repl}}, NextCursor: "1"},
		{Items: []mdm.QueuedCommand{{Command: *repl2}}},
	}}
	got, err := mdm.AcknowledgedValues(context.Background(), q, "d")
	if err != nil {
		t.Fatal(err)
	}
	if got["./A"] != "1" || got["./B"] != "2" || q.calls != 2 {
		t.Errorf("paged values = %+v (calls %d)", got, q.calls)
	}
	// A List error propagates.
	boom := &pagedQueue{err: errSentinel}
	if _, err := mdm.AcknowledgedValues(context.Background(), boom, "d"); err == nil {
		t.Error("list error not propagated")
	}
}

var errSentinel = errorString("boom")

type errorString string

func (e errorString) Error() string { return string(e) }
