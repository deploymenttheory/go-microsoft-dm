package mdm

import (
	"context"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/syncml"
)

// pendingResult accumulates one command's answer across the message before
// it is written to the queue in finish.
type pendingResult struct {
	status   *syncml.StatusCode
	origErr  string
	items    []syncml.Item
	children []ChildResult
}

// ensurePending returns the in-flight result for a command, creating the map
// lazily on the session.
func (sess *Session) ensurePending(id string) *pendingResult {
	if sess.results == nil {
		sess.results = map[string]*pendingResult{}
	}
	pr, ok := sess.results[id]
	if !ok {
		pr = &pendingResult{}
		sess.results[id] = pr
	}
	return pr
}

// pendingStatus records a top-level command's Status.
func (sess *Session) pendingStatus(id string, st *syncml.Status) {
	pr := sess.ensurePending(id)
	code := st.Code()
	pr.status = &code
	pr.origErr = st.Data.OriginalError
}

// recordChild records a group member's Status.
func (sess *Session) recordChild(parent string, st *syncml.Status) {
	pr := sess.ensurePending(parent)
	pr.children = append(pr.children, ChildResult{
		Name: st.Cmd, Status: st.Code(), OriginalError: st.Data.OriginalError,
	})
}

// stashResults records the items of a successful Get.
func (sess *Session) stashResults(id string, items []syncml.Item) {
	pr := sess.ensurePending(id)
	pr.items = append(pr.items, items...)
}

// flushResults writes every completed command result to the queue.
func (s *Service) flushResults(ctx context.Context, sess *Session) error {
	for id, pr := range sess.results {
		if pr.status == nil {
			continue // Results without a Status yet; keep for a later message
		}
		r := Result{
			Status: *pr.status, OriginalError: pr.origErr, Items: pr.items,
			Children: pr.children, ReceivedAt: s.cfg.Clock.Now(),
		}
		if err := s.cfg.Queue.StoreResult(ctx, sess.DeviceID, id, r); err != nil {
			return err
		}
		delete(sess.results, id)
	}
	return nil
}
