package mdm_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/mdm"
	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/syncml"
)

func post(t *testing.T, h http.Handler, ct string, body []byte) *http.Response {
	t.Helper()
	r := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/svc?mode=Machine", bytes.NewReader(body))
	if ct != "" {
		r.Header.Set("Content-Type", ct)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec.Result()
}

func TestHTTPHandler(t *testing.T) {
	t.Parallel()
	h := mdm.NewHandler(newService(t))

	resp := post(t, h, syncml.ContentTypeXML, []byte(packageOne("1", "1")))
	if resp.StatusCode != http.StatusOK {
		t.Errorf("valid = %d", resp.StatusCode)
	}
	if cl := resp.Header.Get("Content-Length"); cl == "" {
		t.Error("no Content-Length")
	} else if n, _ := strconv.Atoi(cl); n == 0 {
		t.Error("zero Content-Length for a non-empty reply")
	}
	if len(resp.TransferEncoding) != 0 {
		t.Errorf("chunked: %v", resp.TransferEncoding)
	}
	if resp.Header.Get("Content-Type") != syncml.ContentTypeXML {
		t.Errorf("content type = %q", resp.Header.Get("Content-Type"))
	}

	// WBXML is 415.
	if resp := post(t, h, syncml.ContentTypeWBXML, []byte("data")); resp.StatusCode != http.StatusUnsupportedMediaType {
		t.Errorf("wbxml = %d", resp.StatusCode)
	}
	// Malformed body is 400.
	if resp := post(t, h, syncml.ContentTypeXML, []byte("<SyncML")); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("malformed = %d", resp.StatusCode)
	}
	// Unenrolled device is 403.
	other := bytes.ReplaceAll([]byte(packageOne("1", "1")), []byte(dev), []byte("NOPE"))
	if resp := post(t, h, syncml.ContentTypeXML, other); resp.StatusCode != http.StatusForbidden {
		t.Errorf("unenrolled = %d", resp.StatusCode)
	}
	// Oversize is 413.
	big := make([]byte, mdm.DefaultRequestBytesForTest()+1)
	if resp := post(t, h, syncml.ContentTypeXML, big); resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Errorf("oversize = %d", resp.StatusCode)
	}
	// GET is 405.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/svc", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET = %d", rec.Code)
	}
	// A bad mode is 400.
	rec = httptest.NewRecorder()
	r := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/svc?mode=Weird", bytes.NewReader([]byte(packageOne("1", "1"))))
	r.Header.Set("Content-Type", syncml.ContentTypeXML)
	h.ServeHTTP(rec, r)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("bad mode = %d", rec.Code)
	}
}
