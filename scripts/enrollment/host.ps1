#requires -Version 7.0
[CmdletBinding()]
param(
    [Parameter(Mandatory)]
    [ValidateSet('Setup', 'Start', 'Stop', 'Trust', 'Enroll', 'Submit', 'Status', 'Query', 'Probe', 'Push', 'PushState', 'Sync', 'Results', 'Unenroll', 'Cleanup')]
    [string]$Action,
    [string]$StateDirectory = (Join-Path $PSScriptRoot '../../tmp/enrollment'),
    [string]$UserName = 'host-validation@example.com',
    [string]$DeviceID,
    [switch]$Capture
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
if (-not $IsWindows) { throw 'Native enrollment validation requires Windows.' }
$repo = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$stateDir = [IO.Path]::GetFullPath($StateDirectory)
$provider = 'go-microsoft-dm-local-test'
$baseURL = 'https://localhost:8443'

function Require-Admin {
    $principal = [Security.Principal.WindowsPrincipal]::new([Security.Principal.WindowsIdentity]::GetCurrent())
    if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
        throw 'Run this action in an elevated PowerShell 7 window. Use pwsh -Mta for Unenroll.'
    }
}

function Get-Registration {
    if (-not ('EnrollmentValidation.Native' -as [type])) {
        Add-Type -TypeDefinition @'
using System;
using System.Text;
using System.Runtime.InteropServices;
namespace EnrollmentValidation {
    public static class Native {
        [DllImport("MDMRegistration.dll", CharSet=CharSet.Unicode)]
        public static extern int IsDeviceRegisteredWithManagement(
            [MarshalAs(UnmanagedType.Bool)] out bool registered, uint capacity, StringBuilder upn);
        [DllImport("MDMRegistration.dll", CharSet=CharSet.Unicode)]
        public static extern int UnregisterDeviceWithManagement(string enrollmentID);
    }
}
'@
    }
    $registered = $false
    $upn = [Text.StringBuilder]::new(1024)
    $hr = [EnrollmentValidation.Native]::IsDeviceRegisteredWithManagement([ref]$registered, 1024, $upn)
    if ($hr -ne 0) { throw ('Registration query failed: 0x{0:X8}' -f $hr) }
    return $registered
}

function Get-TestEnrollment {
    $entries = @(Get-ChildItem 'HKLM:/SOFTWARE/Microsoft/Enrollments' | ForEach-Object {
        $entry = Get-ItemProperty $_.PSPath
        if ($entry.PSObject.Properties['ProviderID'] -and $entry.ProviderID -eq $provider -and
            $entry.PSObject.Properties['DiscoveryServiceFullURL'] -and $entry.DiscoveryServiceFullURL -eq $baseURL) {
            $_.PSChildName
        }
    })
    if ($entries.Count -ne 1) { throw "Expected exactly one localhost test enrollment; found $($entries.Count)." }
    return $entries[0]
}

function Set-ServerEnvironment {
    $credentials = Get-Content (Join-Path $stateDir 'credentials.json') -Raw | ConvertFrom-Json
    if ($credentials.provider -ne $provider) { throw 'Unexpected test provider.' }
    $env:DM_STORE = 'sqlite'
    $env:DM_DSN = 'file:' + ((Join-Path $stateDir 'host.db') -replace '\\', '/')
    $env:DM_LISTEN = '127.0.0.1:8443'
    $env:DM_BASE_URL = $baseURL
    $env:DM_PROVIDER_ID = $provider
    $env:DM_NAME = 'Local enrollment validation'
    $env:DM_ENROLL_USERS = $credentials.upn + ':' + $credentials.password
    $env:DM_ENROLL_ALLOW_ANY = 'false'
    $env:DM_TLS_CERT = Join-Path $stateDir 'tls.pem'
    $env:DM_TLS_KEY = Join-Path $stateDir 'tls-key.pem'
    $env:DM_CA_CERT = Join-Path $stateDir 'root.pem'
    $env:DM_CA_KEY = Join-Path $stateDir 'root-key.pem'
}

function Stop-TestServer {
    $pidFile = Join-Path $stateDir 'server.pid'
    if (-not (Test-Path $pidFile)) { return }
    $serverProcess = Get-Process -Id ([int](Get-Content $pidFile)) -ErrorAction SilentlyContinue
    if ($serverProcess) {
        if ($serverProcess.Path -ne (Join-Path $stateDir 'dmserver.exe')) { throw 'PID belongs to a different process.' }
        Stop-Process -Id $serverProcess.Id
        $serverProcess.WaitForExit()
    }
    Remove-Item -LiteralPath $pidFile
}

