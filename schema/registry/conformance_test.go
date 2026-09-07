package registry_test

import (
	"strings"
	"testing"

	"github.com/deploymenttheory/go-microsoft-dm/schema/csp"
	"github.com/deploymenttheory/go-microsoft-dm/schema/csp/declaredconfiguration"
	"github.com/deploymenttheory/go-microsoft-dm/schema/csp/devdetail"
	"github.com/deploymenttheory/go-microsoft-dm/schema/csp/devinfo"
	"github.com/deploymenttheory/go-microsoft-dm/schema/csp/dmacc"
	"github.com/deploymenttheory/go-microsoft-dm/schema/csp/dmclient"
	"github.com/deploymenttheory/go-microsoft-dm/schema/registry"
)

// TestRegistryCounts pins the bundle census on the generated output.
func TestRegistryCounts(t *testing.T) {
	t.Parallel()
	trees := registry.Trees()
	if len(trees) != 400 {
		t.Fatalf("trees = %d", len(trees))
	}
	nodes, areas, standalone := 0, 0, 0
	for _, tr := range trees {
		nodes += len(tr.Nodes())
		if tr.PolicyArea {
			areas++
		} else {
			standalone++
		}
	}
	if nodes != 5955 || areas != 335 || standalone != 65 {
		t.Fatalf("nodes %d areas %d standalone %d", nodes, areas, standalone)
	}
	if registry.Registry() != registry.Registry() {
		t.Fatal("registry is built once")
	}
}

// TestEveryGeneratedURIResolvesToItsNode walks every tree and resolves
// every node's URI (with a sample value in each dynamic segment) through
// the registry, requiring the same node back. This is the invariant the
// validation package relies on.
func TestEveryGeneratedURIResolvesToItsNode(t *testing.T) {
	t.Parallel()
	reg := registry.Registry()
	checked := 0
	for _, tr := range registry.Trees() {
		for _, n := range tr.Nodes() {
			uri := n.URI
			if strings.Contains(uri, "{") {
				params := map[string]string{}
				for _, seg := range strings.Split(uri, "/") {
					if len(seg) > 2 && seg[0] == '{' && seg[len(seg)-1] == '}' {
						params[seg[1:len(seg)-1]] = "sample value"
					}
				}
				uri = n.Concrete(params)
			}
			m, gotTree, ok := reg.Lookup(uri)
			if !ok {
				t.Errorf("%s: %s does not resolve", tr.File, uri)
				continue
			}
			if m.Node != n {
				t.Errorf("%s: %s resolved to %s in %s", tr.File, uri, m.Node.URI, gotTree.File)
			}
			checked++
		}
	}
	if checked != 5955 {
		t.Fatalf("checked %d", checked)
	}
}

