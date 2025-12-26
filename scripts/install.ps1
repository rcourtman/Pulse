# Pulse Unified Agent Installer (Windows)
# Usage:
#   irm http://pulse/install.ps1 | iex
#   $env:PULSE_URL="..."; $env:PULSE_TOKEN="..."; irm ... | iex

param (
    [string]$Url = $env:PULSE_URL,
    [string]$Token = $env:PULSE_TOKEN,
    [string]$Interval = "30s",
    [bool]$EnableHost = $true,
    [bool]$EnableDocker = $false,
    [bool]$EnableKubernetes = $false,
    [bool]$Insecure = $false,
    [bool]$Uninstall = $false,
    [string]$AgentId = ""
)

$ErrorActionPreference = "Stop"
$AgentName = "PulseAgent"
$BinaryName = "pulse-agent.exe"
$InstallDir = "C:\Program Files\Pulse"
$LogFile = "$env:ProgramData\Pulse\pulse-agent.log"
$DownloadTimeoutSec = 300

function Parse-Bool {
    param(
        [string]$Value,
        [bool]$Default = $false
    )
    if ([string]::IsNullOrWhiteSpace($Value)) {
        return $Default
    }
    switch ($Value.Trim().ToLowerInvariant()) {
        '1' { return $true }
        'true' { return $true }
        'yes' { return $true }
        'y' { return $true }
        'on' { return $true }
        '0' { return $false }
        'false' { return $false }
        'no' { return $false }
        'n' { return $false }
        'off' { return $false }
        default { return $Default }
    }
}

# Support env-var configuration for boolean flags (unless explicitly passed as parameters).
if (-not $PSBoundParameters.ContainsKey('EnableHost') -and -not [string]::IsNullOrWhiteSpace($env:PULSE_ENABLE_HOST)) {
    $EnableHost = Parse-Bool $env:PULSE_ENABLE_HOST $EnableHost
}
if (-not $PSBoundParameters.ContainsKey('EnableDocker') -and -not [string]::IsNullOrWhiteSpace($env:PULSE_ENABLE_DOCKER)) {
    $EnableDocker = Parse-Bool $env:PULSE_ENABLE_DOCKER $EnableDocker
}
if (-not $PSBoundParameters.ContainsKey('EnableKubernetes') -and -not [string]::IsNullOrWhiteSpace($env:PULSE_ENABLE_KUBERNETES)) {
    $EnableKubernetes = Parse-Bool $env:PULSE_ENABLE_KUBERNETES $EnableKubernetes
}
if (-not $PSBoundParameters.ContainsKey('Insecure') -and -not [string]::IsNullOrWhiteSpace($env:PULSE_INSECURE_SKIP_VERIFY)) {
    $Insecure = Parse-Bool $env:PULSE_INSECURE_SKIP_VERIFY $Insecure
}

# --- Administrator Check ---
$isAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
    Write-Host "ERROR: This script must be run as Administrator" -ForegroundColor Red
    Write-Host "Right-click PowerShell and select 'Run as Administrator'" -ForegroundColor Yellow
    Exit 1
}

# --- Cleanup Function ---
$script:TempFiles = @()
function Cleanup {
    foreach ($f in $script:TempFiles) {
        if (Test-Path $f) {
            Remove-Item $f -Force -ErrorAction SilentlyContinue
        }
    }
}

# Register cleanup on exit
$null = Register-EngineEvent -SourceIdentifier PowerShell.Exiting -Action { Cleanup }

function Show-Error {
    param([string]$Message)
    Write-Host $Message -ForegroundColor Red

    # Try to show a popup if running in a GUI environment
    try {
        Add-Type -AssemblyName System.Windows.Forms -ErrorAction SilentlyContinue
        [System.Windows.Forms.MessageBox]::Show($Message, "Pulse Installation Failed", [System.Windows.Forms.MessageBoxButtons]::OK, [System.Windows.Forms.MessageBoxIcon]::Error) | Out-Null
    } catch {
        # Ignore if GUI not available
    }
}

function Test-ValidUrl {
    param([string]$TestUrl)
    if ([string]::IsNullOrWhiteSpace($TestUrl)) { return $false }
    # Must start with http:// or https://
    if ($TestUrl -notmatch '^https?://') { return $false }
    # Basic URL structure validation
    try {
        $uri = [System.Uri]::new($TestUrl)
        return ($uri.Scheme -eq 'http' -or $uri.Scheme -eq 'https') -and (-not [string]::IsNullOrWhiteSpace($uri.Host))
    } catch {
        return $false
    }
}

