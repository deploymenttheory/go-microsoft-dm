# SyncML fixtures

Every file is a message or fragment taken from a primary source, unchanged except for
whitespace where noted. The source and date are in the file name and the table below.

| File | Source | Notes |
|---|---|---|
| `learn-oma-dm-loginstatus-alert.xml` | Learn "OMA DM protocol support", ms.date 2025-08-04 | Alert 1224 `com.microsoft/MDM/LoginStatus` fragment |
| `fleet-mdmtest-package1.xml` | fleetdm/fleet `pkg/mdm/mdmtest/windows.go` `StartManagementSession` | Package 1 the way Fleet's test client sends it: 1201, LoginStatus 1224, DevInfo Replace, Final |
| `fleet-mdmtest-unenroll-1226.xml` | same file, `Unenroll` | Generic alert 1226 `com.microsoft:mdm.unenrollment.userrequest` |
| `msmdm-3.1.5.1.3-atomic.xml` | MS-MDM v15.0 section 3.1.5.1.3 | Atomic with two Replace commands (fragment, header added) |
| `msmdm-3.1.5.1.5-exec.xml` | MS-MDM v15.0 section 3.1.5.1.5 | Exec with Meta Format and Type |
| `msmdm-3.1.5.2-status-results.xml` | MS-MDM v15.0 sections 3.1.5.2.1 and 3.1.5.2.2 | Status and Results for a Get |
| `oma-security-5.3.1-md5-cred.xml` | OMA DM Security 1.2.1 section 5.3.1 | SyncHdr with `Cred` auth-md5 and `MaxMsgSize` (quotes normalised) |
| `learn-diagnosticlog-exec-snap.xml` | Learn DiagnosticLog CSP, ms.date 2025-03-12 | Exec with Format chr and Data SNAP (`SYNCML:SYNCML1.2`, no SyncHdr) |
| `learn-diagnosticlog-results-collection.xml` | same page | Status for SyncHdr, Status for Get, Results whose Data is markup |
| `learn-diagnosticlog-replace-type-no-ns.xml` | same page | `Type` inside Meta without the metinf namespace; LocURI padded with whitespace |
| `learn-edam-add-exec-msiinstalljob.xml` | Learn EnterpriseDesktopAppManagement CSP, ms.date 2025-03-12 | `SYNCML:SYNCML1.1`; Add then Exec whose Data is an MsiInstallJob document |
| `learn-edam-response-utf16-decl.xml` | same page | Response with an `encoding="utf-16"` declaration, empty SyncHdr, Status for SyncHdr |
| `learn-edam-alert-1224-win32csp.xml` | same page, "Alert example" | 1224 with Type, Format and Mark |
| `learn-windc-1224-declaredconfigurationdocuments.xml` | Learn DeclaredConfiguration CSP, ms.date 2025-03-12 (shape recorded in docs/research.md 1.6) | 1224 whose Data carries a `DeclaredConfigurations` document; `SYNCML:SYNCML1.1` |
