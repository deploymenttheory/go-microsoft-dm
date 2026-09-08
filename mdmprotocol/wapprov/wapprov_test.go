package wapprov

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

var (
	rootDER   = []byte("root-der")
	interDER  = []byte("inter-der")
	clientDER = []byte("client-der")
)

func TestThumbprint(t *testing.T) {
	t.Parallel()
	if got := Thumbprint([]byte("abc")); got != "A9993E364706816ABA3E25717850C26C9CD0D89D" {
		t.Errorf("Thumbprint = %s", got)
	}
}

func TestCertificateStoreShape(t *testing.T) {
	t.Parallel()
	cs, err := CertificateStore(CertificateStoreConfig{
		Roots: [][]byte{rootDER}, Intermediates: [][]byte{interDER}, Client: clientDER, Store: StoreUser,
		Renew: &Renew{ROBOSupport: true, RenewPeriod: 60, RetryInterval: 4, ServerURL: "https://r.example/renew"},
	})
	if err != nil {
		t.Fatal(err)
	}
	root := cs.Path("Root", "System", Thumbprint(rootDER))
	if root == nil || root.Value("EncodedCertificate") != base64.StdEncoding.EncodeToString(rootDER) {
		t.Errorf("root entry missing: %+v", cs)
	}
	if cs.Path("CA", "System", Thumbprint(interDER)) == nil {
		t.Error("intermediate entry missing")
	}
	user := cs.Path("My", "User")
	if user == nil || user.Child(Thumbprint(clientDER)) == nil || user.Child("PrivateKeyContainer") == nil {
		t.Errorf("My/User incomplete: %+v", user)
	}
	renew := cs.Path("My", "WSTEP", "Renew")
	if renew == nil {
		t.Fatal("renew missing")
	}
	for name, want := range map[string][2]string{
		"ROBOSupport": {"true", TypeBoolean}, "RenewPeriod": {"60", TypeInteger},
		"RetryInterval": {"4", TypeInteger}, "ServerURL": {"https://r.example/renew", TypeString},
	} {
		p, ok := renew.Parm(name)
		if !ok || p.Value != want[0] || p.DataType != want[1] {
			t.Errorf("%s = %+v", name, p)
		}
	}
	// Device enrollment goes to My/System and has no CA node without an intermediate.
	cs, err = CertificateStore(CertificateStoreConfig{Roots: [][]byte{rootDER}, Client: clientDER, Store: StoreSystem})
	if err != nil {
		t.Fatal(err)
	}
	if cs.Path("My", "System") == nil || cs.Child("CA") != nil || cs.Path("My", "WSTEP") != nil {
		t.Errorf("device store wrong: %+v", cs)
	}
}

func TestCertificateStoreRejects(t *testing.T) {
	t.Parallel()
	good := CertificateStoreConfig{Roots: [][]byte{rootDER}, Client: clientDER, Store: StoreUser}
	cases := map[string]func(c *CertificateStoreConfig){
		"no root":           func(c *CertificateStoreConfig) { c.Roots = nil },
		"two intermediates": func(c *CertificateStoreConfig) { c.Intermediates = [][]byte{interDER, interDER} },
		"no client":         func(c *CertificateStoreConfig) { c.Client = nil },
		"bad store":         func(c *CertificateStoreConfig) { c.Store = "Machine" },
		"empty root":        func(c *CertificateStoreConfig) { c.Roots = [][]byte{{}} },
		"bad renew":         func(c *CertificateStoreConfig) { c.Renew = &Renew{RenewPeriod: 0, RetryInterval: 4} },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c := good
			mutate(&c)
			if _, err := CertificateStore(c); !errors.Is(err, ErrInvalid) {
				t.Errorf("err = %v", err)
			}
		})
	}
}

