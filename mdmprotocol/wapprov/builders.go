package wapprov

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
)

// Characteristic types the builders emit.
const (
	TypeCertificateStore          = "CertificateStore"
	TypeApplication               = "APPLICATION"
	TypeDMClient                  = "DMClient"
	TypeRootCATrustedCertificates = "RootCATrustedCertificates"
)

// Store selects the certificate store for the enrolled identity
// (MS-MDE2 2.2.9.2): My/User for a user (EnrollmentType Full) enrollment,
// My/System for a device enrollment.
type Store string

// The two stores an enrollment identity lands in.
const (
	StoreUser   Store = "User"
	StoreSystem Store = "System"
)

// Renew is the My/WSTEP/Renew characteristic. Periods are in days.
type Renew struct {
	// ROBOSupport enables renew-on-behalf-of (the client renews its own
	// certificate silently).
	ROBOSupport bool
	// RenewPeriod is how many days before expiry renewal starts. Microsoft's
	// example uses 60; the plan recommends 40 to 60.
	RenewPeriod int
	// RetryInterval is the days between renewal attempts; 4 to 5 recommended.
	RetryInterval int
	// ServerURL is the renewal endpoint (Phase 9); empty omits the parm.
	ServerURL string
}

// CertificateStoreConfig describes the CertificateStore characteristic.
type CertificateStoreConfig struct {
	// Root certificates in DER, installed under Root/System. At least one.
	Roots [][]byte
	// Intermediates in DER, installed under CA/System. At most one.
	Intermediates [][]byte
	// Client is the issued identity certificate in DER.
	Client []byte
	// Store is where Client goes: StoreUser or StoreSystem.
	Store Store
	// Renew, when set, adds My/WSTEP/Renew.
	Renew *Renew
}

// CertificateStore builds the CertificateStore characteristic of MS-MDE2
// 2.2.9.2. Certificates are keyed by SHA-1 thumbprint as Windows expects.
func CertificateStore(cfg CertificateStoreConfig) (Characteristic, error) {
	if len(cfg.Roots) == 0 {
		return Characteristic{}, fmt.Errorf("%w: CertificateStore needs a root certificate", ErrInvalid)
	}
	if len(cfg.Intermediates) > 1 {
		return Characteristic{}, fmt.Errorf("%w: CertificateStore carries at most one intermediate", ErrInvalid)
	}
	if len(cfg.Client) == 0 {
		return Characteristic{}, fmt.Errorf("%w: CertificateStore needs the client certificate", ErrInvalid)
	}
	if cfg.Store != StoreUser && cfg.Store != StoreSystem {
		return Characteristic{}, fmt.Errorf("%w: store %q is not User or System", ErrInvalid, cfg.Store)
	}
	for _, der := range append(append([][]byte{}, cfg.Roots...), cfg.Intermediates...) {
		if len(der) == 0 {
			return Characteristic{}, fmt.Errorf("%w: empty certificate", ErrInvalid)
		}
	}
	root := Characteristic{Type: "Root", Children: []Characteristic{{Type: "System", Children: certEntries(cfg.Roots)}}}
	cs := Characteristic{Type: TypeCertificateStore, Children: []Characteristic{root}}
	if len(cfg.Intermediates) == 1 {
		cs.Children = append(cs.Children, Characteristic{
			Type: "CA", Children: []Characteristic{{Type: "System", Children: certEntries(cfg.Intermediates)}},
		})
	}
	my := Characteristic{Type: "My", Children: []Characteristic{{
		Type: string(cfg.Store),
		Children: append(certEntries([][]byte{cfg.Client}),
			Characteristic{Type: "PrivateKeyContainer"}),
	}}}
	if cfg.Renew != nil {
		r := cfg.Renew
		if r.RenewPeriod <= 0 || r.RetryInterval <= 0 {
			return Characteristic{}, fmt.Errorf("%w: Renew periods must be positive days", ErrInvalid)
		}
		renew := Characteristic{Type: "Renew", Parms: []Parm{
			{Name: "ROBOSupport", Value: strconv.FormatBool(r.ROBOSupport), DataType: TypeBoolean},
			{Name: "RenewPeriod", Value: strconv.Itoa(r.RenewPeriod), DataType: TypeInteger},
			{Name: "RetryInterval", Value: strconv.Itoa(r.RetryInterval), DataType: TypeInteger},
		}}
		if r.ServerURL != "" {
			renew.Parms = append(renew.Parms, Parm{Name: "ServerURL", Value: r.ServerURL, DataType: TypeString})
		}
		my.Children = append(my.Children, Characteristic{Type: "WSTEP", Children: []Characteristic{renew}})
	}
	cs.Children = append(cs.Children, my)
	return cs, nil
}

