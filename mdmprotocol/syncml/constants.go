package syncml

import "strconv"

// Namespaces and version strings. MS-MDM 2.2.4.1 requires the 1.2 namespace;
// older Learn samples and every Windows declared configuration sample use 1.1
// and the client accepts both, so the decoder does too.
const (
	NamespaceSyncML12 = "SYNCML:SYNCML1.2"
	NamespaceSyncML11 = "SYNCML:SYNCML1.1"
	NamespaceMetInf   = "syncml:metinf"
	// NamespaceMSFT is declared on SyncBody once SyncApplicationVersion is 3.0
	// or higher, and carries the msft:originalerror attribute on Data.
	NamespaceMSFT = "http://schemas.microsoft.com/MobileDevice/MDM"

	VerDTD   = "1.2"
	VerProto = "DM/1.2"

	// MIME types from OMA DM RepPro 1.2.1 section 5.1; XML is the default.
	ContentTypeXML   = "application/vnd.syncml.dm+xml"
	ContentTypeWBXML = "application/vnd.syncml.dm+wbxml"
)

// Command names as they appear on the wire and in Status/Cmd. SyncHdr is not
// a command but is the Cmd value of the Status that answers a header.
const (
	CmdAdd      = "Add"
	CmdAlert    = "Alert"
	CmdAtomic   = "Atomic"
	CmdDelete   = "Delete"
	CmdExec     = "Exec"
	CmdGet      = "Get"
	CmdReplace  = "Replace"
	CmdResults  = "Results"
	CmdSequence = "Sequence"
	CmdStatus   = "Status"
	CmdSyncHdr  = "SyncHdr"
)

// AlertCode is the numeric Data of an Alert command. OMA DM RepPro 1.2.1
// defines the table; MS-MDM 2.2.7.2 lists the seven Windows supports.
type AlertCode int

const (
	AlertServerInitiated AlertCode = 1200 // SERVER-INITIATED MGMT
	AlertClientInitiated AlertCode = 1201 // CLIENT-INITIATED MGMT
	AlertNextMessage     AlertCode = 1222 // NEXT MESSAGE
	AlertSessionAbort    AlertCode = 1223 // SESSION ABORT
	AlertClientEvent     AlertCode = 1224 // CLIENT EVENT (device alert)
	AlertNoEndOfData     AlertCode = 1225 // NO END OF DATA
	AlertGeneric         AlertCode = 1226 // GENERIC ALERT
)

var alertNames = map[AlertCode]string{
	AlertServerInitiated: "SERVER-INITIATED MGMT",
	AlertClientInitiated: "CLIENT-INITIATED MGMT",
	AlertNextMessage:     "NEXT MESSAGE",
	AlertSessionAbort:    "SESSION ABORT",
	AlertClientEvent:     "CLIENT EVENT",
	AlertNoEndOfData:     "NO END OF DATA",
	AlertGeneric:         "GENERIC ALERT",
}

// Known reports whether the code is one Windows uses.
func (c AlertCode) Known() bool { _, ok := alertNames[c]; return ok }

// String returns the OMA name, or the number for an unknown code.
func (c AlertCode) String() string {
	if n, ok := alertNames[c]; ok {
		return n
	}
	return strconv.Itoa(int(c))
}

// Wire returns the decimal text carried in Alert/Data.
func (c AlertCode) Wire() string { return strconv.Itoa(int(c)) }

