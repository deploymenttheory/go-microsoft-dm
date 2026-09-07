package syncml

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// DefaultMaxSize bounds a document Decode will read. MS-MDM publishes no
// MaxMsgSize for the Windows client; 8 MiB is far above any observed session
// message and small enough that a hostile body cannot exhaust memory.
const DefaultMaxSize = 8 << 20

// DecodeOptions controls Decode.
type DecodeOptions struct {
	// MaxSize replaces DefaultMaxSize when positive.
	MaxSize int
}

// Decode parses one SyncML document. It is structurally strict (DTD elements
// only, Final last, Windows commands only) and lexically lenient (either
// namespace or none, whitespace around identifiers, Meta children with or
// without the metinf namespace, CDATA, a utf-16 declaration on UTF-8 bytes).
// It does not apply the session rules; call Validate for those.
func Decode(data []byte, opts DecodeOptions) (*Message, error) {
	limit := opts.MaxSize
	if limit <= 0 {
		limit = DefaultMaxSize
	}
	if len(data) > limit {
		return nil, fmt.Errorf("%w: %d bytes, limit %d", ErrTooLarge, len(data), limit)
	}
	data = transcodeUTF16(data)
	p := &parser{src: data, dec: xml.NewDecoder(bytes.NewReader(data))}
	p.dec.CharsetReader = charsetReader
	m, err := p.message()
	if err != nil {
		var se *SyntaxError
		if errors.As(err, &se) || errors.Is(err, ErrSyntax) || errors.Is(err, ErrNamespace) ||
			errors.Is(err, ErrUnsupportedCommand) || errors.Is(err, ErrUnknownElement) || errors.Is(err, ErrFinalNotLast) {
			return nil, err
		}
		return nil, &SyntaxError{Element: "SyncML", Msg: "xml", Err: err}
	}
	return m, nil
}

// Unmarshal is Decode with the default options.
func Unmarshal(data []byte) (*Message, error) { return Decode(data, DecodeOptions{}) }

// DecodeCommand parses one command fragment such as an <Alert> sample.
func DecodeCommand(data []byte) (Command, error) {
	if len(data) > DefaultMaxSize {
		return nil, fmt.Errorf("%w: %d bytes", ErrTooLarge, len(data))
	}
	data = transcodeUTF16(data)
	p := &parser{src: data, dec: xml.NewDecoder(bytes.NewReader(data))}
	p.dec.CharsetReader = charsetReader
	start, err := p.nextStart()
	if err != nil {
		return nil, &SyntaxError{Element: "fragment", Msg: "no root element", Err: err}
	}
	c, err := p.command(start, "fragment")
	if err != nil {
		return nil, err
	}
	return c, nil
}

// transcodeUTF16 converts a document that is actually UTF-16 (byte-order
// mark, or a NUL beside the leading '<') to UTF-8 before the XML decoder
// sees it, because the decoder must read the XML declaration itself and
// can only do that in an ASCII-compatible encoding. Anything else is
// returned unchanged.
func transcodeUTF16(data []byte) []byte {
	if len(data) < 2 {
		return data
	}
	switch {
	case data[0] == 0xFE && data[1] == 0xFF, data[0] == 0xFF && data[1] == 0xFE:
		return decodeUTF16(data)
	case data[0] == 0 && data[1] == '<', data[0] == '<' && data[1] == 0:
		return decodeUTF16(data)
	}
	return data
}

// charsetReader accepts the labels Windows and its documentation use. A
// utf-16 label on bytes that contain no NUL is treated as mislabelled UTF-8,
// which is what Microsoft's samples are and what transcodeUTF16 leaves
// behind for a real UTF-16 document.
func charsetReader(label string, r io.Reader) (io.Reader, error) {
	switch strings.ToLower(strings.TrimSpace(label)) {
	case "", "utf-8", "utf8", "us-ascii", "ascii":
		return r, nil
	case "utf-16", "utf-16le", "utf-16be", "utf16":
		raw, err := io.ReadAll(io.LimitReader(r, DefaultMaxSize+1))
		if err != nil {
			return nil, err
		}
		if !bytes.ContainsRune(raw, 0) {
			return bytes.NewReader(raw), nil
		}
		return bytes.NewReader(decodeUTF16(raw)), nil
	default:
		return nil, fmt.Errorf("unsupported charset %q", label)
	}
}

