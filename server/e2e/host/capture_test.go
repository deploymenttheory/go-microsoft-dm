//go:build host

package host

import (
	"bytes"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/syncml"
)

func TestRecorderPreservesWireAndLength(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	wire := []byte(`<SyncML xmlns="SYNCML:SYNCML1.2"><SyncHdr/><SyncBody><Final/></SyncBody></SyncML>`)
	h := &Recorder{Directory: dir, Next: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", syncml.ContentTypeXML)
		_, _ = w.Write(wire)
	})}
	out := httptest.NewRecorder()
	h.ServeHTTP(out, httptest.NewRequest(http.MethodPost, "/ManagementServer/MDM.svc", bytes.NewReader(wire)))
	if out.Code != 200 || !bytes.Equal(out.Body.Bytes(), wire) || out.Header().Get("Content-Length") == "" {
		t.Fatal("baseline recorder changed the response")
	}
	files, err := filepath.Glob(filepath.Join(dir, "*-request.xml"))
	if err != nil || len(files) != 1 {
		t.Fatalf("capture files: %v, %v", files, err)
	}
	got, err := os.ReadFile(files[0])
	if err != nil || !bytes.Equal(got, wire) {
		t.Fatal("request body was not captured exactly")
	}
}

func TestExperimentalEnrollmentResponse(t *testing.T) {
	t.Parallel()
	zero, one := 0, 1
	doc := `<wap-provisioningdoc><parm name="NumberOfFirstRetries" value="5"/><parm name="IntervalForFirstSetOfRetries" value="15"/></wap-provisioningdoc>`
	body := `<EnrollmentVersion>3.0</EnrollmentVersion><o:BinarySecurityToken>` + base64.StdEncoding.EncodeToString([]byte(doc)) + `</o:BinarySecurityToken>`
	out, err := transform([]byte(body), Experiment{EnrollmentVersion: "9.0", FirstRetries: &zero, FirstInterval: &one})
	if err != nil || !bytes.Contains(out, []byte(`<EnrollmentVersion>9.0</EnrollmentVersion>`)) {
		t.Fatalf("version: %s, %v", out, err)
	}
	parts := binaryToken.FindSubmatch(out)
	decoded, err := base64.StdEncoding.DecodeString(string(parts[2]))
	if err != nil || !bytes.Contains(decoded, []byte(`name="NumberOfFirstRetries" value="0"`)) || !bytes.Contains(decoded, []byte(`name="IntervalForFirstSetOfRetries" value="1"`)) {
		t.Fatalf("poll experiment not applied: %s, %v", decoded, err)
	}
}

func TestExperimentalSyncMLResponse(t *testing.T) {
	t.Parallel()
	body := []byte(`<SyncML xmlns="SYNCML:SYNCML1.2"><SyncHdr><VerDTD>1.2</VerDTD><VerProto>DM/1.2</VerProto><SessionID>1</SessionID><MsgID>1</MsgID><Target><LocURI>device</LocURI></Target><Source><LocURI>https://localhost:8443</LocURI></Source></SyncHdr><SyncBody><Final/></SyncBody></SyncML>`)
	out, err := transform(body, Experiment{Namespace: syncml.NamespaceSyncML11, MaxMsgSize: 2048})
	if err != nil {
		t.Fatal(err)
	}
	msg, err := syncml.Decode(out, syncml.DecodeOptions{})
	if err != nil || msg.Namespace != syncml.NamespaceSyncML11 || msg.Header.Meta.MaxMsgSize != 2048 {
		t.Fatalf("unexpected transformed message: %s, %v", out, err)
	}
	if !strings.Contains(string(out), "<VerProto>DM/1.2</VerProto>") {
		t.Fatal("namespace experiment changed protocol version")
	}
}
