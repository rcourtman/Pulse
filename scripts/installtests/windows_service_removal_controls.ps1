param([Parameter(Mandatory = $true)][string]$InstallerPath)

$ErrorActionPreference = 'Stop'
$AgentName = 'PulseAgent'

# Real Get-Service loads this assembly before returning a ServiceController.
# Our SCM double must supply the same enum without making a real service read.
Add-Type -AssemblyName System.ServiceProcess

# Load only the production boundary. No administrator check, download, real
# SCM call or installer mutation runs in this failure-control harness.
$errors = $null
$ast = [System.Management.Automation.Language.Parser]::ParseFile(
    $InstallerPath, [ref]$null, [ref]$errors)
if ($errors.Count) { throw "Installer parse failed: $errors" }
foreach ($name in @('Get-PulseService', 'Remove-PulseService')) {
    $functions = @($ast.FindAll({ param($node)
        $node -is [System.Management.Automation.Language.FunctionDefinitionAst] -and $node.Name -eq $name
    }, $true))
    if ($functions.Count -ne 1) { throw "Expected one production function: $name" }
    Invoke-Expression $functions[0].Extent.Text
}

function Assert-Control {
    param([bool]$Condition, [string]$Message)
    if (-not $Condition) { throw "$script:scenario : $Message; events=$($script:events -join ',')" }
}

function Get-Service {
    [CmdletBinding()]
    param([string]$Name)
    Assert-Control ($Name -eq 'PulseAgent') 'queried a different service'
    $script:events.Add('read')
    if ($script:scenario -eq 'read-denied' -or
        ($script:deleted -and $script:scenario -eq 'read-denied-after-delete')) {
        throw 'SCM access denied'
    }
    $absent = $script:scenario -eq 'absent' -or
        ($script:deleted -and $script:scenario -ne 'delete-pending' -and
         ($script:scenario -ne 'delete-delayed' -or $script:afterDeleteReads -ge 1))
    if ($absent -or $script:scenario -eq 'wrong-not-found-category') {
        $category = [System.Management.Automation.ErrorCategory]::ObjectNotFound
        if ($script:scenario -eq 'wrong-not-found-category') {
            $category = [System.Management.Automation.ErrorCategory]::PermissionDenied
        }
        $record = [System.Management.Automation.ErrorRecord]::new(
            [System.InvalidOperationException]::new('test-only SCM result'),
            'NoServiceFoundForGivenName', $category, $Name)
        $PSCmdlet.ThrowTerminatingError($record)
    }
    if ($script:deleted) { $script:afterDeleteReads++ }
    return $script:controller
}

function sc.exe {
    param([string]$Verb, [string]$Name)
    Assert-Control ($Verb -eq 'delete' -and $Name -eq 'PulseAgent') 'unexpected native operation'
    Assert-Control ($script:controller.Status -eq 'Stopped') 'delete before confirmed stop'
    Assert-Control ($script:events.Contains('dispose')) 'retained controller handle before delete'
    $script:events.Add('delete')
    $global:LASTEXITCODE = 0
    if ($script:scenario -eq 'delete-refused') { $global:LASTEXITCODE = 5 }
    $script:deleted = $global:LASTEXITCODE -eq 0
    return 'test-only sc.exe output'
}

function Start-Sleep {
    param([int]$Milliseconds)
    Assert-Control ($Milliseconds -eq 200) 'unexpected poll interval'
    $script:events.Add('sleep')
}

$scenarios = @('absent', 'stopped', 'running', 'stop-pending', 'read-denied',
    'wrong-not-found-category', 'stop-refused', 'stop-timeout', 'stop-not-observed',
    'delete-refused', 'read-denied-after-delete', 'delete-delayed', 'delete-pending')
foreach ($script:scenario in $scenarios) {
    $script:events = [System.Collections.Generic.List[string]]::new()
    $script:deleted = $false
    $script:afterDeleteReads = 0
    $global:LASTEXITCODE = 0
    $status = 'Running'
    if ($script:scenario -eq 'stopped') { $status = 'Stopped' }
    if ($script:scenario -eq 'stop-pending') { $status = 'StopPending' }
    $script:controller = [PSCustomObject]@{ Status = $status }
    $script:controller | Add-Member ScriptMethod Refresh {
        $script:events.Add('refresh')
    }
    $script:controller | Add-Member ScriptMethod Stop {
        $script:events.Add('stop')
        if ($script:scenario -eq 'stop-refused') { throw 'test-only stop refusal' }
        $this.Status = 'StopPending'
    }
    $script:controller | Add-Member ScriptMethod WaitForStatus {
        param($ExpectedStatus, [TimeSpan]$Timeout)
        $script:events.Add('wait')
        Assert-Control ($ExpectedStatus -is [System.ServiceProcess.ServiceControllerStatus]) 'wait did not receive the real service-status enum'
        Assert-Control ($ExpectedStatus.ToString() -eq 'Stopped') 'waited for a non-stopped state'
        $expectedTimeout = 30
        if ($script:scenario -eq 'delete-pending') { $expectedTimeout = 0 }
        Assert-Control ($Timeout.TotalSeconds -eq $expectedTimeout) 'stop wait is not bounded'
        if ($script:scenario -eq 'stop-timeout') { throw 'test-only stop timeout' }
        if ($script:scenario -ne 'stop-not-observed') { $this.Status = 'Stopped' }
    }
    $script:controller | Add-Member ScriptMethod Dispose {
        $script:events.Add('dispose')
    }

    $failure = $null
    try {
        if ($script:scenario -eq 'delete-pending') {
            Remove-PulseService -TimeoutSeconds 0
        } else {
            Remove-PulseService
        }
        # Model the caller's destructive continuation, not SCM implementation.
        $script:events.Add('binary-mutation')
        $script:events.Add('state-mutation')
        $script:events.Add('remote-deregistration')
    } catch {
        $failure = $_
    }
    $success = $script:scenario -in @('absent', 'stopped', 'running', 'stop-pending', 'delete-delayed')
    Assert-Control (($null -eq $failure) -eq $success) "unexpected outcome: $failure"
    foreach ($mutation in @('binary-mutation', 'state-mutation', 'remote-deregistration')) {
        Assert-Control ($script:events.Contains($mutation) -eq $success) "incorrect continuation: $mutation"
    }
    if ($script:scenario -eq 'absent') {
        Assert-Control ($script:events.Count -eq 4) 'absent service caused SCM mutation'
    } elseif ($script:scenario -notin @('read-denied', 'wrong-not-found-category')) {
        Assert-Control ($script:events.Contains('dispose')) 'controller not disposed on exit'
    }
    if ($script:scenario -in @('stopped', 'stop-pending')) {
        Assert-Control (-not $script:events.Contains('stop')) 'stop was unnecessarily re-requested'
    }
    if ($script:scenario -in @('stop-refused', 'stop-timeout', 'stop-not-observed', 'read-denied', 'wrong-not-found-category')) {
        Assert-Control (-not $script:events.Contains('delete')) 'failed stop/read still deleted service'
    }
    if ($script:scenario -eq 'delete-delayed') {
        Assert-Control ($script:events.Contains('sleep')) 'did not observe delayed SCM deletion'
    }
    Write-Output "PASS $script:scenario : $($script:events -join ',')"
}
Write-Output "PASS all $($scenarios.Count) production service-removal controls; no real SCM or file mutation"
