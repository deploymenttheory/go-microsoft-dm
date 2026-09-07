package enroll

import (
	"bytes"
	"fmt"
	"strconv"
	"time"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/soap"
)

// GetPoliciesRequest is the MS-XCEP GetPolicies body as MS-MDE2 3.3.4.1.1.1
// profiles it: every field nil. The struct exists so that a request can be
// recognised and future fields carried.
type GetPoliciesRequest struct {
	// LastUpdate and PreferredLanguage are nil in MDE2 requests; they are
	// kept when a client sends them.
	LastUpdate        string
	PreferredLanguage string
}

// PolicyResponse is the MS-XCEP GetPoliciesResponse as the Windows
// enrollment client consumes it (MS-MDE2 3.3.4.1.1.2): one policy with the
// key length, validity, hash algorithm and crypto providers the client
// applies when it builds its certificate request.
type PolicyResponse struct {
	// PolicyID is the response policyID; Microsoft's example leaves it empty.
	PolicyID string
	// CommonName names the policy in the client's log.
	CommonName string
	// PolicyOIDReference and HashAlgorithmOIDReference index OIDs; both 0.
	PolicyOIDReference int
	// Validity and Renewal become validityPeriodSeconds and
	// renewalPeriodSeconds. Both required.
	Validity time.Duration
	Renewal  time.Duration
	// Enroll and AutoEnroll are the permission flags.
	Enroll     bool
	AutoEnroll bool
	// MinimalKeyLength is the RSA key size the client generates.
	MinimalKeyLength int
	// CryptoProviders are the CNG providers the client may use, in
	// preference order. Empty omits the element.
	CryptoProviders []string
	// MajorRevision and MinorRevision are the template revision.
	MajorRevision int
	MinorRevision int
	// HashAlgorithm is the hash OID the client signs with, with its
	// display name.
	HashAlgorithm OID
}

// OID is an MS-XCEP oID entry.
type OID struct {
	Value string
	// Group is the OID group; 4 is hash algorithms.
	Group       int
	ReferenceID int
	DefaultName string
}

// SHA256 is the hash OID Microsoft's example returns.
var SHA256 = OID{Value: "2.16.840.1.101.3.4.2.1", Group: 4, ReferenceID: 0, DefaultName: "szOID_NIST_sha256"}

// DefaultPolicy is the response a server returns when nothing else is
// configured: RSA 2048, SHA-256, a one-year certificate renewable in its
// last sixty days, and the two providers Windows offers.
func DefaultPolicy() *PolicyResponse {
	return &PolicyResponse{
		CommonName: "go-microsoft-dm", Validity: 365 * 24 * time.Hour, Renewal: 60 * 24 * time.Hour,
		Enroll: true, MinimalKeyLength: 2048,
		CryptoProviders: []string{"Microsoft Platform Crypto Provider", "Microsoft Software Key Storage Provider"},
		MajorRevision:   101, HashAlgorithm: SHA256,
	}
}

// Validate applies the constraints a usable policy needs.
func (p *PolicyResponse) Validate() error {
	switch {
	case p == nil:
		return fmt.Errorf("%w: nil policy", ErrMessage)
	case p.Validity <= 0 || p.Renewal <= 0:
		return fmt.Errorf("%w: validity and renewal periods must be positive", ErrMessage)
	case p.Renewal >= p.Validity:
		return fmt.Errorf("%w: renewal period must be shorter than validity", ErrMessage)
	case p.MinimalKeyLength < 2048:
		return fmt.Errorf("%w: minimalKeyLength %d is below 2048", ErrMessage, p.MinimalKeyLength)
	case p.HashAlgorithm.Value == "":
		return fmt.Errorf("%w: hash algorithm OID is required", ErrMessage)
	}
	return nil
}

type getPoliciesBody struct {
	GetPolicies *struct {
		Client struct {
			LastUpdate        string `xml:"lastUpdate"`
			PreferredLanguage string `xml:"preferredLanguage"`
		} `xml:"client"`
	} `xml:"http://schemas.microsoft.com/windows/pki/2009/01/enrollmentpolicy GetPolicies"`
}

