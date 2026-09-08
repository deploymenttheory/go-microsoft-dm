//go:build host

// Package host contains the opt-in native desktop conformance harness.
// Experimental wire changes belong here, not in the production server.
package host

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/syncml"
)

// Experiment is reloaded per request so an enrolled desktop can test different
// response encodings without losing server session state. Zero means baseline.
type Experiment struct {
	Label             string `json:"label"`
	EnrollmentVersion string `json:"enrollment_version,omitempty"`
	Namespace         string `json:"namespace,omitempty"`
	MaxMsgSize        int    `json:"max_msg_size,omitempty"`
	FirstRetries      *int   `json:"first_retries,omitempty"`
	FirstInterval     *int   `json:"first_interval,omitempty"`
}

// Recorder saves exact HTTP bodies privately before publishing any fixtures.
type Recorder struct {
	Next           http.Handler
	Directory      string
	ExperimentFile string
	seq            atomic.Uint64
}

func (h *Recorder) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var exp Experiment
	if h.ExperimentFile != "" {
		data, err := os.ReadFile(h.ExperimentFile)
		if err != nil {
			http.Error(w, "read experiment configuration", http.StatusInternalServerError)
			return
		}
		if err := json.Unmarshal(data, &exp); err != nil {
			http.Error(w, "invalid experiment configuration", http.StatusInternalServerError)
			return
		}
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 16<<20+1))
	if err != nil || len(body) > 16<<20 {
		http.Error(w, "capture request exceeds limit", http.StatusRequestEntityTooLarge)
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	response := httptest.NewRecorder()
	h.Next.ServeHTTP(response, r)
	out, err := transform(response.Body.Bytes(), exp)
	if err != nil {
		http.Error(w, "invalid experimental response", http.StatusInternalServerError)
		return
	}
	stamp := time.Now().UTC()
	stem := fmt.Sprintf("%s-%06d", stamp.Format("20060102T150405.000000000Z"), h.seq.Add(1))
	meta, _ := json.MarshalIndent(struct {
		At         time.Time  `json:"at"`
		Method     string     `json:"method"`
		Path       string     `json:"path"`
		Status     int        `json:"status"`
		Experiment Experiment `json:"experiment"`
	}{stamp, r.Method, r.URL.Path, response.Code, exp}, "", "  ")
	for suffix, data := range map[string][]byte{"request.xml": body, "response.xml": out, "metadata.json": meta} {
		if err := os.WriteFile(filepath.Join(h.Directory, stem+"-"+suffix), data, 0600); err != nil {
			http.Error(w, "write capture", http.StatusInternalServerError)
			return
		}
	}
	for k, values := range response.Header() {
		w.Header()[k] = values
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(out)))
	w.WriteHeader(response.Code)
	_, _ = w.Write(out)
}

var enrollmentVersion = regexp.MustCompile(`(<EnrollmentVersion>)[^<]*(</EnrollmentVersion>)`)
var binaryToken = regexp.MustCompile(`(<(?:[[:alnum:]_]+:)?BinarySecurityToken\b[^>]*>)([^<]+)(</(?:[[:alnum:]_]+:)?BinarySecurityToken>)`)

func transform(body []byte, exp Experiment) ([]byte, error) {
	if exp.EnrollmentVersion != "" {
		if exp.EnrollmentVersion != "3.0" && exp.EnrollmentVersion != "5.0" && exp.EnrollmentVersion != "9.0" {
			return nil, fmt.Errorf("unsupported enrollment version")
		}
		body = enrollmentVersion.ReplaceAll(body, []byte("${1}"+exp.EnrollmentVersion+"${2}"))
	}
	if exp.FirstRetries != nil || exp.FirstInterval != nil {
		var replaceErr error
		body = binaryToken.ReplaceAllFunc(body, func(match []byte) []byte {
			parts := binaryToken.FindSubmatch(match)
			doc, err := base64.StdEncoding.DecodeString(string(parts[2]))
			if err != nil {
				replaceErr = err
				return match
			}
			for name, value := range map[string]*int{"NumberOfFirstRetries": exp.FirstRetries, "IntervalForFirstSetOfRetries": exp.FirstInterval} {
				if value != nil {
					if *value < 0 {
						replaceErr = fmt.Errorf("negative polling value")
						return match
					}
					re := regexp.MustCompile(`(<parm name="` + name + `" value=")[^"]*(")`)
					doc = re.ReplaceAll(doc, []byte("${1}"+strconv.Itoa(*value)+"${2}"))
				}
			}
			return []byte(string(parts[1]) + base64.StdEncoding.EncodeToString(doc) + string(parts[3]))
		})
		if replaceErr != nil {
			return nil, replaceErr
		}
	}
	if bytes.Contains(body, []byte("<SyncML")) && (exp.Namespace != "" || exp.MaxMsgSize != 0) {
		if exp.Namespace != "" && exp.Namespace != syncml.NamespaceSyncML11 && exp.Namespace != syncml.NamespaceSyncML12 {
			return nil, fmt.Errorf("unsupported namespace")
		}
		if exp.MaxMsgSize != 0 {
			msg, err := syncml.Decode(body, syncml.DecodeOptions{})
			if err != nil {
				return nil, err
			}
			if msg.Header.Meta == nil {
				msg.Header.Meta = &syncml.Meta{}
			}
			msg.Header.Meta.MaxMsgSize = int64(exp.MaxMsgSize)
			body, err = syncml.Encode(msg, syncml.EncodeOptions{})
			if err != nil {
				return nil, err
			}
		}
		if exp.Namespace != "" {
			body = []byte(strings.ReplaceAll(strings.ReplaceAll(string(body), syncml.NamespaceSyncML11, exp.Namespace), syncml.NamespaceSyncML12, exp.Namespace))
		}
	}
	return body, nil
}
