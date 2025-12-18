# Pulse Host Agent Installation Script for Windows
#
# ┌─────────────────────────────────────────────────────────────────────────────┐
# │  DEPRECATED: This script installs the legacy pulse-host-agent.              │
# │  Please use the unified agent instead:                                      │
# │                                                                             │
# │    $env:PULSE_URL="http://pulse-server:7655"                                │
# │    $env:PULSE_TOKEN="your-token"                                            │
# │    irm http://pulse-server:7655/install.ps1 | iex                           │
# │                                                                             │
# │  The unified agent provides:                                                │
# │    - Combined host + Docker monitoring in one binary                        │
# │    - Automatic updates                                                      │
# │    - Simplified management                                                  │
# └─────────────────────────────────────────────────────────────────────────────┘
#
# Usage:
#   iwr -useb http://pulse-server:7655/install-host-agent.ps1 | iex
#   OR with parameters:
#   $url = "http://pulse-server:7655"; $token = "your-token"; iwr -useb "$url/install-host-agent.ps1" | iex
#
# Parameters can be passed via environment variables or script parameters

param(
    [string]$PulseUrl = $env:PULSE_URL,
    [string]$Token = $env:PULSE_TOKEN,
    [string]$Interval = $env:PULSE_INTERVAL,
    [string]$AgentId = $env:PULSE_AGENT_ID,
    [string]$InstallPath = "C:\Program Files\Pulse",
    [string]$Arch = $env:PULSE_ARCH,
    [switch]$NoService
)

$ErrorActionPreference = "Stop"

function Write-PulseMessage {
    param(
        [string]$Label,
        [string]$Message,
        [ConsoleColor]$Color
    )

    if ($Label) {
        Write-Host ("[{0}] {1}" -f $Label, $Message) -ForegroundColor $Color
    } else {
        Write-Host $Message -ForegroundColor $Color
    }
}

function PulseSuccess { param([string]$msg) Write-PulseMessage -Label 'OK' -Message $msg -Color 'Green' }
function PulseError { param([string]$msg) Write-PulseMessage -Label 'FAIL' -Message $msg -Color 'Red' }
function PulseInfo { param([string]$msg) Write-PulseMessage -Label 'INFO' -Message $msg -Color 'Cyan' }
function PulseWarn { param([string]$msg) Write-PulseMessage -Label 'WARN' -Message $msg -Color 'Yellow' }

function Write-InstallerEvent {
    param(
        [string]$SourceName,
        [string]$Message,
        [ValidateSet('Information', 'Warning', 'Error')] [string]$EntryType = 'Information',
        [int]$EventId = 1000
    )

    if (-not $SourceName) { return }

    try {
        Write-EventLog -LogName Application -Source $SourceName -EventId $EventId -EntryType $EntryType -Message $Message
    } catch {
        PulseWarn "Unable to write installer event log entry: $_"
    }
}

