package schemagen

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/deploymenttheory/go-microsoft-dm/schema/csp"
)

// packageName turns a CSP or area name into a Go package directory name:
// lower-case letters and digits only, prefixed when it would start with a
// digit. "ADMX_AppCompat" becomes "admxappcompat".
func packageName(name string) string {
	var sb strings.Builder
	for _, r := range strings.ToLower(name) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			sb.WriteRune(r)
		}
	}
	s := sb.String()
	if s == "" {
		return "x"
	}
	if s[0] >= '0' && s[0] <= '9' {
		return "n" + s
	}
	return s
}

// identifier makes an exported Go identifier from a node name or title:
// non-alphanumeric characters are dropped and each remaining word is
// capitalised at its first letter. A leading digit gets an "N" prefix.
func identifier(s string) string {
	var sb strings.Builder
	upper := true
	for _, r := range s {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			if upper {
				r = unicode.ToUpper(r)
				upper = false
			}
			sb.WriteRune(r)
		default:
			upper = true
		}
	}
	out := sb.String()
	if out == "" {
		return "X"
	}
	if out[0] >= '0' && out[0] <= '9' {
		out = "N" + out
	}
	return out
}

// paramName makes a lower-camel parameter name from a dynamic node title,
// avoiding Go keywords.
func paramName(title string) string {
	id := identifier(title)
	runes := []rune(id)
	runes[0] = unicode.ToLower(runes[0])
	p := string(runes)
	switch p {
	case "type",
		"func",
		"map",
		"range",
		"var",
		"package",
		"import",
		"default",
		"select",
		"case",
		"go",
		"chan",
		"interface",
		"struct",
		"switch",
		"break",
		"continue",
		"return",
		"if",
		"else",
		"for",
		"const",
		"defer",
		"fallthrough",
		"goto":
		p += "Value"
	}
	return p
}

// named is one node with its generated identifier and parameter list.
type named struct {
	node   *csp.Node
	ident  string
	params []*csp.Node // dynamic ancestors (and self) in path order
	// scoped is the identifier with the scope prefix when the package has
	// two trees.
	scoped string
}

// nameNodes assigns identifiers to every non-root node of a tree. The first
// attempt joins the static segments below the root; nodes that collide are
// renamed with their dynamic ancestors' titles included; any remaining
// collision gets a numeric suffix. The result is deterministic for a given
// tree.
func nameNodes(t *csp.Tree, scopePrefix string) []named {
	var all []named
	var walk func(n *csp.Node, statics []string, dyn []*csp.Node)
	walk = func(n *csp.Node, statics []string, dyn []*csp.Node) {
		for _, c := range n.Children {
			s := statics
			d := dyn
			if c.Dynamic {
				d = append(append([]*csp.Node{}, dyn...), c)
			} else {
				s = append(append([]string{}, statics...), identifier(c.Name))
			}
			all = append(all, named{node: c, ident: strings.Join(s, ""), params: d})
			walk(c, s, d)
		}
	}
	walk(t.Root, nil, nil)
	// Dynamic nodes themselves get their title as the trailing segment.
	for i := range all {
		if all[i].node.Dynamic {
			all[i].ident += identifier(all[i].node.Title)
		}
	}
	resolve := func(key func(named) string) map[string][]int {
		groups := map[string][]int{}
		for i, n := range all {
			groups[key(n)] = append(groups[key(n)], i)
		}
		return groups
	}
	for _, idx := range resolve(func(n named) string { return n.ident }) {
		if len(idx) < 2 {
			continue
		}
		for _, i := range idx {
			all[i].ident = withDynamic(all[i])
		}
	}
	for _, idx := range resolve(func(n named) string { return n.ident }) {
		if len(idx) < 2 {
			continue
		}
		sort.Ints(idx)
		for k, i := range idx {
			if k > 0 {
				all[i].ident = fmt.Sprintf("%s%d", all[i].ident, k+1)
			}
		}
	}
	for i := range all {
		all[i].scoped = scopePrefix + all[i].ident
		if reserved[all[i].scoped] || reserved[all[i].ident] {
			all[i].ident += "Node"
			all[i].scoped += "Node"
		}
	}
	return all
}

// reserved are the identifiers every generated package defines itself.
var reserved = map[string]bool{
	"Root": true, "DeviceRoot": true, "UserRoot": true, "LegacyRoot": true,
	"Tree": true, "DeviceTree": true, "UserTree": true, "LegacyTree": true,
	"Trees": true, "escape": true,
}

// withDynamic appends the titles of a node's dynamic segments to its
// identifier ("X" under {ProfileName} becomes "XByProfileName"), which is
// how two nodes that share their static segments are told apart. A node
// with no dynamic segment keeps its identifier.
func withDynamic(n named) string {
	var parts []string
	for _, seg := range strings.Split(n.node.URI, "/") {
		if len(seg) > 2 && seg[0] == '{' && seg[len(seg)-1] == '}' {
			parts = append(parts, identifier(seg[1:len(seg)-1]))
		}
	}
	if len(parts) == 0 {
		return n.ident
	}
	return n.ident + "By" + strings.Join(parts, "")
}

// paramList returns the deduplicated parameter names for a node's dynamic
// ancestors, in path order.
func paramList(n named) []string {
	out := make([]string, 0, len(n.params))
	seen := map[string]int{}
	for _, d := range n.params {
		p := paramName(d.Title)
		if k := seen[p]; k > 0 {
			p = fmt.Sprintf("%s%d", p, k+1)
		}
		seen[paramName(d.Title)]++
		out = append(out, p)
	}
	return out
}

// enumIdent derives a constant suffix from an enum entry: the first four
// words of the description, or the value when the description yields nothing.
func enumIdent(e csp.EnumValue) string {
	words := strings.FieldsFunc(
		e.Description,
		func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) },
	)
	if len(words) > 4 {
		words = words[:4]
	}
	id := identifier(strings.Join(words, " "))
	if id == "X" || len(words) == 0 {
		var sb strings.Builder
		for _, r := range e.Value {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				sb.WriteRune(r)
			}
		}
		id = "Value" + sb.String()
	}
	return id
}
