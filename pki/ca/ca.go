package ca

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1" //nolint:gosec // Windows names certificate store entries by SHA-1 thumbprint
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/deploymenttheory/go-microsoft-dm/clock"
)

// Errors returned by this package.
var (
	// ErrConfig reports an unusable issuer configuration.
	ErrConfig = errors.New("ca: invalid configuration")
	// ErrRequest reports a request that cannot be signed at all.
	ErrRequest = errors.New("ca: invalid request")
	// ErrPolicy reports a request the policy refuses.
	ErrPolicy = errors.New("ca: request violates policy")
	// ErrPEM reports PEM that does not hold what was asked for.
	ErrPEM = errors.New("ca: invalid PEM")
)

// KeyKind names a public key type and size a policy can allow.
type KeyKind string

// The key kinds a policy can name.
const (
	KeyRSA2048 KeyKind = "rsa-2048"
	KeyRSA3072 KeyKind = "rsa-3072"
	KeyRSA4096 KeyKind = "rsa-4096"
	KeyECP256  KeyKind = "ec-p256"
	KeyECP384  KeyKind = "ec-p384"
	KeyECP521  KeyKind = "ec-p521"
)

// KindOf reports the KeyKind of a public key, and false for a key this
// package cannot describe.
func KindOf(pub crypto.PublicKey) (KeyKind, bool) {
	switch key := pub.(type) {
	case *rsa.PublicKey:
		switch key.N.BitLen() {
		case 2048:
			return KeyRSA2048, true
		case 3072:
			return KeyRSA3072, true
		case 4096:
			return KeyRSA4096, true
		}
	case *ecdsa.PublicKey:
		switch key.Curve {
		case elliptic.P256():
			return KeyECP256, true
		case elliptic.P384():
			return KeyECP384, true
		case elliptic.P521():
			return KeyECP521, true
		}
	}
	return "", false
}

// Policy constrains what Issue produces. Zero values take the defaults.
type Policy struct {
	// Validity of issued certificates; default one year.
	Validity time.Duration
	// NotAfter caps expiry absolutely; zero means Validity alone decides.
	NotAfter time.Time
	// KeyUsage default: digital signature, plus key encipherment for RSA.
	KeyUsage x509.KeyUsage
	// ExtKeyUsage default: client authentication.
	ExtKeyUsage []x509.ExtKeyUsage
	// MinRSABits refuses smaller RSA keys; default 2048.
	MinRSABits int
	// AllowedKeys restricts key kinds; empty allows any RSA key of at least
	// MinRSABits and any ECDSA key on a named curve.
	AllowedKeys []KeyKind
	// Subject replaces the requested subject when set.
	Subject *pkix.Name
	// ExtraExtensions are added to every certificate.
	ExtraExtensions []pkix.Extension
	// CRLDistributionPoints and OCSPServer advertise status URLs.
	CRLDistributionPoints []string
	OCSPServer            []string
}

func (p Policy) withDefaults() Policy {
	if p.Validity == 0 {
		p.Validity = 365 * 24 * time.Hour
	}
	if len(p.ExtKeyUsage) == 0 {
		p.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	}
	if p.MinRSABits == 0 {
		p.MinRSABits = 2048
	}
	return p
}

// Request is what an Issuer signs.
type Request struct {
	// PublicKey is the key to certify. Required.
	PublicKey crypto.PublicKey
	// Subject is the requested subject; the policy may override it.
	Subject pkix.Name
	// Policy, when set, replaces the issuer's policy for this request.
	Policy *Policy
}

// Issuer signs certificates.
type Issuer interface {
	// Issue signs the request under the policy.
	Issue(ctx context.Context, req Request) (*x509.Certificate, error)
	// Certificate is the signing certificate.
	Certificate() *x509.Certificate
	// Chain is the signing certificate followed by its issuers up to and
	// including the root. A self-signed issuer returns itself alone.
	Chain() []*x509.Certificate
}

// Local is an Issuer holding its key in memory.
type Local struct {
	cert   *x509.Certificate
	chain  []*x509.Certificate
	key    crypto.Signer
	policy Policy
	clock  clock.Clock
	random io.Reader
}

// Option configures Local.
type Option func(*Local)

// WithPolicy sets the default policy.
func WithPolicy(p Policy) Option { return func(l *Local) { l.policy = p } }

// WithClock sets the clock that stamps NotBefore.
func WithClock(c clock.Clock) Option { return func(l *Local) { l.clock = c } }

