package syncml_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/syncml"
)

// FuzzDecode requires that anything Decode accepts survives an encode and
// decode unchanged, and that nothing makes it panic.
func FuzzDecode(f *testing.F) {
	entries, err := os.ReadDir("testdata")
	if err != nil {
		f.Fatal(err)
	}
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".xml" {
			b, err := os.ReadFile(filepath.Join("testdata", e.Name()))
			if err != nil {
				f.Fatal(err)
			}
			f.Add(b)
		}
	}
	f.Add([]byte(`<SyncML xmlns="SYNCML:SYNCML1.2"><SyncHdr/><SyncBody><Get><CmdID>1</CmdID><Item><Target><LocURI>./x</LocURI></Target></Item></Get><Final/></SyncBody></SyncML>`))
	f.Fuzz(func(t *testing.T, data []byte) {
		m, err := syncml.Decode(data, syncml.DecodeOptions{MaxSize: 1 << 16})
		if err != nil {
			return
		}
		_ = syncml.Validate(m)
		if m.Namespace == "" {
			m.Namespace = syncml.NamespaceSyncML12
		}
		out, err := syncml.Marshal(m)
		if err != nil {
			t.Fatalf("encode after decode: %v", err)
		}
		again, err := syncml.Unmarshal(out)
		if err != nil {
			t.Fatalf("decode after encode: %v\n%s", err, out)
		}
		if !reflect.DeepEqual(m, again) {
			t.Fatalf("round trip changed the message\n%s", out)
		}
	})
}
