#requires -Version 7.0
[CmdletBinding()]
param(
    [string]$ProviderID = 'go-microsoft-dm-local-test',
    [string]$DiscoveryURL = 'https://localhost:8443'
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
if (-not $IsWindows) { throw 'This wake probe requires Windows.' }

# Limit the probe to the one enrollment created by the local validation server.
$matches = @(Get-ChildItem 'HKLM:/SOFTWARE/Microsoft/Enrollments' | ForEach-Object {
    $entry = Get-ItemProperty $_.PSPath
    if ($entry.PSObject.Properties['ProviderID'] -and $entry.ProviderID -eq $ProviderID -and
        $entry.PSObject.Properties['DiscoveryServiceFullURL'] -and
        $entry.DiscoveryServiceFullURL -eq $DiscoveryURL -and
        $entry.PSObject.Properties['EnrollmentState'] -and
        [int]$entry.EnrollmentState -ne 0) {
        $_.PSChildName
    }
})
if ($matches.Count -ne 1) { throw "Expected exactly one active test enrollment; found $($matches.Count)." }

$enrollmentID = $matches[0]
$before = Get-Date
$process = Start-Process -FilePath "$env:WINDIR/System32/deviceenroller.exe" -ArgumentList @('/o', $enrollmentID, '/c') -Wait -PassThru
$task = Get-ScheduledTask -TaskPath "\Microsoft\Windows\EnterpriseMgmt\$enrollmentID\" -TaskName 'Schedule to run OMADMClient by client' -ErrorAction SilentlyContinue
$lastTaskRun = if ($task) { (Get-ScheduledTaskInfo $task).LastRunTime } else { $null }
[pscustomobject]@{
    EnrollmentID = $enrollmentID
    TriggeredAt = $before.ToString('o')
    DeviceEnrollerExitCode = ('0x{0:X8}' -f ($process.ExitCode -band 0xffffffff))
    ClientTaskLastRun = $lastTaskRun
}

# Windows can return an HRESULT even when it schedules a successful session.
# Check server command results and capture timestamps before judging delivery.
