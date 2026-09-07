package syncml

import (
	"fmt"
	"unicode/utf8"
)

// SplitItem chunks an Item whose Data exceeds maxDataBytes into Items that
// each carry at most that many bytes of Data, with MoreData on every chunk
// but the last (OMA DM Protocol 1.2.1 section 7). Chunks break on rune
// boundaries so each one is valid XML text. Meta is repeated on every chunk
// with Size set to the full length on the first, as the protocol recommends.
// An Item that fits is returned unchanged; maxDataBytes must be positive.
func SplitItem(it Item, maxDataBytes int) ([]Item, error) {
	if maxDataBytes < 1 {
		return nil, fmt.Errorf("%w: chunk size %d", ErrChunk, maxDataBytes)
	}
	if it.Data == nil || it.Data.XML != "" || len(it.Data.Value) <= maxDataBytes {
		return []Item{it}, nil
	}
	if it.MoreData {
		return nil, fmt.Errorf("%w: item already carries MoreData", ErrChunk)
	}
	full := it.Data.Value
	var out []Item
	for len(full) > 0 {
		n := maxDataBytes
		if n >= len(full) {
			n = len(full)
		} else {
			for n > 0 && !utf8.RuneStart(full[n]) {
				n--
			}
			if n == 0 {
				return nil, fmt.Errorf("%w: chunk size %d is smaller than one character", ErrChunk, maxDataBytes)
			}
		}
		chunk := it
		chunk.Data = &Data{Value: full[:n], OriginalError: it.Data.OriginalError}
		if it.Meta != nil {
			m := *it.Meta
			chunk.Meta = &m
		}
		if len(out) == 0 {
			if chunk.Meta == nil {
				chunk.Meta = &Meta{}
			}
			chunk.Meta.Size = int64(len(it.Data.Value))
		}
		full = full[n:]
		chunk.MoreData = len(full) > 0
		out = append(out, chunk)
	}
	return out, nil
}

// Assembler reassembles a large object from the chunks a client sends
// across messages. Feed every Item of every incoming command through Add;
// it returns the complete Item once the chunk without MoreData arrives.
// A different LocURI arriving mid-object, or an object over MaxSize, is an
// ErrChunk. The session engine answers 213 while Pending and 1225 when a
// session ends with an object still Pending.
type Assembler struct {
	// MaxSize bounds the reassembled object; zero means DefaultMaxSize.
	MaxSize int64

	locURI  string
	pending bool
	buf     []byte
	first   Item
	size    int64
}

// Pending reports whether an object is mid-transfer.
func (a *Assembler) Pending() bool { return a.pending }

// LocURI returns the address of the object in transfer, or "".
func (a *Assembler) LocURI() string {
	if !a.pending {
		return ""
	}
	return a.locURI
}

// Reset discards a pending object (for Alert 1225 handling).
func (a *Assembler) Reset() { *a = Assembler{MaxSize: a.MaxSize} }

// Add consumes one Item. done is true when it returns a complete Item: the
// Item itself when it is not chunked, or the reassembled object.
func (a *Assembler) Add(it Item) (out Item, done bool, err error) {
	limit := a.MaxSize
	if limit <= 0 {
		limit = DefaultMaxSize
	}
	uri := it.LocURI()
	if !a.pending {
		if !it.MoreData {
			return it, true, nil
		}
		if it.Data == nil {
			return Item{}, false, fmt.Errorf("%w: first chunk of %q has no Data", ErrChunk, uri)
		}
		a.pending, a.locURI, a.first = true, uri, it
		a.buf = a.buf[:0]
		if it.Meta != nil {
			a.size = it.Meta.Size
			if a.size > limit {
				a.Reset()
				return Item{}, false, fmt.Errorf("%w: declared size %d exceeds limit %d", ErrChunk, a.size, limit)
			}
		}
	} else if uri != a.locURI {
		a.Reset()
		return Item{}, false, fmt.Errorf("%w: chunk for %q arrived while %q was pending", ErrChunk, uri, a.locURI)
	}
	if it.Data != nil {
		if it.Data.XML != "" {
			a.Reset()
			return Item{}, false, fmt.Errorf("%w: markup cannot be chunked", ErrChunk)
		}
		a.buf = append(a.buf, it.Data.Value...)
	}
	if int64(len(a.buf)) > limit {
		a.Reset()
		return Item{}, false, fmt.Errorf("%w: object exceeds limit %d", ErrChunk, limit)
	}
	if it.MoreData {
		return Item{}, false, nil
	}
	if a.size > 0 && int64(len(a.buf)) != a.size {
		got := len(a.buf)
		a.Reset()
		return Item{}, false, fmt.Errorf("%w: declared size %d, received %d", ErrChunk, a.size, got)
	}
	out = a.first
	out.MoreData = false
	out.Data = &Data{Value: string(a.buf), OriginalError: a.first.Data.OriginalError}
	if a.first.Meta != nil {
		m := *a.first.Meta
		out.Meta = &m
	}
	a.Reset()
	return out, true, nil
}
