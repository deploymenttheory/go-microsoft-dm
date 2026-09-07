package enroll

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/soap"
	"github.com/deploymenttheory/go-microsoft-dm/testpki"
)

func fixture(t testing.TB, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// rstFixture returns the MS-MDE2 4.3.1.4 request with a real CSR in it.
func rstFixture(t *testing.T) ([]byte, []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	csr, err := testpki.CSR("7BA748C8-703E-4DF2-A74A-92984117346A", key)
	if err != nil {
		t.Fatal(err)
	}
	doc := strings.Replace(string(fixture(t, "rst-onprem-request.xml")), "@CSR@", base64.StdEncoding.EncodeToString(csr), 1)
	return []byte(doc), csr
}

func TestParseVersion(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]int{"1.0": 1, " 3.0 ": 3, "9.0": 9} {
		if got, err := ParseVersion(in); err != nil || got != want {
			t.Errorf("ParseVersion(%q) = %d, %v", in, got, err)
		}
	}
	for _, in := range []string{"", "3", "3.1", "0.0", "10.0", "x.0", "3.0.0"} {
		if _, err := ParseVersion(in); !errors.Is(err, ErrVersion) {
			t.Errorf("ParseVersion(%q) = %v", in, err)
		}
	}
	if FormatVersion(4) != "4.0" {
		t.Error("FormatVersion")
	}
	if !AuthPolicyOnPremise.Valid() || AuthPolicy("x").Valid() || !EnrollmentTypeDevice.Valid() || EnrollmentType("").Valid() {
		t.Error("Valid")
	}
}

func TestDecodeDiscoverFixture(t *testing.T) {
	t.Parallel()
	h, req, err := DecodeDiscover(fixture(t, "discover-onprem-request.xml"), 0)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(h.Action) != ActionDiscover || h.MessageID != "urn:uuid: 748132ec-a575-4329-b01b-6171a9cf8478" {
		t.Errorf("header = %+v", h)
	}
	want := DiscoverRequest{EmailAddress: "user@contoso.com", RequestVersion: "3.0", DeviceType: DeviceTypeWindowsPhone,
		ApplicationVersion: "10.0.0.0", OSEdition: "3", AuthPolicies: []AuthPolicy{AuthPolicyOnPremise}}
	if req.EmailAddress != want.EmailAddress || req.RequestVersion != want.RequestVersion || req.DeviceType != want.DeviceType ||
		req.ApplicationVersion != want.ApplicationVersion || req.OSEdition != want.OSEdition || len(req.AuthPolicies) != 1 || !req.Supports(AuthPolicyOnPremise) || req.Supports(AuthPolicyFederated) {
		t.Errorf("req = %+v", req)
	}
}

func TestDiscoverRoundTrip(t *testing.T) {
	t.Parallel()
	in := &DiscoverRequest{EmailAddress: "a@b.c", RequestVersion: "5.0", DeviceType: DeviceTypeWindows, ApplicationVersion: "10.0.26100.0",
		OSEdition: "4", AuthPolicies: []AuthPolicy{AuthPolicyOnPremise, AuthPolicyFederated, AuthPolicyCertificate}}
	data, err := EncodeDiscover(in, "urn:uuid:1", "https://enterpriseenrollment.b.c"+DiscoveryPath)
	if err != nil {
		t.Fatal(err)
	}
	h, out, err := DecodeDiscover(data, 0)
	if err != nil {
		t.Fatal(err)
	}
	if h.Action != ActionDiscover || h.MessageID != "urn:uuid:1" || h.ReplyTo == nil || h.ReplyTo.Address != soap.AnonymousAddress || h.Security != nil {
		t.Errorf("header = %+v", h)
	}
	if out.EmailAddress != in.EmailAddress || out.RequestVersion != in.RequestVersion || len(out.AuthPolicies) != 3 || out.AuthPolicies[2] != AuthPolicyCertificate {
		t.Errorf("out = %+v", out)
	}
	for _, bad := range []*DiscoverRequest{nil, {RequestVersion: "3.0", AuthPolicies: in.AuthPolicies}, {EmailAddress: "a", AuthPolicies: in.AuthPolicies}, {EmailAddress: "a", RequestVersion: "3.0"}} {
		if _, err := EncodeDiscover(bad, "m", "t"); !errors.Is(err, ErrMessage) {
			t.Errorf("EncodeDiscover(%+v) = %v", bad, err)
		}
	}
	if _, err := EncodeDiscover(in, "", "t"); !errors.Is(err, ErrMessage) {
		t.Error("missing MessageID accepted")
	}
	if _, _, err := DecodeDiscover([]byte(`<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope"><s:Body/></s:Envelope>`), 0); !errors.Is(err, ErrMessage) {
		t.Errorf("no Discover: %v", err)
	}
	if _, _, err := DecodeDiscover([]byte("<x"), 0); !errors.Is(err, soap.ErrSyntax) {
		t.Errorf("syntax: %v", err)
	}
}

