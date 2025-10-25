# Pulse Host Agent Installation Script for Windows
#
# Usage:
#   iwr -useb http://pulse-server:7656/install-host-agent.ps1 | iex
#   OR with parameters:
#   $url = "http://pulse-server:7656"; $token = "your-token"; iwr -useb "$url/install-host-agent.ps1" | iex
#
# Parameters can be passed via environment variables or script parameters

param(
    [string]$PulseUrl = $env:PULSE_URL,
    [string]$Token = $env:PULSE_TOKEN,
    [string]$Interval = $env:PULSE_INTERVAL,
    [string]$InstallPath = "C:\Program Files\Pulse",
    [switch]$NoService
)

$ErrorActionPreference = "Stop"

# ANSI color codes for output
$Red = "`e[31m"
$Green = "`e[32m"
$Yellow = "`e[33m"
$Blue = "`e[34m"
$Reset = "`e[0m"

function Write-Color {
    param([string]$Color, [string]$Message)
    Write-Host "${Color}${Message}${Reset}"
}

function Write-Success { param([string]$msg) Write-Color $Green "✓ $msg" }
function Write-Error { param([string]$msg) Write-Color $Red "✗ $msg" }
function Write-Info { param([string]$msg) Write-Color $Blue "ℹ $msg" }
function Write-Warning { param([string]$msg) Write-Color $Yellow "⚠ $msg" }

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
        Write-Warning "Unable to write installer event log entry: $_"
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

Write-Host ""
Write-Color $Blue "═══════════════════════════════════════════════════════════"
Write-Color $Blue "  Pulse Host Agent - Windows Installation"
Write-Color $Blue "═══════════════════════════════════════════════════════════"
Write-Host ""

# Check if running as Administrator
$isAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
    Write-Error "This script must be run as Administrator"
    Write-Info "Right-click PowerShell and select 'Run as Administrator'"
    exit 1
}

# Interactive prompts if parameters not provided
if (-not $PulseUrl) {
    $PulseUrl = Read-Host "Enter Pulse server URL (e.g., http://pulse.example.com:7656)"
}
$PulseUrl = $PulseUrl.TrimEnd('/')

if (-not $Token) {
    Write-Warning "No API token provided - agent will attempt to connect without authentication"
    $response = Read-Host "Continue without token? (y/N)"
    if ($response -ne 'y' -and $response -ne 'Y') {
        $Token = Read-Host "Enter API token"
    }
}

if (-not $Interval) {
    $Interval = "30s"
}

Write-Info "Configuration:"
Write-Host "  Pulse URL: $PulseUrl"
Write-Host "  Token: $(if ($Token) { '***' + $Token.Substring([Math]::Max(0, $Token.Length - 4)) } else { 'none' })"
Write-Host "  Interval: $Interval"
Write-Host "  Install Path: $InstallPath"
Write-Host ""

# Determine architecture
$arch = if ([Environment]::Is64BitOperatingSystem) { "amd64" } else { "386" }
$downloadUrl = "$PulseUrl/download/pulse-host-agent?platform=windows&arch=$arch"

