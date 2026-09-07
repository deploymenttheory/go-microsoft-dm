package syncml_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/syncml"
)

func TestSplitItemAndAssemble(t *testing.T) {
	t.Parallel()
	value := strings.Repeat("abcdé", 100) // 600 bytes, multibyte at every fifth
	it := syncml.Item{Target: "./x", Meta: &syncml.Meta{Format: "chr"}, Data: &syncml.Data{Value: value}}
	chunks, err := syncml.SplitItem(it, 128)
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) < 5 || chunks[0].Meta.Size != 600 || !chunks[0].MoreData || chunks[len(chunks)-1].MoreData {
		t.Fatalf("chunks %d first %+v", len(chunks), chunks[0])
	}
	var total int
	for i, c := range chunks {
		if len(c.Data.Value) > 128 || c.Target != "./x" || c.Meta.Format != "chr" {
			t.Fatalf("chunk %d: %+v", i, c)
		}
		if !isValidUTF8(c.Data.Value) {
			t.Fatalf("chunk %d splits a rune", i)
		}
		total += len(c.Data.Value)
	}
	if total != 600 {
		t.Fatalf("total %d", total)
	}
	var a syncml.Assembler
	for i, c := range chunks {
		out, done, err := a.Add(c)
		if err != nil {
			t.Fatal(err)
		}
		if i < len(chunks)-1 {
			if done || !a.Pending() || a.LocURI() != "./x" {
				t.Fatalf("chunk %d: done early", i)
			}
			continue
		}
		if !done || out.Data.Value != value || out.MoreData || out.Meta.Size != 600 || a.Pending() || a.LocURI() != "" {
			t.Fatalf("final: done %v %+v", done, out)
		}
	}
	// Unchunked items pass straight through; items that fit are not split.
	if out, done, err := a.Add(syncml.Item{Target: "./y", Data: &syncml.Data{Value: "v"}}); err != nil || !done || out.Data.Value != "v" {
		t.Fatal("pass-through")
	}
	if one, err := syncml.SplitItem(it, 600); err != nil || len(one) != 1 || one[0].MoreData {
		t.Fatal("fits must not split")
	}
	if one, err := syncml.SplitItem(syncml.Item{Target: "./x", Data: &syncml.Data{XML: strings.Repeat("<a/>", 100)}}, 10); err != nil || len(one) != 1 {
		t.Fatal("markup is never split")
	}
	if one, err := syncml.SplitItem(syncml.Item{Target: "./x"}, 10); err != nil || len(one) != 1 {
		t.Fatal("no data is not split")
	}
	if noMeta, err := syncml.SplitItem(syncml.Item{Target: "./x", Data: &syncml.Data{Value: "0123456789"}}, 4); err != nil || len(noMeta) != 3 || noMeta[0].Meta.Size != 10 || noMeta[1].Meta != nil {
		t.Fatalf("no meta: %v %+v", err, noMeta)
	}
}

func isValidUTF8(s string) bool {
	for _, r := range s {
		if r == '�' {
			return false
		}
	}
	return true
}

func TestSplitItemErrors(t *testing.T) {
	t.Parallel()
	it := syncml.Item{Target: "./x", Data: &syncml.Data{Value: "ééé"}}
	if _, err := syncml.SplitItem(it, 0); !errors.Is(err, syncml.ErrChunk) {
		t.Fatal("size 0")
	}
	if _, err := syncml.SplitItem(it, 1); !errors.Is(err, syncml.ErrChunk) {
		t.Fatal("size smaller than a rune")
	}
	it.MoreData = true
	if _, err := syncml.SplitItem(it, 2); !errors.Is(err, syncml.ErrChunk) {
		t.Fatal("already chunked")
	}
}

func TestAssemblerErrors(t *testing.T) {
	t.Parallel()
	first := syncml.Item{Target: "./x", Meta: &syncml.Meta{Size: 6}, Data: &syncml.Data{Value: "abc"}, MoreData: true}

	var a syncml.Assembler
	if _, _, err := a.Add(syncml.Item{Target: "./x", MoreData: true}); !errors.Is(err, syncml.ErrChunk) {
		t.Fatal("first chunk without Data")
	}
	if _, _, err := a.Add(first); err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.Add(syncml.Item{Target: "./other", Data: &syncml.Data{Value: "z"}}); !errors.Is(err, syncml.ErrChunk) || a.Pending() {
		t.Fatal("interrupting item must fail and reset")
	}

	a = syncml.Assembler{}
	if _, _, err := a.Add(first); err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.Add(syncml.Item{Target: "./x", Data: &syncml.Data{Value: "de"}}); !errors.Is(err, syncml.ErrChunk) {
		t.Fatal("size mismatch must fail")
	}

	a = syncml.Assembler{MaxSize: 4}
	if _, _, err := a.Add(first); !errors.Is(err, syncml.ErrChunk) {
		t.Fatal("declared size over limit")
	}
	a = syncml.Assembler{MaxSize: 4}
	if _, _, err := a.Add(syncml.Item{Target: "./x", Data: &syncml.Data{Value: "abc"}, MoreData: true}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.Add(syncml.Item{Target: "./x", Data: &syncml.Data{Value: "def"}, MoreData: true}); !errors.Is(err, syncml.ErrChunk) {
		t.Fatal("accumulated size over limit")
	}

	a = syncml.Assembler{}
	if _, _, err := a.Add(syncml.Item{Target: "./x", Data: &syncml.Data{Value: "a"}, MoreData: true}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.Add(syncml.Item{Target: "./x", Data: &syncml.Data{XML: "<b/>"}}); !errors.Is(err, syncml.ErrChunk) {
		t.Fatal("markup chunk must fail")
	}

	a = syncml.Assembler{}
	if _, _, err := a.Add(syncml.Item{Target: "./x", Data: &syncml.Data{Value: "a"}, MoreData: true}); err != nil {
		t.Fatal(err)
	}
	a.Reset()
	if a.Pending() {
		t.Fatal("Reset")
	}
	// A chunk without Data in the middle contributes nothing but is accepted.
	a = syncml.Assembler{}
	a.Add(syncml.Item{Target: "./x", Data: &syncml.Data{Value: "ab"}, MoreData: true}) //nolint:errcheck
	a.Add(syncml.Item{Target: "./x", MoreData: true})                                  //nolint:errcheck
	out, done, err := a.Add(syncml.Item{Target: "./x", Data: &syncml.Data{Value: "c"}})
	if err != nil || !done || out.Data.Value != "abc" {
		t.Fatalf("%v %v %+v", err, done, out)
	}
}
