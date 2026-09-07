package wapprov

import "testing"

// FuzzDecode feeds arbitrary bytes to the provisioning document decoder and
// re-encodes what it accepts.
func FuzzDecode(f *testing.F) {
	f.Add([]byte(`<wap-provisioningdoc version="1.1"><characteristic type="APPLICATION"><parm name="APPID" value="w7"/><parm name="BACKCOMPATRETRYDISABLED"/></characteristic></wap-provisioningdoc>`))
	f.Add([]byte(`<wap-provisioningdoc version="1.1"><characteristic type="DMClient"><characteristic type="Provider"><characteristic type="P"><parm name="UPN" value="u" datatype="string"/></characteristic></characteristic></characteristic></wap-provisioningdoc>`))
	f.Fuzz(func(t *testing.T, data []byte) {
		doc, err := Decode(data)
		if err != nil {
			return
		}
		out, err := Encode(doc, Options{})
		if err != nil {
			t.Fatalf("accepted document does not encode: %v", err)
		}
		again, err := Decode(out)
		if err != nil {
			t.Fatalf("encoded document does not decode: %v", err)
		}
		if len(again.Characteristics) != len(doc.Characteristics) {
			t.Fatal("round trip changed the document")
		}
	})
}
