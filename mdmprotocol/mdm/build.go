package mdm

import (
	"errors"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/syncml"
)

// assignID sets the top-level CmdID of a command body.
func assignID(c syncml.Command, id string) {
	switch b := c.(type) {
	case *syncml.Add:
		b.CmdID = id
	case *syncml.Replace:
		b.CmdID = id
	case *syncml.Delete:
		b.CmdID = id
	case *syncml.Get:
		b.CmdID = id
	case *syncml.Exec:
		b.CmdID = id
	case *syncml.Atomic:
		b.CmdID = id
	case *syncml.Sequence:
		b.CmdID = id
	}
}

// mapChildren assigns CmdIDs to the members of an Atomic or Sequence and
// maps each back to the parent command so their statuses can be filed.
func mapChildren(sess *Session, msgID string, c syncml.Command, parentID string) {
	var members []syncml.Command
	switch b := c.(type) {
	case *syncml.Atomic:
		members = b.Commands
	case *syncml.Sequence:
		members = b.Commands
	default:
		return
	}
	next := len(sess.Sent) + len(sess.Children) + 1000
	for _, m := range members {
		next++
		cid := itoa(next)
		assignID(m, cid)
		sess.Children[msgID+"/"+cid] = parentID
	}
}

// setItems replaces the items of a leaf command.
func setItems(c syncml.Command, items []syncml.Item) {
	switch b := c.(type) {
	case *syncml.Add:
		b.Items = items
	case *syncml.Replace:
		b.Items = items
	case *syncml.Delete:
		b.Items = items
	case *syncml.Get:
		b.Items = items
	case *syncml.Exec:
		b.Items = items
	}
}

// cloneCommand deep-copies a command body so the queue's stored command is
// never mutated when the engine assigns CmdIDs or chunks items.
func cloneCommand(c syncml.Command) syncml.Command {
	switch b := c.(type) {
	case *syncml.Add:
		n := *b
		n.Items = cloneItems(b.Items)
		return &n
	case *syncml.Replace:
		n := *b
		n.Items = cloneItems(b.Items)
		return &n
	case *syncml.Delete:
		n := *b
		n.Items = cloneItems(b.Items)
		return &n
	case *syncml.Get:
		n := *b
		n.Items = cloneItems(b.Items)
		return &n
	case *syncml.Exec:
		n := *b
		n.Items = cloneItems(b.Items)
		return &n
	case *syncml.Atomic:
		n := *b
		n.Commands = cloneCommands(b.Commands)
		return &n
	case *syncml.Sequence:
		n := *b
		n.Commands = cloneCommands(b.Commands)
		return &n
	}
	return c
}

func cloneCommands(cs []syncml.Command) []syncml.Command {
	out := make([]syncml.Command, len(cs))
	for i, c := range cs {
		out[i] = cloneCommand(c)
	}
	return out
}

func cloneItems(items []syncml.Item) []syncml.Item {
	out := make([]syncml.Item, len(items))
	for i, it := range items {
		out[i] = it
		if it.Meta != nil {
			m := *it.Meta
			out[i].Meta = &m
		}
		if it.Data != nil {
			d := *it.Data
			out[i].Data = &d
		}
	}
	return out
}

// commandSize is the encoded byte length of a command, used to bound a
// message.
func commandSize(c syncml.Command) int {
	b, err := syncml.EncodeCommand(c, syncml.EncodeOptions{})
	if err != nil {
		return 0
	}
	return len(b)
}

func isConflict(err error) bool { return errors.Is(err, ErrConflict) }

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
