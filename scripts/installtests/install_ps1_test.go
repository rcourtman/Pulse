package installtests

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestInstallPS1ParsesWithPowerShell(t *testing.T) {
	pwsh, err := exec.LookPath("pwsh")
	if err != nil {
		t.Skip("pwsh not installed")
	}

	scriptPath := repoFile("scripts", "install.ps1")
	cmd := exec.Command(pwsh,
		"-NoLogo",
		"-NoProfile",
		"-NonInteractive",
		"-Command",
		`$errors = $null; [System.Management.Automation.Language.Parser]::ParseFile($env:PULSE_INSTALL_PS1_PATH, [ref]$null, [ref]$errors) > $null; if ($errors.Count) { $errors | ForEach-Object { Write-Error $_.ToString() }; exit 1 }`,
	)
	cmd.Env = append(os.Environ(), "PULSE_INSTALL_PS1_PATH="+scriptPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("install.ps1 failed PowerShell parser check: %v\n%s", err, output)
	}
}

func TestInstallPS1KeepsTLS13OptionalOnLegacyWindowsPowerShell(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "install.ps1"))
	if err != nil {
		t.Fatalf("read install.ps1: %v", err)
	}

	script := string(content)
	tls12Fallback := `[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12`
	optionalTLS13 := `[Net.SecurityProtocolType]::Tls12 -bor [Net.SecurityProtocolType]::Tls13`
	tls12Index := strings.Index(script, tls12Fallback)
	tls13Index := strings.Index(script, optionalTLS13)
	if tls12Index == -1 {
		t.Fatal("install.ps1 must establish TLS 1.2 before probing optional TLS 1.3 support")
	}
	if tls13Index == -1 {
		t.Fatal("install.ps1 must enable TLS 1.3 when the runtime exposes it")
	}
	if tls12Index >= tls13Index {
		t.Fatal("install.ps1 must establish the TLS 1.2 fallback before referencing the optional TLS 1.3 enum")
	}

	compatibilityBlock := script[tls12Index:tls13Index]
	if !strings.Contains(compatibilityBlock, "try {") {
		t.Fatal("install.ps1 must guard the optional TLS 1.3 enum behind a compatibility try block")
	}
	if !strings.Contains(script[tls13Index:], "} catch {") {
		t.Fatal("install.ps1 must retain TLS 1.2 when the optional TLS 1.3 enum is unavailable")
	}
}

func TestWindowsAgentLifecycleHarnessParsesWithPowerShell(t *testing.T) {
	pwsh, err := exec.LookPath("pwsh")
	if err != nil {
		t.Skip("pwsh not installed")
	}

	scriptPath := repoFile("scripts", "installtests", "windows_agent_lifecycle.ps1")
	cmd := exec.Command(pwsh,
		"-NoLogo",
		"-NoProfile",
		"-NonInteractive",
		"-Command",
		`$errors = $null; [System.Management.Automation.Language.Parser]::ParseFile($env:PULSE_WINDOWS_LIFECYCLE_PATH, [ref]$null, [ref]$errors) > $null; if ($errors.Count) { $errors | ForEach-Object { Write-Error $_.ToString() }; exit 1 }`,
	)
	cmd.Env = append(os.Environ(), "PULSE_WINDOWS_LIFECYCLE_PATH="+scriptPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Windows lifecycle harness failed PowerShell parser check: %v\n%s", err, output)
	}
}

func TestWindowsAgentLifecycleHarnessPinsCompleteServiceProof(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "installtests", "windows_agent_lifecycle.ps1"))
	if err != nil {
		t.Fatalf("read Windows lifecycle harness: %v", err)
	}

	script := string(content)
	required := []string{
		`[ValidateSet('Full', 'InstallUpdate', 'PostRebootUninstall')]`,
		`Pass -ConfirmLifecycleMutation`,
		`-PreflightOnly ` + "`" + `$true`,
		`Assert-AgentRuntime -ExpectedVersion $versionV1`,
		`Assert-AgentRuntime -ExpectedVersion $versionV2`,
		`Assert-CrashRecovery -ExpectedVersion $versionV2`,
		`Post-reboot persistence and uninstall proof passed.`,
		`PulseAgent service still exists after uninstall.`,
		`Pulse Agent state directory still exists after uninstall.`,
	}
	for _, needle := range required {
		if !strings.Contains(script, needle) {
			t.Fatalf("Windows lifecycle harness missing proof contract: %s", needle)
		}
	}
}

