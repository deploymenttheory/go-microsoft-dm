package validation

import (
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/deploymenttheory/go-microsoft-dm/schema/csp"
)

// Sentinels for the rules Check applies. Every returned error unwraps to
// ErrInvalid and to one of these.
var (
	ErrInvalid       = errors.New("validation: invalid")
	ErrUnknownNode   = errors.New("unknown node")
	ErrVerb          = errors.New("verb not permitted")
	ErrFormat        = errors.New("value does not match the node format")
	ErrValue         = errors.New("value not allowed")
	ErrInteriorValue = errors.New("interior node cannot carry a value")
	ErrDynamicName   = errors.New("dynamic node name does not match its pattern")
)

// Verb is the SyncML command being validated.
type Verb string

const (
	Add     Verb = "Add"
	Replace Verb = "Replace"
	Delete  Verb = "Delete"
	Get     Verb = "Get"
	Exec    Verb = "Exec"
)

// Command is what Check examines.
type Command struct {
	Verb Verb
	URI  string
	// Format is the Meta/Format the command carries, or "" when omitted.
	Format string
	// Value is the Data; nil for a command without Data (Get, Delete, an
	// Exec without parameters, an Add of an interior node).
	Value *string
}

// Result is the outcome of Check.
type Result struct {
	// Node and Tree are set whenever the URI resolved.
	Node *csp.Node
	Tree *csp.Tree
	// Params are the dynamic segment values captured from the URI.
	Params map[string]string
	// AtomicRequired is true when this node or an ancestor demands that
	// writes be grouped in one Atomic.
	AtomicRequired bool
	// Unevaluated names allowed-value constraints the schema records but
	// this package does not evaluate (XSD, SDDL, JSON).
	Unevaluated []string
	// Errors lists every broken rule; nil when the command is acceptable.
	Errors []error
}

// Err joins the errors, or returns nil.
func (r Result) Err() error { return errors.Join(r.Errors...) }

// Problem is one broken rule with its sentinel.
type Problem struct {
	Rule error
	Msg  string
}

func (p *Problem) Error() string   { return "validation: " + p.Msg }
func (p *Problem) Unwrap() []error { return []error{ErrInvalid, p.Rule} }

// Check validates one command against the registry.
func Check(reg *csp.Registry, c Command) Result {
	var res Result
	fail := func(rule error, format string, args ...any) {
		res.Errors = append(res.Errors, &Problem{Rule: rule, Msg: fmt.Sprintf(format, args...)})
	}
	m, tree, ok := reg.Lookup(c.URI)
	if !ok {
		fail(ErrUnknownNode, "%q is not in the DDF bundle", c.URI)
		return res
	}
	res.Node, res.Tree, res.Params = m.Node, tree, m.Params
	n := m.Node
	if verb := csp.Access(c.Verb); !n.Allows(verb) {
		fail(ErrVerb, "%s does not permit %s (allowed: %v)", n.URI, c.Verb, n.Access)
	}
	res.AtomicRequired = atomicRequired(tree, n)
	checkDynamicNames(tree, n, m.Params, fail)
	if c.Verb == Get || c.Verb == Delete {
		return res
	}
	if !n.Leaf() {
		if c.Value != nil && *c.Value != "" {
			fail(ErrInteriorValue, "%s is an interior node", n.URI)
		}
		if c.Format != "" && c.Format != string(csp.FormatNode) {
			fail(ErrFormat, "%s is an interior node; Meta/Format must be node, got %q", n.URI, c.Format)
		}
		return res
	}
	if c.Format != "" && c.Format != string(n.Format) && !(n.Format == csp.FormatChr && c.Format == "xml") {
		fail(ErrFormat, "%s has DFFormat %s; Meta/Format %q does not match", n.URI, n.Format, c.Format)
	}
	if c.Value == nil {
		if c.Verb != Exec && n.Format != csp.FormatNull {
			fail(ErrFormat, "%s %s needs a value", c.Verb, n.URI)
		}
		return res
	}
	v := *c.Value
	if err := checkFormat(n.Format, v); err != nil {
		fail(ErrFormat, "%s: %v", n.URI, err)
	}
	av := n.AllowedValues
	if av == nil {
		return res
	}
	values := []string{v}
	if av.ListDelimiter != "" {
		values = splitList(v, av.ListDelimiter)
	}
	for _, one := range values {
		switch av.Type {
		case csp.ValueTypeEnum:
			if !inEnum(av.Enum, one) {
				fail(ErrValue, "%s: %q is not one of %s", n.URI, one, enumValues(av.Enum))
			}
		case csp.ValueTypeFlag:
			if err := checkFlag(av.Enum, one); err != nil {
				fail(ErrValue, "%s: %v", n.URI, err)
			}
		case csp.ValueTypeRange:
			if err := checkRange(av.Value, one); err != nil {
				fail(ErrValue, "%s: %v", n.URI, err)
			}
		case csp.ValueTypeRegEx:
			re, err := compile(av.Value)
			if err != nil {
				res.Unevaluated = append(res.Unevaluated, "RegEx (pattern does not compile in Go: "+av.Value+")")
			} else if !re.MatchString(one) {
				fail(ErrValue, "%s: %q does not match %s", n.URI, one, av.Value)
			}
		case csp.ValueTypeADMX:
			if err := checkADMX(one); err != nil {
				fail(ErrValue, "%s: %v", n.URI, err)
			}
		case csp.ValueTypeXSD, csp.ValueTypeSDDL, csp.ValueTypeJSON:
			res.Unevaluated = append(res.Unevaluated, string(av.Type))
		}
	}
	return res
}

