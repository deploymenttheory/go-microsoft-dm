package schemagen

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/deploymenttheory/go-microsoft-dm/schema/csp"
)

// The DDF elements carry no namespace (the bundle never declares the
// tempuri.org default namespace the XSD names) and the Microsoft extensions
// are in the http://schemas.microsoft.com/MobileDevice/DM namespace. Go's
// decoder matches the namespaced tags below by URI, so the prefix the file
// uses does not matter.

var (
	// ErrDDF wraps every DDF parse failure.
	ErrDDF = errors.New("schemagen: ddf")
	// ErrDDFNamespace reports a file whose DDF elements are namespaced, which
	// no Microsoft drop has done and the parser does not support.
	ErrDDFNamespace = errors.New("schemagen: ddf elements must not be namespaced")
)

// Drop is every tree parsed from one DDF drop.
type Drop struct {
	// Files is the number of XML files in the archive.
	Files int
	// Trees are the roots of every file, in file name order then document
	// order; a file with a User and a Device root yields two trees.
	Trees []*csp.Tree
}

// ParseZip parses every .xml member of a DDF v2 drop.
func ParseZip(zipPath string) (*Drop, error) {
	f, err := os.Open(zipPath)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDDF, err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDDF, err)
	}
	zr, err := zip.NewReader(f, info.Size())
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %w", ErrDDF, zipPath, err)
	}
	members := make([]*zip.File, 0, len(zr.File))
	for _, m := range zr.File {
		if strings.HasSuffix(strings.ToLower(m.Name), ".xml") {
			members = append(members, m)
		}
	}
	sort.Slice(members, func(i, j int) bool { return members[i].Name < members[j].Name })
	b := &Drop{Files: len(members)}
	for _, m := range members {
		rc, err := m.Open()
		if err != nil {
			return nil, fmt.Errorf("%w: %s: %w", ErrDDF, m.Name, err)
		}
		data, err := io.ReadAll(io.LimitReader(rc, 64<<20))
		rc.Close()
		if err != nil {
			return nil, fmt.Errorf("%w: %s: %w", ErrDDF, m.Name, err)
		}
		trees, err := ParseDDF(path.Base(m.Name), data)
		if err != nil {
			return nil, err
		}
		b.Trees = append(b.Trees, trees...)
	}
	return b, nil
}

// ParseDDF parses one DDF file into one tree per root node.
func ParseDDF(file string, data []byte) ([]*csp.Tree, error) {
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	var tree mgmtTree
	dec := xml.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&tree); err != nil {
		return nil, fmt.Errorf("%w: %s: %w", ErrDDF, file, err)
	}
	if tree.XMLName.Space != "" {
		return nil, fmt.Errorf("%w: %s declares %q", ErrDDFNamespace, file, tree.XMLName.Space)
	}
	if tree.XMLName.Local != "MgmtTree" {
		return nil, fmt.Errorf(
			"%w: %s: root element is %s, want MgmtTree",
			ErrDDF,
			file,
			tree.XMLName.Local,
		)
	}
	if len(tree.Nodes) == 0 {
		return nil, fmt.Errorf("%w: %s: no root Node", ErrDDF, file)
	}
	var out []*csp.Tree
	for i := range tree.Nodes {
		x := &tree.Nodes[i]
		if x.Path == nil {
			return nil, fmt.Errorf("%w: %s: root node %q has no Path", ErrDDF, file, x.NodeName)
		}
		p := strings.TrimSpace(*x.Path)
		// The bundle writes "./Vendor/MSFT/" (SUPL) once; a trailing slash is
		// a typo, not a different root.
		p = strings.TrimSuffix(p, "/")
		if p == "" {
			return nil, fmt.Errorf(
				"%w: %s: root node %q has an empty Path",
				ErrDDF,
				file,
				x.NodeName,
			)
		}
		rootURI := p + "/" + x.NodeName
		if p == "." {
			rootURI = "./" + x.NodeName
		}
		t := &csp.Tree{
			Name:       x.NodeName,
			File:       file,
			Scope:      scopeOf(p),
			PolicyArea: strings.Contains(p, "/Policy/Config"),
		}
		root, err := convert(file, x, rootURI, nil)
		if err != nil {
			return nil, err
		}
		t.Root = root
		out = append(out, t)
	}
	return out, nil
}

func scopeOf(p string) csp.Scope {
	switch {
	case strings.HasPrefix(p, "./Device"):
		return csp.ScopeDevice
	case strings.HasPrefix(p, "./User"):
		return csp.ScopeUser
	default:
		return csp.ScopeLegacy
	}
}

