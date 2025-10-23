# Pulse Host Agent Uninstallation Script for Windows
#
# Usage:
#   iwr -useb http://pulse-server:7656/uninstall-host-agent.ps1 | iex
#

param(
    [string]$InstallPath = "C:\Program Files\Pulse"
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

Write-Host ""
Write-Color $Blue "═══════════════════════════════════════════════════════════"
Write-Color $Blue "  Pulse Host Agent - Windows Uninstallation"
Write-Color $Blue "═══════════════════════════════════════════════════════════"
Write-Host ""

# Check if running as Administrator
$isAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
    Write-Error "This script must be run as Administrator"
    Write-Info "Right-click PowerShell and select 'Run as Administrator'"
    exit 1
}

$serviceName = "PulseHostAgent"

# Stop and remove service
Write-Info "Checking for Pulse Host Agent service..."
$service = Get-Service -Name $serviceName -ErrorAction SilentlyContinue

if ($service) {
    if ($service.Status -eq 'Running') {
        Write-Info "Stopping service..."
        try {
            Stop-Service -Name $serviceName -Force
            Write-Success "Service stopped"
        } catch {
            Write-Warning "Could not stop service: $_"
        }
    }

    Write-Info "Removing service..."
    try {
        sc.exe delete $serviceName | Out-Null
        Write-Success "Service removed"
    } catch {
        Write-Warning "Could not remove service: $_"
    }
} else {
    Write-Info "Service not found (already removed or never installed)"
}

# Ensure all processes are terminated
Write-Info "Ensuring all processes are terminated..."
$processes = Get-Process -Name "pulse-host-agent" -ErrorAction SilentlyContinue
if ($processes) {
    $processes | Stop-Process -Force
    Start-Sleep -Seconds 2
    Write-Success "Processes terminated"
} else {
    Write-Info "No running processes found"
}

# Remove Event Log source
Write-Info "Removing Event Log source..."
try {
    if ([System.Diagnostics.EventLog]::SourceExists($serviceName)) {
        Remove-EventLog -Source $serviceName
        Write-Success "Event Log source removed"
    } else {
        Write-Info "Event Log source not found"
    }
} catch {
    Write-Warning "Could not remove Event Log source: $_"
}

# Remove installation directory with retry logic (Windows file locking)
if (Test-Path $InstallPath) {
    Write-Info "Removing installation directory..."

    $retries = 3
    $success = $false

    while ($retries -gt 0 -and -not $success) {
        try {
            # Wait for file handles to be released after service stop
            Start-Sleep -Seconds 2

            Remove-Item -Path $InstallPath -Recurse -Force -ErrorAction Stop
            Write-Success "Installation directory removed: $InstallPath"
            $success = $true
        } catch {
            $retries--
            if ($retries -gt 0) {
                Write-Warning "File still locked, retrying... ($retries attempts remaining)"
            } else {
                Write-Error "Could not remove installation directory after multiple attempts: $_"
                Write-Warning "The service may still have file handles open."
                Write-Warning "Please wait a few seconds and manually delete: $InstallPath"
                Write-Info "Or reboot and run the uninstall script again."
            }
        }
    }
} else {
    Write-Info "Installation directory not found: $InstallPath"
}

Write-Host ""
Write-Color $Green "═══════════════════════════════════════════════════════════"
Write-Success "Uninstallation complete!"
Write-Color $Green "═══════════════════════════════════════════════════════════"
Write-Host ""

Write-Info "The Pulse Host Agent has been removed from this system."
Write-Info "This host will no longer appear in your Pulse dashboard."
Write-Host ""
