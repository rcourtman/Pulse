# Pulse Host Agent Uninstallation Script for Windows
#
# Usage:
#   iwr -useb http://pulse-server:7655/uninstall-host-agent.ps1 | iex
#

param(
    [string]$InstallPath = "C:\Program Files\Pulse"
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

Write-Host ""
$banner = "=" * 59
Write-Host $banner -ForegroundColor Cyan
Write-Host "  Pulse Host Agent - Windows Uninstallation" -ForegroundColor Cyan
Write-Host $banner -ForegroundColor Cyan
Write-Host ""

# Check if running as Administrator
$isAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
    PulseError "This script must be run as Administrator"
    PulseInfo "Right-click PowerShell and select 'Run as Administrator'"
    exit 1
}

$serviceName = "PulseHostAgent"

# Stop and remove service
PulseInfo "Checking for Pulse Host Agent service..."
$service = Get-Service -Name $serviceName -ErrorAction SilentlyContinue

if ($service) {
    if ($service.Status -eq 'Running') {
        PulseInfo "Stopping service..."
        try {
            Stop-Service -Name $serviceName -Force
            PulseSuccess "Service stopped"
        } catch {
            PulseWarn "Could not stop service: $_"
        }
    }

    PulseInfo "Removing service..."
    try {
        sc.exe delete $serviceName | Out-Null
        PulseSuccess "Service removed"
    } catch {
        PulseWarn "Could not remove service: $_"
    }
} else {
    PulseInfo "Service not found (already removed or never installed)"
}

# Ensure all processes are terminated
PulseInfo "Ensuring all processes are terminated..."
$processes = Get-Process -Name "pulse-host-agent" -ErrorAction SilentlyContinue
if ($processes) {
    $processes | Stop-Process -Force
    Start-Sleep -Seconds 2
    PulseSuccess "Processes terminated"
} else {
    PulseInfo "No running processes found"
}

# Remove Event Log source
PulseInfo "Removing Event Log source..."
try {
    if ([System.Diagnostics.EventLog]::SourceExists($serviceName)) {
        Remove-EventLog -Source $serviceName
        PulseSuccess "Event Log source removed"
    } else {
        PulseInfo "Event Log source not found"
    }
} catch {
    PulseWarn "Could not remove Event Log source: $_"
}

# Remove installation directory with retry logic (Windows file locking)
if (Test-Path $InstallPath) {
    PulseInfo "Removing installation directory..."

    $retries = 3
    $success = $false

    while ($retries -gt 0 -and -not $success) {
        try {
            # Wait for file handles to be released after service stop
            Start-Sleep -Seconds 2

            Remove-Item -Path $InstallPath -Recurse -Force -ErrorAction Stop
            PulseSuccess "Installation directory removed: $InstallPath"
            $success = $true
        } catch {
            $retries--
            if ($retries -gt 0) {
                PulseWarn "File still locked, retrying... ($retries attempts remaining)"
            } else {
                PulseError "Could not remove installation directory after multiple attempts: $_"
                PulseWarn "The service may still have file handles open."
                PulseWarn "Please wait a few seconds and manually delete: $InstallPath"
                PulseInfo "Or reboot and run the uninstall script again."
            }
        }
    }
} else {
    PulseInfo "Installation directory not found: $InstallPath"
}

Write-Host ""
$successBanner = "=" * 59
Write-Host $successBanner -ForegroundColor Green
PulseSuccess "Uninstallation complete!"
Write-Host $successBanner -ForegroundColor Green
Write-Host ""

PulseInfo "The Pulse Host Agent has been removed from this system."
PulseInfo "This host will no longer appear in your Pulse dashboard."
Write-Host ""
