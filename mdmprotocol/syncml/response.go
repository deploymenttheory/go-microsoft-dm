package syncml

import (
	"fmt"
	"strconv"
)

// NextMsgID returns the MsgID that follows last ("" yields "1").
func NextMsgID(last string) (string, error) {
	if last == "" {
		return "1", nil
	}
	n, err := strconv.Atoi(last)
	if err != nil || n < 1 {
		return "", fmt.Errorf("%w: MsgID %q", ErrMsgID, last)
	}
	return strconv.Itoa(n + 1), nil
}

// NewResponse starts the server's reply to req: the same SessionID, the
// given MsgID, Target and Source swapped (Source is serverURL when set, else
// the request's Target), and the request's namespace so a 1.1 client is
// answered in kind. The body is empty; append commands and set Final.
func NewResponse(req *Message, serverURL, msgID string) *Message {
	src := serverURL
	if src == "" {
		src = req.Header.Target.LocURI
	}
	return &Message{
		Namespace: req.Namespace,
		Header: Header{
			VerDTD:    VerDTD,
			VerProto:  VerProto,
			SessionID: req.Header.SessionID,
			MsgID:     msgID,
			Target:    Location{LocURI: req.Header.Source.LocURI},
			Source:    Location{LocURI: src},
		},
	}
}

// StatusForHeader answers the request's SyncHdr, which MS-MDM 2.2.6.1 says
// must be the first Status in the response.
func StatusForHeader(req *Message, cmdID string, code StatusCode) *Status {
	return &Status{CmdID: cmdID, MsgRef: req.Header.MsgID, CmdRef: "0", Cmd: CmdSyncHdr, Data: Data{Value: code.Wire()}}
}

// StatusFor answers one command of the request.
func StatusFor(req *Message, cmd Command, cmdID string, code StatusCode) *Status {
	return &Status{CmdID: cmdID, MsgRef: req.Header.MsgID, CmdRef: cmd.ID(), Cmd: cmd.Name(), Data: Data{Value: code.Wire()}}
}

// NewNextMessage builds the "Next Message" response of OMA DM Protocol
// 6.2: Status for the SyncHdr, Alert 1222, no other command and no Final.
func NewNextMessage(req *Message, serverURL, msgID string) *Message {
	m := NewResponse(req, serverURL, msgID)
	m.Body.Commands = []Command{
		StatusForHeader(req, "1", StatusOK),
		&Alert{CmdID: "2", Data: AlertNextMessage.Wire()},
	}
	return m
}

// NewAbort builds the session-abort response: Status for the SyncHdr and
// Alert 1223 (OMA DM Protocol 8.1).
func NewAbort(req *Message, serverURL, msgID string) *Message {
	m := NewResponse(req, serverURL, msgID)
	m.Body.Commands = []Command{
		StatusForHeader(req, "1", StatusOK),
		&Alert{CmdID: "2", Data: AlertSessionAbort.Wire()},
	}
	m.Body.Final = true
	return m
}

// IsNextMessageRequest reports whether the message asks for more messages
// with Alert 1222 and carries no other command.
func IsNextMessageRequest(m *Message) bool {
	found := false
	for _, c := range m.Body.Commands {
		switch n := c.(type) {
		case *Status:
		case *Alert:
			if n.Code() != AlertNextMessage {
				return false
			}
			found = true
		default:
			return false
		}
	}
	return found
}

// IsPackageOne reports whether the message opens a session: it carries a
// 1200 or 1201 alert and a Replace with DevInfo items (OMA DM Protocol 8.3).
func IsPackageOne(m *Message) bool {
	hasAlert := m.Body.HasAlert(AlertClientInitiated) || m.Body.HasAlert(AlertServerInitiated)
	hasDevInfo := false
	for _, c := range m.Body.Commands {
		if r, ok := c.(*Replace); ok {
			for _, it := range r.Items {
				if len(it.Source) >= 10 && it.Source[:10] == "./DevInfo/" {
					hasDevInfo = true
				}
			}
		}
	}
	return hasAlert && hasDevInfo
}

// DevInfo returns the ./DevInfo/* values from a package 1 Replace, keyed by
// leaf name (DevId, Man, Mod, DmV, Lang).
func DevInfo(m *Message) map[string]string {
	out := map[string]string{}
	for _, c := range m.Body.Commands {
		r, ok := c.(*Replace)
		if !ok {
			continue
		}
		for _, it := range r.Items {
			if len(it.Source) > 10 && it.Source[:10] == "./DevInfo/" {
				out[it.Source[10:]] = it.Data.Text()
			}
		}
	}
	return out
}

// LoginStatus returns the Data of the AlertTypeLoginStatus alert, or "".
func LoginStatus(m *Message) string {
	for _, a := range m.Body.Alerts() {
		if a.Code() != AlertClientEvent {
			continue
		}
		for _, it := range a.Items {
			if it.Meta != nil && it.Meta.Type == AlertTypeLoginStatus {
				return it.Data.Text()
			}
		}
	}
	return ""
}

// CmdIDs hands out sequential CmdIDs for one message, starting at 1.
type CmdIDs struct{ n int }

// Next returns the next CmdID.
func (c *CmdIDs) Next() string {
	c.n++
	return strconv.Itoa(c.n)
}
