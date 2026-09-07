package enroll

import (
	"context"
	"crypto/rand"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/deploymenttheory/go-microsoft-dm/clock"
	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/soap"
	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/wapprov"
)

// Errors the flow reports as the Cause of a fault. Callers test them with
// errors.Is on the returned error.
var (
	// ErrConfig reports an unusable Config.
	ErrConfig = errors.New("enroll: invalid configuration")
	// ErrUnauthenticated is returned by an Authenticator when the
	// credentials are not recognised; it becomes an s:Authentication fault.
	ErrUnauthenticated = errors.New("enroll: credentials not recognized")
	// ErrUnauthorized is returned by an Authenticator or Issuer when the
	// principal may not enroll; it becomes an s:Authorization fault.
	ErrUnauthorized = errors.New("enroll: not allowed to enroll")
	// ErrPKCS7 reports a PKCS#7 renewal token, which Phase 9 accepts.
	ErrPKCS7 = errors.New("enroll: PKCS#7 renewal tokens are not supported")
	// ErrRenew reports a WS-Trust Renew request, which Phase 9 accepts.
	ErrRenew = errors.New("enroll: certificate renewal is not supported")
	// ErrPolicyUnsupported reports an authentication policy this phase does
	// not implement.
	ErrPolicyUnsupported = errors.New("enroll: authentication policy not implemented")
	// ErrPolicyDisabled reports a GetPolicies call to a server that did not
	// advertise an EnrollmentPolicyServiceUrl.
	ErrPolicyDisabled = errors.New("enroll: policy service disabled")
)

// Principal is the authenticated user an enrollment belongs to.
type Principal struct {
	// UPN is the user principal name written to DMClient/Provider/<ID>/UPN.
	UPN string
	// Attributes carries anything else the Authenticator learned.
	Attributes map[string]string
}

// Authenticator verifies on-premise credentials from the WS-Security
// UsernameToken. It returns ErrUnauthenticated for unknown credentials,
// ErrUnauthorized for a known user who may not enroll, a *soap.Fault for
// anything the service should render as is, or any other error for an
// internal failure.
type Authenticator interface {
	Authenticate(ctx context.Context, creds Credentials) (Principal, error)
}

// AuthenticatorFunc adapts a function to Authenticator.
type AuthenticatorFunc func(ctx context.Context, creds Credentials) (Principal, error)

// Authenticate implements Authenticator.
func (f AuthenticatorFunc) Authenticate(ctx context.Context, creds Credentials) (Principal, error) {
	return f(ctx, creds)
}

// Request is what the service hands an Issuer: the DER PKCS#10 as the
// client sent it, the typed context, and who authenticated.
type Request struct {
	CSR       []byte
	Context   AdditionalContext
	Principal Principal
	// Header is the request's SOAP header, for hooks that log MessageID or To.
	Header soap.Header
}

// Issued is a signed certificate with its chain: the issuing certificate
// first, then any intermediates, ending at the root.
type Issued struct {
	Certificate *x509.Certificate
	Chain       []*x509.Certificate
}

// Issuer parses the CSR, applies policy and signs. It returns
// ErrUnauthorized, a *soap.Fault, or any error, which becomes an
// s:CertificateRequest fault.
type Issuer interface {
	Issue(ctx context.Context, req *Request) (*Issued, error)
}

// Enrollment is a completed issuance, handed to the Provisioner and then
// the Recorder.
type Enrollment struct {
	Request     *Request
	Certificate *x509.Certificate
	Chain       []*x509.Certificate
	// Store is where the client keeps the certificate: My/User for a Full
	// enrollment, My/System for a Device enrollment.
	Store      wapprov.Store
	EnrolledAt time.Time
	TraceID    string
}

// Provisioner builds the provisioning document for an enrollment.
type Provisioner interface {
	Provision(ctx context.Context, e *Enrollment) (*wapprov.Document, error)
}

// Recorder persists an enrollment before the provisioning document is
// returned. It is optional. Its error becomes an s:EnrollmentServer fault.
type Recorder interface {
	Record(ctx context.Context, e *Enrollment, doc *wapprov.Document) error
}

