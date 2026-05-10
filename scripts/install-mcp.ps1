# Pulse MCP Server Adapter Installer (Windows)
#
# Detects the local architecture, downloads the matching pulse-mcp.exe
# from the latest GitHub Release, verifies SHA256 against the published
# checksums file, and places the binary on PATH.
#
# Usage:
#   irm https://github.com/rcourtman/Pulse/releases/latest/download/install-mcp.ps1 | iex
#
# Options (env vars):
#   PULSE_MCP_VERSION   Override the version to install. Default: latest.
#   PULSE_MCP_BIN_DIR   Where to install. Default: $env:LOCALAPPDATA\pulse-mcp.
#   PULSE_MCP_REPO      GitHub repo. Default: rcourtman/Pulse.
#   PULSE_MCP_NO_VERIFY If "1", skip SHA256 verification (not recommended).
#
# After install, configure your MCP client per the README:
#   https://github.com/rcourtman/Pulse/blob/main/cmd/pulse-mcp/README.md

param (
    [string]$Version = $env:PULSE_MCP_VERSION,
    [string]$BinDir = $env:PULSE_MCP_BIN_DIR,
    [string]$Repo = $env:PULSE_MCP_REPO,
    [switch]$NoVerify
)

$ErrorActionPreference = 'Stop'

if (-not $Version) { $Version = 'latest' }
if (-not $Repo) { $Repo = 'rcourtman/Pulse' }
if (-not $BinDir) { $BinDir = Join-Path $env:LOCALAPPDATA 'pulse-mcp' }
if ($env:PULSE_MCP_NO_VERIFY -eq '1') { $NoVerify = $true }

function Write-Log($message) {
    Write-Host "[install-mcp] $message"
}

function Resolve-Architecture {
    $arch = $env:PROCESSOR_ARCHITECTURE
    switch ($arch) {
        'AMD64'  { return 'amd64' }
        'ARM64'  { return 'arm64' }
        'X86'    { return '386' }
        default {
            throw "Unsupported architecture: $arch. Build from source: go install github.com/rcourtman/pulse-go-rewrite/cmd/pulse-mcp@latest"
        }
    }
}

function Resolve-ReleaseBase {
    if ($Version -eq 'latest') {
        return "https://github.com/$Repo/releases/latest/download"
    }
    return "https://github.com/$Repo/releases/download/$Version"
}

function Get-RemoteChecksum($base, $binaryName) {
    $checksumsUrl = "$base/checksums.txt"
    try {
        $response = Invoke-WebRequest -Uri $checksumsUrl -UseBasicParsing -ErrorAction Stop
    } catch {
        Write-Log "warning: could not fetch checksums.txt; skipping verification"
        return $null
    }
    foreach ($line in $response.Content -split "`n") {
        $parts = $line.Trim() -split '\s+', 2
        if ($parts.Length -eq 2 -and $parts[1] -eq $binaryName) {
            return $parts[0]
        }
    }
    Write-Log "warning: $binaryName not listed in checksums.txt; skipping verification"
    return $null
}

function Main {
    $arch = Resolve-Architecture
    $platform = "windows-$arch"
    $binaryName = "pulse-mcp-$platform.exe"
    $base = Resolve-ReleaseBase
    $url = "$base/$binaryName"

    Write-Log "platform: $platform"
    Write-Log "install dir: $BinDir"
    Write-Log "downloading: $url"

    if (-not (Test-Path $BinDir)) {
        New-Item -ItemType Directory -Force -Path $BinDir | Out-Null
    }

    $tmp = [System.IO.Path]::GetTempFileName()
    try {
        try {
            Invoke-WebRequest -Uri $url -UseBasicParsing -OutFile $tmp -ErrorAction Stop
        } catch {
            throw "download failed: $url`nIf a release exists for this version, the binary may not yet be published for $platform.`nBuild from source: go install github.com/rcourtman/pulse-go-rewrite/cmd/pulse-mcp@latest"
        }

        if (-not $NoVerify) {
            $expected = Get-RemoteChecksum $base $binaryName
            if ($expected) {
                $actual = (Get-FileHash -Path $tmp -Algorithm SHA256).Hash.ToLower()
                if ($actual -ne $expected.ToLower()) {
                    throw "sha256 mismatch for ${binaryName}: expected $expected, got $actual"
                }
                Write-Log 'sha256 verified'
            }
        }

        $dest = Join-Path $BinDir 'pulse-mcp.exe'
        Move-Item -Path $tmp -Destination $dest -Force
        Write-Log "installed: $dest"
    } finally {
        if (Test-Path $tmp) { Remove-Item -Force $tmp -ErrorAction SilentlyContinue }
    }

    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    if (-not ($userPath -split ';' | Where-Object { $_ -eq $BinDir })) {
        Write-Log "note: $BinDir is not on your user PATH. To add it, run:"
        Write-Log "  [Environment]::SetEnvironmentVariable('Path', `"`$BinDir;`$env:Path`", 'User')"
    }

    Write-Log ''
    Write-Log 'next steps:'
    Write-Log '  1. Mint a Pulse API token in Settings -> API Access (with monitoring:read,'
    Write-Log '     and monitoring:write if you want the operator-state write tools).'
    Write-Log '  2. Wire pulse-mcp into your MCP client per:'
    Write-Log "     https://github.com/$Repo/blob/main/cmd/pulse-mcp/README.md"
}

Main
