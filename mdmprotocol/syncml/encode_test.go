package syncml

import (
	"errors"
	"strings"
	"testing"
)

type bogusCommand struct{ cmd }

func (bogusCommand) Name() string { return "Bogus" }
func (bogusCommand) ID() string   { return "1" }

func TestSealedMarkerAndErrors(t *testing.T) {
	t.Parallel()
	for _, c := range []Command{&Add{}, &Alert{}, &Atomic{}, &Delete{}, &Exec{}, &Get{}, &Replace{}, &Results{}, &Sequence{}, &Status{}} {
		if c.command() != (sealed{}) || c.Name() == "" {
			t.Fatalf("%T", c)
		}
	}
	se := &SyntaxError{Element: "X", Msg: "bad"}
	if se.Error() != "syncml: X: bad" || !errors.Is(se, ErrSyntax) {
		t.Fatal(se)
	}
	wrapped := &SyntaxError{Element: "X", Msg: "bad", Err: ErrNamespace}
	if wrapped.Error() != "syncml: X: bad: syncml: unsupported namespace" || !errors.Is(wrapped, ErrNamespace) || !errors.Is(wrapped, ErrSyntax) {
		t.Fatal(wrapped)
	}
}

func TestEncodeCanonicalShape(t *testing.T) {
	t.Parallel()
	m := &Message{
		Header: Header{VerDTD: VerDTD, VerProto: VerProto, SessionID: "7", MsgID: "2",
			Target: Location{LocURI: "dev"}, Source: Location{LocURI: "https://s", LocName: "srv"}},
		Body: Body{Commands: []Command{
			&Status{CmdID: "1", MsgRef: "2", CmdRef: "0", Cmd: CmdSyncHdr, Data: Data{Value: "212"}, Chal: NewMD5Chal([]byte("nonce"))},
			&Replace{CmdID: "2", Items: []Item{{Target: "./Device/Vendor/MSFT/Policy/Config/A/B", Meta: &Meta{Format: FormatInt, Type: "text/plain"}, Data: &Data{Value: "1 < 2 & \"x\""}}}},
			&Atomic{CmdID: "3", Commands: []Command{&Add{CmdID: "4", Items: []Item{{Target: "./x", Meta: &Meta{Format: FormatNode}}}}}},
			&Delete{CmdID: "5", Items: []Item{{Target: "./y"}}},
			&Sequence{CmdID: "6", Commands: []Command{&Exec{CmdID: "7", Items: []Item{{Target: "./z", Data: &Data{Value: "go"}}}}}},
		}, Final: true},
	}
	out, err := Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	want := `<SyncML xmlns="SYNCML:SYNCML1.2"><SyncHdr><VerDTD>1.2</VerDTD><VerProto>DM/1.2</VerProto><SessionID>7</SessionID><MsgID>2</MsgID><Target><LocURI>dev</LocURI></Target><Source><LocURI>https://s</LocURI><LocName>srv</LocName></Source></SyncHdr>` +
		`<SyncBody><Status><CmdID>1</CmdID><MsgRef>2</MsgRef><CmdRef>0</CmdRef><Cmd>SyncHdr</Cmd><Chal><Meta><Format xmlns="syncml:metinf">b64</Format><Type xmlns="syncml:metinf">syncml:auth-md5</Type><NextNonce xmlns="syncml:metinf">bm9uY2U=</NextNonce></Meta></Chal><Data>212</Data></Status>` +
		`<Replace><CmdID>2</CmdID><Item><Target><LocURI>./Device/Vendor/MSFT/Policy/Config/A/B</LocURI></Target><Meta><Format xmlns="syncml:metinf">int</Format><Type xmlns="syncml:metinf">text/plain</Type></Meta><Data>1 &lt; 2 &amp; &#34;x&#34;</Data></Item></Replace>` +
		`<Atomic><CmdID>3</CmdID><Add><CmdID>4</CmdID><Item><Target><LocURI>./x</LocURI></Target><Meta><Format xmlns="syncml:metinf">node</Format></Meta></Item></Add></Atomic>` +
		`<Delete><CmdID>5</CmdID><Item><Target><LocURI>./y</LocURI></Target></Item></Delete>` +
		`<Sequence><CmdID>6</CmdID><Exec><CmdID>7</CmdID><Item><Target><LocURI>./z</LocURI></Target><Data>go</Data></Item></Exec></Sequence>` +
		`<Final/></SyncBody></SyncML>`
	if string(out) != want {
		t.Fatalf("got\n%s\nwant\n%s", out, want)
	}
	pretty, err := Encode(m, EncodeOptions{Indent: "  ", XMLDeclaration: true})
	if err != nil || !strings.HasPrefix(string(pretty), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<SyncML xmlns=\"SYNCML:SYNCML1.2\">\n  <SyncHdr>\n    <VerDTD>1.2</VerDTD>") || !strings.HasSuffix(string(pretty), "    <Final/>\n  </SyncBody>\n</SyncML>\n") {
		t.Fatalf("pretty: %v\n%s", err, pretty)
	}
}