// DecodeGetPolicies parses a GetPolicies request envelope.
func DecodeGetPolicies(data []byte, maxSize int) (*soap.Header, *GetPoliciesRequest, error) {
	env, err := soap.Decode[getPoliciesBody](data, maxSize)
	if err != nil {
		return nil, nil, err
	}
	if env.Body.GetPolicies == nil {
		return &env.Header, nil, fmt.Errorf("%w: no GetPolicies element", ErrMessage)
	}
	c := env.Body.GetPolicies.Client
	return &env.Header, &GetPoliciesRequest{LastUpdate: c.LastUpdate, PreferredLanguage: c.PreferredLanguage}, nil
}

// EncodeGetPolicies writes a GetPolicies request envelope, for clients.
func EncodeGetPolicies(messageID, to string, creds *Credentials) ([]byte, error) {
	body := `<GetPolicies xmlns="` + NamespacePolicy + `"><client><lastUpdate xsi:nil="true"/><preferredLanguage xsi:nil="true"/></client><requestFilter xsi:nil="true"/></GetPolicies>`
	return encodeRequest(ActionGetPolicies, messageID, to, creds, []byte(body))
}

func nilElement(b *bytes.Buffer, name string) {
	b.WriteString(`<` + name + ` xsi:nil="true"/>`)
}

func intElement(b *bytes.Buffer, name string, n int) {
	soap.Element{Name: name, Text: strconv.Itoa(n)}.Write(b)
}

// EncodePolicyResponse writes the GetPoliciesResponse of MS-MDE2 4.2.2.
func EncodePolicyResponse(p *PolicyResponse, relatesTo string) ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	var b bytes.Buffer
	b.WriteString(`<GetPoliciesResponse xmlns="` + NamespacePolicy + `"><response>`)
	soap.Element{Name: "policyID", Text: p.PolicyID, Empty: true}.Write(&b)
	nilElement(&b, "policyFriendlyName")
	nilElement(&b, "nextUpdateHours")
	nilElement(&b, "policiesNotChanged")
	b.WriteString(`<policies><policy>`)
	intElement(&b, "policyOIDReference", p.PolicyOIDReference)
	nilElement(&b, "cAs")
	b.WriteString(`<attributes>`)
	soap.Element{Name: "commonName", Text: p.CommonName}.Write(&b)
	b.WriteString(`<policySchema>3</policySchema><certificateValidity>`)
	intElement(&b, "validityPeriodSeconds", int(p.Validity/time.Second))
	intElement(&b, "renewalPeriodSeconds", int(p.Renewal/time.Second))
	b.WriteString(`</certificateValidity><permission>`)
	soap.Element{Name: "enroll", Text: strconv.FormatBool(p.Enroll)}.Write(&b)
	soap.Element{Name: "autoEnroll", Text: strconv.FormatBool(p.AutoEnroll)}.Write(&b)
	b.WriteString(`</permission><privateKeyAttributes>`)
	intElement(&b, "minimalKeyLength", p.MinimalKeyLength)
	nilElement(&b, "keySpec")
	nilElement(&b, "keyUsageProperty")
	nilElement(&b, "permissions")
	nilElement(&b, "algorithmOIDReference")
	if len(p.CryptoProviders) > 0 {
		b.WriteString(`<cryptoProviders>`)
		for _, prov := range p.CryptoProviders {
			soap.Element{Name: "provider", Text: prov}.Write(&b)
		}
		b.WriteString(`</cryptoProviders>`)
	}
	b.WriteString(`</privateKeyAttributes><revision>`)
	intElement(&b, "majorRevision", p.MajorRevision)
	intElement(&b, "minorRevision", p.MinorRevision)
	b.WriteString(`</revision>`)
	for _, name := range []string{"supersededPolicies", "privateKeyFlags", "subjectNameFlags", "enrollmentFlags", "generalFlags"} {
		nilElement(&b, name)
	}
	intElement(&b, "hashAlgorithmOIDReference", p.HashAlgorithm.ReferenceID)
	for _, name := range []string{"rARequirements", "keyArchivalAttributes", "extensions"} {
		nilElement(&b, name)
	}
	b.WriteString(`</attributes></policy></policies></response>`)
	nilElement(&b, "cAs")
	b.WriteString(`<oIDs><oID>`)
	soap.Element{Name: "value", Text: p.HashAlgorithm.Value}.Write(&b)
	intElement(&b, "group", p.HashAlgorithm.Group)
	intElement(&b, "oIDReferenceID", p.HashAlgorithm.ReferenceID)
	soap.Element{Name: "defaultName", Text: p.HashAlgorithm.DefaultName}.Write(&b)
	b.WriteString(`</oID></oIDs></GetPoliciesResponse>`)
	return soap.Encode(soap.Response{Action: ActionGetPoliciesResponse, RelatesTo: relatesTo, SchemaAttributes: true, Body: b.Bytes()})
}

