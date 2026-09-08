package simulator

import (
	"strconv"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/syncml"
)

// packageOne builds the client's initialization message.
func (c *Device) packageOne(st *sessionState) *syncml.Message {
	st.clientMsg = 1
	m := &syncml.Message{Namespace: c.Namespace, Header: c.header(st)}
	ids := &syncml.CmdIDs{}
	m.Body.Commands = append(m.Body.Commands, &syncml.Alert{CmdID: ids.Next(), Data: syncml.AlertClientInitiated.Wire()})
	if c.LoginStatus != "" {
		m.Body.Commands = append(m.Body.Commands, loginAlert(ids.Next(), syncml.AlertTypeLoginStatus, c.LoginStatus))
	}
	if c.SyncType != "" {
		m.Body.Commands = append(m.Body.Commands, loginAlert(ids.Next(), syncml.AlertTypeSyncType, c.SyncType))
	}
	replace := &syncml.Replace{CmdID: ids.Next()}
	for _, leaf := range []string{"DevId", "Man", "Mod", "DmV", "Lang"} {
		if v, ok := c.DevInfo[leaf]; ok {
			replace.Items = append(replace.Items, syncml.Item{Source: "./DevInfo/" + leaf, Data: &syncml.Data{Value: v}})
		}
	}
	m.Body.Commands = append(m.Body.Commands, replace)
	for _, g := range c.Generics {
		it := syncml.Item{Source: g.Source, Meta: &syncml.Meta{Type: g.Type, Format: g.Format, Mark: g.Mark}, Data: &syncml.Data{Value: g.Data}}
		m.Body.Commands = append(m.Body.Commands, &syncml.Alert{CmdID: ids.Next(), Data: syncml.AlertGeneric.Wire(), Items: []syncml.Item{it}})
	}
	m.Body.Final = true
	return m
}

// header builds the SyncHdr for the next client message.
func (c *Device) header(st *sessionState) syncml.Header {
	h := syncml.Header{
		VerDTD: syncml.VerDTD, VerProto: syncml.VerProto, SessionID: st.sessionID, MsgID: strconv.Itoa(st.clientMsg),
		Target: syncml.Location{LocURI: c.ManagementURL}, Source: syncml.Location{LocURI: c.DeviceID},
	}
	if c.MaxObjSize > 0 {
		h.Meta = &syncml.Meta{MaxObjSize: c.MaxObjSize}
	}
	if st.authSent && len(st.nonce) > 0 && c.Password != "" {
		h.Cred = syncml.NewMD5Cred(syncml.MD5Digest(c.Username, c.Password, st.nonce))
	}
	return h
}

func loginAlert(cmdID, typ, data string) *syncml.Alert {
	return &syncml.Alert{CmdID: cmdID, Data: syncml.AlertClientEvent.Wire(), Items: []syncml.Item{
		{Meta: &syncml.Meta{Type: typ, Format: syncml.FormatChr}, Data: &syncml.Data{Value: data}},
	}}
}

// answer processes one server message and builds the client's reply. done is
// true when the server ended the session (Final and no command needing a
// response, or a session abort).
func (c *Device) answer(resp *syncml.Message, st *sessionState) (*syncml.Message, bool, error) {
	// Record and handle an authentication challenge on the SyncHdr status.
	for _, s := range resp.Body.Statuses() {
		if s.CmdRef == "0" && s.Chal != nil {
			if nonce, err := s.Chal.Nonce(); err == nil {
				st.nonce = nonce
			}
			st.authSent = true
			st.transcript.Challenged++
			if st.transcript.Challenged > 2 {
				return nil, false, ErrChallengeLoop
			}
			// Revert to package 1 with credentials (OMA DM Security 9).
			st.clientMsg++
			return c.reAuth(st), false, nil
		}
	}

	st.clientMsg++
	reply := &syncml.Message{Namespace: c.Namespace, Header: c.header(st)}
	ids := &syncml.CmdIDs{}
	reply.Body.Commands = append(reply.Body.Commands, syncml.StatusForHeader(resp, ids.Next(), syncml.StatusOK))

	emit := resp.Body.Final
	needsResponse := false
	for _, cmd := range resp.Body.Commands {
		if _, ok := cmd.(*syncml.Status); ok {
			continue
		}
		needsResponse = true
		c.handleCommand(resp, cmd, reply, ids, st, emit)
	}

	if !resp.Body.Final {
		// The server is mid-package (a large object): accumulate and ask for
		// more with Alert 1222.
		reply.Body.Commands = []syncml.Command{syncml.StatusForHeader(resp, "1", syncml.StatusOK),
			&syncml.Alert{CmdID: "2", Data: syncml.AlertNextMessage.Wire()}}
		return reply, false, nil
	}
	if !needsResponse {
		return nil, true, nil
	}
	reply.Body.Final = true
	return reply, false, nil
}

