package mdm_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/mdm"
	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/syncml"
	"github.com/deploymenttheory/go-microsoft-dm/storage/inmem"
)

func TestReenrollmentDoesNotReuseSessionAuthentication(t *testing.T) {
	t.Parallel()
	for _, msgID := range []string{"1", "2", "10"} {
		t.Run(msgID, func(t *testing.T) {
			ctx := context.Background()
			sessions := mdm.NewMemorySessions()
			sessions.Now = func() time.Time { return t0 }
			if err := sessions.PutSession(ctx, &mdm.Session{
				Key: mdm.SessionKey(dev, "1"), DeviceID: dev, SessionID: "1", EnrollmentKey: "old",
				ClientMsgID: 9, ServerMsgID: 9, Authenticated: true, LastSeen: t0,
				Sent: map[string]string{}, Children: map[string]string{},
			}); err != nil {
				t.Fatal(err)
			}
			svc, err := mdm.New(mdm.Config{ServerURL: "https://mdm.example.test/svc", Auth: fakeAuth{authType: mdm.AuthDigest}, Queue: inmem.NewQueue(), Sessions: sessions})
			if err != nil {
				t.Fatal(err)
			}
			out, err := svc.Handle(ctx, nil, []byte(packageOne("1", msgID)))
			if msgID != "1" {
				if !errors.Is(err, syncml.ErrInvalid) {
					t.Fatalf("new enrollment continued an old session: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			response, err := syncml.Decode(out, syncml.DecodeOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if response.Header.MsgID != "1" || response.Body.Statuses()[0].Code() != syncml.StatusAuthenticationRequired {
				t.Fatal("new enrollment must restart session numbering and authenticate again")
			}
		})
	}
}