// Config configures a Service.
type Config struct {
	// EnrollmentServiceURL is the absolute URL of the RequestSecurityToken
	// endpoint. Required.
	EnrollmentServiceURL string
	// EnrollmentPolicyServiceURL is the GetPolicies endpoint. Empty
	// disables the policy service and omits it from discovery.
	EnrollmentPolicyServiceURL string
	// EnrollmentVersion is what discovery advertises; default 3.0. It must be
	// between 3.0 and 9.0.
	EnrollmentVersion string
	// AuthPolicy is the policy discovery answers with. Only OnPremise is
	// implemented in this phase.
	AuthPolicy AuthPolicy
	// Authenticator verifies on-premise credentials. Required.
	Authenticator Authenticator
	// Issuer signs certificates. Required.
	Issuer Issuer
	// Provisioner builds provisioning documents. Required.
	Provisioner Provisioner
	// Recorder persists enrollments. Optional.
	Recorder Recorder
	// Policy is the GetPolicies answer; default DefaultPolicy.
	Policy *PolicyResponse
	// Clock stamps EnrolledAt and the response timestamp; default real time.
	Clock clock.Clock
	// MaxRequestSize bounds request bodies; default soap.DefaultMaxSize.
	MaxRequestSize int
	// TraceID generates the per-request identifier written into faults and
	// ActivityId headers; default a random UUID.
	TraceID func() string
}

// Service implements the three MS-MDE2 operations over decoded messages.
// It is transport-neutral; Handler adapts it to HTTP.
type Service struct {
	cfg Config
}

// New validates the configuration and returns a Service.
func New(cfg Config) (*Service, error) {
	if cfg.EnrollmentServiceURL == "" {
		return nil, fmt.Errorf("%w: EnrollmentServiceURL is required", ErrConfig)
	}
	if err := checkURL(cfg.EnrollmentServiceURL); err != nil {
		return nil, err
	}
	if cfg.EnrollmentPolicyServiceURL != "" {
		if err := checkURL(cfg.EnrollmentPolicyServiceURL); err != nil {
			return nil, err
		}
	}
	if cfg.EnrollmentVersion == "" {
		cfg.EnrollmentVersion = DefaultEnrollmentVersion
	}
	if n, err := ParseVersion(cfg.EnrollmentVersion); err != nil || n < MinEnrollmentVersion {
		return nil, fmt.Errorf("%w: EnrollmentVersion %q must be 3.0 to 9.0", ErrConfig, cfg.EnrollmentVersion)
	}
	if cfg.AuthPolicy == "" {
		cfg.AuthPolicy = AuthPolicyOnPremise
	}
	if cfg.AuthPolicy != AuthPolicyOnPremise {
		return nil, fmt.Errorf("%w: %w: %s", ErrConfig, ErrPolicyUnsupported, cfg.AuthPolicy)
	}
	if cfg.Authenticator == nil || cfg.Issuer == nil || cfg.Provisioner == nil {
		return nil, fmt.Errorf("%w: Authenticator, Issuer and Provisioner are required", ErrConfig)
	}
	if cfg.Policy == nil {
		cfg.Policy = DefaultPolicy()
	}
	if err := cfg.Policy.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrConfig, err)
	}
	if cfg.Clock == nil {
		cfg.Clock = clock.Real{}
	}
	if cfg.MaxRequestSize <= 0 {
		cfg.MaxRequestSize = soap.DefaultMaxSize
	}
	if cfg.TraceID == nil {
		cfg.TraceID = NewUUID
	}
	return &Service{cfg: cfg}, nil
}

func checkURL(s string) error {
	u, err := url.Parse(s)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return fmt.Errorf("%w: %q is not an absolute https URL", ErrConfig, s)
	}
	return nil
}

// Config returns the effective configuration.
func (s *Service) Config() Config { return s.cfg }

// NewUUID returns a random version 4 UUID in its canonical text form.
func NewUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand does not fail on supported platforms; a zero id is
		// still a well-formed trace id.
		return "00000000-0000-4000-8000-000000000000"
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	h := hex.EncodeToString(b[:])
	return h[:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:]
}

// relatesTo picks the RelatesTo for a reply: the request MessageID when the
// header parsed, otherwise a fixed placeholder so the envelope stays valid.
func relatesTo(h *soap.Header) string {
	if h != nil && h.MessageID != "" {
		return h.MessageID
	}
	return "urn:uuid:00000000-0000-0000-0000-000000000000"
}

