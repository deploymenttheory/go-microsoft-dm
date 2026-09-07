package enroll

import "github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/soap"

// Endpoint paths and namespaces (MS-MDE2 2.2.1 and 3.1).
const (
	// DiscoveryPath is the constant path of the Discovery Service.
	DiscoveryPath = "/EnrollmentServer/Discovery.svc"
	// NamespaceDiscovery holds Discover and DiscoverResponse.
	NamespaceDiscovery = "http://schemas.microsoft.com/windows/management/2012/01/enrollment"
	// NamespacePolicy is MS-XCEP's namespace (GetPolicies).
	NamespacePolicy = "http://schemas.microsoft.com/windows/pki/2009/01/enrollmentpolicy"
	// NamespaceEnrollment is MS-WSTEP's namespace (RequestID, DispositionMessage).
	NamespaceEnrollment = soap.NamespaceEnrollment
)

// SOAP actions (MS-MDE2 3.1.4.1, 3.3.4.1, 3.4.4.1).
const (
	ActionDiscover            = "http://schemas.microsoft.com/windows/management/2012/01/enrollment/IDiscoveryService/Discover"
	ActionDiscoverResponse    = "http://schemas.microsoft.com/windows/management/2012/01/enrollment/IDiscoveryService/DiscoverResponse"
	ActionGetPolicies         = "http://schemas.microsoft.com/windows/pki/2009/01/enrollmentpolicy/IPolicy/GetPolicies"
	ActionGetPoliciesResponse = "http://schemas.microsoft.com/windows/pki/2009/01/enrollmentpolicy/IPolicy/GetPoliciesResponse"
	ActionRST                 = "http://schemas.microsoft.com/windows/pki/2009/01/enrollment/RST/wstep"
	ActionRSTRC               = "http://schemas.microsoft.com/windows/pki/2009/01/enrollment/RSTRC/wstep"
)

// WS-Trust token and request types (MS-MDE2 3.4.4.1.1.1).
const (
	// TokenTypeDeviceEnrollment is the wst:TokenType of a request and reply.
	TokenTypeDeviceEnrollment = "http://schemas.microsoft.com/5.0.0.0/ConfigurationManager/Enrollment/DeviceEnrollmentToken"
	// RequestTypeIssue asks for a new certificate.
	RequestTypeIssue = "http://docs.oasis-open.org/ws-sx/ws-trust/200512/Issue"
	// RequestTypeRenew is the WS-Trust renewal request MS-MDE2 3.5 uses; it
	// arrives with a PKCS#7 token and is accepted from Phase 9.
	RequestTypeRenew = "http://docs.oasis-open.org/ws-sx/ws-trust/200512/Renew"
	// ValueTypePKCS10 is the BinarySecurityToken ValueType of a PKCS#10 CSR.
	ValueTypePKCS10 = "http://schemas.microsoft.com/windows/pki/2009/01/enrollment#PKCS10"
	// ValueTypePKCS7 is the ValueType of a renewal token.
	ValueTypePKCS7 = "http://schemas.microsoft.com/windows/pki/2009/01/enrollment#PKCS7"
	// ValueTypeProvisionDoc is the ValueType of the returned provisioning
	// document.
	ValueTypeProvisionDoc = "http://schemas.microsoft.com/5.0.0.0/ConfigurationManager/Enrollment/DeviceEnrollmentProvisionDoc"
)

// AuthPolicy is the authentication policy a discovery exchange settles on.
type AuthPolicy string

// The three policies MS-MDE2 3.1.4.1.3.2 allows.
const (
	AuthPolicyOnPremise   AuthPolicy = "OnPremise"
	AuthPolicyFederated   AuthPolicy = "Federated"
	AuthPolicyCertificate AuthPolicy = "Certificate"
)

// Valid reports whether the policy is one MS-MDE2 names.
func (p AuthPolicy) Valid() bool {
	switch p {
	case AuthPolicyOnPremise, AuthPolicyFederated, AuthPolicyCertificate:
		return true
	}
	return false
}

// DeviceType is the DeviceType a client reports.
type DeviceType string

// Device types MS-MDE2 3.1.4.1.3.1 and 3.4.4.1.1.1 name.
const (
	DeviceTypeWindows         DeviceType = "CIMClient_Windows"
	DeviceTypeWindowsPhone    DeviceType = "WindowsPhone"
	DeviceTypeWindowsHandheld DeviceType = "WindowsHandheld"
)

// EnrollmentType is the AdditionalContext EnrollmentType item.
type EnrollmentType string

// The two enrollment types. Full is a user enrollment whose certificate
// lands in My/User; Device is a device enrollment into My/System.
const (
	EnrollmentTypeFull   EnrollmentType = "Full"
	EnrollmentTypeDevice EnrollmentType = "Device"
)

// Valid reports whether the type is Full or Device.
func (e EnrollmentType) Valid() bool {
	return e == EnrollmentTypeFull || e == EnrollmentTypeDevice
}

// Versions the protocol names (MS-MDE2 3.1.4.1.3.1 and 3.1.4.1.3.2).
const (
	// MinRequestVersion and MaxRequestVersion bound RequestVersion.
	MinRequestVersion = 1
	MaxRequestVersion = 9
	// MinEnrollmentVersion is the lowest EnrollmentVersion a server may
	// advertise.
	MinEnrollmentVersion = 3
	// DefaultEnrollmentVersion is what this implementation advertises until
	// later phases implement the features the higher versions imply.
	DefaultEnrollmentVersion = "3.0"
)