Write-Info "Downloading agent binary from $downloadUrl..."
try {
    # Create install directory
    if (-not (Test-Path $InstallPath)) {
        New-Item -ItemType Directory -Path $InstallPath -Force | Out-Null
    }

    $agentPath = Join-Path $InstallPath "pulse-host-agent.exe"

    # Download binary
    Invoke-WebRequest -Uri $downloadUrl -OutFile $agentPath -UseBasicParsing
    Write-Success "Downloaded agent to $agentPath"

    $agentArgs = @("--url", "`"$PulseUrl`"", "--interval", $Interval)
    if ($Token) {
        $agentArgs += @("--token", "`"$Token`"")
    }
    $serviceBinaryPath = "`"$agentPath`" $($agentArgs -join ' ')"
    $manualCommand = "& `"$agentPath`" $($agentArgs -join ' ')"
} catch {
    Write-Error "Failed to download agent: $_"
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

$config | ConvertTo-Json | Set-Content $configPath
Write-Success "Created configuration at $configPath"

# Stop existing service if running
$serviceName = "PulseHostAgent"
$existingService = Get-Service -Name $serviceName -ErrorAction SilentlyContinue
if ($existingService) {
    Write-Info "Stopping existing service..."
    Stop-Service -Name $serviceName -Force
    Write-Success "Stopped existing service"
}

if (-not $NoService) {
    Write-Info "Installing native Windows service with built-in service support..."

    try {
        if ($existingService) {
            Write-Info "Removing existing service..."
            sc.exe delete $serviceName | Out-Null
            Start-Sleep -Seconds 2
        }

        # Create the service using New-Service
        New-Service -Name $serviceName `
                    -BinaryPathName $serviceBinaryPath `
                    -DisplayName "Pulse Host Agent" `
                    -Description "Monitors system metrics and reports to Pulse monitoring server" `
                    -StartupType Automatic | Out-Null

        Write-Success "Created Windows service '$serviceName'"

        # Register Windows Event Log source
        try {
            if (-not ([System.Diagnostics.EventLog]::SourceExists($serviceName))) {
                New-EventLog -LogName Application -Source $serviceName
                Write-Success "Registered Event Log source"
            }
        } catch {
            Write-Warning "Could not register Event Log source (not critical): $_"
        }

        Write-InstallerEvent -SourceName $serviceName -Message "Pulse Host Agent installer registered service version $(Get-Item $agentPath).VersionInfo.FileVersion" -EventId 1000

        # Configure service recovery options (restart on failure)
        sc.exe failure $serviceName reset= 86400 actions= restart/60000/restart/60000/restart/60000 | Out-Null
        Write-Success "Configured automatic restart on failure"

        # Start the service
        Write-Info "Starting service..."
        Start-Service -Name $serviceName
        Start-Sleep -Seconds 3

        $status = (Get-Service -Name $serviceName).Status
        if ($status -eq 'Running') {
            Write-Success "Service started successfully!"

            Write-Info "Waiting 10 seconds to validate agent reporting..."
            Start-Sleep -Seconds 10

            $hostname = $env:COMPUTERNAME
            $lookupHost = Test-AgentRegistration -PulseUrl $PulseUrl -Hostname $hostname -Token $Token
            if ($lookupHost) {
                Write-Success "Agent successfully registered with Pulse (host '$hostname')."
                if ($lookupHost.status) {
                    $lastSeen = $lookupHost.lastSeen
                    if ($lastSeen -is [DateTime]) {
                        $lastSeen = $lastSeen.ToString("u")
                    }
                    Write-Info ("Pulse reports status: {0} (last seen {1})" -f $lookupHost.status, $lastSeen)
                }
                Write-Info "Check your Pulse dashboard - this host should appear shortly."
                $statusForLog = if ($lookupHost.status) { $lookupHost.status } else { 'unknown' }
                Write-InstallerEvent -SourceName $serviceName -Message "Installer verified host '$hostname' reporting to Pulse (status: $statusForLog)." -EventId 1010
            } elseif ($Token) {
                Write-Warning "Agent is running but the lookup endpoint has not confirmed registration yet."
                Write-Info "It may take another moment for metrics to appear in the dashboard."
                Write-InstallerEvent -SourceName $serviceName -Message "Installer could not yet confirm host '$hostname' registration with Pulse." -EntryType Warning -EventId 1011
            } else {
                Write-Info "Registration check skipped (no API token available)."
                Write-InstallerEvent -SourceName $serviceName -Message "Installer skipped registration lookup (no API token provided)." -EventId 1012
            }

            $recentLogs = Get-RecentAgentEvents -ProviderName $serviceName -Max 5
            if ($recentLogs) {
                Write-Info "Recent service events:"
                $recentLogs | Select-Object -First 3 | ForEach-Object {
                    $time = $_.TimeCreated
                    if (-not $time) { $time = $_.TimeGenerated }
                    Write-Host ("    [{0}] {1}" -f $time.ToString("u"), $_.Message)
                }
            } else {
                Write-Warning "No recent Application log entries were found for $serviceName."
            }
        } else {
            Write-Warning "Service status: $status"
            Write-Info "Checking service logs..."
            $recentLogs = Get-RecentAgentEvents -ProviderName $serviceName -Max 5
            if ($recentLogs) {
                $recentLogs | ForEach-Object {
                    $time = $_.TimeCreated
                    if (-not $time) { $time = $_.TimeGenerated }
                    Write-Host ("    [{0}] {1}" -f $time.ToString("u"), $_.Message)
                }
            } else {
                Write-Warning "No Application log entries were found for $serviceName."
            }
        }

    } catch {
        Write-Error "Failed to create/start service: $_"
        Write-Info "You can start the agent manually with:"
        Write-Host "  $manualCommand"
        Write-Host ""
        Write-Info "Or check Windows Event Viewer (Application log) for error details."
        exit 1
    }
} else {
    Write-Info "Skipping service installation (--NoService flag)"
    Write-Host ""
    Write-Info "To start the agent manually:"
    Write-Host "  $manualCommand"
}

Write-Host ""
Write-Color $Green "═══════════════════════════════════════════════════════════"
Write-Success "Installation complete!"
Write-Color $Green "═══════════════════════════════════════════════════════════"
Write-Host ""

Write-Info "Service Management Commands:"
Write-Host "  Start:   Start-Service -Name PulseHostAgent"
Write-Host "  Stop:    Stop-Service -Name PulseHostAgent"
Write-Host "  Restart: Restart-Service -Name PulseHostAgent"
Write-Host "  Status:  Get-Service -Name PulseHostAgent | Select Status, StartType"
Write-Host "  Remove:  sc.exe delete PulseHostAgent"
Write-Host "  Logs:    Get-WinEvent -FilterHashtable @{LogName='Application'; ProviderName='PulseHostAgent'} -MaxEvents 50"
Write-Host ""

Write-Info "Files installed:"
Write-Host "  Binary: $agentPath"
Write-Host "  Config: $configPath"
Write-Host ""

Write-Info "The agent is now reporting to: $PulseUrl"
Write-Host ""
