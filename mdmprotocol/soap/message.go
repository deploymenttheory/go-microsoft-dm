package soap

import (
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"strings"
)

// Errors returned by this package.
var (
	// ErrSyntax reports XML that is not a SOAP envelope this package reads.
	ErrSyntax = errors.New("soap: malformed envelope")
	// ErrTooLarge reports a request over the size bound.
	ErrTooLarge = errors.New("soap: request too large")
	// ErrEncoding reports a BinarySecurityToken whose EncodingType is not
	// base64 or whose content does not decode.
	ErrEncoding = errors.New("soap: token encoding")
)

// Envelope is a decoded SOAP request whose body is B. The envelope
// namespace is recorded so that a reply can answer in the same version.
type Envelope[B any] struct {
	XMLName xml.Name `xml:"Envelope"`
	Header  Header   `xml:"Header"`
	Body    B        `xml:"Body"`
}

// Header holds the WS-Addressing and WS-Security headers MS-MDE2 profiles.
type Header struct {
	Action    string    `xml:"http://www.w3.org/2005/08/addressing Action"`
	MessageID string    `xml:"http://www.w3.org/2005/08/addressing MessageID"`
	ReplyTo   *ReplyTo  `xml:"http://www.w3.org/2005/08/addressing ReplyTo"`
	To        string    `xml:"http://www.w3.org/2005/08/addressing To"`
	Security  *Security `xml:"http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-secext-1.0.xsd Security"`
}

// ReplyTo is the WS-Addressing reply endpoint.
type ReplyTo struct {
	Address string `xml:"http://www.w3.org/2005/08/addressing Address"`
}

// Security is the wsse:Security header.
type Security struct {
	MustUnderstand      string               `xml:"mustUnderstand,attr"`
	Timestamp           *Timestamp           `xml:"http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-utility-1.0.xsd Timestamp"`
	UsernameToken       *UsernameToken       `xml:"UsernameToken"`
	BinarySecurityToken *BinarySecurityToken `xml:"BinarySecurityToken"`
}

// Timestamp is the u:Timestamp element.
type Timestamp struct {
	ID      string `xml:"http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-utility-1.0.xsd Id,attr"`
	Created string `xml:"http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-utility-1.0.xsd Created"`
	Expires string `xml:"http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-utility-1.0.xsd Expires"`
}

// UsernameToken carries on-premise credentials (WS-Security UsernameToken
// Profile 1.0, MS-MDE2 3.3.4.1.1.1.3).
type UsernameToken struct {
	ID       string   `xml:"http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-utility-1.0.xsd Id,attr"`
	Username string   `xml:"Username"`
	Password Password `xml:"Password"`
}

// Password is the wsse:Password element with its Type attribute.
type Password struct {
	Type  string `xml:"Type,attr"`
	Value string `xml:",chardata"`
}

// BinarySecurityToken is a wsse:BinarySecurityToken in a header or body.
type BinarySecurityToken struct {
	ValueType    string `xml:"ValueType,attr"`
	EncodingType string `xml:"EncodingType,attr"`
	Value        string `xml:",chardata"`
}

// Bytes decodes the token content. The EncodingType must be base64 (or
// empty, which MS-MDE2 examples never send but WS-Security allows to mean
// the same). Whitespace inside the content is ignored because the client
// wraps long tokens.
func (t *BinarySecurityToken) Bytes() ([]byte, error) {
	if t == nil {
		return nil, fmt.Errorf("%w: no token", ErrEncoding)
	}
	if t.EncodingType != "" && t.EncodingType != EncodingBase64 {
		return nil, fmt.Errorf("%w: unsupported EncodingType %q", ErrEncoding, t.EncodingType)
	}
	clean := strings.Map(func(r rune) rune {
		switch r {
		case ' ', '\t', '\r', '\n':
			return -1
		}
		return r
	}, t.Value)
	out, err := base64.StdEncoding.DecodeString(clean)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrEncoding, err)
	}
	return out, nil
}

// Decode parses a SOAP request into an envelope whose body type is B. The
// envelope namespace must be SOAP 1.2 or 1.1; the bound is DefaultMaxSize
// when maxSize is zero.
func Decode[B any](data []byte, maxSize int) (*Envelope[B], error) {
	if maxSize <= 0 {
		maxSize = DefaultMaxSize
	}
	if len(data) > maxSize {
		return nil, fmt.Errorf("%w: %d bytes over %d", ErrTooLarge, len(data), maxSize)
	}
	var env Envelope[B]
	if err := xml.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrSyntax, err)
	}
	switch env.XMLName.Space {
	case NamespaceEnvelope, NamespaceEnvelope11:
	default:
		return nil, fmt.Errorf("%w: envelope namespace %q", ErrSyntax, env.XMLName.Space)
	}
	// Microsoft's own examples wrap header values in whitespace and line
	// breaks; identifiers are compared trimmed. Passwords are never trimmed.
	h := &env.Header
	h.Action, h.MessageID, h.To = strings.TrimSpace(h.Action), strings.TrimSpace(h.MessageID), strings.TrimSpace(h.To)
	if h.ReplyTo != nil {
		h.ReplyTo.Address = strings.TrimSpace(h.ReplyTo.Address)
	}
	if h.Security != nil && h.Security.UsernameToken != nil {
		h.Security.UsernameToken.Username = strings.TrimSpace(h.Security.UsernameToken.Username)
	}
	return &env, nil
}

// Version reports which SOAP envelope namespace the request used.
func (e *Envelope[B]) Version() string {
	if e.XMLName.Space == NamespaceEnvelope11 {
		return NamespaceEnvelope11
	}
	return NamespaceEnvelope
}