func TestInstallPS1OwnsWindowsServiceLoggingAndRecovery(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "install.ps1"))
	if err != nil {
		t.Fatalf("read install.ps1: %v", err)
	}

	script := string(content)
	required := []string{
		`$ServiceArgs += @("--log-file", "` + "`" + `"$LogFile` + "`" + `"")`,
		`"--state-dir", "` + "`" + `"$StateDir` + "`" + `"`,
		`$scOutput = sc.exe failure $AgentName reset= 86400 actions= restart/5000/restart/5000/restart/5000`,
		`Show-Error "Failed to configure service recovery actions: $scOutput"`,
		`$scOutput = sc.exe failureflag $AgentName 1`,
		`Show-Error "Failed to enable service recovery for non-crash failures: $scOutput"`,
		`$logReady = (Test-Path $LogFile) -and ((Get-Item $LogFile).Length -gt 0)`,
		`if ($response.StatusCode -eq 200 -and $logReady)`,
		`Installation did not reach a healthy, logged runtime.`,
	}
	for _, needle := range required {
		if !strings.Contains(script, needle) {
			t.Fatalf("install.ps1 missing Windows service logging/recovery contract: %s", needle)
		}
	}
}

func TestInstallPS1DockerModeDefaultsHostOff(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "install.ps1"))
	if err != nil {
		t.Fatalf("read install.ps1: %v", err)
	}

	script := string(content)
	required := []string{
		`if ($EnableDocker -and -not $PSBoundParameters.ContainsKey('EnableHost') -and [string]::IsNullOrWhiteSpace($env:PULSE_ENABLE_HOST)) {`,
		`$EnableHost = $false`,
		`if ($EnableHost) { $ServiceArgs += "--enable-host" } else { $ServiceArgs += "--enable-host=false" }`,
	}
	for _, needle := range required {
		if !strings.Contains(script, needle) {
			t.Fatalf("install.ps1 missing docker/host parity guard: %s", needle)
		}
	}
}

func TestInstallPS1PersistsAndVerifiesServerFingerprint(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "install.ps1"))
	if err != nil {
		t.Fatalf("read install.ps1: %v", err)
	}

	script := string(content)
	required := []string{
		`[string]$ServerFingerprint = $env:PULSE_SERVER_FINGERPRINT,`,
		`$sha256.ComputeHash($certificate.GetRawCertData())`,
		`$lines += "PULSE_SERVER_FINGERPRINT='$ServerFingerprint'"`,
		`$ServerFingerprint = Get-ConnectionStateValue "PULSE_SERVER_FINGERPRINT"`,
		`$ServiceArgs += @("--server-fingerprint", "` + "`" + `"$ServerFingerprint` + "`" + `"")`,
	}
	for _, needle := range required {
		if !strings.Contains(script, needle) {
			t.Fatalf("install.ps1 missing server-fingerprint lifecycle contract: %s", needle)
		}
	}
}

func TestInstallPS1PreservesObserverDestinationConfig(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "install.ps1"))
	if err != nil {
		t.Fatalf("read install.ps1: %v", err)
	}

	script := string(content)
	required := []string{
		`[string]$ObserversFile = $env:PULSE_OBSERVERS_FILE,`,
		`$lines += "PULSE_OBSERVERS_FILE='$ObserversFile'"`,
		`[System.IO.Path]::IsPathRooted($ObserversFile)`,
		`Test-Path $ObserversFile -PathType Leaf`,
		`$ServiceArgs += @("--observers-file", "` + "`" + `"$ObserversFile` + "`" + `"")`,
	}
	for _, needle := range required {
		if !strings.Contains(script, needle) {
			t.Fatalf("install.ps1 missing observer-config lifecycle contract: %s", needle)
		}
	}
}

func TestInstallPS1AllowsMissingTokenForOptionalAuth(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "install.ps1"))
	if err != nil {
		t.Fatalf("read install.ps1: %v", err)
	}

	script := string(content)
	required := []string{
		`[string]$TokenFile = $env:PULSE_TOKEN_FILE,`,
		`if (-not [string]::IsNullOrWhiteSpace($Token) -and -not (Test-ValidToken $Token)) {`,
		`if ([string]::IsNullOrWhiteSpace($Token) -and -not [string]::IsNullOrWhiteSpace($TokenFile)) {`,
		`$Token = (Get-Content -Path $resolvedTokenFile -Raw -ErrorAction Stop).Trim()`,
		`function Write-RuntimeTokenFile {`,
		`if (-not [string]::IsNullOrWhiteSpace($Token)) { $ServiceArgs += @("--token-file", "` + "`" + `"$TokenFilePath` + "`" + `"") }`,
	}
	for _, needle := range required {
		if !strings.Contains(script, needle) {
			t.Fatalf("install.ps1 missing optional-token handling: %s", needle)
		}
	}
}

