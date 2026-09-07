package enroll

import (
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"strings"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/soap"
)

// ContextItem is one ac:ContextItem as sent.
type ContextItem struct {
	Name  string
	Value string
}

// AdditionalContext is the typed view of the ac:AdditionalContext items
// MS-MDE2 3.4.4.1.1.1 catalogues. Values are kept as the client wrote them,
// trimmed; booleans are parsed case-insensitively because the client writes
// both "true" and "True". Items the specification does not name are kept in
// Unknown, and Items holds every item in wire order for policy hooks that
// want the raw view.
type AdditionalContext struct {
	OSEdition          string
	OSVersion          string
	DeviceName         string
	EnrollmentType     EnrollmentType
	DeviceType         DeviceType
	ApplicationVersion string
	DeviceID           string
	EnrollmentData     string
	// MAC and IMEI repeat once per interface or radio.
	MAC                  []string
	IMEI                 []string
	TargetedUserLoggedIn bool

	// Implementation-specific items (MS-MDE2 3.4.4.1.1.1 footnotes).
	Locale                               string
	HWDevID                              string
	BulkAADJ                             bool
	ZeroTouchProvisioning                string
	OfflineAutoPilotEnrollmentCorrelator string
	UXInitiated                          bool
	ExternalMgmtAgentHint                string
	DomainName                           string
	BootstrapDomainJoin                  bool
	PlugandForget                        bool
	WhiteGlove                           bool
	WhiteGloveHybridJoin                 bool
	NotInOobe                            bool

	// Attestation items, EnrollmentVersion 5.0 and later.
	AIKAttestationClaim               string
	AIKPub                            string
	AIKCert                           string
	AadAIKAttestationClaim            string
	AADPub                            string
	EmmDeviceID                       string
	RequestVersion                    string
	AzureAttestationBlob              string
	AttestationStatus                 string
	AttestationStatusHResult          string
	AzureAttestationCorrelationVector string
	AIKAlgorithm                      string

	Unknown []ContextItem
	Items   []ContextItem
}

// Get returns the first item with the name, for callers that want the raw
// value of any item including unknown ones.
func (c *AdditionalContext) Get(name string) (string, bool) {
	for _, it := range c.Items {
		if it.Name == name {
			return it.Value, true
		}
	}
	return "", false
}

// ParseAdditionalContext builds the typed view from items in wire order.
func ParseAdditionalContext(items []ContextItem) AdditionalContext {
	var c AdditionalContext
	c.Items = make([]ContextItem, 0, len(items))
	for _, raw := range items {
		it := ContextItem{Name: strings.TrimSpace(raw.Name), Value: strings.TrimSpace(raw.Value)}
		c.Items = append(c.Items, it)
		v := it.Value
		switch it.Name {
		case "OSEdition":
			c.OSEdition = v
		case "OSVersion":
			c.OSVersion = v
		case "DeviceName":
			c.DeviceName = v
		case "EnrollmentType":
			c.EnrollmentType = EnrollmentType(v)
		case "DeviceType":
			c.DeviceType = DeviceType(v)
		case "ApplicationVersion":
			c.ApplicationVersion = v
		case "DeviceID":
			c.DeviceID = v
		case "EnrollmentData":
			c.EnrollmentData = v
		case "MAC":
			c.MAC = append(c.MAC, v)
		case "IMEI":
			c.IMEI = append(c.IMEI, v)
		case "TargetedUserLoggedIn":
			c.TargetedUserLoggedIn = isTrue(v)
		case "Locale":
			c.Locale = v
		case "HWDevID":
			c.HWDevID = v
		case "BulkAADJ":
			c.BulkAADJ = isTrue(v)
		case "ZeroTouchProvisioning":
			c.ZeroTouchProvisioning = v
		case "OfflineAutoPilotEnrollmentCorrelator":
			c.OfflineAutoPilotEnrollmentCorrelator = v
		case "UXInitiated":
			c.UXInitiated = isTrue(v)
		case "ExternalMgmtAgentHint":
			c.ExternalMgmtAgentHint = v
		case "DomainName":
			c.DomainName = v
		case "BootstrapDomainJoin":
			c.BootstrapDomainJoin = isTrue(v)
		case "PlugandForget":
			c.PlugandForget = isTrue(v)
		case "WhiteGlove":
			c.WhiteGlove = isTrue(v)
		case "WhiteGloveHybridJoin":
			c.WhiteGloveHybridJoin = isTrue(v)
		case "NotInOobe":
			c.NotInOobe = isTrue(v)
		case "AIKAttestationClaim":
			c.AIKAttestationClaim = v
		case "AIKPub":
			c.AIKPub = v
		case "AIKCert":
			c.AIKCert = v
		case "AadAIKAttestationClaim":
			c.AadAIKAttestationClaim = v
		case "AADPub":
			c.AADPub = v
		case "EmmDeviceId":
			c.EmmDeviceID = v
		case "RequestVersion":
			c.RequestVersion = v
		case "AzureAttestationBlob":
			c.AzureAttestationBlob = v
		case "AttestationStatus":
			c.AttestationStatus = v
		case "AttestationStatusHResult":
			c.AttestationStatusHResult = v
		case "AzureAttestationCorrelationVector":
			c.AzureAttestationCorrelationVector = v
		case "AIKAlgorithm":
			c.AIKAlgorithm = v
		default:
			c.Unknown = append(c.Unknown, it)
		}
	}
	return c
}

