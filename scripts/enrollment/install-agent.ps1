#requires -Version 7.0
[CmdletBinding()]
param(
    [Parameter(Mandatory)][string]$ConfigPath,
    [string]$InstallDirectory = (Join-Path $env:ProgramData 'go-microsoft-dm/agent'),
    [string]$TaskName = 'go-microsoft-dm agent wake'
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
if (-not $IsWindows) { throw 'Windows agent installation requires Windows.' }
$principal = [Security.Principal.WindowsPrincipal]::new([Security.Principal.WindowsIdentity]::GetCurrent())
if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) { throw 'Run in an elevated PowerShell window.' }

$source = (Resolve-Path -LiteralPath $ConfigPath).Path
$config = Get-Content -LiteralPath $source -Raw | ConvertFrom-Json
foreach ($name in @('ServerURL', 'DeviceID', 'Token', 'ProviderID', 'DiscoveryURL')) {
    if (-not $config.PSObject.Properties[$name] -or -not $config.$name) { throw "Missing agent config $name." }
}
$target = [IO.Path]::GetFullPath($InstallDirectory)
$null = New-Item -ItemType Directory -Path $target -Force
Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'agent-wake.ps1') -Destination $target -Force
Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'agent-poll.ps1') -Destination $target -Force
Copy-Item -LiteralPath $source -Destination (Join-Path $target 'config.json') -Force

# The token file is read by SYSTEM and local administrators only.
& icacls.exe $target /inheritance:r /grant:r '*S-1-5-18:(OI)(CI)F' '*S-1-5-32-544:(OI)(CI)F' | Out-Null
if ($LASTEXITCODE -ne 0) { throw 'Could not restrict agent directory ACL.' }

$exe = (Get-Command pwsh.exe).Source
$args = '-NoProfile -File "' + (Join-Path $target 'agent-poll.ps1') + '" -ConfigPath "' + (Join-Path $target 'config.json') + '"'
$action = New-ScheduledTaskAction -Execute $exe -Argument $args
$trigger = New-ScheduledTaskTrigger -Once -At (Get-Date).AddMinutes(1) -RepetitionInterval (New-TimeSpan -Minutes 1)
$settings = New-ScheduledTaskSettingsSet -MultipleInstances IgnoreNew -ExecutionTimeLimit (New-TimeSpan -Minutes 1)
Register-ScheduledTask -TaskName $TaskName -Action $action -Trigger $trigger -Settings $settings -User 'SYSTEM' -RunLevel Highest -Force | Out-Null
Write-Output "Installed $TaskName; config at $target"
