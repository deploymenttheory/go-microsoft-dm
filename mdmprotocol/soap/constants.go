package soap

// XML namespaces used by MS-MDE2 messages (MS-MDE2 2.2.1).
const (
	// NamespaceEnvelope is the SOAP 1.2 envelope namespace every MS-MDE2
	// example uses.
	NamespaceEnvelope = "http://www.w3.org/2003/05/soap-envelope"
	// NamespaceEnvelope11 is the SOAP 1.1 envelope namespace. MS-MDE2 2.1
	// names SOAP 1.1 as the transport and the RequestSecurityTokenResponse
	// example in section 4.3.2 uses this namespace, so it is accepted on
	// input.
	NamespaceEnvelope11 = "http://schemas.xmlsoap.org/soap/envelope/"
	// NamespaceAddressing is WS-Addressing 1.0.
	NamespaceAddressing = "http://www.w3.org/2005/08/addressing"
	// NamespaceSecurity is WS-Security 2004 (wsse).
	NamespaceSecurity = "http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-secext-1.0.xsd"
	// NamespaceUtility is WS-Security Utility (wsu, u).
	NamespaceUtility = "http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-utility-1.0.xsd"
	// NamespaceTrust is WS-Trust 1.3 (wst).
	NamespaceTrust = "http://docs.oasis-open.org/ws-sx/ws-trust/200512"
	// NamespaceAuthorization holds AdditionalContext and ContextItem (ac).
	NamespaceAuthorization = "http://schemas.xmlsoap.org/ws/2006/12/authorization"
	// NamespaceSchemaInstance is XML Schema instance (xsi).
	NamespaceSchemaInstance = "http://www.w3.org/2001/XMLSchema-instance"
	// NamespaceSchema is XML Schema (xsd).
	NamespaceSchema = "http://www.w3.org/2001/XMLSchema"
	// NamespaceDiagnostics carries the ActivityId header in Microsoft's
	// responses.
	NamespaceDiagnostics = "http://schemas.microsoft.com/2004/09/ServiceModel/Diagnostics"
	// NamespaceEnrollment is the MS-WSTEP namespace that carries the fault
	// detail, DispositionMessage and RequestID elements.
	NamespaceEnrollment = "http://schemas.microsoft.com/windows/pki/2009/01/enrollment"
)

// WS-Addressing and WS-Security values with a fixed spelling.
const (
	// AnonymousAddress is the ReplyTo address every request carries.
	AnonymousAddress = "http://www.w3.org/2005/08/addressing/anonymous"
	// EncodingBase64 is the BinarySecurityToken EncodingType MS-MDE2 requires.
	EncodingBase64 = "http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-secext-1.0.xsd#base64binary"
	// PasswordText is the wsse:Password Type for on-premise authentication.
	PasswordText = "http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-username-token-profile-1.0#PasswordText"
	// UsernameTokenID is the u:Id MS-MDE2 3.3.4.1.1.1.3 requires on the
	// UsernameToken for on-premise authentication.
	UsernameTokenID = "uuid-cc1ccc1f-2fba-4bcf-b063-ffc0cac77917-4"
	// ValueTypeUserToken is the BinarySecurityToken ValueType for a federated
	// security token in the header.
	ValueTypeUserToken = "http://schemas.microsoft.com/5.0.0.0/ConfigurationManager/Enrollment/DeviceEnrollmentUserToken"
	// ValueTypeX509v3 is the header token ValueType for certificate
	// authentication.
	ValueTypeX509v3 = "http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-x509-token-profile-1.0#X509v3"
	// FaultLang is the xml:lang on fault reason text.
	FaultLang = "en-US"
	// DefaultMaxSize bounds a decoded request. Enrollment requests are a few
	// kilobytes; a CSR with attestation claims is tens of kilobytes.
	DefaultMaxSize = 1 << 20
)
