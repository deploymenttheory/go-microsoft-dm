package syncml

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// Validate applies the rules a session engine relies on before acting on a
// decoded message. It returns every failure joined, each a *ValidationError
// unwrapping to ErrInvalid and its rule sentinel, or nil.
//
// Rules: the namespace is 1.2 or 1.1; VerDTD is "1.2" and VerProto "DM/1.2";
// SessionID is non-empty; MsgID is a positive integer; Target and Source
// LocURIs are non-empty; the body has at least one command; every CmdID is
// non-empty, not "0" and unique within the message including nested
// commands; Add, Replace, Delete and Get carry at least one Item, Exec
// exactly one; every Item LocURI passes CheckLocURI; Alert Data is a known
// code, and 1224 and 1226 alerts carry an Item with Meta/Type; Status has
// MsgRef, CmdRef, a known Cmd and a numeric Data; Results has CmdRef and an
// Item; Meta/Format values are known; Atomic contains no Atomic or Get and
// no Add then Replace on the same node.
func Validate(m *Message) error {
	v := &validator{ids: map[string]string{}}
	v.header(&m.Header, m.Namespace)
	v.body(&m.Body)
	return errors.Join(v.errs...)
}

// CheckLocURI applies the Learn OMA DM page's address rules: not empty, not
// starting with "/", every node name non-empty and not just "*". A "?prop="
// or "?list=" query is allowed and ignored.
func CheckLocURI(uri string) error {
	if uri == "" {
		return errors.New("empty LocURI")
	}
	if strings.HasPrefix(uri, "/") {
		return errors.New("LocURI cannot start with /")
	}
	path := uri
	if i := strings.IndexByte(uri, '?'); i >= 0 {
		path = uri[:i]
	}
	for _, seg := range strings.Split(path, "/") {
		if seg == "" {
			return errors.New("empty node name in LocURI")
		}
		if seg == "*" {
			return errors.New("node name cannot be only *")
		}
	}
	return nil
}

// ChildNodes splits the Data a Get on an interior node returns: child names
// joined by "/" and URI-encoded.
func ChildNodes(data string) []string {
	if strings.TrimSpace(data) == "" {
		return nil
	}
	parts := strings.Split(strings.TrimSpace(data), "/")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p == "" {
			continue
		}
		if u, err := url.PathUnescape(p); err == nil {
			p = u
		}
		out = append(out, p)
	}
	return out
}

type validator struct {
	errs []error
	// ids maps CmdID to the path of its first use.
	ids map[string]string
}

func (v *validator) fail(rule error, path, format string, args ...any) {
	v.errs = append(v.errs, &ValidationError{Rule: rule, Path: path, Msg: fmt.Sprintf(format, args...)})
}

func (v *validator) header(h *Header, ns string) {
	if ns != NamespaceSyncML12 && ns != NamespaceSyncML11 {
		v.fail(ErrVersion, "SyncML", "xmlns must be %s (or %s); got %q", NamespaceSyncML12, NamespaceSyncML11, ns)
	}
	if h.VerDTD != VerDTD {
		v.fail(ErrVersion, "SyncHdr/VerDTD", "must be %q; got %q", VerDTD, h.VerDTD)
	}
	if h.VerProto != VerProto {
		v.fail(ErrVersion, "SyncHdr/VerProto", "must be %q; got %q", VerProto, h.VerProto)
	}
	if h.SessionID == "" {
		v.fail(ErrSessionID, "SyncHdr/SessionID", "missing")
	}
	if n, err := strconv.Atoi(h.MsgID); err != nil || n < 1 {
		v.fail(ErrMsgID, "SyncHdr/MsgID", "must be a positive integer; got %q", h.MsgID)
	}
	if h.Target.LocURI == "" {
		v.fail(ErrRouting, "SyncHdr/Target/LocURI", "missing")
	}
	if h.Source.LocURI == "" {
		v.fail(ErrRouting, "SyncHdr/Source/LocURI", "missing")
	}
	if h.Cred != nil {
		v.cred(h.Cred, "SyncHdr/Cred")
	}
	if h.Meta != nil {
		v.meta(h.Meta, "SyncHdr/Meta")
	}
}

