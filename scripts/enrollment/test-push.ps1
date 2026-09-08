#requires -Version 7.0
[CmdletBinding()]
param(
    [Parameter(Mandatory)][string]$DeviceID,
    [string]$StateDirectory = (Join-Path $PSScriptRoot '../../tmp/enrollment'),
    [ValidateRange(1,600)][int]$TimeoutSeconds = 120
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
if (-not $IsWindows) { throw 'Native push validation requires Windows.' }
if (-not $env:DM_WNS_PFN -or -not $env:DM_WNS_CLIENT_ID -or -not $env:DM_WNS_CLIENT_SECRET) {
    Write-Output 'Skipped: configure DM_WNS_PFN, DM_WNS_CLIENT_ID and DM_WNS_CLIENT_SECRET.'
    return
}
$hostScript = Join-Path $PSScriptRoot 'host.ps1'
$before = & $hostScript -Action PushState -DeviceID $DeviceID -StateDirectory $StateDirectory | ConvertFrom-Json
if (-not $before.has_channel) { throw 'No channel reported yet. Provision the matching PFN and complete a management session first.' }
$started = [DateTimeOffset]::UtcNow
$push = & $hostScript -Action Push -DeviceID $DeviceID -StateDirectory $StateDirectory | ConvertFrom-Json
$arrived = $false
$after = $before
do {
    Start-Sleep -Seconds 2
    $after = & $hostScript -Action PushState -DeviceID $DeviceID -StateDirectory $StateDirectory | ConvertFrom-Json
    $arrived = ([DateTimeOffset]$after.last_seen) -gt $started
} while (-not $arrived -and ([DateTimeOffset]::UtcNow - $started).TotalSeconds -lt $TimeoutSeconds)
$eventErrors = @()
$events = @(Get-WinEvent -FilterHashtable @{
    LogName = 'Microsoft-Windows-DeviceManagement-Enterprise-Diagnostics-Provider/Admin'
    Id = 4603
    StartTime = $started.LocalDateTime
} -ErrorAction SilentlyContinue -ErrorVariable eventErrors | Select-Object Id, TimeCreated, RecordId)
$evidence = [ordered]@{
    StartedAt = $started
    WNS = $push
    SessionObserved = $arrived
    LastSeen = $after.last_seen
    Event4603 = $events
    EventReadErrors = @($eventErrors | ForEach-Object { $_.FullyQualifiedErrorId })
    Attribution = 'Compare with polling task times: session arrival during this window alone does not prove push caused it.'
}
$path = Join-Path $StateDirectory ('push-evidence-' + $started.ToString('yyyyMMddTHHmmss') + '.json')
$evidence | ConvertTo-Json -Depth 6 | Set-Content -LiteralPath $path
Write-Output "Evidence saved to $path"
if (-not $arrived) { throw 'WNS request completed, but no authenticated management session was observed before timeout.' }
