package soap

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestEncodeMatchesExampleShape(t *testing.T) {
	t.Parallel()
	created := time.Date(2012, 8, 2, 0, 32, 59, 420_000_000, time.UTC)
	out, err := Encode(Response{
		Action:           "http://schemas.microsoft.com/windows/pki/2009/01/enrollment/RSTRC/wstep",
		RelatesTo:        "urn:uuid:81a5419a-496b-474f-a627-5cdd33eed8ab",
		ActivityID:       "d9eb2fdd-e38a-46ee-bd93-aea9dc86a3b8",
		Timestamp:        &TimestampRange{Created: created, Expires: created.Add(5 * time.Minute)},
		SchemaAttributes: true,
		Body:             []byte(`<X xmlns="urn:x">1 &amp; 2</X>`),
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope" xmlns:a="http://www.w3.org/2005/08/addressing" xmlns:u="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-utility-1.0.xsd">` +
		`<s:Header><a:Action s:mustUnderstand="1">http://schemas.microsoft.com/windows/pki/2009/01/enrollment/RSTRC/wstep</a:Action>` +
		`<ActivityId CorrelationId="d9eb2fdd-e38a-46ee-bd93-aea9dc86a3b8" xmlns="http://schemas.microsoft.com/2004/09/ServiceModel/Diagnostics">d9eb2fdd-e38a-46ee-bd93-aea9dc86a3b8</ActivityId>` +
		`<a:RelatesTo>urn:uuid:81a5419a-496b-474f-a627-5cdd33eed8ab</a:RelatesTo>` +
		`<o:Security s:mustUnderstand="1" xmlns:o="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-secext-1.0.xsd"><u:Timestamp u:Id="_0"><u:Created>2012-08-02T00:32:59.420Z</u:Created><u:Expires>2012-08-02T00:37:59.420Z</u:Expires></u:Timestamp></o:Security>` +
		`</s:Header><s:Body xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:xsd="http://www.w3.org/2001/XMLSchema"><X xmlns="urn:x">1 &amp; 2</X></s:Body></s:Envelope>`
	if string(out) != want {
		t.Errorf("got\n%s\nwant\n%s", out, want)
	}
	// The response must decode as an envelope again.
	env, err := Decode[struct{}](out, 0)
	if err != nil {
		t.Fatal(err)
	}
	if env.Header.Action == "" {
		t.Error("action lost")
	}
}

func TestEncodeMinimal(t *testing.T) {
	t.Parallel()
	out, err := Encode(Response{Action: "a", RelatesTo: "r<", Body: []byte("<B/>")})
	if err != nil {
		t.Fatal(err)
	}
	want := `<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope" xmlns:a="http://www.w3.org/2005/08/addressing"><s:Header><a:Action s:mustUnderstand="1">a</a:Action><a:RelatesTo>r&lt;</a:RelatesTo></s:Header><s:Body><B/></s:Body></s:Envelope>`
	if string(out) != want {
		t.Errorf("got %s", out)
	}
}

func TestEncodeRejects(t *testing.T) {
	t.Parallel()
	cases := map[string]Response{
		"no action":    {RelatesTo: "r", Body: []byte("<B/>")},
		"no relatesTo": {Action: "a", Body: []byte("<B/>")},
		"no body":      {Action: "a", RelatesTo: "r", Body: []byte("  ")},
	}
	for name, r := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := Encode(r); !errors.Is(err, ErrResponse) {
				t.Errorf("err = %v", err)
			}
		})
	}
}

func TestElementWrite(t *testing.T) {
	t.Parallel()
	var b bytes.Buffer
	Element{Name: "a", Attrs: [][2]string{{"k", `v"1`}}, Text: "x<y"}.Write(&b)
	Element{Name: "e", Empty: true}.Write(&b)
	Element{Name: "f"}.Write(&b)
	want := `<a k="v&#34;1">x&lt;y</a><e/><f></f>`
	if b.String() != want {
		t.Errorf("got %s want %s", b.String(), want)
	}
}

func TestFaultRoundTrip(t *testing.T) {
	t.Parallel()
	f := NewFault(SubcodeAuthorization, "device cap reached").
		WithDetail(ErrorDeviceCapReached, "2493ee37-beeb-4cb9-833c-cadde9067645").
		WithCause(errors.New("cap"))
	out, err := EncodeFault(f, "http://schemas.microsoft.com/windows/pki/2009/01/enrollment/RSTRC/wstep", "urn:uuid:0d5a1441-5891-453b-becf-a2e5f6ea3749")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`<s:Fault><s:Code><s:Value>s:Receiver</s:Value><s:Subcode><s:Value>s:Authorization</s:Value></s:Subcode></s:Code>`,
		`<s:Reason><s:Text xml:lang="en-US">device cap reached</s:Text></s:Reason>`,
		`<s:Detail><deviceenrollmentserviceerror xmlns="http://schemas.microsoft.com/windows/pki/2009/01/enrollment"><errortype>DeviceCapReached</errortype><message>device cap reached</message><traceid>2493ee37-beeb-4cb9-833c-cadde9067645</traceid></deviceenrollmentserviceerror></s:Detail>`,
		`<ActivityId CorrelationId="2493ee37-beeb-4cb9-833c-cadde9067645"`,
	} {
		if !strings.Contains(string(out), want) {
			t.Errorf("missing %s in\n%s", want, out)
		}
	}
	d, err := DecodeFault(out)
	if err != nil {
		t.Fatal(err)
	}
	if d.Code != "s:Receiver" || d.Subcode != "s:Authorization" || d.Reason != "device cap reached" {
		t.Errorf("decoded = %+v", d)
	}
	if d.Detail == nil || d.Detail.ErrorType != "DeviceCapReached" || d.Detail.TraceID != "2493ee37-beeb-4cb9-833c-cadde9067645" {
		t.Errorf("detail = %+v", d.Detail)
	}
	if !strings.Contains(d.Error(), "DeviceCapReached") || !strings.Contains(f.Error(), "0x80180003") || !strings.Contains(f.Error(), "0x80180013") {
		t.Errorf("Error() = %q / %q", d.Error(), f.Error())
	}
	if !errors.Is(f, f.Cause) {
		t.Error("Unwrap lost the cause")
	}
}