func (v *validator) cred(c *Cred, path string) {
	if c.Meta.Type != AuthBasic && c.Meta.Type != AuthMD5 {
		v.fail(ErrAuth, path+"/Meta/Type", "must be %s or %s; got %q", AuthBasic, AuthMD5, c.Meta.Type)
	}
	if c.Data == "" {
		v.fail(ErrAuth, path+"/Data", "missing")
	}
}

func (v *validator) meta(m *Meta, path string) {
	if m.Format != "" && !KnownFormat(m.Format) {
		v.fail(ErrFormat, path+"/Format", "unknown format %q", m.Format)
	}
}

func (v *validator) body(b *Body) {
	if len(b.Commands) == 0 {
		v.fail(ErrEmptyBody, "SyncBody", "no commands")
	}
	for _, c := range b.Commands {
		v.command(c, "SyncBody", false)
	}
}

func (v *validator) cmdID(c Command, path string) string {
	id := c.ID()
	p := fmt.Sprintf("%s/%s[%s]", path, c.Name(), id)
	if id == "" || id == "0" {
		v.fail(ErrCmdID, p, "CmdID must be present and not \"0\"")
		return p
	}
	if first, dup := v.ids[id]; dup {
		v.fail(ErrDuplicateCmdID, p, "CmdID %q already used by %s", id, first)
	} else {
		v.ids[id] = p
	}
	return p
}

func (v *validator) items(items []Item, path string, minItems, maxItems int) {
	if len(items) < minItems {
		v.fail(ErrItems, path, "needs at least %d Item; got %d", minItems, len(items))
	}
	if maxItems > 0 && len(items) > maxItems {
		v.fail(ErrItems, path, "allows at most %d Item; got %d", maxItems, len(items))
	}
	for i := range items {
		it := &items[i]
		ip := fmt.Sprintf("%s/Item[%d]", path, i)
		if uri := it.LocURI(); uri == "" {
			v.fail(ErrLocURI, ip, "Item needs a Target or Source LocURI")
		} else if err := CheckLocURI(uri); err != nil {
			v.fail(ErrLocURI, ip, "%v: %q", err, uri)
		}
		if it.Meta != nil {
			v.meta(it.Meta, ip+"/Meta")
		}
		if it.Data != nil && it.Data.Value != "" && it.Data.XML != "" {
			v.fail(ErrItems, ip+"/Data", "has both Value and XML")
		}
	}
}