func TestDiscoverResponseRoundTrip(t *testing.T) {
	t.Parallel()
	resp := &DiscoverResponse{AuthPolicy: AuthPolicyOnPremise, EnrollmentVersion: "3.0",
		EnrollmentPolicyServiceURL: "https://e.example/ENROLLMENTSERVER/DEVICEENROLLMENTWEBSERVICE.SVC",
		EnrollmentServiceURL:       "https://e.example/ENROLLMENTSERVER/DEVICEENROLLMENTWEBSERVICE.SVC",
		AuthenticationServiceURL:   "https://e.example/auth", DeviceAssociationMaaURL: "https://maa.example", GatewayService: "https://gw.example"}
	data, err := EncodeDiscoverResponse(resp, "urn:uuid:1", "act-1")
	if err != nil {
		t.Fatal(err)
	}
	want := `<DiscoverResponse xmlns="http://schemas.microsoft.com/windows/management/2012/01/enrollment"><DiscoverResult><AuthPolicy>OnPremise</AuthPolicy><EnrollmentVersion>3.0</EnrollmentVersion><EnrollmentPolicyServiceUrl>https://e.example/ENROLLMENTSERVER/DEVICEENROLLMENTWEBSERVICE.SVC</EnrollmentPolicyServiceUrl><EnrollmentServiceUrl>https://e.example/ENROLLMENTSERVER/DEVICEENROLLMENTWEBSERVICE.SVC</EnrollmentServiceUrl><AuthenticationServiceUrl>https://e.example/auth</AuthenticationServiceUrl><DeviceAssociationMaaUrl>https://maa.example</DeviceAssociationMaaUrl><GatewayService>https://gw.example</GatewayService></DiscoverResult></DiscoverResponse>`
	if !strings.Contains(string(data), want) || !strings.Contains(string(data), `<ActivityId CorrelationId="act-1"`) || !strings.Contains(string(data), `xmlns:xsi=`) {
		t.Errorf("got %s", data)
	}
	h, out, err := DecodeDiscoverResponse(data)
	if err != nil {
		t.Fatal(err)
	}
	if h.Action != ActionDiscoverResponse || *out != *resp {
		t.Errorf("out = %+v", out)
	}
	// Minimal: no optional elements.
	data, err = EncodeDiscoverResponse(&DiscoverResponse{AuthPolicy: AuthPolicyOnPremise, EnrollmentServiceURL: "https://e"}, "r", "")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "EnrollmentVersion") || strings.Contains(string(data), "EnrollmentPolicyServiceUrl") || strings.Contains(string(data), "ActivityId") {
		t.Errorf("optional elements present: %s", data)
	}
	for _, bad := range []*DiscoverResponse{nil, {EnrollmentServiceURL: "x"}, {AuthPolicy: AuthPolicyOnPremise}, {AuthPolicy: "Nope", EnrollmentServiceURL: "x"}} {
		if _, err := EncodeDiscoverResponse(bad, "r", ""); !errors.Is(err, ErrMessage) {
			t.Errorf("EncodeDiscoverResponse(%+v) = %v", bad, err)
		}
	}
	fault, _ := soap.EncodeFault(soap.NewFault(soap.SubcodeMessageFormat, "bad"), ActionDiscoverResponse, "r")
	var df *soap.DecodedFault
	if _, _, err := DecodeDiscoverResponse(fault); !errors.As(err, &df) || df.Subcode != "s:MessageFormat" {
		t.Errorf("fault: %v", err)
	}
	if _, _, err := DecodeDiscoverResponse([]byte("<x")); !errors.Is(err, soap.ErrSyntax) {
		t.Errorf("syntax: %v", err)
	}
	other, _ := soap.Encode(soap.Response{Action: "a", RelatesTo: "r", Body: []byte("<Other/>")})
	if _, _, err := DecodeDiscoverResponse(other); !errors.Is(err, ErrMessage) {
		t.Errorf("other body: %v", err)
	}
}

