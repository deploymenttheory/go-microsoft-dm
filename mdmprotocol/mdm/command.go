package mdm

import (
	"errors"
	"fmt"
	"strings"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/syncml"
	"github.com/deploymenttheory/go-microsoft-dm/schema/csp"
	"github.com/deploymenttheory/go-microsoft-dm/schema/csp/dmclient"
	"github.com/deploymenttheory/go-microsoft-dm/schema/validation"
)

// Errors returned by the command builders.
var (
	// ErrCommand reports a command the Windows client would refuse.
	ErrCommand = errors.New("mdm: invalid command")
	// ErrScope reports a scope conflict.
	ErrScope = errors.New("mdm: scope")
)

// Scope is the management tree a command targets.
type Scope string

// The two scopes. The Windows client applies a ./Device or ./Vendor URI to
// the device and a ./User URI to the signed-in user; user settings fail
// with 500 or 507 when no user is signed in (research pitfall "User-scope
// commands before sign-in").
const (
	ScopeDevice Scope = "device"
	ScopeUser   Scope = "user"
)

// ScopeOf reports the scope a LocURI addresses: user for ./User/..., device
// for everything else, which is what the client assumes when the prefix is
// absent.
func ScopeOf(uri string) Scope {
	u := strings.TrimSpace(uri)
	if len(u) >= 7 && strings.EqualFold(u[:7], "./User/") || strings.EqualFold(u, "./User") {
		return ScopeUser
	}
	return ScopeDevice
}

// Command is one management operation to deliver to a device.
type Command struct {
	// ID is the caller's identifier, unique per device; the engine maps the
	// wire CmdIDs it assigns back to it. Empty means the queue assigns one.
	ID string
	// Scope is derived from the LocURIs by the builders.
	Scope Scope
	// Body is the SyncML command. Its CmdID fields are assigned at send time.
	Body syncml.Command
	// Internal marks commands the engine enqueues on its own (the first
	// session reads of DevDetail and the per-session ChannelURI read) so
	// callers can filter them out of listings.
	Internal bool
}

// Option tunes a builder.
type Option func(*options)

type options struct {
	format string
	typ    string
	xml    bool
	id     string
}

// WithFormat sets Meta/Format on the item (chr, int, bool, b64, xml, node...).
func WithFormat(f string) Option { return func(o *options) { o.format = f } }

// WithType sets Meta/Type on the item.
func WithType(t string) Option { return func(o *options) { o.typ = t } }

// WithXML marks the value as markup written verbatim (MS-MDM 2.2.5.1).
func WithXML() Option { return func(o *options) { o.xml = true } }

// WithID sets the command's ID.
func WithID(id string) Option { return func(o *options) { o.id = id } }

func build(opts []Option) options {
	var o options
	for _, f := range opts {
		f(&o)
	}
	return o
}

func item(uri string, value *string, o options) (syncml.Item, error) {
	if err := syncml.CheckLocURI(uri); err != nil {
		return syncml.Item{}, fmt.Errorf("%w: %w", ErrCommand, err)
	}
	it := syncml.Item{Target: uri}
	if o.format != "" || o.typ != "" {
		it.Meta = &syncml.Meta{Format: o.format, Type: o.typ}
	}
	if value != nil {
		if o.xml {
			it.Data = &syncml.Data{XML: *value}
		} else {
			it.Data = &syncml.Data{Value: *value}
		}
	}
	return it, nil
}

// NewAdd builds an Add of a leaf value. An interior node is added with
// WithFormat("node") and an empty value.
func NewAdd(uri, value string, opts ...Option) (*Command, error) {
	o := build(opts)
	it, err := item(uri, &value, o)
	if err != nil {
		return nil, err
	}
	return &Command{ID: o.id, Scope: ScopeOf(uri), Body: &syncml.Add{Items: []syncml.Item{it}}}, nil
}

// NewReplace builds a Replace of a leaf value.
func NewReplace(uri, value string, opts ...Option) (*Command, error) {
	o := build(opts)
	it, err := item(uri, &value, o)
	if err != nil {
		return nil, err
	}
	return &Command{ID: o.id, Scope: ScopeOf(uri), Body: &syncml.Replace{Items: []syncml.Item{it}}}, nil
}

// NewDelete builds a Delete of a node and its subtree.
func NewDelete(uri string, opts ...Option) (*Command, error) {
	o := build(opts)
	it, err := item(uri, nil, o)
	if err != nil {
		return nil, err
	}
	return &Command{ID: o.id, Scope: ScopeOf(uri), Body: &syncml.Delete{Items: []syncml.Item{it}}}, nil
}