type policyResponseBody struct {
	Response *struct {
		Response struct {
			PolicyID string `xml:"policyID"`
			Policies struct {
				Policy struct {
					PolicyOIDReference int `xml:"policyOIDReference"`
					Attributes         struct {
						CommonName          string `xml:"commonName"`
						CertificateValidity struct {
							ValidityPeriodSeconds int `xml:"validityPeriodSeconds"`
							RenewalPeriodSeconds  int `xml:"renewalPeriodSeconds"`
						} `xml:"certificateValidity"`
						Permission struct {
							Enroll     bool `xml:"enroll"`
							AutoEnroll bool `xml:"autoEnroll"`
						} `xml:"permission"`
						PrivateKeyAttributes struct {
							MinimalKeyLength int      `xml:"minimalKeyLength"`
							CryptoProviders  []string `xml:"cryptoProviders>provider"`
						} `xml:"privateKeyAttributes"`
						Revision struct {
							Major int `xml:"majorRevision"`
							Minor int `xml:"minorRevision"`
						} `xml:"revision"`
						HashAlgorithmOIDReference int `xml:"hashAlgorithmOIDReference"`
					} `xml:"attributes"`
				} `xml:"policy"`
			} `xml:"policies"`
		} `xml:"response"`
		OIDs []struct {
			Value       string `xml:"value"`
			Group       int    `xml:"group"`
			ReferenceID int    `xml:"oIDReferenceID"`
			DefaultName string `xml:"defaultName"`
		} `xml:"oIDs>oID"`
	} `xml:"http://schemas.microsoft.com/windows/pki/2009/01/enrollmentpolicy GetPoliciesResponse"`
}

// DecodePolicyResponse reads a GetPoliciesResponse, for clients. A fault
// envelope is returned as a *soap.DecodedFault error.
func DecodePolicyResponse(data []byte) (*soap.Header, *PolicyResponse, error) {
	if f, err := soap.DecodeFault(data); err != nil {
		return nil, nil, err
	} else if f != nil {
		return nil, nil, f
	}
	env, err := soap.Decode[policyResponseBody](data, 0)
	if err != nil {
		return nil, nil, err
	}
	if env.Body.Response == nil {
		return &env.Header, nil, fmt.Errorf("%w: no GetPoliciesResponse element", ErrMessage)
	}
	r := env.Body.Response
	a := r.Response.Policies.Policy.Attributes
	p := &PolicyResponse{
		PolicyID: r.Response.PolicyID, CommonName: a.CommonName, PolicyOIDReference: r.Response.Policies.Policy.PolicyOIDReference,
		Validity: time.Duration(a.CertificateValidity.ValidityPeriodSeconds) * time.Second,
		Renewal:  time.Duration(a.CertificateValidity.RenewalPeriodSeconds) * time.Second,
		Enroll:   a.Permission.Enroll, AutoEnroll: a.Permission.AutoEnroll,
		MinimalKeyLength: a.PrivateKeyAttributes.MinimalKeyLength, CryptoProviders: a.PrivateKeyAttributes.CryptoProviders,
		MajorRevision: a.Revision.Major, MinorRevision: a.Revision.Minor,
	}
	for _, o := range r.OIDs {
		if o.ReferenceID == a.HashAlgorithmOIDReference {
			p.HashAlgorithm = OID{Value: o.Value, Group: o.Group, ReferenceID: o.ReferenceID, DefaultName: o.DefaultName}
		}
	}
	return &env.Header, p, nil
}