// Alert Type strings the Windows client puts in Item/Meta/Type of a 1224 or
// 1226 alert. They are exact wire values and are never normalised.
const (
	// AlertTypeLoginStatus is sent in package 1 with Data user, others or none.
	AlertTypeLoginStatus = "com.microsoft/MDM/LoginStatus"
	// AlertTypeSyncType is sent with Data user, device or mixed (MS-MDM 3.2.5.1.5).
	AlertTypeSyncType = "com.microsoft.mdm.synctype"
	// AlertTypeDevicePrepSync reports provisioning state (MS-MDM 3.2.5.1.6).
	AlertTypeDevicePrepSync = "com.microsoft/MDM/DevicePrepSync"
	// AlertTypeAADUserToken carries the Entra user token (MS-MDM 3.2.5.1.2).
	AlertTypeAADUserToken = "com.microsoft/MDM/AADUserToken"
	// AlertTypeWin32CSPInstall reports asynchronous MSI installation results.
	AlertTypeWin32CSPInstall = "Reversed-Domain-Name:com.microsoft.mdm.win32csp_install"
	// AlertTypeDeclaredConfigurationDocuments carries WinDC document state.
	AlertTypeDeclaredConfigurationDocuments = "com.microsoft.mdm.declaredconfigurationdocuments"
	// AlertTypeUnenrollmentUserRequest is the 1226 a user-initiated unenroll sends.
	AlertTypeUnenrollmentUserRequest = "com.microsoft:mdm.unenrollment.userrequest"
)

// LoginStatus values carried in the AlertTypeLoginStatus alert.
const (
	LoginStatusUser   = "user"
	LoginStatusOthers = "others"
	LoginStatusNone   = "none"
)

// StatusCode is the numeric Data of a Status command, from SyncML RepPro
// 1.2.2 section 10 with the Windows meanings from the Learn OMA DM page.
type StatusCode int

const (
	StatusOK                     StatusCode = 200
	StatusAccepted               StatusCode = 202
	StatusAuthenticationAccepted StatusCode = 212
	StatusChunkedItemAccepted    StatusCode = 213
	StatusOperationCancelled     StatusCode = 214
	StatusNotExecuted            StatusCode = 215
	StatusAtomicRollbackOK       StatusCode = 216
	StatusBadRequest             StatusCode = 400
	StatusInvalidCredentials     StatusCode = 401
	StatusForbidden              StatusCode = 403
	StatusNotFound               StatusCode = 404
	StatusCommandNotAllowed      StatusCode = 405
	StatusOptionalFeature        StatusCode = 406
	StatusAuthenticationRequired StatusCode = 407
	StatusIncompleteCommand      StatusCode = 412
	StatusRequestEntityTooLarge  StatusCode = 413
	StatusUnsupportedMediaType   StatusCode = 415
	StatusRequestedSizeTooBig    StatusCode = 416
	StatusAlreadyExists          StatusCode = 418
	StatusDeviceFull             StatusCode = 420
	StatusSizeMismatch           StatusCode = 424
	StatusPermissionDenied       StatusCode = 425
	StatusCommandFailed          StatusCode = 500
	StatusAtomicFailed           StatusCode = 507
	StatusRefreshRequired        StatusCode = 508
	StatusAtomicRollbackFailed   StatusCode = 516
)

var statusDescriptions = map[StatusCode]string{
	StatusOK:                     "OK",
	StatusAccepted:               "Accepted for processing (asynchronous operation)",
	StatusAuthenticationAccepted: "Authentication accepted; no further authentication this session",
	StatusChunkedItemAccepted:    "Chunked item accepted and buffered",
	StatusOperationCancelled:     "Operation cancelled; no more commands are processed in the session",
	StatusNotExecuted:            "Not executed; the user cancelled the command",
	StatusAtomicRollbackOK:       "Atomic roll back OK",
	StatusBadRequest:             "Bad request; malformed syntax",
	StatusInvalidCredentials:     "Invalid credentials",
	StatusForbidden:              "Forbidden",
	StatusNotFound:               "Not found; the node does not exist",
	StatusCommandNotAllowed:      "Command not allowed; write to a read-only node, or wrong sync type scope",
	StatusOptionalFeature:        "Optional feature not supported by the CSP",
	StatusAuthenticationRequired: "Authentication required",
	StatusIncompleteCommand:      "Incomplete command",
	StatusRequestEntityTooLarge:  "Request entity too large",
	StatusUnsupportedMediaType:   "Unsupported type or format; XML parsing or formatting error",
	StatusRequestedSizeTooBig:    "Requested size too big",
	StatusAlreadyExists:          "Already exists",
	StatusDeviceFull:             "Device full",
	StatusSizeMismatch:           "Size mismatch",
	StatusPermissionDenied:       "Permission denied (ACL)",
	StatusCommandFailed:          "Command failed; the SyncML DPU could not map the originating error",
	StatusAtomicFailed:           "Atomic failed",
	StatusRefreshRequired:        "Refresh required",
	StatusAtomicRollbackFailed:   "Atomic roll back failed",
}

