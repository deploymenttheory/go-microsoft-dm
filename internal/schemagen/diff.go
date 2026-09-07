package schemagen

import (
	"fmt"
	"sort"
	"strings"

	"github.com/deploymenttheory/go-microsoft-dm/schema/csp"
)

// Report is the difference between two drops, keyed by node URI (with
// scope). It is what a reviewer reads when Microsoft publishes a new bundle.
type Report struct {
	Added   []string
	Removed []string
	Changed []Change
	// FilesAdded and FilesRemoved list DDF files present in only one drop.
	FilesAdded   []string
	FilesRemoved []string
}

// Change is one node whose recorded facts differ.
type Change struct {
	URI    string
	Fields []string
}

// Diff compares two drops node by node.
func Diff(oldDrop, newDrop *Drop) Report {
	oldNodes, oldFiles := index(oldDrop)
	newNodes, newFiles := index(newDrop)
	var r Report
	for uri, n := range newNodes {
		o, ok := oldNodes[uri]
		if !ok {
			r.Added = append(r.Added, uri)
			continue
		}
		if fields := changedFields(o, n); len(fields) > 0 {
			r.Changed = append(r.Changed, Change{URI: uri, Fields: fields})
		}
	}
	for uri := range oldNodes {
		if _, ok := newNodes[uri]; !ok {
			r.Removed = append(r.Removed, uri)
		}
	}
	for f := range newFiles {
		if !oldFiles[f] {
			r.FilesAdded = append(r.FilesAdded, f)
		}
	}
	for f := range oldFiles {
		if !newFiles[f] {
			r.FilesRemoved = append(r.FilesRemoved, f)
		}
	}
	sort.Strings(r.Added)
	sort.Strings(r.Removed)
	sort.Strings(r.FilesAdded)
	sort.Strings(r.FilesRemoved)
	sort.Slice(r.Changed, func(i, j int) bool { return r.Changed[i].URI < r.Changed[j].URI })
	return r
}

func index(d *Drop) (map[string]*csp.Node, map[string]bool) {
	nodes := map[string]*csp.Node{}
	files := map[string]bool{}
	for _, t := range d.Trees {
		files[t.File] = true
		for _, n := range t.Nodes() {
			nodes[n.URI] = n
		}
	}
	return nodes, files
}

func changedFields(o, n *csp.Node) []string {
	var out []string
	if o.Format != n.Format {
		out = append(out, fmt.Sprintf("format %s -> %s", o.Format, n.Format))
	}
	if fmt.Sprint(o.Access) != fmt.Sprint(n.Access) {
		out = append(out, fmt.Sprintf("access %v -> %v", o.Access, n.Access))
	}
	oa, na := appString(o), appString(n)
	if oa != na {
		out = append(out, fmt.Sprintf("applicability %s -> %s", oa, na))
	}
	if avString(o.AllowedValues) != avString(n.AllowedValues) {
		out = append(out, "allowed values")
	}
	if (o.Deprecated == nil) != (n.Deprecated == nil) {
		out = append(
			out,
			fmt.Sprintf("deprecated %v -> %v", o.Deprecated != nil, n.Deprecated != nil),
		)
	}
	if o.Default != n.Default {
		out = append(out, fmt.Sprintf("default %q -> %q", o.Default, n.Default))
	}
	if o.Description != n.Description {
		out = append(out, "description")
	}
	return out
}

func appString(n *csp.Node) string {
	if n.Applicability == nil {
		return "none"
	}
	return strings.Join(n.Applicability.OsBuildVersions, ",") + " csp " + n.Applicability.CspVersion
}

func avString(av *csp.AllowedValues) string {
	if av == nil {
		return ""
	}
	return fmt.Sprintf("%s|%s|%v|%v|%s", av.Type, av.Value, av.Enum, av.ADMX, av.ListDelimiter)
}

// String renders the report for a terminal.
func (r Report) String() string {
	var b strings.Builder
	fmt.Fprintf(
		&b,
		"files added: %d, removed: %d; nodes added: %d, removed: %d, changed: %d\n",
		len(r.FilesAdded),
		len(r.FilesRemoved),
		len(r.Added),
		len(r.Removed),
		len(r.Changed),
	)
	for _, f := range r.FilesAdded {
		fmt.Fprintf(&b, "+ file %s\n", f)
	}
	for _, f := range r.FilesRemoved {
		fmt.Fprintf(&b, "- file %s\n", f)
	}
	for _, u := range r.Added {
		fmt.Fprintf(&b, "+ %s\n", u)
	}
	for _, u := range r.Removed {
		fmt.Fprintf(&b, "- %s\n", u)
	}
	for _, c := range r.Changed {
		fmt.Fprintf(&b, "~ %s: %s\n", c.URI, strings.Join(c.Fields, "; "))
	}
	return b.String()
}
