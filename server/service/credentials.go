package service

import (
	"context"
	"fmt"

	"github.com/deploymenttheory/go-microsoft-dm/clock"
	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/enroll"
	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/mdm"
	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/syncml"
	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/wapprov"
	"github.com/deploymenttheory/go-microsoft-dm/storage"
)

// credentialSource mints the OMA DM account credentials for an enrollment and
// persists the APPSRV secret so the management session can authenticate the
// device.
type credentialSource struct {
	store storage.CredentialStore
	clock clock.Clock
}

var _ enroll.CredentialSource = (*credentialSource)(nil)

// Credentials implements enroll.CredentialSource. The APPSRV credential (how
// the device authenticates to the server) is DIGEST; its H(name:secret) is
// stored keyed by DeviceID. The CLIENT credential (how the server would
// authenticate to the device) is also DIGEST, as MS-MDE2 2.2.9.5 requires,
// and is not verified by this reference server.
func (c *credentialSource) Credentials(ctx context.Context, e *enroll.Enrollment) (wapprov.Credential, wapprov.Credential, error) {
	deviceID := e.Request.Context.DeviceID
	name := deviceID
	serverSecret, err := randomSecret(24)
	if err != nil {
		return wapprov.Credential{}, wapprov.Credential{}, fmt.Errorf("service: credential: %w", err)
	}
	clientSecret, err := randomSecret(24)
	if err != nil {
		return wapprov.Credential{}, wapprov.Credential{}, fmt.Errorf("service: credential: %w", err)
	}
	if err := c.store.PutMDMCredential(ctx, storage.MDMCredential{
		DeviceID: deviceID, AuthType: mdm.AuthDigest, CredentialHash: syncml.CredentialHash(name, serverSecret),
	}); err != nil {
		return wapprov.Credential{}, wapprov.Credential{}, fmt.Errorf("service: store credential: %w", err)
	}
	server := wapprov.Credential{Type: wapprov.AuthDigest, Name: name, Secret: serverSecret}
	client := wapprov.Credential{Type: wapprov.AuthDigest, Secret: clientSecret}
	return server, client, nil
}