func decodeUTF16(raw []byte) []byte {
	bigEndian := false
	switch {
	case len(raw) >= 2 && raw[0] == 0xFE && raw[1] == 0xFF:
		bigEndian, raw = true, raw[2:]
	case len(raw) >= 2 && raw[0] == 0xFF && raw[1] == 0xFE:
		raw = raw[2:]
	case len(raw) >= 2 && raw[0] == 0 && raw[1] != 0:
		bigEndian = true
	}
	u := make([]uint16, 0, len(raw)/2)
	for i := 0; i+1 < len(raw); i += 2 {
		if bigEndian {
			u = append(u, uint16(raw[i])<<8|uint16(raw[i+1]))
		} else {
			u = append(u, uint16(raw[i+1])<<8|uint16(raw[i]))
		}
	}
	return []byte(string(utf16.Decode(u)))
}

type parser struct {
	src []byte
	dec *xml.Decoder
}

// nextStart skips to the next StartElement at the current level, returning
// io.EOF at the enclosing EndElement.
func (p *parser) nextStart() (xml.StartElement, error) {
	for {
		tok, err := p.dec.Token()
		if err != nil {
			return xml.StartElement{}, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			return t, nil
		case xml.EndElement:
			return xml.StartElement{}, io.EOF
		case xml.CharData:
			if strings.TrimSpace(string(t)) != "" {
				return xml.StartElement{}, &SyntaxError{Element: "", Msg: "unexpected text " + strconv.Quote(strings.TrimSpace(string(t)))}
			}
		}
	}
}

// text reads the character content of an element whose start was just
// consumed, up to its end tag, and rejects child elements.
func (p *parser) text(name string) (string, error) {
	var sb strings.Builder
	for {
		tok, err := p.dec.Token()
		if err != nil {
			return "", err
		}
		switch t := tok.(type) {
		case xml.CharData:
			sb.Write(t)
		case xml.EndElement:
			return sb.String(), nil
		case xml.StartElement:
			return "", &SyntaxError{Element: name, Msg: "unexpected child element " + t.Name.Local}
		}
	}
}

func (p *parser) trimmedText(name string) (string, error) {
	s, err := p.text(name)
	return strings.TrimSpace(s), err
}

func (p *parser) skipEmpty(name string) error {
	s, err := p.text(name)
	if err != nil {
		return err
	}
	if strings.TrimSpace(s) != "" {
		return &SyntaxError{Element: name, Msg: "must be empty"}
	}
	return nil
}

func (p *parser) message() (*Message, error) {
	root, err := p.nextStart()
	if err != nil {
		return nil, &SyntaxError{Element: "SyncML", Msg: "no root element", Err: err}
	}
	if root.Name.Local != "SyncML" {
		return nil, &SyntaxError{Element: root.Name.Local, Msg: "root element must be SyncML"}
	}
	m := &Message{}
	for _, a := range root.Attr {
		if a.Name.Local == "xmlns" && a.Name.Space == "" {
			m.Namespace = a.Value
		}
	}
	switch m.Namespace {
	case "", NamespaceSyncML12, NamespaceSyncML11:
	default:
		return nil, fmt.Errorf("%w: %q", ErrNamespace, m.Namespace)
	}
	seenHdr, seenBody := false, false
	for {
		el, err := p.nextStart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		switch el.Name.Local {
		case "SyncHdr":
			if seenHdr {
				return nil, &SyntaxError{Element: "SyncHdr", Msg: "repeated"}
			}
			seenHdr = true
			if err := p.header(&m.Header); err != nil {
				return nil, err
			}
		case "SyncBody":
			if seenBody {
				return nil, &SyntaxError{Element: "SyncBody", Msg: "repeated"}
			}
			seenBody = true
			for _, a := range el.Attr {
				if a.Name.Space == "xmlns" && a.Name.Local == "msft" || a.Name.Local == "msft" && a.Value == NamespaceMSFT {
					m.Body.MSFTNamespace = true
				}
			}
			if err := p.body(&m.Body); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("%w: %s under SyncML", ErrUnknownElement, el.Name.Local)
		}
	}
	// Anything after the root element other than whitespace is an error.
	for {
		tok, err := p.dec.Token()
		if errors.Is(err, io.EOF) {
			return m, nil
		}
		if err != nil {
			return nil, err
		}
		if cd, ok := tok.(xml.CharData); ok && strings.TrimSpace(string(cd)) == "" {
			continue
		}
		return nil, &SyntaxError{Element: "SyncML", Msg: "content after the root element"}
	}
}

