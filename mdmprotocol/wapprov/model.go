package wapprov

import (
	"bytes"
	"crypto/sha1" //nolint:gosec // Windows names certificate store entries by SHA-1 thumbprint
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"strings"
)

// Errors returned by this package.
var (
	// ErrInvalid reports a document or configuration that breaks a rule the
	// specification states.
	ErrInvalid = errors.New("wapprov: invalid")
	// ErrSyntax reports XML that is not a provisioning document.
	ErrSyntax = errors.New("wapprov: malformed document")
)

// Version is the wap-provisioningdoc version every Windows example uses.
const Version = "1.1"

// Parm data types used by the CertificateStore and DMClient examples in
// MS-MDE2 2.2.9.1. The w7 APPLICATION characteristic carries no datatype
// attribute.
const (
	TypeString  = "string"
	TypeInteger = "integer"
	TypeBoolean = "boolean"
)

// Document is a wap-provisioningdoc.
type Document struct {
	Version         string
	Characteristics []Characteristic
}

// Characteristic is a typed node holding parameters and child nodes.
type Characteristic struct {
	Type     string
	Parms    []Parm
	Children []Characteristic
}

// Parm is a name and value, optionally typed. A Flag parm is written as
// <parm name="X"/> with no value attribute; the w7 characteristic uses that
// form for BACKCOMPATRETRYDISABLED and USEHWDEVID, whose presence is the
// setting.
type Parm struct {
	Name     string
	Value    string
	DataType string
	Flag     bool
}

// Find returns the first top-level characteristic of the type, or nil.
func (d *Document) Find(typ string) *Characteristic {
	if d == nil {
		return nil
	}
	for i := range d.Characteristics {
		if d.Characteristics[i].Type == typ {
			return &d.Characteristics[i]
		}
	}
	return nil
}

// FindAll returns every top-level characteristic of the type. A document
// may carry several CertificateStore characteristics.
func (d *Document) FindAll(typ string) []*Characteristic {
	var out []*Characteristic
	if d == nil {
		return out
	}
	for i := range d.Characteristics {
		if d.Characteristics[i].Type == typ {
			out = append(out, &d.Characteristics[i])
		}
	}
	return out
}

// Child returns the first child characteristic of the type, or nil.
func (c *Characteristic) Child(typ string) *Characteristic {
	if c == nil {
		return nil
	}
	for i := range c.Children {
		if c.Children[i].Type == typ {
			return &c.Children[i]
		}
	}
	return nil
}

// Path descends through children by type and returns the last, or nil.
func (c *Characteristic) Path(types ...string) *Characteristic {
	cur := c
	for _, t := range types {
		cur = cur.Child(t)
		if cur == nil {
			return nil
		}
	}
	return cur
}

// Parm returns the named parameter and whether it exists.
func (c *Characteristic) Parm(name string) (Parm, bool) {
	if c == nil {
		return Parm{}, false
	}
	for _, p := range c.Parms {
		if p.Name == name {
			return p, true
		}
	}
	return Parm{}, false
}

// Value returns the named parameter's value, or "" when absent.
func (c *Characteristic) Value(name string) string {
	p, _ := c.Parm(name)
	return p.Value
}

// Options tune Encode.
type Options struct {
	// Indent pretty-prints with two-space indentation.
	Indent bool
	// XMLDeclaration prefixes <?xml version="1.0" encoding="UTF-8"?>.
	XMLDeclaration bool
}

// Encode writes the document. It validates first.
func Encode(d *Document, o Options) ([]byte, error) {
	if err := Validate(d); err != nil {
		return nil, err
	}
	var b bytes.Buffer
	if o.XMLDeclaration {
		b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
		if o.Indent {
			b.WriteByte('\n')
		}
	}
	b.WriteString(`<wap-provisioningdoc version="`)
	esc(&b, d.Version)
	b.WriteString(`">`)
	for i := range d.Characteristics {
		writeCharacteristic(&b, &d.Characteristics[i], 1, o.Indent)
	}
	if o.Indent {
		b.WriteByte('\n')
	}
	b.WriteString(`</wap-provisioningdoc>`)
	return b.Bytes(), nil
}

