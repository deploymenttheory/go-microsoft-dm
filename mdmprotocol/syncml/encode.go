package syncml

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
)

// EncodeOptions controls Encode.
type EncodeOptions struct {
	// Indent pretty-prints with this string per level; empty writes the
	// compact form the client expects.
	Indent string
	// XMLDeclaration prefixes <?xml version="1.0" encoding="UTF-8"?>.
	XMLDeclaration bool
}

// Encode writes the message in DTD order with the SYNCML:SYNCML1.2 namespace
// (or 1.1 when the message says so) and syncml:metinf on every Meta child.
// Data is escaped as character data; Data.XML is written verbatim. It fails
// only when a Data has both Value and XML.
func Encode(m *Message, opts EncodeOptions) ([]byte, error) {
	w := &writer{indent: opts.Indent}
	if opts.XMLDeclaration {
		w.raw(`<?xml version="1.0" encoding="UTF-8"?>`)
		w.newline()
	}
	ns := NamespaceSyncML12
	if m.Namespace == NamespaceSyncML11 {
		ns = NamespaceSyncML11
	}
	w.open("SyncML", attr{"xmlns", ns})
	w.header(&m.Header)
	if err := w.body(&m.Body); err != nil {
		return nil, err
	}
	w.close("SyncML")
	return w.buf.Bytes(), nil
}

// Marshal is Encode with the compact default options.
func Marshal(m *Message) ([]byte, error) { return Encode(m, EncodeOptions{}) }

// EncodeCommand writes one command as a fragment, for logs and tests.
func EncodeCommand(c Command, opts EncodeOptions) ([]byte, error) {
	w := &writer{indent: opts.Indent}
	if err := w.command(c); err != nil {
		return nil, err
	}
	return w.buf.Bytes(), nil
}

type attr struct{ name, value string }

type writer struct {
	buf    bytes.Buffer
	indent string
	depth  int
	// inline is set while writing an element whose content is text, so the
	// closing tag follows the text without a newline.
	inline bool
}

func (w *writer) raw(s string) { w.buf.WriteString(s) }

func (w *writer) newline() {
	if w.indent != "" {
		w.buf.WriteByte('\n')
	}
}

func (w *writer) pad() {
	if w.indent == "" {
		return
	}
	for i := 0; i < w.depth; i++ {
		w.buf.WriteString(w.indent)
	}
}

func (w *writer) open(name string, attrs ...attr) {
	w.pad()
	w.buf.WriteByte('<')
	w.buf.WriteString(name)
	for _, a := range attrs {
		w.buf.WriteByte(' ')
		w.buf.WriteString(a.name)
		w.buf.WriteString(`="`)
		xml.EscapeText(&w.buf, []byte(a.value)) //nolint:errcheck // bytes.Buffer never fails
		w.buf.WriteByte('"')
	}
	w.buf.WriteByte('>')
	w.newline()
	w.depth++
}

func (w *writer) close(name string) {
	w.depth--
	w.pad()
	w.buf.WriteString("</")
	w.buf.WriteString(name)
	w.buf.WriteByte('>')
	w.newline()
}

func (w *writer) empty(name string) {
	w.pad()
	w.buf.WriteByte('<')
	w.buf.WriteString(name)
	w.buf.WriteString("/>")
	w.newline()
}

// text writes <name>escaped</name> on one line.
func (w *writer) text(name, value string, attrs ...attr) {
	w.pad()
	w.buf.WriteByte('<')
	w.buf.WriteString(name)
	for _, a := range attrs {
		w.buf.WriteByte(' ')
		w.buf.WriteString(a.name)
		w.buf.WriteString(`="`)
		xml.EscapeText(&w.buf, []byte(a.value)) //nolint:errcheck // bytes.Buffer never fails
		w.buf.WriteByte('"')
	}
	w.buf.WriteByte('>')
	xml.EscapeText(&w.buf, []byte(value)) //nolint:errcheck // bytes.Buffer never fails
	w.buf.WriteString("</")
	w.buf.WriteString(name)
	w.buf.WriteByte('>')
	w.newline()
}

func (w *writer) textIf(name, value string) {
	if value != "" {
		w.text(name, value)
	}
}