func TestInstallPS1AgentDownloadIsServerVersionAware(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "install.ps1"))
	if err != nil {
		t.Fatalf("read install.ps1: %v", err)
	}

	script := string(content)
	required := []string{
		`Invoke-WebRequest -Uri "$Url/api/version"`,
		`$versionInfo = $versionResponse.Content | ConvertFrom-Json`,
		`$ServerVersion = [string]$versionInfo.version`,
		`$escapedServerVersion = [Uri]::EscapeDataString($ServerVersion)`,
		`$DownloadUrl = "$DownloadUrl&serverVersion=$escapedServerVersion"`,
		`downloaded agent version ($DownloadedVersion) does not match Pulse server version ($ServerVersion)`,
	}
	for _, needle := range required {
		if !strings.Contains(script, needle) {
			t.Fatalf("install.ps1 missing version-aware agent download behavior: %s", needle)
		}
	}
}

func TestInstallPS1AllowsOptionalAuthUninstallWithoutToken(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "install.ps1"))
	if err != nil {
		t.Fatalf("read install.ps1: %v", err)
	}

	script := string(content)
	required := []string{
		`if (-not [string]::IsNullOrWhiteSpace($Url)) {`,
		`$invokeArgs = @{`,
		`if (-not [string]::IsNullOrWhiteSpace($Token)) {`,
		`$invokeArgs.Headers = @{ "X-API-Token" = $Token }`,
		`Invoke-RestMethod @invokeArgs | Out-Null`,
	}
	for _, needle := range required {
		if !strings.Contains(script, needle) {
			t.Fatalf("install.ps1 missing optional-auth uninstall handling: %s", needle)
		}
	}
}

func TestInstallPS1PreservesProxmoxProfileParity(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "install.ps1"))
	if err != nil {
		t.Fatalf("read install.ps1: %v", err)
	}

	script := string(content)
	required := []string{
		`[bool]$EnableProxmox = $false,`,
		`[string]$ProxmoxType = "",`,
		`if (-not $PSBoundParameters.ContainsKey('EnableProxmox') -and -not [string]::IsNullOrWhiteSpace($env:PULSE_ENABLE_PROXMOX)) {`,
		`$EnableProxmox = Parse-Bool $env:PULSE_ENABLE_PROXMOX $EnableProxmox`,
		`if (-not $PSBoundParameters.ContainsKey('ProxmoxType') -and -not [string]::IsNullOrWhiteSpace($env:PULSE_PROXMOX_TYPE)) {`,
		`$ProxmoxType = $env:PULSE_PROXMOX_TYPE`,
		`if ($EnableProxmox) { $ServiceArgs += "--enable-proxmox" }`,
		`if (-not [string]::IsNullOrWhiteSpace($NormalizedProxmoxType)) { $ServiceArgs += @("--proxmox-type", "` + "`" + `"$NormalizedProxmoxType` + "`" + `"") }`,
	}
	for _, needle := range required {
		if !strings.Contains(script, needle) {
			t.Fatalf("install.ps1 missing proxmox profile parity: %s", needle)
		}
	}
}

func TestInstallPS1PreservesCommandExecutionParity(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "install.ps1"))
	if err != nil {
		t.Fatalf("read install.ps1: %v", err)
	}

	script := string(content)
	required := []string{
		`[bool]$EnableCommands = $false,`,
		`if (-not $PSBoundParameters.ContainsKey('EnableCommands') -and -not [string]::IsNullOrWhiteSpace($env:PULSE_ENABLE_COMMANDS)) {`,
		`$EnableCommands = Parse-Bool $env:PULSE_ENABLE_COMMANDS $EnableCommands`,
		`if ($EnableCommands) { $ServiceArgs += "--enable-commands" }`,
	}
	for _, needle := range required {
		if !strings.Contains(script, needle) {
			t.Fatalf("install.ps1 missing command-execution parity: %s", needle)
		}
	}
}