// WithChain sets the issuers above the signing certificate, nearest first,
// ending at the root.
func WithChain(chain ...*x509.Certificate) Option { return func(l *Local) { l.chain = chain } }

// WithRandom sets the entropy source for serials.
func WithRandom(r io.Reader) Option { return func(l *Local) { l.random = r } }

// NewLocal creates an issuer from a CA certificate and its key.
func NewLocal(cert *x509.Certificate, key crypto.Signer, opts ...Option) (*Local, error) {
	if cert == nil || key == nil {
		return nil, fmt.Errorf("%w: certificate and key are required", ErrConfig)
	}
	if !cert.IsCA {
		return nil, fmt.Errorf("%w: certificate is not a CA", ErrConfig)
	}
	l := &Local{cert: cert, key: key, clock: clock.Real{}, random: rand.Reader}
	for _, o := range opts {
		o(l)
	}
	for _, c := range l.chain {
		if c == nil {
			return nil, fmt.Errorf("%w: nil certificate in chain", ErrConfig)
		}
	}
	return l, nil
}

// Certificate implements Issuer.
func (l *Local) Certificate() *x509.Certificate { return l.cert }

// Chain implements Issuer.
func (l *Local) Chain() []*x509.Certificate {
	return append([]*x509.Certificate{l.cert}, l.chain...)
}

// Policy returns the default policy.
func (l *Local) Policy() Policy { return l.policy }

// Issue implements Issuer.
func (l *Local) Issue(_ context.Context, req Request) (*x509.Certificate, error) {
	if req.PublicKey == nil {
		return nil, fmt.Errorf("%w: no public key", ErrRequest)
	}
	p := l.policy
	if req.Policy != nil {
		p = *req.Policy
	}
	p = p.withDefaults()
	keyUsage := p.KeyUsage
	switch pub := req.PublicKey.(type) {
	case *rsa.PublicKey:
		if pub.N.BitLen() < p.MinRSABits {
			return nil, fmt.Errorf("%w: RSA key is %d bits, minimum %d", ErrPolicy, pub.N.BitLen(), p.MinRSABits)
		}
		if keyUsage == 0 {
			keyUsage = x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment
		}
	case *ecdsa.PublicKey:
		if keyUsage == 0 {
			keyUsage = x509.KeyUsageDigitalSignature
		}
	default:
		return nil, fmt.Errorf("%w: unsupported key type %T", ErrPolicy, req.PublicKey)
	}
	if len(p.AllowedKeys) > 0 {
		kind, known := KindOf(req.PublicKey)
		if !known || !slices.Contains(p.AllowedKeys, kind) {
			return nil, fmt.Errorf("%w: key %q is not among the allowed kinds %v", ErrPolicy, kind, p.AllowedKeys)
		}
	}
	serial, err := SerialFrom(l.random)
	if err != nil {
		return nil, err
	}
	now := l.clock.Now()
	notAfter := now.Add(p.Validity)
	if !p.NotAfter.IsZero() && p.NotAfter.Before(notAfter) {
		notAfter = p.NotAfter
	}
	if !notAfter.After(now) {
		return nil, fmt.Errorf("%w: NotAfter %s has already passed", ErrPolicy, notAfter.UTC())
	}
	subject := req.Subject
	if p.Subject != nil {
		subject = *p.Subject
	}
	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               subject,
		NotBefore:             now,
		NotAfter:              notAfter,
		KeyUsage:              keyUsage,
		ExtKeyUsage:           p.ExtKeyUsage,
		BasicConstraintsValid: true,
		ExtraExtensions:       slices.Clone(p.ExtraExtensions),
		CRLDistributionPoints: slices.Clone(p.CRLDistributionPoints),
		OCSPServer:            slices.Clone(p.OCSPServer),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, l.cert, req.PublicKey, l.key)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrRequest, err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, fmt.Errorf("ca: parse issued certificate: %w", err)
	}
	return cert, nil
}

// Serial returns a random positive 127-bit serial number.
func Serial() (*big.Int, error) { return SerialFrom(rand.Reader) }

// SerialFrom draws a serial from r.
func SerialFrom(r io.Reader) (*big.Int, error) {
	limit := new(big.Int).Lsh(big.NewInt(1), 127)
	n, err := rand.Int(r, limit)
	if err != nil {
		return nil, fmt.Errorf("ca: serial: %w", err)
	}
	return n.Add(n, big.NewInt(1)), nil
}

// Thumbprint returns the SHA-1 hash of the certificate as uppercase hex,
// the name Windows gives it under CertificateStore.
func Thumbprint(cert *x509.Certificate) string {
	sum := sha1.Sum(cert.Raw) //nolint:gosec // store key, not a security control
	return strings.ToUpper(hex.EncodeToString(sum[:]))
}