func isTrue(v string) bool { return strings.EqualFold(v, "true") }

// RequestSecurityToken is the wst:RequestSecurityToken body of MS-MDE2
// 3.4.4.1.1.1.
type RequestSecurityToken struct {
	TokenType   string
	RequestType string
	// Token is the BinarySecurityToken carrying the PKCS#10 request (or,
	// for renewal, a PKCS#7).
	Token   soap.BinarySecurityToken
	Context AdditionalContext
}

type rstBody struct {
	RST *struct {
		TokenType   string `xml:"TokenType"`
		RequestType string `xml:"RequestType"`
		Token       *struct {
			ValueType    string `xml:"ValueType,attr"`
			EncodingType string `xml:"EncodingType,attr"`
			Value        string `xml:",chardata"`
		} `xml:"http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-secext-1.0.xsd BinarySecurityToken"`
		Context *struct {
			Items []struct {
				Name  string `xml:"Name,attr"`
				Value string `xml:"Value"`
			} `xml:"ContextItem"`
		} `xml:"http://schemas.xmlsoap.org/ws/2006/12/authorization AdditionalContext"`
	} `xml:"http://docs.oasis-open.org/ws-sx/ws-trust/200512 RequestSecurityToken"`
}

// DecodeRequestSecurityToken parses a RequestSecurityToken envelope.
func DecodeRequestSecurityToken(data []byte, maxSize int) (*soap.Header, *RequestSecurityToken, error) {
	env, err := soap.Decode[rstBody](data, maxSize)
	if err != nil {
		return nil, nil, err
	}
	if env.Body.RST == nil {
		return &env.Header, nil, fmt.Errorf("%w: no RequestSecurityToken element", ErrMessage)
	}
	r := env.Body.RST
	rst := &RequestSecurityToken{TokenType: strings.TrimSpace(r.TokenType), RequestType: strings.TrimSpace(r.RequestType)}
	if r.Token != nil {
		rst.Token = soap.BinarySecurityToken{
			ValueType: strings.TrimSpace(r.Token.ValueType), EncodingType: strings.TrimSpace(r.Token.EncodingType), Value: r.Token.Value,
		}
	}
	if r.Context != nil {
		items := make([]ContextItem, 0, len(r.Context.Items))
		for _, it := range r.Context.Items {
			items = append(items, ContextItem{Name: it.Name, Value: it.Value})
		}
		rst.Context = ParseAdditionalContext(items)
	}
	return &env.Header, rst, nil
}

// EncodeRequestSecurityToken writes a RequestSecurityToken envelope, for
// clients. csr is the DER PKCS#10; items are written in order.
func EncodeRequestSecurityToken(messageID, to string, creds *Credentials, csr []byte, items []ContextItem) ([]byte, error) {
	if len(csr) == 0 {
		return nil, fmt.Errorf("%w: certificate request is required", ErrMessage)
	}
	var b bytes.Buffer
	b.WriteString(`<wst:RequestSecurityToken>`)
	soap.Element{Name: "wst:TokenType", Text: TokenTypeDeviceEnrollment}.Write(&b)
	soap.Element{Name: "wst:RequestType", Text: RequestTypeIssue}.Write(&b)
	soap.Element{Name: "wsse:BinarySecurityToken", Attrs: [][2]string{
		{"ValueType", ValueTypePKCS10}, {"EncodingType", soap.EncodingBase64},
	}, Text: base64.StdEncoding.EncodeToString(csr)}.Write(&b)
	b.WriteString(`<ac:AdditionalContext xmlns="` + soap.NamespaceAuthorization + `">`)
	for _, it := range items {
		b.WriteString(`<ac:ContextItem Name="`)
		_ = xml.EscapeText(&b, []byte(it.Name))
		b.WriteString(`">`)
		soap.Element{Name: "ac:Value", Text: it.Value}.Write(&b)
		b.WriteString(`</ac:ContextItem>`)
	}
	b.WriteString(`</ac:AdditionalContext></wst:RequestSecurityToken>`)
	return encodeRequest(ActionRST, messageID, to, creds, b.Bytes())
}