func (p *parser) header(h *Header) error {
	for {
		el, err := p.nextStart()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		var v string
		switch el.Name.Local {
		case "VerDTD":
			h.VerDTD, err = p.trimmedText("VerDTD")
		case "VerProto":
			h.VerProto, err = p.trimmedText("VerProto")
		case "SessionID":
			h.SessionID, err = p.trimmedText("SessionID")
		case "MsgID":
			h.MsgID, err = p.trimmedText("MsgID")
		case "Target":
			h.Target, err = p.location("Target")
		case "Source":
			h.Source, err = p.location("Source")
		case "RespURI":
			h.RespURI, err = p.trimmedText("RespURI")
		case "NoResp":
			h.NoResp, err = true, p.skipEmpty("NoResp")
		case "Cred":
			h.Cred, err = p.cred()
		case "Meta":
			h.Meta, err = p.meta()
		default:
			return fmt.Errorf("%w: %s under SyncHdr", ErrUnknownElement, el.Name.Local)
		}
		_ = v
		if err != nil {
			return err
		}
	}
}

func (p *parser) location(name string) (Location, error) {
	var l Location
	for {
		el, err := p.nextStart()
		if errors.Is(err, io.EOF) {
			return l, nil
		}
		if err != nil {
			return l, err
		}
		switch el.Name.Local {
		case "LocURI":
			l.LocURI, err = p.trimmedText("LocURI")
		case "LocName":
			l.LocName, err = p.trimmedText("LocName")
		default:
			return l, fmt.Errorf("%w: %s under %s", ErrUnknownElement, el.Name.Local, name)
		}
		if err != nil {
			return l, err
		}
	}
}

// locURI reads <Target> or <Source> inside an Item, which carry a LocURI only.
func (p *parser) locURI(name string) (string, error) {
	l, err := p.location(name)
	return l.LocURI, err
}

func (p *parser) cred() (*Cred, error) {
	c := &Cred{}
	for {
		el, err := p.nextStart()
		if errors.Is(err, io.EOF) {
			return c, nil
		}
		if err != nil {
			return nil, err
		}
		switch el.Name.Local {
		case "Meta":
			var m *Meta
			m, err = p.meta()
			if m != nil {
				c.Meta = *m
			}
		case "Data":
			c.Data, err = p.trimmedText("Data")
		default:
			return nil, fmt.Errorf("%w: %s under Cred", ErrUnknownElement, el.Name.Local)
		}
		if err != nil {
			return nil, err
		}
	}
}

func (p *parser) chal() (*Chal, error) {
	c := &Chal{}
	for {
		el, err := p.nextStart()
		if errors.Is(err, io.EOF) {
			return c, nil
		}
		if err != nil {
			return nil, err
		}
		if el.Name.Local != "Meta" {
			return nil, fmt.Errorf("%w: %s under Chal", ErrUnknownElement, el.Name.Local)
		}
		m, err := p.meta()
		if err != nil {
			return nil, err
		}
		c.Meta = *m
	}
}