func convert(file string, x *xmlNode, uri string, inherited *csp.Applicability) (*csp.Node, error) {
	n := &csp.Node{
		Name:    x.NodeName,
		Title:   strings.TrimSpace(x.Properties.Title),
		URI:     uri,
		Dynamic: x.NodeName == "",
	}
	if x.Path != nil && uri != "" &&
		!strings.HasPrefix(uri, strings.TrimSuffix(strings.TrimSpace(*x.Path), "/")) {
		return nil, fmt.Errorf("%w: %s: Path on a non-root node %q", ErrDDF, file, uri)
	}
	pr := &x.Properties
	f := pr.Format.first()
	if f == "" {
		return nil, fmt.Errorf("%w: %s: %s has no DFFormat", ErrDDF, file, uri)
	}
	switch csp.Format(f) {
	case csp.FormatNode,
		csp.FormatChr,
		csp.FormatInt,
		csp.FormatBool,
		csp.FormatB64,
		csp.FormatNull,
		csp.FormatXML,
		csp.FormatBin,
		csp.FormatTime,
		csp.FormatDate,
		csp.FormatFloat:
		n.Format = csp.Format(f)
	default:
		return nil, fmt.Errorf("%w: %s: %s has unknown DFFormat %q", ErrDDF, file, uri, f)
	}
	for _, a := range pr.AccessType.all() {
		n.Access = append(n.Access, csp.Access(a))
	}
	sort.Slice(n.Access, func(i, j int) bool { return n.Access[i] < n.Access[j] })
	n.Description = strings.TrimSpace(pr.Description)
	n.Default = strings.TrimSpace(pr.DefaultValue)
	n.Occurrence = pr.Occurrence.first()
	n.Permanent = pr.Scope.first() == "Permanent"
	n.MIME = strings.TrimSpace(pr.DFType.MIME)
	n.CaseSensitive = pr.CaseSense.first() == "CS"
	if pr.DynamicNaming != nil {
		switch {
		case pr.DynamicNaming.UniqueName != nil:
			n.Naming = csp.Naming{
				Kind:    csp.NamingUniqueName,
				Pattern: strings.TrimSpace(*pr.DynamicNaming.UniqueName),
			}
		case pr.DynamicNaming.ClientInventory != nil:
			n.Naming = csp.Naming{Kind: csp.NamingClientInventory}
		case pr.DynamicNaming.ServerGenerated != nil:
			n.Naming = csp.Naming{Kind: csp.NamingServerGeneratedUniqueIdentifier}
		default:
			return nil, fmt.Errorf("%w: %s: %s has an empty DynamicNodeNaming", ErrDDF, file, uri)
		}
	}
	if pr.Applicability != nil {
		n.Applicability = pr.Applicability.convert()
		n.OwnApplicability = true
	} else {
		n.Applicability = inherited
	}
	if pr.AllowedValues != nil {
		av, err := pr.AllowedValues.convert(file, uri)
		if err != nil {
			return nil, err
		}
		n.AllowedValues = av
	}
	if pr.GpMapping != nil {
		n.GpMapping = &csp.GpMapping{
			EnglishName: pr.GpMapping.EnglishName,
			AreaPath:    pr.GpMapping.AreaPath,
			Element:     pr.GpMapping.Element,
		}
	}
	n.ConflictResolution = strings.TrimSpace(pr.ConflictResolution)
	n.ReplaceBehavior = strings.TrimSpace(pr.ReplaceBehavior)
	n.RebootBehavior = strings.TrimSpace(pr.RebootBehavior)
	n.AtomicRequired = pr.AtomicRequired != nil
	if pr.Deprecated != nil {
		n.Deprecated = &csp.Deprecated{
			OsBuildDeprecated: strings.TrimSpace(pr.Deprecated.OsBuildDeprecated),
		}
	}
	if pr.DependencyBehavior != nil {
		for _, g := range pr.DependencyBehavior.Groups {
			dg := csp.DependencyGroup{FriendlyID: g.FriendlyID}
			if g.Changed != nil {
				cav, err := g.Changed.convert(file, uri)
				if err != nil {
					return nil, err
				}
				dg.ChangedAllowedValues = cav
			}
			for _, d := range g.Dependencies {
				dep := csp.Dependency{Type: d.Type, URI: strings.TrimSpace(d.URI)}
				if d.AllowedValue != nil {
					dep.ValueType = csp.ValueType(d.AllowedValue.ValueType)
					dep.Value = strings.TrimSpace(d.AllowedValue.Value)
					for _, e := range d.AllowedValue.Enum {
						dep.Enum = append(
							dep.Enum,
							csp.EnumValue{
								Value:       strings.TrimSpace(e.Value),
								Description: strings.TrimSpace(e.Description),
							},
						)
					}
				}
				dg.Dependencies = append(dg.Dependencies, dep)
			}
			n.Dependencies = append(n.Dependencies, dg)
		}
	}
	seen := map[string]bool{}
	for i := range x.Nodes {
		c := &x.Nodes[i]
		seg := c.NodeName
		if seg == "" {
			seg = "{" + strings.TrimSpace(c.Properties.Title) + "}"
			if seg == "{}" {
				return nil, fmt.Errorf(
					"%w: %s: dynamic child of %s has no DFTitle",
					ErrDDF,
					file,
					uri,
				)
			}
		}
		key := strings.ToLower(seg)
		if seen[key] {
			return nil, fmt.Errorf("%w: %s: %s has two children named %q", ErrDDF, file, uri, seg)
		}
		seen[key] = true
		child, err := convert(file, c, uri+"/"+seg, n.Applicability)
		if err != nil {
			return nil, err
		}
		n.Children = append(n.Children, child)
	}
	return n, nil
}

