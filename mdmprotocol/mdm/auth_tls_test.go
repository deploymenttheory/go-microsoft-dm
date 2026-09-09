package mdm_test

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/enroll"
	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/mdm"
	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/syncml"
	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/wapprov"
	"github.com/deploymenttheory/go-microsoft-dm/storage"
	"github.com/deploymenttheory/go-microsoft-dm/storage/inmem"
	"github.com/deploymenttheory/go-microsoft-dm/testpki"
)

func TestTLSCertificateAuthentication(t *testing.T) {
	t.Parallel()
	ca, err := testpki.NewCA("Enrollment CA")
	if err != nil {
		t.Fatal(err)
	}
	otherCA, err := testpki.NewCA("Other CA")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().Add(-time.Minute)
	issue := func(ca *testpki.CA, at time.Time) *testpki.Identity {
		t.Helper()
		id, err := ca.Issue(dev+"-certificate", at)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	enrolled := issue(ca, now)
	otherEnrollment := issue(ca, now)
	otherIssuer := issue(otherCA, now) // Same serial as enrolled, different certificate.
	expired := issue(ca, now.Add(-48*time.Hour))
	serverOnly, err := ca.IssueServer(dev, now)
	if err != nil {
		t.Fatal(err)
	}
	// A self-signed leaf copies both the device CN and enrollment serial.
	spoofTemplate := x509.Certificate{
		Subject: enrolled.Cert.Subject, SerialNumber: enrolled.Cert.SerialNumber,
		NotBefore: now, NotAfter: now.Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	spoofDER, err := x509.CreateCertificate(rand.Reader, &spoofTemplate, &spoofTemplate, otherIssuer.Key.Public(), otherIssuer.Key)
	if err != nil {
		t.Fatal(err)
	}
	spoofCert, err := x509.ParseCertificate(spoofDER)
	if err != nil {
		t.Fatal(err)
	}
	spoof := &testpki.Identity{Cert: spoofCert, Key: otherIssuer.Key}

	for _, tc := range []struct {
		name             string
		mode             tls.ClientAuthType
		client           *testpki.Identity
		custom           bool
		accept           bool
		trustOtherCA     bool
		want             int
		handshakeFailure bool
	}{
		{name: "verified enrollment", mode: tls.VerifyClientCertIfGiven, client: enrolled, want: 200},
		{name: "no certificate", mode: tls.VerifyClientCertIfGiven, want: 407},
		{name: "unverified enrollment", mode: tls.RequestClientCert, client: enrolled, want: 407},
		{name: "self signed matching CN and serial", mode: tls.RequestClientCert, client: spoof, want: 407},
		{name: "other enrollment same CN", mode: tls.VerifyClientCertIfGiven, client: otherEnrollment, want: 407},
		{name: "other CA same serial", mode: tls.VerifyClientCertIfGiven, client: otherIssuer, trustOtherCA: true, want: 407},
		{name: "custom rejection is final", mode: tls.VerifyClientCertIfGiven, client: enrolled, custom: true, want: 407},
		{name: "custom acceptance requires verification", mode: tls.RequestClientCert, client: spoof, custom: true, accept: true, want: 407},
		{name: "untrusted CA", mode: tls.VerifyClientCertIfGiven, client: otherIssuer, handshakeFailure: true},
		{name: "expired", mode: tls.VerifyClientCertIfGiven, client: expired, handshakeFailure: true},
		{name: "wrong extended key usage", mode: tls.VerifyClientCertIfGiven, client: serverOnly, handshakeFailure: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			store, queue := inmem.New(), inmem.NewQueue()
			if err := store.Create(ctx, &storage.Enrollment{
				DeviceID: dev, Serial: enrolled.Cert.SerialNumber.String(), Thumbprint: wapprov.Thumbprint(enrolled.Cert.Raw),
				EnrollmentType: enroll.EnrollmentTypeFull, EnrolledAt: now,
			}); err != nil {
				t.Fatal(err)
			}
			// The certificate shortcut must not bypass an existing digest credential.
			if err := store.PutMDMCredential(ctx, storage.MDMCredential{DeviceID: dev, AuthType: mdm.AuthDigest,
				CredentialHash: syncml.CredentialHash("u", "p")}); err != nil {
				t.Fatal(err)
			}
			auth := &storage.MDMAuthenticator{Enrollments: store, Credentials: store}
			var calls atomic.Int32
			if tc.custom {
				auth.TrustCert = func(*mdm.Identity, [][]*x509.Certificate) bool { calls.Add(1); return tc.accept }
			}
			svc, err := mdm.New(mdm.Config{ServerURL: "https://mdm.example.test/svc", Auth: auth, Queue: queue})
			if err != nil {
				t.Fatal(err)
			}
			cmd, err := mdm.NewGet([]string{"./DevDetail/SwV"})
			if err != nil {
				t.Fatal(err)
			}
			queued, err := queue.Enqueue(ctx, dev, cmd, now)
			if err != nil {
				t.Fatal(err)
			}
			pool := ca.Pool()
			if tc.trustOtherCA {
				pool.AddCert(otherCA.Cert)
			}
			srv := httptest.NewUnstartedServer(mdm.NewHandler(svc))
			srv.Config.ErrorLog = log.New(io.Discard, "", 0)
			srv.TLS = &tls.Config{MinVersion: tls.VersionTLS12, ClientAuth: tc.mode, ClientCAs: pool}
			srv.StartTLS()
			t.Cleanup(srv.Close)
			tr := srv.Client().Transport.(*http.Transport).Clone()
			t.Cleanup(tr.CloseIdleConnections)
			if tc.client != nil {
				// Always present the configured identity, even if its issuer is not
				// in the server's advertised CA list, to exercise TLS rejection.
				tr.TLSClientConfig.GetClientCertificate = func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
					return &tls.Certificate{Certificate: [][]byte{tc.client.Cert.Raw}, PrivateKey: tc.client.Key}, nil
				}
			}
			client := &http.Client{Transport: tr}
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, srv.URL, strings.NewReader(packageOne("1", "1")))
			if err != nil {
				t.Fatal(err)
			}
			res, err := client.Do(req)
			if tc.handshakeFailure {
				if err == nil {
					res.Body.Close()
					t.Fatal("invalid certificate completed TLS handshake")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			defer res.Body.Close()
			body, err := io.ReadAll(res.Body)
			if err != nil {
				t.Fatal(err)
			}
			msg, err := syncml.Decode(body, syncml.DecodeOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if got := msg.Body.Statuses()[0].Code(); got != syncml.StatusCode(tc.want) {
				t.Fatalf("SyncHdr status = %d, want %d", got, tc.want)
			}
			if tc.want == 407 {
				if len(msg.Body.Commands) != 1 {
					t.Error("unauthenticated request received commands")
				}
				got, err := queue.Get(ctx, dev, queued.ID)
				if err != nil || got.State != mdm.StatePending {
					t.Errorf("queued command state = %+v, %v", got, err)
				}
			}
			if tc.custom && tc.mode == tls.RequestClientCert && calls.Load() != 0 {
				t.Error("unverified peer reached trust callback")
			}
			if tc.custom && tc.mode == tls.VerifyClientCertIfGiven && calls.Load() != 1 {
				t.Error("verified peer did not reach trust callback")
			}
		})
	}
}