func TestInstallPS1UsesInsecureTlsForRuntimeTransport(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "install.ps1"))
	if err != nil {
		t.Fatalf("read install.ps1: %v", err)
	}

	script := string(content)
	required := []string{
		`function Invoke-WithOptionalInsecureTls {`,
		`if ($AllowInsecure -or $null -ne $CustomCaCertificate -or -not [string]::IsNullOrWhiteSpace($ServerFingerprint)) {`,
		`if ($AllowInsecure) {`,
		`return Test-CertificateTrustedByCustomCa -Certificate $certificate -CustomCaCertificate $CustomCaCertificate`,
		`if ($Url.ToLowerInvariant().StartsWith("http://") -and -not $Insecure) {`,
		`Plain HTTP Pulse URL detected; enabling insecure mode for persisted agent update checks.`,
		`Invoke-WithOptionalInsecureTls -AllowInsecure $Insecure -CustomCaCertificate $CustomCaCertificate -Action {`,
		`Invoke-RestMethod @invokeArgs | Out-Null`,
		`$downloadTask = $webClient.DownloadFileTaskAsync($DownloadUrl, $TempPath)`,
	}
	for _, needle := range required {
		if !strings.Contains(script, needle) {
			t.Fatalf("install.ps1 missing insecure runtime transport handling: %s", needle)
		}
	}
}

func TestInstallPS1SupportsDownloadPreflightBeforeAdministratorInstall(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "install.ps1"))
	if err != nil {
		t.Fatalf("read install.ps1: %v", err)
	}

	script := string(content)
	required := []string{
		`[bool]$PreflightOnly = $false,`,
		`[string]$Output = $env:PULSE_OUTPUT,`,
		`[bool]$NonInteractive = $false`,
		`if (-not $isAdmin -and -not $PreflightOnly) {`,
		`function Write-InstallerEvent {`,
		`function Get-ResponseHeaderValue {`,
		`function Invoke-AgentDownloadPreflight {`,
		`Invoke-WebRequest -Uri $Uri -Method Head -UseBasicParsing -TimeoutSec 10 -ErrorAction Stop`,
		`$checksum = Get-ResponseHeaderValue -Headers $preflightResponse.Headers -Name "X-Checksum-Sha256"`,
		`$downloadPreflightResponse = Invoke-WithOptionalInsecureTls -AllowInsecure $Insecure -CustomCaCertificate $CustomCaCertificate -Action {`,
		`Invoke-WebRequest -Uri $DownloadUrl -Method Head -UseBasicParsing -TimeoutSec 10 -ErrorAction Stop`,
		`$serverChecksum = Get-ResponseHeaderValue -Headers $downloadPreflightResponse.Headers -Name "X-Checksum-Sha256"`,
		`$downloadMetadata = Invoke-WithOptionalInsecureTls -AllowInsecure $Insecure -CustomCaCertificate $CustomCaCertificate -Action {`,
		`$downloadChecksum = Get-ResponseHeaderValue -Headers $webClient.ResponseHeaders -Name "X-Checksum-Sha256"`,
		`agent_download_checksum_missing`,
		`agent_download_available`,
		`agent_download_unavailable`,
		`if ($PreflightOnly) {`,
		`Invoke-AgentDownloadPreflight $DownloadUrl`,
		`if (-not $NonInteractive) {`,
	}
	for _, needle := range required {
		if !strings.Contains(script, needle) {
			t.Fatalf("install.ps1 missing download preflight handling: %s", needle)
		}
	}
}

func TestInstallPS1ReadsAgentIdentityFromEnvironment(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "install.ps1"))
	if err != nil {
		t.Fatalf("read install.ps1: %v", err)
	}

	script := string(content)
	required := []string{
		`[string]$AgentId = $env:PULSE_AGENT_ID,`,
		`[string]$Hostname = $env:PULSE_HOSTNAME`,
	}
	for _, needle := range required {
		if !strings.Contains(script, needle) {
			t.Fatalf("install.ps1 missing agent identity env handling: %s", needle)
		}
	}
}

func TestInstallPS1UsesHostnameLookupForUninstallFallback(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "install.ps1"))
	if err != nil {
		t.Fatalf("read install.ps1: %v", err)
	}

	script := string(content)
	required := []string{
		`$lookupHostname = $Hostname`,
		`$lookupHostname = $env:COMPUTERNAME`,
		`Uri         = "$Url/api/agents/agent/lookup?hostname=$([System.Uri]::EscapeDataString($lookupHostname))"`,
		`$lookupResult = Invoke-WithOptionalInsecureTls -AllowInsecure $Insecure -CustomCaCertificate $customCaCertificate -Action {`,
		`Invoke-RestMethod @lookupArgs`,
	}
	for _, needle := range required {
		if !strings.Contains(script, needle) {
			t.Fatalf("install.ps1 missing uninstall hostname lookup fallback: %s", needle)
		}
	}
}

