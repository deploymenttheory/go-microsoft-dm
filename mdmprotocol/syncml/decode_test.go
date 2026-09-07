package syncml_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/syncml"
)

const minimal = `<SyncML xmlns="SYNCML:SYNCML1.2"><SyncHdr><VerDTD>1.2</VerDTD><VerProto>DM/1.2</VerProto><SessionID>1</SessionID><MsgID>1</MsgID><Target><LocURI>https://s</LocURI></Target><Source><LocURI>dev</LocURI></Source></SyncHdr><SyncBody>%s</SyncBody></SyncML>`

func body(inner string) []byte { return []byte(strings.Replace(minimal, "%s", inner, 1)) }

func TestDecodeRejections(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		in   []byte
		want error
	}{
		"Copy is not a Windows command":  {body(`<Copy><CmdID>1</CmdID></Copy>`), syncml.ErrUnsupportedCommand},
		"Sync is not a Windows command":  {body(`<Sync><CmdID>1</CmdID></Sync>`), syncml.ErrUnsupportedCommand},
		"Map inside Atomic":              {body(`<Atomic><CmdID>1</CmdID><Map><CmdID>2</CmdID></Map></Atomic>`), syncml.ErrUnsupportedCommand},
		"unknown element in body":        {body(`<Foo/>`), syncml.ErrUnknownElement},
		"unknown element in header":      {[]byte(`<SyncML xmlns="SYNCML:SYNCML1.2"><SyncHdr><Bogus/></SyncHdr><SyncBody><Final/></SyncBody></SyncML>`), syncml.ErrUnknownElement},
		"unknown element under Item":     {body(`<Get><CmdID>1</CmdID><Item><Nope/></Item></Get>`), syncml.ErrUnknownElement},
		"TargetParent is not used":       {body(`<Add><CmdID>1</CmdID><Item><TargetParent><LocURI>./x</LocURI></TargetParent></Item></Add>`), syncml.ErrUnknownElement},
		"Status inside Atomic":           {body(`<Atomic><CmdID>1</CmdID><Status><CmdID>2</CmdID></Status></Atomic>`), syncml.ErrUnknownElement},
		"Lang under Add":                 {body(`<Add><CmdID>1</CmdID><Lang>en</Lang></Add>`), syncml.ErrUnknownElement},
		"Final not last":                 {body(`<Final/><Get><CmdID>1</CmdID></Get>`), syncml.ErrFinalNotLast},
		"two Finals":                     {body(`<Final/><Final/>`), syncml.ErrFinalNotLast},
		"wrong namespace":                {[]byte(`<SyncML xmlns="SYNCML:SYNCML1.0"><SyncHdr/><SyncBody/></SyncML>`), syncml.ErrNamespace},
		"root is not SyncML":             {[]byte(`<Envelope/>`), syncml.ErrSyntax},
		"not XML":                        {[]byte(`<SyncML xmlns="SYNCML:SYNCML1.2"><SyncHdr>`), syncml.ErrSyntax},
		"empty input":                    {[]byte(``), syncml.ErrSyntax},
		"text where an element must be":  {body(`<Get><CmdID>1</CmdID>stray</Get>`), syncml.ErrSyntax},
		"child element inside CmdID":     {body(`<Get><CmdID><b>1</b></CmdID></Get>`), syncml.ErrSyntax},
		"Final with content":             {body(`<Final>x</Final>`), syncml.ErrSyntax},
		"MoreData with content":          {body(`<Add><CmdID>1</CmdID><Item><MoreData>1</MoreData></Item></Add>`), syncml.ErrSyntax},
		"Size not a number":              {body(`<Add><CmdID>1</CmdID><Item><Meta><Size xmlns="syncml:metinf">big</Size></Meta></Item></Add>`), syncml.ErrSyntax},
		"MaxMsgSize negative":            {[]byte(`<SyncML xmlns="SYNCML:SYNCML1.2"><SyncHdr><Meta><MaxMsgSize xmlns="syncml:metinf">-1</MaxMsgSize></Meta></SyncHdr><SyncBody/></SyncML>`), syncml.ErrSyntax},
		"repeated SyncHdr":               {[]byte(`<SyncML xmlns="SYNCML:SYNCML1.2"><SyncHdr/><SyncHdr/><SyncBody/></SyncML>`), syncml.ErrSyntax},
		"content after root":             {append(body(`<Final/>`), []byte(`<More/>`)...), syncml.ErrSyntax},
		"unknown charset":                {[]byte(`<?xml version="1.0" encoding="koi8-r"?>` + string(body(`<Final/>`))), syncml.ErrSyntax},
		"Chal with a non-Meta child":     {body(`<Status><CmdID>1</CmdID><Chal><Data>x</Data></Chal></Status>`), syncml.ErrUnknownElement},
		"Cred with unknown child":        {body(`<Add><CmdID>1</CmdID><Cred><Nonce/></Cred></Add>`), syncml.ErrUnknownElement},
		"unknown element under Alert":    {body(`<Alert><CmdID>1</CmdID><Item/><Nope/></Alert>`), syncml.ErrUnknownElement},
		"unknown element under Exec":     {body(`<Exec><CmdID>1</CmdID><Nope/></Exec>`), syncml.ErrUnknownElement},
		"unknown element under Status":   {body(`<Status><CmdID>1</CmdID><Nope/></Status>`), syncml.ErrUnknownElement},
		"unknown element under Results":  {body(`<Results><CmdID>1</CmdID><Nope/></Results>`), syncml.ErrUnknownElement},
		"unknown element under Sequence": {body(`<Sequence><CmdID>1</CmdID><Nope/></Sequence>`), syncml.ErrUnknownElement},
		"unknown element under Target":   {body(`<Get><CmdID>1</CmdID><Item><Target><Filter/></Target></Item></Get>`), syncml.ErrUnknownElement},
		"unknown element under SyncML":   {[]byte(`<SyncML xmlns="SYNCML:SYNCML1.2"><Body/></SyncML>`), syncml.ErrUnknownElement},
	}
	for name, tc := range cases {
		_, err := syncml.Unmarshal(tc.in)
		if !errors.Is(err, tc.want) {
			t.Errorf("%s: err = %v, want %v", name, err, tc.want)
		}
	}
}

