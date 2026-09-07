package soap

import (
	"bytes"
	"errors"
	"fmt"
)

// Subcode is a fault subcode from MS-MDE2 2.2.10, spelled as it appears on
// the wire including its namespace prefix.
type Subcode string

// The seven subcodes of MS-MDE2 2.2.10 with their HRESULT values.
const (
	SubcodeMessageFormat        Subcode = "s:MessageFormat"        // 0x80180001 MENROLL_E_DEVICE_MESSAGE_FORMAT_ERROR
	SubcodeAuthentication       Subcode = "s:Authentication"       // 0x80180002 MENROLL_E_DEVICE_AUTHENTICATION_ERROR
	SubcodeAuthorization        Subcode = "s:Authorization"        // 0x80180003 MENROLL_E_DEVICE_AUTHORIZATION_ERROR
	SubcodeCertificateRequest   Subcode = "s:CertificateRequest"   // 0x80180004 MENROLL_E_DEVICE_CERTIFCATEREQUEST_ERROR
	SubcodeEnrollmentServer     Subcode = "s:EnrollmentServer"     // 0x80180005 MENROLL_E_DEVICE_CONFIGMGRSERVER_ERROR
	SubcodeInternalServiceFault Subcode = "a:InternalServiceFault" // 0x80180006 MENROLL_E_DEVICE_INTERNALSERVICE_ERROR
	SubcodeInvalidSecurity      Subcode = "a:InvalidSecurity"      // 0x80180007 MENROLL_E_DEVICE_INVALIDSECURITY_ERROR
)

// HResult returns the MENROLL_E_* value the subcode maps to, or 0 for an
// unknown subcode.
func (s Subcode) HResult() uint32 {
	switch s {
	case SubcodeMessageFormat:
		return 0x80180001
	case SubcodeAuthentication:
		return 0x80180002
	case SubcodeAuthorization:
		return 0x80180003
	case SubcodeCertificateRequest:
		return 0x80180004
	case SubcodeEnrollmentServer:
		return 0x80180005
	case SubcodeInternalServiceFault:
		return 0x80180006
	case SubcodeInvalidSecurity:
		return 0x80180007
	}
	return 0
}

// Valid reports whether the subcode is one MS-MDE2 defines.
func (s Subcode) Valid() bool { return s.HResult() != 0 }

// ErrorType is the deviceenrollmentserviceerror errortype of MS-MDE2
// 2.2.10, spelled as on the wire.
type ErrorType string

// The detail error types of MS-MDE2 2.2.10 with their HRESULT values.
const (
	ErrorDeviceCapReached      ErrorType = "DeviceCapReached"      // 0x80180013
	ErrorDeviceNotSupported    ErrorType = "DeviceNotSupported"    // 0x80180014
	ErrorNotSupported          ErrorType = "NotSupported"          // 0x80180015
	ErrorNotEligibleToRenew    ErrorType = "NotEligibleToRenew"    // 0x80180016
	ErrorInMaintenance         ErrorType = "InMaintenance"         // 0x80180017
	ErrorUserLicense           ErrorType = "UserLicense"           // 0x80180018
	ErrorInvalidEnrollmentData ErrorType = "InvalidEnrollmentData" // 0x80180019
	ErrorCustomServerError     ErrorType = "CustomServerError"     // 0x80180032
)

// HResult returns the MENROLL_E_* value the error type maps to, or 0 for
// an unknown type.
func (e ErrorType) HResult() uint32 {
	switch e {
	case ErrorDeviceCapReached:
		return 0x80180013
	case ErrorDeviceNotSupported:
		return 0x80180014
	case ErrorNotSupported:
		return 0x80180015
	case ErrorNotEligibleToRenew:
		return 0x80180016
	case ErrorInMaintenance:
		return 0x80180017
	case ErrorUserLicense:
		return 0x80180018
	case ErrorInvalidEnrollmentData:
		return 0x80180019
	case ErrorCustomServerError:
		return 0x80180032
	}
	return 0
}

// Valid reports whether the error type is one MS-MDE2 defines.
func (e ErrorType) Valid() bool { return e.HResult() != 0 }

// Fault is a SOAP fault the enrollment service returns. It is an error so
// that a handler can return it from deep inside a flow and the transport
// layer can render it.
type Fault struct {
	// Subcode selects the s: or a: subcode; required.
	Subcode Subcode
	// Reason is the s:Text the client logs and, for CustomServerError,
	// shows to the user.
	Reason string
	// Detail, when set, adds the deviceenrollmentserviceerror element.
	Detail ErrorType
	// TraceID identifies the server-side state for support; written into
	// the detail when Detail is set.
	TraceID string
	// Cause is the underlying error, kept for logs and never written to
	// the wire.
	Cause error
}