// XML shapes. Element names are unqualified; MSFT extensions use the
// namespace URI so either prefix binds.

type mgmtTree struct {
	XMLName xml.Name  `xml:"MgmtTree"`
	VerDTD  string    `xml:"VerDTD"`
	Nodes   []xmlNode `xml:"Node"`
}

type xmlNode struct {
	NodeName   string        `xml:"NodeName"`
	Path       *string       `xml:"Path"`
	Properties xmlProperties `xml:"DFProperties"`
	Nodes      []xmlNode     `xml:"Node"`
}

type xmlProperties struct {
	AccessType         xmlChoice          `xml:"AccessType"`
	Description        string             `xml:"Description"`
	Format             xmlChoice          `xml:"DFFormat"`
	Occurrence         xmlChoice          `xml:"Occurrence"`
	Scope              xmlChoice          `xml:"Scope"`
	Title              string             `xml:"DFTitle"`
	DFType             xmlDFType          `xml:"DFType"`
	DefaultValue       string             `xml:"DefaultValue"`
	CaseSense          xmlChoice          `xml:"CaseSense"`
	Applicability      *xmlApplicability  `xml:"http://schemas.microsoft.com/MobileDevice/DM Applicability"`
	DynamicNaming      *xmlDynamicNaming  `xml:"http://schemas.microsoft.com/MobileDevice/DM DynamicNodeNaming"`
	AllowedValues      *xmlAllowedValues  `xml:"http://schemas.microsoft.com/MobileDevice/DM AllowedValues"`
	GpMapping          *xmlGpMapping      `xml:"http://schemas.microsoft.com/MobileDevice/DM GpMapping"`
	ConflictResolution string             `xml:"http://schemas.microsoft.com/MobileDevice/DM ConflictResolution"`
	ReplaceBehavior    string             `xml:"http://schemas.microsoft.com/MobileDevice/DM ReplaceBehavior"`
	RebootBehavior     string             `xml:"http://schemas.microsoft.com/MobileDevice/DM RebootBehavior"`
	AtomicRequired     *struct{}          `xml:"http://schemas.microsoft.com/MobileDevice/DM AtomicRequired"`
	Deprecated         *xmlDeprecated     `xml:"http://schemas.microsoft.com/MobileDevice/DM Deprecated"`
	DependencyBehavior *xmlDependencyBhvr `xml:"http://schemas.microsoft.com/MobileDevice/DM DependencyBehavior"`
}

// xmlChoice reads an element whose content is one (or more) empty child
// elements naming a choice: <DFFormat><chr/></DFFormat>.
type xmlChoice struct {
	Elems []xmlElem `xml:",any"`
}

type xmlElem struct {
	XMLName xml.Name `xml:""`
}

func (c xmlChoice) first() string {
	if len(c.Elems) == 0 {
		return ""
	}
	return c.Elems[0].XMLName.Local
}

func (c xmlChoice) all() []string {
	out := make([]string, 0, len(c.Elems))
	for _, e := range c.Elems {
		out = append(out, e.XMLName.Local)
	}
	return out
}

type xmlDFType struct {
	MIME    string    `xml:"MIME"`
	DDFName *struct{} `xml:"DDFName"`
}

type xmlApplicability struct {
	OsBuildVersion   string    `xml:"http://schemas.microsoft.com/MobileDevice/DM OsBuildVersion"`
	CspVersion       string    `xml:"http://schemas.microsoft.com/MobileDevice/DM CspVersion"`
	EditionAllowList string    `xml:"http://schemas.microsoft.com/MobileDevice/DM EditionAllowList"`
	RequiresAzureAd  *struct{} `xml:"http://schemas.microsoft.com/MobileDevice/DM RequiresAzureAd"`
}