// fault renders err as a fault envelope for the operation whose reply
// action is given, and returns it with the *soap.Fault.
func (s *Service) fault(err error, action string, h *soap.Header, traceID string) ([]byte, error) {
	f := soap.AsFault(err)
	if f.TraceID == "" {
		f.TraceID = traceID
	}
	out, encErr := soap.EncodeFault(f, action, relatesTo(h))
	if encErr != nil {
		// Only an invalid subcode or detail reaches here, which is a bug in
		// the caller; fall back to an internal fault so the client gets an
		// answer.
		f = &soap.Fault{Subcode: soap.SubcodeInternalServiceFault, Reason: "internal service fault", TraceID: traceID, Cause: errors.Join(err, encErr)}
		out, _ = soap.EncodeFault(f, action, relatesTo(h))
	}
	return out, f
}

// Discover answers a Discover request (MS-MDE2 3.1.4.1). The returned bytes
// are always a complete envelope: the response, or a fault when err is
// non-nil (err is then a *soap.Fault).
func (s *Service) Discover(_ context.Context, body []byte) ([]byte, error) {
	trace := s.cfg.TraceID()
	h, req, err := DecodeDiscover(body, s.cfg.MaxRequestSize)
	if err != nil {
		return s.fault(soap.NewFault(soap.SubcodeMessageFormat, "cannot parse Discover request").WithCause(err), ActionDiscoverResponse, h, trace)
	}
	if h.Action != "" && h.Action != ActionDiscover {
		return s.fault(soap.NewFault(soap.SubcodeMessageFormat, "unexpected action "+h.Action), ActionDiscoverResponse, h, trace)
	}
	if req.EmailAddress == "" {
		return s.fault(soap.NewFault(soap.SubcodeMessageFormat, "EmailAddress is required"), ActionDiscoverResponse, h, trace)
	}
	reqVersion, err := ParseVersion(req.RequestVersion)
	if err != nil {
		return s.fault(soap.NewFault(soap.SubcodeMessageFormat, "RequestVersion must be 1.0 to 9.0").WithCause(err), ActionDiscoverResponse, h, trace)
	}
	if !req.Supports(s.cfg.AuthPolicy) {
		return s.fault(soap.NewFault(soap.SubcodeAuthentication,
			fmt.Sprintf("this server enrolls with the %s policy, which the client did not offer", s.cfg.AuthPolicy)).
			WithCause(ErrPolicyUnsupported), ActionDiscoverResponse, h, trace)
	}
	resp := &DiscoverResponse{
		AuthPolicy:                 s.cfg.AuthPolicy,
		EnrollmentPolicyServiceURL: s.cfg.EnrollmentPolicyServiceURL,
		EnrollmentServiceURL:       s.cfg.EnrollmentServiceURL,
	}
	// The server never advertises a version the client did not ask for,
	// and clients below 3.0 predate the element.
	serverVersion, _ := ParseVersion(s.cfg.EnrollmentVersion)
	if reqVersion >= MinEnrollmentVersion {
		resp.EnrollmentVersion = FormatVersion(min(serverVersion, reqVersion))
	}
	out, err := EncodeDiscoverResponse(resp, relatesTo(h), trace)
	if err != nil {
		return s.fault(err, ActionDiscoverResponse, h, trace)
	}
	return out, nil
}

// authenticate checks the WS-Security header for the on-premise policy.
func (s *Service) authenticate(ctx context.Context, h *soap.Header) (Principal, error) {
	if h.Security == nil {
		return Principal{}, soap.NewFault(soap.SubcodeInvalidSecurity, "missing wsse:Security header")
	}
	ut := h.Security.UsernameToken
	if ut == nil {
		if h.Security.BinarySecurityToken != nil {
			return Principal{}, soap.NewFault(soap.SubcodeInvalidSecurity, "this server accepts on-premise credentials, not a security token").WithCause(ErrPolicyUnsupported)
		}
		return Principal{}, soap.NewFault(soap.SubcodeInvalidSecurity, "missing wsse:UsernameToken")
	}
	if ut.Password.Type != "" && ut.Password.Type != soap.PasswordText {
		return Principal{}, soap.NewFault(soap.SubcodeInvalidSecurity, "unsupported password type "+ut.Password.Type)
	}
	if ut.Username == "" {
		return Principal{}, soap.NewFault(soap.SubcodeInvalidSecurity, "empty wsse:Username")
	}
	p, err := s.cfg.Authenticator.Authenticate(ctx, Credentials{Username: ut.Username, Password: ut.Password.Value})
	switch {
	case err == nil:
		return p, nil
	case errors.Is(err, ErrUnauthenticated):
		return Principal{}, soap.NewFault(soap.SubcodeAuthentication, "user not recognized").WithCause(err)
	case errors.Is(err, ErrUnauthorized):
		return Principal{}, soap.NewFault(soap.SubcodeAuthorization, "user not allowed to enroll").WithCause(err)
	}
	var f *soap.Fault
	if errors.As(err, &f) {
		return Principal{}, f
	}
	return Principal{}, soap.NewFault(soap.SubcodeEnrollmentServer, "authentication failed").WithCause(err)
}

