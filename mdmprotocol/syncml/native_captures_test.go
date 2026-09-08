package syncml_test

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/syncml"
)

func TestNativeWindowsCaptures(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob("testdata/captures/*.xml")
	if err != nil || len(files) < 9 {
		t.Fatalf("native captures: %d, %v", len(files), err)
	}
	for _, path := range files {
		t.Run(filepath.Base(path), func(t *testing.T) {
			name := filepath.Base(path)
			msg, err := syncml.Decode(fixture(t, filepath.Join("captures", name)), syncml.DecodeOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if err := syncml.Validate(msg); err != nil {
				t.Fatal(err)
			}
			out, err := syncml.Encode(msg, syncml.EncodeOptions{})
			if err != nil {
				t.Fatal(err)
			}
			back, err := syncml.Decode(out, syncml.DecodeOptions{})
			if err != nil || !reflect.DeepEqual(msg, back) {
				t.Fatalf("native message changed on round-trip: %v", err)
			}
			switch {
			case strings.Contains(name, "package1-"):
				if !syncml.IsPackageOne(msg) || syncml.LoginStatus(msg) != syncml.LoginStatusUser {
					t.Fatal("native initialization lost its user login status")
				}
			case strings.Contains(name, "unenroll-1226"):
				if !msg.Body.HasAlert(syncml.AlertGeneric) {
					t.Fatal("native unenrollment alert missing")
				}
			case strings.Contains(name, "large-result-abort"):
				if !msg.Body.HasAlert(syncml.AlertSessionAbort) {
					t.Fatal("native large-result rejection lost its abort alert")
				}
			case strings.Contains(name, "namespace11-response"):
				if msg.Namespace != syncml.NamespaceSyncML11 {
					t.Fatal("server experiment namespace changed")
				}
			case strings.Contains(name, "capability-probes"):
				var rejected int
				for _, status := range msg.Body.Statuses() {
					if status.Data.Text() == "406" {
						rejected++
					}
				}
				if rejected != 3 {
					t.Fatalf("WinDC probe rejections = %d, want 3", rejected)
				}
			}
		})
	}
}
