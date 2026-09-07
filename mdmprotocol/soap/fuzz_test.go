package soap

import "testing"

// FuzzDecode feeds arbitrary bytes to the envelope and fault decoders; they
// must return an error or a value, never panic.
func FuzzDecode(f *testing.F) {
	f.Add([]byte(onPremRequest))
	fault, _ := EncodeFault(NewFault(SubcodeAuthorization, "x").WithDetail(ErrorDeviceCapReached, "t"), "a", "r")
	f.Add(fault)
	f.Add([]byte(`<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope"><s:Body/></s:Envelope>`))
	f.Fuzz(func(t *testing.T, data []byte) {
		env, err := Decode[probeBody](data, 0)
		if err == nil && env == nil {
			t.Fatal("nil envelope without error")
		}
		d, err := DecodeFault(data)
		if err != nil && d != nil {
			t.Fatal("fault with error")
		}
	})
}
