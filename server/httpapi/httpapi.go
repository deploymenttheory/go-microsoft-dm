package httpapi

import (
	"net/http"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/enroll"
	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/mdm"
	"github.com/deploymenttheory/go-microsoft-dm/server/service"
)

// Handler routes the enrollment and management endpoints.
type Handler struct {
	mux *http.ServeMux
	// Log, when set, receives the method, path and status of each request.
	Log func(method, path string, status int)
}

// New builds the HTTP handler for a composed service.
func New(svc *service.Service) *Handler {
	h := &Handler{mux: http.NewServeMux()}
	enrollHandler := enroll.NewHandler(svc.Enroll)
	mgmtHandler := mdm.NewHandler(svc.Management)

	// The enrollment handler owns discovery, policy and enrollment; it routes
	// internally by path, so mount it on each enrollment path.
	h.mux.Handle(svc.Paths.Discovery, enrollHandler)
	h.mux.Handle(svc.Paths.Policy, enrollHandler)
	h.mux.Handle(svc.Paths.Enrollment, enrollHandler)
	h.mux.Handle(svc.Paths.Management, mgmtHandler)
	return h
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.Log == nil {
		h.mux.ServeHTTP(w, r)
		return
	}
	rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
	h.mux.ServeHTTP(rec, r)
	h.Log(r.Method, r.URL.Path, rec.status)
}

// statusRecorder captures the response status for the access log.
type statusRecorder struct {
	http.ResponseWriter
	status  int
	written bool
}

func (r *statusRecorder) WriteHeader(code int) {
	if !r.written {
		r.status = code
		r.written = true
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	r.written = true
	return r.ResponseWriter.Write(b)
}
