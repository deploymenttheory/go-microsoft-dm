package enroll

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/soap"
)

// ContentType is the media type of every SOAP 1.2 response.
const ContentType = "application/soap+xml; charset=utf-8"

// Handler serves the three MS-MDE2 endpoints over HTTP. Responses always
// carry Content-Length, because the Windows enrollment client rejects
// chunked transfer encoding, and a fault is sent with status 500 as SOAP
// 1.2 prescribes for a Receiver fault.
type Handler struct {
	svc *Service
	mux *http.ServeMux
	// Log receives every fault with the operation name. Optional.
	Log func(op string, err error)
}

// NewHandler routes DiscoveryPath and the paths of the configured policy
// and enrollment URLs to the service.
func NewHandler(s *Service) *Handler {
	h := &Handler{svc: s, mux: http.NewServeMux()}
	h.mux.HandleFunc(DiscoveryPath, h.Discovery)
	enrollPath := pathOf(s.cfg.EnrollmentServiceURL)
	policyPath := ""
	if s.cfg.EnrollmentPolicyServiceURL != "" {
		policyPath = pathOf(s.cfg.EnrollmentPolicyServiceURL)
	}
	switch policyPath {
	case "":
		h.mux.HandleFunc(enrollPath, h.Enrollment)
	case enrollPath:
		// One endpoint for both operations, as Microsoft's examples show:
		// the action decides.
		h.mux.HandleFunc(enrollPath, h.policyOrEnrollment)
	default:
		h.mux.HandleFunc(enrollPath, h.Enrollment)
		h.mux.HandleFunc(policyPath, h.Policy)
	}
	return h
}

func pathOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Path == "" {
		return "/"
	}
	return u.Path
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) { h.mux.ServeHTTP(w, r) }

// Discovery handles GET (the client's reachability probe, answered with an
// empty 200) and POST (the Discover operation).
func (h *Handler) Discovery(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Length", "0")
		w.WriteHeader(http.StatusOK)
	case http.MethodPost:
		h.serve(w, r, "Discover", h.svc.Discover)
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// Policy handles the GetPolicies operation.
func (h *Handler) Policy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	h.serve(w, r, "GetPolicies", h.svc.GetPolicies)
}

// Enrollment handles the RequestSecurityToken operation.
func (h *Handler) Enrollment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	h.serve(w, r, "RequestSecurityToken", h.svc.Enroll)
}

func (h *Handler) policyOrEnrollment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body, ok := h.read(w, r)
	if !ok {
		return
	}
	op, fn := "RequestSecurityToken", h.svc.Enroll
	if env, err := soap.Decode[struct{}](body, h.svc.cfg.MaxRequestSize); err == nil && env.Header.Action == ActionGetPolicies {
		op, fn = "GetPolicies", h.svc.GetPolicies
	}
	h.respond(w, r.Context(), op, body, fn)
}

func (h *Handler) read(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	body, err := io.ReadAll(io.LimitReader(r.Body, int64(h.svc.cfg.MaxRequestSize)+1))
	if err != nil {
		http.Error(w, "cannot read request", http.StatusBadRequest)
		return nil, false
	}
	if len(body) > h.svc.cfg.MaxRequestSize {
		http.Error(w, "request too large", http.StatusRequestEntityTooLarge)
		return nil, false
	}
	return body, true
}

func (h *Handler) serve(w http.ResponseWriter, r *http.Request, op string, fn func(context.Context, []byte) ([]byte, error)) {
	body, ok := h.read(w, r)
	if !ok {
		return
	}
	h.respond(w, r.Context(), op, body, fn)
}

func (h *Handler) respond(w http.ResponseWriter, ctx context.Context, op string, body []byte, fn func(context.Context, []byte) ([]byte, error)) {
	out, err := fn(ctx, body)
	status := http.StatusOK
	if err != nil {
		status = http.StatusInternalServerError
		if h.Log != nil {
			h.Log(op, err)
		}
	}
	w.Header().Set("Content-Type", ContentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(out)))
	w.WriteHeader(status)
	_, _ = w.Write(out)
}

// IsFault reports whether an error from the Service is a fault and returns
// it. Every non-nil error the Service returns is one.
func IsFault(err error) (*soap.Fault, bool) {
	var f *soap.Fault
	if errors.As(err, &f) {
		return f, true
	}
	return nil, false
}