func (a *xmlApplicability) convert() *csp.Applicability {
	out := &csp.Applicability{
		CspVersion:      strings.TrimSpace(a.CspVersion),
		RequiresAzureAd: a.RequiresAzureAd != nil,
	}
	out.OsBuildVersions = splitList(a.OsBuildVersion, ",")
	out.EditionAllowList = splitList(a.EditionAllowList, ";")
	return out
}

func splitList(s, sep string) []string {
	var out []string
	for _, p := range strings.Split(s, sep) {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

type xmlDynamicNaming struct {
	UniqueName      *string   `xml:"http://schemas.microsoft.com/MobileDevice/DM UniqueName"`
	ClientInventory *struct{} `xml:"http://schemas.microsoft.com/MobileDevice/DM ClientInventory"`
	ServerGenerated *struct{} `xml:"http://schemas.microsoft.com/MobileDevice/DM ServerGeneratedUniqueIdentifier"`
}

type xmlDeprecated struct {
	OsBuildDeprecated string `xml:"OsBuildDeprecated,attr"`
}

type xmlAllowedValues struct {
	ValueType string    `xml:"ValueType,attr"`
	Value     string    `xml:"http://schemas.microsoft.com/MobileDevice/DM Value"`
	Enum      []xmlEnum `xml:"http://schemas.microsoft.com/MobileDevice/DM Enum"`
	Admx      *xmlAdmx  `xml:"http://schemas.microsoft.com/MobileDevice/DM AdmxBacked"`
	List      *xmlList  `xml:"http://schemas.microsoft.com/MobileDevice/DM List"`
}

type xmlEnum struct {
	Value       string `xml:"http://schemas.microsoft.com/MobileDevice/DM Value"`
	Description string `xml:"http://schemas.microsoft.com/MobileDevice/DM ValueDescription"`
}

type xmlAdmx struct {
	Area string `xml:"Area,attr"`
	Name string `xml:"Name,attr"`
	File string `xml:"File,attr"`
}

type xmlList struct {
	Delimiter string `xml:"Delimiter,attr"`
}

func (a *xmlAllowedValues) convert(file, uri string) (*csp.AllowedValues, error) {
	out := &csp.AllowedValues{Type: csp.ValueType(a.ValueType), Value: strings.TrimSpace(a.Value)}
	switch out.Type {
	case csp.ValueTypeNone,
		csp.ValueTypeEnum,
		csp.ValueTypeFlag,
		csp.ValueTypeRange,
		csp.ValueTypeRegEx,
		csp.ValueTypeXSD,
		csp.ValueTypeADMX,
		csp.ValueTypeSDDL,
		csp.ValueTypeJSON:
	default:
		return nil, fmt.Errorf(
			"%w: %s: %s has unknown AllowedValues ValueType %q",
			ErrDDF,
			file,
			uri,
			a.ValueType,
		)
	}
	for _, e := range a.Enum {
		out.Enum = append(
			out.Enum,
			csp.EnumValue{
				Value:       strings.TrimSpace(e.Value),
				Description: strings.TrimSpace(e.Description),
			},
		)
	}
	if a.Admx != nil {
		out.ADMX = &csp.ADMX{Area: a.Admx.Area, Name: a.Admx.Name, File: a.Admx.File}
	}
	if a.List != nil {
		out.ListDelimiter = a.List.Delimiter
	}
	return out, nil
}

type xmlGpMapping struct {
	EnglishName string `xml:"GpEnglishName,attr"`
	AreaPath    string `xml:"GpAreaPath,attr"`
	Element     string `xml:"GpElement,attr"`
}

type xmlDependencyBhvr struct {
	Groups []xmlDependencyGroup `xml:"http://schemas.microsoft.com/MobileDevice/DM DependencyGroup"`
}

type xmlDependencyGroup struct {
	FriendlyID   string            `xml:"FriendlyId,attr"`
	Changed      *xmlAllowedValues `xml:"http://schemas.microsoft.com/MobileDevice/DM DependencyChangedAllowedValues"`
	Dependencies []xmlDependency   `xml:"http://schemas.microsoft.com/MobileDevice/DM Dependency"`
}

type xmlDependency struct {
	Type         string            `xml:"Type,attr"`
	URI          string            `xml:"http://schemas.microsoft.com/MobileDevice/DM DependencyUri"`
	AllowedValue *xmlAllowedValues `xml:"http://schemas.microsoft.com/MobileDevice/DM DependencyAllowedValue"`
}
