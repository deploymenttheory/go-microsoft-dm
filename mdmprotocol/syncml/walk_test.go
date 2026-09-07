package syncml_test

import (
	"testing"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/syncml"
)

func TestWalkStopsEarly(t *testing.T) {
	t.Parallel()
	b := syncml.Body{Commands: []syncml.Command{
		&syncml.Atomic{CmdID: "1", Commands: []syncml.Command{&syncml.Add{CmdID: "2"}, &syncml.Replace{CmdID: "3"}}},
		&syncml.Sequence{CmdID: "4", Commands: []syncml.Command{&syncml.Exec{CmdID: "5"}, &syncml.Exec{CmdID: "6"}}},
		&syncml.Get{CmdID: "7"},
	}}
	for stopAt, want := range map[string]int{"2": 2, "3": 3, "4": 4, "5": 5, "7": 7, "never": 7} {
		n := 0
		b.Walk(func(c syncml.Command, _ int) bool {
			n++
			return c.ID() != stopAt
		})
		if n != want {
			t.Errorf("stop at %s: visited %d, want %d", stopAt, n, want)
		}
	}
}