function Test-ValidToken {
    param([string]$TestToken)
    if ([string]::IsNullOrWhiteSpace($TestToken)) { return $false }
    # Token should be hex string (32-128 chars typical)
    if ($TestToken.Length -lt 16 -or $TestToken.Length -gt 256) { return $false }
    # Allow alphanumeric and common token characters
    return $TestToken -match '^[a-zA-Z0-9_\-]+$'
}

function Test-ValidInterval {
    param([string]$TestInterval)
    if ([string]::IsNullOrWhiteSpace($TestInterval)) { return $false }
    # Must match pattern like 30s, 1m, 5m, etc.
    return $TestInterval -match '^\d+[smh]$'
}

function Test-PEBinary {
    param([string]$FilePath)
    if (-not (Test-Path $FilePath)) { return $false }
    try {
        $bytes = [System.IO.File]::ReadAllBytes($FilePath)
        if ($bytes.Length -lt 2) { return $false }
        # PE files start with 'MZ' (0x4D 0x5A)
        return ($bytes[0] -eq 0x4D) -and ($bytes[1] -eq 0x5A)
    } catch {
        return $false
    }
}

function Get-FileChecksum {
    param([string]$FilePath)
    $hasher = [System.Security.Cryptography.SHA256]::Create()
    try {
        $stream = [System.IO.File]::OpenRead($FilePath)
        try {
            $hash = $hasher.ComputeHash($stream)
            return [BitConverter]::ToString($hash).Replace("-", "").ToLower()
        } finally {
            $stream.Close()
        }
    } finally {
        $hasher.Dispose()
    }
}

# --- Uninstall Logic ---
if ($Uninstall) {
    Write-Host "Uninstalling $AgentName..." -ForegroundColor Cyan

    # Try to notify the Pulse server about uninstallation if we have connection details
    if (-not [string]::IsNullOrWhiteSpace($Url) -and -not [string]::IsNullOrWhiteSpace($Token)) {
        # Try to recover agent ID if not provided
        $detectedAgentId = $AgentId
        $stateFile = "$env:ProgramData\Pulse\agent-id"
        if ([string]::IsNullOrWhiteSpace($detectedAgentId) -and (Test-Path $stateFile)) {
            $detectedAgentId = Get-Content $stateFile -Raw
            if ($detectedAgentId) { $detectedAgentId = $detectedAgentId.Trim() }
        }

        if (-not [string]::IsNullOrWhiteSpace($detectedAgentId)) {
            Write-Host "Notifying Pulse server to unregister agent ID: $detectedAgentId..." -ForegroundColor Gray
            try {
                $body = @{ hostId = $detectedAgentId } | ConvertTo-Json
                $headers = @{ "X-API-Token" = $Token }

                Invoke-RestMethod -Uri "$Url/api/agents/host/uninstall" `
                                 -Method Post `
                                 -Body $body `
                                 -ContentType "application/json" `
                                 -Headers $headers `
                                 -TimeoutSec 5 `
                                 -ErrorAction SilentlyContinue | Out-Null
            } catch {
                # Ignore errors during uninstall
            }
        }
    }

    if (Get-Service $AgentName -ErrorAction SilentlyContinue) {
        Stop-Service $AgentName -Force -ErrorAction SilentlyContinue
        $scOutput = sc.exe delete $AgentName 2>&1
        if ($LASTEXITCODE -ne 0) {
            Write-Host "Warning: Failed to delete service: $scOutput" -ForegroundColor Yellow
        } else {
            Write-Host "Service deleted successfully" -ForegroundColor Green
        }
    } else {
        Write-Host "Service '$AgentName' not found (already removed)" -ForegroundColor Yellow
    }

    Remove-Item "$InstallDir\$BinaryName" -Force -ErrorAction SilentlyContinue
    Write-Host "Uninstallation complete." -ForegroundColor Green
    Exit 0
}

# --- Input Validation ---
Write-Host "Validating parameters..." -ForegroundColor Cyan

if (-not (Test-ValidUrl $Url)) {
    Show-Error "Invalid or missing URL. Must be a valid http:// or https:// URL.`nProvided: $Url"
    Exit 1
}