// NewGet builds a Get of one or more nodes.
func NewGet(uris []string, opts ...Option) (*Command, error) {
	if len(uris) == 0 {
		return nil, fmt.Errorf("%w: Get needs at least one URI", ErrCommand)
	}
	o := build(opts)
	items := make([]syncml.Item, 0, len(uris))
	scope := ScopeOf(uris[0])
	for _, u := range uris {
		it, err := item(u, nil, o)
		if err != nil {
			return nil, err
		}
		if ScopeOf(u) != scope {
			return nil, fmt.Errorf("%w: Get mixes device and user URIs", ErrScope)
		}
		items = append(items, it)
	}
	return &Command{ID: o.id, Scope: scope, Body: &syncml.Get{Items: items}}, nil
}

// NewExec builds an Exec; value is the parameter data, or "" for none.
func NewExec(uri, value string, opts ...Option) (*Command, error) {
	o := build(opts)
	var v *string
	if value != "" {
		v = &value
	}
	it, err := item(uri, v, o)
	if err != nil {
		return nil, err
	}
	return &Command{ID: o.id, Scope: ScopeOf(uri), Body: &syncml.Exec{Items: []syncml.Item{it}}}, nil
}

// NewAtomic groups commands that succeed or fail together. It refuses a
// nested Atomic (the client answers 500 and the parent 507), a Get (500),
// an Add followed by a Replace on the same node (unsupported), and a mix of
// device and user scope (a signed-out user rolls back the device settings).
func NewAtomic(cmds []*Command, opts ...Option) (*Command, error) {
	if len(cmds) == 0 {
		return nil, fmt.Errorf("%w: empty Atomic", ErrCommand)
	}
	if cmds[0] == nil || cmds[0].Body == nil {
		return nil, fmt.Errorf("%w: nil command in Atomic", ErrCommand)
	}
	o := build(opts)
	seen := map[string]string{}
	var body []syncml.Command
	scope := cmds[0].Scope
	for _, c := range cmds {
		if c == nil || c.Body == nil {
			return nil, fmt.Errorf("%w: nil command in Atomic", ErrCommand)
		}
		switch b := c.Body.(type) {
		case *syncml.Atomic:
			return nil, fmt.Errorf("%w: nested Atomic", ErrCommand)
		case *syncml.Get:
			return nil, fmt.Errorf("%w: Get inside Atomic", ErrCommand)
		case *syncml.Sequence:
			for _, inner := range b.Commands {
				if _, ok := inner.(*syncml.Get); ok {
					return nil, fmt.Errorf("%w: Get inside Atomic (through a Sequence)", ErrCommand)
				}
				if _, ok := inner.(*syncml.Atomic); ok {
					return nil, fmt.Errorf("%w: nested Atomic (through a Sequence)", ErrCommand)
				}
			}
		}
		if c.Scope != scope {
			return nil, fmt.Errorf("%w: Atomic mixes device and user commands", ErrScope)
		}
		for _, uri := range targets(c.Body) {
			prev := seen[uri]
			name := c.Body.Name()
			if prev == syncml.CmdAdd && name == syncml.CmdReplace {
				return nil, fmt.Errorf("%w: Add then Replace on %s inside Atomic", ErrCommand, uri)
			}
			seen[uri] = name
		}
		body = append(body, c.Body)
	}
	return &Command{ID: o.id, Scope: scope, Body: &syncml.Atomic{Commands: body}}, nil
}

// NewSequence groups commands that run in order. A Sequence may hold a Get;
// it refuses a nested Sequence or Atomic and mixed scopes.
func NewSequence(cmds []*Command, opts ...Option) (*Command, error) {
	if len(cmds) == 0 {
		return nil, fmt.Errorf("%w: empty Sequence", ErrCommand)
	}
	if cmds[0] == nil || cmds[0].Body == nil {
		return nil, fmt.Errorf("%w: nil command in Sequence", ErrCommand)
	}
	o := build(opts)
	var body []syncml.Command
	scope := cmds[0].Scope
	for _, c := range cmds {
		if c == nil || c.Body == nil {
			return nil, fmt.Errorf("%w: nil command in Sequence", ErrCommand)
		}
		switch c.Body.(type) {
		case *syncml.Atomic, *syncml.Sequence:
			return nil, fmt.Errorf("%w: %s inside Sequence", ErrCommand, c.Body.Name())
		}
		if c.Scope != scope {
			return nil, fmt.Errorf("%w: Sequence mixes device and user commands", ErrScope)
		}
		body = append(body, c.Body)
	}
	return &Command{ID: o.id, Scope: scope, Body: &syncml.Sequence{Commands: body}}, nil
}