// Known reports whether the code is in the SyncML RepPro table as Windows uses it.
func (c StatusCode) Known() bool { _, ok := statusDescriptions[c]; return ok }

// String returns the Windows meaning of the code, or the number for an unknown one.
func (c StatusCode) String() string {
	if d, ok := statusDescriptions[c]; ok {
		return d
	}
	return strconv.Itoa(int(c))
}

// Wire returns the decimal text carried in Status/Data.
func (c StatusCode) Wire() string { return strconv.Itoa(int(c)) }

// Success reports whether the code is in the 2xx class.
func (c StatusCode) Success() bool { return c >= 200 && c < 300 }

// ParseStatusCode parses the decimal Data of a Status.
func ParseStatusCode(s string) (StatusCode, error) {
	n, err := strconv.Atoi(s)
	if err != nil || n < 100 || n > 999 {
		return 0, &SyntaxError{Element: "Status/Data", Msg: "not a status code: " + strconv.Quote(s)}
	}
	return StatusCode(n), nil
}

// ParseAlertCode parses the decimal Data of an Alert.
func ParseAlertCode(s string) (AlertCode, error) {
	n, err := strconv.Atoi(s)
	if err != nil || n < 1000 || n > 9999 {
		return 0, &SyntaxError{Element: "Alert/Data", Msg: "not an alert code: " + strconv.Quote(s)}
	}
	return AlertCode(n), nil
}

// Meta/Format values. The first nine are the DFFormat values in the DDF
// bundle; date and float complete the OMA MetaInfo set.
const (
	FormatChr   = "chr"
	FormatInt   = "int"
	FormatNode  = "node"
	FormatBool  = "bool"
	FormatB64   = "b64"
	FormatNull  = "null"
	FormatXML   = "xml"
	FormatBin   = "bin"
	FormatTime  = "time"
	FormatDate  = "date"
	FormatFloat = "float"
)

var knownFormats = map[string]bool{
	FormatChr: true, FormatInt: true, FormatNode: true, FormatBool: true, FormatB64: true,
	FormatNull: true, FormatXML: true, FormatBin: true, FormatTime: true, FormatDate: true, FormatFloat: true,
}

// KnownFormat reports whether f is a Meta/Format value the tree can carry.
func KnownFormat(f string) bool { return knownFormats[f] }

// Meta/Mark importance levels for generic alerts (OMA DM Protocol 8.7.1.8),
// most important first. A missing Mark means informational. MS-MDM sends the
// MDM-GenericAlert HTTP header when a session is triggered by a fatal or
// critical alert.
const (
	MarkFatal         = "fatal"
	MarkCritical      = "critical"
	MarkMinor         = "minor"
	MarkWarning       = "warning"
	MarkInformational = "informational"
	MarkHarmless      = "harmless"
	MarkIndeterminate = "indeterminate"
)

// Authentication types carried in Cred/Meta/Type and Chal/Meta/Type.
const (
	AuthBasic = "syncml:auth-basic"
	AuthMD5   = "syncml:auth-md5"
)

// AttrOriginalError is the attribute a Windows client adds to Status/Data
// with the HRESULT of the failing component once SyncApplicationVersion is
// 3.0 or higher (MS-MDM 2.2.5.1).
const AttrOriginalError = "originalerror"