func certEntries(ders [][]byte) []Characteristic {
	out := make([]Characteristic, 0, len(ders))
	for _, der := range ders {
		out = append(out, Characteristic{
			Type:  Thumbprint(der),
			Parms: []Parm{{Name: "EncodedCertificate", Value: base64.StdEncoding.EncodeToString(der)}},
		})
	}
	return out
}

// RootCATrustedCertificates builds the RootCATrustedCertificates
// characteristic of MS-MDE2 2.2.9.4 for roots in DER, under Root/CertHash.
func RootCATrustedCertificates(roots [][]byte) (Characteristic, error) {
	if len(roots) == 0 {
		return Characteristic{}, fmt.Errorf("%w: RootCATrustedCertificates needs a certificate", ErrInvalid)
	}
	for _, der := range roots {
		if len(der) == 0 {
			return Characteristic{}, fmt.Errorf("%w: empty certificate", ErrInvalid)
		}
	}
	return Characteristic{Type: TypeRootCATrustedCertificates, Children: []Characteristic{
		{Type: "Root", Children: certEntries(roots)},
	}}, nil
}

// AuthType is the w7 AAUTHTYPE value.
type AuthType string

// The two AAUTHTYPE values.
const (
	AuthBasic  AuthType = "BASIC"  // syncml:auth-basic
	AuthDigest AuthType = "DIGEST" // syncml:auth-md5
)

// Encoding is the w7 DEFAULTENCODING value.
type Encoding string

// The two DEFAULTENCODING values.
const (
	EncodingXML   Encoding = "application/vnd.syncml.dm+xml"
	EncodingWBXML Encoding = "application/vnd.syncml.dm+wbxml"
)

// Credential is one APPAUTH characteristic.
type Credential struct {
	// Type is BASIC or DIGEST. The CLIENT level must be DIGEST.
	Type AuthType
	// Name is AAUTHNAME, the client name the server addresses; optional
	// for the CLIENT level.
	Name string
	// Secret is AAUTHSECRET, the shared secret. Required.
	Secret string
	// Nonce is AAUTHDATA, the first nonce for DIGEST, written base64.
	Nonce []byte
}

// ApplicationConfig describes the w7 APPLICATION characteristic.
type ApplicationConfig struct {
	// ProviderID is PROVIDER-ID, the server identifier the DMClient CSP
	// keys its provider node on. Required.
	ProviderID string
	// Name is NAME, the user-readable identity; optional.
	Name string
	// Address is ADDR, the management URL. Required.
	Address string
	// ConnRetryFreq, InitialBackoffTime and MaxBackoffTime tune the client's
	// connection retries; zero omits the parm and the client uses its
	// defaults (3, 16000 ms, 86400000 ms).
	ConnRetryFreq      int
	InitialBackoffTime int
	MaxBackoffTime     int
	// BackCompatRetryDisabled emits BACKCOMPATRETRYDISABLED so the client
	// never retries a package as SyncML 1.1.
	BackCompatRetryDisabled bool
	// DefaultEncoding is DEFAULTENCODING; empty omits it (the client
	// defaults to XML).
	DefaultEncoding Encoding
	// ProtoVer is PROTOVER, "1.1" or "1.2"; empty omits it.
	ProtoVer string
	// UseHWDevID emits USEHWDEVID.
	UseHWDevID bool
	// SSLClientCertSearchCriteria selects the client TLS certificate; see
	// SearchCriteria. Empty omits the parm.
	SSLClientCertSearchCriteria string
	// ServerAuth is the APPSRV credential: how the client authenticates to
	// the server. Required; BASIC or DIGEST.
	ServerAuth Credential
	// ClientAuth is the CLIENT credential: how the server authenticates to
	// the client. Required; DIGEST.
	ClientAuth Credential
}

