package support

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/deploymenttheory/go-microsoft-dm/schema/csp"
)

// ErrBuild reports an unparsable build string.
var ErrBuild = errors.New("support: invalid build")

// Build is a Windows build number: Major.Minor.Build[.Revision].
type Build struct {
	Major, Minor, Build, Revision int
}

// ParseBuild parses "10.0.26100.9278" or "10.0.26100".
func ParseBuild(s string) (Build, error) {
	parts := strings.Split(strings.TrimSpace(s), ".")
	if len(parts) < 3 || len(parts) > 4 {
		return Build{}, fmt.Errorf("%w: %q", ErrBuild, s)
	}
	var n [4]int
	for i, p := range parts {
		v, err := strconv.Atoi(p)
		if err != nil || v < 0 {
			return Build{}, fmt.Errorf("%w: %q", ErrBuild, s)
		}
		n[i] = v
	}
	return Build{Major: n[0], Minor: n[1], Build: n[2], Revision: n[3]}, nil
}

// String renders the build without a zero revision.
func (b Build) String() string {
	if b.Revision == 0 {
		return fmt.Sprintf("%d.%d.%d", b.Major, b.Minor, b.Build)
	}
	return fmt.Sprintf("%d.%d.%d.%d", b.Major, b.Minor, b.Build, b.Revision)
}

// Kind classifies a DDF OsBuildVersion entry.
type Kind int

const (
	// Released is a build on a general-availability branch.
	Released Kind = iota
	// Insider is an Insider build (25000, 26000, 25145, 25965 and the like).
	Insider
	// Server is a Windows Server build (20348, 25398).
	Server
	// Unreleased is a sentinel (99.9.99999, 99.9.9999, 88.8.88888).
	Unreleased
)

func (k Kind) String() string {
	switch k {
	case Released:
		return "released"
	case Insider:
		return "insider"
	case Server:
		return "server"
	default:
		return "unreleased"
	}
}

// Classify says what a DDF OsBuildVersion entry denotes and normalises the
// two nodes stamped 11.0.x, which the research store records as a data
// quirk, to 10.0.x.
func Classify(entry string) (Build, Kind, error) {
	b, err := ParseBuild(entry)
	if err != nil {
		return Build{}, Unreleased, err
	}
	switch {
	case b.Major >= 88:
		return b, Unreleased, nil
	case b.Major == 11 && b.Minor == 0:
		b.Major = 10
	}
	switch b.Build {
	case 20348, 25398:
		return b, Server, nil
	case 25000, 26000, 25145, 25965:
		return b, Insider, nil
	}
	return b, Released, nil
}

// branch maps a build to its servicing branch. 24H2 (26100), 25H2 (26200)
// and 26H2 (26300) are one branch; 22H2 and 23H2 (22621, 22631) are one;
// 2004 to 22H2 on Windows 10 (19041 to 19045) are one.
func branch(build int) int {
	switch build {
	case 26200, 26300:
		return 26100
	case 22631:
		return 22621
	case 19042, 19043, 19044, 19045:
		return 19041
	}
	return build
}

// Result is the answer to SupportedOn.
type Result struct {
	Supported bool
	// Reason says why, in one sentence, for dmctl explain and logs.
	Reason string
	// Matched is the OsBuildVersion entry that decided it, or "".
	Matched string
	// Deprecated is set when the node is deprecated on or before the build.
	Deprecated bool
}

// SupportedOn reports whether the node applies to a device on build. A node
// without applicability inherits none and is treated as supported. A node
// applies when any released entry has the same branch and a revision no
// later than the device's, or an earlier branch. Insider, Server and
// sentinel entries never satisfy a desktop build.
func SupportedOn(n *csp.Node, build Build) Result {
	if n.Applicability == nil || len(n.Applicability.OsBuildVersions) == 0 {
		return Result{Supported: true, Reason: "no applicability recorded; assumed supported"}
	}
	res := Result{}
	dev := branch(build.Build)
	for _, entry := range n.Applicability.OsBuildVersions {
		b, kind, err := Classify(entry)
		if err != nil {
			continue
		}
		if kind != Released {
			continue
		}
		eb := branch(b.Build)
		switch {
		case eb < dev:
			res = Result{Supported: true, Reason: fmt.Sprintf("introduced in %s, before the device's %s branch", b, buildName(dev)), Matched: entry}
		case eb == dev && b.Revision <= build.Revision:
			res = Result{Supported: true, Reason: fmt.Sprintf("introduced in %s on the device's branch", b), Matched: entry}
		}
		if res.Supported {
			break
		}
	}
	if !res.Supported {
		kinds := map[Kind]bool{}
		for _, entry := range n.Applicability.OsBuildVersions {
			if _, k, err := Classify(entry); err == nil {
				kinds[k] = true
			}
		}
		switch {
		case kinds[Unreleased] && !kinds[Released]:
			res.Reason = "not in a released build (sentinel applicability)"
		case kinds[Server] && !kinds[Released]:
			res.Reason = "Windows Server only"
		case kinds[Insider] && !kinds[Released]:
			res.Reason = "Insider builds only"
		default:
			res.Reason = fmt.Sprintf("requires %s or later; device is %s", strings.Join(n.Applicability.OsBuildVersions, " or "), build)
		}
	}
	if n.Deprecated != nil {
		if n.Deprecated.OsBuildDeprecated == "" {
			res.Deprecated = true
		} else if d, err := ParseBuild(n.Deprecated.OsBuildDeprecated); err == nil {
			db := branch(d.Build)
			if db < dev || db == dev && d.Revision <= build.Revision {
				res.Deprecated = true
			}
		}
	}
	return res
}

func buildName(build int) string {
	switch build {
	case 26100:
		return "24H2/25H2/26H2 (26100)"
	case 22621:
		return "22H2/23H2 (22621)"
	case 22000:
		return "21H2 (22000)"
	case 19041:
		return "Windows 10 2004 to 22H2 (19041)"
	}
	return strconv.Itoa(build)
}

// EditionAllowed reports whether the hex edition id (as Windows reports it
// in DevDetail/Ext/Microsoft/OSPlatform-independent SKU terms, "0x30") is in
// the node's allow list. A node without an allow list allows every edition.
func EditionAllowed(n *csp.Node, edition string) bool {
	if n.Applicability == nil || len(n.Applicability.EditionAllowList) == 0 {
		return true
	}
	for _, e := range n.Applicability.EditionAllowList {
		if strings.EqualFold(e, edition) {
			return true
		}
	}
	return false
}