// meta reads the metinf children. Windows sends Format, Type, NextNonce and
// MaxMsgSize; generic alerts add Mark; large objects add Size and MaxObjSize.
// Elements outside that set are skipped, because the MetaInfo DTD allows
// several more (Anchor, Version, EMI, Mem) that a client might one day send
// and none of them changes how a message is handled.
func (p *parser) meta() (*Meta, error) {
	m := &Meta{}
	for {
		el, err := p.nextStart()
		if errors.Is(err, io.EOF) {
			return m, nil
		}
		if err != nil {
			return nil, err
		}
		var s string
		switch el.Name.Local {
		case "Format":
			m.Format, err = p.trimmedText("Format")
		case "Type":
			m.Type, err = p.trimmedText("Type")
		case "Mark":
			m.Mark, err = p.trimmedText("Mark")
		case "NextNonce":
			m.NextNonce, err = p.trimmedText("NextNonce")
		case "Size":
			s, err = p.trimmedText("Size")
			if err == nil {
				m.Size, err = parseInt("Meta/Size", s)
			}
		case "MaxMsgSize":
			s, err = p.trimmedText("MaxMsgSize")
			if err == nil {
				m.MaxMsgSize, err = parseInt("Meta/MaxMsgSize", s)
			}
		case "MaxObjSize":
			s, err = p.trimmedText("MaxObjSize")
			if err == nil {
				m.MaxObjSize, err = parseInt("Meta/MaxObjSize", s)
			}
		default:
			err = p.dec.Skip()
		}
		if err != nil {
			return nil, err
		}
	}
}

func parseInt(el, s string) (int64, error) {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n < 0 {
		return 0, &SyntaxError{Element: el, Msg: "not a non-negative integer: " + strconv.Quote(s)}
	}
	return n, nil
}

// data reads a Data element whose start is el. Markup inside is captured
// verbatim from the source bytes; character data (including CDATA) is
// normalised to a string.
func (p *parser) data(el xml.StartElement) (*Data, error) {
	d := &Data{}
	for _, a := range el.Attr {
		if a.Name.Local == AttrOriginalError {
			d.OriginalError = a.Value
		}
	}
	start := p.dec.InputOffset()
	var text strings.Builder
	markup := false
	depth := 0
	prev := start
	for {
		tok, err := p.dec.Token()
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			markup = true
			depth++
		case xml.EndElement:
			if depth == 0 {
				if markup {
					d.XML = strings.TrimSpace(string(p.src[start:prev]))
					if !utf8.ValidString(d.XML) {
						return nil, &SyntaxError{Element: "Data", Msg: "markup is not UTF-8 in the source; re-encode the document as UTF-8"}
					}
				} else {
					d.Value = text.String()
				}
				return d, nil
			}
			depth--
		case xml.CharData:
			if !markup {
				text.Write(t)
			}
		}
		prev = p.dec.InputOffset()
	}
}

func (p *parser) item() (Item, error) {
	var it Item
	for {
		el, err := p.nextStart()
		if errors.Is(err, io.EOF) {
			return it, nil
		}
		if err != nil {
			return it, err
		}
		switch el.Name.Local {
		case "Target":
			it.Target, err = p.locURI("Target")
		case "Source":
			it.Source, err = p.locURI("Source")
		case "Meta":
			it.Meta, err = p.meta()
		case "Data":
			it.Data, err = p.data(el)
		case "MoreData":
			it.MoreData, err = true, p.skipEmpty("MoreData")
		case "TargetParent", "SourceParent":
			return it, fmt.Errorf("%w: %s is not used by Windows", ErrUnknownElement, el.Name.Local)
		default:
			return it, fmt.Errorf("%w: %s under Item", ErrUnknownElement, el.Name.Local)
		}
		if err != nil {
			return it, err
		}
	}
}

func (p *parser) body(b *Body) error {
	for {
		el, err := p.nextStart()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if b.Final {
			return fmt.Errorf("%w: found %s after Final", ErrFinalNotLast, el.Name.Local)
		}
		if el.Name.Local == "Final" {
			if err := p.skipEmpty("Final"); err != nil {
				return err
			}
			b.Final = true
			continue
		}
		c, err := p.command(el, "SyncBody")
		if err != nil {
			return err
		}
		b.Commands = append(b.Commands, c)
	}
}

var unsupportedCommands = map[string]bool{"Copy": true, "Map": true, "Move": true, "Put": true, "Search": true, "Sync": true}