func TestRootCATrustedCertificates(t *testing.T) {
	t.Parallel()
	c, err := RootCATrustedCertificates([][]byte{rootDER})
	if err != nil || c.Path("Root", Thumbprint(rootDER)) == nil {
		t.Errorf("%+v, %v", c, err)
	}
	if _, err := RootCATrustedCertificates(nil); !errors.Is(err, ErrInvalid) {
		t.Error("nil accepted")
	}
	if _, err := RootCATrustedCertificates([][]byte{nil}); !errors.Is(err, ErrInvalid) {
		t.Error("empty accepted")
	}
}

// This CSP's management tree differs from CertificateStore/Root/System.
func TestRootCATrustedCertificatesHasNoSystemContainer(t *testing.T) {
	t.Parallel()
	c, err := RootCATrustedCertificates([][]byte{rootDER})
	if err != nil {
		t.Fatal(err)
	}
	root := c.Child("Root")
	if root == nil || len(root.Children) != 1 || root.Child("System") != nil {
		t.Fatal("MS-MDE2 2.2.9.4 requires Root/CertHash without a System container")
	}
	cert := root.Child(Thumbprint(rootDER))
	if cert == nil || cert.Value("EncodedCertificate") != base64.StdEncoding.EncodeToString(rootDER) {
		t.Fatal("missing base64 DER at Root/CertHash/EncodedCertificate")
	}
}

func appConfig() ApplicationConfig {
	return ApplicationConfig{
		ProviderID: "TestServer", Name: "Test", Address: "https://mdm.example/ManagementServer/MDM.svc",
		ConnRetryFreq: 6, InitialBackoffTime: 30000, MaxBackoffTime: 120000, BackCompatRetryDisabled: true,
		DefaultEncoding: EncodingXML, ProtoVer: "1.2", UseHWDevID: true,
		SSLClientCertSearchCriteria: SearchCriteria{Subject: "CN=Tester,O=Microsoft", Stores: []string{StoreMyUser}}.String(),
		ServerAuth:                  Credential{Type: AuthBasic, Name: "dev1", Secret: "s3cret"},
		ClientAuth:                  Credential{Type: AuthDigest, Secret: "srvsecret", Nonce: []byte{1, 2, 3}},
	}
}

