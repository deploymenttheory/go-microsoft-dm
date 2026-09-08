package mdm

import (
	"context"
	"sort"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/syncml"
	"github.com/deploymenttheory/go-microsoft-dm/paging"
)

// Setting is one desired leaf value in the management tree.
type Setting struct {
	// URI is the node's LocURI.
	URI string
	// Value is the desired data.
	Value string
	// Format is the Meta/Format to send (chr, int, bool, b64, ...); empty
	// omits it.
	Format string
}

// Diff returns the commands needed to bring a device to the desired settings
// given what it has already acknowledged: one Replace per setting whose
// desired value differs from acknowledged[URI] or is absent from it, and
// nothing for a setting whose acknowledged value already matches. Commands
// come out in URI order so the result is deterministic.
//
// It is the "diff desired vs acknowledged" step that lets a server enqueue
// only what changed instead of re-sending the whole configuration every
// session (research pitfall "Aggressive polling"). A Replace built here fails
// only when its URI is malformed, in which case that setting is skipped; call
// Check against the schema before enqueueing if the desired set is untrusted.
func Diff(desired []Setting, acknowledged map[string]string) []*Command {
	sorted := make([]Setting, len(desired))
	copy(sorted, desired)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].URI < sorted[j].URI })
	var out []*Command
	for _, s := range sorted {
		if cur, ok := acknowledged[s.URI]; ok && cur == s.Value {
			continue
		}
		var opts []Option
		if s.Format != "" {
			opts = append(opts, WithFormat(s.Format))
		}
		cmd, err := NewReplace(s.URI, s.Value, opts...)
		if err != nil {
			continue
		}
		out = append(out, cmd)
	}
	return out
}

// AcknowledgedValues reads back what a device is known to hold: for every
// command the queue has acknowledged, the value of each Add or Replace item,
// and the data of each successful Get result, keyed by LocURI. A later
// acknowledgement overrides an earlier one, so the map holds the current
// value. It is the input to Diff.
func AcknowledgedValues(ctx context.Context, q CommandQueue, deviceID string) (map[string]string, error) {
	acked := []State{StateAcknowledged}
	out := map[string]string{}
	page := paging.Page{Limit: paging.MaxPageSize}
	for {
		res, err := q.List(ctx, deviceID, Query{States: acked}, page)
		if err != nil {
			return nil, err
		}
		for i := range res.Items {
			collectValues(&res.Items[i], out)
		}
		if res.NextCursor == "" {
			break
		}
		page.Cursor = res.NextCursor
	}
	return out, nil
}

// collectValues records the acknowledged values a command carried or read.
func collectValues(qc *QueuedCommand, out map[string]string) {
	for _, leaf := range Leaves(qc.Command.Body) {
		switch leaf.(type) {
		case *syncml.Add, *syncml.Replace:
			for _, it := range itemsOf(leaf) {
				if it.Target != "" && it.Data != nil {
					out[it.Target] = it.Data.Text()
				}
			}
		}
	}
	if qc.Result != nil {
		for _, it := range qc.Result.Items {
			if uri := it.LocURI(); uri != "" && it.Data != nil {
				out[uri] = it.Data.Text()
			}
		}
	}
}
