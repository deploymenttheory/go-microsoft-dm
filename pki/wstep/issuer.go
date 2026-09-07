package wstep

import (
	"context"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"fmt"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/enroll"
	"github.com/deploymenttheory/go-microsoft-dm/pki/ca"
)

// ErrConfig reports an unusable Issuer.
var ErrConfig = errors.New("wstep: invalid configuration")

// SubjectFunc chooses the certificate subject for a request. It receives
// the enrollment request and the parsed CSR and returns the subject to
// certify.
type SubjectFunc func(req *enroll.Request, csr *x509.CertificateRequest) pkix.Name

// Issuer implements enroll.Issuer over a certificate authority.
type Issuer struct {
	ca      ca.Issuer
	policy  *ca.Policy
	subject SubjectFunc
}

// Option configures an Issuer.
type Option func(*Issuer)

// WithPolicy overrides the CA's default policy for enrollment certificates.
func WithPolicy(p ca.Policy) Option { return func(i *Issuer) { i.policy = &p } }

// WithSubject sets the subject hook. The default keeps the CSR subject.
func WithSubject(f SubjectFunc) Option { return func(i *Issuer) { i.subject = f } }

// NewIssuer wraps a CA.
func NewIssuer(authority ca.Issuer, opts ...Option) (*Issuer, error) {
	if authority == nil {
		return nil, fmt.Errorf("%w: certificate authority is required", ErrConfig)
	}
	i := &Issuer{ca: authority, subject: func(_ *enroll.Request, csr *x509.CertificateRequest) pkix.Name { return csr.Subject }}
	for _, o := range opts {
		o(i)
	}
	return i, nil
}

// Issue implements enroll.Issuer: parse and verify the CSR, choose the
// subject, sign, and return the certificate with the CA's chain.
func (i *Issuer) Issue(ctx context.Context, req *enroll.Request) (*enroll.Issued, error) {
	if req == nil {
		return nil, fmt.Errorf("%w: nil request", ErrCSR)
	}
	csr, err := ParseCSR(req.CSR)
	if err != nil {
		return nil, err
	}
	cert, err := i.ca.Issue(ctx, ca.Request{PublicKey: csr.PublicKey, Subject: i.subject(req, csr), Policy: i.policy})
	if err != nil {
		return nil, fmt.Errorf("wstep: issue: %w", err)
	}
	return &enroll.Issued{Certificate: cert, Chain: i.ca.Chain()}, nil
}