func TestApplicationShape(t *testing.T) {
	t.Parallel()
	app, err := Application(appConfig())
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"APPID": "w7", "PROVIDER-ID": "TestServer", "NAME": "Test", "ADDR": "https://mdm.example/ManagementServer/MDM.svc",
		"PROTOVER": "1.2", "CONNRETRYFREQ": "6", "INITIALBACKOFFTIME": "30000", "MAXBACKOFFTIME": "120000",
		"DEFAULTENCODING":             "application/vnd.syncml.dm+xml",
		"SSLCLIENTCERTSEARCHCRITERIA": "Subject=CN%3DTester,O%3DMicrosoft&Stores=My%5CUser",
	}
	for k, v := range want {
		if got := app.Value(k); got != v {
			t.Errorf("%s = %q, want %q", k, got, v)
		}
	}
	for _, flag := range []string{"BACKCOMPATRETRYDISABLED", "USEHWDEVID"} {
		p, ok := app.Parm(flag)
		if !ok || !p.Flag {
			t.Errorf("%s = %+v", flag, p)
		}
	}
	if len(app.Children) != 2 {
		t.Fatalf("children = %d", len(app.Children))
	}
	srv, cli := app.Children[0], app.Children[1]
	if srv.Value("AAUTHLEVEL") != "APPSRV" || srv.Value("AAUTHTYPE") != "BASIC" || srv.Value("AAUTHNAME") != "dev1" || srv.Value("AAUTHSECRET") != "s3cret" {
		t.Errorf("APPSRV = %+v", srv)
	}
	if _, ok := srv.Parm("AAUTHDATA"); ok {
		t.Error("APPSRV without nonce should not carry AAUTHDATA")
	}
	if cli.Value("AAUTHLEVEL") != "CLIENT" || cli.Value("AAUTHTYPE") != "DIGEST" || cli.Value("AAUTHSECRET") != "srvsecret" || cli.Value("AAUTHDATA") != "AQID" {
		t.Errorf("CLIENT = %+v", cli)
	}
	if _, ok := cli.Parm("AAUTHNAME"); ok {
		t.Error("CLIENT without name should not carry AAUTHNAME")
	}
	// Minimal config omits every optional parm.
	app, err = Application(ApplicationConfig{
		ProviderID: "P", Address: "https://x", ServerAuth: Credential{Type: AuthDigest, Name: "n", Secret: "s"},
		ClientAuth: Credential{Type: AuthDigest, Secret: "s"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(app.Parms) != 3 {
		t.Errorf("minimal parms = %+v", app.Parms)
	}
}

func TestApplicationRejects(t *testing.T) {
	t.Parallel()
	cases := map[string]func(c *ApplicationConfig){
		"no provider":      func(c *ApplicationConfig) { c.ProviderID = "" },
		"no address":       func(c *ApplicationConfig) { c.Address = "" },
		"bad protover":     func(c *ApplicationConfig) { c.ProtoVer = "2.0" },
		"bad encoding":     func(c *ApplicationConfig) { c.DefaultEncoding = "text/plain" },
		"server type":      func(c *ApplicationConfig) { c.ServerAuth.Type = "NTLM" },
		"server no name":   func(c *ApplicationConfig) { c.ServerAuth.Name = "" },
		"server no secret": func(c *ApplicationConfig) { c.ServerAuth.Secret = "" },
		"client basic":     func(c *ApplicationConfig) { c.ClientAuth.Type = AuthBasic },
		"client no secret": func(c *ApplicationConfig) { c.ClientAuth.Secret = "" },
		"client no type":   func(c *ApplicationConfig) { c.ClientAuth.Type = "" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c := appConfig()
			mutate(&c)
			if _, err := Application(c); !errors.Is(err, ErrInvalid) {
				t.Errorf("err = %v", err)
			}
		})
	}
}

func TestSearchCriteria(t *testing.T) {
	t.Parallel()
	// The example in MS-MDE2 2.2.9.5.
	got := SearchCriteria{Subject: "CN=Tester,O=Microsoft", Stores: []string{StoreMyUser}}.String()
	if got != "Subject=CN%3DTester,O%3DMicrosoft&Stores=My%5CUser" {
		t.Errorf("got %s", got)
	}
	got = SearchCriteria{Stores: []string{"My\\User", "My\\System"}}.String()
	if got != "Stores=My%5CUser%EF%80%80My%5CSystem" {
		t.Errorf("got %s", got)
	}
	if (SearchCriteria{}).String() != "" {
		t.Error("empty criteria")
	}
	if got := (SearchCriteria{Subject: "a b&c"}).String(); got != "Subject=a%20b%26c" {
		t.Errorf("escape: %s", got)
	}
}

func TestDMClientShape(t *testing.T) {
	t.Parallel()
	dm, err := DMClient(DMClientConfig{ProviderID: "TestServer", UPN: "user@contoso.com", EntDeviceName: "PC-1", EntDMID: "dm-1",
		Extra: []Parm{{Name: "HelpPhone", Value: "1234", DataType: TypeString}}})
	if err != nil {
		t.Fatal(err)
	}
	prov := dm.Path("Provider", "TestServer")
	if prov == nil {
		t.Fatalf("provider missing: %+v", dm)
	}
	for k, v := range map[string]string{"UPN": "user@contoso.com", "EntDeviceName": "PC-1", "EntDMID": "dm-1", "HelpPhone": "1234"} {
		if p, ok := prov.Parm(k); !ok || p.Value != v || p.DataType != TypeString {
			t.Errorf("%s = %+v", k, p)
		}
	}
	poll := prov.Child("Poll")
	want := map[string]string{
		"IntervalForFirstSetOfRetries": "15", "NumberOfFirstRetries": "5",
		"IntervalForSecondSetOfRetries": "60", "NumberOfSecondRetries": "10",
		"IntervalForRemainingScheduledRetries": "1440", "NumberOfRemainingScheduledRetries": "0",
	}
	if len(poll.Parms) != len(want) {
		t.Errorf("poll parms = %+v", poll.Parms)
	}
	for k, v := range want {
		if p, ok := poll.Parm(k); !ok || p.Value != v || p.DataType != TypeInteger {
			t.Errorf("%s = %+v", k, p)
		}
	}
	custom := DefaultPoll()
	custom.PollOnLogin, custom.AllUsersPollOnFirstLogin = true, true
	dm, err = DMClient(DMClientConfig{ProviderID: "P", Poll: &custom})
	if err != nil {
		t.Fatal(err)
	}
	poll = dm.Path("Provider", "P", "Poll")
	if poll.Value("PollOnLogin") != "true" || poll.Value("AllUsersPollOnFirstLogin") != "true" {
		t.Errorf("login polls: %+v", poll.Parms)
	}
	if len(dm.Path("Provider", "P").Parms) != 0 {
		t.Error("device enrollment without UPN should have no provider parms")
	}
}

func TestDMClientRejects(t *testing.T) {
	t.Parallel()
	if _, err := DMClient(DMClientConfig{}); !errors.Is(err, ErrInvalid) {
		t.Errorf("no provider: %v", err)
	}
	if _, err := DMClient(DMClientConfig{ProviderID: "P", Extra: []Parm{{Value: "x"}}}); !errors.Is(err, ErrInvalid) {
		t.Errorf("unnamed extra: %v", err)
	}
	// Fleet's one-minute schedule breaks the CSP's rule against NumberOfFirstRetries 0.
	fleet := Poll{IntervalForFirstSetOfRetries: 1, NumberOfFirstRetries: 0}
	if _, err := DMClient(DMClientConfig{ProviderID: "P", Poll: &fleet}); !errors.Is(err, ErrInvalid) {
		t.Errorf("fleet poll: %v", err)
	}
	cases := map[string]Poll{
		"negative":        {IntervalForFirstSetOfRetries: 15, NumberOfFirstRetries: 5, IntervalForSecondSetOfRetries: -1, IntervalForRemainingScheduledRetries: 1440},
		"zero first":      {IntervalForFirstSetOfRetries: 0, NumberOfFirstRetries: 5, IntervalForRemainingScheduledRetries: 1440},
		"short remaining": {IntervalForFirstSetOfRetries: 15, NumberOfFirstRetries: 5, IntervalForRemainingScheduledRetries: 60},
	}
	for name, p := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if err := p.Validate(); !errors.Is(err, ErrInvalid) {
				t.Errorf("err = %v", err)
			}
		})
	}
	if err := DefaultPoll().Validate(); err != nil {
		t.Error(err)
	}
}

