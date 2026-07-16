# Pulse Unified Agent Installer (Windows)
# Usage:
#   irm http://pulse/install.ps1 | iex
#   $env:PULSE_URL="..."; $env:PULSE_TOKEN="..."; irm ... | iex
#
# Uninstall:
#   $env:PULSE_UNINSTALL="true"; irm http://pulse/install.ps1 | iex

param (
    [string]$Url = $env:PULSE_URL,
    [string]$Token = $env:PULSE_TOKEN,
    [string]$Interval = "30s",
    [bool]$EnableHost = $true,
    [bool]$EnableDocker = $false,
    [bool]$EnableKubernetes = $false,
    [bool]$EnableProxmox = $false,
    [string]$ProxmoxType = "",
    [bool]$EnableCommands = $false,
    [bool]$Insecure = $false,
    [bool]$Uninstall = $false,
    [string]$CACertPath = $env:PULSE_CACERT,
    [string]$ServerFingerprint = $env:PULSE_SERVER_FINGERPRINT,
    [string]$AgentId = $env:PULSE_AGENT_ID,
    [string]$Hostname = $env:PULSE_HOSTNAME,
    [string]$TokenFile = $env:PULSE_TOKEN_FILE,
    [bool]$PreflightOnly = $false,
    [string]$Output = $env:PULSE_OUTPUT,
    [bool]$NonInteractive = $false
)

$ErrorActionPreference = "Stop"
$AgentName = "PulseAgent"
$BinaryName = "pulse-agent.exe"
$InstallDir = "C:\Program Files\Pulse"
$StateDir = "$env:ProgramData\Pulse"
$ConnectionStatePath = "$StateDir\connection.env"
$TokenFilePath = "$StateDir\token"
$LogFile = "$env:ProgramData\Pulse\pulse-agent.log"
$DownloadTimeoutSec = 300
$PinnedInstallerSshPublicKey = "__PULSE_INSTALLER_SSH_PUBLIC_KEY__"
$InstallerSignatureNamespace = "pulse-install"
$InstallerSignatureIdentity = "pulse-installer"
$InstallerSignatureHeaderName = "X-Signature-SSHSIG"

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
if (-not $PSBoundParameters.ContainsKey('EnableProxmox') -and -not [string]::IsNullOrWhiteSpace($env:PULSE_ENABLE_PROXMOX)) {
    $EnableProxmox = Parse-Bool $env:PULSE_ENABLE_PROXMOX $EnableProxmox
}
if (-not $PSBoundParameters.ContainsKey('ProxmoxType') -and -not [string]::IsNullOrWhiteSpace($env:PULSE_PROXMOX_TYPE)) {
    $ProxmoxType = $env:PULSE_PROXMOX_TYPE
}
if (-not $PSBoundParameters.ContainsKey('EnableCommands') -and -not [string]::IsNullOrWhiteSpace($env:PULSE_ENABLE_COMMANDS)) {
    $EnableCommands = Parse-Bool $env:PULSE_ENABLE_COMMANDS $EnableCommands
}
if (-not $PSBoundParameters.ContainsKey('Insecure') -and -not [string]::IsNullOrWhiteSpace($env:PULSE_INSECURE_SKIP_VERIFY)) {
    $Insecure = Parse-Bool $env:PULSE_INSECURE_SKIP_VERIFY $Insecure
}
if (-not $PSBoundParameters.ContainsKey('Uninstall') -and -not [string]::IsNullOrWhiteSpace($env:PULSE_UNINSTALL)) {
    $Uninstall = Parse-Bool $env:PULSE_UNINSTALL $Uninstall
}
if (-not $PSBoundParameters.ContainsKey('PreflightOnly') -and -not [string]::IsNullOrWhiteSpace($env:PULSE_PREFLIGHT_ONLY)) {
    $PreflightOnly = Parse-Bool $env:PULSE_PREFLIGHT_ONLY $PreflightOnly
}
if (-not $PSBoundParameters.ContainsKey('NonInteractive') -and -not [string]::IsNullOrWhiteSpace($env:PULSE_NON_INTERACTIVE)) {
    $NonInteractive = Parse-Bool $env:PULSE_NON_INTERACTIVE $NonInteractive
}

# Docker-only installs should not silently fall back to host metrics unless the
# caller explicitly opts back in.
if ($EnableDocker -and -not $PSBoundParameters.ContainsKey('EnableHost') -and [string]::IsNullOrWhiteSpace($env:PULSE_ENABLE_HOST)) {
    $EnableHost = $false
}

