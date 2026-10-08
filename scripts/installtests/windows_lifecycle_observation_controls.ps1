param([Parameter(Mandatory = $true)][string]$HarnessPath)
$ErrorActionPreference = 'Stop'
$serviceName = 'PulseAgent'
$stateDir = 'test-only-state'
$errors = $null
$ast = [System.Management.Automation.Language.Parser]::ParseFile($HarnessPath, [ref]$null, [ref]$errors)
if ($errors.Count) { throw "Harness parse failed: $errors" }
# Only the actual observation functions run. No administrator check, actual SCM,
# file deletion, listener mutation, or application install is admitted here.
foreach ($name in @('Get-LifecycleService', 'Assert-LifecycleAgentAbsent', 'Invoke-UninstallAndAssertClean', 'Stop-LifecycleServer')) {
    $functions = @($ast.FindAll({ param($node)
        $node -is [System.Management.Automation.Language.FunctionDefinitionAst] -and $node.Name -eq $name
    }, $true))
    if ($functions.Count -ne 1) { throw "Expected one lifecycle function: $name" }
    Invoke-Expression $functions[0].Extent.Text
}
function Get-Service {
    [CmdletBinding()]
    param([string]$Name)
    if ($Name -ne 'PulseAgent') { throw 'Different service selected' }
    $script:reads++
    if ($script:scenario -eq 'present') {
        $controller = [PSCustomObject]@{}
        $controller | Add-Member ScriptMethod Dispose { $script:disposed = $true }
        return $controller
    }
    $id = 'NoServiceFoundForGivenName'
    $category = [System.Management.Automation.ErrorCategory]::ObjectNotFound
    if ($script:scenario -eq 'read-denied') { $category = [System.Management.Automation.ErrorCategory]::PermissionDenied }
    if ($script:scenario -eq 'wrong-error-id') { $id = 'OtherNotFound' }
    if ($script:scenario -eq 'wrong-category') { $category = [System.Management.Automation.ErrorCategory]::InvalidOperation }
    $record = [System.Management.Automation.ErrorRecord]::new(
        [System.InvalidOperationException]::new('test-only SCM result'), $id, $category, $Name)
    $PSCmdlet.ThrowTerminatingError($record)
}
function Test-Path {
    param([string]$Path)
    if ($script:scenario -eq 'binary-present' -and $Path -eq "$env:ProgramFiles\Pulse\pulse-agent.exe") { return $true }
    return $script:scenario -eq 'state-present' -and $Path -eq $stateDir
}
function Get-NetTCPConnection {
    [CmdletBinding()]
    param()
    if ($script:scenario -eq 'listener-read-denied') { throw 'test-only listener read denied' }
    if ($script:scenario -eq 'listener-present') { return [PSCustomObject]@{ LocalPort = 9191; State = 'Listen' } }
    # Unrelated listeners and connected sockets do not prevent uninstall.
    return @([PSCustomObject]@{ LocalPort = 80; State = 'Listen' },
        [PSCustomObject]@{ LocalPort = 9191; State = 'Established' })
}
function Invoke-Installer {
    param([string]$Arguments, [string]$Label)
    if ($Arguments -ne '-Uninstall $true -NonInteractive $true') { throw 'Unexpected installer arguments' }
    $script:labels.Add($Label)
}
$scenarios = @('absent', 'present', 'read-denied', 'wrong-error-id', 'wrong-category',
    'binary-present', 'state-present', 'listener-read-denied', 'listener-present')
foreach ($script:scenario in $scenarios) {
    $script:reads = 0
    $script:disposed = $false
    $failure = $null
    try { Assert-LifecycleAgentAbsent } catch { $failure = $_ }
    $success = $script:scenario -eq 'absent'
    if (($null -eq $failure) -ne $success -or $script:reads -ne 1) {
        throw "$script:scenario accepted uncertain state or did not independently query SCM: $failure"
    }
    if ($script:scenario -eq 'present' -and -not $script:disposed) { throw 'Controller handle not disposed' }
    Write-Output "PASS $script:scenario"
}
$script:scenario = 'absent'
$script:reads = 0
$script:labels = [System.Collections.Generic.List[string]]::new()
Invoke-UninstallAndAssertClean
if (($script:labels -join ',') -ne 'uninstall,repeated uninstall' -or $script:reads -ne 2) {
    throw 'Both uninstall attempts require their own independent SCM readback'
}
Write-Output 'PASS repeated uninstall with independent absence after each attempt'
function Stop-Process {
    [CmdletBinding()]
    param([int]$Id, [switch]$Force)
    if ($Id -ne 42 -or -not $Force) { throw 'Wrong owned process selected' }
    $script:stops++
    if ($script:scenario -eq 'server-stop-denied') { throw 'test-only stop denied' }
}
foreach ($script:scenario in @('server-already-exited', 'server-stopped', 'server-stop-denied', 'server-stop-timeout')) {
    $script:stops = 0
    $script:disposed = $false
    $script:serverProcess = [PSCustomObject]@{ Id = 42; HasExited = ($script:scenario -eq 'server-already-exited') }
    $script:serverProcess | Add-Member ScriptMethod WaitForExit {
        param([int]$Timeout)
        if ($Timeout -ne 5000) { throw 'Unbounded server cleanup' }
        return $script:scenario -ne 'server-stop-timeout'
    }
    $script:serverProcess | Add-Member ScriptMethod Dispose { $script:disposed = $true }
    $failure = $null
    try { Stop-LifecycleServer } catch { $failure = $_ }
    $success = $script:scenario -in @('server-already-exited', 'server-stopped')
    if (($null -eq $failure) -ne $success -or $script:disposed -ne $success -or
        ($null -eq $script:serverProcess) -ne $success) {
        throw "$script:scenario lost owned cleanup uncertainty: $failure"
    }
    $expectedStops = 1
    if ($script:scenario -eq 'server-already-exited') { $expectedStops = 0 }
    if ($script:stops -ne $expectedStops) { throw 'Wrong cleanup stop count' }
    Write-Output "PASS $script:scenario"
}
Write-Output 'PASS all 14 lifecycle observation controls; no actual SCM or file mutation' 