// SelfSignedOptions configure NewSelfSigned.
type SelfSignedOptions struct {
	// Subject; default CN "go-microsoft-dm CA".
	Subject pkix.Name
	// Validity; default ten years.
	Validity time.Duration
	// RSABits; default 2048. Ignored when Key is set.
	RSABits int
	// Key, when set, is certified instead of generating an RSA key.
	Key crypto.Signer
	// Random; default crypto/rand.
	Random io.Reader
	// Clock stamps NotBefore; default real time.
	Clock clock.Clock
}

// NewSelfSigned generates a root CA certificate and, unless a key was
// given, an RSA key, and returns them as a Local issuer.
func NewSelfSigned(o SelfSignedOptions, opts ...Option) (*Local, error) {
	if o.Validity == 0 {
		o.Validity = 10 * 365 * 24 * time.Hour
	}
	if o.RSABits == 0 {
		o.RSABits = 2048
	}
	if o.Subject.CommonName == "" {
		o.Subject.CommonName = "go-microsoft-dm CA"
	}
	if o.Random == nil {
		o.Random = rand.Reader
	}
	if o.Clock == nil {
		o.Clock = clock.Real{}
	}
	key := o.Key
	if key == nil {
		k, err := rsa.GenerateKey(o.Random, o.RSABits)
		if err != nil {
			return nil, fmt.Errorf("ca: generate key: %w", err)
		}
		key = k
	}
	serial, err := SerialFrom(o.Random)
	if err != nil {
		return nil, err
	}
	now := o.Clock.Now()
	tmpl := &x509.Certificate{
		SerialNumber: serial, Subject: o.Subject,
		NotBefore: now, NotAfter: now.Add(o.Validity),
		IsCA: true, BasicConstraintsValid: true, MaxPathLenZero: true,
		KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, key.Public(), key)
	if err != nil {
		return nil, fmt.Errorf("ca: create certificate: %w", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, fmt.Errorf("ca: parse certificate: %w", err)
	}
	return NewLocal(cert, key, append([]Option{WithClock(o.Clock), WithRandom(o.Random)}, opts...)...)
}

// LoadPEM builds an issuer from a PEM certificate, a PEM PKCS#8, PKCS#1 or
// EC private key, and optional PEM issuer certificates nearest first.
func LoadPEM(certPEM, keyPEM []byte, chainPEM ...[]byte) (*Local, error) {
	cert, err := parseCertPEM(certPEM)
	if err != nil {
		return nil, err
	}
	key, err := parseKeyPEM(keyPEM)
	if err != nil {
		return nil, err
	}
	chain := make([]*x509.Certificate, 0, len(chainPEM))
	for _, c := range chainPEM {
		parsed, err := parseCertPEM(c)
		if err != nil {
			return nil, err
		}
		chain = append(chain, parsed)
	}
	return NewLocal(cert, key, WithChain(chain...))
}

// LoadFiles is LoadPEM over files.
func LoadFiles(certPath, keyPath string, chainPaths ...string) (*Local, error) {
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return nil, fmt.Errorf("ca: %w", err)
	}
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("ca: %w", err)
	}
	chain := make([][]byte, 0, len(chainPaths))
	for _, p := range chainPaths {
		c, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("ca: %w", err)
		}
		chain = append(chain, c)
	}
	return LoadPEM(certPEM, keyPEM, chain...)
}

func parseCertPEM(data []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(data)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, fmt.Errorf("%w: expected a CERTIFICATE block", ErrPEM)
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrPEM, err)
	}
	return cert, nil
}

func parseKeyPEM(data []byte) (crypto.Signer, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("%w: expected a private key block", ErrPEM)
	}
	var (
		key any
		err error
	)
	switch block.Type {
	case "PRIVATE KEY":
		key, err = x509.ParsePKCS8PrivateKey(block.Bytes)
	case "RSA PRIVATE KEY":
		key, err = x509.ParsePKCS1PrivateKey(block.Bytes)
	case "EC PRIVATE KEY":
		key, err = x509.ParseECPrivateKey(block.Bytes)
	default:
		return nil, fmt.Errorf("%w: unsupported key block %q", ErrPEM, block.Type)
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrPEM, err)
	}
	signer, ok := key.(crypto.Signer)
	if !ok {
		return nil, fmt.Errorf("%w: key type %T cannot sign", ErrPEM, key)
	}
	return signer, nil
}
