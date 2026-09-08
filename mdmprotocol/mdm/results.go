package mdm

import (

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/syncml"
)

// fileStatus files a client Status against the command it answers. The
// client references our CmdID in CmdRef and our MsgID in MsgRef; the engine
// mapped "MsgID/CmdID" to the command ID when it sent it.
func (s *Service) fileStatus(sess *Session, st *syncml.Status) {
	if st.CmdRef == "0" {
		return // status on our SyncHdr
	}
	key := st.MsgRef + "/" + st.CmdRef
	if parent, ok := sess.Children[key]; ok {
		sess.recordChild(parent, st)
		delete(sess.Children, key)
		return
	}
	id, ok := sess.Sent[key]
	if !ok {
		return
	}
	sess.pendingStatus(id, st)
	// A Status answers the command; a chunked Get's Results may still be
	// arriving, so keep the entry until reassembly completes.
	if sess.AssemblingCommand != id {
		delete(sess.Sent, key)
	}
}

// fileResults files a client Results against the Get it answers, reassembling
// a chunked object first.
func (s *Service) fileResults(sess *Session, res *syncml.Results) error {
	key := res.MsgRef
	if key == "" {
		key = "1"
	}
	key += "/" + res.CmdRef
	id, ok := sess.Sent[key]
	if !ok {
		if p, okc := sess.Children[key]; okc {
			id = p
		} else {
			return nil
		}
	}
	if sess.Assembler == nil {
		sess.Assembler = &syncml.Assembler{MaxSize: int64(s.cfg.MaxRequestSize)}
	}
	var assembled []syncml.Item
	for _, it := range res.Items {
		out, done, err := sess.Assembler.Add(it)
		if err != nil {
			return err
		}
		if done {
			assembled = append(assembled, out)
		} else {
			sess.AssemblingCommand = id
		}
	}
	if sess.Assembler.Pending() {
		return nil // more chunks to come; answered with 213 elsewhere
	}
	sess.AssemblingCommand = ""
	sess.stashResults(id, assembled)
	// The Sent entry is kept: the matching Status that follows the Results
	// finalizes the command and clears it.
	return nil
}