// Application builds the w7 APPLICATION characteristic of MS-MDE2 2.2.9.5.
// ROLE is never set: the enterprise enrollment client fixes it.
func Application(cfg ApplicationConfig) (Characteristic, error) {
	if cfg.ProviderID == "" {
		return Characteristic{}, fmt.Errorf("%w: PROVIDER-ID is required", ErrInvalid)
	}
	if cfg.Address == "" {
		return Characteristic{}, fmt.Errorf("%w: ADDR is required", ErrInvalid)
	}
	if cfg.ProtoVer != "" && cfg.ProtoVer != "1.1" && cfg.ProtoVer != "1.2" {
		return Characteristic{}, fmt.Errorf("%w: PROTOVER %q is not 1.1 or 1.2", ErrInvalid, cfg.ProtoVer)
	}
	if cfg.DefaultEncoding != "" && cfg.DefaultEncoding != EncodingXML && cfg.DefaultEncoding != EncodingWBXML {
		return Characteristic{}, fmt.Errorf("%w: DEFAULTENCODING %q", ErrInvalid, cfg.DefaultEncoding)
	}
	srv, err := appAuth("APPSRV", cfg.ServerAuth)
	if err != nil {
		return Characteristic{}, err
	}
	cli, err := appAuth("CLIENT", cfg.ClientAuth)
	if err != nil {
		return Characteristic{}, err
	}
	app := Characteristic{Type: TypeApplication, Parms: []Parm{
		{Name: "APPID", Value: "w7"},
		{Name: "PROVIDER-ID", Value: cfg.ProviderID},
	}}
	if cfg.Name != "" {
		app.Parms = append(app.Parms, Parm{Name: "NAME", Value: cfg.Name})
	}
	app.Parms = append(app.Parms, Parm{Name: "ADDR", Value: cfg.Address})
	if cfg.ProtoVer != "" {
		app.Parms = append(app.Parms, Parm{Name: "PROTOVER", Value: cfg.ProtoVer})
	}
	if cfg.ConnRetryFreq > 0 {
		app.Parms = append(app.Parms, Parm{Name: "CONNRETRYFREQ", Value: strconv.Itoa(cfg.ConnRetryFreq)})
	}
	if cfg.InitialBackoffTime > 0 {
		app.Parms = append(app.Parms, Parm{Name: "INITIALBACKOFFTIME", Value: strconv.Itoa(cfg.InitialBackoffTime)})
	}
	if cfg.MaxBackoffTime > 0 {
		app.Parms = append(app.Parms, Parm{Name: "MAXBACKOFFTIME", Value: strconv.Itoa(cfg.MaxBackoffTime)})
	}
	if cfg.BackCompatRetryDisabled {
		app.Parms = append(app.Parms, Parm{Name: "BACKCOMPATRETRYDISABLED", Flag: true})
	}
	if cfg.DefaultEncoding != "" {
		app.Parms = append(app.Parms, Parm{Name: "DEFAULTENCODING", Value: string(cfg.DefaultEncoding)})
	}
	if cfg.UseHWDevID {
		app.Parms = append(app.Parms, Parm{Name: "USEHWDEVID", Flag: true})
	}
	if cfg.SSLClientCertSearchCriteria != "" {
		app.Parms = append(app.Parms, Parm{Name: "SSLCLIENTCERTSEARCHCRITERIA", Value: cfg.SSLClientCertSearchCriteria})
	}
	app.Children = []Characteristic{srv, cli}
	return app, nil
}