func (p *parser) command(el xml.StartElement, parent string) (Command, error) {
	name := el.Name.Local
	if unsupportedCommands[name] {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedCommand, name)
	}
	switch name {
	case CmdAdd:
		c := &Add{}
		err := p.itemCommand(name, &c.CmdID, &c.NoResp, nil, &c.Cred, &c.Meta, &c.Items)
		return c, err
	case CmdReplace:
		c := &Replace{}
		err := p.itemCommand(name, &c.CmdID, &c.NoResp, nil, &c.Cred, &c.Meta, &c.Items)
		return c, err
	case CmdDelete:
		c := &Delete{}
		err := p.itemCommand(name, &c.CmdID, &c.NoResp, nil, &c.Cred, &c.Meta, &c.Items)
		return c, err
	case CmdGet:
		c := &Get{}
		err := p.itemCommand(name, &c.CmdID, &c.NoResp, &c.Lang, &c.Cred, &c.Meta, &c.Items)
		return c, err
	case CmdExec:
		return p.exec()
	case CmdAlert:
		return p.alert()
	case CmdAtomic:
		c := &Atomic{}
		err := p.group(name, &c.CmdID, &c.NoResp, &c.Meta, &c.Commands)
		return c, err
	case CmdSequence:
		c := &Sequence{}
		err := p.group(name, &c.CmdID, &c.NoResp, &c.Meta, &c.Commands)
		return c, err
	case CmdStatus:
		return p.status()
	case CmdResults:
		return p.results()
	default:
		return nil, fmt.Errorf("%w: %s under %s", ErrUnknownElement, name, parent)
	}
}

// itemCommand reads Add, Replace, Delete and Get. lang is non-nil for Get.
func (p *parser) itemCommand(name string, cmdID *string, noResp *bool, lang *string, cred **Cred, meta **Meta, items *[]Item) error {
	for {
		el, err := p.nextStart()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		switch el.Name.Local {
		case "CmdID":
			*cmdID, err = p.trimmedText("CmdID")
		case "NoResp":
			*noResp, err = true, p.skipEmpty("NoResp")
		case "Lang":
			if lang == nil {
				return fmt.Errorf("%w: Lang under %s", ErrUnknownElement, name)
			}
			*lang, err = p.trimmedText("Lang")
		case "Cred":
			*cred, err = p.cred()
		case "Meta":
			*meta, err = p.meta()
		case "Item":
			var it Item
			it, err = p.item()
			*items = append(*items, it)
		default:
			return fmt.Errorf("%w: %s under %s", ErrUnknownElement, el.Name.Local, name)
		}
		if err != nil {
			return err
		}
	}
}

func (p *parser) exec() (*Exec, error) {
	c := &Exec{}
	for {
		el, err := p.nextStart()
		if errors.Is(err, io.EOF) {
			return c, nil
		}
		if err != nil {
			return nil, err
		}
		switch el.Name.Local {
		case "CmdID":
			c.CmdID, err = p.trimmedText("CmdID")
		case "NoResp":
			c.NoResp, err = true, p.skipEmpty("NoResp")
		case "Cred":
			c.Cred, err = p.cred()
		case "Meta":
			c.Meta, err = p.meta()
		case "Correlator":
			c.Correlator, err = p.trimmedText("Correlator")
		case "Item":
			var it Item
			it, err = p.item()
			c.Items = append(c.Items, it)
		default:
			return nil, fmt.Errorf("%w: %s under Exec", ErrUnknownElement, el.Name.Local)
		}
		if err != nil {
			return nil, err
		}
	}
}

func (p *parser) alert() (*Alert, error) {
	c := &Alert{}
	for {
		el, err := p.nextStart()
		if errors.Is(err, io.EOF) {
			return c, nil
		}
		if err != nil {
			return nil, err
		}
		switch el.Name.Local {
		case "CmdID":
			c.CmdID, err = p.trimmedText("CmdID")
		case "NoResp":
			c.NoResp, err = true, p.skipEmpty("NoResp")
		case "Cred":
			c.Cred, err = p.cred()
		case "Data":
			c.Data, err = p.trimmedText("Data")
		case "Correlator":
			c.Correlator, err = p.trimmedText("Correlator")
		case "Item":
			var it Item
			it, err = p.item()
			c.Items = append(c.Items, it)
		default:
			return nil, fmt.Errorf("%w: %s under Alert", ErrUnknownElement, el.Name.Local)
		}
		if err != nil {
			return nil, err
		}
	}
}

