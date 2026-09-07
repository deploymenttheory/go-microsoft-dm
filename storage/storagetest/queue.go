package storagetest

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/mdm"
	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/syncml"
	"github.com/deploymenttheory/go-microsoft-dm/paging"
)

// QueueFactory returns a fresh, empty command queue for one test.
type QueueFactory func(t *testing.T) mdm.CommandQueue

// RunQueueSuite exercises mdm.CommandQueue.
func RunQueueSuite(t *testing.T, newQueue QueueFactory) {
	t.Helper()
	ctx := context.Background()
	const dev = "DEVICE-01"

	replace := func(id, uri, val string) *mdm.Command {
		c, err := mdm.NewReplace(uri, val, mdm.WithID(id), mdm.WithFormat(syncml.FormatChr))
		if err != nil {
			t.Fatal(err)
		}
		return c
	}

	t.Run("EnqueueAssignsSeqAndID", func(t *testing.T) {
		q := newQueue(t)
		a, err := q.Enqueue(ctx, dev, replace("a", "./Vendor/MSFT/Policy/Config/A", "1"), t0)
		if err != nil || a.Seq != 1 || a.ID != "a" || a.State != mdm.StatePending {
			t.Fatalf("first = %+v, %v", a, err)
		}
		auto, err := q.Enqueue(ctx, dev, replace("", "./Vendor/MSFT/Policy/Config/B", "2"), t0)
		if err != nil || auto.Seq != 2 || auto.ID == "" {
			t.Fatalf("auto id = %+v, %v", auto, err)
		}
		if _, err := q.Enqueue(ctx, dev, replace("a", "./Vendor/MSFT/Policy/Config/C", "3"), t0); !errors.Is(err, mdm.ErrConflict) {
			t.Errorf("duplicate id = %v", err)
		}
	})

	t.Run("DeliverAndAcknowledge", func(t *testing.T) {
		q := newQueue(t)
		for i := 1; i <= 3; i++ {
			if _, err := q.Enqueue(ctx, dev, replace(fmt.Sprintf("c%d", i), fmt.Sprintf("./Vendor/MSFT/Policy/Config/N%d", i), "1"), t0); err != nil {
				t.Fatal(err)
			}
		}
		got, err := q.Deliverable(ctx, dev, 10)
		if err != nil || len(got) != 3 || got[0].ID != "c1" || got[2].ID != "c3" {
			t.Fatalf("deliverable = %+v, %v", got, err)
		}
		if err := q.MarkSent(ctx, dev, []mdm.Delivery{{CommandID: "c1", MsgID: "2", At: t0}}); err != nil {
			t.Fatal(err)
		}
		sent, _ := q.Get(ctx, dev, "c1")
		if sent.State != mdm.StateSent || sent.SentMsgID != "2" || sent.Attempts != 1 {
			t.Errorf("sent = %+v", sent)
		}
		// A sent command is still deliverable until it is acknowledged.
		got, _ = q.Deliverable(ctx, dev, 10)
		if len(got) != 3 {
			t.Errorf("sent still deliverable: %d", len(got))
		}
		if err := q.StoreResult(ctx, dev, "c1", mdm.Result{Status: syncml.StatusOK, ReceivedAt: t0}); err != nil {
			t.Fatal(err)
		}
		done, _ := q.Get(ctx, dev, "c1")
		if done.State != mdm.StateAcknowledged || done.Result == nil {
			t.Errorf("acked = %+v", done)
		}
		got, _ = q.Deliverable(ctx, dev, 10)
		if len(got) != 2 {
			t.Errorf("acked still deliverable: %d", len(got))
		}
		if err := q.StoreResult(ctx, dev, "c2", mdm.Result{Status: syncml.StatusCommandFailed, OriginalError: "0x80000001", ReceivedAt: t0}); err != nil {
			t.Fatal(err)
		}
		failed, _ := q.Get(ctx, dev, "c2")
		if failed.State != mdm.StateFailed || failed.Result.OriginalError != "0x80000001" {
			t.Errorf("failed = %+v", failed)
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		q := newQueue(t)
		if _, err := q.Get(ctx, dev, "nope"); !errors.Is(err, mdm.ErrNotFound) {
			t.Errorf("Get: %v", err)
		}
		if err := q.MarkSent(ctx, dev, []mdm.Delivery{{CommandID: "nope", MsgID: "2"}}); !errors.Is(err, mdm.ErrNotFound) {
			t.Errorf("MarkSent: %v", err)
		}
		if err := q.StoreResult(ctx, dev, "nope", mdm.Result{Status: syncml.StatusOK}); !errors.Is(err, mdm.ErrNotFound) {
			t.Errorf("StoreResult: %v", err)
		}
		if _, err := q.Enqueue(ctx, "", replace("a", "./A", "1"), t0); err == nil {
			t.Error("empty device accepted")
		}
		if _, err := q.Enqueue(ctx, dev, nil, t0); err == nil {
			t.Error("nil command accepted")
		}
	})

	t.Run("ListFilterAndPage", func(t *testing.T) {
		q := newQueue(t)
		for i := 1; i <= 4; i++ {
			c := replace(fmt.Sprintf("c%d", i), fmt.Sprintf("./Vendor/MSFT/Policy/Config/N%d", i), "1")
			if i == 4 {
				var err error
				c, err = mdm.NewGet([]string{"./DevDetail/SwV"}, mdm.WithID("c4"))
				if err != nil {
					t.Fatal(err)
				}
				c.Internal = true
			}
			if _, err := q.Enqueue(ctx, dev, c, t0); err != nil {
				t.Fatal(err)
			}
		}
		if err := q.MarkSent(ctx, dev, []mdm.Delivery{{CommandID: "c1", MsgID: "2", At: t0}}); err != nil {
			t.Fatal(err)
		}
		yes := true
		internal, _ := q.List(ctx, dev, mdm.Query{Internal: &yes}, paging.Page{})
		if len(internal.Items) != 1 || internal.Items[0].ID != "c4" {
			t.Errorf("internal = %+v", internal.Items)
		}
		sent, _ := q.List(ctx, dev, mdm.Query{States: []mdm.State{mdm.StateSent}}, paging.Page{})
		if len(sent.Items) != 1 || sent.Items[0].ID != "c1" {
			t.Errorf("sent filter = %+v", sent.Items)
		}
		p1, err := q.List(ctx, dev, mdm.Query{}, paging.Page{Limit: 2})
		if err != nil || len(p1.Items) != 2 || p1.NextCursor == "" || p1.Items[0].Seq != 1 {
			t.Fatalf("page 1 = %+v, %v", p1, err)
		}
		p2, _ := q.List(ctx, dev, mdm.Query{}, paging.Page{Limit: 2, Cursor: p1.NextCursor})
		if len(p2.Items) != 2 || p2.Items[0].ID != "c3" || p2.NextCursor != "" {
			t.Errorf("page 2 = %+v", p2)
		}
	})

	t.Run("CancelAndPrune", func(t *testing.T) {
		q := newQueue(t)
		for i := 1; i <= 3; i++ {
			if _, err := q.Enqueue(ctx, dev, replace(fmt.Sprintf("c%d", i), fmt.Sprintf("./Vendor/MSFT/Policy/Config/N%d", i), "1"), t0); err != nil {
				t.Fatal(err)
			}
		}
		if err := q.StoreResult(ctx, dev, "c1", mdm.Result{Status: syncml.StatusOK, ReceivedAt: t0}); err != nil {
			t.Fatal(err)
		}
		n, err := q.Cancel(ctx, dev, []string{"c1", "c2", "nope"}, t0.Add(time.Minute))
		if err != nil || n != 1 {
			t.Fatalf("cancel = %d, %v", n, err)
		}
		c2, _ := q.Get(ctx, dev, "c2")
		if c2.State != mdm.StateCancelled {
			t.Errorf("c2 = %+v", c2)
		}
		pruned, err := q.Prune(ctx, t0.Add(time.Hour), 10)
		if err != nil || pruned != 2 {
			t.Fatalf("prune = %d, %v", pruned, err)
		}
		if _, err := q.Get(ctx, dev, "c1"); !errors.Is(err, mdm.ErrNotFound) {
			t.Errorf("c1 survived prune: %v", err)
		}
		if _, err := q.Get(ctx, dev, "c3"); err != nil {
			t.Errorf("pending c3 pruned: %v", err)
		}
	})

	t.Run("Concurrency", func(t *testing.T) {
		q := newQueue(t)
		var wg sync.WaitGroup
		errs := make(chan error, 32)
		for i := 0; i < 32; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				if _, err := q.Enqueue(ctx, dev, replace(fmt.Sprintf("k%d", i), fmt.Sprintf("./Vendor/MSFT/Policy/Config/K%d", i), "1"), t0); err != nil {
					errs <- err
				}
			}(i)
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			t.Error(err)
		}
		all, _ := q.List(ctx, dev, mdm.Query{}, paging.Page{Limit: 1000})
		if len(all.Items) != 32 {
			t.Errorf("enqueued %d", len(all.Items))
		}
		seqs := map[int64]bool{}
		for _, qc := range all.Items {
			if seqs[qc.Seq] {
				t.Errorf("duplicate seq %d", qc.Seq)
			}
			seqs[qc.Seq] = true
		}
	})
}