func writeCharacteristic(b *bytes.Buffer, c *Characteristic, depth int, indent bool) {
	newline(b, depth, indent)
	b.WriteString(`<characteristic type="`)
	esc(b, c.Type)
	if len(c.Parms) == 0 && len(c.Children) == 0 {
		b.WriteString(`"/>`)
		return
	}
	b.WriteString(`">`)
	for _, p := range c.Parms {
		newline(b, depth+1, indent)
		b.WriteString(`<parm name="`)
		esc(b, p.Name)
		b.WriteByte('"')
		if !p.Flag {
			b.WriteString(` value="`)
			esc(b, p.Value)
			b.WriteByte('"')
		}
		if p.DataType != "" {
			b.WriteString(` datatype="`)
			esc(b, p.DataType)
			b.WriteByte('"')
		}
		b.WriteString(`/>`)
	}
	for i := range c.Children {
		writeCharacteristic(b, &c.Children[i], depth+1, indent)
	}
	newline(b, depth, indent)
	b.WriteString(`</characteristic>`)
}

func newline(b *bytes.Buffer, depth int, indent bool) {
	if !indent {
		return
	}
	b.WriteByte('\n')
	for i := 0; i < depth; i++ {
		b.WriteString("  ")
	}
}

func esc(b *bytes.Buffer, s string) {
	_ = xml.EscapeText(b, []byte(s))
}

// Validate checks the structural rules: a version, a type on every
// characteristic, a name on every parm, and uppercase parm names and
// characteristic types below an APPLICATION characteristic (MS-MDE2 2.2.9.5).
func Validate(d *Document) error {
	if d == nil {
		return fmt.Errorf("%w: nil document", ErrInvalid)
	}
	if d.Version == "" {
		return fmt.Errorf("%w: version is required", ErrInvalid)
	}
	if len(d.Characteristics) == 0 {
		return fmt.Errorf("%w: no characteristics", ErrInvalid)
	}
	for i := range d.Characteristics {
		if err := validateCharacteristic(&d.Characteristics[i], d.Characteristics[i].Type, d.Characteristics[i].Type == TypeApplication); err != nil {
			return err
		}
	}
	return nil
}

func validateCharacteristic(c *Characteristic, path string, upper bool) error {
	if c.Type == "" {
		return fmt.Errorf("%w: characteristic under %q has no type", ErrInvalid, path)
	}
	if upper && c.Type != strings.ToUpper(c.Type) {
		return fmt.Errorf("%w: characteristic %q under APPLICATION must be uppercase", ErrInvalid, path)
	}
	for _, p := range c.Parms {
		if p.Name == "" {
			return fmt.Errorf("%w: parm in %q has no name", ErrInvalid, path)
		}
		if upper && p.Name != strings.ToUpper(p.Name) {
			return fmt.Errorf("%w: parm %q in %q must be uppercase", ErrInvalid, p.Name, path)
		}
	}
	for i := range c.Children {
		child := &c.Children[i]
		if err := validateCharacteristic(child, path+"/"+child.Type, upper); err != nil {
			return err
		}
	}
	return nil
}

type xmlDoc struct {
	XMLName         xml.Name             `xml:"wap-provisioningdoc"`
	Version         string               `xml:"version,attr"`
	Characteristics []xmlCharacteristics `xml:"characteristic"`
}

type xmlCharacteristics struct {
	Type     string               `xml:"type,attr"`
	Parms    []xmlParm            `xml:"parm"`
	Children []xmlCharacteristics `xml:"characteristic"`
}

type xmlParm struct {
	Name     string  `xml:"name,attr"`
	Value    *string `xml:"value,attr"`
	DataType string  `xml:"datatype,attr"`
}

// Decode parses a provisioning document.
func Decode(data []byte) (*Document, error) {
	var x xmlDoc
	if err := xml.Unmarshal(data, &x); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrSyntax, err)
	}
	d := &Document{Version: x.Version, Characteristics: convert(x.Characteristics)}
	if err := Validate(d); err != nil {
		return nil, err
	}
	return d, nil
}

func convert(in []xmlCharacteristics) []Characteristic {
	out := make([]Characteristic, len(in))
	for i, c := range in {
		out[i].Type = c.Type
		for _, p := range c.Parms {
			parm := Parm{Name: p.Name, DataType: p.DataType}
			if p.Value == nil {
				parm.Flag = true
			} else {
				parm.Value = *p.Value
			}
			out[i].Parms = append(out[i].Parms, parm)
		}
		out[i].Children = convert(c.Children)
	}
	return out
}

// Thumbprint returns the SHA-1 hash of a DER certificate as uppercase hex,
// the name Windows gives a certificate under CertificateStore.
func Thumbprint(der []byte) string {
	sum := sha1.Sum(der) //nolint:gosec // store key, not a security control
	return strings.ToUpper(hex.EncodeToString(sum[:]))
}