func (v *validator) command(c Command, path string, inAtomic bool) {
	p := v.cmdID(c, path)
	switch n := c.(type) {
	case *Add:
		v.items(n.Items, p, 1, 0)
		if n.Meta != nil {
			v.meta(n.Meta, p+"/Meta")
		}
	case *Replace:
		v.items(n.Items, p, 1, 0)
		if n.Meta != nil {
			v.meta(n.Meta, p+"/Meta")
		}
	case *Delete:
		v.items(n.Items, p, 1, 0)
	case *Get:
		v.items(n.Items, p, 1, 0)
		if inAtomic {
			v.fail(ErrAtomicGet, p, "Get inside Atomic returns 500 on Windows")
		}
	case *Exec:
		v.items(n.Items, p, 1, 1)
	case *Alert:
		code, err := ParseAlertCode(n.Data)
		if err != nil || !code.Known() {
			v.fail(ErrAlertCode, p+"/Data", "unknown alert code %q", n.Data)
		}
		if code == AlertClientEvent || code == AlertGeneric {
			if len(n.Items) == 0 {
				v.fail(ErrItems, p, "alert %d needs an Item", code)
			}
			for i := range n.Items {
				if n.Items[i].Meta == nil || n.Items[i].Meta.Type == "" {
					v.fail(ErrItems, fmt.Sprintf("%s/Item[%d]/Meta/Type", p, i), "alert %d Item needs Meta/Type", code)
				}
			}
		}
		for i := range n.Items {
			if n.Items[i].Meta != nil {
				v.meta(n.Items[i].Meta, fmt.Sprintf("%s/Item[%d]/Meta", p, i))
			}
		}
	case *Atomic:
		if inAtomic {
			v.fail(ErrAtomicNested, p, "nested Atomic returns 500 and the parent 507 on Windows")
		}
		if len(n.Commands) == 0 {
			v.fail(ErrItems, p, "Atomic needs at least one command")
		}
		v.atomicAddReplace(n.Commands, p)
		for _, sub := range n.Commands {
			v.command(sub, p, true)
		}
	case *Sequence:
		if len(n.Commands) == 0 {
			v.fail(ErrItems, p, "Sequence needs at least one command")
		}
		for _, sub := range n.Commands {
			v.command(sub, p, inAtomic)
		}
	case *Status:
		if n.MsgRef == "" {
			v.fail(ErrStatusRef, p+"/MsgRef", "missing")
		}
		if n.CmdRef == "" {
			v.fail(ErrStatusRef, p+"/CmdRef", "missing")
		}
		switch n.Cmd {
		case CmdSyncHdr, CmdAdd, CmdAlert, CmdAtomic, CmdDelete, CmdExec, CmdGet, CmdReplace, CmdResults, CmdSequence, CmdStatus:
		default:
			v.fail(ErrStatusRef, p+"/Cmd", "unknown command name %q", n.Cmd)
		}
		if n.CmdRef == "0" && n.Cmd != CmdSyncHdr {
			v.fail(ErrStatusRef, p+"/Cmd", "CmdRef 0 must name SyncHdr; got %q", n.Cmd)
		}
		if _, err := ParseStatusCode(n.Data.Value); err != nil {
			v.fail(ErrStatusCode, p+"/Data", "not a status code: %q", n.Data.Value)
		}
		if n.Chal != nil && n.Chal.Meta.Type != AuthBasic && n.Chal.Meta.Type != AuthMD5 {
			v.fail(ErrAuth, p+"/Chal/Meta/Type", "must be %s or %s; got %q", AuthBasic, AuthMD5, n.Chal.Meta.Type)
		}
		if n.Cred != nil {
			v.cred(n.Cred, p+"/Cred")
		}
		for i := range n.Items {
			if n.Items[i].Meta != nil {
				v.meta(n.Items[i].Meta, fmt.Sprintf("%s/Item[%d]/Meta", p, i))
			}
		}
	case *Results:
		if n.CmdRef == "" {
			v.fail(ErrStatusRef, p+"/CmdRef", "missing")
		}
		if n.MsgRef != "" {
			if k, err := strconv.Atoi(n.MsgRef); err != nil || k < 1 {
				v.fail(ErrStatusRef, p+"/MsgRef", "must be a positive integer; got %q", n.MsgRef)
			}
		}
		v.items(n.Items, p, 1, 0)
		if n.Meta != nil {
			v.meta(n.Meta, p+"/Meta")
		}
	}
}

// atomicAddReplace flags an Add followed by a Replace on the same node
// inside one Atomic, which the Learn OMA DM page says is unsupported.
func (v *validator) atomicAddReplace(cmds []Command, path string) {
	added := map[string]bool{}
	for _, c := range cmds {
		switch n := c.(type) {
		case *Add:
			for _, it := range n.Items {
				added[it.LocURI()] = true
			}
		case *Replace:
			for _, it := range n.Items {
				if added[it.LocURI()] {
					v.fail(ErrAtomicAddThenReplace, fmt.Sprintf("%s/Replace[%s]", path, n.CmdID), "Add then Replace on %q inside one Atomic is unsupported", it.LocURI())
				}
			}
		}
	}
}