func appAuth(level string, c Credential) (Characteristic, error) {
	switch {
	case c.Type != AuthBasic && c.Type != AuthDigest:
		return Characteristic{}, fmt.Errorf("%w: %s AAUTHTYPE %q is not BASIC or DIGEST", ErrInvalid, level, c.Type)
	case level == "CLIENT" && c.Type != AuthDigest:
		return Characteristic{}, fmt.Errorf("%w: CLIENT AAUTHTYPE must be DIGEST", ErrInvalid)
	case c.Secret == "":
		return Characteristic{}, fmt.Errorf("%w: %s AAUTHSECRET is required", ErrInvalid, level)
	case level == "APPSRV" && c.Name == "":
		return Characteristic{}, fmt.Errorf("%w: APPSRV AAUTHNAME is required", ErrInvalid)
	}
	ch := Characteristic{Type: "APPAUTH", Parms: []Parm{
		{Name: "AAUTHLEVEL", Value: level},
		{Name: "AAUTHTYPE", Value: string(c.Type)},
	}}
	if c.Name != "" {
		ch.Parms = append(ch.Parms, Parm{Name: "AAUTHNAME", Value: c.Name})
	}
	ch.Parms = append(ch.Parms, Parm{Name: "AAUTHSECRET", Value: c.Secret})
	if len(c.Nonce) > 0 {
		ch.Parms = append(ch.Parms, Parm{Name: "AAUTHDATA", Value: base64.StdEncoding.EncodeToString(c.Nonce)})
	}
	return ch, nil
}

// SearchCriteria is the SSLCLIENTCERTSEARCHCRITERIA value: name=value pairs
// joined by "&", multiple values joined by U+F000, and characters outside
// the RFC 2396 unreserved set percent-encoded. The comma in a subject is
// left as MS-MDE2's own example leaves it.
type SearchCriteria struct {
	// Subject is the certificate subject to match, for example
	// "CN=Tester,O=Microsoft".
	Subject string
	// Stores are the stores to search; Windows accepts "My\User".
	Stores []string
}

// StoreMyUser is the only store value MS-MDE2 2.2.9.5 documents.
const StoreMyUser = `My\User`

// String encodes the criteria.
func (s SearchCriteria) String() string {
	var parts []string
	if s.Subject != "" {
		parts = append(parts, "Subject="+criteriaEscape(s.Subject))
	}
	if len(s.Stores) > 0 {
		vals := make([]string, len(s.Stores))
		for i, st := range s.Stores {
			vals[i] = criteriaEscape(st)
		}
		parts = append(parts, "Stores="+strings.Join(vals, "%EF%80%80"))
	}
	return strings.Join(parts, "&")
}