// TestNodesTheProtocolPhasesUse checks the nodes research sections 1.4 and
// 1.6 enumerate for the enrollment, session, push and WinDC work.
func TestNodesTheProtocolPhasesUse(t *testing.T) {
	t.Parallel()
	reg := registry.Registry()
	uris := []string{
		dmclient.DeviceProviderEntDMID("MS DM Server"),
		dmclient.DeviceProviderEntDeviceName("MS DM Server"),
		dmclient.DeviceProviderPollNumberOfFirstRetries("MS DM Server"),
		dmclient.DeviceProviderPollIntervalForFirstSetOfRetries("MS DM Server"),
		dmclient.DeviceProviderPushChannelURI("MS DM Server"),
		dmclient.DeviceProviderPushPFN("MS DM Server"),
		dmclient.DeviceProviderSyncApplicationVersion("MS DM Server"),
		dmclient.DeviceProviderMaxSyncApplicationVersion("MS DM Server"),
		dmclient.DeviceProviderEnableOmaDmKeepAliveMessage("MS DM Server"),
		dmclient.DeviceProviderRequireMessageSigning("MS DM Server"),
		dmclient.DeviceProviderUnenroll("MS DM Server"),
		dmclient.DeviceProviderLinkedEnrollmentDiscoveryEndpoint("MS DM Server"),
		dmclient.DeviceProviderLinkedEnrollmentEnroll("MS DM Server"),
		dmclient.DeviceProviderLinkedEnrollmentEnrollStatus("MS DM Server"),
		dmclient.DeviceProviderFirstSyncStatusIsSyncDone("MS DM Server"),
		dmclient.DeviceProviderRecoveryInitiateRecovery("MS DM Server"),
		dmclient.DeviceHWDevID,
		dmclient.DeviceUnenroll,
		devdetail.SwV, devdetail.LrgObj, devdetail.URIMaxSegLen, devdetail.ExtMicrosoftOSPlatform,
		devinfo.DevId, devinfo.Man, devinfo.Mod, devinfo.DmV, devinfo.Lang,
		dmacc.AppID("acct"), dmacc.AppAuthAAuthType("acct", "auth"),
		declaredconfiguration.HostCompleteDocumentsDocument("11111111-2222-3333-4444-555555555555"),
		declaredconfiguration.HostCompleteDocumentsPropertiesAbandoned("11111111-2222-3333-4444-555555555555"),
		declaredconfiguration.HostCompleteResultsDocument("11111111-2222-3333-4444-555555555555"),
		declaredconfiguration.HostInventoryDocumentsDocument("11111111-2222-3333-4444-555555555555"),
		declaredconfiguration.ManagementServiceConfigurationConflictResolution,
		"./Vendor/MSFT/Policy/Config/BITS/JobInactivityTimeout",
		"./User/Vendor/MSFT/Policy/Config/Browser/AllowAutofill",
		"./Device/Vendor/MSFT/Policy/Config/ADMX_AppCompat/AppCompatTurnOffProgramCompatibilityAssistant_1",
		"./Device/Vendor/MSFT/Reboot/RebootNow",
		"./Device/Vendor/MSFT/RemoteWipe/doWipe",
		"./Device/Vendor/MSFT/EnterpriseDesktopAppManagement/MSI/%7B1803A630-3C38-4D2B-9B9A-0CB37243539C%7D/DownloadInstall",
		"./Device/Vendor/MSFT/DeviceManageability/Capabilities/CSPVersions",
		"./Vendor/MSFT/DeviceStatus/CertAttestation/MDMClientCertAttestation",
		"./Vendor/MSFT/DMClient/Provider/MS%20DM%20SERVER/Poll/IntervalForFirstSetOfRetries",
	}
	for _, uri := range uris {
		if _, _, ok := reg.Lookup(uri); !ok {
			t.Errorf("%s does not resolve", uri)
		}
	}
	// Exec-only and value facts the session engine relies on.
	m, _, _ := reg.Lookup("./Device/Vendor/MSFT/Reboot/RebootNow")
	if !m.Node.Allows(csp.AccessExec) || m.Node.Format != csp.FormatNull {
		t.Errorf("RebootNow %+v", m.Node)
	}
	m, _, _ = reg.Lookup(dmclient.DeviceProviderPollNumberOfFirstRetries("x"))
	if m.Node.Format != csp.FormatInt || m.Node.Applicability == nil || m.Node.OwnApplicability {
		t.Errorf("NumberOfFirstRetries must inherit applicability: %+v", m.Node)
	}
	m, tr, _ := reg.Lookup(declaredconfiguration.HostCompleteDocumentsDocument("11111111-2222-3333-4444-555555555555"))
	if tr.Name != "DeclaredConfiguration" || m.Node.Format != csp.FormatChr || m.Params["DocID"] != "11111111-2222-3333-4444-555555555555" {
		t.Errorf("WinDC document %+v %v", m.Node, m.Params)
	}
	if m.Node.Applicability == nil || len(m.Node.Applicability.OsBuildVersions) == 0 {
		t.Errorf("WinDC document lacks applicability")
	}
	// The DeclaredConfiguration DDF lacks RefreshInterval and BulkTemplate
	// (open question 6); assert the absence so a future drop is noticed.
	for _, absent := range []string{
		"./Device/Vendor/MSFT/DeclaredConfiguration/ManagementServiceConfiguration/RefreshInterval",
		"./Device/Vendor/MSFT/DeclaredConfiguration/Host/BulkTemplate",
		"./User/Vendor/MSFT/DeclaredConfiguration/Host/Complete",
	} {
		if _, _, ok := reg.Lookup(absent); ok {
			t.Errorf("%s resolved; the DDF gained it, update decision record 0006 and open question 6", absent)
		}
	}
	if dmclient.DeviceProviderEntDMID("MS DM Server") != "./Device/Vendor/MSFT/DMClient/Provider/MS%20DM%20Server/EntDMID" {
		t.Errorf("escape: %s", dmclient.DeviceProviderEntDMID("MS DM Server"))
	}
	if dmclient.DeviceProviderFirstSyncStatusIsSyncDone("x") == dmclient.UserProviderFirstSyncStatusIsSyncDone("x") {
		t.Error("device and user trees must differ")
	}
}
