package csp

import (
	"net/url"
	"strings"
)

// Scope is the tree a root lives in, from the leading segment of its Path.
type Scope string

const (
	// ScopeDevice is ./Device/... .
	ScopeDevice Scope = "Device"
	// ScopeUser is ./User/... .
	ScopeUser Scope = "User"
	// ScopeLegacy is a root without a Device or User prefix (./Vendor/MSFT/...,
	// ./DevInfo, ./SyncML/DMAcc); Windows treats it as device scope.
	ScopeLegacy Scope = "Legacy"
)

// Format is the DFFormat of a node.
type Format string

const (
	FormatNode  Format = "node"
	FormatChr   Format = "chr"
	FormatInt   Format = "int"
	FormatBool  Format = "bool"
	FormatB64   Format = "b64"
	FormatNull  Format = "null"
	FormatXML   Format = "xml"
	FormatBin   Format = "bin"
	FormatTime  Format = "time"
	FormatDate  Format = "date"
	FormatFloat Format = "float"
)

// Access is one AccessType verb.
type Access string

const (
	AccessAdd     Access = "Add"
	AccessDelete  Access = "Delete"
	AccessExec    Access = "Exec"
	AccessGet     Access = "Get"
	AccessReplace Access = "Replace"
)

// NamingKind is how a dynamic node's name is chosen (MSFT:DynamicNodeNaming).
type NamingKind string

const (
	// NamingUnspecified is a dynamic node without MSFT:DynamicNodeNaming.
	NamingUnspecified NamingKind = ""
	// NamingUniqueName is a server-chosen name that must match Pattern.
	NamingUniqueName NamingKind = "UniqueName"
	// NamingClientInventory is a name the client creates and the server reads.
	NamingClientInventory NamingKind = "ClientInventory"
	// NamingServerGeneratedUniqueIdentifier is a server-chosen unique id.
	NamingServerGeneratedUniqueIdentifier NamingKind = "ServerGeneratedUniqueIdentifier"
)

// ValueType is the MSFT:AllowedValues ValueType attribute.
type ValueType string

const (
	ValueTypeNone  ValueType = "None"
	ValueTypeEnum  ValueType = "ENUM"
	ValueTypeFlag  ValueType = "Flag"
	ValueTypeRange ValueType = "Range"
	ValueTypeRegEx ValueType = "RegEx"
	ValueTypeXSD   ValueType = "XSD"
	ValueTypeADMX  ValueType = "ADMX"
	ValueTypeSDDL  ValueType = "SDDL"
	ValueTypeJSON  ValueType = "JSON"
)

// Tree is one root of one DDF file and everything under it.
type Tree struct {
	// Name is the CSP name (DMClient) or Policy area name (BITS, ADMX_AppCompat).
	Name string
	// File is the DDF file the tree came from.
	File string
	// Scope is the root's scope; policy areas and 87 CSP files have one tree per scope.
	Scope Scope
	// PolicyArea is true for a Policy/Config area tree.
	PolicyArea bool
	// Root is the root node; Root.URI is the full path of the root.
	Root *Node
}

// Node is one node of a CSP tree.
type Node struct {
	// Name is the node name, or "" for a dynamic node.
	Name string
	// Title is the DFTitle; for a dynamic node it names the placeholder.
	Title string
	// URI is the full OMA-URI with dynamic segments as {Title}.
	URI string
	// Format is the DFFormat; FormatNode for interior nodes.
	Format Format
	// Access lists the permitted verbs, sorted.
	Access []Access
	// Description is the DDF Description, verbatim.
	Description string
	// Default is the DefaultValue, or "".
	Default string
	// Occurrence is One, ZeroOrOne, ZeroOrMore, OneOrMore, ZeroOrN or OneOrN.
	Occurrence string
	// Permanent is true for Scope/Permanent, false for Scope/Dynamic.
	Permanent bool
	// MIME is the DFType MIME content, or "" (DDFName types report "").
	MIME string
	// CaseSensitive is true for CaseSense/CS; false for CIS or absent.
	CaseSensitive bool
	// Dynamic is true when Name is empty.
	Dynamic bool
	// Naming describes how a dynamic node is named.
	Naming Naming
	// Applicability is the effective applicability: the node's own
	// MSFT:Applicability, or the nearest ancestor's when it has none.
	Applicability *Applicability
	// OwnApplicability is true when Applicability came from this node.
	OwnApplicability bool
	// AllowedValues is the MSFT:AllowedValues, or nil.
	AllowedValues *AllowedValues
	// GpMapping is the Group Policy mapping, or nil.
	GpMapping *GpMapping
	// ConflictResolution is LastWrite, HighestValueMostSecure, ... or "".
	ConflictResolution string
	// ReplaceBehavior is Append or Replace, or "".
	ReplaceBehavior string
	// RebootBehavior is Automatic or ServerInitiated, or "".
	RebootBehavior string
	// AtomicRequired means writes below this node must be in one Atomic.
	AtomicRequired bool
	// Deprecated is set when the node carries MSFT:Deprecated.
	Deprecated *Deprecated
	// Dependencies lists the MSFT:DependencyBehavior groups.
	Dependencies []DependencyGroup
	// Children in document order.
	Children []*Node
}

// Naming is MSFT:DynamicNodeNaming.
type Naming struct {
	Kind NamingKind
	// Pattern is the UniqueName regular expression, when Kind is NamingUniqueName.
	Pattern string
}

// Applicability is MSFT:Applicability.
type Applicability struct {
	// OsBuildVersions is the comma-separated list split and trimmed; the first
	// entry is the major release, later ones are backports with revisions.
	OsBuildVersions []string
	CspVersion      string
	// EditionAllowList holds the hex edition ids as written (0x4, 0x1B, ...).
	EditionAllowList []string
	RequiresAzureAd  bool
}