func TestGetPoliciesCodec(t *testing.T) {
	t.Parallel()
	h, req, err := DecodeGetPolicies(fixture(t, "getpolicies-onprem-request.xml"), 0)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(h.Action) != ActionGetPolicies || h.Security == nil || h.Security.UsernameToken == nil || h.Security.UsernameToken.Username != "user@contoso.com" {
		t.Errorf("header = %+v", h)
	}
	if req == nil || req.LastUpdate != "" {
		t.Errorf("req = %+v", req)
	}
	data, err := EncodeGetPolicies("urn:uuid:2", "https://e.example/svc", &Credentials{Username: "u", Password: "p"})
	if err != nil {
		t.Fatal(err)
	}
	h, _, err = DecodeGetPolicies(data, 0)
	if err != nil {
		t.Fatal(err)
	}
	ut := h.Security.UsernameToken
	if ut.ID != soap.UsernameTokenID || ut.Username != "u" || ut.Password.Value != "p" || ut.Password.Type != soap.PasswordText {
		t.Errorf("token = %+v", ut)
	}
	if _, _, err := DecodeGetPolicies([]byte(`<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope"><s:Body/></s:Envelope>`), 0); !errors.Is(err, ErrMessage) {
		t.Errorf("no GetPolicies: %v", err)
	}
	if _, _, err := DecodeGetPolicies(nil, 0); !errors.Is(err, soap.ErrSyntax) {
		t.Errorf("syntax: %v", err)
	}
	if _, err := EncodeGetPolicies("", "t", nil); !errors.Is(err, ErrMessage) {
		t.Error("missing MessageID accepted")
	}
}

func TestPolicyResponseCodec(t *testing.T) {
	t.Parallel()
	p := DefaultPolicy()
	p.PolicyID = "pol-1"
	data, err := EncodePolicyResponse(p, "urn:uuid:3")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`<GetPoliciesResponse xmlns="http://schemas.microsoft.com/windows/pki/2009/01/enrollmentpolicy"><response><policyID>pol-1</policyID><policyFriendlyName xsi:nil="true"/>`,
		`<policyOIDReference>0</policyOIDReference><cAs xsi:nil="true"/><attributes><commonName>go-microsoft-dm</commonName><policySchema>3</policySchema>`,
		`<certificateValidity><validityPeriodSeconds>31536000</validityPeriodSeconds><renewalPeriodSeconds>5184000</renewalPeriodSeconds></certificateValidity>`,
		`<permission><enroll>true</enroll><autoEnroll>false</autoEnroll></permission>`,
		`<privateKeyAttributes><minimalKeyLength>2048</minimalKeyLength><keySpec xsi:nil="true"/><keyUsageProperty xsi:nil="true"/><permissions xsi:nil="true"/><algorithmOIDReference xsi:nil="true"/><cryptoProviders><provider>Microsoft Platform Crypto Provider</provider><provider>Microsoft Software Key Storage Provider</provider></cryptoProviders></privateKeyAttributes>`,
		`<revision><majorRevision>101</majorRevision><minorRevision>0</minorRevision></revision>`,
		`<hashAlgorithmOIDReference>0</hashAlgorithmOIDReference><rARequirements xsi:nil="true"/><keyArchivalAttributes xsi:nil="true"/><extensions xsi:nil="true"/></attributes></policy></policies></response><cAs xsi:nil="true"/>`,
		`<oIDs><oID><value>2.16.840.1.101.3.4.2.1</value><group>4</group><oIDReferenceID>0</oIDReferenceID><defaultName>szOID_NIST_sha256</defaultName></oID></oIDs></GetPoliciesResponse>`,
	} {
		if !strings.Contains(string(data), want) {
			t.Errorf("missing %s\nin %s", want, data)
		}
	}
	h, out, err := DecodePolicyResponse(data)
	if err != nil {
		t.Fatal(err)
	}
	if h.Action != ActionGetPoliciesResponse || out.PolicyID != "pol-1" || out.CommonName != p.CommonName || out.Validity != p.Validity || out.Renewal != p.Renewal ||
		!out.Enroll || out.AutoEnroll || out.MinimalKeyLength != 2048 || len(out.CryptoProviders) != 2 || out.MajorRevision != 101 || out.HashAlgorithm != SHA256 {
		t.Errorf("out = %+v", out)
	}
	// An empty policyID is written as an empty element, as in Microsoft's example.
	p.PolicyID, p.CryptoProviders = "", nil
	data, err = EncodePolicyResponse(p, "r")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "<policyID/>") || strings.Contains(string(data), "cryptoProviders") {
		t.Errorf("got %s", data)
	}
	if _, _, err := DecodePolicyResponse([]byte("<x")); !errors.Is(err, soap.ErrSyntax) {
		t.Errorf("syntax: %v", err)
	}
	fault, _ := soap.EncodeFault(soap.NewFault(soap.SubcodeAuthentication, "no"), ActionGetPoliciesResponse, "r")
	var df *soap.DecodedFault
	if _, _, err := DecodePolicyResponse(fault); !errors.As(err, &df) {
		t.Errorf("fault: %v", err)
	}
	other, _ := soap.Encode(soap.Response{Action: "a", RelatesTo: "r", Body: []byte("<Other/>")})
	if _, _, err := DecodePolicyResponse(other); !errors.Is(err, ErrMessage) {
		t.Errorf("other body: %v", err)
	}
}

