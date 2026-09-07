package wstep

import (
	"bytes"
	"crypto/x509"
	"encoding/asn1"
	"errors"
	"fmt"
	"strings"
)

// Errors returned by ParseCSR.
var (
	// ErrCSR reports bytes that are not a PKCS#10 request.
	ErrCSR = errors.New("wstep: invalid certificate request")
	// ErrPKCS7 reports a PKCS#7 (CMS) body where a PKCS#10 was expected.
	ErrPKCS7 = errors.New("wstep: PKCS#7 token is not a certificate request")
	// ErrSubject reports a subject string the relaxation does not cover.
	ErrSubject = errors.New("wstep: subject has characters outside PrintableString and the Windows relaxation")
	// ErrSignature reports a request whose signature does not verify.
	ErrSignature = errors.New("wstep: certificate request signature does not verify")
)

// RelaxedCharacters are the bytes a Windows client has been observed to put
// in a PrintableString common name: the exclamation mark that separates the
// two halves of an enrollment identifier, and a NUL.
const RelaxedCharacters = "!\x00"

var oidPKCS7 = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 7}

// IsPKCS7 reports whether der is a PKCS#7 ContentInfo: a SEQUENCE whose
// first element is a content type OID under 1.2.840.113549.1.7.
func IsPKCS7(der []byte) bool {
	var outer asn1.RawValue
	if rest, err := asn1.Unmarshal(der, &outer); err != nil || len(rest) != 0 || !outer.IsCompound || outer.Tag != asn1.TagSequence {
		return false
	}
	var oid asn1.ObjectIdentifier
	if _, err := asn1.Unmarshal(outer.Bytes, &oid); err != nil {
		return false
	}
	return len(oid) == len(oidPKCS7)+1 && oid[:len(oidPKCS7)].Equal(oidPKCS7)
}

// ParseCSR parses a DER PKCS#10 request, tolerating the subject strings
// Windows sends, and verifies its signature.
func ParseCSR(der []byte) (*x509.CertificateRequest, error) {
	if len(der) == 0 {
		return nil, fmt.Errorf("%w: empty", ErrCSR)
	}
	if IsPKCS7(der) {
		return nil, ErrPKCS7
	}
	csr, err := x509.ParseCertificateRequest(der)
	if err != nil {
		// crypto/x509 reports the subject problem as a plain error whose
		// text names the string type; nothing else about the request is
		// retried.
		if !strings.Contains(err.Error(), "PrintableString") {
			return nil, fmt.Errorf("%w: %w", ErrCSR, err)
		}
		csr, err = parseRelaxed(der)
		if err != nil {
			return nil, err
		}
	}
	if err := csr.CheckSignature(); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrSignature, err)
	}
	return csr, nil
}

type certificationRequest struct {
	TBS       asn1.RawValue
	SigAlg    asn1.RawValue
	Signature asn1.RawValue
}

type tbsRequest struct {
	Version    asn1.RawValue
	Subject    asn1.RawValue
	SPKI       asn1.RawValue
	Attributes asn1.RawValue `asn1:"optional,tag:0"`
}

// parseRelaxed re-tags the offending PrintableStrings as UTF8Strings in a
// copy, parses the copy, and restores the original raw bytes.
func parseRelaxed(der []byte) (*x509.CertificateRequest, error) {
	var req certificationRequest
	if rest, err := asn1.Unmarshal(der, &req); err != nil || len(rest) != 0 {
		return nil, fmt.Errorf("%w: %v", ErrCSR, err)
	}
	var tbs tbsRequest
	if rest, err := asn1.Unmarshal(req.TBS.FullBytes, &tbs); err != nil || len(rest) != 0 {
		return nil, fmt.Errorf("%w: %v", ErrCSR, err)
	}
	subject, err := relaxSubject(tbs.Subject)
	if err != nil {
		return nil, err
	}
	tbsRaw := tlv(constructed|asn1.TagSequence, concat(tbs.Version.FullBytes, subject, tbs.SPKI.FullBytes, tbs.Attributes.FullBytes))
	fixed := tlv(constructed|asn1.TagSequence, concat(tbsRaw, req.SigAlg.FullBytes, req.Signature.FullBytes))
	csr, err := x509.ParseCertificateRequest(fixed)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCSR, err)
	}
	// The client signed the original bytes; the parsed structure keeps them
	// so CheckSignature and any later hashing see what was sent.
	csr.Raw = der
	csr.RawTBSCertificateRequest = req.TBS.FullBytes
	csr.RawSubject = tbs.Subject.FullBytes
	return csr, nil
}