// targets lists the Target LocURIs of a leaf command.
func targets(c syncml.Command) []string {
	var out []string
	for _, it := range itemsOf(c) {
		out = append(out, it.Target)
	}
	return out
}

// itemsOf returns the items of a leaf command, or nil for a group.
func itemsOf(c syncml.Command) []syncml.Item {
	switch b := c.(type) {
	case *syncml.Add:
		return b.Items
	case *syncml.Replace:
		return b.Items
	case *syncml.Delete:
		return b.Items
	case *syncml.Get:
		return b.Items
	case *syncml.Exec:
		return b.Items
	}
	return nil
}

// Leaves returns the leaf commands of a command in order: the command
// itself, or the members of an Atomic or Sequence.
func Leaves(c syncml.Command) []syncml.Command {
	switch b := c.(type) {
	case *syncml.Atomic:
		var out []syncml.Command
		for _, inner := range b.Commands {
			out = append(out, Leaves(inner)...)
		}
		return out
	case *syncml.Sequence:
		var out []syncml.Command
		for _, inner := range b.Commands {
			out = append(out, Leaves(inner)...)
		}
		return out
	}
	return []syncml.Command{c}
}

// Validate checks a command's structure the way the builders do, for
// commands constructed by hand.
func Validate(c *Command) error {
	if c == nil || c.Body == nil {
		return fmt.Errorf("%w: nil command", ErrCommand)
	}
	switch c.Body.(type) {
	case *syncml.Status, *syncml.Results, *syncml.Alert:
		return fmt.Errorf("%w: %s is not a server command", ErrCommand, c.Body.Name())
	}
	leaves := Leaves(c.Body)
	if len(leaves) == 0 {
		return fmt.Errorf("%w: empty %s", ErrCommand, c.Body.Name())
	}
	for _, leaf := range leaves {
		items := itemsOf(leaf)
		if len(items) == 0 {
			return fmt.Errorf("%w: %s without items", ErrCommand, leaf.Name())
		}
		if _, ok := leaf.(*syncml.Exec); ok && len(items) != 1 {
			return fmt.Errorf("%w: Exec needs exactly one item", ErrCommand)
		}
		for _, it := range items {
			if err := syncml.CheckLocURI(it.Target); err != nil {
				return fmt.Errorf("%w: %w", ErrCommand, err)
			}
			if ScopeOf(it.Target) != c.Scope {
				return fmt.Errorf("%w: %s is %s but the command is %s", ErrScope, it.Target, ScopeOf(it.Target), c.Scope)
			}
		}
	}
	if a, ok := c.Body.(*syncml.Atomic); ok {
		members := make([]*Command, 0, len(a.Commands))
		for _, inner := range a.Commands {
			members = append(members, &Command{Scope: c.Scope, Body: inner})
		}
		if _, err := NewAtomic(members); err != nil {
			return err
		}
	}
	return nil
}

// Check validates every leaf of the command against the generated schema
// and returns the joined problems, or nil. A nil registry checks nothing.
func Check(reg *csp.Registry, c *Command) error {
	if err := Validate(c); err != nil {
		return err
	}
	if reg == nil {
		return nil
	}
	var errs []error
	for _, leaf := range Leaves(c.Body) {
		verb := validation.Verb(leaf.Name())
		for _, it := range itemsOf(leaf) {
			vc := validation.Command{Verb: verb, URI: it.Target}
			if it.Meta != nil {
				vc.Format = it.Meta.Format
			}
			if it.Data != nil {
				v := it.Data.Text()
				vc.Value = &v
			}
			if err := validation.Check(reg, vc).Err(); err != nil {
				errs = append(errs, err)
			}
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%w: %w", ErrCommand, errors.Join(errs...))
	}
	return nil
}

// NewUnenroll builds the Exec that unenrolls one management provider
// (MS-MDM: Exec on DMClient/Provider/{ID}/Unenroll). The client ends the
// enrollment and, on a user-initiated unenroll, sends a 1226 alert which the
// session engine turns into an Unenrolled hook.
func NewUnenroll(providerID string) (*Command, error) {
	if providerID == "" {
		return nil, fmt.Errorf("%w: Unenroll needs a provider id", ErrCommand)
	}
	return NewExec(dmclient.DeviceProviderUnenroll(providerID), "")
}

// NewUnenrollDevice builds the Exec on the device-wide unenroll node
// (./Device/Vendor/MSFT/DMClient/Unenroll).
func NewUnenrollDevice() (*Command, error) {
	return NewExec(dmclient.DeviceUnenroll, "")
}
