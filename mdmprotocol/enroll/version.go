package enroll

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ErrVersion reports a RequestVersion or EnrollmentVersion outside the
// protocol's range.
var ErrVersion = errors.New("enroll: unsupported version")

// ParseVersion reads a "N.0" protocol version and returns N. MS-MDE2 writes
// every version as a decimal with a zero fraction; anything else is refused.
func ParseVersion(s string) (int, error) {
	s = strings.TrimSpace(s)
	major, frac, ok := strings.Cut(s, ".")
	if !ok || frac != "0" {
		return 0, fmt.Errorf("%w: %q", ErrVersion, s)
	}
	n, err := strconv.Atoi(major)
	if err != nil || n < MinRequestVersion || n > MaxRequestVersion {
		return 0, fmt.Errorf("%w: %q", ErrVersion, s)
	}
	return n, nil
}

// FormatVersion writes N as "N.0".
func FormatVersion(n int) string { return strconv.Itoa(n) + ".0" }