func TestDecodeSizeLimit(t *testing.T) {
	t.Parallel()
	in := body(`<Final/>`)
	if _, err := syncml.Decode(in, syncml.DecodeOptions{MaxSize: len(in) - 1}); !errors.Is(err, syncml.ErrTooLarge) {
		t.Fatalf("err = %v", err)
	}
	if _, err := syncml.Decode(in, syncml.DecodeOptions{MaxSize: len(in)}); err != nil {
		t.Fatal(err)
	}
	if _, err := syncml.DecodeCommand(bytes.Repeat([]byte("x"), syncml.DefaultMaxSize+1)); !errors.Is(err, syncml.ErrTooLarge) {
		t.Fatalf("command: err = %v", err)
	}
	if _, err := syncml.DecodeCommand([]byte("  ")); !errors.Is(err, syncml.ErrSyntax) {
		t.Fatalf("empty command: err = %v", err)
	}
	if _, err := syncml.DecodeCommand([]byte("<Copy/>")); !errors.Is(err, syncml.ErrUnsupportedCommand) {
		t.Fatalf("copy command: err = %v", err)
	}
}

// TestCDATAIsNormalised pins the Learn known issue: the client does not
// support CDATA, so the decoder normalises it and the encoder never emits it.
func TestCDATAIsNormalised(t *testing.T) {
	t.Parallel()
	m, err := syncml.Unmarshal(body(`<Replace><CmdID>1</CmdID><Item><Target><LocURI>./x</LocURI></Target><Data><![CDATA[<enabled/>&]]></Data></Item></Replace><Final/>`))
	if err != nil {
		t.Fatal(err)
	}
	d := m.Body.Commands[0].(*syncml.Replace).Items[0].Data
	if d.Value != "<enabled/>&" || d.XML != "" {
		t.Fatalf("data %+v", d)
	}
	out, err := syncml.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out, []byte(`<Data>&lt;enabled/&gt;&amp;</Data>`)) || bytes.Contains(out, []byte("CDATA")) {
		t.Fatalf("encoded %s", out)
	}
}

