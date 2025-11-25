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
    [bool]$Insecure = $false,
    [bool]$Uninstall = $false
)

$ErrorActionPreference = "Stop"
$AgentName = "PulseAgent"
$BinaryName = "pulse-agent.exe"
$InstallDir = "C:\Program Files\Pulse"
$LogFile = "$env:ProgramData\Pulse\pulse-agent.log"

# --- Uninstall Logic ---
if ($Uninstall) {
    Write-Host "Uninstalling $AgentName..." -ForegroundColor Cyan
    
    if (Get-Service $AgentName -ErrorAction SilentlyContinue) {
        Stop-Service $AgentName -Force -ErrorAction SilentlyContinue
        sc.exe delete $AgentName | Out-Null
    }
    
    Remove-Item "$InstallDir\$BinaryName" -Force -ErrorAction SilentlyContinue
    Write-Host "Uninstallation complete." -ForegroundColor Green
    Exit 0
}

# --- Validation ---
if ([string]::IsNullOrWhiteSpace($Url) -or [string]::IsNullOrWhiteSpace($Token)) {
    Write-Error "Missing required parameters: Url and Token. Set PULSE_URL/PULSE_TOKEN env vars or pass arguments."
    Exit 1
}

# --- Download ---
$DownloadUrl = "$Url/download/pulse-agent?os=windows&arch=amd64"
Write-Host "Downloading agent from $DownloadUrl..." -ForegroundColor Cyan

if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
}

$DestPath = "$InstallDir\$BinaryName"
try {
    Invoke-WebRequest -Uri $DownloadUrl -OutFile $DestPath -UseBasicParsing
} catch {
    Write-Host "Failed to download agent: $_" -ForegroundColor Red
    
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
    Exit 1
}

# --- Service Installation ---
Write-Host "Configuring Windows Service..." -ForegroundColor Cyan

if (Get-Service $AgentName -ErrorAction SilentlyContinue) {
    Stop-Service $AgentName -Force -ErrorAction SilentlyContinue
    sc.exe delete $AgentName | Out-Null
    Start-Sleep -Seconds 2
}

$Args = "--url `"$Url`" --token `"$Token`" --interval `"$Interval`" --enable-host=$EnableHost --enable-docker=$EnableDocker --insecure=$Insecure"
$BinPath = "`"$DestPath`" $Args"

# Create Service
sc.exe create $AgentName binPath= $BinPath start= auto displayname= "Pulse Unified Agent" | Out-Null
sc.exe description $AgentName "Pulse Unified Agent for Host and Docker monitoring" | Out-Null
sc.exe failure $AgentName reset= 86400 actions= restart/5000/restart/5000/restart/5000 | Out-Null

Start-Service $AgentName

Write-Host "Installation complete." -ForegroundColor Green
Write-Host "Service: $AgentName"
Write-Host "Logs:    $LogFile"
