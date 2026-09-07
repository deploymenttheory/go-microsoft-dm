package soap

import (
	"errors"
	"strings"
	"testing"
)

type probeBody struct {
	Probe struct {
		Value string `xml:"Value"`
	} `xml:"http://example.test/probe Probe"`
}

const onPremRequest = `<?xml version="1.0"?>
<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope"
 xmlns:a="http://www.w3.org/2005/08/addressing"
 xmlns:u="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-utility-1.0.xsd"
 xmlns:wsse="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-secext-1.0.xsd">
<s:Header>
  <a:Action s:mustUnderstand="1">http://example.test/Probe</a:Action>
  <a:MessageID>urn:uuid:72048B64-0F19-448F-8C2E-B4C661860AA0</a:MessageID>
  <a:ReplyTo><a:Address>http://www.w3.org/2005/08/addressing/anonymous</a:Address></a:ReplyTo>
  <a:To s:mustUnderstand="1">https://enrolltest.contoso.com/ENROLLMENTSERVER/DEVICEENROLLMENTWEBSERVICE.SVC</a:To>
  <wsse:Security s:mustUnderstand="1">
    <wsse:UsernameToken u:Id="uuid-cc1ccc1f-2fba-4bcf-b063-ffc0cac77917-4">
      <wsse:Username>user@contoso.com</wsse:Username>
      <wsse:Password wsse:Type="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-username-token-profile-1.0#PasswordText">mypassword</wsse:Password>
    </wsse:UsernameToken>
  </wsse:Security>
</s:Header>
<s:Body xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
  <Probe xmlns="http://example.test/probe"><Value>hello</Value></Probe>
</s:Body>
</s:Envelope>`

func TestDecodeReadsHeaderAndBody(t *testing.T) {
	t.Parallel()
	env, err := Decode[probeBody]([]byte(onPremRequest), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := env.Header
	if h.Action != "http://example.test/Probe" {
		t.Errorf("Action = %q", h.Action)
	}
	if h.MessageID != "urn:uuid:72048B64-0F19-448F-8C2E-B4C661860AA0" {
		t.Errorf("MessageID = %q", h.MessageID)
	}
	if h.ReplyTo == nil || h.ReplyTo.Address != AnonymousAddress {
		t.Errorf("ReplyTo = %+v", h.ReplyTo)
	}
	if !strings.HasPrefix(h.To, "https://enrolltest.contoso.com/") {
		t.Errorf("To = %q", h.To)
	}
	if h.Security == nil || h.Security.MustUnderstand != "1" {
		t.Fatalf("Security = %+v", h.Security)
	}
	ut := h.Security.UsernameToken
	if ut == nil || ut.ID != UsernameTokenID || ut.Username != "user@contoso.com" ||
		ut.Password.Type != PasswordText || ut.Password.Value != "mypassword" {
		t.Errorf("UsernameToken = %+v", ut)
	}
	if h.Security.BinarySecurityToken != nil || h.Security.Timestamp != nil {
		t.Errorf("unexpected header tokens: %+v", h.Security)
	}
	if env.Body.Probe.Value != "hello" {
		t.Errorf("body = %+v", env.Body)
	}
	if env.Version() != NamespaceEnvelope {
		t.Errorf("Version = %q", env.Version())
	}
}

func TestDecodeAcceptsSOAP11(t *testing.T) {
	t.Parallel()
	doc := strings.Replace(onPremRequest, NamespaceEnvelope, NamespaceEnvelope11, 1)
	env, err := Decode[probeBody]([]byte(doc), 0)
	if err != nil {
		t.Fatal(err)
	}
	if env.Version() != NamespaceEnvelope11 {
		t.Errorf("Version = %q", env.Version())
	}
}

func TestDecodeHeaderTimestampAndBinaryToken(t *testing.T) {
	t.Parallel()
	doc := `<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope" xmlns:a="http://www.w3.org/2005/08/addressing"
 xmlns:u="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-utility-1.0.xsd"
 xmlns:wsse="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-secext-1.0.xsd">
<s:Header><a:Action>x</a:Action>
 <wsse:Security><u:Timestamp u:Id="_0"><u:Created>2012-08-02T00:32:59.420Z</u:Created><u:Expires>2012-08-02T00:37:59.420Z</u:Expires></u:Timestamp>
 <wsse:BinarySecurityToken ValueType="` + ValueTypeUserToken + `" EncodingType="` + EncodingBase64 + `">aGVs
 bG8=</wsse:BinarySecurityToken></wsse:Security></s:Header><s:Body/></s:Envelope>`
	env, err := Decode[struct{}]([]byte(doc), 0)
	if err != nil {
		t.Fatal(err)
	}
	ts := env.Header.Security.Timestamp
	if ts == nil || ts.ID != "_0" || ts.Created != "2012-08-02T00:32:59.420Z" || ts.Expires != "2012-08-02T00:37:59.420Z" {
		t.Errorf("Timestamp = %+v", ts)
	}
	bst := env.Header.Security.BinarySecurityToken
	if bst == nil || bst.ValueType != ValueTypeUserToken {
		t.Fatalf("token = %+v", bst)
	}
	got, err := bst.Bytes()
	if err != nil || string(got) != "hello" {
		t.Errorf("Bytes = %q, %v", got, err)
	}
}

func TestDecodeRejects(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		doc  string
		max  int
		want error
	}{
		"not xml":         {doc: "<s:Envelope", want: ErrSyntax},
		"wrong root":      {doc: `<Foo xmlns="http://www.w3.org/2003/05/soap-envelope"/>`, want: ErrSyntax},
		"wrong namespace": {doc: `<Envelope xmlns="http://example.test/soap"><Body/></Envelope>`, want: ErrSyntax},
		"no namespace":    {doc: `<Envelope><Body/></Envelope>`, want: ErrSyntax},
		"too large":       {doc: onPremRequest, max: 10, want: ErrTooLarge},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := Decode[probeBody]([]byte(tc.doc), tc.max)
			if !errors.Is(err, tc.want) {
				t.Errorf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestBinarySecurityTokenBytes(t *testing.T) {
	t.Parallel()
	var nilTok *BinarySecurityToken
	if _, err := nilTok.Bytes(); !errors.Is(err, ErrEncoding) {
		t.Errorf("nil token: %v", err)
	}
	if _, err := (&BinarySecurityToken{EncodingType: "hex", Value: "00"}).Bytes(); !errors.Is(err, ErrEncoding) {
		t.Errorf("hex: %v", err)
	}
	if _, err := (&BinarySecurityToken{EncodingType: EncodingBase64, Value: "!!!"}).Bytes(); !errors.Is(err, ErrEncoding) {
		t.Errorf("bad base64: %v", err)
	}
	got, err := (&BinarySecurityToken{Value: "\n  aGVsbG8=\r\n"}).Bytes()
	if err != nil || string(got) != "hello" {
		t.Errorf("empty encoding: %q %v", got, err)
	}
}
