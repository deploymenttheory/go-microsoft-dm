package syncml

// Message is one SyncML document: a SyncHdr and a SyncBody.
type Message struct {
	// Namespace is the xmlns of the root as decoded: NamespaceSyncML12,
	// NamespaceSyncML11 or "" for a fragment without one. Encode always
	// writes NamespaceSyncML12 unless the field is NamespaceSyncML11.
	Namespace string
	Header    Header
	Body      Body
}

// Header is the SyncHdr element. Element order on the wire follows the DTD:
// VerDTD, VerProto, SessionID, MsgID, Target, Source, RespURI?, NoResp?,
// Cred?, Meta?.
type Header struct {
	VerDTD    string
	VerProto  string
	SessionID string
	MsgID     string
	Target    Location
	Source    Location
	RespURI   string
	NoResp    bool
	Cred      *Cred
	Meta      *Meta
}

// Location is a Target or Source in the header: a LocURI with an optional
// LocName (the user id for MD5 authentication).
type Location struct {
	LocURI  string
	LocName string
}

// Body is the SyncBody element: commands in document order and the Final flag.
type Body struct {
	Commands []Command
	// Final marks the last message of a package. Decode rejects a Final that
	// is not the last element.
	Final bool
	// MSFTNamespace declares xmlns:msft on SyncBody, which MS-MDM 2.2.4.3
	// says the server SHOULD do once SyncApplicationVersion is 3.0 or higher.
	MSFTNamespace bool
}

// Command is one of the ten SyncML commands Windows exchanges. The interface
// is sealed; the implementations are Add, Alert, Atomic, Delete, Exec, Get,
// Replace, Results, Sequence and Status.
type Command interface {
	// Name is the element name on the wire.
	Name() string
	// ID is the CmdID.
	ID() string
	command() sealed
}

// sealed is the marker type that keeps Command closed to this package.
type sealed struct{}

// cmd is embedded by every command so only this package can satisfy Command.
type cmd struct{}

func (cmd) command() sealed { return sealed{} }

// Item is the operand of a command, or the extra information on a Status.
// Target and Source are LocURIs; the DTD's TargetParent and SourceParent are
// not used by Windows and are rejected on decode.
type Item struct {
	Target   string
	Source   string
	Meta     *Meta
	Data     *Data
	MoreData bool
}

// Data is the content of an Item or the code of a Status or Alert. Exactly
// one of Value and XML is set: Value is character data that Encode escapes;
// XML is markup carried verbatim, which MS-MDM 2.2.5.1 permits when every
// element in it declares its own namespace (an MsiInstallJob, a WinDC
// DeclaredConfigurations document, a DiagnosticLog Collection).
type Data struct {
	Value string
	XML   string
	// OriginalError is the msft:originalerror attribute a Windows client adds
	// to a failing Status once SyncApplicationVersion is 3.0 or higher.
	OriginalError string
}

// Text returns Value, or XML when the data is markup.
func (d *Data) Text() string {
	if d == nil {
		return ""
	}
	if d.XML != "" {
		return d.XML
	}
	return d.Value
}

// Meta carries the syncml:metinf children Windows uses. Size, MaxMsgSize and
// MaxObjSize are zero when absent.
type Meta struct {
	Format     string
	Type       string
	Mark       string
	Size       int64
	NextNonce  string
	MaxMsgSize int64
	MaxObjSize int64
}

// Cred is the authentication credential in a SyncHdr (OMA DM Security 5.3).
type Cred struct {
	Meta Meta
	Data string
}

// Chal is the authentication challenge inside a Status; its Meta carries the
// Type, Format and NextNonce of the challenge.
type Chal struct {
	Meta Meta
}

// Add adds nodes; Windows also performs an implicit Add on Replace.
type Add struct {
	cmd
	CmdID  string
	NoResp bool
	Cred   *Cred
	Meta   *Meta
	Items  []Item
}

// Replace writes leaf values.
type Replace struct {
	cmd
	CmdID  string
	NoResp bool
	Cred   *Cred
	Meta   *Meta
	Items  []Item
}

// Delete removes a node and its subtree.
type Delete struct {
	cmd
	CmdID  string
	NoResp bool
	Cred   *Cred
	Meta   *Meta
	Items  []Item
}

// Get reads a leaf, or the URI-encoded child names of an interior node.
type Get struct {
	cmd
	CmdID  string
	NoResp bool
	Lang   string
	Cred   *Cred
	Meta   *Meta
	Items  []Item
}

// Exec invokes an executable node. The DTD allows exactly one Item;
// Validate enforces it. Correlator is unsupported by MS-MDM and is kept only
// so a decoded value survives a round trip.
type Exec struct {
	cmd
	CmdID      string
	NoResp     bool
	Cred       *Cred
	Meta       *Meta
	Correlator string
	Items      []Item
}