func (w *writer) header(h *Header) {
	w.open("SyncHdr")
	w.text("VerDTD", h.VerDTD)
	w.text("VerProto", h.VerProto)
	w.text("SessionID", h.SessionID)
	w.text("MsgID", h.MsgID)
	w.location("Target", h.Target)
	w.location("Source", h.Source)
	w.textIf("RespURI", h.RespURI)
	if h.NoResp {
		w.empty("NoResp")
	}
	w.cred(h.Cred)
	w.meta(h.Meta)
	w.close("SyncHdr")
}

func (w *writer) location(name string, l Location) {
	w.open(name)
	w.text("LocURI", l.LocURI)
	w.textIf("LocName", l.LocName)
	w.close(name)
}

func (w *writer) cred(c *Cred) {
	if c == nil {
		return
	}
	w.open("Cred")
	w.meta(&c.Meta)
	w.text("Data", c.Data)
	w.close("Cred")
}

func (w *writer) chal(c *Chal) {
	if c == nil {
		return
	}
	w.open("Chal")
	w.meta(&c.Meta)
	w.close("Chal")
}

// meta writes the metinf children in DTD order (MetaInfo 1.2.2 section 5.1):
// Format, Type, Mark, Size, NextNonce, MaxMsgSize, MaxObjSize.
func (w *writer) meta(m *Meta) {
	if m == nil {
		return
	}
	w.open("Meta")
	ns := attr{"xmlns", NamespaceMetInf}
	if m.Format != "" {
		w.text("Format", m.Format, ns)
	}
	if m.Type != "" {
		w.text("Type", m.Type, ns)
	}
	if m.Mark != "" {
		w.text("Mark", m.Mark, ns)
	}
	if m.Size != 0 {
		w.text("Size", strconv.FormatInt(m.Size, 10), ns)
	}
	if m.NextNonce != "" {
		w.text("NextNonce", m.NextNonce, ns)
	}
	if m.MaxMsgSize != 0 {
		w.text("MaxMsgSize", strconv.FormatInt(m.MaxMsgSize, 10), ns)
	}
	if m.MaxObjSize != 0 {
		w.text("MaxObjSize", strconv.FormatInt(m.MaxObjSize, 10), ns)
	}
	w.close("Meta")
}

func (w *writer) data(d *Data) error {
	if d == nil {
		return nil
	}
	if d.Value != "" && d.XML != "" {
		return ErrDataConflict
	}
	var attrs []attr
	if d.OriginalError != "" {
		attrs = append(attrs, attr{"msft:" + AttrOriginalError, d.OriginalError})
	}
	if d.XML == "" {
		w.text("Data", d.Value, attrs...)
		return nil
	}
	w.open("Data", attrs...)
	// Markup is written as it was given, on its own lines when indenting.
	w.pad()
	w.buf.WriteString(strings.TrimSpace(d.XML))
	w.newline()
	w.close("Data")
	return nil
}

func (w *writer) item(it *Item) error {
	w.open("Item")
	if it.Target != "" {
		w.open("Target")
		w.text("LocURI", it.Target)
		w.close("Target")
	}
	if it.Source != "" {
		w.open("Source")
		w.text("LocURI", it.Source)
		w.close("Source")
	}
	w.meta(it.Meta)
	if err := w.data(it.Data); err != nil {
		return err
	}
	if it.MoreData {
		w.empty("MoreData")
	}
	w.close("Item")
	return nil
}

func (w *writer) items(items []Item) error {
	for i := range items {
		if err := w.item(&items[i]); err != nil {
			return err
		}
	}
	return nil
}

func (w *writer) body(b *Body) error {
	if b.MSFTNamespace {
		w.open("SyncBody", attr{"xmlns:msft", NamespaceMSFT})
	} else {
		w.open("SyncBody")
	}
	for _, c := range b.Commands {
		if err := w.command(c); err != nil {
			return err
		}
	}
	if b.Final {
		w.empty("Final")
	}
	w.close("SyncBody")
	return nil
}

