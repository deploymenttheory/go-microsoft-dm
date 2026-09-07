package mdm

import (
	"context"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/syncml"
)

func TestParseTransport(t *testing.T) {
	t.Parallel()
	r := httptest.NewRequest(http.MethodPost, "/mdm?mode=Maintenance&Platform=WoA", strings.NewReader("<x/>"))
	r.Header.Set("Content-Type", syncml.ContentTypeXML)
	r.Header.Set("User-Agent", "MSFT OMA DM Client/1.2.0.1")
	r.Header.Set("Authorization", "Bearer abc.def")
	r.Header.Set(HeaderDeviceToken, "devtok")
	r.Header.Set(HeaderMSSignature, "sig==")
	r.Header.Set(HeaderGenericAlert, "<Type1><Type2>")
	r.Header.Set(HeaderClientRequestID, "entdmid")
	r.Header.Set(HeaderUserAgentOrigin, "1,2")
	tr, err := ParseTransport(r)
	if err != nil {
		t.Fatal(err)
	}
	if tr.Mode != ModeMaintenance || tr.Platform != "WoA" || tr.BearerToken != "abc.def" || tr.DeviceToken != "devtok" ||
		tr.MSSignature != "sig==" || tr.ClientRequestID != "entdmid" || tr.UserAgentOrigin != "1,2" {
		t.Errorf("transport = %+v", tr)
	}
	if len(tr.GenericAlerts) != 2 || tr.GenericAlerts[0] != "Type1" || tr.GenericAlerts[1] != "Type2" {
		t.Errorf("generic alerts = %v", tr.GenericAlerts)
	}
}

func TestParseTransportContentType(t *testing.T) {
	t.Parallel()
	// No content type is taken as XML.
	r := httptest.NewRequest(http.MethodPost, "/mdm", strings.NewReader("<x/>"))
	r.Header.Del("Content-Type")
	if tr, err := ParseTransport(r); err != nil || tr.ContentType != syncml.ContentTypeXML {
		t.Errorf("no content type: %+v, %v", tr, err)
	}
	// WBXML is refused until Phase 14.
	r = httptest.NewRequest(http.MethodPost, "/mdm", strings.NewReader(""))
	r.Header.Set("Content-Type", syncml.ContentTypeWBXML)
	if _, err := ParseTransport(r); !errors.Is(err, ErrContentType) {
		t.Errorf("wbxml: %v", err)
	}
	// An unknown mode is refused.
	r = httptest.NewRequest(http.MethodPost, "/mdm?mode=Weird", strings.NewReader(""))
	r.Header.Set("Content-Type", syncml.ContentTypeXML)
	if _, err := ParseTransport(r); !errors.Is(err, ErrMode) {
		t.Errorf("bad mode: %v", err)
	}
	// A garbage media type is refused.
	r = httptest.NewRequest(http.MethodPost, "/mdm", strings.NewReader(""))
	r.Header.Set("Content-Type", "not/a/type/at/all;;")
	if _, err := ParseTransport(r); !errors.Is(err, ErrContentType) {
		t.Errorf("garbage media type: %v", err)
	}
}

func TestParseGenericAlerts(t *testing.T) {
	t.Parallel()
	if got := ParseGenericAlerts(""); got != nil {
		t.Errorf("empty = %v", got)
	}
	got := ParseGenericAlerts("<A><B>")
	if len(got) != 2 || got[0] != "A" || got[1] != "B" {
		t.Errorf("got %v", got)
	}
}

func TestMemorySessions(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	m := NewMemorySessions()
	now := t0
	m.Now = func() time.Time { return now }
	if _, err := m.GetSession(ctx, "k"); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing: %v", err)
	}
	s := &Session{Key: "k", DeviceID: "d", SessionID: "1", LastSeen: now}
	if err := m.PutSession(ctx, s); err != nil {
		t.Fatal(err)
	}
	if m.Len() != 1 {
		t.Errorf("len = %d", m.Len())
	}
	got, err := m.GetSession(ctx, "k")
	if err != nil || got.DeviceID != "d" {
		t.Fatalf("get = %+v, %v", got, err)
	}
	// Returned sessions are copies.
	got.DeviceID = "changed"
	again, _ := m.GetSession(ctx, "k")
	if again.DeviceID != "d" {
		t.Error("store shares memory")
	}
	// Expiry.
	now = now.Add(2 * time.Hour)
	if _, err := m.GetSession(ctx, "k"); !errors.Is(err, ErrNotFound) {
		t.Errorf("expired: %v", err)
	}
	if err := m.DeleteSession(ctx, "gone"); err != nil {
		t.Errorf("delete unknown: %v", err)
	}
}

func TestCertTrusts(t *testing.T) {
	t.Parallel()
	id := &Identity{DeviceID: "DEVICE-01"}
	match := &x509.Certificate{Subject: pkix.Name{CommonName: "DEVICE-01"}}
	other := &x509.Certificate{Subject: pkix.Name{CommonName: "OTHER"}}
	if !certTrusts(id, []*x509.Certificate{other, match}) {
		t.Error("matching cert not trusted")
	}
	if certTrusts(id, []*x509.Certificate{other}) {
		t.Error("non-matching cert trusted")
	}
	if certTrusts(&Identity{}, []*x509.Certificate{match}) {
		t.Error("empty device id trusted")
	}
}