# --- Administrator Check ---
$isAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin -and -not $PreflightOnly) {
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

function Write-InstallerEvent {
    param(
        [string]$Phase,
        [string]$Code,
        [string]$Message,
        [int]$ExitCode = 0
    )

    if ($Output -eq "json") {
        @{ phase = $Phase; code = $Code; message = $Message; exitCode = $ExitCode } | ConvertTo-Json -Compress
        return
    }

    Write-Host $Message
}

function Get-ResponseHeaderValue {
    param(
        $Headers,
        [string]$Name
    )

    if ($null -eq $Headers -or [string]::IsNullOrWhiteSpace($Name)) {
        return ""
    }

    try {
        $value = $Headers[$Name]
        if ($null -ne $value) {
            if ($value -is [array]) {
                return [string]$value[0]
            }
            return [string]$value
        }
    } catch {
        # Fall back to case-insensitive enumeration below.
    }

    try {
        foreach ($key in $Headers.Keys) {
            if ([string]::Equals([string]$key, $Name, [System.StringComparison]::OrdinalIgnoreCase)) {
                $value = $Headers[$key]
                if ($value -is [array]) {
                    return [string]$value[0]
                }
                return [string]$value
            }
        }
    } catch {
        return ""
    }

    return ""
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

function Load-CustomCaCertificate {
    param([string]$Path)

    if ([string]::IsNullOrWhiteSpace($Path)) {
        return $null
    }

    $resolvedPath = $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($Path)
    $raw = Get-Content -Path $resolvedPath -Raw -ErrorAction Stop
    if ($raw -match '-----BEGIN CERTIFICATE-----(?<body>[\s\S]+?)-----END CERTIFICATE-----') {
        $base64 = ($matches.body -replace '\s+', '')
        $bytes = [Convert]::FromBase64String($base64)
        return [System.Security.Cryptography.X509Certificates.X509Certificate2]::new($bytes)
    }

    return [System.Security.Cryptography.X509Certificates.X509Certificate2]::new($resolvedPath)
}

function Test-CertificateTrustedByCustomCa {
    param(
        [System.Security.Cryptography.X509Certificates.X509Certificate]$Certificate,
        [System.Security.Cryptography.X509Certificates.X509Certificate2]$CustomCaCertificate
    )

    if ($null -eq $Certificate -or $null -eq $CustomCaCertificate) {
        return $false
    }

    $leaf = if ($Certificate -is [System.Security.Cryptography.X509Certificates.X509Certificate2]) {
        $Certificate
    } else {
        [System.Security.Cryptography.X509Certificates.X509Certificate2]::new($Certificate)
    }

    $chain = [System.Security.Cryptography.X509Certificates.X509Chain]::new()
    try {
        $chain.ChainPolicy.RevocationMode = [System.Security.Cryptography.X509Certificates.X509RevocationMode]::NoCheck
        $chain.ChainPolicy.VerificationFlags = [System.Security.Cryptography.X509Certificates.X509VerificationFlags]::AllowUnknownCertificateAuthority
        $null = $chain.ChainPolicy.ExtraStore.Add($CustomCaCertificate)
        if (-not $chain.Build($leaf)) {
            return $false
        }

        foreach ($element in $chain.ChainElements) {
            if ($element.Certificate.Thumbprint -eq $CustomCaCertificate.Thumbprint) {
                return $true
            }
        }
        return $false
    } finally {
        $chain.Dispose()
    }
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

function Test-HasPinnedInstallerSignatureKey {
    return (-not [string]::IsNullOrWhiteSpace($PinnedInstallerSshPublicKey)) -and ($PinnedInstallerSshPublicKey -ne "__PULSE_INSTALLER_SSH_PUBLIC_KEY__")
}

function Get-SshKeygenPath {
    $command = Get-Command ssh-keygen.exe -ErrorAction SilentlyContinue
    if ($null -eq $command) {
        $command = Get-Command ssh-keygen -ErrorAction SilentlyContinue
    }
    if ($null -eq $command) {
        throw "ssh-keygen is required to verify signed Pulse downloads."
    }
    return $command.Source
}

function Invoke-InstallerSignatureVerification {
    param(
        [string]$FilePath,
        [string]$SignatureHeader
    )

    if (-not (Test-HasPinnedInstallerSignatureKey)) {
        return
    }
    if ([string]::IsNullOrWhiteSpace($SignatureHeader)) {
        throw "Server did not provide SSH signature metadata; refusing signed install."
    }

    $allowedSignersPath = [System.IO.Path]::GetTempFileName()
    $signaturePath = [System.IO.Path]::GetTempFileName()
    $stdoutPath = [System.IO.Path]::GetTempFileName()
    $stderrPath = [System.IO.Path]::GetTempFileName()
    $script:TempFiles += @($allowedSignersPath, $signaturePath, $stdoutPath, $stderrPath)

    [System.IO.File]::WriteAllText($allowedSignersPath, "$InstallerSignatureIdentity $PinnedInstallerSshPublicKey`n")
    try {
        [System.IO.File]::WriteAllBytes($signaturePath, [Convert]::FromBase64String($SignatureHeader.Trim()))
    } catch {
        throw "Server provided an invalid SSH signature payload."
    }

    $sshKeygen = Get-SshKeygenPath
    $commandLine = "`"$sshKeygen`" -Y verify -f `"$allowedSignersPath`" -I `"$InstallerSignatureIdentity`" -n `"$InstallerSignatureNamespace`" -s `"$signaturePath`" < `"$FilePath`""
    $process = Start-Process -FilePath "cmd.exe" `
                             -ArgumentList "/d", "/s", "/c", $commandLine `
                             -NoNewWindow `
                             -Wait `
                             -PassThru `
                             -RedirectStandardOutput $stdoutPath `
                             -RedirectStandardError $stderrPath
    if ($process.ExitCode -ne 0) {
        $stderr = ""
        if (Test-Path $stderrPath) {
            $stderr = (Get-Content $stderrPath -Raw -ErrorAction SilentlyContinue).Trim()
        }
        if ([string]::IsNullOrWhiteSpace($stderr)) {
            throw "Cryptographic signature verification failed for the downloaded agent binary."
        }
        throw "Cryptographic signature verification failed for the downloaded agent binary. $stderr"
    }

    Write-Host "Cryptographic signature verified." -ForegroundColor Green
}

function Invoke-WithOptionalInsecureTls {
    param(
        [bool]$AllowInsecure,
        [System.Security.Cryptography.X509Certificates.X509Certificate2]$CustomCaCertificate,
        [scriptblock]$Action
    )

    $previousCallback = [System.Net.ServicePointManager]::ServerCertificateValidationCallback
    if ($AllowInsecure -or $null -ne $CustomCaCertificate -or -not [string]::IsNullOrWhiteSpace($ServerFingerprint)) {
        [System.Net.ServicePointManager]::ServerCertificateValidationCallback = {
            param($sender, $certificate, $chain, $sslPolicyErrors)

            if (-not [string]::IsNullOrWhiteSpace($ServerFingerprint)) {
                try {
                    $sha256 = [System.Security.Cryptography.SHA256]::Create()
                    try {
                        $actualFingerprint = ([System.BitConverter]::ToString($sha256.ComputeHash($certificate.GetRawCertData()))).Replace('-', '').ToLowerInvariant()
                    } finally {
                        $sha256.Dispose()
                    }
                    return [string]::Equals($actualFingerprint, $ServerFingerprint, [System.StringComparison]::OrdinalIgnoreCase)
                } catch {
                    return $false
                }
            }

            if ($AllowInsecure) {
                return $true
            }

            if ($sslPolicyErrors -eq [System.Net.Security.SslPolicyErrors]::None) {
                return $true
            }

            return Test-CertificateTrustedByCustomCa -Certificate $certificate -CustomCaCertificate $CustomCaCertificate
        }
    }

    try {
        & $Action
    } finally {
        if ($AllowInsecure -or $null -ne $CustomCaCertificate -or -not [string]::IsNullOrWhiteSpace($ServerFingerprint)) {
            [System.Net.ServicePointManager]::ServerCertificateValidationCallback = $previousCallback
        }
    }
}

function Save-ConnectionState {
    $lines = @("PULSE_URL='$Url'")
    if (Test-Path $TokenFilePath) {
        $lines += "PULSE_TOKEN_FILE='$TokenFilePath'"
    }
    if (-not [string]::IsNullOrWhiteSpace($AgentId)) {
        $lines += "PULSE_AGENT_ID='$AgentId'"
    }
    if (-not [string]::IsNullOrWhiteSpace($Hostname)) {
        $lines += "PULSE_HOSTNAME='$Hostname'"
    }
    if ($Insecure) {
        $lines += "PULSE_INSECURE_SKIP_VERIFY='true'"
    }
    if (-not [string]::IsNullOrWhiteSpace($CACertPath)) {
        $lines += "PULSE_CACERT='$CACertPath'"
    }
    if (-not [string]::IsNullOrWhiteSpace($ServerFingerprint)) {
        $lines += "PULSE_SERVER_FINGERPRINT='$ServerFingerprint'"
    }

    New-Item -ItemType Directory -Path $StateDir -Force | Out-Null
    Set-Content -Path $ConnectionStatePath -Value ($lines -join "`n") -Encoding UTF8
}

function Write-RuntimeTokenFile {
    if ([string]::IsNullOrWhiteSpace($Token)) {
        Remove-Item $TokenFilePath -Force -ErrorAction SilentlyContinue
        return
    }

    New-Item -ItemType Directory -Path $StateDir -Force | Out-Null
    Set-Content -Path $TokenFilePath -Value $Token -NoNewline -Encoding ASCII
    try {
        icacls.exe $TokenFilePath /inheritance:r /grant:r "SYSTEM:F" "Administrators:F" | Out-Null
    } catch {
        Write-Host "Warning: Failed to tighten token file ACLs: $_" -ForegroundColor Yellow
    }
}

function Get-ConnectionStateValue {
    param(
        [string]$Key
    )

    if (-not (Test-Path $ConnectionStatePath)) {
        return ""
    }

    $line = Get-Content $ConnectionStatePath | Where-Object { $_ -match "^${Key}=" } | Select-Object -First 1
    if ([string]::IsNullOrWhiteSpace($line)) {
        return ""
    }

    $value = $line -replace "^${Key}=", ""
    return $value.Trim("'")
}

# --- Uninstall Logic ---
if ($Uninstall) {
    Write-Host "Uninstalling $AgentName..." -ForegroundColor Cyan

    # Try to notify the Pulse server about uninstallation if we have connection details
    if ([string]::IsNullOrWhiteSpace($Url)) {
        $Url = Get-ConnectionStateValue "PULSE_URL"
    }
    if ([string]::IsNullOrWhiteSpace($Token)) {
        $Token = Get-ConnectionStateValue "PULSE_TOKEN"
    }
    if ([string]::IsNullOrWhiteSpace($Token)) {
        $savedTokenFile = Get-ConnectionStateValue "PULSE_TOKEN_FILE"
        if (-not [string]::IsNullOrWhiteSpace($savedTokenFile) -and (Test-Path $savedTokenFile)) {
            $Token = (Get-Content -Path $savedTokenFile -Raw).Trim()
        }
    }
    if ([string]::IsNullOrWhiteSpace($AgentId)) {
        $AgentId = Get-ConnectionStateValue "PULSE_AGENT_ID"
    }
    if ([string]::IsNullOrWhiteSpace($Hostname)) {
        $Hostname = Get-ConnectionStateValue "PULSE_HOSTNAME"
    }
    if (-not $Insecure) {
        $Insecure = Parse-Bool (Get-ConnectionStateValue "PULSE_INSECURE_SKIP_VERIFY") $Insecure
    }
    if ([string]::IsNullOrWhiteSpace($CACertPath)) {
        $CACertPath = Get-ConnectionStateValue "PULSE_CACERT"
    }
    if ([string]::IsNullOrWhiteSpace($ServerFingerprint)) {
        $ServerFingerprint = Get-ConnectionStateValue "PULSE_SERVER_FINGERPRINT"
    }

    $customCaCertificate = $null
    if (-not [string]::IsNullOrWhiteSpace($CACertPath)) {
        try {
            $customCaCertificate = Load-CustomCaCertificate $CACertPath
        } catch {
            Write-Host "Warning: Failed to load custom CA certificate from $CACertPath during uninstall: $_" -ForegroundColor Yellow
            $customCaCertificate = $null
        }
    }

    if (-not [string]::IsNullOrWhiteSpace($Url)) {
        # Try to recover agent ID if not provided
        $detectedAgentId = $AgentId
        $stateFile = "$StateDir\agent-id"
        if ([string]::IsNullOrWhiteSpace($detectedAgentId) -and (Test-Path $stateFile)) {
            $detectedAgentId = Get-Content $stateFile -Raw
            if ($detectedAgentId) { $detectedAgentId = $detectedAgentId.Trim() }
        }
        if ([string]::IsNullOrWhiteSpace($detectedAgentId)) {
            $lookupHostname = $Hostname
            if ([string]::IsNullOrWhiteSpace($lookupHostname)) {
                $lookupHostname = $env:COMPUTERNAME
            }
            if (-not [string]::IsNullOrWhiteSpace($lookupHostname)) {
                try {
                    $lookupArgs = @{
                        Uri         = "$Url/api/agents/agent/lookup?hostname=$([System.Uri]::EscapeDataString($lookupHostname))"
                        Method      = "Get"
                        TimeoutSec  = 5
                        ErrorAction = "SilentlyContinue"
                    }
                    if (-not [string]::IsNullOrWhiteSpace($Token)) {
                        $lookupArgs.Headers = @{ "X-API-Token" = $Token }
                    }

                    $lookupResult = Invoke-WithOptionalInsecureTls -AllowInsecure $Insecure -CustomCaCertificate $customCaCertificate -Action {
                        Invoke-RestMethod @lookupArgs
                    }
                    if ($lookupResult -and $lookupResult.agent -and -not [string]::IsNullOrWhiteSpace($lookupResult.agent.id)) {
                        $detectedAgentId = $lookupResult.agent.id.Trim()
                    }
                } catch {
                    # Ignore lookup errors during uninstall
                }
            }
        }

        if (-not [string]::IsNullOrWhiteSpace($detectedAgentId)) {
            Write-Host "Notifying Pulse server to unregister agent ID: $detectedAgentId..." -ForegroundColor Gray
            try {
                $body = @{ agentId = $detectedAgentId } | ConvertTo-Json
                $invokeArgs = @{
                    Uri         = "$Url/api/agents/agent/uninstall"
                    Method      = "Post"
                    Body        = $body
                    ContentType = "application/json"
                    TimeoutSec  = 5
                    ErrorAction = "SilentlyContinue"
                }
                if (-not [string]::IsNullOrWhiteSpace($Token)) {
                    $invokeArgs.Headers = @{ "X-API-Token" = $Token }
                }

                Invoke-WithOptionalInsecureTls -AllowInsecure $Insecure -CustomCaCertificate $customCaCertificate -Action {
                    Invoke-RestMethod @invokeArgs | Out-Null
                }
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

    if (Test-Path $StateDir) {
        Remove-Item $StateDir -Recurse -Force -ErrorAction SilentlyContinue
    }

    Remove-Item "$InstallDir\$BinaryName" -Force -ErrorAction SilentlyContinue
    Write-Host "Uninstallation complete." -ForegroundColor Green
    Exit 0
}

if ([string]::IsNullOrWhiteSpace($Token) -and -not [string]::IsNullOrWhiteSpace($TokenFile)) {
    try {
        $resolvedTokenFile = $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($TokenFile)
        if (-not (Test-Path $resolvedTokenFile)) {
            Show-Error "Invalid token file. File does not exist.`nProvided: $TokenFile"
            Exit 1
        }
        $Token = (Get-Content -Path $resolvedTokenFile -Raw -ErrorAction Stop).Trim()
    } catch {
        Show-Error "Failed to read token file.`nProvided: $TokenFile`nError: $_"
        Exit 1
    }
}

# --- Input Validation ---
Write-Host "Validating parameters..." -ForegroundColor Cyan

if (-not (Test-ValidUrl $Url)) {
    Show-Error "Invalid or missing URL. Must be a valid http:// or https:// URL.`nProvided: $Url"
    Exit 1
}

if (-not [string]::IsNullOrWhiteSpace($Token) -and -not (Test-ValidToken $Token)) {
    Show-Error "Invalid Token. Must be 16-256 alphanumeric characters when provided.`nSet PULSE_URL and PULSE_TOKEN environment variables or pass as arguments."
    Exit 1
}

if (-not (Test-ValidInterval $Interval)) {
    Show-Error "Invalid Interval format. Must be like '30s', '1m', '5m'.`nProvided: $Interval"
    Exit 1
}

if (-not [string]::IsNullOrWhiteSpace($CACertPath) -and -not (Test-Path $CACertPath)) {
    Show-Error "Invalid CA certificate path. File does not exist.`nProvided: $CACertPath"
    Exit 1
}

$CustomCaCertificate = $null
if (-not [string]::IsNullOrWhiteSpace($CACertPath)) {
    try {
        $CustomCaCertificate = Load-CustomCaCertificate $CACertPath
    } catch {
        Show-Error "Invalid CA certificate file. Provide a PEM, CRT, or CER certificate.`nPath: $CACertPath`nError: $_"
        Exit 1
    }
}

$NormalizedProxmoxType = $ProxmoxType.Trim().ToLowerInvariant()
if ($NormalizedProxmoxType -eq 'auto') {
    $NormalizedProxmoxType = ''
}
if (-not [string]::IsNullOrWhiteSpace($NormalizedProxmoxType) -and $NormalizedProxmoxType -notin @('pve', 'pbs')) {
    Show-Error "Invalid Proxmox type. Must be 'pve', 'pbs', or 'auto'.`nProvided: $ProxmoxType"
    Exit 1
}

# Normalize URL (remove trailing slash)
$Url = $Url.TrimEnd('/')
$ServerFingerprint = ($ServerFingerprint -replace '[:\s]', '').ToLowerInvariant()
if (-not [string]::IsNullOrWhiteSpace($ServerFingerprint) -and $ServerFingerprint -notmatch '^[a-f0-9]{64}$') {
    Show-Error "Invalid server certificate fingerprint. Expected 64 hexadecimal SHA-256 characters."
    Exit 1
}
if (-not [string]::IsNullOrWhiteSpace($ServerFingerprint) -and -not $Url.ToLowerInvariant().StartsWith("https://")) {
    Show-Error "Server certificate fingerprint pinning requires an https:// Pulse URL."
    Exit 1
}
if ($Url.ToLowerInvariant().StartsWith("http://") -and -not $Insecure) {
    Write-Host "Plain HTTP Pulse URL detected; enabling insecure mode for persisted agent update checks." -ForegroundColor Yellow
    $Insecure = $true
}

# --- Download ---
# Determine architecture
$processorArch = "$env:PROCESSOR_ARCHITECTURE"
$processorArch64 = "$env:PROCESSOR_ARCHITEW6432"
if ($processorArch -eq "ARM64" -or $processorArch64 -eq "ARM64") {
    $Arch = "arm64"
} elseif ([Environment]::Is64BitOperatingSystem) {
    $Arch = "amd64"
} else {
    $Arch = "386"
}
$ArchParam = "windows-$Arch"
$DownloadUrl = "$Url/download/pulse-agent?arch=$ArchParam"
$ServerVersion = ""

try {
    $versionResponse = Invoke-WithOptionalInsecureTls -AllowInsecure $Insecure -CustomCaCertificate $CustomCaCertificate -Action {
        Invoke-WebRequest -Uri "$Url/api/version" -UseBasicParsing -TimeoutSec 10 -ErrorAction Stop
    }
    if ($versionResponse -and $versionResponse.Content) {
        $versionInfo = $versionResponse.Content | ConvertFrom-Json
        if ($versionInfo -and $versionInfo.version) {
            $ServerVersion = [string]$versionInfo.version
            Write-Host "Pulse server version: $ServerVersion" -ForegroundColor Cyan
        }
    }
} catch {
}

if (-not [string]::IsNullOrWhiteSpace($ServerVersion)) {
    $escapedServerVersion = [Uri]::EscapeDataString($ServerVersion)
    $DownloadUrl = "$DownloadUrl&serverVersion=$escapedServerVersion"
}

function Invoke-AgentDownloadPreflight {
    param([string]$Uri)

    try {
        $preflightResponse = Invoke-WithOptionalInsecureTls -AllowInsecure $Insecure -CustomCaCertificate $CustomCaCertificate -Action {
            Invoke-WebRequest -Uri $Uri -Method Head -UseBasicParsing -TimeoutSec 10 -ErrorAction Stop
        }

        $checksum = Get-ResponseHeaderValue -Headers $preflightResponse.Headers -Name "X-Checksum-Sha256"
        if ([string]::IsNullOrWhiteSpace($checksum)) {
            Write-InstallerEvent -Phase "preflight" -Code "agent_download_checksum_missing" -Message "Agent download exists but did not include a checksum header: $Uri" -ExitCode 12
            Exit 12
        }

        Write-InstallerEvent -Phase "preflight" -Code "agent_download_available" -Message "Agent download is available for $ArchParam." -ExitCode 0
    } catch {
        Write-InstallerEvent -Phase "preflight" -Code "agent_download_unavailable" -Message "Agent download is not available for $ArchParam at $Uri. $_" -ExitCode 11
        Exit 11
    }
}

if ($PreflightOnly) {
    Invoke-AgentDownloadPreflight $DownloadUrl
    return
}

Write-Host "Downloading agent from $DownloadUrl..." -ForegroundColor Cyan

if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
}

# Download to temp file first
$TempPath = [System.IO.Path]::GetTempFileName() + ".exe"
$script:TempFiles += $TempPath
$DestPath = "$InstallDir\$BinaryName"
$serverChecksum = $null
$serverSshSignature = $null

try {
    # Configure TLS 1.2 minimum
    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12 -bor [Net.SecurityProtocolType]::Tls13

    $downloadPreflightResponse = Invoke-WithOptionalInsecureTls -AllowInsecure $Insecure -CustomCaCertificate $CustomCaCertificate -Action {
        Invoke-WebRequest -Uri $DownloadUrl -Method Head -UseBasicParsing -TimeoutSec 10 -ErrorAction Stop
    }
    $serverChecksum = Get-ResponseHeaderValue -Headers $downloadPreflightResponse.Headers -Name "X-Checksum-Sha256"
    $serverSshSignature = Get-ResponseHeaderValue -Headers $downloadPreflightResponse.Headers -Name $InstallerSignatureHeaderName
    if ([string]::IsNullOrWhiteSpace($serverChecksum)) {
        Cleanup
        Show-Error "Server did not provide checksum header; refusing install."
        Exit 1
    }

    $downloadMetadata = Invoke-WithOptionalInsecureTls -AllowInsecure $Insecure -CustomCaCertificate $CustomCaCertificate -Action {
        # Download with timeout
        $webClient = New-Object System.Net.WebClient
        try {
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

            # Prefer the same-download headers if Windows exposes them; otherwise
            # retain the HEAD metadata captured immediately before download.
            $downloadChecksum = Get-ResponseHeaderValue -Headers $webClient.ResponseHeaders -Name "X-Checksum-Sha256"
            $downloadSshSignature = Get-ResponseHeaderValue -Headers $webClient.ResponseHeaders -Name $InstallerSignatureHeaderName
            if (-not [string]::IsNullOrWhiteSpace($downloadChecksum)) {
                $serverChecksum = $downloadChecksum
            }
            if (-not [string]::IsNullOrWhiteSpace($downloadSshSignature)) {
                $serverSshSignature = $downloadSshSignature
            }

            @{
                Checksum = $serverChecksum
                SshSignature = $serverSshSignature
            }
        } finally {
            $webClient.Dispose()
        }
    }
    if ($downloadMetadata -and -not [string]::IsNullOrWhiteSpace($downloadMetadata.Checksum)) {
        $serverChecksum = $downloadMetadata.Checksum
    }
    if ($downloadMetadata -and -not [string]::IsNullOrWhiteSpace($downloadMetadata.SshSignature)) {
        $serverSshSignature = $downloadMetadata.SshSignature
    }

} catch {
    Cleanup
    Show-Error "Failed to download agent: $_"
    if (-not $NonInteractive) {
        Write-Host ""
        Write-Host "Press Enter to exit..." -ForegroundColor Yellow
        Read-Host
    }
    Exit 1
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

# Verify checksum from the same download response.
if ([string]::IsNullOrWhiteSpace($serverChecksum)) {
    Cleanup
    Show-Error "Server did not provide checksum header; refusing install."
    Exit 1
}
$localChecksum = Get-FileChecksum $TempPath
if ($localChecksum -ne $serverChecksum.ToLower()) {
    Cleanup
    Show-Error "Checksum verification failed!`nExpected: $serverChecksum`nGot: $localChecksum"
    Exit 1
}
Write-Host "Checksum verified: $localChecksum" -ForegroundColor Green

if (Test-HasPinnedInstallerSignatureKey) {
    if ([string]::IsNullOrWhiteSpace($serverChecksum)) {
        Cleanup
        Show-Error "Server did not provide checksum header; refusing signed install."
        Exit 1
    }
    if ([string]::IsNullOrWhiteSpace($serverSshSignature)) {
        Cleanup
        Show-Error "Server did not provide SSH signature header; refusing signed install."
        Exit 1
    }
    try {
        Invoke-InstallerSignatureVerification -FilePath $TempPath -SignatureHeader $serverSshSignature
    } catch {
        Cleanup
        Show-Error "$_"
        Exit 1
    }
}

$DownloadedVersion = ""
try {
    $DownloadedVersion = ((& $TempPath --version 2>$null) | Select-Object -First 1).Trim()
} catch {
    $DownloadedVersion = ""
}

$DownloadedVersionNormalized = $DownloadedVersion.TrimStart('v')
$ServerVersionNormalized = $ServerVersion.TrimStart('v')

if (-not [string]::IsNullOrWhiteSpace($ServerVersion) -and -not [string]::IsNullOrWhiteSpace($DownloadedVersion) -and $DownloadedVersionNormalized -ne $ServerVersionNormalized) {
    Write-Host "Warning: downloaded agent version ($DownloadedVersion) does not match Pulse server version ($ServerVersion). Check that Pulse is upgraded and that any reverse proxy is not serving a stale cached binary." -ForegroundColor Yellow
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
Write-RuntimeTokenFile
Save-ConnectionState

# Build command line args (properly escaped)
$ServiceArgs = @(
    "--url", "`"$Url`"",
    "--interval", "`"$Interval`""
)
if (-not [string]::IsNullOrWhiteSpace($Token)) { $ServiceArgs += @("--token-file", "`"$TokenFilePath`"") }
if ($EnableHost) { $ServiceArgs += "--enable-host" } else { $ServiceArgs += "--enable-host=false" }
if ($EnableDocker) { $ServiceArgs += "--enable-docker" }
if ($EnableKubernetes) { $ServiceArgs += "--enable-kubernetes" }
if ($EnableProxmox) { $ServiceArgs += "--enable-proxmox" }
if (-not [string]::IsNullOrWhiteSpace($NormalizedProxmoxType)) { $ServiceArgs += @("--proxmox-type", "`"$NormalizedProxmoxType`"") }
if ($EnableCommands) { $ServiceArgs += "--enable-commands" }
if ($Insecure) { $ServiceArgs += "--insecure" }
if (-not [string]::IsNullOrWhiteSpace($CACertPath)) { $ServiceArgs += @("--cacert", "`"$CACertPath`"") }
if (-not [string]::IsNullOrWhiteSpace($ServerFingerprint)) { $ServiceArgs += @("--server-fingerprint", "`"$ServerFingerprint`"") }
if (-not [string]::IsNullOrWhiteSpace($AgentId)) { $ServiceArgs += @("--agent-id", "`"$AgentId`"") }
if (-not [string]::IsNullOrWhiteSpace($Hostname)) { $ServiceArgs += @("--hostname", "`"$Hostname`"") }
$ServiceArgs += @("--log-file", "`"$LogFile`"")

$BinPath = "`"$DestPath`" $($ServiceArgs -join ' ')"

# Create the state/log directory before SCM starts the service so the agent can
# establish its rotating file sink as part of startup.
$LogDir = Split-Path $LogFile -Parent
if (-not (Test-Path $LogDir)) {
    New-Item -ItemType Directory -Force -Path $LogDir | Out-Null
}

# Create Service using New-Service (more reliable than sc.exe create)
try {
    New-Service -Name $AgentName `
                -BinaryPathName $BinPath `
                -DisplayName "Pulse Unified Agent" `
                -Description "Pulse Unified Agent for Host, Docker, Kubernetes, and Proxmox monitoring" `
                -StartupType Automatic | Out-Null
    Write-Host "Service created successfully" -ForegroundColor Green
} catch {
    Show-Error "Failed to create service '$AgentName'.`nError: $_"
    Exit 1
}

$scOutput = sc.exe failure $AgentName reset= 86400 actions= restart/5000/restart/5000/restart/5000 2>&1
if ($LASTEXITCODE -ne 0) {
    Show-Error "Failed to configure service recovery actions: $scOutput"
    Exit 1
}

$scOutput = sc.exe failureflag $AgentName 1 2>&1
if ($LASTEXITCODE -ne 0) {
    Show-Error "Failed to enable service recovery for non-crash failures: $scOutput"
    Exit 1
}

# Start the service
try {
    Start-Service $AgentName -ErrorAction Stop
    Write-Host "Service started successfully" -ForegroundColor Green
} catch {
    Show-Error "Failed to start service '$AgentName': $_"
    Exit 1
}

# Verify agent is running and healthy
Write-Host "Verifying agent started successfully..." -ForegroundColor Cyan
$healthUrl = "http://127.0.0.1:9191/readyz"
$maxIterations = 8
$interval = 2
$healthy = $false

Start-Sleep -Seconds 2

for ($i = 0; $i -lt $maxIterations; $i++) {
    try {
        $response = Invoke-WebRequest -Uri $healthUrl -UseBasicParsing -TimeoutSec 2 -ErrorAction Stop
        $logReady = (Test-Path $LogFile) -and ((Get-Item $LogFile).Length -gt 0)
        if ($response.StatusCode -eq 200 -and $logReady) {
            $healthy = $true
            break
        }
    } catch {
        # Health endpoint not ready yet
    }

    # Check if the service process is still alive after a grace period
    if ($i -ge 3) {
        $svc = Get-Service -Name $AgentName -ErrorAction SilentlyContinue
        if (-not $svc -or ($svc.Status -ne 'Running' -and $svc.Status -ne 'StartPending')) {
            $statusMsg = if ($svc) { $svc.Status } else { "not found" }
            Write-Host "WARNING: Agent service is not running (status: $statusMsg)!" -ForegroundColor Yellow
            if (Test-Path $LogFile) {
                Write-Host "Last log lines:" -ForegroundColor Yellow
                Get-Content $LogFile -Tail 5 | ForEach-Object { Write-Host "  $_" -ForegroundColor Yellow }
            }
            break
        }
    }

    Start-Sleep -Seconds $interval
}

Write-Host ""
if ($healthy) {
    Write-Host "Installation complete! Agent is running." -ForegroundColor Green
} else {
    Show-Error "Installation did not reach a healthy, logged runtime. Check logs with: Get-Content '$LogFile' -Tail 50"
    Exit 1
}
Write-Host "Service: $AgentName"
Write-Host "Binary:  $DestPath"
Write-Host "Logs:    $LogFile"
Write-Host ""