// EncodeTokenResponse writes the RequestSecurityTokenResponseCollection of
// MS-MDE2 3.4.4.1.1.2 carrying the provisioning document.
func EncodeTokenResponse(provisioningDoc []byte, relatesTo string, ts *soap.TimestampRange) ([]byte, error) {
	if len(bytes.TrimSpace(provisioningDoc)) == 0 {
		return nil, fmt.Errorf("%w: provisioning document is required", ErrMessage)
	}
	var b bytes.Buffer
	b.WriteString(`<RequestSecurityTokenResponseCollection xmlns="` + soap.NamespaceTrust + `"><RequestSecurityTokenResponse>`)
	soap.Element{Name: "TokenType", Text: TokenTypeDeviceEnrollment}.Write(&b)
	b.WriteString(`<DispositionMessage xmlns="` + NamespaceEnrollment + `"></DispositionMessage><RequestedSecurityToken>`)
	soap.Element{Name: "BinarySecurityToken", Attrs: [][2]string{
		{"ValueType", ValueTypeProvisionDoc}, {"EncodingType", soap.EncodingBase64}, {"xmlns", soap.NamespaceSecurity},
	}, Text: base64.StdEncoding.EncodeToString(provisioningDoc)}.Write(&b)
	b.WriteString(`</RequestedSecurityToken><RequestID xmlns="` + NamespaceEnrollment + `">0</RequestID>`)
	b.WriteString(`</RequestSecurityTokenResponse></RequestSecurityTokenResponseCollection>`)
	return soap.Encode(soap.Response{Action: ActionRSTRC, RelatesTo: relatesTo, Timestamp: ts, Body: b.Bytes()})
}

type rstrcBody struct {
	Collection *struct {
		Response struct {
			TokenType string `xml:"TokenType"`
			Requested struct {
				Token *soap.BinarySecurityToken `xml:"http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-secext-1.0.xsd BinarySecurityToken"`
			} `xml:"RequestedSecurityToken"`
			RequestID string `xml:"http://schemas.microsoft.com/windows/pki/2009/01/enrollment RequestID"`
		} `xml:"RequestSecurityTokenResponse"`
	} `xml:"http://docs.oasis-open.org/ws-sx/ws-trust/200512 RequestSecurityTokenResponseCollection"`
}

// DecodeTokenResponse reads a RequestSecurityTokenResponseCollection and
// returns the provisioning document, for clients. A fault envelope is
// returned as a *soap.DecodedFault error.
func DecodeTokenResponse(data []byte) (*soap.Header, []byte, error) {
	if f, err := soap.DecodeFault(data); err != nil {
		return nil, nil, err
	} else if f != nil {
		return nil, nil, f
	}
	env, err := soap.Decode[rstrcBody](data, 0)
	if err != nil {
		return nil, nil, err
	}
	if env.Body.Collection == nil {
		return &env.Header, nil, fmt.Errorf("%w: no RequestSecurityTokenResponseCollection element", ErrMessage)
	}
	r := env.Body.Collection.Response
	if strings.TrimSpace(r.TokenType) != TokenTypeDeviceEnrollment {
		return &env.Header, nil, fmt.Errorf("%w: TokenType %q", ErrMessage, strings.TrimSpace(r.TokenType))
	}
	if r.Requested.Token == nil || strings.TrimSpace(r.Requested.Token.ValueType) != ValueTypeProvisionDoc {
		return &env.Header, nil, fmt.Errorf("%w: no provisioning document token", ErrMessage)
	}
	doc, err := r.Requested.Token.Bytes()
	if err != nil {
		return &env.Header, nil, err
	}
	return &env.Header, doc, nil
}