func (p *parser) group(name string, cmdID *string, noResp *bool, meta **Meta, cmds *[]Command) error {
	for {
		el, err := p.nextStart()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		switch el.Name.Local {
		case "CmdID":
			*cmdID, err = p.trimmedText("CmdID")
		case "NoResp":
			*noResp, err = true, p.skipEmpty("NoResp")
		case "Meta":
			*meta, err = p.meta()
		case CmdAdd, CmdReplace, CmdDelete, CmdGet, CmdExec, CmdAlert, CmdAtomic, CmdSequence,
			"Copy", "Map", "Move", "Put", "Search", "Sync", CmdStatus, CmdResults:
			if el.Name.Local == CmdStatus || el.Name.Local == CmdResults {
				return fmt.Errorf("%w: %s under %s", ErrUnknownElement, el.Name.Local, name)
			}
			var c Command
			c, err = p.command(el, name)
			if err == nil {
				*cmds = append(*cmds, c)
			}
		default:
			return fmt.Errorf("%w: %s under %s", ErrUnknownElement, el.Name.Local, name)
		}
		if err != nil {
			return err
		}
	}
}

func (p *parser) status() (*Status, error) {
	c := &Status{}
	for {
		el, err := p.nextStart()
		if errors.Is(err, io.EOF) {
			return c, nil
		}
		if err != nil {
			return nil, err
		}
		var s string
		switch el.Name.Local {
		case "CmdID":
			c.CmdID, err = p.trimmedText("CmdID")
		case "MsgRef":
			c.MsgRef, err = p.trimmedText("MsgRef")
		case "CmdRef":
			c.CmdRef, err = p.trimmedText("CmdRef")
		case "Cmd":
			c.Cmd, err = p.trimmedText("Cmd")
		case "TargetRef":
			s, err = p.trimmedText("TargetRef")
			c.TargetRef = append(c.TargetRef, s)
		case "SourceRef":
			s, err = p.trimmedText("SourceRef")
			c.SourceRef = append(c.SourceRef, s)
		case "Cred":
			c.Cred, err = p.cred()
		case "Chal":
			c.Chal, err = p.chal()
		case "Data":
			var d *Data
			d, err = p.data(el)
			if d != nil {
				d.Value = strings.TrimSpace(d.Value)
				c.Data = *d
			}
		case "Item":
			var it Item
			it, err = p.item()
			c.Items = append(c.Items, it)
		default:
			return nil, fmt.Errorf("%w: %s under Status", ErrUnknownElement, el.Name.Local)
		}
		if err != nil {
			return nil, err
		}
	}
}

func (p *parser) results() (*Results, error) {
	c := &Results{}
	for {
		el, err := p.nextStart()
		if errors.Is(err, io.EOF) {
			return c, nil
		}
		if err != nil {
			return nil, err
		}
		switch el.Name.Local {
		case "CmdID":
			c.CmdID, err = p.trimmedText("CmdID")
		case "MsgRef":
			c.MsgRef, err = p.trimmedText("MsgRef")
		case "CmdRef":
			c.CmdRef, err = p.trimmedText("CmdRef")
		case "Cmd":
			c.Cmd, err = p.trimmedText("Cmd")
		case "Meta":
			c.Meta, err = p.meta()
		case "TargetRef":
			c.TargetRef, err = p.trimmedText("TargetRef")
		case "SourceRef":
			c.SourceRef, err = p.trimmedText("SourceRef")
		case "Item":
			var it Item
			it, err = p.item()
			c.Items = append(c.Items, it)
		default:
			return nil, fmt.Errorf("%w: %s under Results", ErrUnknownElement, el.Name.Local)
		}
		if err != nil {
			return nil, err
		}
	}
}
