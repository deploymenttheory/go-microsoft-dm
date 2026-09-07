package inmem

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/mdm"
	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/syncml"
	"github.com/deploymenttheory/go-microsoft-dm/paging"
)

// Queue implements mdm.CommandQueue in memory.
type Queue struct {
	mu     sync.Mutex
	byDev  map[string][]*mdm.QueuedCommand // device id -> commands in Seq order
	nextID int64
	seq    map[string]int64 // device id -> last Seq
}

// NewQueue returns an empty queue.
func NewQueue() *Queue {
	return &Queue{byDev: map[string][]*mdm.QueuedCommand{}, seq: map[string]int64{}}
}

var _ mdm.CommandQueue = (*Queue)(nil)

// Enqueue implements mdm.CommandQueue.
func (q *Queue) Enqueue(_ context.Context, deviceID string, cmd *mdm.Command, at time.Time) (*mdm.QueuedCommand, error) {
	if deviceID == "" {
		return nil, fmt.Errorf("%w: empty device id", mdm.ErrNotFound)
	}
	if err := mdm.Validate(cmd); err != nil {
		return nil, err
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	id := cmd.ID
	if id == "" {
		q.nextID++
		id = "cmd-" + strconv.FormatInt(q.nextID, 10)
	}
	for _, existing := range q.byDev[deviceID] {
		if existing.ID == id {
			return nil, fmt.Errorf("%w: command %q exists for device %q", mdm.ErrConflict, id, deviceID)
		}
	}
	q.seq[deviceID]++
	stored := *cmd
	stored.ID = id
	qc := &mdm.QueuedCommand{Command: stored, Seq: q.seq[deviceID], State: mdm.StatePending, EnqueuedAt: at.UTC()}
	q.byDev[deviceID] = append(q.byDev[deviceID], qc)
	out := *qc
	return &out, nil
}

// Deliverable implements mdm.CommandQueue.
func (q *Queue) Deliverable(_ context.Context, deviceID string, limit int) ([]mdm.QueuedCommand, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	var out []mdm.QueuedCommand
	for _, qc := range q.byDev[deviceID] {
		if qc.State == mdm.StatePending || qc.State == mdm.StateSent {
			out = append(out, *qc)
			if limit > 0 && len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

// MarkSent implements mdm.CommandQueue.
func (q *Queue) MarkSent(_ context.Context, deviceID string, deliveries []mdm.Delivery) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	index := map[string]*mdm.QueuedCommand{}
	for _, qc := range q.byDev[deviceID] {
		index[qc.ID] = qc
	}
	for _, d := range deliveries {
		qc, ok := index[d.CommandID]
		if !ok {
			return fmt.Errorf("%w: command %q", mdm.ErrNotFound, d.CommandID)
		}
		qc.State = mdm.StateSent
		qc.LastSentAt = d.At.UTC()
		qc.SentMsgID = d.MsgID
		qc.Attempts++
	}
	return nil
}

// StoreResult implements mdm.CommandQueue.
func (q *Queue) StoreResult(_ context.Context, deviceID, commandID string, r mdm.Result) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, qc := range q.byDev[deviceID] {
		if qc.ID != commandID {
			continue
		}
		res := r
		qc.Result = &res
		qc.CompletedAt = r.ReceivedAt.UTC()
		if syncml.StatusCode(r.Status).Success() {
			qc.State = mdm.StateAcknowledged
		} else {
			qc.State = mdm.StateFailed
		}
		return nil
	}
	return fmt.Errorf("%w: command %q for device %q", mdm.ErrNotFound, commandID, deviceID)
}

// Get implements mdm.CommandQueue.
func (q *Queue) Get(_ context.Context, deviceID, commandID string) (*mdm.QueuedCommand, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, qc := range q.byDev[deviceID] {
		if qc.ID == commandID {
			out := *qc
			return &out, nil
		}
	}
	return nil, fmt.Errorf("%w: command %q", mdm.ErrNotFound, commandID)
}

// List implements mdm.CommandQueue.
func (q *Queue) List(_ context.Context, deviceID string, query mdm.Query, p paging.Page) (paging.Result[mdm.QueuedCommand], error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	var all []mdm.QueuedCommand
	for _, qc := range q.byDev[deviceID] {
		if !matches(qc, query) {
			continue
		}
		all = append(all, *qc)
	}
	return page(all, p)
}

func matches(qc *mdm.QueuedCommand, query mdm.Query) bool {
	if query.Internal != nil && qc.Internal != *query.Internal {
		return false
	}
	if query.Scope != "" && qc.Scope != query.Scope {
		return false
	}
	if len(query.States) > 0 {
		found := false
		for _, st := range query.States {
			if qc.State == st {
				found = true
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// Cancel implements mdm.CommandQueue.
func (q *Queue) Cancel(_ context.Context, deviceID string, ids []string, at time.Time) (int, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	want := map[string]bool{}
	for _, id := range ids {
		want[id] = true
	}
	n := 0
	for _, qc := range q.byDev[deviceID] {
		if want[qc.ID] && !qc.State.Terminal() {
			qc.State = mdm.StateCancelled
			qc.CompletedAt = at.UTC()
			n++
		}
	}
	return n, nil
}

// Prune implements mdm.CommandQueue.
func (q *Queue) Prune(_ context.Context, before time.Time, limit int) (int, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	n := 0
	for dev, cmds := range q.byDev {
		kept := cmds[:0]
		for _, qc := range cmds {
			if qc.State.Terminal() && !qc.CompletedAt.IsZero() && qc.CompletedAt.Before(before) && (limit <= 0 || n < limit) {
				n++
				continue
			}
			kept = append(kept, qc)
		}
		q.byDev[dev] = kept
	}
	return n, nil
}

