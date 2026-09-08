//go:build host && windows

package host

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// TestNativePush is opt-in and targets an already enrolled localhost test device.
func TestNativePush(t *testing.T) {
	state, id := os.Getenv("HOST_WNS_STATE_DIR"), os.Getenv("HOST_WNS_DEVICE_ID")
	if state == "" || id == "" || os.Getenv("DM_WNS_PFN") == "" || os.Getenv("DM_WNS_CLIENT_ID") == "" || os.Getenv("DM_WNS_CLIENT_SECRET") == "" {
		t.Skip("native WNS requires HOST_WNS_STATE_DIR, HOST_WNS_DEVICE_ID and DM_WNS credentials")
	}
	script, err := filepath.Abs("../../../scripts/enrollment/test-push.ps1")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "pwsh", "-NoProfile", "-File", script, "-StateDirectory", state, "-DeviceID", id)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("native push: %v\n%s", err, out)
	} else {
		t.Log(string(out))
	}
}