func TestPolicyResponseValidate(t *testing.T) {
	t.Parallel()
	if err := DefaultPolicy().Validate(); err != nil {
		t.Fatal(err)
	}
	cases := map[string]func(p *PolicyResponse){
		"zero validity":  func(p *PolicyResponse) { p.Validity = 0 },
		"renewal longer": func(p *PolicyResponse) { p.Renewal = p.Validity },
		"small key":      func(p *PolicyResponse) { p.MinimalKeyLength = 1024 },
		"no hash":        func(p *PolicyResponse) { p.HashAlgorithm = OID{} },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			p := DefaultPolicy()
			mutate(p)
			if err := p.Validate(); !errors.Is(err, ErrMessage) {
				t.Errorf("err = %v", err)
			}
			if _, err := EncodePolicyResponse(p, "r"); !errors.Is(err, ErrMessage) {
				t.Errorf("encode err = %v", err)
			}
		})
	}
	var nilPolicy *PolicyResponse
	if err := nilPolicy.Validate(); !errors.Is(err, ErrMessage) {
		t.Error("nil policy")
	}
}

func TestDecodeRequestSecurityTokenFixture(t *testing.T) {
	t.Parallel()
	body, csr := rstFixture(t)
	h, rst, err := DecodeRequestSecurityToken(body, 0)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(h.Action) != ActionRST || h.MessageID != "urn:uuid:0d5a1441-5891-453b-becf-a2e5f6ea3749" || h.Security.UsernameToken.Password.Value != "mypassword" {
		t.Errorf("header = %+v", h)
	}
	if rst.TokenType != TokenTypeDeviceEnrollment || rst.RequestType != RequestTypeIssue || rst.Token.ValueType != ValueTypePKCS10 || rst.Token.EncodingType != soap.EncodingBase64 {
		t.Errorf("rst = %+v", rst)
	}
	got, err := rst.Token.Bytes()
	if err != nil || string(got) != string(csr) {
		t.Errorf("CSR bytes differ: %v", err)
	}
	c := rst.Context
	checks := map[string]bool{
		"UXInitiated":           c.UXInitiated,
		"ExternalMgmtAgentHint": c.ExternalMgmtAgentHint == "Agent1:Value1",
		"DomainName":            c.DomainName == "mydomain.fabrikam.com",
		"OSEdition trimmed":     c.OSEdition == "4",
		"OSVersion":             c.OSVersion == "10.0.9999.0",
		"DeviceName":            c.DeviceName == "MY_WINDOWS_DEVICE",
		"MAC":                   len(c.MAC) == 2 && c.MAC[1] == "CC:CC:CC:CC:CC:CC",
		"IMEI":                  len(c.IMEI) == 2 && c.IMEI[0] == "49015420323756",
		"EnrollmentType":        c.EnrollmentType == EnrollmentTypeFull,
		"DeviceType":            c.DeviceType == DeviceTypeWindows,
		"ApplicationVersion":    c.ApplicationVersion == "10.0.9999.0",
		"DeviceID":              c.DeviceID == "7BA748C8-703E-4DF2-A74A-92984117346A",
		"EnrollmentData":        c.EnrollmentData == "3J4KLJ9SDJFAL93JLAKHJSDFJHAO83HAKSHFLAHSKFNHNPA2934342",
		"TargetedUserLoggedIn":  c.TargetedUserLoggedIn,
		"Locale":                c.Locale == "en-us",
		"HWDevID trimmed":       c.HWDevID == strings.Repeat("F", 64),
		"ZeroTouchProvisioning": c.ZeroTouchProvisioning == "ffffffff-ffff-4fff-afff-ffffffffffff",
		"OfflineAutoPilot":      c.OfflineAutoPilotEnrollmentCorrelator == "ffffffff-ffff-4fff-afff-ffffffffffff",
		"NotInOobe":             c.NotInOobe,
		"Unknown kept":          len(c.Unknown) == 1 && c.Unknown[0].Name == "FutureItem" && c.Unknown[0].Value == "kept",
		"Items count":           len(c.Items) == 22,
	}
	for name, ok := range checks {
		if !ok {
			t.Errorf("%s failed: %+v", name, c)
		}
	}
	if v, ok := c.Get("FutureItem"); !ok || v != "kept" {
		t.Error("Get unknown")
	}
	if _, ok := c.Get("Missing"); ok {
		t.Error("Get missing")
	}
}

