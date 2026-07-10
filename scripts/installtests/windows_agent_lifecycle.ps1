[CmdletBinding()]
param (
    [ValidateSet('Full', 'InstallUpdate', 'PostRebootUninstall')]
    [string]$Phase = 'Full',
    [Parameter(Mandatory = $true)]
    [string]$ServerBinary,
    [string]$AgentV1,
    [Parameter(Mandatory = $true)]
    [string]$AgentV2,
    [Parameter(Mandatory = $true)]
    [string]$InstallerPath,
    [int]$Port = 17655,
    [switch]$ConfirmLifecycleMutation
)

$ErrorActionPreference = 'Stop'
$serviceName = 'PulseAgent'
$stateDir = Join-Path $env:ProgramData 'Pulse'
$logFile = Join-Path $stateDir 'pulse-agent.log'
$proofStatePath = Join-Path $stateDir 'windows-lifecycle-proof.json'
$baseUrl = "http://127.0.0.1:$Port"
$serverProcess = $null
$previousDisableAutoUpdate = $null
$restoreAutoUpdateAtExit = $true

function Assert-Administrator {
    $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = [Security.Principal.WindowsPrincipal]$identity
    if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
        throw 'Windows lifecycle proof must run from an elevated PowerShell session.'
    }
}

function Resolve-RequiredPath {
    param([string]$Path, [string]$Label)
    if ([string]::IsNullOrWhiteSpace($Path) -or -not (Test-Path $Path -PathType Leaf)) {
        throw "$Label does not exist: $Path"
    }
    return (Resolve-Path $Path).Path
}

function Stop-LifecycleServer {
    if ($null -ne $script:serverProcess -and -not $script:serverProcess.HasExited) {
        Stop-Process -Id $script:serverProcess.Id -Force -ErrorAction SilentlyContinue
        $script:serverProcess.WaitForExit(5000) | Out-Null
    }
    $script:serverProcess = $null
}

function Start-LifecycleServer {
    param([string]$AgentPath)
    Stop-LifecycleServer
    $version = ((& $AgentPath --version 2>$null) | Select-Object -First 1).Trim()
    if ([string]::IsNullOrWhiteSpace($version)) {
        throw "Could not read agent version from $AgentPath"
    }
    $arguments = @(
        '--listen', "127.0.0.1:$Port",
        '--agent-binary', ('"{0}"' -f $AgentPath),
        '--version', $version
    )
    $script:serverProcess = Start-Process -FilePath $script:resolvedServerBinary -ArgumentList $arguments -PassThru -NoNewWindow
    for ($i = 0; $i -lt 30; $i++) {
        if ($script:serverProcess.HasExited) {
            throw "Lifecycle server exited with code $($script:serverProcess.ExitCode)."
        }
        try {
            $response = Invoke-RestMethod -Uri "$baseUrl/api/version" -TimeoutSec 2
            if ($response.version -eq $version) {
                return $version
            }
        } catch {
        }
        Start-Sleep -Milliseconds 500
    }
    throw "Lifecycle server did not become ready at $baseUrl."
}

function Invoke-Installer {
    param([string]$Arguments, [string]$Label)
    $escapedInstaller = $script:resolvedInstallerPath.Replace("'", "''")
    $command = "& '$escapedInstaller' $Arguments"
    & $script:powerShellPath -NoLogo -NoProfile -NonInteractive -ExecutionPolicy Bypass -Command $command
    if ($LASTEXITCODE -ne 0) {
        throw "$Label failed with exit code $LASTEXITCODE."
    }
}

function Wait-AgentReady {
    param([int]$TimeoutSeconds = 45)
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    do {
        try {
            $service = Get-Service -Name $serviceName -ErrorAction Stop
            $ready = Invoke-RestMethod -Uri 'http://127.0.0.1:9191/readyz' -TimeoutSec 2
            if ($service.Status -eq 'Running' -and $ready.ready -eq $true) {
                return
            }
        } catch {
        }
        Start-Sleep -Seconds 1
    } while ((Get-Date) -lt $deadline)
    throw 'PulseAgent did not reach local readiness before the timeout.'
}