func fullDocument(t *testing.T) *Document {
	t.Helper()
	cs, err := CertificateStore(CertificateStoreConfig{Roots: [][]byte{rootDER}, Client: clientDER, Store: StoreUser,
		Renew: &Renew{ROBOSupport: true, RenewPeriod: 60, RetryInterval: 4}})
	if err != nil {
		t.Fatal(err)
	}
	app, err := Application(appConfig())
	if err != nil {
		t.Fatal(err)
	}
	dm, err := DMClient(DMClientConfig{ProviderID: "TestServer", UPN: "user@contoso.com"})
	if err != nil {
		t.Fatal(err)
	}
	return &Document{Version: Version, Characteristics: []Characteristic{cs, app, dm}}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	t.Parallel()
	doc := fullDocument(t)
	for _, o := range []Options{{}, {Indent: true, XMLDeclaration: true}} {
		out, err := Encode(doc, o)
		if err != nil {
			t.Fatal(err)
		}
		if o.XMLDeclaration != strings.HasPrefix(string(out), `<?xml version="1.0" encoding="UTF-8"?>`) {
			t.Errorf("declaration mismatch: %.60s", out)
		}
		if o.Indent != strings.Contains(string(out), "\n  <characteristic") {
			t.Errorf("indent mismatch")
		}
		for _, want := range []string{
			`<wap-provisioningdoc version="1.1">`,
			`<characteristic type="CertificateStore">`,
			`<parm name="BACKCOMPATRETRYDISABLED"/>`,
			`<parm name="ROBOSupport" value="true" datatype="boolean"/>`,
			`<characteristic type="PrivateKeyContainer"/>`,
		} {
			if !strings.Contains(string(out), want) {
				t.Errorf("missing %s", want)
			}
		}
		back, err := Decode(out)
		if err != nil {
			t.Fatal(err)
		}
		again, err := Encode(back, o)
		if err != nil {
			t.Fatal(err)
		}
		if string(again) != string(out) {
			t.Errorf("round trip differs:\n%s\n%s", out, again)
		}
		if back.Find(TypeApplication) == nil || len(back.FindAll(TypeCertificateStore)) != 1 || back.Find("Nope") != nil {
			t.Error("lookups after decode")
		}
		p, ok := back.Find(TypeApplication).Parm("BACKCOMPATRETRYDISABLED")
		if !ok || !p.Flag {
			t.Errorf("flag lost: %+v", p)
		}
	}
}

