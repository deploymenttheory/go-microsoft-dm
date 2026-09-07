package support_test

import (
	"errors"
	"testing"

	"github.com/deploymenttheory/go-microsoft-dm/schema/csp"
	"github.com/deploymenttheory/go-microsoft-dm/schema/support"
)

func node(versions ...string) *csp.Node {
	return &csp.Node{URI: "./x", Applicability: &csp.Applicability{OsBuildVersions: versions}}
}

func TestParseBuild(t *testing.T) {
	t.Parallel()
	b, err := support.ParseBuild("10.0.26100.9278")
	if err != nil || b != (support.Build{Major: 10, Minor: 0, Build: 26100, Revision: 9278}) || b.String() != "10.0.26100.9278" {
		t.Fatal(b, err)
	}
	if b, err := support.ParseBuild(" 10.0.22000 "); err != nil || b.Revision != 0 || b.String() != "10.0.22000" {
		t.Fatal(b, err)
	}
	for _, bad := range []string{"", "10.0", "10.0.x", "10.0.1.2.3", "-1.0.1"} {
		if _, err := support.ParseBuild(bad); !errors.Is(err, support.ErrBuild) {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestClassify(t *testing.T) {
	t.Parallel()
	for entry, want := range map[string]support.Kind{
		"10.0.26100.7019": support.Released, "11.0.26200.7019": support.Released, "10.0.22000": support.Released,
		"99.9.99999": support.Unreleased, "99.9.9999": support.Unreleased, "88.8.88888": support.Unreleased,
		"10.0.20348.2227": support.Server, "10.0.25398.643": support.Server,
		"10.0.25000": support.Insider, "10.0.26000": support.Insider, "10.0.25145": support.Insider, "10.0.25965": support.Insider,
	} {
		b, k, err := support.Classify(entry)
		if err != nil || k != want {
			t.Errorf("%s: %v %v", entry, k, err)
		}
		if entry == "11.0.26200.7019" && b.Major != 10 {
			t.Error("11.0 quirk not normalised")
		}
	}
	if _, _, err := support.Classify("nope"); !errors.Is(err, support.ErrBuild) {
		t.Fatal("bad entry")
	}
	if support.Released.String() != "released" || support.Insider.String() != "insider" || support.Server.String() != "server" || support.Unreleased.String() != "unreleased" {
		t.Fatal("kind names")
	}
}

func TestSupportedOn(t *testing.T) {
	t.Parallel()
	dev24H2, _ := support.ParseBuild("10.0.26100.9278")
	dev26H2, _ := support.ParseBuild("10.0.26300.9278")
	dev23H2, _ := support.ParseBuild("10.0.22631.7517")
	w10, _ := support.ParseBuild("10.0.19045.4717")
	old24H2, _ := support.ParseBuild("10.0.26100.1000")
	cases := map[string]struct {
		n     *csp.Node
		dev   support.Build
		want  bool
		match string
	}{
		"older major release":            {node("10.0.10240"), dev24H2, true, "10.0.10240"},
		"same branch, revision met":      {node("10.0.26100.7019"), dev24H2, true, "10.0.26100.7019"},
		"same branch, revision not yet":  {node("10.0.26100.7019"), old24H2, false, ""},
		"26H2 shares the 26100 branch":   {node("10.0.26100.3613"), dev26H2, true, "10.0.26100.3613"},
		"26200 entry on a 26300 device":  {node("11.0.26200.7019", "11.0.26100.7019"), dev26H2, true, "11.0.26200.7019"},
		"backport list picks the branch": {node("10.0.22000", "10.0.19043.1202", "10.0.19042.1202", "10.0.19041.1202"), w10, true, "10.0.19043.1202"},
		"backport revision not met":      {node("10.0.22000", "10.0.19041.9999"), w10, false, ""},
		"23H2 on the 22621 branch":       {node("10.0.22621.5126"), dev23H2, true, "10.0.22621.5126"},
		"newer branch than the device":   {node("10.0.26100"), dev23H2, false, ""},
		"sentinel only":                  {node("99.9.99999"), dev24H2, false, ""},
		"sentinel plus released":         {node("99.9.99999", "10.0.26100.3613"), dev24H2, true, "10.0.26100.3613"},
		"server only":                    {node("10.0.20348"), dev24H2, false, ""},
		"insider only":                   {node("10.0.25000"), dev24H2, false, ""},
		"no applicability":               {&csp.Node{URI: "./x"}, dev24H2, true, ""},
		"empty applicability":            {&csp.Node{URI: "./x", Applicability: &csp.Applicability{}}, dev24H2, true, ""},
		"unparsable entry is skipped":    {node("garbage", "10.0.10240"), dev24H2, true, "10.0.10240"},
		"unparsable only":                {node("garbage"), dev24H2, false, ""},
	}
	for name, tc := range cases {
		got := support.SupportedOn(tc.n, tc.dev)
		if got.Supported != tc.want || got.Matched != tc.match || got.Reason == "" {
			t.Errorf("%s: %+v", name, got)
		}
	}
	// Reasons name the cause.
	for name, tc := range map[string]struct {
		n    *csp.Node
		want string
	}{
		"sentinel": {node("99.9.99999"), "sentinel"},
		"server":   {node("10.0.20348"), "Server"},
		"insider":  {node("10.0.25000"), "Insider"},
		"newer":    {node("10.0.26100"), "requires"},
	} {
		if r := support.SupportedOn(tc.n, dev23H2); !contains(r.Reason, tc.want) {
			t.Errorf("%s: reason %q", name, r.Reason)
		}
	}
	// Deprecation.
	dep := node("10.0.10240")
	dep.Deprecated = &csp.Deprecated{OsBuildDeprecated: "10.0.22000"}
	if r := support.SupportedOn(dep, dev24H2); !r.Supported || !r.Deprecated {
		t.Fatalf("deprecated on 24H2: %+v", r)
	}
	if r := support.SupportedOn(dep, w10); r.Deprecated {
		t.Fatalf("not yet deprecated on Windows 10: %+v", r)
	}
	dep.Deprecated = &csp.Deprecated{}
	if r := support.SupportedOn(dep, w10); !r.Deprecated {
		t.Fatal("bare Deprecated applies everywhere")
	}
	dep.Deprecated = &csp.Deprecated{OsBuildDeprecated: "10.0.26100.712"}
	early, _ := support.ParseBuild("10.0.26100.500")
	if r := support.SupportedOn(dep, early); r.Deprecated {
		t.Fatal("deprecation revision not reached")
	}
	dep.Deprecated = &csp.Deprecated{OsBuildDeprecated: "junk"}
	if r := support.SupportedOn(dep, old24H2); r.Deprecated {
		t.Fatal("unparsable deprecation ignored")
	}
}

func contains(s, sub string) bool {
	return len(sub) > 0 && len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func TestEditionAllowed(t *testing.T) {
	t.Parallel()
	n := &csp.Node{Applicability: &csp.Applicability{EditionAllowList: []string{"0x4", "0x1B", "0x30"}}}
	if !support.EditionAllowed(n, "0x30") || !support.EditionAllowed(n, "0X1b") || support.EditionAllowed(n, "0x88") {
		t.Fatal("membership")
	}
	if !support.EditionAllowed(&csp.Node{}, "0x88") {
		t.Fatal("no list allows all")
	}
}
