package simulator

import (
	"os"
	"reflect"
	"testing"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/syncml"
)

func TestPackageOneMatchesNativeWindowsSemantics(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile("../mdmprotocol/syncml/testdata/captures/windows-26200.9278-package1-user.xml")
	if err != nil {
		t.Fatal(err)
	}
	native, err := syncml.Decode(body, syncml.DecodeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	device := &Device{
		DeviceID: native.Header.Source.LocURI, ManagementURL: native.Header.Target.LocURI,
		DevInfo: syncml.DevInfo(native), LoginStatus: syncml.LoginStatus(native),
		Namespace: native.Namespace,
	}
	simulated := device.packageOne(&sessionState{sessionID: native.Header.SessionID})
	if !reflect.DeepEqual(native.Header, simulated.Header) || !reflect.DeepEqual(syncml.DevInfo(native), syncml.DevInfo(simulated)) || syncml.LoginStatus(native) != syncml.LoginStatus(simulated) {
		t.Fatal("simulator package 1 differs from native identity, version, facts or login status")
	}
	if len(native.Body.Commands) != len(simulated.Body.Commands) || native.Body.Final != simulated.Body.Final {
		t.Fatal("package structure differs")
	}
	for i := range native.Body.Commands {
		if native.Body.Commands[i].Name() != simulated.Body.Commands[i].Name() {
			t.Fatal("package command order differs")
		}
	}
	// Native Windows starts these CmdIDs at 2; the simulator starts at 1.
	// CmdIDs are message-local correlation identifiers, not fixed positions.
	if native.Body.Commands[0].ID() != "2" || simulated.Body.Commands[0].ID() != "1" {
		t.Fatal("update the documented native/simulator CmdID difference")
	}
}