// TestDecodeMicrosoftExample reads the shape of the MS-MDE2 2.2.9.1 sample.
func TestDecodeMicrosoftExample(t *testing.T) {
	t.Parallel()
	const sample = `<wap-provisioningdoc version="1.1">
  <characteristic type="CertificateStore">
    <characteristic type="Root">
      <characteristic type="System">
        <characteristic type="031336C933CC7E228B88880D78824FB2909A0A2F">
          <parm name="EncodedCertificate" value="B64EncodedCertInsertedHere" />
        </characteristic>
      </characteristic>
    </characteristic>
    <characteristic type="My">
      <characteristic type="User">
        <characteristic type="F9A4F20FC50D990FDD0E3DB9AFCBF401818D5462">
          <parm name="EncodedCertificate" value="B64EncodedCertInsertedHere" />
        </characteristic>
        <characteristic type="PrivateKeyContainer" />
      </characteristic>
      <characteristic type="WSTEP">
        <characteristic type="Renew">
          <parm name="ROBOSupport" value="true" datatype="boolean" />
          <parm name="RenewPeriod" value="60" datatype="integer" />
          <parm name="RetryInterval" value="4" datatype="integer" />
        </characteristic>
      </characteristic>
    </characteristic>
  </characteristic>
  <characteristic type="APPLICATION">
    <parm name="APPID" value="w7" />
    <parm name="PROVIDER-ID" value="TestMDMServer" />
    <parm name="NAME" value="Microsoft" />
    <parm name="ADDR" value="https://DM.contoso.com:443/omadm/Windows.ashx" />
    <parm name="CONNRETRYFREQ" value="6" />
    <parm name="INITIALBACKOFFTIME" value="30000" />
    <parm name="MAXBACKOFFTIME" value="120000" />
    <parm name="BACKCOMPATRETRYDISABLED" />
    <parm name="DEFAULTENCODING" value="application/vnd.syncml.dm+wbxml" />
    <parm name="SSLCLIENTCERTSEARCHCRITERIA" value="Subject=DC%3dcom%2cDC%3dmicrosoft%2cCN%3dUsers%2cCN%3dAdministrator&amp;Stores=My%5CUser" />
    <characteristic type="APPAUTH">
      <parm name="AAUTHLEVEL" value="CLIENT" />
      <parm name="AAUTHTYPE" value="DIGEST" />
      <parm name="AAUTHSECRET" value="password1" />
      <parm name="AAUTHDATA" value="B64encodedBinaryNonceInsertedHere" />
    </characteristic>
    <characteristic type="APPAUTH">
      <parm name="AAUTHLEVEL" value="APPSRV" />
      <parm name="AAUTHTYPE" value="BASIC" />
      <parm name="AAUTHNAME" value="testclient" />
      <parm name="AAUTHSECRET" value="password2" />
    </characteristic>
  </characteristic>
  <characteristic type="DMClient">
    <characteristic type="Provider">
      <characteristic type="TestMDMServer">
        <parm name="UPN" value="UserPrincipalName@contoso.com" datatype="string" />
        <characteristic type="Poll">
          <parm name="NumberOfFirstRetries" value="8" datatype="integer" />
        </characteristic>
      </characteristic>
    </characteristic>
  </characteristic>
</wap-provisioningdoc>`
	doc, err := Decode([]byte(sample))
	if err != nil {
		t.Fatal(err)
	}
	if doc.Version != "1.1" || len(doc.Characteristics) != 3 {
		t.Fatalf("doc = %+v", doc)
	}
	if got := doc.Find(TypeApplication).Value("SSLCLIENTCERTSEARCHCRITERIA"); !strings.HasSuffix(got, "&Stores=My%5CUser") {
		t.Errorf("criteria = %q", got)
	}
	if doc.Find(TypeDMClient).Path("Provider", "TestMDMServer", "Poll").Value("NumberOfFirstRetries") != "8" {
		t.Error("poll lookup")
	}
	if doc.Find(TypeCertificateStore).Path("My", "User", "PrivateKeyContainer") == nil {
		t.Error("PrivateKeyContainer")
	}
	var nilDoc *Document
	if nilDoc.Find("x") != nil || nilDoc.FindAll("x") != nil {
		t.Error("nil document lookups")
	}
	var nilCh *Characteristic
	if nilCh.Child("x") != nil || nilCh.Value("x") != "" || nilCh.Path("a", "b") != nil {
		t.Error("nil characteristic lookups")
	}
}

