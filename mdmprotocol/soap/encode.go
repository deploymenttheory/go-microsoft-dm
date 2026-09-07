package soap

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"time"
)

// ErrResponse reports a response that cannot be written.
var ErrResponse = errors.New("soap: invalid response")

// Response is a reply envelope. Body is the already-encoded body element,
// written verbatim inside s:Body; callers produce it with the writers in
// their own packages so that this package never sees message-specific
// types.
type Response struct {
	// Action is the WS-Addressing reply action; required.
	Action string
	// RelatesTo echoes the request MessageID; required. MS-MDE2 examples
	// show a fault with the literal "invalid_message_id"-style placeholder
	// when the request had none; callers decide what to put here.
	RelatesTo string
	// ActivityID, when set, adds the ActivityId diagnostics header Microsoft's
	// service emits, with CorrelationId equal to it.
	ActivityID string
	// Timestamp, when set, adds an o:Security header holding a u:Timestamp
	// with Created and Expires, as the RequestSecurityTokenResponse example
	// in MS-MDE2 4.3.2 does.
	Timestamp *TimestampRange
	// SchemaAttributes adds xmlns:xsi and xmlns:xsd on s:Body, as the
	// Discover and GetPolicies responses in MS-MDE2 section 4 do.
	SchemaAttributes bool
	// Body is the encoded body element.
	Body []byte
}

// TimestampRange is the interval a u:Timestamp header states.
type TimestampRange struct {
	Created time.Time
	Expires time.Time
}

// Encode writes the response as a SOAP 1.2 envelope with the s, a and u
// prefixes of the MS-MDE2 examples.
func Encode(r Response) ([]byte, error) {
	if r.Action == "" {
		return nil, fmt.Errorf("%w: Action is required", ErrResponse)
	}
	if r.RelatesTo == "" {
		return nil, fmt.Errorf("%w: RelatesTo is required", ErrResponse)
	}
	if len(bytes.TrimSpace(r.Body)) == 0 {
		return nil, fmt.Errorf("%w: Body is required", ErrResponse)
	}
	var b bytes.Buffer
	b.WriteString(`<s:Envelope xmlns:s="` + NamespaceEnvelope + `" xmlns:a="` + NamespaceAddressing + `"`)
	if r.Timestamp != nil {
		b.WriteString(` xmlns:u="` + NamespaceUtility + `"`)
	}
	b.WriteString(`><s:Header><a:Action s:mustUnderstand="1">`)
	escape(&b, r.Action)
	b.WriteString(`</a:Action>`)
	if r.ActivityID != "" {
		b.WriteString(`<ActivityId CorrelationId="`)
		escape(&b, r.ActivityID)
		b.WriteString(`" xmlns="` + NamespaceDiagnostics + `">`)
		escape(&b, r.ActivityID)
		b.WriteString(`</ActivityId>`)
	}
	b.WriteString(`<a:RelatesTo>`)
	escape(&b, r.RelatesTo)
	b.WriteString(`</a:RelatesTo>`)
	if r.Timestamp != nil {
		b.WriteString(`<o:Security s:mustUnderstand="1" xmlns:o="` + NamespaceSecurity + `"><u:Timestamp u:Id="_0"><u:Created>`)
		b.WriteString(r.Timestamp.Created.UTC().Format(timestampLayout))
		b.WriteString(`</u:Created><u:Expires>`)
		b.WriteString(r.Timestamp.Expires.UTC().Format(timestampLayout))
		b.WriteString(`</u:Expires></u:Timestamp></o:Security>`)
	}
	b.WriteString(`</s:Header><s:Body`)
	if r.SchemaAttributes {
		b.WriteString(` xmlns:xsi="` + NamespaceSchemaInstance + `" xmlns:xsd="` + NamespaceSchema + `"`)
	}
	b.WriteString(`>`)
	b.Write(r.Body)
	b.WriteString(`</s:Body></s:Envelope>`)
	return b.Bytes(), nil
}

// timestampLayout is the millisecond UTC form of the MS-MDE2 examples.
const timestampLayout = "2006-01-02T15:04:05.000Z"

func escape(b *bytes.Buffer, s string) {
	// EscapeText never fails on a bytes.Buffer.
	_ = xml.EscapeText(b, []byte(s))
}

// Element is a helper for body writers: it writes <name attrs>text</name>
// with escaping, or an empty element when text is empty and empty is set.
type Element struct {
	Name  string
	Attrs [][2]string
	Text  string
	// Empty writes <name/> when Text is empty.
	Empty bool
}

// Write writes the element to b.
func (e Element) Write(b *bytes.Buffer) {
	b.WriteByte('<')
	b.WriteString(e.Name)
	for _, a := range e.Attrs {
		b.WriteByte(' ')
		b.WriteString(a[0])
		b.WriteString(`="`)
		escape(b, a[1])
		b.WriteByte('"')
	}
	if e.Text == "" && e.Empty {
		b.WriteString("/>")
		return
	}
	b.WriteByte('>')
	escape(b, e.Text)
	b.WriteString("</")
	b.WriteString(e.Name)
	b.WriteByte('>')
}
