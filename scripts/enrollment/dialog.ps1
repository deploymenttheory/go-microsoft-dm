#requires -Version 7.0
[CmdletBinding()]
param(
    [string]$StateDirectory = (Join-Path $PSScriptRoot '../../tmp/enrollment'),
    [switch]$Submit
)
$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName UIAutomationClient
Add-Type -AssemblyName UIAutomationTypes
$window = [System.Windows.Automation.AutomationElement]::RootElement.FindFirst(
    [System.Windows.Automation.TreeScope]::Children,
    [System.Windows.Automation.PropertyCondition]::new([System.Windows.Automation.AutomationElement]::NameProperty, 'Microsoft account'))
if (-not $window) { throw 'Native enrollment window not found. Run the Enroll action first.' }

function Find-Control([string]$id) {
    $control = $window.FindFirst([System.Windows.Automation.TreeScope]::Descendants,
        [System.Windows.Automation.PropertyCondition]::new([System.Windows.Automation.AutomationElement]::AutomationIdProperty, $id))
    if (-not $control) { throw "Enrollment control not found: $id. Inspect the dialog before continuing." }
    return $control
}

if ($Submit) {
    $credentials = Get-Content (Join-Path $StateDirectory 'credentials.json') -Raw | ConvertFrom-Json
    $initial = $window.FindFirst([System.Windows.Automation.TreeScope]::Descendants,
        [System.Windows.Automation.PropertyCondition]::new([System.Windows.Automation.AutomationElement]::AutomationIdProperty, 'userName'))
    if ($initial) {
        $server = Find-Control 'serverName'
        $next = Find-Control 'NextButton'
        $initial.GetCurrentPattern([System.Windows.Automation.ValuePattern]::Pattern).SetValue($credentials.upn)
        $server.GetCurrentPattern([System.Windows.Automation.ValuePattern]::Pattern).SetValue('https://localhost:8443')
        $next.GetCurrentPattern([System.Windows.Automation.InvokePattern]::Pattern).Invoke()
        Write-Output 'Discovery submitted. Run Submit again when the password form appears.'
        return
    }
    $username = Find-Control 'domainName'
    $password = Find-Control 'password'
    $next = Find-Control 'NextButton'
    $username.GetCurrentPattern([System.Windows.Automation.ValuePattern]::Pattern).SetValue($credentials.upn)
    $password.GetCurrentPattern([System.Windows.Automation.ValuePattern]::Pattern).SetValue($credentials.password)
    $next.GetCurrentPattern([System.Windows.Automation.InvokePattern]::Pattern).Invoke()
} else {
    $window.FindAll([System.Windows.Automation.TreeScope]::Descendants, [System.Windows.Automation.Condition]::TrueCondition) |
        Where-Object { $_.Current.ControlType.ProgrammaticName -in 'ControlType.Text', 'ControlType.Edit', 'ControlType.Button' } |
        ForEach-Object { [pscustomobject]@{ Name = $_.Current.Name; Id = $_.Current.AutomationId; Enabled = $_.Current.IsEnabled } }
}