// AllowedValues is MSFT:AllowedValues.
type AllowedValues struct {
	Type ValueType
	// Value is the single MSFT:Value for Range, RegEx, XSD and JSON.
	Value string
	// Enum lists the ENUM and Flag entries in order.
	Enum []EnumValue
	// ADMX is the AdmxBacked link for ValueTypeADMX.
	ADMX *ADMX
	// ListDelimiter is set when the node takes a delimited list (MSFT:List).
	ListDelimiter string
}

// EnumValue is one MSFT:Enum entry.
type EnumValue struct {
	Value       string
	Description string
}

// ADMX is MSFT:AdmxBacked.
type ADMX struct {
	Area string
	Name string
	File string
}

// GpMapping is MSFT:GpMapping.
type GpMapping struct {
	EnglishName string
	AreaPath    string
	Element     string
}

// Deprecated is MSFT:Deprecated.
type Deprecated struct {
	// OsBuildDeprecated is the build the node was deprecated in, or "".
	OsBuildDeprecated string
}

// DependencyGroup is one MSFT:DependencyGroup: when every Dependency holds,
// ChangedAllowedValues (if set) replaces the node's AllowedValues.
type DependencyGroup struct {
	FriendlyID           string
	ChangedAllowedValues *AllowedValues
	Dependencies         []Dependency
}

// Dependency is one MSFT:Dependency.
type Dependency struct {
	Type      string
	URI       string
	ValueType ValueType
	Value     string
	Enum      []EnumValue
}

// Leaf reports whether the node holds a value or is an Exec target.
func (n *Node) Leaf() bool { return n.Format != FormatNode }

// Allows reports whether the verb is in Access.
func (n *Node) Allows(a Access) bool {
	for _, x := range n.Access {
		if x == a {
			return true
		}
	}
	return false
}

// Walk visits the node and its descendants in document order until fn
// returns false.
func (n *Node) Walk(fn func(*Node) bool) bool {
	if !fn(n) {
		return false
	}
	for _, c := range n.Children {
		if !c.Walk(fn) {
			return false
		}
	}
	return true
}

// Nodes returns every node of the tree in document order.
func (t *Tree) Nodes() []*Node {
	var out []*Node
	t.Root.Walk(func(n *Node) bool { out = append(out, n); return true })
	return out
}

// Match is the result of resolving a URI against a tree.
type Match struct {
	Node *Node
	// Params maps each dynamic segment's Title to the decoded value in the URI.
	Params map[string]string
}

// Lookup resolves a concrete OMA-URI against the tree. Segments are compared
// case-insensitively (the tree is case-insensitive unless a node says CS),
// dynamic nodes match any single segment, and a trailing "?prop=" or
// "?list=" query is ignored. The scope prefix is normalised: a legacy root
// matches with or without ./Device.
func (t *Tree) Lookup(uri string) (Match, bool) {
	segs := Segments(uri)
	root := Segments(t.Root.URI)
	if t.Scope == ScopeLegacy && len(segs) > 1 && strings.EqualFold(segs[1], "Device") {
		segs = append([]string{segs[0]}, segs[2:]...)
	}
	if len(segs) < len(root) {
		return Match{}, false
	}
	for i := range root {
		if !strings.EqualFold(segs[i], root[i]) {
			return Match{}, false
		}
	}
	n := t.Root
	params := map[string]string{}
	for _, seg := range segs[len(root):] {
		var next *Node
		for _, c := range n.Children {
			if c.Dynamic {
				continue
			}
			if c.CaseSensitive && c.Name == seg || !c.CaseSensitive && strings.EqualFold(c.Name, seg) {
				next = c
				break
			}
		}
		if next == nil {
			for _, c := range n.Children {
				if c.Dynamic {
					next = c
					params[c.Title] = seg
					break
				}
			}
		}
		if next == nil {
			return Match{}, false
		}
		n = next
	}
	return Match{Node: n, Params: params}, true
}

// Segments splits an OMA-URI into decoded segments, dropping a trailing
// query. "./Device/Vendor/MSFT/DMClient" yields [".", "Device", "Vendor",
// "MSFT", "DMClient"].
func Segments(uri string) []string {
	if i := strings.IndexByte(uri, '?'); i >= 0 {
		uri = uri[:i]
	}
	uri = strings.TrimSuffix(uri, "/")
	parts := strings.Split(uri, "/")
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

// Concrete builds a concrete URI from the node's template and the values for its
// dynamic segments, encoding each value for a LocURI. Missing values are
// left as their {Title} placeholder.
func (n *Node) Concrete(params map[string]string) string {
	segs := strings.Split(n.URI, "/")
	for i, s := range segs {
		if len(s) > 2 && s[0] == '{' && s[len(s)-1] == '}' {
			if v, ok := params[s[1:len(s)-1]]; ok {
				segs[i] = url.PathEscape(v)
			}
		}
	}
	return strings.Join(segs, "/")
}

// Inherit propagates each node's Applicability to descendants that have none,
// the way MSFT:Applicability inherits in the DDF, and returns the tree. The
// generated packages call it on their literals, which record only a node's
// own applicability.
func Inherit(t *Tree) *Tree {
	var walk func(n *Node, inherited *Applicability)
	walk = func(n *Node, inherited *Applicability) {
		if n.Applicability == nil {
			n.Applicability = inherited
		}
		for _, c := range n.Children {
			walk(c, n.Applicability)
		}
	}
	walk(t.Root, nil)
	return t
}
