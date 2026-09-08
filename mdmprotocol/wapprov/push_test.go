package wapprov_test

import (
	"testing"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/wapprov"
)

func TestOptionalPushPreservesPolling(t *testing.T) {
	for _, pfn := range []string{"", "Example.Test_12345"} {
		ch, err := wapprov.DMClient(wapprov.DMClientConfig{ProviderID: "test", PushPFN: pfn})
		if err != nil {
			t.Fatal(err)
		}
		provider := ch.Children[0].Children[0]
		if provider.Children[0].Type != "Poll" {
			t.Fatal("push removed polling fallback")
		}
		if pfn == "" {
			if len(provider.Children) != 1 {
				t.Fatal("unsolicited push configuration")
			}
			continue
		}
		push := provider.Children[1]
		if push.Type != "Push" || len(push.Parms) != 1 || push.Parms[0].Name != "PFN" || push.Parms[0].Value != pfn {
			t.Fatalf("invalid Push/PFN: %+v", push)
		}
	}
}
