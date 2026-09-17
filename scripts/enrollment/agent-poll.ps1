#requires -Version 7.0
[CmdletBinding()]
param([Parameter(Mandatory)][string]$ConfigPath)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
if (-not $IsWindows) { throw 'Windows agent polling requires Windows.' }

$config = Get-Content -LiteralPath $ConfigPath -Raw | ConvertFrom-Json
foreach ($name in @('ServerURL', 'DeviceID', 'Token', 'ProviderID', 'DiscoveryURL')) {
    if (-not $config.PSObject.Properties[$name] -or -not $config.$name) { throw "Missing agent config $name." }
}
$base = [Uri]$config.ServerURL
if ($base.Scheme -ne 'https' -or $base.UserInfo -or $base.Query -or $base.Fragment) { throw 'ServerURL must be a plain HTTPS base URL.' }
if ($config.Token -notmatch '^[A-Za-z0-9_-]{43}$') { throw 'Invalid agent token format.' }

$query = [Uri]::EscapeDataString([string]$config.DeviceID)
$uri = [Uri]::new($base, '/agent/wake?device_id=' + $query)
$response = Invoke-RestMethod -Uri $uri -Method Get -Headers @{ Authorization = 'Bearer ' + $config.Token } -TimeoutSec 15
if (-not $response.PSObject.Properties['wake_id'] -or -not $response.wake_id) { return }

$statePath = Join-Path (Split-Path -Parent $ConfigPath) 'agent-state.json'
$previous = if (Test-Path -LiteralPath $statePath) { Get-Content -LiteralPath $statePath -Raw | ConvertFrom-Json } else { $null }
$now = [DateTimeOffset]::UtcNow
if ($previous -and $previous.WakeID -eq $response.wake_id -and
    ($now - [DateTimeOffset]::Parse($previous.TriggeredAt)).TotalSeconds -lt 120) { return }

$wakeScript = Join-Path $PSScriptRoot 'agent-wake.ps1'
$result = & $wakeScript -ProviderID $config.ProviderID -DiscoveryURL $config.DiscoveryURL
if (-not $result -or -not $result.ClientTaskLastRun -or $result.ClientTaskLastRun -lt $now.AddMinutes(-1).LocalDateTime) {
    throw 'Client-triggered OMA-DM task did not run; wake will be retried.'
}
@{ WakeID = [string]$response.wake_id; TriggeredAt = $now.ToString('o') } |
    ConvertTo-Json -Compress | Set-Content -LiteralPath $statePath
$result