func TestDecodeMarkupAndAttributes(t *testing.T) {
	t.Parallel()
	m, err := syncml.Unmarshal(body(`<Status><CmdID>1</CmdID><MsgRef>1</MsgRef><CmdRef>2</CmdRef><Cmd>Replace</Cmd><Data msft:originalerror="0x86000002">500</Data></Status>` +
		`<Results><CmdID>3</CmdID><CmdRef>4</CmdRef><Item><Source><LocURI>./a</LocURI></Source><Data> <x a="1"><y/></x> </Data></Item></Results><Final/>`))
	if err != nil {
		t.Fatal(err)
	}
	st := m.Body.Statuses()[0]
	if st.Data.OriginalError != "0x86000002" || st.Code() != syncml.StatusCommandFailed {
		t.Fatalf("status %+v", st)
	}
	res := m.Body.Commands[1].(*syncml.Results)
	if res.Items[0].Data.XML != `<x a="1"><y/></x>` || res.Items[0].Data.Value != "" {
		t.Fatalf("data %+v", res.Items[0].Data)
	}
	out, err := syncml.Marshal(m)
	if err != nil || !bytes.Contains(out, []byte(`<Data msft:originalerror="0x86000002">500</Data>`)) || !bytes.Contains(out, []byte(`<Data><x a="1"><y/></x></Data>`)) {
		t.Fatalf("encoded %v\n%s", err, out)
	}
	// Body-level msft namespace and header NoResp/RespURI survive.
	m2, err := syncml.Unmarshal([]byte(`<SyncML xmlns="SYNCML:SYNCML1.2"><SyncHdr><VerDTD>1.2</VerDTD><VerProto>DM/1.2</VerProto><SessionID>1</SessionID><MsgID>1</MsgID><Target><LocURI>a</LocURI></Target><Source><LocURI>b</LocURI></Source><RespURI>https://r</RespURI><NoResp/></SyncHdr><SyncBody xmlns:msft="http://schemas.microsoft.com/MobileDevice/MDM"><Get><CmdID>1</CmdID><NoResp/><Lang>en</Lang><Item><Target><LocURI>./x</LocURI></Target></Item></Get><Exec><CmdID>2</CmdID><Correlator>c1</Correlator><Item><Target><LocURI>./y</LocURI></Target></Item></Exec></SyncBody></SyncML>`))
	if err != nil {
		t.Fatal(err)
	}
	if !m2.Body.MSFTNamespace || m2.Header.RespURI != "https://r" || !m2.Header.NoResp || !m2.Body.Commands[0].(*syncml.Get).NoResp || m2.Body.Commands[0].(*syncml.Get).Lang != "en" || m2.Body.Commands[1].(*syncml.Exec).Correlator != "c1" {
		t.Fatalf("decoded %+v", m2)
	}
	out2, err := syncml.Marshal(m2)
	if err != nil || !bytes.Contains(out2, []byte(`<SyncBody xmlns:msft="http://schemas.microsoft.com/MobileDevice/MDM">`)) || !bytes.Contains(out2, []byte(`<RespURI>https://r</RespURI><NoResp/>`)) || !bytes.Contains(out2, []byte(`<NoResp/><Lang>en</Lang>`)) || !bytes.Contains(out2, []byte(`<Correlator>c1</Correlator>`)) {
		t.Fatalf("encoded %v\n%s", err, out2)
	}
}

func TestDecodeRealUTF16(t *testing.T) {
	t.Parallel()
	src := `<?xml version="1.0" encoding="utf-16"?>` + string(body(`<Alert><CmdID>1</CmdID><Data>1201</Data></Alert><Final/>`))
	for name, enc := range map[string]func([]uint16) []byte{
		"le with bom": func(u []uint16) []byte {
			b := []byte{0xFF, 0xFE}
			for _, c := range u {
				b = append(b, byte(c), byte(c>>8))
			}
			return b
		},
		"be with bom": func(u []uint16) []byte {
			b := []byte{0xFE, 0xFF}
			for _, c := range u {
				b = append(b, byte(c>>8), byte(c))
			}
			return b
		},
		"be no bom": func(u []uint16) []byte {
			var b []byte
			for _, c := range u {
				b = append(b, byte(c>>8), byte(c))
			}
			return b
		},
	} {
		m, err := syncml.Unmarshal(enc(utf16.Encode([]rune(src))))
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if !m.Body.HasAlert(syncml.AlertClientInitiated) {
			t.Errorf("%s: alert lost", name)
		}
	}
}

func TestUnknownMetaChildrenAreSkipped(t *testing.T) {
	t.Parallel()
	m, err := syncml.Unmarshal(body(`<Add><CmdID>1</CmdID><Item><Target><LocURI>./x</LocURI></Target><Meta><Anchor xmlns="syncml:metinf"><Next>1</Next></Anchor><Format xmlns="syncml:metinf">chr</Format><Mark xmlns="syncml:metinf">critical</Mark><MaxObjSize xmlns="syncml:metinf">10</MaxObjSize></Meta></Item></Add><Final/>`))
	if err != nil {
		t.Fatal(err)
	}
	meta := m.Body.Commands[0].(*syncml.Add).Items[0].Meta
	if meta.Format != "chr" || meta.Mark != "critical" || meta.MaxObjSize != 10 {
		t.Fatalf("meta %+v", meta)
	}
}