try {
    [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12 -bor [Net.SecurityProtocolType]::Tls13
} catch {
    # Ignore if platform does not expose TLS 1.3
}

function Get-RecentAgentEvents {
    param(
        [string]$ProviderName,
        [int]$Max = 5
    )
    try {
        return Get-WinEvent -FilterHashtable @{ LogName = 'Application'; ProviderName = $ProviderName } -MaxEvents $Max -ErrorAction Stop
    } catch {
        return Get-EventLog -LogName Application -Source $ProviderName -Newest $Max -ErrorAction SilentlyContinue
    }
}

function Test-AgentRegistration {
    param(
        [string]$PulseUrl,
        [string]$Hostname,
        [string]$Token
    )

    if (-not $Token) {
        return $null
    }

    try {
        $encodedHostname = [System.Uri]::EscapeDataString($Hostname)
        $lookupUri = "$PulseUrl/api/agents/host/lookup?hostname=$encodedHostname"
        $headers = @{ Authorization = "Bearer $Token" }
        $response = Invoke-RestMethod -Uri $lookupUri -Headers $headers -Method Get -ErrorAction Stop
        return $response.host
    } catch {
        return $null
    }
}

function Resolve-PulseArchitectureCandidate {
    param(
        [string]$CandidateValue,
        [string]$Source
    )

    if ([string]::IsNullOrWhiteSpace($CandidateValue)) {
        return $null
    }

    $normalized = $CandidateValue.Trim()
    $upper = $normalized.ToUpperInvariant()

    $mapping = $null
    if ($upper -match 'ARM64|AARCH64') {
        $mapping = @{ OsLabel = 'Arm64'; DownloadArch = 'arm64' }
    } elseif ($upper -match 'AMD64|X64|64-BIT') {
        $mapping = @{ OsLabel = 'X64'; DownloadArch = 'amd64' }
    } elseif ($upper -match 'X86|I386|IA32|32-BIT') {
        $mapping = @{ OsLabel = 'X86'; DownloadArch = '386' }
    }

    if (-not $mapping) {
        return $null
    }

    return [pscustomobject]@{
        OsLabel       = $mapping.OsLabel
        DownloadArch  = $mapping.DownloadArch
        RawValue      = $normalized
        Source        = $Source
        UsedHeuristic = $false
        ObservedValues = @()
    }
}

function Get-PulseArchitecture {
    param(
        [string]$OverrideArch
    )

    $observations = @()

    $candidateSources = @()
    if ($OverrideArch) {
        $candidateSources += @{ Name = 'Override'; Getter = { $OverrideArch } }
    }

    $candidateSources += @(
        @{ Name = 'RuntimeInformation.OSArchitecture'; Getter = { [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture } },
        @{ Name = 'mscorlib.RuntimeInformation.OSArchitecture'; Getter = { [System.Runtime.InteropServices.RuntimeInformation,mscorlib]::OSArchitecture } },
        @{ Name = 'Win32_OperatingSystem (CIM)'; Getter = { (Get-CimInstance -ClassName Win32_OperatingSystem -Property OSArchitecture -ErrorAction Stop).OSArchitecture } },
        @{ Name = 'Win32_OperatingSystem (WMI)'; Getter = { (Get-WmiObject -Class Win32_OperatingSystem -ErrorAction Stop).OSArchitecture } },
        @{ Name = 'PROCESSOR_ARCHITEW6432'; Getter = { $env:PROCESSOR_ARCHITEW6432 } },
        @{ Name = 'PROCESSOR_ARCHITECTURE'; Getter = { $env:PROCESSOR_ARCHITECTURE } },
        @{ Name = 'PROCESSOR_IDENTIFIER'; Getter = { $env:PROCESSOR_IDENTIFIER } }
    )

    foreach ($source in $candidateSources) {
        try {
            $value = & $source.Getter
        } catch {
            continue
        }

        if ([string]::IsNullOrWhiteSpace($value)) {
            continue
        }

        $valueString = $value.ToString().Trim()
        $observations += [pscustomobject]@{ Source = $source.Name; Value = $valueString }

        $resolved = Resolve-PulseArchitectureCandidate -CandidateValue $valueString -Source $source.Name
        if ($resolved) {
            $resolved.ObservedValues = $observations
            return $resolved
        }
    }

    $armHint = $observations | Where-Object { $_.Value -match 'ARM' } | Select-Object -First 1
    if ($armHint) {
        return [pscustomobject]@{
            OsLabel       = 'Arm64'
            DownloadArch  = 'arm64'
            RawValue      = $armHint.Value
            Source        = "$($armHint.Source) heuristic"
            UsedHeuristic = $true
            ObservedValues = $observations
        }
    }

    $is64BitOS = $null
    try { $is64BitOS = [Environment]::Is64BitOperatingSystem } catch { }
    if ($is64BitOS -eq $null) {
        $is64BitOS = [System.IntPtr]::Size -ge 8
    }

    if ($is64BitOS -eq $true) {
        return [pscustomobject]@{
            OsLabel        = 'X64'
            DownloadArch   = 'amd64'
            RawValue       = 'Environment.Is64BitOperatingSystem=True'
            Source         = 'Environment heuristic'
            UsedHeuristic  = $true
            ObservedValues = $observations
        }
    }

    if ($is64BitOS -eq $false) {
        return [pscustomobject]@{
            OsLabel        = 'X86'
            DownloadArch   = '386'
            RawValue       = 'Environment.Is64BitOperatingSystem=False'
            Source         = 'Environment heuristic'
            UsedHeuristic  = $true
            ObservedValues = $observations
        }
    }

    return $null
}

Write-Host ""
$banner = "=" * 59
Write-Host $banner -ForegroundColor Cyan
Write-Host "  Pulse Host Agent - Windows Installation" -ForegroundColor Cyan
Write-Host $banner -ForegroundColor Cyan
Write-Host ""

# Check if running as Administrator
$isAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
    PulseError "This script must be run as Administrator"
    PulseInfo "Right-click PowerShell and select 'Run as Administrator'"
    exit 1
}

# Interactive prompts if parameters not provided
if (-not $PulseUrl) {
    $PulseUrl = Read-Host "Enter Pulse server URL (e.g., http://pulse.example.com:7655)"
}
$PulseUrl = $PulseUrl.TrimEnd('/')

if (-not $Token) {
    PulseWarn "No API token provided - agent will attempt to connect without authentication"
    $response = Read-Host "Continue without token? (y/N)"
    if ($response -ne 'y' -and $response -ne 'Y') {
        $Token = Read-Host "Enter API token"
    }
}

if (-not $Interval) {
    $Interval = "30s"
}

PulseInfo "Configuration:"
Write-Host "  Pulse URL: $PulseUrl"
Write-Host "  Token: $(if ($Token) { '***' + $Token.Substring([Math]::Max(0, $Token.Length - 4)) } else { 'none' })"
Write-Host "  Agent ID: $(if ($AgentId) { $AgentId } else { 'machine-id (default)' })"
Write-Host "  Interval: $Interval"
Write-Host "  Install Path: $InstallPath"
if ($Arch) {
    Write-Host "  Architecture Override: $Arch"
}
Write-Host ""

# Determine architecture (support both legacy Windows PowerShell and pwsh)
if ($Arch) {
    if (-not (Resolve-PulseArchitectureCandidate -CandidateValue $Arch -Source "Override")) {
        PulseError "Invalid architecture override '$Arch'. Supported values: amd64, arm64, 386"
        exit 1
    }
}

$archInfo = Get-PulseArchitecture -OverrideArch $Arch
$observedSummary = $null
if ($archInfo -and $archInfo.ObservedValues) {
    $observedSummary = ($archInfo.ObservedValues | ForEach-Object { "$($_.Source)='$($_.Value)'" }) -join "; "
}

if (-not $archInfo -or -not $archInfo.DownloadArch) {
    PulseError "Unable to determine operating system architecture"
    if ($observedSummary) {
        PulseInfo "Observed architecture values: $observedSummary"
    }
    PulseInfo "Specify -Arch amd64|arm64|386 or set PULSE_ARCH to override detection."
    exit 1
}

$osArch = $archInfo.OsLabel
$arch = $archInfo.DownloadArch
$rawArchValue = $archInfo.RawValue
$archSource = if ($archInfo.Source) { $archInfo.Source } else { "auto-detected" }
$downloadUrl = "$PulseUrl/download/pulse-host-agent?platform=windows&arch=$arch"

PulseInfo "System Information:"
if ($rawArchValue -and $rawArchValue -ne $osArch) {
    Write-Host "  OS Architecture: $osArch (reported as '$rawArchValue')"
} else {
    Write-Host "  OS Architecture: $osArch"
}
Write-Host "  Detection Source: $archSource"
Write-Host "  Download Architecture: $arch"
Write-Host "  Download URL: $downloadUrl"
Write-Host ""

if ($archInfo.UsedHeuristic) {
    PulseWarn "Architecture detected via fallback ($archSource). If this looks wrong, rerun with -Arch amd64|arm64|386 or set PULSE_ARCH."
    if ($observedSummary) {
        PulseInfo "Observed architecture values: $observedSummary"
    }
}

PulseInfo "Downloading agent binary from $downloadUrl..."
try {
    # Create install directory
    if (-not (Test-Path $InstallPath)) {
        New-Item -ItemType Directory -Path $InstallPath -Force | Out-Null
    }

    $agentPath = Join-Path $InstallPath "pulse-host-agent.exe"

    # Download binary
    Invoke-WebRequest -Uri $downloadUrl -OutFile $agentPath -UseBasicParsing
    PulseSuccess "Downloaded agent to $agentPath"

    # Validate PE header
    $fileBytes = [System.IO.File]::ReadAllBytes($agentPath)
    $fileSizeMB = [math]::Round($fileBytes.Length / 1MB, 2)
    PulseInfo "File size: $fileSizeMB MB ($($fileBytes.Length) bytes)"

    if ($fileBytes.Length -lt 64) {
        throw "Downloaded file is too small ($($fileBytes.Length) bytes) - expected Windows PE executable"
    }

    # Check for MZ signature (PE header)
    if ($fileBytes[0] -ne 0x4D -or $fileBytes[1] -ne 0x5A) {
        $firstBytes = ($fileBytes[0..15] | ForEach-Object { $_.ToString("X2") }) -join " "
        throw "Downloaded file is not a valid Windows executable (missing MZ signature). First bytes: $firstBytes"
    }

    # Get PE header offset (at 0x3C)
    $peOffset = [BitConverter]::ToUInt32($fileBytes, 0x3C)
    if ($peOffset -ge $fileBytes.Length - 6) {
        throw "Invalid PE header offset in downloaded file"
    }

    # Check PE signature
    if ($fileBytes[$peOffset] -ne 0x50 -or $fileBytes[$peOffset+1] -ne 0x45) {
        throw "Downloaded file has invalid PE signature"
    }

    # Check machine type (should be 0x8664 for x64, 0xAA64 for ARM64)
    $machineType = [BitConverter]::ToUInt16($fileBytes, $peOffset + 4)
    $expectedMachine = switch ($arch) {
        'amd64' { 0x8664 }
        'arm64' { 0xAA64 }
        '386'   { 0x014C }
        default { 0x0000 }
    }

    if ($machineType -ne $expectedMachine) {
        $machineStr = "0x" + $machineType.ToString("X4")
        $expectedStr = "0x" + $expectedMachine.ToString("X4")
        throw "Downloaded binary is for wrong architecture (got $machineStr, expected $expectedStr for $arch)"
    }

    PulseSuccess "Verified PE executable for $osArch architecture"

    # Verify checksum
    PulseInfo "Verifying checksum..."
    $checksumUrl = "$PulseUrl/download/pulse-host-agent.sha256?platform=windows&arch=$arch"
    try {
        $expectedChecksum = (Invoke-WebRequest -Uri $checksumUrl -UseBasicParsing).Content.Trim().Split()[0]
        $actualChecksum = (Get-FileHash -Path $agentPath -Algorithm SHA256).Hash.ToLower()

        if ($actualChecksum -ne $expectedChecksum.ToLower()) {
            throw "Checksum mismatch! Expected: $expectedChecksum, Got: $actualChecksum"
        }
        PulseSuccess "Checksum verified: $actualChecksum"
    } catch {
        PulseWarn "Could not verify checksum: $_"
        PulseInfo "Continuing anyway (PE header was validated)"
    }

    $agentArgs = @("--url", "`"$PulseUrl`"", "--interval", $Interval)
    if ($Token) {
        $agentArgs += @("--token", "`"$Token`"")
    }
    if ($AgentId) {
        $agentArgs += @("--agent-id", "`"$AgentId`"")
    }
    $serviceBinaryPath = "`"$agentPath`" $($agentArgs -join ' ')"
    $manualCommand = "& `"$agentPath`" $($agentArgs -join ' ')"
} catch {
    PulseError "Failed to download agent: $_"
    
    # Try to show a popup if running in a GUI environment
    try {
        Add-Type -AssemblyName System.Windows.Forms
        [System.Windows.Forms.MessageBox]::Show("Failed to download agent.`n`n$_", "Pulse Installation Failed", [System.Windows.Forms.MessageBoxButtons]::OK, [System.Windows.Forms.MessageBoxIcon]::Error) | Out-Null
    } catch {
        # Ignore if GUI not available
    }

    Write-Host ""
    Write-Host "Press Enter to exit..." -ForegroundColor Yellow
    Read-Host
    exit 1
}

# Create configuration
$configPath = Join-Path $InstallPath "config.json"
$config = @{
    url = $PulseUrl
    interval = $Interval
}
if ($Token) {
    $config.token = $Token
}
if ($AgentId) {
    $config.agentId = $AgentId
}

$config | ConvertTo-Json | Set-Content $configPath
PulseSuccess "Created configuration at $configPath"

# Stop existing service if running
$serviceName = "PulseHostAgent"
$existingService = Get-Service -Name $serviceName -ErrorAction SilentlyContinue
if ($existingService) {
    PulseInfo "Stopping existing service..."
    Stop-Service -Name $serviceName -Force
    PulseSuccess "Stopped existing service"
}

if (-not $NoService) {
    PulseInfo "Installing native Windows service with built-in service support..."

    try {
        if ($existingService) {
            PulseInfo "Removing existing service..."
            sc.exe delete $serviceName | Out-Null
            Start-Sleep -Seconds 2
        }

        # Create the service using New-Service
        New-Service -Name $serviceName `
                    -BinaryPathName $serviceBinaryPath `
                    -DisplayName "Pulse Host Agent" `
                    -Description "Monitors system metrics and reports to Pulse monitoring server" `
                    -StartupType Automatic | Out-Null

        PulseSuccess "Created Windows service '$serviceName'"

        # Register Windows Event Log source
        try {
            if (-not ([System.Diagnostics.EventLog]::SourceExists($serviceName))) {
                New-EventLog -LogName Application -Source $serviceName
                PulseSuccess "Registered Event Log source"
            }
        } catch {
            PulseWarn "Could not register Event Log source (not critical): $_"
        }

        Write-InstallerEvent -SourceName $serviceName -Message "Pulse Host Agent installer registered service version $(Get-Item $agentPath).VersionInfo.FileVersion" -EventId 1000

        # Configure service recovery options (restart on failure)
        sc.exe failure $serviceName reset= 86400 actions= restart/60000/restart/60000/restart/60000 | Out-Null
        PulseSuccess "Configured automatic restart on failure"

        # Start the service
        PulseInfo "Starting service..."
        Start-Service -Name $serviceName
        Start-Sleep -Seconds 3

        $status = (Get-Service -Name $serviceName).Status
        if ($status -eq 'Running') {
            PulseSuccess "Service started successfully!"

            PulseInfo "Waiting 10 seconds to validate agent reporting..."
            Start-Sleep -Seconds 10

            $hostname = $env:COMPUTERNAME
            $lookupHost = Test-AgentRegistration -PulseUrl $PulseUrl -Hostname $hostname -Token $Token
            if ($lookupHost) {
                PulseSuccess "Agent successfully registered with Pulse (host '$hostname')."
                if ($lookupHost.status) {
                    $lastSeen = $lookupHost.lastSeen
                    if ($lastSeen -is [DateTime]) {
                        $lastSeen = $lastSeen.ToString("u")
                    }
                    PulseInfo ("Pulse reports status: {0} (last seen {1})" -f $lookupHost.status, $lastSeen)
                }
                PulseInfo "Check your Pulse dashboard - this host should appear shortly."
                $statusForLog = if ($lookupHost.status) { $lookupHost.status } else { 'unknown' }
                Write-InstallerEvent -SourceName $serviceName -Message "Installer verified host '$hostname' reporting to Pulse (status: $statusForLog)." -EventId 1010
            } elseif ($Token) {
                PulseWarn "Agent is running but the lookup endpoint has not confirmed registration yet."
                PulseInfo "It may take another moment for metrics to appear in the dashboard."
                Write-InstallerEvent -SourceName $serviceName -Message "Installer could not yet confirm host '$hostname' registration with Pulse." -EntryType Warning -EventId 1011
            } else {
                PulseInfo "Registration check skipped (no API token available)."
                Write-InstallerEvent -SourceName $serviceName -Message "Installer skipped registration lookup (no API token provided)." -EventId 1012
            }

            $recentLogs = Get-RecentAgentEvents -ProviderName $serviceName -Max 5
            if ($recentLogs) {
                PulseInfo "Recent service events:"
                $recentLogs | Select-Object -First 3 | ForEach-Object {
                    $time = $_.TimeCreated
                    if (-not $time) { $time = $_.TimeGenerated }
                    Write-Host ("    [{0}] {1}" -f $time.ToString("u"), $_.Message)
                }
            } else {
                PulseWarn "No recent Application log entries were found for $serviceName."
            }
        } else {
            PulseWarn "Service status: $status"
            PulseInfo "Checking service logs..."
            $recentLogs = Get-RecentAgentEvents -ProviderName $serviceName -Max 5
            if ($recentLogs) {
                $recentLogs | ForEach-Object {
                    $time = $_.TimeCreated
                    if (-not $time) { $time = $_.TimeGenerated }
                    Write-Host ("    [{0}] {1}" -f $time.ToString("u"), $_.Message)
                }
            } else {
                PulseWarn "No Application log entries were found for $serviceName."
            }
        }

    } catch {
        PulseError "Failed to create/start service: $_"
        PulseInfo "You can start the agent manually with:"
        Write-Host "  $manualCommand"
        Write-Host ""
        PulseInfo "Or check Windows Event Viewer (Application log) for error details."
        exit 1
    }
} else {
    PulseInfo "Skipping service installation (--NoService flag)"
    Write-Host ""
    PulseInfo "To start the agent manually:"
    Write-Host "  $manualCommand"
}

Write-Host ""
$successBanner = "=" * 59
Write-Host $successBanner -ForegroundColor Green
PulseSuccess "Installation complete!"
Write-Host $successBanner -ForegroundColor Green
Write-Host ""

PulseInfo "Service Management Commands:"
Write-Host "  Start:   Start-Service -Name PulseHostAgent"
Write-Host "  Stop:    Stop-Service -Name PulseHostAgent"
Write-Host "  Restart: Restart-Service -Name PulseHostAgent"
Write-Host "  Status:  Get-Service -Name PulseHostAgent | Select Status, StartType"
Write-Host "  Remove:  sc.exe delete PulseHostAgent"
Write-Host "  Logs:    Get-WinEvent -FilterHashtable @{LogName='Application'; ProviderName='PulseHostAgent'} -MaxEvents 50"
Write-Host ""

PulseInfo "Files installed:"
Write-Host "  Binary: $agentPath"
Write-Host "  Config: $configPath"
Write-Host ""

PulseInfo "The agent is now reporting to: $PulseUrl"
Write-Host ""
