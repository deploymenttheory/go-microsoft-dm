package simulator

import (
	"crypto/rand"
	"fmt"
	"strconv"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/wapprov"
)

// randReader is the entropy source for generated keys.
var randReader = rand.Reader

// pollField sets one DMClient Poll parm on the schedule.
func setPoll(p *wapprov.Poll, parm wapprov.Parm) error {
	if parm.Name == "PollOnLogin" || parm.Name == "AllUsersPollOnFirstLogin" {
		v, err := strconv.ParseBool(parm.Value)
		if err != nil {
			return fmt.Errorf("%w: Poll/%s: %w", ErrProvisioning, parm.Name, err)
		}
		if parm.Name == "PollOnLogin" {
			p.PollOnLogin = v
		} else {
			p.AllUsersPollOnFirstLogin = v
		}
		return nil
	}
	n, err := strconv.Atoi(parm.Value)
	if err != nil {
		return fmt.Errorf("%w: Poll/%s: %w", ErrProvisioning, parm.Name, err)
	}
	switch parm.Name {
	case "IntervalForFirstSetOfRetries":
		p.IntervalForFirstSetOfRetries = n
	case "NumberOfFirstRetries":
		p.NumberOfFirstRetries = n
	case "IntervalForSecondSetOfRetries":
		p.IntervalForSecondSetOfRetries = n
	case "NumberOfSecondRetries":
		p.NumberOfSecondRetries = n
	case "IntervalForRemainingScheduledRetries":
		p.IntervalForRemainingScheduledRetries = n
	case "NumberOfRemainingScheduledRetries":
		p.NumberOfRemainingScheduledRetries = n
	default:
		return fmt.Errorf("%w: unknown Poll parm %q", ErrProvisioning, parm.Name)
	}
	return nil
}