func atomicRequired(t *csp.Tree, target *csp.Node) bool {
	found := false
	var walk func(n *csp.Node, inherited bool) bool
	walk = func(n *csp.Node, inherited bool) bool {
		req := inherited || n.AtomicRequired
		if n == target {
			found = req
			return false
		}
		for _, c := range n.Children {
			if !walk(c, req) {
				return false
			}
		}
		return true
	}
	walk(t.Root, false)
	return found
}

// checkDynamicNames applies UniqueName patterns to the captured segments.
func checkDynamicNames(t *csp.Tree, target *csp.Node, params map[string]string, fail func(error, string, ...any)) {
	var path []*csp.Node
	var walk func(n *csp.Node, trail []*csp.Node) bool
	walk = func(n *csp.Node, trail []*csp.Node) bool {
		trail = append(trail, n)
		if n == target {
			path = append([]*csp.Node{}, trail...)
			return false
		}
		for _, c := range n.Children {
			if !walk(c, trail) {
				return false
			}
		}
		return true
	}
	walk(t.Root, nil)
	for _, n := range path {
		if !n.Dynamic || n.Naming.Kind != csp.NamingUniqueName || n.Naming.Pattern == "" {
			continue
		}
		v, ok := params[n.Title]
		if !ok {
			continue
		}
		re, err := compile("^(?:" + n.Naming.Pattern + ")$")
		if err != nil {
			continue
		}
		if !re.MatchString(v) {
			fail(ErrDynamicName, "%s: %q does not match %s", n.URI, v, n.Naming.Pattern)
		}
	}
}

var (
	reMu    sync.Mutex
	reCache = map[string]*regexp.Regexp{}
)

func compile(p string) (*regexp.Regexp, error) {
	reMu.Lock()
	defer reMu.Unlock()
	if re, ok := reCache[p]; ok {
		return re, nil
	}
	re, err := regexp.Compile(p)
	if err != nil {
		return nil, fmt.Errorf("regexp: %w", err)
	}
	reCache[p] = re
	return re, nil
}

func checkFormat(f csp.Format, v string) error {
	switch f {
	case csp.FormatInt:
		if _, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64); err != nil {
			return fmt.Errorf("%q is not an integer", v)
		}
	case csp.FormatBool:
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "true", "false":
		default:
			return fmt.Errorf("%q is not true or false", v)
		}
	case csp.FormatB64:
		if _, err := base64.StdEncoding.DecodeString(strings.TrimSpace(v)); err != nil {
			return fmt.Errorf("value is not base64: %w", err)
		}
	case csp.FormatXML:
		if err := wellFormed(v); err != nil {
			return fmt.Errorf("value is not well-formed XML: %w", err)
		}
	case csp.FormatFloat:
		if _, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err != nil {
			return fmt.Errorf("%q is not a number", v)
		}
	case csp.FormatNull:
		if strings.TrimSpace(v) != "" {
			return errors.New("null-format node takes no value")
		}
	}
	return nil
}