func TestFaultWithoutDetail(t *testing.T) {
	t.Parallel()
	out, err := EncodeFault(NewFault(SubcodeMessageFormat, "bad"), "a", "r")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "s:Detail") || strings.Contains(string(out), "ActivityId") {
		t.Errorf("unexpected detail: %s", out)
	}
	d, err := DecodeFault(out)
	if err != nil || d.Detail != nil || d.Subcode != "s:MessageFormat" {
		t.Errorf("decoded %+v, %v", d, err)
	}
	if d.Error() != "soap fault s:MessageFormat: bad" {
		t.Errorf("Error() = %q", d.Error())
	}
}

func TestEncodeFaultRejects(t *testing.T) {
	t.Parallel()
	if _, err := EncodeFault(nil, "a", "r"); !errors.Is(err, ErrResponse) {
		t.Errorf("nil: %v", err)
	}
	if _, err := EncodeFault(NewFault("s:Nope", "x"), "a", "r"); !errors.Is(err, ErrResponse) {
		t.Errorf("subcode: %v", err)
	}
	if _, err := EncodeFault(NewFault(SubcodeAuthorization, "x").WithDetail("Nope", ""), "a", "r"); !errors.Is(err, ErrResponse) {
		t.Errorf("detail: %v", err)
	}
	if _, err := EncodeFault(NewFault(SubcodeAuthorization, "x"), "", "r"); !errors.Is(err, ErrResponse) {
		t.Errorf("action: %v", err)
	}
}

func TestDecodeFault(t *testing.T) {
	t.Parallel()
	if _, err := DecodeFault([]byte("<nope")); !errors.Is(err, ErrSyntax) {
		t.Errorf("syntax: %v", err)
	}
	ok, err := Encode(Response{Action: "a", RelatesTo: "r", Body: []byte("<B/>")})
	if err != nil {
		t.Fatal(err)
	}
	d, err := DecodeFault(ok)
	if err != nil || d != nil {
		t.Errorf("non-fault: %+v, %v", d, err)
	}
}

func TestAsFault(t *testing.T) {
	t.Parallel()
	if AsFault(nil) != nil {
		t.Error("nil should stay nil")
	}
	f := NewFault(SubcodeAuthentication, "no")
	wrapped := errors.Join(errors.New("outer"), f)
	if got := AsFault(wrapped); got != f {
		t.Errorf("AsFault(wrapped) = %v", got)
	}
	plain := errors.New("boom")
	got := AsFault(plain)
	if got.Subcode != SubcodeInternalServiceFault || !errors.Is(got, plain) {
		t.Errorf("AsFault(plain) = %+v", got)
	}
}

func TestHResultTables(t *testing.T) {
	t.Parallel()
	subs := map[Subcode]uint32{
		SubcodeMessageFormat: 0x80180001, SubcodeAuthentication: 0x80180002, SubcodeAuthorization: 0x80180003,
		SubcodeCertificateRequest: 0x80180004, SubcodeEnrollmentServer: 0x80180005,
		SubcodeInternalServiceFault: 0x80180006, SubcodeInvalidSecurity: 0x80180007,
	}
	for s, want := range subs {
		if s.HResult() != want || !s.Valid() {
			t.Errorf("%s = %#x", s, s.HResult())
		}
	}
	if Subcode("x").Valid() {
		t.Error("unknown subcode valid")
	}
	types := map[ErrorType]uint32{
		ErrorDeviceCapReached: 0x80180013, ErrorDeviceNotSupported: 0x80180014, ErrorNotSupported: 0x80180015,
		ErrorNotEligibleToRenew: 0x80180016, ErrorInMaintenance: 0x80180017, ErrorUserLicense: 0x80180018,
		ErrorInvalidEnrollmentData: 0x80180019, ErrorCustomServerError: 0x80180032,
	}
	for e, want := range types {
		if e.HResult() != want || !e.Valid() {
			t.Errorf("%s = %#x", e, e.HResult())
		}
	}
	if ErrorType("x").Valid() {
		t.Error("unknown type valid")
	}
}
