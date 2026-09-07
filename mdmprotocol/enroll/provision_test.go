package enroll

import (
	"context"
	"crypto/x509"
	"errors"
	"testing"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/wapprov"
	"github.com/deploymenttheory/go-microsoft-dm/testpki"
)

func provisionEnrollment(t *testing.T, chainLen int, et EnrollmentType) *Enrollment {
	t.Helper()
	root, err := testpki.NewCA("Root")
	if err != nil {
		t.Fatal(err)
	}
	inter, err := testpki.NewCA("Intermediate")
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := root.Issue("device", t0)
	if err != nil {
		t.Fatal(err)
	}
	chain := []*x509.Certificate{root.Cert}
	if chainLen == 2 {
		chain = []*x509.Certificate{inter.Cert, root.Cert}
	}
	if chainLen == 0 {
		chain = nil
	}
	store := wapprov.StoreUser
	if et == EnrollmentTypeDevice {
		store = wapprov.StoreSystem
	}
	return &Enrollment{
		Request:     &Request{Context: AdditionalContext{DeviceID: "d1", DeviceName: "PC-1", EnrollmentType: et}, Principal: Principal{UPN: "u@c"}},
		Certificate: leaf.Cert, Chain: chain, Store: store, EnrolledAt: t0,
	}
}

func TestNewProvisionerRejects(t *testing.T) {
	t.Parallel()
	good := ProvisionConfig{ManagementURL: "https://m", ProviderID: "P", Credentials: CredentialSourceFunc(staticCreds)}
	badPoll := wapprov.Poll{NumberOfFirstRetries: 0}
	cases := map[string]func(c *ProvisionConfig){
		"no url":         func(c *ProvisionConfig) { c.ManagementURL = "" },
		"no provider":    func(c *ProvisionConfig) { c.ProviderID = "" },
		"no credentials": func(c *ProvisionConfig) { c.Credentials = nil },
		"bad poll":       func(c *ProvisionConfig) { c.Poll = &badPoll },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c := good
			mutate(&c)
			if _, err := NewProvisioner(c); !errors.Is(err, ErrConfig) {
				t.Errorf("err = %v", err)
			}
		})
	}
	p, err := NewProvisioner(good)
	if err != nil {
		t.Fatal(err)
	}
	if p.cfg.Renew == nil || p.cfg.Poll == nil || *p.cfg.Renew != *DefaultRenew() {
		t.Error("defaults")
	}
}

func TestProvisionShapes(t *testing.T) {
	t.Parallel()
	p, err := NewProvisioner(ProvisionConfig{ManagementURL: "https://m", ProviderID: "P", Credentials: CredentialSourceFunc(staticCreds),
		IncludeRootCATrusted: true, EntDMID: func(e *Enrollment) string { return "dm-" + e.Request.Context.DeviceID },
		Application: wapprov.ApplicationConfig{ConnRetryFreq: 6, DefaultEncoding: wapprov.EncodingXML}})
	if err != nil {
		t.Fatal(err)
	}
	e := provisionEnrollment(t, 2, EnrollmentTypeDevice)
	doc, err := p.Provision(context.Background(), e)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Characteristics) != 4 {
		t.Fatalf("characteristics = %d", len(doc.Characteristics))
	}
	cs := doc.Find(wapprov.TypeCertificateStore)
	if cs.Path("Root", "System", wapprov.Thumbprint(e.Chain[1].Raw)) == nil || cs.Path("CA", "System", wapprov.Thumbprint(e.Chain[0].Raw)) == nil {
		t.Error("chain split")
	}
	if cs.Path("My", "System", wapprov.Thumbprint(e.Certificate.Raw)) == nil {
		t.Error("client under My/System")
	}
	if doc.Find(wapprov.TypeRootCATrustedCertificates).Path("Root", "System", wapprov.Thumbprint(e.Chain[1].Raw)) == nil {
		t.Error("RootCATrustedCertificates")
	}
	app := doc.Find(wapprov.TypeApplication)
	if app.Value("CONNRETRYFREQ") != "6" || app.Value("DEFAULTENCODING") != string(wapprov.EncodingXML) || app.Value("ADDR") != "https://m" {
		t.Errorf("application = %+v", app.Parms)
	}
	prov := doc.Find(wapprov.TypeDMClient).Path("Provider", "P")
	if prov.Value("EntDMID") != "dm-d1" || prov.Value("EntDeviceName") != "PC-1" {
		t.Errorf("provider = %+v", prov.Parms)
	}
	if _, ok := prov.Parm("UPN"); ok {
		t.Error("device enrollment has UPN")
	}
	full := provisionEnrollment(t, 1, EnrollmentTypeFull)
	doc, err = p.Provision(context.Background(), full)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Find(wapprov.TypeDMClient).Path("Provider", "P").Value("UPN") != "u@c" || doc.Find(wapprov.TypeCertificateStore).Child("CA") != nil {
		t.Error("full enrollment")
	}
}

func TestProvisionErrors(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	base := ProvisionConfig{ManagementURL: "https://m", ProviderID: "P", Credentials: CredentialSourceFunc(staticCreds)}
	p, err := NewProvisioner(base)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Provision(context.Background(), nil); !errors.Is(err, ErrConfig) {
		t.Errorf("nil: %v", err)
	}
	if _, err := p.Provision(context.Background(), &Enrollment{Certificate: provisionEnrollment(t, 1, EnrollmentTypeFull).Certificate}); !errors.Is(err, ErrConfig) {
		t.Errorf("no request: %v", err)
	}
	if _, err := p.Provision(context.Background(), provisionEnrollment(t, 0, EnrollmentTypeFull)); !errors.Is(err, ErrConfig) {
		t.Errorf("no chain: %v", err)
	}
	// Three certificates in the chain means two intermediates, which the
	// CertificateStore builder refuses.
	e := provisionEnrollment(t, 2, EnrollmentTypeFull)
	e.Chain = append([]*x509.Certificate{e.Chain[0]}, e.Chain...)
	if _, err := p.Provision(context.Background(), e); !errors.Is(err, wapprov.ErrInvalid) {
		t.Errorf("two intermediates: %v", err)
	}
	c := base
	c.Credentials = CredentialSourceFunc(func(context.Context, *Enrollment) (wapprov.Credential, wapprov.Credential, error) {
		return wapprov.Credential{}, wapprov.Credential{}, boom
	})
	p, _ = NewProvisioner(c)
	if _, err := p.Provision(context.Background(), provisionEnrollment(t, 1, EnrollmentTypeFull)); !errors.Is(err, boom) {
		t.Errorf("credentials: %v", err)
	}
	c = base
	c.Credentials = CredentialSourceFunc(func(context.Context, *Enrollment) (wapprov.Credential, wapprov.Credential, error) {
		return wapprov.Credential{Type: wapprov.AuthBasic, Name: "n", Secret: "s"}, wapprov.Credential{Type: wapprov.AuthBasic, Secret: "s"}, nil
	})
	p, _ = NewProvisioner(c)
	if _, err := p.Provision(context.Background(), provisionEnrollment(t, 1, EnrollmentTypeFull)); !errors.Is(err, wapprov.ErrInvalid) {
		t.Errorf("client basic: %v", err)
	}
	c = base
	c.EntDMID = func(*Enrollment) string { return "" }
	c.Application = wapprov.ApplicationConfig{ProtoVer: "9"}
	p, _ = NewProvisioner(c)
	if _, err := p.Provision(context.Background(), provisionEnrollment(t, 1, EnrollmentTypeFull)); !errors.Is(err, wapprov.ErrInvalid) {
		t.Errorf("bad application: %v", err)
	}
}