func TestValidateAndDecodeRejects(t *testing.T) {
	t.Parallel()
	cases := map[string]*Document{
		"nil":            nil,
		"no version":     {Characteristics: []Characteristic{{Type: "X"}}},
		"empty":          {Version: "1.1"},
		"untyped":        {Version: "1.1", Characteristics: []Characteristic{{Type: "X", Children: []Characteristic{{}}}}},
		"unnamed parm":   {Version: "1.1", Characteristics: []Characteristic{{Type: "X", Parms: []Parm{{Value: "v"}}}}},
		"lowercase parm": {Version: "1.1", Characteristics: []Characteristic{{Type: TypeApplication, Parms: []Parm{{Name: "Appid", Value: "w7"}}}}},
		"lowercase child": {Version: "1.1", Characteristics: []Characteristic{{Type: TypeApplication, Children: []Characteristic{
			{Type: "AppAuth", Parms: []Parm{{Name: "AAUTHLEVEL", Value: "CLIENT"}}},
		}}}},
		"lowercase nested parm": {Version: "1.1", Characteristics: []Characteristic{{Type: TypeApplication, Children: []Characteristic{
			{Type: "APPAUTH", Parms: []Parm{{Name: "aauthlevel", Value: "CLIENT"}}},
		}}}},
	}
	for name, d := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if err := Validate(d); !errors.Is(err, ErrInvalid) {
				t.Errorf("Validate: %v", err)
			}
			if _, err := Encode(d, Options{}); !errors.Is(err, ErrInvalid) {
				t.Errorf("Encode: %v", err)
			}
		})
	}
	if _, err := Decode([]byte("<wap-provisioningdoc")); !errors.Is(err, ErrSyntax) {
		t.Errorf("syntax: %v", err)
	}
	if _, err := Decode([]byte(`<wap-provisioningdoc version="1.1"/>`)); !errors.Is(err, ErrInvalid) {
		t.Errorf("empty: %v", err)
	}
	// Mixed case is fine outside APPLICATION.
	if err := Validate(&Document{Version: "1.1", Characteristics: []Characteristic{{Type: "DMClient", Parms: []Parm{{Name: "EntDMID"}}}}}); err != nil {
		t.Error(err)
	}
}