// GetPolicies answers a GetPolicies request (MS-MDE2 3.3.4.1).
func (s *Service) GetPolicies(ctx context.Context, body []byte) ([]byte, error) {
	trace := s.cfg.TraceID()
	h, _, err := DecodeGetPolicies(body, s.cfg.MaxRequestSize)
	if err != nil {
		return s.fault(soap.NewFault(soap.SubcodeMessageFormat, "cannot parse GetPolicies request").WithCause(err), ActionGetPoliciesResponse, h, trace)
	}
	if s.cfg.EnrollmentPolicyServiceURL == "" {
		return s.fault(soap.NewFault(soap.SubcodeEnrollmentServer, "policy service is not enabled").WithCause(ErrPolicyDisabled), ActionGetPoliciesResponse, h, trace)
	}
	if h.Action != "" && h.Action != ActionGetPolicies {
		return s.fault(soap.NewFault(soap.SubcodeMessageFormat, "unexpected action "+h.Action), ActionGetPoliciesResponse, h, trace)
	}
	if _, err := s.authenticate(ctx, h); err != nil {
		return s.fault(err, ActionGetPoliciesResponse, h, trace)
	}
	out, err := EncodePolicyResponse(s.cfg.Policy, relatesTo(h))
	if err != nil {
		return s.fault(err, ActionGetPoliciesResponse, h, trace)
	}
	return out, nil
}

// Enroll answers a RequestSecurityToken request (MS-MDE2 3.4.4.1): it
// authenticates, checks the token, issues through the Issuer, builds the
// provisioning document, records the enrollment and returns the
// RequestSecurityTokenResponseCollection.
func (s *Service) Enroll(ctx context.Context, body []byte) ([]byte, error) {
	trace := s.cfg.TraceID()
	h, rst, err := DecodeRequestSecurityToken(body, s.cfg.MaxRequestSize)
	if err != nil {
		return s.fault(soap.NewFault(soap.SubcodeMessageFormat, "cannot parse RequestSecurityToken").WithCause(err), ActionRSTRC, h, trace)
	}
	if h.Action != "" && h.Action != ActionRST {
		return s.fault(soap.NewFault(soap.SubcodeMessageFormat, "unexpected action "+h.Action), ActionRSTRC, h, trace)
	}
	principal, err := s.authenticate(ctx, h)
	if err != nil {
		return s.fault(err, ActionRSTRC, h, trace)
	}
	req, err := s.checkToken(rst, principal, h, trace)
	if err != nil {
		return s.fault(err, ActionRSTRC, h, trace)
	}
	issued, err := s.cfg.Issuer.Issue(ctx, req)
	if err != nil {
		return s.fault(issueFault(err), ActionRSTRC, h, trace)
	}
	if issued == nil || issued.Certificate == nil {
		return s.fault(soap.NewFault(soap.SubcodeCertificateRequest, "issuer returned no certificate"), ActionRSTRC, h, trace)
	}
	store := wapprov.StoreUser
	if req.Context.EnrollmentType == EnrollmentTypeDevice {
		store = wapprov.StoreSystem
	}
	now := s.cfg.Clock.Now()
	e := &Enrollment{Request: req, Certificate: issued.Certificate, Chain: issued.Chain, Store: store, EnrolledAt: now, TraceID: trace}
	doc, err := s.cfg.Provisioner.Provision(ctx, e)
	if err != nil {
		return s.fault(wrapFault(err, soap.SubcodeEnrollmentServer, "cannot build provisioning document"), ActionRSTRC, h, trace)
	}
	encoded, err := wapprov.Encode(doc, wapprov.Options{})
	if err != nil {
		return s.fault(soap.NewFault(soap.SubcodeEnrollmentServer, "invalid provisioning document").WithCause(err), ActionRSTRC, h, trace)
	}
	if s.cfg.Recorder != nil {
		if err := s.cfg.Recorder.Record(ctx, e, doc); err != nil {
			return s.fault(wrapFault(err, soap.SubcodeEnrollmentServer, "cannot record enrollment"), ActionRSTRC, h, trace)
		}
	}
	out, err := EncodeTokenResponse(encoded, relatesTo(h), &soap.TimestampRange{Created: now, Expires: now.Add(5 * time.Minute)})
	if err != nil {
		return s.fault(err, ActionRSTRC, h, trace)
	}
	return out, nil
}

