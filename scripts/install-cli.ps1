# Pulse CLI Installer (Windows)
#
# Detects the local architecture, downloads the matching pulse-cli.exe
# from a GitHub Release, verifies the signed checksum manifest against
# Pulse's pinned release key, verifies SHA256, and installs it as pulse.exe.
#
# Verify this script before running it (see docs/AGENT_SUBSTRATE.md):
#   & .\install-cli.ps1 -Version vX.Y.Z
#
# Options (env vars or parameters):
#   PULSE_CLI_VERSION / -Version  Release tag to install. Default: latest.
#   PULSE_CLI_BIN_DIR / -BinDir   Where to install. Default: $env:LOCALAPPDATA\pulse.
#   PULSE_CLI_REPO    / -Repo     GitHub repo. Default: rcourtman/Pulse.
#
# After install, authenticate with `pulse auth login https://your-pulse-host`,
# wire your MCP client with `pulse connect claude-code` (or codex/opencode/claude-desktop),
# and verify with `pulse doctor`.

param (
    [string]$Version = $env:PULSE_CLI_VERSION,
    [string]$BinDir = $env:PULSE_CLI_BIN_DIR,
    [string]$Repo = $env:PULSE_CLI_REPO
)

$ErrorActionPreference = 'Stop'

if (-not $Version) { $Version = 'latest' }
if (-not $Repo) { $Repo = 'rcourtman/Pulse' }
if (-not $BinDir) { $BinDir = Join-Path $env:LOCALAPPDATA 'pulse' }
$SignatureIdentity = 'pulse-installer'
$SignatureNamespace = 'pulse-install'
$PinnedReleaseSshPublicKey = 'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIMZd/DaH+BldzOkq1A8KVTcFk73nAyrE8aJOyf7i00jm pulse-installer'

function Write-Log($message) {
    Write-Host "[install-cli] $message"
}

function Resolve-Architecture {
    $arch = $env:PROCESSOR_ARCHITECTURE
    switch ($arch) {
        'AMD64'  { return 'amd64' }
        'ARM64'  { return 'arm64' }
        'X86'    { return '386' }
        default {
            throw "Unsupported architecture: $arch. Build from source: go install github.com/rcourtman/pulse-go-rewrite/cmd/pulse-cli@latest"
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
    $binaryName = "pulse-cli-$platform.exe"
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
            throw "download failed: $url`nIf a release exists for this version, the binary may not yet be published for $platform.`nBuild from source: go install github.com/rcourtman/pulse-go-rewrite/cmd/pulse-cli@latest"
        }

        $expected = Get-VerifiedRemoteChecksum $base $binaryName
        $actual = (Get-FileHash -Path $tmp -Algorithm SHA256).Hash.ToLowerInvariant()
        if ($actual -ne $expected) {
            throw "sha256 mismatch for ${binaryName}: expected $expected, got $actual"
        }
        Write-Log 'sha256 verified'

        $dest = Join-Path $BinDir 'pulse.exe'
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
    Write-Log '  1. Authenticate with a read-only monitoring:read token (stored in your OS credential store):'
    Write-Log '     pulse auth login https://your-pulse-host'
    Write-Log '  2. Connect your AI agent (claude-code, codex, opencode, or claude-desktop):'
    Write-Log '     pulse connect claude-code'
    Write-Log '  3. Verify connectivity:'
    Write-Log '     pulse doctor'
}

Main