// reAuth rebuilds package 1 with credentials after a challenge.
func (c *Device) reAuth(st *sessionState) *syncml.Message {
	saved := st.clientMsg
	m := c.packageOne(st)
	st.clientMsg = saved
	m.Header.MsgID = strconv.Itoa(saved)
	if len(st.nonce) > 0 && c.Password != "" {
		m.Header.Cred = syncml.NewMD5Cred(syncml.MD5Digest(c.Username, c.Password, st.nonce))
	}
	return m
}

// handleCommand answers one server command. When emit is false (the server
// is mid-package) it only accumulates chunked items and produces no output.
func (c *Device) handleCommand(req *syncml.Message, cmd syncml.Command, reply *syncml.Message, ids *syncml.CmdIDs, st *sessionState, emit bool) {
	st.transcript.ServerCommands = append(st.transcript.ServerCommands, cmd)
	switch b := cmd.(type) {
	case *syncml.Get:
		if !emit {
			return
		}
		status := syncml.StatusOK
		for _, it := range b.Items {
			if v, ok := c.Tree[it.Target]; ok {
				reply.Body.Commands = append(reply.Body.Commands, &syncml.Results{
					CmdID: ids.Next(), MsgRef: req.Header.MsgID, CmdRef: b.CmdID, Cmd: syncml.CmdGet,
					Items: []syncml.Item{{Source: it.Target, Meta: &syncml.Meta{Format: syncml.FormatChr}, Data: &syncml.Data{Value: v}}},
				})
				st.transcript.Results = append(st.transcript.Results, it.Target)
			} else {
				status = syncml.StatusNotFound
			}
		}
		c.status(req, cmd, reply, ids, st, status)
	case *syncml.Add:
		c.applyItems(b.Items, st, req, cmd, reply, ids, emit)
	case *syncml.Replace:
		c.applyItems(b.Items, st, req, cmd, reply, ids, emit)
	case *syncml.Delete:
		if !emit {
			return
		}
		for _, it := range b.Items {
			delete(c.Tree, it.Target)
		}
		c.status(req, cmd, reply, ids, st, syncml.StatusOK)
	case *syncml.Atomic:
		c.handleGroup(req, b.Commands, reply, ids, st, emit)
		if emit {
			c.status(req, cmd, reply, ids, st, syncml.StatusOK)
		}
	case *syncml.Sequence:
		c.handleGroup(req, b.Commands, reply, ids, st, emit)
		if emit {
			c.status(req, cmd, reply, ids, st, syncml.StatusOK)
		}
	default:
		// Exec and any other leaf command are acknowledged.
		if emit {
			c.status(req, cmd, reply, ids, st, syncml.StatusOK)
		}
	}
}

// applyItems reassembles chunked items and writes them to the tree.
func (c *Device) applyItems(items []syncml.Item, st *sessionState, req *syncml.Message, cmd syncml.Command, reply *syncml.Message, ids *syncml.CmdIDs, emit bool) {
	for _, it := range items {
		full, done, err := st.assembler.Add(it)
		if err != nil || !done {
			continue // more chunks to come
		}
		if full.Target != "" {
			c.Tree[full.Target] = full.Data.Text()
		}
	}
	if emit {
		c.status(req, cmd, reply, ids, st, syncml.StatusOK)
	}
}

// status appends a Status for a command.
func (c *Device) status(req *syncml.Message, cmd syncml.Command, reply *syncml.Message, ids *syncml.CmdIDs, st *sessionState, code syncml.StatusCode) {
	st.transcript.Statuses[cmd.ID()] = code
	reply.Body.Commands = append(reply.Body.Commands, &syncml.Status{
		CmdID: ids.Next(), MsgRef: req.Header.MsgID, CmdRef: cmd.ID(), Cmd: cmd.Name(), Data: syncml.Data{Value: code.Wire()},
	})
}

// handleGroup answers each member of an Atomic or Sequence.
func (c *Device) handleGroup(req *syncml.Message, members []syncml.Command, reply *syncml.Message, ids *syncml.CmdIDs, st *sessionState, emit bool) {
	for _, m := range members {
		c.handleCommand(req, m, reply, ids, st, emit)
	}
}