func TestEncodeFailures(t *testing.T) {
	t.Parallel()
	conflict := &Message{Body: Body{Commands: []Command{&Add{CmdID: "1", Items: []Item{{Target: "./x", Data: &Data{Value: "a", XML: "<b/>"}}}}}}}
	if _, err := Marshal(conflict); !errors.Is(err, ErrDataConflict) {
		t.Fatalf("conflict: %v", err)
	}
	if _, err := EncodeCommand(&Status{CmdID: "1", Data: Data{Value: "200", XML: "<x/>"}}, EncodeOptions{}); !errors.Is(err, ErrDataConflict) {
		t.Fatalf("status conflict: %v", err)
	}
	if _, err := Marshal(&Message{Body: Body{Commands: []Command{bogusCommand{}}}}); !errors.Is(err, ErrUnknownElement) {
		t.Fatalf("bogus: %v", err)
	}
	if _, err := Marshal(&Message{Body: Body{Commands: []Command{&Atomic{CmdID: "1", Commands: []Command{bogusCommand{}}}}}}); !errors.Is(err, ErrUnknownElement) {
		t.Fatalf("bogus in group: %v", err)
	}
	for _, c := range []Command{
		&Exec{CmdID: "1", Items: []Item{{Data: &Data{Value: "a", XML: "<b/>"}}}},
		&Alert{CmdID: "1", Items: []Item{{Data: &Data{Value: "a", XML: "<b/>"}}}},
		&Results{CmdID: "1", Items: []Item{{Data: &Data{Value: "a", XML: "<b/>"}}}},
		&Status{CmdID: "1", Items: []Item{{Data: &Data{Value: "a", XML: "<b/>"}}}},
		&Get{CmdID: "1", Items: []Item{{Data: &Data{Value: "a", XML: "<b/>"}}}},
	} {
		if _, err := EncodeCommand(c, EncodeOptions{}); !errors.Is(err, ErrDataConflict) {
			t.Errorf("%s: %v", c.Name(), err)
		}
	}
}

func TestEncodeMetaOrderAndCred(t *testing.T) {
	t.Parallel()
	m := &Message{Header: Header{Cred: NewBasicCred("u", "p"), Meta: &Meta{MaxMsgSize: 5000, MaxObjSize: 100000, Size: 3, NextNonce: "x", Mark: MarkFatal}}, Body: Body{Commands: []Command{&Alert{CmdID: "1", Data: "1226", Correlator: "abc", Items: []Item{{Source: "./s", Meta: &Meta{Type: "Reversed-Domain-Name:x", Format: FormatXML, Mark: MarkCritical}, Data: &Data{XML: "<a/>"}, MoreData: true}}}}}}
	out, err := Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	for _, want := range []string{
		`<Cred><Meta><Format xmlns="syncml:metinf">b64</Format><Type xmlns="syncml:metinf">syncml:auth-basic</Type></Meta><Data>dTpw</Data></Cred>`,
		`<Meta><Mark xmlns="syncml:metinf">fatal</Mark><Size xmlns="syncml:metinf">3</Size><NextNonce xmlns="syncml:metinf">x</NextNonce><MaxMsgSize xmlns="syncml:metinf">5000</MaxMsgSize><MaxObjSize xmlns="syncml:metinf">100000</MaxObjSize></Meta></SyncHdr>`,
		`<Alert><CmdID>1</CmdID><Data>1226</Data><Correlator>abc</Correlator><Item><Source><LocURI>./s</LocURI></Source><Meta><Format xmlns="syncml:metinf">xml</Format><Type xmlns="syncml:metinf">Reversed-Domain-Name:x</Type><Mark xmlns="syncml:metinf">critical</Mark></Meta><Data><a/></Data><MoreData/></Item></Alert>`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %s in\n%s", want, s)
		}
	}
}
