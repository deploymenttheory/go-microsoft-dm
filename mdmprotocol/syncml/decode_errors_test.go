package syncml_test

import (
	"errors"
	"testing"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/syncml"
)

// TestDecodeErrorsPropagateFromEveryElement puts a malformed child (an
// element where text is required) in each position the decoder reads, so
// every error branch returns the error instead of swallowing it.
func TestDecodeErrorsPropagateFromEveryElement(t *testing.T) {
	t.Parallel()
	bad := "<b/>"
	cases := map[string][]byte{
		"header VerDTD":                         []byte(`<SyncML xmlns="SYNCML:SYNCML1.2"><SyncHdr><VerDTD>` + bad + `</VerDTD></SyncHdr></SyncML>`),
		"header RespURI":                        []byte(`<SyncML xmlns="SYNCML:SYNCML1.2"><SyncHdr><RespURI>` + bad + `</RespURI></SyncHdr></SyncML>`),
		"header NoResp":                         []byte(`<SyncML xmlns="SYNCML:SYNCML1.2"><SyncHdr><NoResp>x</NoResp></SyncHdr></SyncML>`),
		"header Cred":                           []byte(`<SyncML xmlns="SYNCML:SYNCML1.2"><SyncHdr><Cred><Data>` + bad + `</Data></Cred></SyncHdr></SyncML>`),
		"header Cred Meta":                      []byte(`<SyncML xmlns="SYNCML:SYNCML1.2"><SyncHdr><Cred><Meta><Format>` + bad + `</Format></Meta></Cred></SyncHdr></SyncML>`),
		"header Meta":                           []byte(`<SyncML xmlns="SYNCML:SYNCML1.2"><SyncHdr><Meta><Type>` + bad + `</Type></Meta></SyncHdr></SyncML>`),
		"header Target":                         []byte(`<SyncML xmlns="SYNCML:SYNCML1.2"><SyncHdr><Target><LocURI>` + bad + `</LocURI></Target></SyncHdr></SyncML>`),
		"header LocName":                        []byte(`<SyncML xmlns="SYNCML:SYNCML1.2"><SyncHdr><Source><LocName>` + bad + `</LocName></Source></SyncHdr></SyncML>`),
		"header unterminated":                   []byte(`<SyncML xmlns="SYNCML:SYNCML1.2"><SyncHdr><VerDTD>1.2`),
		"body unterminated":                     body(`<Get><CmdID>1`),
		"Add CmdID":                             body(`<Add><CmdID>` + bad + `</CmdID></Add>`),
		"Add NoResp":                            body(`<Add><CmdID>1</CmdID><NoResp>x</NoResp></Add>`),
		"Add Cred":                              body(`<Add><CmdID>1</CmdID><Cred><Data>` + bad + `</Data></Cred></Add>`),
		"Add Meta":                              body(`<Add><CmdID>1</CmdID><Meta><Format>` + bad + `</Format></Meta></Add>`),
		"Add Item":                              body(`<Add><CmdID>1</CmdID><Item><Target><LocURI>` + bad + `</LocURI></Target></Item></Add>`),
		"Get Lang":                              body(`<Get><CmdID>1</CmdID><Lang>` + bad + `</Lang></Get>`),
		"Item Source":                           body(`<Get><CmdID>1</CmdID><Item><Source><LocURI>` + bad + `</LocURI></Source></Item></Get>`),
		"Item Meta":                             body(`<Get><CmdID>1</CmdID><Item><Meta><Mark>` + bad + `</Mark></Meta></Item></Get>`),
		"Item MoreData":                         body(`<Get><CmdID>1</CmdID><Item><MoreData>x</MoreData></Item></Get>`),
		"Item Data unterminated":                body(`<Get><CmdID>1</CmdID><Item><Data>x`),
		"Meta NextNonce":                        body(`<Get><CmdID>1</CmdID><Item><Meta><NextNonce>` + bad + `</NextNonce></Meta></Item></Get>`),
		"Meta MaxObjSize":                       body(`<Get><CmdID>1</CmdID><Item><Meta><MaxObjSize>x</MaxObjSize></Meta></Item></Get>`),
		"Meta skipped child unterminated":       body(`<Get><CmdID>1</CmdID><Item><Meta><Anchor><Next>1`),
		"Exec CmdID":                            body(`<Exec><CmdID>` + bad + `</CmdID></Exec>`),
		"Exec NoResp":                           body(`<Exec><CmdID>1</CmdID><NoResp>x</NoResp></Exec>`),
		"Exec Cred":                             body(`<Exec><CmdID>1</CmdID><Cred><Data>` + bad + `</Data></Cred></Exec>`),
		"Exec Meta":                             body(`<Exec><CmdID>1</CmdID><Meta><Format>` + bad + `</Format></Meta></Exec>`),
		"Exec Correlator":                       body(`<Exec><CmdID>1</CmdID><Correlator>` + bad + `</Correlator></Exec>`),
		"Exec Item":                             body(`<Exec><CmdID>1</CmdID><Item><Nope/></Item></Exec>`),
		"Alert CmdID":                           body(`<Alert><CmdID>` + bad + `</CmdID></Alert>`),
		"Alert NoResp":                          body(`<Alert><CmdID>1</CmdID><NoResp>x</NoResp></Alert>`),
		"Alert Cred":                            body(`<Alert><CmdID>1</CmdID><Cred><Data>` + bad + `</Data></Cred></Alert>`),
		"Alert Data":                            body(`<Alert><CmdID>1</CmdID><Data>` + bad + `</Data></Alert>`),
		"Alert Correlator":                      body(`<Alert><CmdID>1</CmdID><Correlator>` + bad + `</Correlator></Alert>`),
		"Alert Item":                            body(`<Alert><CmdID>1</CmdID><Item><Nope/></Item></Alert>`),
		"Atomic CmdID":                          body(`<Atomic><CmdID>` + bad + `</CmdID></Atomic>`),
		"Atomic NoResp":                         body(`<Atomic><CmdID>1</CmdID><NoResp>x</NoResp></Atomic>`),
		"Atomic Meta":                           body(`<Atomic><CmdID>1</CmdID><Meta><Format>` + bad + `</Format></Meta></Atomic>`),
		"Atomic child":                          body(`<Atomic><CmdID>1</CmdID><Add><CmdID>` + bad + `</CmdID></Add></Atomic>`),
		"Sequence child":                        body(`<Sequence><CmdID>1</CmdID><Alert><CmdID>` + bad + `</CmdID></Alert></Sequence>`),
		"Status CmdID":                          body(`<Status><CmdID>` + bad + `</CmdID></Status>`),
		"Status MsgRef":                         body(`<Status><CmdID>1</CmdID><MsgRef>` + bad + `</MsgRef></Status>`),
		"Status CmdRef":                         body(`<Status><CmdID>1</CmdID><CmdRef>` + bad + `</CmdRef></Status>`),
		"Status Cmd":                            body(`<Status><CmdID>1</CmdID><Cmd>` + bad + `</Cmd></Status>`),
		"Status TargetRef":                      body(`<Status><CmdID>1</CmdID><TargetRef>` + bad + `</TargetRef></Status>`),
		"Status SourceRef":                      body(`<Status><CmdID>1</CmdID><SourceRef>` + bad + `</SourceRef></Status>`),
		"Status Cred":                           body(`<Status><CmdID>1</CmdID><Cred><Data>` + bad + `</Data></Cred></Status>`),
		"Status Chal":                           body(`<Status><CmdID>1</CmdID><Chal><Meta><NextNonce>` + bad + `</NextNonce></Meta></Chal></Status>`),
		"Status Chal unterminated":              body(`<Status><CmdID>1</CmdID><Chal><Meta>`),
		"Status Data":                           body(`<Status><CmdID>1</CmdID><Data>x`),
		"Status Item":                           body(`<Status><CmdID>1</CmdID><Item><Nope/></Item></Status>`),
		"Results CmdID":                         body(`<Results><CmdID>` + bad + `</CmdID></Results>`),
		"Results MsgRef":                        body(`<Results><CmdID>1</CmdID><MsgRef>` + bad + `</MsgRef></Results>`),
		"Results CmdRef":                        body(`<Results><CmdID>1</CmdID><CmdRef>` + bad + `</CmdRef></Results>`),
		"Results Cmd":                           body(`<Results><CmdID>1</CmdID><Cmd>` + bad + `</Cmd></Results>`),
		"Results Meta":                          body(`<Results><CmdID>1</CmdID><Meta><Format>` + bad + `</Format></Meta></Results>`),
		"Results TargetRef":                     body(`<Results><CmdID>1</CmdID><TargetRef>` + bad + `</TargetRef></Results>`),
		"Results SourceRef":                     body(`<Results><CmdID>1</CmdID><SourceRef>` + bad + `</SourceRef></Results>`),
		"Results Item":                          body(`<Results><CmdID>1</CmdID><Item><Nope/></Item></Results>`),
		"Cred Meta unterminated":                body(`<Add><CmdID>1</CmdID><Cred><Meta>`),
		"Cred unterminated":                     body(`<Add><CmdID>1</CmdID><Cred>`),
		"Alert unterminated":                    body(`<Alert><CmdID>1</CmdID>`),
		"Exec unterminated":                     body(`<Exec><CmdID>1</CmdID>`),
		"Status unterminated":                   body(`<Status><CmdID>1</CmdID>`),
		"Results unterminated":                  body(`<Results><CmdID>1</CmdID>`),
		"Atomic unterminated":                   body(`<Atomic><CmdID>1</CmdID>`),
		"Item unterminated":                     body(`<Get><CmdID>1</CmdID><Item>`),
		"Meta unterminated":                     body(`<Get><CmdID>1</CmdID><Item><Meta>`),
		"Target unterminated":                   body(`<Get><CmdID>1</CmdID><Item><Target>`),
		"skipEmpty unterminated":                body(`<Final>`),
		"text where element expected in header": []byte(`<SyncML xmlns="SYNCML:SYNCML1.2"><SyncHdr>stray</SyncHdr></SyncML>`),
	}
	for name, in := range cases {
		_, err := syncml.Unmarshal(in)
		if err == nil {
			t.Errorf("%s: accepted", name)
			continue
		}
		if !errors.Is(err, syncml.ErrSyntax) && !errors.Is(err, syncml.ErrUnknownElement) {
			t.Errorf("%s: err = %v, want a syntax or unknown-element error", name, err)
		}
	}
}
