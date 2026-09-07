package schemagen

import "github.com/deploymenttheory/go-microsoft-dm/schema/csp"

// Census counts what a bundle contains, so a wrong parse is caught by
// numbers before anything is generated from it.
type Census struct {
	Files          int
	Trees          int
	StandaloneCSPs int
	PolicyAreas    int
	DualScopeFiles int
	Nodes          int
	Leaves         int
	DynamicNodes   int
	Formats        map[csp.Format]int
	ValueTypes     map[csp.ValueType]int
	Applicability  int
	AdmxBacked     int
	GpMappings     int
	Lists          int
	NamingKinds    map[csp.NamingKind]int
	Dependencies   int
	Deprecated     int
	AtomicRequired int
	RebootBehavior int
}

// TakeCensus counts the bundle.
func TakeCensus(b *Drop) Census {
	c := Census{
		Files:       b.Files,
		Trees:       len(b.Trees),
		Formats:     map[csp.Format]int{},
		ValueTypes:  map[csp.ValueType]int{},
		NamingKinds: map[csp.NamingKind]int{},
	}
	perFile := map[string]int{}
	standalone := map[string]bool{}
	areas := map[string]bool{}
	for _, t := range b.Trees {
		perFile[t.File]++
		if t.PolicyArea {
			areas[t.File] = true
		} else {
			standalone[t.File] = true
		}
		for _, n := range t.Nodes() {
			c.Nodes++
			if n.Leaf() {
				c.Leaves++
			}
			if n.Dynamic {
				c.DynamicNodes++
				c.NamingKinds[n.Naming.Kind]++
			}
			c.Formats[n.Format]++
			if n.OwnApplicability {
				c.Applicability++
			}
			if n.AllowedValues != nil {
				c.ValueTypes[n.AllowedValues.Type]++
				if n.AllowedValues.ADMX != nil {
					c.AdmxBacked++
				}
				if n.AllowedValues.ListDelimiter != "" {
					c.Lists++
				}
			}
			if n.GpMapping != nil {
				c.GpMappings++
			}
			if len(n.Dependencies) > 0 {
				c.Dependencies++
			}
			if n.Deprecated != nil {
				c.Deprecated++
			}
			if n.AtomicRequired {
				c.AtomicRequired++
			}
			if n.RebootBehavior != "" {
				c.RebootBehavior++
			}
		}
	}
	c.StandaloneCSPs = len(standalone)
	c.PolicyAreas = len(areas)
	for _, k := range perFile {
		if k > 1 {
			c.DualScopeFiles++
		}
	}
	return c
}