switch ($Action) {
    'Push' {
        if (-not $DeviceID) { throw 'Specify -DeviceID.' }
        Set-ServerEnvironment
        & (Join-Path $stateDir 'dmctl.exe') push $DeviceID
        if ($LASTEXITCODE -ne 0) { throw 'WNS push failed.' }
    }
    'PushState' {
        if (-not $DeviceID) { throw 'Specify -DeviceID.' }
        Set-ServerEnvironment
        & (Join-Path $stateDir 'dmctl.exe') push-state $DeviceID
        if ($LASTEXITCODE -ne 0) { throw 'Reading push state failed.' }
    }
    'Setup' {
        if ($UserName -notmatch '^[^@\s]+@[^@\s.]+(?:\.[^@\s.]+)+$') { throw 'Use a full username such as host-validation@example.com.' }
        if (Test-Path $stateDir) {
            if (@(Get-ChildItem -LiteralPath $stateDir -Force).Count -gt 0) { throw 'Setup requires a new or empty state directory.' }
        } else { $null = New-Item -ItemType Directory -Path $stateDir }
        Push-Location $repo
        try {
            & go run ./scripts/enrollment/_setup/main.go -dir $stateDir -upn $UserName
            if ($LASTEXITCODE -ne 0) { throw 'Certificate setup failed.' }
        } finally { Pop-Location }
        Write-Output "Created local test credentials and certificates in $stateDir"
    }
    'Start' {
        if (Test-Path (Join-Path $stateDir 'server.pid')) { throw 'Run Stop before restarting this test server.' }
        if (Get-NetTCPConnection -LocalPort 8443 -State Listen -ErrorAction SilentlyContinue) { throw 'Port 8443 is already in use.' }
        Set-ServerEnvironment
        Push-Location (Join-Path $repo 'server')
        try {
            foreach ($name in @('dmserver', 'dmctl')) {
                $source = "./cmd/$name"
                if ($Capture -and $name -eq 'dmserver') { $source = './e2e/host/_server' }
                $buildArgs = @('build')
                if ($Capture -and $name -eq 'dmserver') { $buildArgs += @('-tags', 'host') }
                & go @buildArgs -o (Join-Path $stateDir "$name.exe") $source
                if ($LASTEXITCODE -ne 0) { throw "Build failed: $name" }
            }
        } finally { Pop-Location }
        $stamp = Get-Date -Format 'yyyyMMdd-HHmmss-fff'
        if ($Capture) {
            $env:DM_CAPTURE_DIR = Join-Path $stateDir 'captures'
            $env:DM_EXPERIMENT_FILE = Join-Path $stateDir 'experiment.json'
            if (-not (Test-Path $env:DM_EXPERIMENT_FILE)) { '{"label":"baseline"}' | Set-Content $env:DM_EXPERIMENT_FILE }
        }
        $stderr = Join-Path $stateDir "server-$stamp-stderr.log"
        $stdout = Join-Path $stateDir "server-$stamp-stdout.log"
        $process = Start-Process -FilePath (Join-Path $stateDir 'dmserver.exe') -WindowStyle Hidden -PassThru -RedirectStandardError $stderr -RedirectStandardOutput $stdout
        $process.Id | Set-Content (Join-Path $stateDir 'server.pid')
        Write-Output "Started PID $($process.Id); log: $stderr"
    }
    'Stop' { Stop-TestServer }
    'Trust' {
        Require-Admin
        $cert = [Security.Cryptography.X509Certificates.X509Certificate2]::new((Join-Path $stateDir 'root.cer'))
        if ($cert.Subject -ne 'CN=go-microsoft-dm local enrollment validation' -or $cert.NotAfter -le (Get-Date)) { throw 'Unexpected or expired test root.' }
        if (Test-Path "Cert:/LocalMachine/Root/$($cert.Thumbprint)") { throw 'Test root already trusted; preserve its existing ownership marker.' }
        $store = [Security.Cryptography.X509Certificates.X509Store]::new('Root', 'LocalMachine')
        try { $store.Open('ReadWrite'); $store.Add($cert) } finally { $store.Close() }
        $cert.Thumbprint | Set-Content (Join-Path $stateDir 'installed-root-thumbprint.txt')
        Write-Output 'Installed the local test root.'
    }
    'Enroll' {
        if (Get-Registration) { throw 'This device is already managed. Remove only a previous test enrollment before retrying.' }
        $credentials = Get-Content (Join-Path $stateDir 'credentials.json') -Raw | ConvertFrom-Json
        $null = Invoke-WebRequest "$baseURL/EnrollmentServer/Discovery.svc"
        $upn = [Uri]::EscapeDataString($credentials.upn)
        $server = [Uri]::EscapeDataString($baseURL)
        Start-Process "ms-device-enrollment:?mode=mdm&username=$upn&servername=$server"
    }
    'Submit' { & (Join-Path $PSScriptRoot 'dialog.ps1') -StateDirectory $stateDir -Submit }
    'Status' {
        [pscustomobject]@{ Registered = (Get-Registration); StateDirectory = $stateDir }
        Get-WinEvent -FilterHashtable @{
            LogName = 'Microsoft-Windows-DeviceManagement-Enterprise-Diagnostics-Provider/Enrollment'
            Id = 16, 17, 56, 58, 72
            StartTime = (Get-Date).AddHours(-1)
        } -MaxEvents 12 -ErrorAction SilentlyContinue | Select-Object TimeCreated, Id, Message
    }
    'Query' {
        if (-not $DeviceID) { throw 'Specify -DeviceID from the Results enrollment list.' }
        Set-ServerEnvironment
        $commandFile = Join-Path $stateDir 'get-man.txt'
        'get ./DevInfo/Man' | Set-Content $commandFile
        & (Join-Path $stateDir 'dmctl.exe') queue $DeviceID $commandFile
        if ($LASTEXITCODE -ne 0) { throw 'Queueing the read-only query failed.' }
    }
    'Sync' {
        Require-Admin
        $id = Get-TestEnrollment
        Start-ScheduledTask -TaskPath "\Microsoft\Windows\EnterpriseMgmt\$id\" -TaskName 'Schedule #1 created by enrollment client'
    }
    'Probe' {
        if (-not $DeviceID) { throw 'Specify -DeviceID from the Results enrollment list.' }
        Set-ServerEnvironment
        $commandFile = Join-Path $stateDir 'probes.txt'
        @(
            'get ./DevDetail/SwV'
            'get ./Device/Vendor/MSFT/DeviceManageability/Capabilities/CSPVersions'
            'get ./Device/Vendor/MSFT/DeclaredConfiguration/Host/BulkTemplate'
            'get ./User/Vendor/MSFT/DeclaredConfiguration'
            'get ./Device/Vendor/MSFT/DeclaredConfiguration/ManagementServiceConfiguration/RefreshInterval'
        ) | Set-Content $commandFile
        & (Join-Path $stateDir 'dmctl.exe') queue $DeviceID $commandFile
        if ($LASTEXITCODE -ne 0) { throw 'Queueing conformance probes failed; use the capture harness for unknown CSP paths.' }
    }
    'Results' {
        Set-ServerEnvironment
        $cliArgs = @('enrollments')
        if ($DeviceID) { $cliArgs = @('results', $DeviceID) }
        & (Join-Path $stateDir 'dmctl.exe') @cliArgs
        if ($LASTEXITCODE -ne 0) { throw 'Reading test results failed.' }
    }
    'Unenroll' {
        Require-Admin
        if ([Threading.Thread]::CurrentThread.GetApartmentState() -ne 'MTA') { throw 'Run Unenroll with pwsh -Mta -File scripts/enrollment/host.ps1 -Action Unenroll.' }
        $id = Get-TestEnrollment
        $null = Get-Registration
        $hr = [EnrollmentValidation.Native]::UnregisterDeviceWithManagement($id)
        if ($hr -ne 0) { throw ('Unregister failed: 0x{0:X8}' -f $hr) }
        Write-Output "Unregistered localhost test enrollment $id"
    }
    'Cleanup' {
        Require-Admin
        if (Get-Registration) { throw 'Unenroll the test enrollment first; no trust changes were made.' }
        $marker = Join-Path $stateDir 'installed-root-thumbprint.txt'
        if (Test-Path $marker) {
            $cert = [Security.Cryptography.X509Certificates.X509Certificate2]::new((Join-Path $stateDir 'root.cer'))
            $thumbprint = (Get-Content $marker).Trim()
            if ($cert.Subject -ne 'CN=go-microsoft-dm local enrollment validation' -or $thumbprint -ne $cert.Thumbprint) { throw 'Test certificate does not match its ownership marker.' }
            $certPath = "Cert:/LocalMachine/Root/$thumbprint"
            if (Test-Path $certPath) { Remove-Item -LiteralPath $certPath }
            Remove-Item -LiteralPath $marker
        }
        Stop-TestServer
        Write-Output 'Test server stopped and test trust removed. Evidence files retained.'
    }
}
