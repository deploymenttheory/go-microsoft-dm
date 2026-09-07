package mdm

import (
	"context"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/syncml"
)

// finish persists the results collected this message, advances and stores
// the session (or deletes it when the session ended), runs the unenroll hook
// when a user asked to unenroll, and encodes the reply. A build error from an
// earlier step is returned unchanged.
func (s *Service) finish(ctx context.Context, sess *Session, req *syncml.Message, resp *syncml.Message, ids *syncml.CmdIDs, buildErr error) ([]byte, error) {
	if buildErr != nil {
		return nil, buildErr
	}
	if err := s.flushResults(ctx, sess); err != nil {
		return nil, err
	}
	msgID := mustAtoi(resp.Header.MsgID)
	sess.ServerMsgID = msgID

	// The session ends when the server sends Final and has nothing pending:
	// no command awaiting a status, no object in flight.
	ending := sess.Closed || (resp.Body.Final && !s.hasWork(sess))
	if ending {
		if err := s.cfg.Sessions.DeleteSession(ctx, sess.Key); err != nil {
			return nil, err
		}
		if sess.unenroll && s.cfg.Hooks != nil {
			if err := s.cfg.Hooks.Unenrolled(ctx, sess.DeviceID); err != nil {
				return nil, err
			}
		}
	} else {
		if err := s.cfg.Sessions.PutSession(ctx, sess); err != nil {
			return nil, err
		}
	}

	// OMA DM Protocol 8: when the server has only a SyncHdr status and no
	// work, the exchange is over and the empty package is not sent.
	if ending && onlyHeaderStatus(resp) {
		return nil, nil
	}
	out, err := syncml.Encode(resp, syncml.EncodeOptions{})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// hasWork reports whether the session still expects another round trip: a
// command was sent and not yet acknowledged, or an object is in flight.
func (s *Service) hasWork(sess *Session) bool {
	return len(sess.Sent) > 0 || len(sess.Children) > 0 || sess.Outgoing != nil ||
		(sess.Assembler != nil && sess.Assembler.Pending())
}

// onlyHeaderStatus reports whether the reply is just the SyncHdr status.
func onlyHeaderStatus(resp *syncml.Message) bool {
	if len(resp.Body.Commands) != 1 {
		return false
	}
	st, ok := resp.Body.Commands[0].(*syncml.Status)
	return ok && st.CmdRef == "0"
}