function Assert-AgentRuntime {
    param([string]$ExpectedVersion)
    Wait-AgentReady
    $service = Get-CimInstance Win32_Service | Where-Object Name -eq $serviceName
    if ($null -eq $service -or $service.State -ne 'Running' -or $service.StartMode -ne 'Auto') {
        throw "Unexpected service state: $($service | ConvertTo-Json -Compress)"
    }
    $installedVersion = ((& "$env:ProgramFiles\Pulse\pulse-agent.exe" --version 2>$null) | Select-Object -First 1).Trim()
    if ($installedVersion -ne $ExpectedVersion) {
        throw "Installed version is $installedVersion; expected $ExpectedVersion."
    }
    if ($service.PathName -notlike '*--log-file*' -or $service.PathName -notlike '*pulse-agent.log*') {
        throw "Service command does not carry the canonical log-file argument: $($service.PathName)"
    }
    if (-not (Test-Path $logFile) -or (Get-Item $logFile).Length -le 0) {
        throw "Agent log file is missing or empty: $logFile"
    }
    $logText = Get-Content $logFile -Raw
    $hasStartupEvent = $logText -like '*Starting Pulse Unified Agent*' -or $logText -like '*Pulse Agent service is running*'
    if (-not $hasStartupEvent -or $logText -notlike "*$ExpectedVersion*") {
        throw "Agent log does not contain startup evidence for $ExpectedVersion."
    }
    $recovery = (& sc.exe qfailure $serviceName 2>&1 | Out-String)
    if ([regex]::Matches($recovery, 'RESTART').Count -lt 3) {
        throw "Service recovery actions are incomplete: $recovery"
    }
    $failureFlag = (& sc.exe qfailureflag $serviceName 2>&1 | Out-String)
    if ($failureFlag -notmatch 'TRUE|1') {
        throw "Service non-crash recovery flag is not enabled: $failureFlag"
    }
    return [uint32]$service.ProcessId
}

function Assert-CrashRecovery {
    param([string]$ExpectedVersion)
    $previousPid = Assert-AgentRuntime -ExpectedVersion $ExpectedVersion
    Stop-Process -Id $previousPid -Force
    $deadline = (Get-Date).AddSeconds(45)
    do {
        Start-Sleep -Seconds 1
        $service = Get-CimInstance Win32_Service | Where-Object Name -eq $serviceName
        if ($null -ne $service -and $service.State -eq 'Running' -and [uint32]$service.ProcessId -ne $previousPid) {
            try {
                Wait-AgentReady -TimeoutSeconds 5
                return
            } catch {
            }
        }
    } while ((Get-Date) -lt $deadline)
    throw 'PulseAgent did not recover after its process was terminated.'
}

function Invoke-UninstallAndAssertClean {
    Invoke-Installer -Label 'uninstall' -Arguments '-Uninstall $true -NonInteractive $true'
    Start-Sleep -Seconds 2
    if ($null -ne (Get-Service $serviceName -ErrorAction SilentlyContinue)) {
        throw 'PulseAgent service still exists after uninstall.'
    }
    if (Test-Path "$env:ProgramFiles\Pulse\pulse-agent.exe") {
        throw 'Pulse Agent binary still exists after uninstall.'
    }
    if (Test-Path $stateDir) {
        throw 'Pulse Agent state directory still exists after uninstall.'
    }
    if (Get-NetTCPConnection -LocalPort 9191 -State Listen -ErrorAction SilentlyContinue) {
        throw 'Pulse Agent readiness listener still exists after uninstall.'
    }
}

Assert-Administrator
if (-not $ConfirmLifecycleMutation) {
    throw 'Pass -ConfirmLifecycleMutation to acknowledge service installation, process termination, and uninstall on this dedicated Windows runner.'
}

