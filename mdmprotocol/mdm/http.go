package mdm

import (
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/syncml"
)

// Handler serves the OMA DM management endpoint. Responses carry
// Content-Length and are never chunked, because the Windows client rejects
// chunked transfer encoding (research pitfall "Transport"). It performs no
// checks on the request beyond the content type: no User-Agent, no fixed
// URIs, as the Learn OMA DM page advises.
type Handler struct {
	svc *Service
	// Log receives handler-level errors with the device id when known.
	Log func(deviceID string, err error)
}

// NewHandler wraps a Service.
func NewHandler(s *Service) *Handler { return &Handler{svc: s} }

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	t, err := ParseTransport(r)
	if err != nil {
		if errors.Is(err, ErrContentType) {
			http.Error(w, err.Error(), http.StatusUnsupportedMediaType)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, int64(h.svc.cfg.MaxRequestSize)+1))
	if err != nil {
		http.Error(w, "cannot read request", http.StatusBadRequest)
		return
	}
	if len(body) > h.svc.cfg.MaxRequestSize {
		http.Error(w, "request too large", http.StatusRequestEntityTooLarge)
		return
	}
	out, err := h.svc.Handle(r.Context(), t, body)
	if err != nil {
		if h.Log != nil {
			h.Log("", err)
		}
		http.Error(w, "cannot process request", statusFor(err))
		return
	}
	w.Header().Set("Content-Type", syncml.ContentTypeXML)
	w.Header().Set("Content-Length", strconv.Itoa(len(out)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(out)
}

// statusFor maps a Handle error to an HTTP status.
func statusFor(err error) int {
	switch {
	case errors.Is(err, ErrUnenrolled):
		return http.StatusForbidden
	case errors.Is(err, syncml.ErrSyntax), errors.Is(err, syncml.ErrInvalid),
		errors.Is(err, syncml.ErrTooLarge), errors.Is(err, syncml.ErrNamespace):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

// DefaultRequestBytesForTest exposes the default request bound to tests in
// the external test package.
func DefaultRequestBytesForTest() int { return syncml.DefaultMaxSize }