func wellFormed(v string) error {
	dec := xml.NewDecoder(strings.NewReader("<r>" + v + "</r>"))
	for {
		if _, err := dec.Token(); err != nil {
			if errors.Is(err, errEOF) || err.Error() == "EOF" {
				return nil
			}
			return err
		}
	}
}

var errEOF = errors.New("EOF")

func splitList(v, delim string) []string {
	switch delim {
	case "0xF000", "\\xF000", "":
		delim = ""
	}
	parts := strings.Split(v, delim)
	out := parts[:0]
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func inEnum(es []csp.EnumValue, v string) bool {
	for _, e := range es {
		if strings.EqualFold(e.Value, strings.TrimSpace(v)) {
			return true
		}
	}
	return false
}

func enumValues(es []csp.EnumValue) string {
	vals := make([]string, len(es))
	for i, e := range es {
		vals[i] = e.Value
	}
	return strings.Join(vals, ", ")
}

// checkFlag accepts any bitwise OR of the flag values.
func checkFlag(es []csp.EnumValue, v string) error {
	n, err := strconv.ParseInt(strings.TrimSpace(v), 0, 64)
	if err != nil {
		return fmt.Errorf("%q is not an integer flag", v)
	}
	var mask int64
	for _, e := range es {
		f, err := strconv.ParseInt(e.Value, 0, 64)
		if err != nil {
			continue
		}
		mask |= f
	}
	if n&^mask != 0 {
		return fmt.Errorf("%d has bits outside the allowed flags (%s)", n, enumValues(es))
	}
	return nil
}

var rangeRE = regexp.MustCompile(`^\[\s*(-?\d+)\s*(?:-\s*(-?\d+))?\s*\]$`)

// checkRange parses "[lo-hi]" (or "[n]" for a single value) and every
// comma-separated alternative the DDF writes as "[0-5],[10]".
func checkRange(spec, v string) error {
	n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
	if err != nil {
		return fmt.Errorf("%q is not an integer", v)
	}
	for _, alt := range strings.Split(spec, ",") {
		m := rangeRE.FindStringSubmatch(strings.TrimSpace(alt))
		if m == nil {
			return fmt.Errorf("range %q is not understood", spec)
		}
		lo, _ := strconv.ParseInt(m[1], 10, 64)
		hi := lo
		if m[2] != "" {
			hi, _ = strconv.ParseInt(m[2], 10, 64)
		}
		if n >= lo && n <= hi {
			return nil
		}
	}
	return fmt.Errorf("%d is outside %s", n, spec)
}

// checkADMX accepts <enabled/>, <disabled/>, or <enabled/> followed by
// <data id="..." value="..."/> elements, in any casing, which is the payload
// grammar the Learn ADMX pages define.
func checkADMX(v string) error {
	dec := xml.NewDecoder(strings.NewReader("<r>" + v + "</r>"))
	depth := 0
	state := ""
	for {
		tok, err := dec.Token()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return fmt.Errorf("ADMX payload is not well-formed XML: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			if depth == 1 {
				continue
			}
			if depth > 2 {
				return errors.New("ADMX payload elements cannot nest")
			}
			switch strings.ToLower(t.Name.Local) {
			case "enabled":
				if state != "" {
					return errors.New("ADMX payload has more than one enabled/disabled element")
				}
				state = "enabled"
			case "disabled":
				if state != "" {
					return errors.New("ADMX payload has more than one enabled/disabled element")
				}
				state = "disabled"
			case "data":
				if state != "enabled" {
					return errors.New("ADMX data elements must follow <enabled/>")
				}
				hasID, hasValue := false, false
				for _, a := range t.Attr {
					switch strings.ToLower(a.Name.Local) {
					case "id":
						hasID = a.Value != ""
					case "value":
						hasValue = true
					}
				}
				if !hasID || !hasValue {
					return errors.New("ADMX data element needs id and value attributes")
				}
			default:
				return fmt.Errorf("ADMX payload has unexpected element %s", t.Name.Local)
			}
		case xml.EndElement:
			depth--
		case xml.CharData:
			if depth >= 1 && strings.TrimSpace(string(t)) != "" {
				return errors.New("ADMX payload cannot contain text")
			}
		}
	}
	if state == "" {
		return errors.New("ADMX payload needs <enabled/> or <disabled/>")
	}
	return nil
}