// children splits a constructed value into its immediate TLV children.
func children(v asn1.RawValue, tag int) ([]asn1.RawValue, error) {
	if !v.IsCompound || v.Class != asn1.ClassUniversal || v.Tag != tag {
		return nil, fmt.Errorf("%w: subject: expected constructed tag %d", ErrCSR, tag)
	}
	var out []asn1.RawValue
	for rest := v.Bytes; len(rest) > 0; {
		var child asn1.RawValue
		var err error
		rest, err = asn1.Unmarshal(rest, &child)
		if err != nil {
			return nil, fmt.Errorf("%w: subject: %w", ErrCSR, err)
		}
		out = append(out, child)
	}
	return out, nil
}

// relaxSubject returns the RDNSequence with offending PrintableStrings
// re-tagged, or ErrSubject. Only tag bytes change, so every length in the
// re-encoded structure equals the original's.
func relaxSubject(subject asn1.RawValue) ([]byte, error) {
	rdns, err := children(subject, asn1.TagSequence)
	if err != nil {
		return nil, err
	}
	var out []byte
	for _, set := range rdns {
		atvs, err := children(set, asn1.TagSet)
		if err != nil {
			return nil, err
		}
		var setBytes []byte
		for _, raw := range atvs {
			parts, err := children(raw, asn1.TagSequence)
			if err != nil {
				return nil, err
			}
			if len(parts) != 2 {
				return nil, fmt.Errorf("%w: subject: AttributeTypeAndValue with %d parts", ErrCSR, len(parts))
			}
			val := parts[1]
			if val.Class == asn1.ClassUniversal && val.Tag == asn1.TagPrintableString && !printable(val.Bytes) {
				if !relaxedPrintable(val.Bytes) {
					return nil, fmt.Errorf("%w: %q", ErrSubject, val.Bytes)
				}
				setBytes = append(setBytes, tlv(constructed|asn1.TagSequence, concat(parts[0].FullBytes, tlv(asn1.TagUTF8String, val.Bytes)))...)
				continue
			}
			setBytes = append(setBytes, raw.FullBytes...)
		}
		out = append(out, tlv(constructed|asn1.TagSet, setBytes)...)
	}
	return tlv(constructed|asn1.TagSequence, out), nil
}

// constructed is the ASN.1 constructed bit of an identifier octet.
const constructed = 0x20

// tlv encodes a universal-class value with a DER length.
func tlv(tag byte, content []byte) []byte {
	n := len(content)
	out := []byte{tag}
	switch {
	case n < 0x80:
		out = append(out, byte(n))
	default:
		var lenBytes []byte
		for v := n; v > 0; v >>= 8 {
			lenBytes = append([]byte{byte(v)}, lenBytes...)
		}
		out = append(out, 0x80|byte(len(lenBytes)))
		out = append(out, lenBytes...)
	}
	return append(out, content...)
}

// printable reports whether b is a PrintableString per X.680 41.4, with the
// '*' and '&' allowances crypto/x509 itself makes.
func printable(b []byte) bool {
	for _, c := range b {
		if !isPrintableByte(c) {
			return false
		}
	}
	return true
}

func isPrintableByte(c byte) bool {
	return 'a' <= c && c <= 'z' ||
		'A' <= c && c <= 'Z' ||
		'0' <= c && c <= '9' ||
		'\'' <= c && c <= ')' ||
		'+' <= c && c <= '/' ||
		c == ' ' || c == ':' || c == '=' || c == '?' || c == '*' || c == '&'
}

// relaxedPrintable reports whether every byte is printable or in
// RelaxedCharacters.
func relaxedPrintable(b []byte) bool {
	for _, c := range b {
		if !isPrintableByte(c) && !bytes.ContainsRune([]byte(RelaxedCharacters), rune(c)) {
			return false
		}
	}
	return true
}

func concat(parts ...[]byte) []byte {
	var n int
	for _, p := range parts {
		n += len(p)
	}
	out := make([]byte, 0, n)
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}