// Alert carries a session type (1200, 1201), flow control (1222, 1223,
// 1225) or a client event (1224) or generic alert (1226) with Items.
type Alert struct {
	cmd
	CmdID      string
	NoResp     bool
	Cred       *Cred
	Data       string
	Correlator string
	Items      []Item
}

// Code parses Data as an AlertCode; an unparsable Data yields 0.
func (a *Alert) Code() AlertCode {
	c, err := ParseAlertCode(a.Data)
	if err != nil {
		return 0
	}
	return c
}

// Atomic executes its commands as a set or not at all.
type Atomic struct {
	cmd
	CmdID    string
	NoResp   bool
	Meta     *Meta
	Commands []Command
}

// Sequence executes its commands in order.
type Sequence struct {
	cmd
	CmdID    string
	NoResp   bool
	Meta     *Meta
	Commands []Command
}

// Status answers one command (or the SyncHdr, with CmdRef "0" and Cmd
// "SyncHdr"). Element order: CmdID, MsgRef, CmdRef, Cmd, TargetRef*,
// SourceRef*, Cred?, Chal?, Data, Item*.
type Status struct {
	cmd
	CmdID     string
	MsgRef    string
	CmdRef    string
	Cmd       string
	TargetRef []string
	SourceRef []string
	Cred      *Cred
	Chal      *Chal
	Data      Data
	Items     []Item
}

// Code parses Data as a StatusCode; an unparsable Data yields 0.
func (s *Status) Code() StatusCode {
	c, err := ParseStatusCode(s.Data.Value)
	if err != nil {
		return 0
	}
	return c
}

// Results returns the data of a successful Get. MS-MDM adds Cmd after
// CmdRef; a missing MsgRef is processed as "1".
type Results struct {
	cmd
	CmdID     string
	MsgRef    string
	CmdRef    string
	Cmd       string
	Meta      *Meta
	TargetRef string
	SourceRef string
	Items     []Item
}

func (c *Add) Name() string      { return CmdAdd }
func (c *Alert) Name() string    { return CmdAlert }
func (c *Atomic) Name() string   { return CmdAtomic }
func (c *Delete) Name() string   { return CmdDelete }
func (c *Exec) Name() string     { return CmdExec }
func (c *Get) Name() string      { return CmdGet }
func (c *Replace) Name() string  { return CmdReplace }
func (c *Results) Name() string  { return CmdResults }
func (c *Sequence) Name() string { return CmdSequence }
func (c *Status) Name() string   { return CmdStatus }

func (c *Add) ID() string      { return c.CmdID }
func (c *Alert) ID() string    { return c.CmdID }
func (c *Atomic) ID() string   { return c.CmdID }
func (c *Delete) ID() string   { return c.CmdID }
func (c *Exec) ID() string     { return c.CmdID }
func (c *Get) ID() string      { return c.CmdID }
func (c *Replace) ID() string  { return c.CmdID }
func (c *Results) ID() string  { return c.CmdID }
func (c *Sequence) ID() string { return c.CmdID }
func (c *Status) ID() string   { return c.CmdID }

// Walk calls fn for every command in the body, descending into Atomic and
// Sequence, in document order. It stops when fn returns false.
func (b *Body) Walk(fn func(cmd Command, depth int) bool) {
	walk(b.Commands, 0, fn)
}

func walk(cmds []Command, depth int, fn func(Command, int) bool) bool {
	for _, c := range cmds {
		if !fn(c, depth) {
			return false
		}
		switch n := c.(type) {
		case *Atomic:
			if !walk(n.Commands, depth+1, fn) {
				return false
			}
		case *Sequence:
			if !walk(n.Commands, depth+1, fn) {
				return false
			}
		}
	}
	return true
}

// Alerts returns the top-level Alert commands, in order.
func (b *Body) Alerts() []*Alert {
	var out []*Alert
	for _, c := range b.Commands {
		if a, ok := c.(*Alert); ok {
			out = append(out, a)
		}
	}
	return out
}

// Statuses returns the top-level Status commands, in order.
func (b *Body) Statuses() []*Status {
	var out []*Status
	for _, c := range b.Commands {
		if s, ok := c.(*Status); ok {
			out = append(out, s)
		}
	}
	return out
}

// HasAlert reports whether a top-level Alert carries code.
func (b *Body) HasAlert(code AlertCode) bool {
	for _, a := range b.Alerts() {
		if a.Code() == code {
			return true
		}
	}
	return false
}

// LocURI returns the Item's address: Target, or Source when Target is empty.
func (it *Item) LocURI() string {
	if it.Target != "" {
		return it.Target
	}
	return it.Source
}
