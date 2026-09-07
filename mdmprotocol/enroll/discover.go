package enroll

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/soap"
)

// ErrMessage reports a request body that does not match the operation.
var ErrMessage = errors.New("enroll: invalid message")

// DiscoverRequest is the DiscoveryRequest of MS-MDE2 3.1.4.1.3.1.
type DiscoverRequest struct {
	EmailAddress       string
	RequestVersion     string
	DeviceType         DeviceType
	ApplicationVersion string
	OSEdition          string
	AuthPolicies       []AuthPolicy
}

// DiscoverResponse is the DiscoveryResponse of MS-MDE2 3.1.4.1.3.2.
type DiscoverResponse struct {
	AuthPolicy                 AuthPolicy
	EnrollmentVersion          string
	EnrollmentPolicyServiceURL string
	EnrollmentServiceURL       string
	AuthenticationServiceURL   string
	// DeviceAssociationMaaURL and GatewayService are parsed and written but
	// unused in this phase.
	DeviceAssociationMaaURL string
	GatewayService          string
}

type discoverBody struct {
	Discover *struct {
		Request struct {
			EmailAddress       string `xml:"EmailAddress"`
			RequestVersion     string `xml:"RequestVersion"`
			DeviceType         string `xml:"DeviceType"`
			ApplicationVersion string `xml:"ApplicationVersion"`
			OSEdition          string `xml:"OSEdition"`
			AuthPolicies       struct {
				AuthPolicy []string `xml:"AuthPolicy"`
			} `xml:"AuthPolicies"`
		} `xml:"request"`
	} `xml:"http://schemas.microsoft.com/windows/management/2012/01/enrollment Discover"`
}

// DecodeDiscover parses a Discover request envelope.
func DecodeDiscover(data []byte, maxSize int) (*soap.Header, *DiscoverRequest, error) {
	env, err := soap.Decode[discoverBody](data, maxSize)
	if err != nil {
		return nil, nil, err
	}
	if env.Body.Discover == nil {
		return &env.Header, nil, fmt.Errorf("%w: no Discover element", ErrMessage)
	}
	r := env.Body.Discover.Request
	req := &DiscoverRequest{
		EmailAddress:       strings.TrimSpace(r.EmailAddress),
		RequestVersion:     strings.TrimSpace(r.RequestVersion),
		DeviceType:         DeviceType(strings.TrimSpace(r.DeviceType)),
		ApplicationVersion: strings.TrimSpace(r.ApplicationVersion),
		OSEdition:          strings.TrimSpace(r.OSEdition),
	}
	for _, p := range r.AuthPolicies.AuthPolicy {
		req.AuthPolicies = append(req.AuthPolicies, AuthPolicy(strings.TrimSpace(p)))
	}
	return &env.Header, req, nil
}

// Supports reports whether the client listed the policy.
func (r *DiscoverRequest) Supports(p AuthPolicy) bool {
	for _, q := range r.AuthPolicies {
		if q == p {
			return true
		}
	}
	return false
}

// EncodeDiscoverResponse writes the DiscoverResponse body inside a reply
// envelope that relates to the request.
func EncodeDiscoverResponse(resp *DiscoverResponse, relatesTo, activityID string) ([]byte, error) {
	if resp == nil || !resp.AuthPolicy.Valid() {
		return nil, fmt.Errorf("%w: AuthPolicy is required", ErrMessage)
	}
	if resp.EnrollmentServiceURL == "" {
		return nil, fmt.Errorf("%w: EnrollmentServiceUrl is required", ErrMessage)
	}
	var b bytes.Buffer
	b.WriteString(`<DiscoverResponse xmlns="` + NamespaceDiscovery + `"><DiscoverResult>`)
	soap.Element{Name: "AuthPolicy", Text: string(resp.AuthPolicy)}.Write(&b)
	if resp.EnrollmentVersion != "" {
		soap.Element{Name: "EnrollmentVersion", Text: resp.EnrollmentVersion}.Write(&b)
	}
	if resp.EnrollmentPolicyServiceURL != "" {
		soap.Element{Name: "EnrollmentPolicyServiceUrl", Text: resp.EnrollmentPolicyServiceURL}.Write(&b)
	}
	soap.Element{Name: "EnrollmentServiceUrl", Text: resp.EnrollmentServiceURL}.Write(&b)
	if resp.AuthenticationServiceURL != "" {
		soap.Element{Name: "AuthenticationServiceUrl", Text: resp.AuthenticationServiceURL}.Write(&b)
	}
	if resp.DeviceAssociationMaaURL != "" {
		soap.Element{Name: "DeviceAssociationMaaUrl", Text: resp.DeviceAssociationMaaURL}.Write(&b)
	}
	if resp.GatewayService != "" {
		soap.Element{Name: "GatewayService", Text: resp.GatewayService}.Write(&b)
	}
	b.WriteString(`</DiscoverResult></DiscoverResponse>`)
	return soap.Encode(soap.Response{
		Action: ActionDiscoverResponse, RelatesTo: relatesTo, ActivityID: activityID,
		SchemaAttributes: true, Body: b.Bytes(),
	})
}