if (-not (Test-ValidToken $Token)) {
    Show-Error "Invalid or missing Token. Must be 16-256 alphanumeric characters.`nSet PULSE_URL and PULSE_TOKEN environment variables or pass as arguments."
    Exit 1
}

if (-not (Test-ValidInterval $Interval)) {
    Show-Error "Invalid Interval format. Must be like '30s', '1m', '5m'.`nProvided: $Interval"
    Exit 1
}

# Normalize URL (remove trailing slash)
$Url = $Url.TrimEnd('/')

# --- Download ---
# Determine architecture
$Arch = if ([Environment]::Is64BitOperatingSystem) { "amd64" } else { "386" }
$ArchParam = "windows-$Arch"
$DownloadUrl = "$Url/download/pulse-agent?arch=$ArchParam"
Write-Host "Downloading agent from $DownloadUrl..." -ForegroundColor Cyan

if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
}

# Download to temp file first
$TempPath = [System.IO.Path]::GetTempFileName() + ".exe"
$script:TempFiles += $TempPath
$DestPath = "$InstallDir\$BinaryName"

try {
    # Configure TLS 1.2 minimum
    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12 -bor [Net.SecurityProtocolType]::Tls13

    # Download with timeout
    $webClient = New-Object System.Net.WebClient
    $webClient.Headers.Add("User-Agent", "PulseInstaller/1.0")

    # Set up async download with timeout
    $downloadTask = $webClient.DownloadFileTaskAsync($DownloadUrl, $TempPath)
    if (-not $downloadTask.Wait($DownloadTimeoutSec * 1000)) {
        $webClient.CancelAsync()
        throw "Download timed out after $DownloadTimeoutSec seconds"
    }
    if ($downloadTask.IsFaulted) {
        throw $downloadTask.Exception.InnerException
    }

    # Get checksum from server response headers if available
    $serverChecksum = $webClient.ResponseHeaders["X-Checksum-Sha256"]

} catch {
    Cleanup
    Show-Error "Failed to download agent: $_"
    Write-Host ""
    Write-Host "Press Enter to exit..." -ForegroundColor Yellow
    Read-Host
    Exit 1
} finally {
    if ($webClient) { $webClient.Dispose() }
}

# --- Binary Verification ---
Write-Host "Verifying downloaded binary..." -ForegroundColor Cyan

# Check file size (should be reasonable - between 1MB and 100MB)
$fileInfo = Get-Item $TempPath
$fileSizeMB = $fileInfo.Length / 1MB
if ($fileSizeMB -lt 1 -or $fileSizeMB -gt 100) {
    Cleanup
    Show-Error "Downloaded file has unexpected size: $([math]::Round($fileSizeMB, 2)) MB. Expected 1-100 MB."
    Exit 1
}

# Verify PE signature (MZ header)
if (-not (Test-PEBinary $TempPath)) {
    Cleanup
    Show-Error "Downloaded file is not a valid Windows executable."
    Exit 1
}

# Verify checksum if server provided one
if (-not [string]::IsNullOrWhiteSpace($serverChecksum)) {
    $localChecksum = Get-FileChecksum $TempPath
    if ($localChecksum -ne $serverChecksum.ToLower()) {
        Cleanup
        Show-Error "Checksum verification failed!`nExpected: $serverChecksum`nGot: $localChecksum"
        Exit 1
    }
    Write-Host "Checksum verified: $localChecksum" -ForegroundColor Green
} else {
    Write-Host "Warning: Server did not provide checksum header" -ForegroundColor Yellow
}

# --- Legacy Cleanup ---
# Remove old agents if they exist to prevent conflicts
Write-Host "Checking for legacy agents..." -ForegroundColor Cyan

if (Get-Service "PulseHostAgent" -ErrorAction SilentlyContinue) {
    Write-Host "Removing legacy PulseHostAgent..." -ForegroundColor Yellow
    Stop-Service "PulseHostAgent" -Force -ErrorAction SilentlyContinue
    $scOutput = sc.exe delete "PulseHostAgent" 2>&1
    if ($LASTEXITCODE -ne 0) {
        Write-Host "Warning: Failed to delete PulseHostAgent service: $scOutput" -ForegroundColor Yellow
    }
    Remove-Item "C:\Program Files\Pulse\pulse-host-agent.exe" -Force -ErrorAction SilentlyContinue
    Start-Sleep -Seconds 2
}