func (w *writer) command(c Command) error {
	switch n := c.(type) {
	case *Add:
		return w.itemCommand(CmdAdd, n.CmdID, n.NoResp, "", n.Cred, n.Meta, n.Items)
	case *Replace:
		return w.itemCommand(CmdReplace, n.CmdID, n.NoResp, "", n.Cred, n.Meta, n.Items)
	case *Delete:
		return w.itemCommand(CmdDelete, n.CmdID, n.NoResp, "", n.Cred, n.Meta, n.Items)
	case *Get:
		return w.itemCommand(CmdGet, n.CmdID, n.NoResp, n.Lang, n.Cred, n.Meta, n.Items)
	case *Exec:
		// DTD: CmdID, NoResp?, Cred?, Meta?, Correlator?, Item
		w.open(CmdExec)
		w.text("CmdID", n.CmdID)
		if n.NoResp {
			w.empty("NoResp")
		}
		w.cred(n.Cred)
		w.meta(n.Meta)
		w.textIf("Correlator", n.Correlator)
		if err := w.items(n.Items); err != nil {
			return err
		}
		w.close(CmdExec)
	case *Alert:
		// DTD: CmdID, NoResp?, Cred?, Data?, Correlator?, Item*
		w.open(CmdAlert)
		w.text("CmdID", n.CmdID)
		if n.NoResp {
			w.empty("NoResp")
		}
		w.cred(n.Cred)
		w.textIf("Data", n.Data)
		w.textIf("Correlator", n.Correlator)
		if err := w.items(n.Items); err != nil {
			return err
		}
		w.close(CmdAlert)
	case *Atomic:
		return w.group(CmdAtomic, n.CmdID, n.NoResp, n.Meta, n.Commands)
	case *Sequence:
		return w.group(CmdSequence, n.CmdID, n.NoResp, n.Meta, n.Commands)
	case *Status:
		// DTD: CmdID, MsgRef, CmdRef, Cmd, TargetRef*, SourceRef*, Cred?, Chal?, Data, Item*
		w.open(CmdStatus)
		w.text("CmdID", n.CmdID)
		w.text("MsgRef", n.MsgRef)
		w.text("CmdRef", n.CmdRef)
		w.text("Cmd", n.Cmd)
		for _, r := range n.TargetRef {
			w.text("TargetRef", r)
		}
		for _, r := range n.SourceRef {
			w.text("SourceRef", r)
		}
		w.cred(n.Cred)
		w.chal(n.Chal)
		d := n.Data
		if err := w.data(&d); err != nil {
			return err
		}
		if err := w.items(n.Items); err != nil {
			return err
		}
		w.close(CmdStatus)
	case *Results:
		// DTD: CmdID, MsgRef?, CmdRef, Meta?, TargetRef?, SourceRef?, Item+;
		// MS-MDM 2.2.7.8 inserts Cmd after CmdRef.
		w.open(CmdResults)
		w.text("CmdID", n.CmdID)
		w.textIf("MsgRef", n.MsgRef)
		w.text("CmdRef", n.CmdRef)
		w.textIf("Cmd", n.Cmd)
		w.meta(n.Meta)
		w.textIf("TargetRef", n.TargetRef)
		w.textIf("SourceRef", n.SourceRef)
		if err := w.items(n.Items); err != nil {
			return err
		}
		w.close(CmdResults)
	default:
		return fmt.Errorf("%w: %T", ErrUnknownElement, c)
	}
	return nil
}

// itemCommand writes Add, Replace, Delete and Get, whose DTD order is
// CmdID, NoResp?, (Lang?), Cred?, Meta?, Item+.
func (w *writer) itemCommand(name, cmdID string, noResp bool, lang string, cred *Cred, meta *Meta, items []Item) error {
	w.open(name)
	w.text("CmdID", cmdID)
	if noResp {
		w.empty("NoResp")
	}
	w.textIf("Lang", lang)
	w.cred(cred)
	w.meta(meta)
	if err := w.items(items); err != nil {
		return err
	}
	w.close(name)
	return nil
}

func (w *writer) group(name, cmdID string, noResp bool, meta *Meta, cmds []Command) error {
	w.open(name)
	w.text("CmdID", cmdID)
	if noResp {
		w.empty("NoResp")
	}
	w.meta(meta)
	for _, c := range cmds {
		if err := w.command(c); err != nil {
			return err
		}
	}
	w.close(name)
	return nil
}