$resolvedServerBinary = Resolve-RequiredPath $ServerBinary 'Lifecycle server binary'
$resolvedAgentV2 = Resolve-RequiredPath $AgentV2 'Version-two agent binary'
$resolvedInstallerPath = Resolve-RequiredPath $InstallerPath 'Windows installer'
$resolvedAgentV1 = $null
if ($Phase -ne 'PostRebootUninstall') {
    $resolvedAgentV1 = Resolve-RequiredPath $AgentV1 'Version-one agent binary'
}
$powerShellPath = (Get-Process -Id $PID).Path

try {
    if ($Phase -eq 'PostRebootUninstall') {
        $proofState = Get-Content $proofStatePath -Raw | ConvertFrom-Json
        $previousDisableAutoUpdate = $proofState.previousDisableAutoUpdate
        $restoreAutoUpdateAtExit = $true
        $versionV2 = Start-LifecycleServer -AgentPath $resolvedAgentV2
        Assert-AgentRuntime -ExpectedVersion $versionV2 | Out-Null
        Invoke-UninstallAndAssertClean
        Write-Host 'Post-reboot persistence and uninstall proof passed.' -ForegroundColor Green
        return
    }

    if (Get-Service $serviceName -ErrorAction SilentlyContinue) {
        throw 'PulseAgent is already installed. Use a disposable runner or uninstall it before starting this proof.'
    }
    if (Test-Path $stateDir) {
        throw "Pulse state already exists at $stateDir. Use a clean disposable runner."
    }

    $previousDisableAutoUpdate = [Environment]::GetEnvironmentVariable('PULSE_DISABLE_AUTO_UPDATE', 'Machine')
    [Environment]::SetEnvironmentVariable('PULSE_DISABLE_AUTO_UPDATE', 'true', 'Machine')

    $versionV1 = Start-LifecycleServer -AgentPath $resolvedAgentV1
    Invoke-Installer -Label 'preflight' -Arguments "-Url '$baseUrl' -PreflightOnly `$true -NonInteractive `$true"
    Invoke-Installer -Label 'install' -Arguments "-Url '$baseUrl' -AgentId 'windows-lifecycle-agent' -Hostname 'windows-lifecycle-agent' -EnableHost `$true -EnableDocker `$false -EnableKubernetes `$false -EnableProxmox `$false -EnableCommands `$false -NonInteractive `$true"
    Assert-AgentRuntime -ExpectedVersion $versionV1 | Out-Null

    $versionV2 = Start-LifecycleServer -AgentPath $resolvedAgentV2
    Invoke-Installer -Label 'update' -Arguments "-Url '$baseUrl' -AgentId 'windows-lifecycle-agent' -Hostname 'windows-lifecycle-agent' -EnableHost `$true -EnableDocker `$false -EnableKubernetes `$false -EnableProxmox `$false -EnableCommands `$false -NonInteractive `$true"
    Assert-AgentRuntime -ExpectedVersion $versionV2 | Out-Null
    Assert-CrashRecovery -ExpectedVersion $versionV2

    if ($Phase -eq 'InstallUpdate') {
        [ordered]@{
            expectedVersion = $versionV2
            previousDisableAutoUpdate = $previousDisableAutoUpdate
        } | ConvertTo-Json | Set-Content $proofStatePath -Encoding ascii
        $restoreAutoUpdateAtExit = $false
        Write-Host 'Install, update, logging, and crash-recovery proof passed; reboot the VM and run PostRebootUninstall.' -ForegroundColor Green
        return
    }

    Restart-Service $serviceName -Force
    Assert-AgentRuntime -ExpectedVersion $versionV2 | Out-Null
    Invoke-UninstallAndAssertClean
    Write-Host 'Full Windows service lifecycle proof passed.' -ForegroundColor Green
} finally {
    Stop-LifecycleServer
    if ($restoreAutoUpdateAtExit) {
        [Environment]::SetEnvironmentVariable('PULSE_DISABLE_AUTO_UPDATE', $previousDisableAutoUpdate, 'Machine')
    }
}