// EncodeDiscover writes a Discover request envelope, for clients.
func EncodeDiscover(req *DiscoverRequest, messageID, to string) ([]byte, error) {
	if req == nil || req.EmailAddress == "" || req.RequestVersion == "" || len(req.AuthPolicies) == 0 {
		return nil, fmt.Errorf("%w: EmailAddress, RequestVersion and AuthPolicies are required", ErrMessage)
	}
	var b bytes.Buffer
	b.WriteString(`<Discover xmlns="` + NamespaceDiscovery + `"><request xmlns:i="` + soap.NamespaceSchemaInstance + `">`)
	soap.Element{Name: "EmailAddress", Text: req.EmailAddress}.Write(&b)
	soap.Element{Name: "OSEdition", Text: req.OSEdition}.Write(&b)
	soap.Element{Name: "RequestVersion", Text: req.RequestVersion}.Write(&b)
	soap.Element{Name: "DeviceType", Text: string(req.DeviceType)}.Write(&b)
	soap.Element{Name: "ApplicationVersion", Text: req.ApplicationVersion}.Write(&b)
	b.WriteString(`<AuthPolicies>`)
	for _, p := range req.AuthPolicies {
		soap.Element{Name: "AuthPolicy", Text: string(p)}.Write(&b)
	}
	b.WriteString(`</AuthPolicies></request></Discover>`)
	return encodeRequest(ActionDiscover, messageID, to, nil, b.Bytes())
}

type discoverResponseBody struct {
	Response *struct {
		Result struct {
			AuthPolicy                 string `xml:"AuthPolicy"`
			EnrollmentVersion          string `xml:"EnrollmentVersion"`
			EnrollmentPolicyServiceURL string `xml:"EnrollmentPolicyServiceUrl"`
			EnrollmentServiceURL       string `xml:"EnrollmentServiceUrl"`
			AuthenticationServiceURL   string `xml:"AuthenticationServiceUrl"`
			DeviceAssociationMaaURL    string `xml:"DeviceAssociationMaaUrl"`
			GatewayService             string `xml:"GatewayService"`
		} `xml:"DiscoverResult"`
	} `xml:"http://schemas.microsoft.com/windows/management/2012/01/enrollment DiscoverResponse"`
}

// DecodeDiscoverResponse reads a DiscoverResponse envelope, for clients. A
// fault envelope is returned as a *soap.DecodedFault error.
func DecodeDiscoverResponse(data []byte) (*soap.Header, *DiscoverResponse, error) {
	if f, err := soap.DecodeFault(data); err != nil {
		return nil, nil, err
	} else if f != nil {
		return nil, nil, f
	}
	env, err := soap.Decode[discoverResponseBody](data, 0)
	if err != nil {
		return nil, nil, err
	}
	if env.Body.Response == nil {
		return &env.Header, nil, fmt.Errorf("%w: no DiscoverResponse element", ErrMessage)
	}
	r := env.Body.Response.Result
	return &env.Header, &DiscoverResponse{
		AuthPolicy:                 AuthPolicy(strings.TrimSpace(r.AuthPolicy)),
		EnrollmentVersion:          strings.TrimSpace(r.EnrollmentVersion),
		EnrollmentPolicyServiceURL: strings.TrimSpace(r.EnrollmentPolicyServiceURL),
		EnrollmentServiceURL:       strings.TrimSpace(r.EnrollmentServiceURL),
		AuthenticationServiceURL:   strings.TrimSpace(r.AuthenticationServiceURL),
		DeviceAssociationMaaURL:    strings.TrimSpace(r.DeviceAssociationMaaURL),
		GatewayService:             strings.TrimSpace(r.GatewayService),
	}, nil
}

// Credentials are on-premise credentials for a request header.
type Credentials struct {
	Username string
	Password string
}

// encodeRequest writes a request envelope with the WS-Addressing headers
// every MS-MDE2 request carries and, when creds is set, the on-premise
// UsernameToken.
func encodeRequest(action, messageID, to string, creds *Credentials, body []byte) ([]byte, error) {
	if messageID == "" || to == "" {
		return nil, fmt.Errorf("%w: MessageID and To are required", ErrMessage)
	}
	var b bytes.Buffer
	b.WriteString(`<s:Envelope xmlns:s="` + soap.NamespaceEnvelope + `" xmlns:a="` + soap.NamespaceAddressing + `"`)
	b.WriteString(` xmlns:u="` + soap.NamespaceUtility + `" xmlns:wsse="` + soap.NamespaceSecurity + `"`)
	b.WriteString(` xmlns:wst="` + soap.NamespaceTrust + `" xmlns:ac="` + soap.NamespaceAuthorization + `">`)
	b.WriteString(`<s:Header>`)
	soap.Element{Name: "a:Action", Attrs: [][2]string{{"s:mustUnderstand", "1"}}, Text: action}.Write(&b)
	soap.Element{Name: "a:MessageID", Text: messageID}.Write(&b)
	b.WriteString(`<a:ReplyTo><a:Address>` + soap.AnonymousAddress + `</a:Address></a:ReplyTo>`)
	soap.Element{Name: "a:To", Attrs: [][2]string{{"s:mustUnderstand", "1"}}, Text: to}.Write(&b)
	if creds != nil {
		b.WriteString(`<wsse:Security s:mustUnderstand="1"><wsse:UsernameToken u:Id="` + soap.UsernameTokenID + `">`)
		soap.Element{Name: "wsse:Username", Text: creds.Username}.Write(&b)
		soap.Element{Name: "wsse:Password", Attrs: [][2]string{{"wsse:Type", soap.PasswordText}}, Text: creds.Password}.Write(&b)
		b.WriteString(`</wsse:UsernameToken></wsse:Security>`)
	}
	b.WriteString(`</s:Header><s:Body xmlns:xsi="` + soap.NamespaceSchemaInstance + `" xmlns:xsd="` + soap.NamespaceSchema + `">`)
	b.Write(body)
	b.WriteString(`</s:Body></s:Envelope>`)
	return b.Bytes(), nil
}