// checkToken validates the WS-Trust fields and the context and returns the
// issuer request.
func (s *Service) checkToken(rst *RequestSecurityToken, p Principal, h *soap.Header, trace string) (*Request, error) {
	if rst.TokenType != TokenTypeDeviceEnrollment {
		return nil, soap.NewFault(soap.SubcodeMessageFormat, "unexpected TokenType "+rst.TokenType)
	}
	switch rst.RequestType {
	case RequestTypeIssue:
	case RequestTypeRenew:
		return nil, soap.NewFault(soap.SubcodeCertificateRequest, "renewal is not supported").
			WithDetail(soap.ErrorNotEligibleToRenew, trace).WithCause(ErrRenew)
	default:
		return nil, soap.NewFault(soap.SubcodeMessageFormat, "unexpected RequestType "+rst.RequestType)
	}
	switch rst.Token.ValueType {
	case ValueTypePKCS10:
	case ValueTypePKCS7:
		return nil, soap.NewFault(soap.SubcodeCertificateRequest, "PKCS#7 renewal tokens are not supported").
			WithDetail(soap.ErrorNotEligibleToRenew, trace).WithCause(ErrPKCS7)
	case "":
		return nil, soap.NewFault(soap.SubcodeMessageFormat, "missing BinarySecurityToken")
	default:
		return nil, soap.NewFault(soap.SubcodeMessageFormat, "unexpected token ValueType "+rst.Token.ValueType)
	}
	csr, err := rst.Token.Bytes()
	if err != nil || len(csr) == 0 {
		return nil, soap.NewFault(soap.SubcodeMessageFormat, "BinarySecurityToken is not base64").WithCause(err)
	}
	c := rst.Context
	if c.DeviceID == "" {
		return nil, soap.NewFault(soap.SubcodeMessageFormat, "DeviceID is required").WithDetail(soap.ErrorInvalidEnrollmentData, trace)
	}
	if !c.EnrollmentType.Valid() {
		return nil, soap.NewFault(soap.SubcodeMessageFormat, fmt.Sprintf("EnrollmentType %q is not Full or Device", c.EnrollmentType)).
			WithDetail(soap.ErrorInvalidEnrollmentData, trace)
	}
	if c.RequestVersion != "" {
		if _, err := ParseVersion(c.RequestVersion); err != nil {
			return nil, soap.NewFault(soap.SubcodeMessageFormat, "RequestVersion must be 1.0 to 9.0").
				WithDetail(soap.ErrorInvalidEnrollmentData, trace).WithCause(err)
		}
	}
	return &Request{CSR: csr, Context: c, Principal: p, Header: *h}, nil
}

// issueFault maps an Issuer error to a fault: a *soap.Fault as is,
// ErrUnauthorized to s:Authorization, anything else to s:CertificateRequest.
func issueFault(err error) error {
	var f *soap.Fault
	if errors.As(err, &f) {
		return f
	}
	if errors.Is(err, ErrUnauthorized) {
		return soap.NewFault(soap.SubcodeAuthorization, "device not allowed to enroll").WithCause(err)
	}
	return soap.NewFault(soap.SubcodeCertificateRequest, "failed to get certificate").WithCause(err)
}

// wrapFault keeps a *soap.Fault and wraps anything else in the subcode.
func wrapFault(err error, sub soap.Subcode, reason string) error {
	var f *soap.Fault
	if errors.As(err, &f) {
		return f
	}
	return soap.NewFault(sub, reason).WithCause(err)
}
