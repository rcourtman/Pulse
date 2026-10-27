param(
    [Parameter(Mandatory = $true)][string]$HarnessPath,
    [Parameter(Mandatory = $true)][string]$InstallerPath,
    [Parameter(Mandatory = $true)][string]$TempDir
)
$ErrorActionPreference = 'Stop'
$powerShellPath = (Get-Process -Id $PID).Path
foreach ($source in @(
    @{ Path = $InstallerPath; Name = 'Show-Error' },
    @{ Path = $HarnessPath; Name = 'Invoke-SavedUninstall' }
)) {
    $errors = $null
    $ast = [System.Management.Automation.Language.Parser]::ParseFile($source.Path, [ref]$null, [ref]$errors)
    if ($errors.Count) { throw 'Production source did not parse.' }
    $name = $source.Name
    $functions = @($ast.FindAll({ param($node)
        $node -is [System.Management.Automation.Language.FunctionDefinitionAst] -and $node.Name -eq $name
    }, $true))
    if ($functions.Count -ne 1) { throw "Expected one production function: $name" }
    Invoke-Expression $functions[0].Extent.Text
}

# Observe the real error function without opening any desktop UI. An attempted
# desktop assembly load is counted then refused by this private control.
function Add-Type {
    [CmdletBinding()]
    param([string]$AssemblyName)
    $script:desktopAttempts++
    throw 'Desktop UI disabled by this no-mutation control.'
}
foreach ($NonInteractive in @($true, $false)) {
    $script:desktopAttempts = 0
    Show-Error 'Controlled installer failure'
    $expected = 1
    if ($NonInteractive) { $expected = 0 }
    if ($script:desktopAttempts -ne $expected) { throw 'NonInteractive error attempted desktop UI, or interactive behaviour changed.' }
}
Write-Output 'PASS real installer error function: unattended returns without desktop; interactive behaviour retained'

$names = @('PULSE_URL', 'PULSE_TOKEN', 'PULSE_TOKEN_FILE', 'PULSE_AGENT_ID',
    'PULSE_HOSTNAME', 'PULSE_CACERT', 'PULSE_SERVER_FINGERPRINT', 'PULSE_INSECURE_SKIP_VERIFY')
$before = @{}
$childInputCheck = ''
foreach ($name in $names) {
    $before[$name] = [Environment]::GetEnvironmentVariable($name, 'Process')
    [Environment]::SetEnvironmentVariable($name, 'ambient-fixture-must-not-be-inherited', 'Process')
    $childInputCheck += "if (-not [string]::IsNullOrEmpty(`$env:$name)) { throw 'Ambient input inherited' }; "
}
$resolvedInstallerPath = Join-Path $TempDir "saved removal ' child.ps1"
$diagnostic = 'The binary, saved identity, credentials and logs have been retained.'
$controls = @(
    @{ Name = 'success'; Body = "Write-Host 'Uninstallation complete.'; exit 0"; Failure = $false; Accepted = $true },
    @{ Name = 'refusal'; Body = "Write-Host '$diagnostic'; exit 1"; Failure = $true; Accepted = $true },
    @{ Name = 'wrong-exit'; Body = "Write-Host '$diagnostic'; exit 0"; Failure = $true; Accepted = $false },
    @{ Name = 'unexpected-error'; Body = "Write-Host '$diagnostic'; exit 1"; Failure = $false; Accepted = $false },
    @{ Name = 'missing-diagnostic'; Body = 'exit 1'; Failure = $true; Accepted = $false },
    @{ Name = 'false-completion'; Body = "Write-Host '$diagnostic Uninstallation complete.'; exit 1"; Failure = $true; Accepted = $false },
    @{ Name = 'private-response'; Body = "Write-Host '$diagnostic lifecycle-private-response-must-not-appear'; exit 1"; Failure = $true; Accepted = $false },
    @{ Name = 'credential'; Body = "Write-Host '$diagnostic windows-lifecycle-fixture-not-a-secret'; exit 1"; Failure = $true; Accepted = $false }
)
try {
    foreach ($control in $controls) {
        Set-Content -LiteralPath $resolvedInstallerPath -Value ($childInputCheck + $control.Body) -Encoding UTF8
        $failure = $null
        try { Invoke-SavedUninstall -Label $control.Name -ExpectFailure $control.Failure } catch { $failure = $_ }
        if (($null -eq $failure) -ne $control.Accepted) { throw "Saved removal child verdict mismatch: $($control.Name)" }
        foreach ($name in $names) {
            if ([Environment]::GetEnvironmentVariable($name, 'Process') -cne 'ambient-fixture-must-not-be-inherited') {
                throw 'Child environment isolation changed the parent environment.'
            }
        }
        Write-Output "PASS saved removal child $($control.Name): exit, diagnostic, private-output and ambient-input controls"
    }
} finally {
    foreach ($name in $names) { [Environment]::SetEnvironmentVariable($name, $before[$name], 'Process') }
}
Write-Output 'PASS all 10 saved removal controls; no actual SCM, install, deletion or provider request'
