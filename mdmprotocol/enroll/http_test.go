package enroll

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/soap"
)

func post(t *testing.T, h http.Handler, path string, body []byte) *http.Response {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", ContentType)
	h.ServeHTTP(rec, req)
	return rec.Result()
}

func checkFraming(t *testing.T, resp *http.Response) []byte {
	t.Helper()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Header.Get("Content-Type") != ContentType {
		t.Errorf("Content-Type = %q", resp.Header.Get("Content-Type"))
	}
	if cl := resp.Header.Get("Content-Length"); cl != strconv.Itoa(len(body)) {
		t.Errorf("Content-Length = %q, body %d bytes", cl, len(body))
	}
	if len(resp.TransferEncoding) != 0 {
		t.Errorf("Transfer-Encoding = %v", resp.TransferEncoding)
	}
	return body
}

func TestHandlerRoutes(t *testing.T) {
	t.Parallel()
	e := newEnv(t, nil)
	var logged []string
	h := NewHandler(e.svc)
	h.Log = func(op string, err error) { logged = append(logged, op+": "+err.Error()) }

	// GET on the discovery path is the reachability probe.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, DiscoveryPath, nil))
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Length") != "0" || rec.Body.Len() != 0 {
		t.Errorf("GET = %d %q %q", rec.Code, rec.Header().Get("Content-Length"), rec.Body.String())
	}

	resp := post(t, h, DiscoveryPath, fixture(t, "discover-onprem-request.xml"))
	body := checkFraming(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("discover status = %d: %s", resp.StatusCode, body)
	}
	if _, _, err := DecodeDiscoverResponse(body); err != nil {
		t.Error(err)
	}

	// Policy and enrollment share one path in this configuration; the
	// action decides.
	svcPath := "/ENROLLMENTSERVER/DEVICEENROLLMENTWEBSERVICE.SVC"
	resp = post(t, h, svcPath, fixture(t, "getpolicies-onprem-request.xml"))
	body = checkFraming(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("policy status = %d: %s", resp.StatusCode, body)
	}
	if _, _, err := DecodePolicyResponse(body); err != nil {
		t.Error(err)
	}
	rst, _ := rstFixture(t)
	resp = post(t, h, svcPath, rst)
	body = checkFraming(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("enroll status = %d: %s", resp.StatusCode, body)
	}
	if _, _, err := DecodeTokenResponse(body); err != nil {
		t.Error(err)
	}

	// A fault is a 500 with the fault envelope, still framed.
	resp = post(t, h, DiscoveryPath, []byte("<s:Envelope"))
	body = checkFraming(t, resp)
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("fault status = %d", resp.StatusCode)
	}
	if f, err := soap.DecodeFault(body); err != nil || f == nil || f.Subcode != string(soap.SubcodeMessageFormat) {
		t.Errorf("fault = %+v, %v", f, err)
	}
	if len(logged) != 1 || !strings.HasPrefix(logged[0], "Discover: ") {
		t.Errorf("logged = %v", logged)
	}
	// Garbage on the shared path is treated as an enrollment.
	resp = post(t, h, svcPath, []byte("<s:Envelope"))
	if resp.StatusCode != http.StatusInternalServerError || !strings.HasPrefix(logged[1], "RequestSecurityToken: ") {
		t.Errorf("garbage = %d, %v", resp.StatusCode, logged)
	}

	for _, path := range []string{DiscoveryPath, svcPath} {
		rec = httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodPut, path, nil))
		if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") == "" {
			t.Errorf("PUT %s = %d", path, rec.Code)
		}
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, svcPath, nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET service = %d", rec.Code)
	}
	resp = post(t, h, svcPath, make([]byte, soap.DefaultMaxSize+1))
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Errorf("too large = %d", resp.StatusCode)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/nowhere", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("unknown path = %d", rec.Code)
	}
}

func TestHandlerSeparatePaths(t *testing.T) {
	t.Parallel()
	e := newEnv(t, func(c *Config) {
		c.EnrollmentPolicyServiceURL = "https://enrolltest.contoso.com/EnrollmentServer/Policy.svc"
		c.EnrollmentServiceURL = "https://enrolltest.contoso.com/EnrollmentServer/Enrollment.svc"
	})
	h := NewHandler(e.svc)
	resp := post(t, h, "/EnrollmentServer/Policy.svc", fixture(t, "getpolicies-onprem-request.xml"))
	if body := checkFraming(t, resp); resp.StatusCode != http.StatusOK {
		t.Errorf("policy = %d: %s", resp.StatusCode, body)
	}
	rst, _ := rstFixture(t)
	resp = post(t, h, "/EnrollmentServer/Enrollment.svc", rst)
	if body := checkFraming(t, resp); resp.StatusCode != http.StatusOK {
		t.Errorf("enroll = %d: %s", resp.StatusCode, body)
	}
	// The policy handler on its own path does not dispatch on action.
	resp = post(t, h, "/EnrollmentServer/Policy.svc", rst)
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("rst on policy path = %d", resp.StatusCode)
	}
	for _, path := range []string{"/EnrollmentServer/Policy.svc", "/EnrollmentServer/Enrollment.svc"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, path, nil))
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("GET %s = %d", path, rec.Code)
		}
		resp = post(t, h, path, make([]byte, soap.DefaultMaxSize+1))
		if resp.StatusCode != http.StatusRequestEntityTooLarge {
			t.Errorf("too large %s = %d", path, resp.StatusCode)
		}
	}
}

func TestHandlerNoPolicyService(t *testing.T) {
	t.Parallel()
	e := newEnv(t, func(c *Config) { c.EnrollmentPolicyServiceURL = "" })
	h := NewHandler(e.svc)
	resp := post(t, h, "/ENROLLMENTSERVER/DEVICEENROLLMENTWEBSERVICE.SVC", fixture(t, "getpolicies-onprem-request.xml"))
	body := checkFraming(t, resp)
	f, err := soap.DecodeFault(body)
	if resp.StatusCode != http.StatusInternalServerError || err != nil || f == nil {
		t.Errorf("GetPolicies without policy service = %d %+v %v", resp.StatusCode, f, err)
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

func TestHandlerUnreadableBody(t *testing.T) {
	t.Parallel()
	e := newEnv(t, nil)
	h := NewHandler(e.svc)
	for _, path := range []string{DiscoveryPath, "/ENROLLMENTSERVER/DEVICEENROLLMENTWEBSERVICE.SVC"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodPost, path, failingReader{}))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s = %d", path, rec.Code)
		}
	}
	if pathOf("://bad") != "/" || pathOf("https://h") != "/" || pathOf("https://h/a/b") != "/a/b" {
		t.Error("pathOf")
	}
}
