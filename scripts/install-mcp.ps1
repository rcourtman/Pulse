# Pulse MCP Server Adapter Installer (Windows)
#
# Detects the local architecture, downloads the matching pulse-mcp.exe
# from the latest GitHub Release, verifies the signed checksum manifest against
# Pulse's pinned release key, verifies SHA256, and places the binary on PATH.
#
# Usage:
#   irm https://github.com/rcourtman/Pulse/releases/latest/download/install-mcp.ps1 | iex
#
# Options (env vars):
#   PULSE_MCP_VERSION   Override the version to install. Default: latest.
#   PULSE_MCP_BIN_DIR   Where to install. Default: $env:LOCALAPPDATA\pulse-mcp.
#   PULSE_MCP_REPO      GitHub repo. Default: rcourtman/Pulse.
#
# After install, configure your MCP client per `cmd/pulse-mcp/README.md`
# in the Pulse repository (or your installed Pulse server's `docs/AGENT_SUBSTRATE.md`).

param (
    [string]$Version = $env:PULSE_MCP_VERSION,
    [string]$BinDir = $env:PULSE_MCP_BIN_DIR,
    [string]$Repo = $env:PULSE_MCP_REPO
)

$ErrorActionPreference = 'Stop'

if (-not $Version) { $Version = 'latest' }
if (-not $Repo) { $Repo = 'rcourtman/Pulse' }
if (-not $BinDir) { $BinDir = Join-Path $env:LOCALAPPDATA 'pulse-mcp' }
$SignatureIdentity = 'pulse-installer'
$SignatureNamespace = 'pulse-install'
$PinnedReleaseSshPublicKey = 'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIMZd/DaH+BldzOkq1A8KVTcFk73nAyrE8aJOyf7i00jm pulse-installer'

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

function Get-SshKeygenPath {
    $command = Get-Command ssh-keygen.exe -ErrorAction SilentlyContinue
    if ($null -eq $command) {
        $command = Get-Command ssh-keygen -ErrorAction SilentlyContinue
    }
    if ($null -eq $command) {
        throw 'ssh-keygen is required to verify signed Pulse downloads; refusing unverified install'
    }
    return $command.Source
}

function Assert-ChecksumManifestSignature($manifestPath, $signaturePath) {
    $allowedSignersPath = [System.IO.Path]::GetTempFileName()
    $stdoutPath = [System.IO.Path]::GetTempFileName()
    $stderrPath = [System.IO.Path]::GetTempFileName()
    try {
        [System.IO.File]::WriteAllText($allowedSignersPath, "$SignatureIdentity $PinnedReleaseSshPublicKey`n")
        $sshKeygen = Get-SshKeygenPath
        $commandLine = "`"$sshKeygen`" -Y verify -f `"$allowedSignersPath`" -I `"$SignatureIdentity`" -n `"$SignatureNamespace`" -s `"$signaturePath`" < `"$manifestPath`""
        $process = Start-Process -FilePath 'cmd.exe' `
                                 -ArgumentList '/d', '/s', '/c', $commandLine `
                                 -NoNewWindow `
                                 -Wait `
                                 -PassThru `
                                 -RedirectStandardOutput $stdoutPath `
                                 -RedirectStandardError $stderrPath
        if ($process.ExitCode -ne 0) {
            throw 'cryptographic signature verification failed for checksums.txt'
        }
    } finally {
        Remove-Item -Force $allowedSignersPath, $stdoutPath, $stderrPath -ErrorAction SilentlyContinue
    }
}

function Get-VerifiedRemoteChecksum($base, $binaryName) {
    $checksumsUrl = "$base/checksums.txt"
    $signatureUrl = "$checksumsUrl.sshsig"
    $manifestPath = [System.IO.Path]::GetTempFileName()
    $signaturePath = [System.IO.Path]::GetTempFileName()
    try {
        try {
            Invoke-WebRequest -Uri $checksumsUrl -UseBasicParsing -OutFile $manifestPath -ErrorAction Stop
        } catch {
            throw 'could not fetch checksums.txt; refusing unverified install'
        }
        try {
            Invoke-WebRequest -Uri $signatureUrl -UseBasicParsing -OutFile $signaturePath -ErrorAction Stop
        } catch {
            throw 'could not fetch checksums.txt.sshsig; refusing unverified install'
        }
        Assert-ChecksumManifestSignature $manifestPath $signaturePath
        Write-Log 'release signature verified'

        $matches = @()
        foreach ($line in [System.IO.File]::ReadAllLines($manifestPath)) {
            $parts = $line.Trim() -split '\s+', 2
            if ($parts.Length -eq 2 -and $parts[1] -ceq $binaryName) {
                $matches += $parts[0]
            }
        }
        if ($matches.Count -ne 1 -or $matches[0] -notmatch '^[0-9a-fA-F]{64}$') {
            throw "checksums.txt must contain exactly one valid SHA256 entry for $binaryName"
        }
        return $matches[0].ToLowerInvariant()
    } finally {
        Remove-Item -Force $manifestPath, $signaturePath -ErrorAction SilentlyContinue
    }
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

        $expected = Get-VerifiedRemoteChecksum $base $binaryName
        $actual = (Get-FileHash -Path $tmp -Algorithm SHA256).Hash.ToLowerInvariant()
        if ($actual -ne $expected) {
            throw "sha256 mismatch for ${binaryName}: expected $expected, got $actual"
        }
        Write-Log 'sha256 verified'

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
    Write-Log '  2. Wire pulse-mcp into your MCP client per the cmd/pulse-mcp/README.md'
    Write-Log '     in the Pulse repository (or docs/AGENT_SUBSTRATE.md in your Pulse install).'
}

Main
