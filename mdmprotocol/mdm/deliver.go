package mdm

import (
	"context"
	"strconv"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/syncml"
	"github.com/deploymenttheory/go-microsoft-dm/schema/csp/devdetail"
	"github.com/deploymenttheory/go-microsoft-dm/schema/csp/devicemanageability"
	"github.com/deploymenttheory/go-microsoft-dm/schema/csp/dmclient"
)

// deliver fills the reply with deliverable commands and sets Final unless a
// large object is left in flight. It gates user-scoped commands on
// LoginStatus and the AVD SyncType, chunks items that exceed the client's
// MaxObjSize, and stops before the message grows past MaxMessageBytes.
func (s *Service) deliver(ctx context.Context, sess *Session, req *syncml.Message, resp *syncml.Message, ids *syncml.CmdIDs) error {
	// While the client is uploading a large object, answer 213 and ask for
	// the next chunk with Alert 1222, sending no new commands until the
	// object is complete (OMA DM Protocol 1.2.1 section 7).
	if sess.Assembler != nil && sess.Assembler.Pending() {
		resp.Body.Commands = append(resp.Body.Commands, &syncml.Status{
			CmdID: ids.Next(), MsgRef: strconv.Itoa(sess.ClientMsgID), CmdRef: "0", Cmd: syncml.CmdResults,
			Data: syncml.Data{Value: syncml.StatusChunkedItemAccepted.Wire()},
		}, &syncml.Alert{CmdID: ids.Next(), Data: syncml.AlertNextMessage.Wire()})
		resp.Body.Final = false
		return nil
	}
	deliverable, err := s.cfg.Queue.Deliverable(ctx, sess.DeviceID, 256)
	if err != nil {
		return err
	}
	var deliveries []Delivery
	budget := s.cfg.MaxMessageBytes - len(resp.Body.Commands)*128
	for i := range deliverable {
		qc := &deliverable[i]
		if !s.scopeAllows(sess, qc.Scope) {
			continue
		}
		body, chunks := s.prepare(sess, qc)
		if body == nil {
			continue
		}
		size := commandSize(body)
		if budget <= 0 && len(deliveries) > 0 {
			break // no room; the rest goes next message
		}
		budget -= size
		cmdID := ids.Next()
		assignID(body, cmdID)
		resp.Body.Commands = append(resp.Body.Commands, body)
		sess.Sent[strconv.Itoa(mustAtoi(resp.Header.MsgID))+"/"+cmdID] = qc.ID
		mapChildren(sess, resp.Header.MsgID, body, qc.ID)
		deliveries = append(deliveries, Delivery{CommandID: qc.ID, MsgID: resp.Header.MsgID, At: s.cfg.Clock.Now()})
		if chunks != nil {
			sess.Outgoing = &Outgoing{CommandID: qc.ID, Chunks: chunks, Name: body.Name()}
			break // a large object holds the rest of the queue until it is done
		}
	}
	if len(deliveries) > 0 {
		if err := s.cfg.Queue.MarkSent(ctx, sess.DeviceID, deliveries); err != nil {
			return err
		}
	}
	// Final on every message unless an object is still going out.
	resp.Body.Final = sess.Outgoing == nil
	return nil
}

// continueOrNext answers a "Next Message" request: the next chunk of the
// object in flight, or, when the server has nothing more, the 1222 shape.
func (s *Service) continueOrNext(ctx context.Context, sess *Session, req *syncml.Message, resp *syncml.Message, ids *syncml.CmdIDs) error {
	if sess.Outgoing != nil && len(sess.Outgoing.Chunks) > 0 {
		next := sess.Outgoing.Chunks[0]
		sess.Outgoing.Chunks = sess.Outgoing.Chunks[1:]
		cmdID := ids.Next()
		body := &syncml.Replace{CmdID: cmdID, Items: []syncml.Item{next}}
		resp.Body.Commands = append(resp.Body.Commands, body)
		sess.Sent[resp.Header.MsgID+"/"+cmdID] = sess.Outgoing.CommandID
		if len(sess.Outgoing.Chunks) == 0 {
			sess.Outgoing = nil
			resp.Body.Final = true
		}
		return nil
	}
	// Nothing more to send: more commands may have arrived, so deliver.
	return s.deliver(ctx, sess, req, resp, ids)
}

// scopeAllows reports whether a scoped command may be sent now. User-scoped
// commands wait for a signed-in user (research pitfall "User-scope commands
// before sign-in"); in an AVD device session the server may not send user
// settings and vice versa (MS-MDM 3.2.5.1.5).
func (s *Service) scopeAllows(sess *Session, sc Scope) bool {
	switch sess.Facts.SyncType {
	case syncml.LoginStatusUser: // "user" session
		if sc != ScopeUser {
			return false
		}
	case "device":
		if sc != ScopeDevice {
			return false
		}
	}
	if sc == ScopeUser {
		return sess.Facts.LoginStatus == syncml.LoginStatusUser
	}
	return true
}

// prepare returns the command body to send and, when its single item is a
// large object over the client's MaxObjSize, the chunks after the first.
func (s *Service) prepare(sess *Session, qc *QueuedCommand) (syncml.Command, []syncml.Item) {
	limit := sess.MaxObjSize
	body := cloneCommand(qc.Body)
	if limit <= 0 {
		return body, nil
	}
	items := itemsOf(body)
	if len(items) != 1 || items[0].Data == nil || int64(len(items[0].Data.Value)) <= limit {
		return body, nil
	}
	split, err := syncml.SplitItem(items[0], int(limit))
	if err != nil || len(split) < 2 {
		return body, nil
	}
	setItems(body, []syncml.Item{split[0]})
	return body, split[1:]
}

// queueFirstSessionReads enqueues, once, the reads MS-MDM and the enrollment
// design want on the first session: DevDetail (SwV, LrgObj, MaxSegLen),
// DeviceManageability CSP versions, and the DMClient push channel URI (Phase
// 8 consumes it; the known-issues page says re-read it every session).
func (s *Service) queueFirstSessionReads(ctx context.Context, sess *Session) error {
	uris := []string{devdetail.SwV, devdetail.LrgObj, devdetail.URIMaxSegLen, devicemanageability.CapabilitiesCSPVersions}
	if s.cfg.ProviderID != "" {
		uris = append(uris, dmclient.DeviceProviderPushChannelURI(s.cfg.ProviderID))
	}
	get, err := NewGet(uris, WithID(internalID(sess, "first-reads")))
	if err != nil {
		return err
	}
	get.Internal = true
	if _, err := s.cfg.Queue.Enqueue(ctx, sess.DeviceID, get, s.cfg.Clock.Now()); err != nil && !isConflict(err) {
		return err
	}
	return nil
}

func internalID(sess *Session, name string) string {
	return "mdm-internal/" + sess.DeviceID + "/" + name
}

func mustAtoi(s string) int { n, _ := strconv.Atoi(s); return n }
