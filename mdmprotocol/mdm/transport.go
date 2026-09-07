package mdm

import (
	"crypto/x509"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"strings"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/syncml"
)

// Errors for the transport layer.
var (
	// ErrContentType reports a body that is not SyncML XML.
	ErrContentType = errors.New("mdm: unsupported content type")
	// ErrMode reports an unknown mode query parameter.
	ErrMode = errors.New("mdm: unknown mode")
)

// Mode is the client's execution context from the mode query parameter
// (MS-MDM 2.1 note 1).
type Mode string

// The two modes, plus empty for an absent parameter.
const (
	// ModeMaintenance means a user is signed in and the client may reach
	// the user's profile.
	ModeMaintenance Mode = "Maintenance"
	// ModeMachine means the client runs as SYSTEM with no user profile.
	ModeMachine Mode = "Machine"
)

// HTTP header names MS-MDM 2.1 defines.
const (
	HeaderMSSignature     = "MS-Signature"
	HeaderDeviceToken     = "DeviceToken"
	HeaderGenericAlert    = "MDM-GenericAlert"
	HeaderClientRequestID = "client-request-id"
	HeaderCorrelation     = "MS-CV"
	HeaderUserAgentOrigin = "UserAgentOrigin"
)

// Transport is what the HTTP layer knew about a request, parsed but never
// enforced beyond the content type: the Learn enrollment page says a server
// must not check User-Agent, hostnames or value formats.
type Transport struct {
	Mode     Mode
	Platform string
	// UserAgent is logged; MS-MDM names "MSFT OMA DM Client/1.2.0.1".
	UserAgent string
	// BearerToken is the Entra token from Authorization: Bearer; validated
	// in Phase 10.
	BearerToken string
	// DeviceToken is the Entra device token sent when ForceAadToken is set.
	DeviceToken string
	// MSSignature is the base64 CMS detached signature when
	// RequireMessageSigning is on; verified in Phase 9.
	MSSignature string
	// GenericAlerts are the alert types from MDM-GenericAlert, in order.
	GenericAlerts []string
	// ClientRequestID is EntDMID when the server set it.
	ClientRequestID string
	// CorrelationVector is MS-CV.
	CorrelationVector string
	// UserAgentOrigin is MS-MDM 3.1.7.2, comma-separated integers.
	UserAgentOrigin string
	// ContentType is the media type without parameters.
	ContentType string
	// Certificates are the TLS peer certificates, leaf first.
	Certificates []*x509.Certificate
}

// ParseTransport reads the transport facts from an HTTP request. It returns
// ErrContentType for WBXML (Phase 14) or anything that is not SyncML XML;
// an absent Content-Type is taken as XML because the Windows client has
// been observed to omit it.
func ParseTransport(r *http.Request) (*Transport, error) {
	t := &Transport{}
	if err := t.parseQuery(r.URL.Query()); err != nil {
		return nil, err
	}
	t.UserAgent = r.UserAgent()
	if auth := r.Header.Get("Authorization"); auth != "" {
		if scheme, tok, ok := strings.Cut(auth, " "); ok && strings.EqualFold(scheme, "Bearer") {
			t.BearerToken = strings.TrimSpace(tok)
		}
	}
	t.DeviceToken = r.Header.Get(HeaderDeviceToken)
	t.MSSignature = r.Header.Get(HeaderMSSignature)
	t.GenericAlerts = ParseGenericAlerts(r.Header.Get(HeaderGenericAlert))
	t.ClientRequestID = r.Header.Get(HeaderClientRequestID)
	t.CorrelationVector = r.Header.Get(HeaderCorrelation)
	t.UserAgentOrigin = r.Header.Get(HeaderUserAgentOrigin)
	if r.TLS != nil {
		t.Certificates = r.TLS.PeerCertificates
	}
	ct := r.Header.Get("Content-Type")
	if ct == "" {
		t.ContentType = syncml.ContentTypeXML
		return t, nil
	}
	mt, _, err := mime.ParseMediaType(ct)
	if err != nil {
		return nil, fmt.Errorf("%w: %q", ErrContentType, ct)
	}
	t.ContentType = mt
	switch mt {
	case syncml.ContentTypeXML, "text/xml", "application/xml":
	case syncml.ContentTypeWBXML:
		return t, fmt.Errorf("%w: WBXML is not implemented", ErrContentType)
	default:
		return t, fmt.Errorf("%w: %q", ErrContentType, mt)
	}
	return t, nil
}

func (t *Transport) parseQuery(q url.Values) error {
	switch m := q.Get("mode"); m {
	case "":
	case string(ModeMaintenance), string(ModeMachine):
		t.Mode = Mode(m)
	default:
		return fmt.Errorf("%w: %q", ErrMode, m)
	}
	t.Platform = q.Get("Platform")
	return nil
}

// ParseGenericAlerts splits the MDM-GenericAlert header value, which is
// "<Type1><Type2>" (MS-MDM 2.1 note 5).
func ParseGenericAlerts(v string) []string {
	var out []string
	for _, part := range strings.Split(v, "<") {
		part = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(part), ">"))
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