if (Get-Service "PulseDockerAgent" -ErrorAction SilentlyContinue) {
    Write-Host "Removing legacy PulseDockerAgent..." -ForegroundColor Yellow
    Stop-Service "PulseDockerAgent" -Force -ErrorAction SilentlyContinue
    $scOutput = sc.exe delete "PulseDockerAgent" 2>&1
    if ($LASTEXITCODE -ne 0) {
        Write-Host "Warning: Failed to delete PulseDockerAgent service: $scOutput" -ForegroundColor Yellow
    }
    Remove-Item "C:\Program Files\Pulse\pulse-docker-agent.exe" -Force -ErrorAction SilentlyContinue
    Start-Sleep -Seconds 2
}

# --- Install Binary ---
Write-Host "Installing binary..." -ForegroundColor Cyan

# Stop existing service if running
if (Get-Service $AgentName -ErrorAction SilentlyContinue) {
    Write-Host "Removing existing $AgentName service..." -ForegroundColor Yellow
    Stop-Service $AgentName -Force -ErrorAction SilentlyContinue
    $scOutput = sc.exe delete $AgentName 2>&1
    if ($LASTEXITCODE -ne 0) {
        Write-Host "Warning: Failed to delete existing service: $scOutput" -ForegroundColor Yellow
    }
    Start-Sleep -Seconds 2
}

# Move temp file to final location
try {
    # Create backup of existing binary if present
    if (Test-Path $DestPath) {
        $BackupPath = "$DestPath.backup"
        Move-Item $DestPath $BackupPath -Force -ErrorAction SilentlyContinue
    }

    Move-Item $TempPath $DestPath -Force
} catch {
    # Restore backup on failure
    if (Test-Path "$DestPath.backup") {
        Move-Item "$DestPath.backup" $DestPath -Force -ErrorAction SilentlyContinue
    }
    Cleanup
    Show-Error "Failed to install binary: $_"
    Exit 1
}

# Remove backup on success
Remove-Item "$DestPath.backup" -Force -ErrorAction SilentlyContinue

# --- Service Installation ---
Write-Host "Configuring Windows Service..." -ForegroundColor Cyan

# Build command line args (properly escaped)
$ServiceArgs = @(
    "--url", "`"$Url`"",
    "--token", "`"$Token`"",
    "--interval", "`"$Interval`""
)
if ($EnableHost) { $ServiceArgs += "--enable-host" }
if ($EnableDocker) { $ServiceArgs += "--enable-docker" }
if ($EnableKubernetes) { $ServiceArgs += "--enable-kubernetes" }
if ($Insecure) { $ServiceArgs += "--insecure" }
if (-not [string]::IsNullOrWhiteSpace($AgentId)) { $ServiceArgs += @("--agent-id", "`"$AgentId`"") }

$BinPath = "`"$DestPath`" $($ServiceArgs -join ' ')"

# Create Service using New-Service (more reliable than sc.exe create)
try {
    New-Service -Name $AgentName `
                -BinaryPathName $BinPath `
                -DisplayName "Pulse Unified Agent" `
                -Description "Pulse Unified Agent for Host, Docker, and Kubernetes monitoring" `
                -StartupType Automatic | Out-Null
    Write-Host "Service created successfully" -ForegroundColor Green
} catch {
    Show-Error "Failed to create service '$AgentName'.`nError: $_"
    Exit 1
}

$scOutput = sc.exe failure $AgentName reset= 86400 actions= restart/5000/restart/5000/restart/5000 2>&1
if ($LASTEXITCODE -ne 0) {
    Write-Host "Warning: Failed to configure service recovery: $scOutput" -ForegroundColor Yellow
}

# Ensure log directory exists
$LogDir = Split-Path $LogFile -Parent
if (-not (Test-Path $LogDir)) {
    New-Item -ItemType Directory -Force -Path $LogDir | Out-Null
}

# Start the service
try {
    Start-Service $AgentName -ErrorAction Stop
    Write-Host "Service started successfully" -ForegroundColor Green
} catch {
    Show-Error "Failed to start service '$AgentName': $_"
    Exit 1
}

Write-Host ""
Write-Host "Installation complete." -ForegroundColor Green
Write-Host "Service: $AgentName"
Write-Host "Binary:  $DestPath"
Write-Host "Logs:    $LogFile"
Write-Host ""