func TestParseAdditionalContextCoversCatalogue(t *testing.T) {
	t.Parallel()
	names := []string{"BulkAADJ", "BootstrapDomainJoin", "PlugandForget", "WhiteGlove", "WhiteGloveHybridJoin",
		"AIKAttestationClaim", "AIKPub", "AIKCert", "AadAIKAttestationClaim", "AADPub", "EmmDeviceId", "RequestVersion",
		"AzureAttestationBlob", "AttestationStatus", "AttestationStatusHResult", "AzureAttestationCorrelationVector", "AIKAlgorithm"}
	items := make([]ContextItem, 0, len(names))
	for _, n := range names {
		v := "v-" + n
		if strings.HasPrefix(n, "Bulk") || strings.HasPrefix(n, "Bootstrap") || strings.HasPrefix(n, "Plug") || strings.HasPrefix(n, "White") {
			v = "TRUE"
		}
		items = append(items, ContextItem{Name: n, Value: v})
	}
	c := ParseAdditionalContext(items)
	if !c.BulkAADJ || !c.BootstrapDomainJoin || !c.PlugandForget || !c.WhiteGlove || !c.WhiteGloveHybridJoin {
		t.Errorf("booleans: %+v", c)
	}
	strs := map[string]string{"AIKAttestationClaim": c.AIKAttestationClaim, "AIKPub": c.AIKPub, "AIKCert": c.AIKCert,
		"AadAIKAttestationClaim": c.AadAIKAttestationClaim, "AADPub": c.AADPub, "EmmDeviceId": c.EmmDeviceID, "RequestVersion": c.RequestVersion,
		"AzureAttestationBlob": c.AzureAttestationBlob, "AttestationStatus": c.AttestationStatus, "AttestationStatusHResult": c.AttestationStatusHResult,
		"AzureAttestationCorrelationVector": c.AzureAttestationCorrelationVector, "AIKAlgorithm": c.AIKAlgorithm}
	for n, got := range strs {
		if got != "v-"+n {
			t.Errorf("%s = %q", n, got)
		}
	}
	if len(c.Unknown) != 0 {
		t.Errorf("unknown = %+v", c.Unknown)
	}
}

func TestRequestSecurityTokenRoundTrip(t *testing.T) {
	t.Parallel()
	_, csr := rstFixture(t)
	items := []ContextItem{{Name: "DeviceID", Value: "d1"}, {Name: "EnrollmentType", Value: "Device"}, {Name: "DeviceName", Value: `A&B "quoted"`}}
	data, err := EncodeRequestSecurityToken("urn:uuid:9", "https://e.example/svc", &Credentials{Username: "u", Password: "p"}, csr, items)
	if err != nil {
		t.Fatal(err)
	}
	h, rst, err := DecodeRequestSecurityToken(data, 0)
	if err != nil {
		t.Fatal(err)
	}
	if h.Action != ActionRST || h.Security.UsernameToken.Username != "u" {
		t.Errorf("header = %+v", h)
	}
	got, err := rst.Token.Bytes()
	if err != nil || string(got) != string(csr) {
		t.Error("CSR differs")
	}
	if rst.Context.DeviceID != "d1" || rst.Context.EnrollmentType != EnrollmentTypeDevice || rst.Context.DeviceName != `A&B "quoted"` {
		t.Errorf("context = %+v", rst.Context)
	}
	if _, err := EncodeRequestSecurityToken("m", "t", nil, nil, nil); !errors.Is(err, ErrMessage) {
		t.Error("empty CSR accepted")
	}
	if _, err := EncodeRequestSecurityToken("", "t", nil, csr, nil); !errors.Is(err, ErrMessage) {
		t.Error("missing MessageID accepted")
	}
	if _, _, err := DecodeRequestSecurityToken([]byte(`<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope"><s:Body/></s:Envelope>`), 0); !errors.Is(err, ErrMessage) {
		t.Errorf("no RST: %v", err)
	}
	if _, _, err := DecodeRequestSecurityToken([]byte("<"), 0); !errors.Is(err, soap.ErrSyntax) {
		t.Errorf("syntax: %v", err)
	}
	// A body without token or context still decodes.
	bare := `<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope" xmlns:wst="http://docs.oasis-open.org/ws-sx/ws-trust/200512"><s:Body><wst:RequestSecurityToken><wst:TokenType>t</wst:TokenType></wst:RequestSecurityToken></s:Body></s:Envelope>`
	_, rst, err = DecodeRequestSecurityToken([]byte(bare), 0)
	if err != nil || rst.TokenType != "t" || rst.Token.ValueType != "" || len(rst.Context.Items) != 0 {
		t.Errorf("bare = %+v, %v", rst, err)
	}
}

