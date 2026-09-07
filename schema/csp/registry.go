package csp

import (
	"sort"
	"strings"
)

// Registry resolves URIs across every tree of a bundle.
type Registry struct {
	trees []*Tree
	// byRoot indexes trees by the lower-cased root URI.
	byRoot map[string][]*Tree
}

// NewRegistry indexes trees. Order is preserved for Trees.
func NewRegistry(trees ...*Tree) *Registry {
	r := &Registry{trees: trees, byRoot: map[string][]*Tree{}}
	for _, t := range trees {
		key := strings.ToLower(t.Root.URI)
		r.byRoot[key] = append(r.byRoot[key], t)
	}
	return r
}

// Trees returns every tree in registration order.
func (r *Registry) Trees() []*Tree { return r.trees }

// Tree returns the tree named name in the given scope (or any scope when
// scope is ""), policy areas included.
func (r *Registry) Tree(name string, scope Scope) (*Tree, bool) {
	for _, t := range r.trees {
		if strings.EqualFold(t.Name, name) && (scope == "" || t.Scope == scope) {
			return t, true
		}
	}
	return nil, false
}

// Lookup resolves a concrete URI. Roots are tried longest first so a Policy
// area (./Device/Vendor/MSFT/Policy/Config/BITS) wins over the Policy CSP's
// own skeleton (./Device/Vendor/MSFT/Policy). A URI without a ./Device or
// ./User prefix is device scope: it matches legacy roots as written and
// ./Device roots with the prefix supplied.
func (r *Registry) Lookup(uri string) (Match, *Tree, bool) {
	candidates := r.candidates(uri)
	for _, t := range candidates {
		if m, ok := t.Lookup(uri); ok {
			return m, t, true
		}
	}
	if !hasScopePrefix(uri) {
		aliased := "./Device" + strings.TrimPrefix(uri, ".")
		for _, t := range r.candidates(aliased) {
			if m, ok := t.Lookup(aliased); ok {
				return m, t, true
			}
		}
	}
	return Match{}, nil, false
}

func hasScopePrefix(uri string) bool {
	l := strings.ToLower(uri)
	return strings.HasPrefix(l, "./device/") || strings.HasPrefix(l, "./user/") || l == "./device" || l == "./user"
}

// candidates returns the trees whose root is a prefix of uri, longest root
// first.
func (r *Registry) candidates(uri string) []*Tree {
	segs := Segments(uri)
	var out []*Tree
	for i := len(segs); i >= 1; i-- {
		key := strings.ToLower(strings.Join(segs[:i], "/"))
		if i == 1 && segs[0] == "." {
			break
		}
		// Root URIs start with "./"; Segments drops the empty part, so
		// rebuild the same shape for the key.
		if ts, ok := r.byRoot[key]; ok {
			out = append(out, ts...)
		}
	}
	// A legacy root (./Vendor/MSFT/X) must also match ./Device/Vendor/MSFT/X.
	if len(segs) > 1 && strings.EqualFold(segs[1], "Device") {
		for i := len(segs); i >= 2; i-- {
			key := strings.ToLower(strings.Join(append([]string{segs[0]}, segs[2:i]...), "/"))
			if ts, ok := r.byRoot[key]; ok {
				for _, t := range ts {
					if t.Scope == ScopeLegacy {
						out = append(out, t)
					}
				}
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return len(out[i].Root.URI) > len(out[j].Root.URI) })
	return out
}