func TestInstallPS1PersistsAndRecoversConnectionIdentity(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "install.ps1"))
	if err != nil {
		t.Fatalf("read install.ps1: %v", err)
	}

	script := string(content)
	required := []string{
		`$ConnectionStatePath = "$StateDir\connection.env"`,
		`$TokenFilePath = "$StateDir\token"`,
		`Set-Content -Path $ConnectionStatePath -Value ($lines -join "` + "`n" + `") -Encoding UTF8`,
		`$lines += "PULSE_TOKEN_FILE='$TokenFilePath'"`,
		`$Url = Get-ConnectionStateValue "PULSE_URL"`,
		`$Token = Get-ConnectionStateValue "PULSE_TOKEN"`,
		`$savedTokenFile = Get-ConnectionStateValue "PULSE_TOKEN_FILE"`,
		`$AgentId = Get-ConnectionStateValue "PULSE_AGENT_ID"`,
		`$Hostname = Get-ConnectionStateValue "PULSE_HOSTNAME"`,
		`$Insecure = Parse-Bool (Get-ConnectionStateValue "PULSE_INSECURE_SKIP_VERIFY") $Insecure`,
		`$CACertPath = Get-ConnectionStateValue "PULSE_CACERT"`,
		`$lines += "PULSE_INSECURE_SKIP_VERIFY='true'"`,
		`$lines += "PULSE_CACERT='$CACertPath'"`,
		`Write-RuntimeTokenFile`,
		`Save-ConnectionState`,
	}
	for _, needle := range required {
		if !strings.Contains(script, needle) {
			t.Fatalf("install.ps1 missing persisted connection identity handling: %s", needle)
		}
	}
}

func TestInstallPS1SupportsCustomCATransport(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "install.ps1"))
	if err != nil {
		t.Fatalf("read install.ps1: %v", err)
	}

	script := string(content)
	required := []string{
		`[string]$CACertPath = $env:PULSE_CACERT,`,
		`function Load-CustomCaCertificate {`,
		`return [System.Security.Cryptography.X509Certificates.X509Certificate2]::new($resolvedPath)`,
		`function Test-CertificateTrustedByCustomCa {`,
		`$chain.ChainPolicy.ExtraStore.Add($CustomCaCertificate)`,
		`Invoke-WithOptionalInsecureTls -AllowInsecure $Insecure -CustomCaCertificate $CustomCaCertificate -Action {`,
		`Show-Error "Invalid CA certificate path. File does not exist.`,
		`Show-Error "Invalid CA certificate file. Provide a PEM, CRT, or CER certificate.`,
		`if (-not [string]::IsNullOrWhiteSpace($CACertPath)) { $ServiceArgs += @("--cacert", "` + "`" + `"$CACertPath` + "`" + `"") }`,
	}
	for _, needle := range required {
		if !strings.Contains(script, needle) {
			t.Fatalf("install.ps1 missing custom CA transport handling: %s", needle)
		}
	}
}

func TestInstallPS1ClearsPersistedStateAfterUninstall(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "install.ps1"))
	if err != nil {
		t.Fatalf("read install.ps1: %v", err)
	}

	script := string(content)
	required := []string{
		`if (Test-Path $StateDir) {`,
		`Remove-Item $StateDir -Recurse -Force -ErrorAction SilentlyContinue`,
		`Write-Host "Uninstallation complete." -ForegroundColor Green`,
	}
	for _, needle := range required {
		if !strings.Contains(script, needle) {
			t.Fatalf("install.ps1 missing persisted state cleanup after uninstall: %s", needle)
		}
	}
}

func TestInstallPS1RequiresPinnedSignatureVerificationForReleaseDownloads(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "install.ps1"))
	if err != nil {
		t.Fatalf("read install.ps1: %v", err)
	}

	script := string(content)
	required := []string{
		`$PinnedInstallerSshPublicKey = "__PULSE_INSTALLER_SSH_PUBLIC_KEY__"`,
		`function Test-HasPinnedInstallerSignatureKey {`,
		`function Invoke-InstallerSignatureVerification {`,
		`Get-Command ssh-keygen.exe -ErrorAction SilentlyContinue`,
		`$serverSshSignature = Get-ResponseHeaderValue -Headers $downloadPreflightResponse.Headers -Name $InstallerSignatureHeaderName`,
		`Show-Error "Server did not provide checksum header; refusing signed install."`,
		`Show-Error "Server did not provide SSH signature header; refusing signed install."`,
		`Invoke-InstallerSignatureVerification -FilePath $TempPath -SignatureHeader $serverSshSignature`,
		`-Y verify -f`,
	}
	for _, needle := range required {
		if !strings.Contains(script, needle) {
			t.Fatalf("install.ps1 missing signed-download verification contract: %s", needle)
		}
	}
}