func criteriaEscape(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case 'a' <= c && c <= 'z', 'A' <= c && c <= 'Z', '0' <= c && c <= '9',
			c == '-', c == '_', c == '.', c == '!', c == '~', c == '*', c == '\'', c == '(', c == ')', c == ',':
			b.WriteByte(c)
		default:
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

// Poll is the DMClient Provider/<ID>/Poll schedule. Intervals are minutes.
type Poll struct {
	IntervalForFirstSetOfRetries         int
	NumberOfFirstRetries                 int
	IntervalForSecondSetOfRetries        int
	NumberOfSecondRetries                int
	IntervalForRemainingScheduledRetries int
	NumberOfRemainingScheduledRetries    int
	// PollOnLogin and AllUsersPollOnFirstLogin are written when set.
	PollOnLogin              bool
	AllUsersPollOnFirstLogin bool
}

// DefaultPoll is the schedule Microsoft documents for the DMClient CSP:
// five retries every 15 minutes, ten every hour, then daily for ever.
func DefaultPoll() Poll {
	return Poll{
		IntervalForFirstSetOfRetries: 15, NumberOfFirstRetries: 5,
		IntervalForSecondSetOfRetries: 60, NumberOfSecondRetries: 10,
		IntervalForRemainingScheduledRetries: 1440, NumberOfRemainingScheduledRetries: 0,
	}
}

// Validate applies the DMClient CSP rules: the server must not set
// NumberOfFirstRetries to 0, and the remaining-retries interval is at least
// 1440 minutes.
func (p Poll) Validate() error {
	if p.NumberOfFirstRetries == 0 {
		return fmt.Errorf("%w: NumberOfFirstRetries must not be 0", ErrInvalid)
	}
	if p.IntervalForFirstSetOfRetries <= 0 || p.IntervalForSecondSetOfRetries < 0 ||
		p.NumberOfFirstRetries < 0 || p.NumberOfSecondRetries < 0 || p.NumberOfRemainingScheduledRetries < 0 {
		return fmt.Errorf("%w: poll intervals and counts must not be negative", ErrInvalid)
	}
	if p.IntervalForRemainingScheduledRetries < 1440 {
		return fmt.Errorf("%w: IntervalForRemainingScheduledRetries must be at least 1440 minutes", ErrInvalid)
	}
	return nil
}

// DMClientConfig describes the DMClient characteristic.
type DMClientConfig struct {
	// ProviderID must equal the APPLICATION PROVIDER-ID.
	ProviderID string
	// UPN is the enrolling user's principal name; optional for device
	// enrollments.
	UPN string
	// EntDeviceName is the name the management console shows; optional.
	EntDeviceName string
	// EntDMID is the server's own identifier for the device; optional.
	EntDMID string
	// Poll is the schedule; zero means DefaultPoll.
	Poll *Poll
	// Extra are further parms under Provider/<ID>, for settings this
	// package does not model.
	Extra []Parm
}

// DMClient builds the DMClient characteristic of MS-MDE2 2.2.9.3.
func DMClient(cfg DMClientConfig) (Characteristic, error) {
	if cfg.ProviderID == "" {
		return Characteristic{}, fmt.Errorf("%w: DMClient ProviderID is required", ErrInvalid)
	}
	poll := DefaultPoll()
	if cfg.Poll != nil {
		poll = *cfg.Poll
	}
	if err := poll.Validate(); err != nil {
		return Characteristic{}, err
	}
	prov := Characteristic{Type: cfg.ProviderID}
	if cfg.UPN != "" {
		prov.Parms = append(prov.Parms, Parm{Name: "UPN", Value: cfg.UPN, DataType: TypeString})
	}
	if cfg.EntDeviceName != "" {
		prov.Parms = append(prov.Parms, Parm{Name: "EntDeviceName", Value: cfg.EntDeviceName, DataType: TypeString})
	}
	if cfg.EntDMID != "" {
		prov.Parms = append(prov.Parms, Parm{Name: "EntDMID", Value: cfg.EntDMID, DataType: TypeString})
	}
	for _, p := range cfg.Extra {
		if p.Name == "" {
			return Characteristic{}, fmt.Errorf("%w: DMClient extra parm has no name", ErrInvalid)
		}
		prov.Parms = append(prov.Parms, p)
	}
	pollCh := Characteristic{Type: "Poll", Parms: []Parm{
		{Name: "IntervalForFirstSetOfRetries", Value: strconv.Itoa(poll.IntervalForFirstSetOfRetries), DataType: TypeInteger},
		{Name: "NumberOfFirstRetries", Value: strconv.Itoa(poll.NumberOfFirstRetries), DataType: TypeInteger},
		{Name: "IntervalForSecondSetOfRetries", Value: strconv.Itoa(poll.IntervalForSecondSetOfRetries), DataType: TypeInteger},
		{Name: "NumberOfSecondRetries", Value: strconv.Itoa(poll.NumberOfSecondRetries), DataType: TypeInteger},
		{Name: "IntervalForRemainingScheduledRetries", Value: strconv.Itoa(poll.IntervalForRemainingScheduledRetries), DataType: TypeInteger},
		{Name: "NumberOfRemainingScheduledRetries", Value: strconv.Itoa(poll.NumberOfRemainingScheduledRetries), DataType: TypeInteger},
	}}
	if poll.PollOnLogin {
		pollCh.Parms = append(pollCh.Parms, Parm{Name: "PollOnLogin", Value: "true", DataType: TypeBoolean})
	}
	if poll.AllUsersPollOnFirstLogin {
		pollCh.Parms = append(pollCh.Parms, Parm{Name: "AllUsersPollOnFirstLogin", Value: "true", DataType: TypeBoolean})
	}
	prov.Children = []Characteristic{pollCh}
	return Characteristic{Type: TypeDMClient, Children: []Characteristic{
		{Type: "Provider", Children: []Characteristic{prov}},
	}}, nil
}