// Error implements error.
func (f *Fault) Error() string {
	s := fmt.Sprintf("soap fault %s (0x%08x)", f.Subcode, f.Subcode.HResult())
	if f.Detail != "" {
		s += fmt.Sprintf(" %s (0x%08x)", f.Detail, f.Detail.HResult())
	}
	if f.Reason != "" {
		s += ": " + f.Reason
	}
	return s
}

// Unwrap returns the cause.
func (f *Fault) Unwrap() error { return f.Cause }

// NewFault builds a fault with a subcode and reason.
func NewFault(sub Subcode, reason string) *Fault {
	return &Fault{Subcode: sub, Reason: reason}
}

// WithDetail sets the deviceenrollmentserviceerror type and trace id.
func (f *Fault) WithDetail(t ErrorType, traceID string) *Fault {
	f.Detail = t
	f.TraceID = traceID
	return f
}

// WithCause records the underlying error.
func (f *Fault) WithCause(err error) *Fault {
	f.Cause = err
	return f
}

// AsFault returns the *Fault in err's chain, or an InternalServiceFault
// wrapping err when there is none. A nil err yields nil.
func AsFault(err error) *Fault {
	if err == nil {
		return nil
	}
	var f *Fault
	if errors.As(err, &f) {
		return f
	}
	return &Fault{Subcode: SubcodeInternalServiceFault, Reason: "internal service fault", Cause: err}
}

// EncodeFault writes the fault as the SOAP 1.2 envelope of MS-MDE2 2.2.10.
// The reply action is the RSTRC action in Microsoft's examples whatever the
// request was; callers pass the action for the operation that failed.
func EncodeFault(f *Fault, action, relatesTo string) ([]byte, error) {
	if f == nil {
		return nil, fmt.Errorf("%w: nil fault", ErrResponse)
	}
	if !f.Subcode.Valid() {
		return nil, fmt.Errorf("%w: unknown subcode %q", ErrResponse, f.Subcode)
	}
	if f.Detail != "" && !f.Detail.Valid() {
		return nil, fmt.Errorf("%w: unknown error type %q", ErrResponse, f.Detail)
	}
	var b bytes.Buffer
	b.WriteString(`<s:Fault><s:Code><s:Value>s:Receiver</s:Value><s:Subcode><s:Value>`)
	escape(&b, string(f.Subcode))
	b.WriteString(`</s:Value></s:Subcode></s:Code><s:Reason><s:Text xml:lang="` + FaultLang + `">`)
	escape(&b, f.Reason)
	b.WriteString(`</s:Text></s:Reason>`)
	if f.Detail != "" {
		b.WriteString(`<s:Detail><deviceenrollmentserviceerror xmlns="` + NamespaceEnrollment + `">`)
		Element{Name: "errortype", Text: string(f.Detail)}.Write(&b)
		Element{Name: "message", Text: f.Reason}.Write(&b)
		Element{Name: "traceid", Text: f.TraceID}.Write(&b)
		b.WriteString(`</deviceenrollmentserviceerror></s:Detail>`)
	}
	b.WriteString(`</s:Fault>`)
	return Encode(Response{Action: action, RelatesTo: relatesTo, ActivityID: f.TraceID, Body: b.Bytes()})
}

// DecodedFault is a fault read back from the wire, used by the simulator
// and by tests.
type DecodedFault struct {
	Code    string
	Subcode string
	Reason  string
	Detail  *ErrorDetail
}

// ErrorDetail is the decoded deviceenrollmentserviceerror element.
type ErrorDetail struct {
	ErrorType string `xml:"errortype"`
	Message   string `xml:"message"`
	TraceID   string `xml:"traceid"`
}

// Error implements error.
func (d *DecodedFault) Error() string {
	s := "soap fault " + d.Subcode
	if d.Detail != nil {
		s += " " + d.Detail.ErrorType
	}
	if d.Reason != "" {
		s += ": " + d.Reason
	}
	return s
}

type faultBody struct {
	Fault *struct {
		Code struct {
			Value   string `xml:"Value"`
			Subcode struct {
				Value string `xml:"Value"`
			} `xml:"Subcode"`
		} `xml:"Code"`
		Reason struct {
			Text string `xml:"Text"`
		} `xml:"Reason"`
		Detail *struct {
			Error *ErrorDetail `xml:"deviceenrollmentserviceerror"`
		} `xml:"Detail"`
	} `xml:"Fault"`
}

// DecodeFault reads a fault envelope. It returns nil, nil for an envelope
// that carries no fault.
func DecodeFault(data []byte) (*DecodedFault, error) {
	env, err := Decode[faultBody](data, 0)
	if err != nil {
		return nil, err
	}
	if env.Body.Fault == nil {
		return nil, nil //nolint:nilnil // no fault is a valid outcome
	}
	f := env.Body.Fault
	d := &DecodedFault{Code: f.Code.Value, Subcode: f.Code.Subcode.Value, Reason: f.Reason.Text}
	if f.Detail != nil && f.Detail.Error != nil {
		d.Detail = f.Detail.Error
	}
	return d, nil
}
