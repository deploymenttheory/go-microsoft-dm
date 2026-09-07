package mdm

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
)

type recorder struct {
	code int
	hdr  http.Header
}

func newRecorder() *recorder { return &recorder{code: 200, hdr: http.Header{}} }

func (r *recorder) Header() http.Header  { return r.hdr }
func (r *recorder) Write(b []byte) (int, error) { return len(b), nil }
func (r *recorder) WriteHeader(c int)    { r.code = c }

func newPost(body io.ReadCloser) *http.Request {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/svc?mode=Machine", nil)
	req.Body = body
	req.Header.Set("Content-Type", "application/vnd.syncml.dm+xml")
	return req
}