func TestTokenResponseRoundTrip(t *testing.T) {
	t.Parallel()
	doc := []byte(`<wap-provisioningdoc version="1.1"><characteristic type="X"/></wap-provisioningdoc>`)
	now := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	data, err := EncodeTokenResponse(doc, "urn:uuid:5", &soap.TimestampRange{Created: now, Expires: now.Add(5 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`<RequestSecurityTokenResponseCollection xmlns="http://docs.oasis-open.org/ws-sx/ws-trust/200512"><RequestSecurityTokenResponse><TokenType>` + TokenTypeDeviceEnrollment + `</TokenType>`,
		`<DispositionMessage xmlns="http://schemas.microsoft.com/windows/pki/2009/01/enrollment"></DispositionMessage><RequestedSecurityToken><BinarySecurityToken ValueType="` + ValueTypeProvisionDoc + `" EncodingType="` + soap.EncodingBase64 + `" xmlns="` + soap.NamespaceSecurity + `">`,
		`</BinarySecurityToken></RequestedSecurityToken><RequestID xmlns="http://schemas.microsoft.com/windows/pki/2009/01/enrollment">0</RequestID></RequestSecurityTokenResponse></RequestSecurityTokenResponseCollection>`,
		`<u:Created>2026-09-07T12:00:00.000Z</u:Created><u:Expires>2026-09-07T12:05:00.000Z</u:Expires>`,
	} {
		if !strings.Contains(string(data), want) {
			t.Errorf("missing %s\nin %s", want, data)
		}
	}
	h, got, err := DecodeTokenResponse(data)
	if err != nil {
		t.Fatal(err)
	}
	if h.Action != ActionRSTRC || string(got) != string(doc) {
		t.Errorf("decoded %s", got)
	}
	if _, err := EncodeTokenResponse(nil, "r", nil); !errors.Is(err, ErrMessage) {
		t.Error("empty doc accepted")
	}
	if _, _, err := DecodeTokenResponse([]byte("<")); !errors.Is(err, soap.ErrSyntax) {
		t.Errorf("syntax: %v", err)
	}
	fault, _ := soap.EncodeFault(soap.NewFault(soap.SubcodeCertificateRequest, "no").WithDetail(soap.ErrorNotEligibleToRenew, "t"), ActionRSTRC, "r")
	var df *soap.DecodedFault
	if _, _, err := DecodeTokenResponse(fault); !errors.As(err, &df) || df.Detail.ErrorType != "NotEligibleToRenew" {
		t.Errorf("fault: %v", err)
	}
	other, _ := soap.Encode(soap.Response{Action: "a", RelatesTo: "r", Body: []byte("<Other/>")})
	if _, _, err := DecodeTokenResponse(other); !errors.Is(err, ErrMessage) {
		t.Errorf("other body: %v", err)
	}
	bad := strings.Replace(string(data), TokenTypeDeviceEnrollment, "urn:other", 1)
	if _, _, err := DecodeTokenResponse([]byte(bad)); !errors.Is(err, ErrMessage) {
		t.Errorf("token type: %v", err)
	}
	bad = strings.Replace(string(data), ValueTypeProvisionDoc, "urn:other", 1)
	if _, _, err := DecodeTokenResponse([]byte(bad)); !errors.Is(err, ErrMessage) {
		t.Errorf("value type: %v", err)
	}
	bad = strings.Replace(string(data), base64.StdEncoding.EncodeToString(doc), "!!!", 1)
	if _, _, err := DecodeTokenResponse([]byte(bad)); !errors.Is(err, soap.ErrEncoding) {
		t.Errorf("base64: %v", err)
	}
}
