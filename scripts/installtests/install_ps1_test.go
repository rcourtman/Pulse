package installtests

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
)

// Windows proof is specifically the supported legacy engine. Missing or newer
// PowerShell must not silently skip (or stand in for) Windows PowerShell 5.1.
func nativeInstallerPowerShell(t *testing.T) string {
	t.Helper()
	binary := "pwsh"
	if runtime.GOOS == "windows" {
		binary = "powershell.exe"
	}
	powerShell, err := exec.LookPath(binary)
	if err != nil {
		if runtime.GOOS == "windows" {
			t.Fatal("native Windows installer proof requires Windows PowerShell 5.1")
		}
		t.Skip("PowerShell unavailable; native Windows installer proof remains unexecuted")
	}
	if runtime.GOOS == "windows" {
		cmd := exec.Command(powerShell, "-NoLogo", "-NoProfile", "-NonInteractive", "-Command",
			`if ($PSVersionTable.PSEdition -ne 'Desktop' -or $PSVersionTable.PSVersion.Major -ne 5 -or $PSVersionTable.PSVersion.Minor -ne 1) { throw 'Windows PowerShell 5.1 required' }; Write-Output $PSVersionTable.PSVersion.ToString()`)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Windows installer proof selected the wrong engine: %v\n%s", err, output)
		}
		t.Logf("Windows PowerShell %s", strings.TrimSpace(string(output)))
	}
	return powerShell
}

func TestInstallPS1ParsesWithPowerShell(t *testing.T) {
	pwsh := nativeInstallerPowerShell(t)

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
	pwsh := nativeInstallerPowerShell(t)

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

func TestNativeWindowsSelfTestDoesNotPreseedLifecycleState(t *testing.T) {
	content, err := os.ReadFile(repoFile(".github", "workflows", "unified-agent-native.yml"))
	if err != nil {
		t.Fatalf("read native agent workflow: %v", err)
	}

	workflow := string(content)
	required := []string{
		`uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1`,
		`node-version: '24'`,
		`$selfTestStateDir = Join-Path $env:RUNNER_TEMP 'pulse-agent-self-test'`,
		`$selfTestLogFile = Join-Path $selfTestStateDir 'pulse-agent.log'`,
		`--self-test --state-dir $selfTestStateDir --log-file $selfTestLogFile`,
	}
	for _, needle := range required {
		if !strings.Contains(workflow, needle) {
			t.Fatalf("native Windows workflow must keep its governed lifecycle proof intact: %s", needle)
		}
	}
	if strings.Contains(workflow, "Remove-Item -Path $lifecycleStateDir") ||
		strings.Contains(workflow, "Get-Service -Name 'PulseAgent' -ErrorAction SilentlyContinue") {
		t.Fatal("native proof must reject pre-existing or unknown state, not erase it")
	}

}

func TestNativeWindowsExecutesGeneratedInstallCommand(t *testing.T) {
	content, err := os.ReadFile(repoFile(".github", "workflows", "unified-agent-native.yml"))
	if err != nil {
		t.Fatalf("read native agent workflow: %v", err)
	}

	workflow := string(content)
	for _, needle := range []string{
		`frontend-modern/src/utils/agentInstallCommand.ts`,
		`Execute generated command with Windows PowerShell 5.1`,
		`uses: actions/setup-node@820762786026740c76f36085b0efc47a31fe5020 # v7.0.0`,
		`node-version: '24'`,
		`cache-dependency-path: 'frontend-modern/package-lock.json'`,
		`agentInstallCommand.windows.test.ts`,
	} {
		if !strings.Contains(workflow, needle) {
			t.Fatalf("native Windows workflow missing generated install-command proof: %s", needle)
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
		`sha256.ComputeHash(certificate.GetRawCertData())`,
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
		`[string]$CommandAuthority = $env:PULSE_COMMAND_AUTHORITY,`,
		`if (-not $PSBoundParameters.ContainsKey('EnableCommands') -and -not [string]::IsNullOrWhiteSpace($env:PULSE_ENABLE_COMMANDS)) {`,
		`$EnableCommands = Parse-Bool $env:PULSE_ENABLE_COMMANDS $EnableCommands`,
		`if ($EnableCommands) { $ServiceArgs += "--enable-commands" }`,
		`$ServiceArgs += @("--command-authority", "` + "`" + `"$CommandAuthority` + "`" + `"")`,
		`$CommandAuthority = 'monitoring-only'`,
		`$CommandAuthority = 'legacy'`,
		`$CommandAuthority = 'command-capable'`,
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
		`$needsCallback = $AllowInsecure -or $null -ne $CustomCaCertificate -or -not [string]::IsNullOrWhiteSpace($ServerFingerprint)`,
		// The callback must be a compiled delegate, not a scriptblock:
		// ServicePointManager can invoke it on a worker thread with no
		// PowerShell runspace, where a scriptblock fails closed.
		`[System.Net.ServicePointManager]::ServerCertificateValidationCallback = [PulseAgentInstallTlsValidator]::Callback`,
		`[PulseAgentInstallTlsValidator]::AllowInsecure = $AllowInsecure`,
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
		`public static class PulseAgentInstallTlsValidator`,
		`candidateChain.ChainPolicy.ExtraStore.Add(CustomCa);`,
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
		`Remove-Item $StateDir -Recurse -Force -ErrorAction Stop`,
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

// These native exit checks live with the registry-named Windows lifecycle
// completion proof. Their shared parsers and native execution controls remain
// in native_windows_exit_test.go; no check or adverse control is removed.
func TestWindowsAgentLifecycleWorkflowChecksEveryNativeExit(t *testing.T) {
	steps := nativeWindowsExitSteps(t)
	guards, err := nativeWindowsExitGuards(steps)
	if err != nil {
		t.Fatal(err)
	}
	for _, guard := range guards {
		t.Run(guard, func(t *testing.T) {
			changed := append([]nativeWindowsExitStep(nil), steps...)
			for i := range changed {
				// Reproduce the parent's unchecked command at each boundary.
				changed[i].Run = strings.Replace(changed[i].Run, guard, "", 1)
			}
			if _, err := nativeWindowsExitGuards(changed); err == nil {
				t.Fatal("accepted a native failure that a later successful command could hide")
			}
		})
	}
	for _, stepName := range []string{"Build and execute native Windows agent", "Exercise native Windows service lifecycle"} {
		t.Run(stepName+" cannot tolerate failure", func(t *testing.T) {
			changed := append([]nativeWindowsExitStep(nil), steps...)
			for i := range changed {
				if changed[i].Name == stepName {
					changed[i].ContinueOnError = true
				}
			}
			if _, err := nativeWindowsExitGuards(changed); err == nil {
				t.Fatal("accepted a continue-on-error native Windows proof")
			}
		})
	}
}

func TestWindowsAgentLifecycleHarnessChecksEveryNativeExit(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "installtests", "windows_agent_lifecycle.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	guards, err := nativeWindowsHarnessExitGuards(string(content))
	if err != nil {
		t.Fatal(err)
	}
	for _, guard := range guards {
		t.Run(guard, func(t *testing.T) {
			changed := strings.Replace(string(content), guard, "", 1)
			if _, err := nativeWindowsHarnessExitGuards(changed); err == nil {
				t.Fatal("accepted failed version/installer/service query evidence")
			}
		})
	}
}

// Native PowerShell controls execute the production functions, not a separate
// model. The Linux proof's missing runtime remains an explicit skip; the
// existing Windows native workflow selects these tests and uses PowerShell 5.1.
func TestInstallPS1ServiceRemovalRuntime(t *testing.T) {
	powerShell := nativeInstallerPowerShell(t)
	cmd := exec.Command(powerShell, "-NoLogo", "-NoProfile", "-NonInteractive", "-File",
		repoFile("scripts", "installtests", "windows_service_removal_controls.ps1"),
		"-InstallerPath", repoFile("scripts", "install.ps1"))
	output, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(output), "PASS all 13 production service-removal controls") {
		t.Fatalf("production service-removal controls failed: %v\n%s", err, output)
	}
	t.Logf("%s", output)
}

func windowsRemovalFixtureContract(fixture string) error {
	load := strings.Index(fixture, "Add-Type -AssemblyName System.ServiceProcess")
	extract := strings.Index(fixture, "foreach ($name in @('Get-PulseService', 'Remove-PulseService'))")
	if load < 0 || extract <= load ||
		!strings.Contains(fixture, "$ExpectedStatus -is [System.ServiceProcess.ServiceControllerStatus]") {
		return fmt.Errorf("SCM double must load and check the real service-status enum before production controls")
	}
	return nil
}

func TestInstallPS1ServiceRemovalFixtureLoadsRealControllerEnum(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "installtests", "windows_service_removal_controls.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	fixture := string(content)
	if err := windowsRemovalFixtureContract(fixture); err != nil {
		t.Fatal(err)
	}
	for _, boundary := range []string{"Add-Type -AssemblyName System.ServiceProcess", "$ExpectedStatus -is [System.ServiceProcess.ServiceControllerStatus]"} {
		t.Run(boundary, func(t *testing.T) {
			if err := windowsRemovalFixtureContract(strings.Replace(fixture, boundary, "", 1)); err == nil {
				t.Fatal("accepted an unloaded or simulated enum")
			}
		})
	}
	if parentPath := os.Getenv("PULSE_WINDOWS_REMOVAL_FIXTURE_PARENT"); parentPath != "" {
		parent, err := os.ReadFile(parentPath)
		if err != nil {
			t.Fatal(err)
		}
		if err := windowsRemovalFixtureContract(string(parent)); err == nil {
			t.Fatal("accepted the exact parent's missing assembly precondition")
		}
		t.Log("rejected supplied parent fixture without the real enum precondition")
	}
}

func windowsRemovalContract(script string) error {
	for _, needle := range []string{
		`return Get-Service -Name $AgentName -ErrorAction Stop`,
		`$_.FullyQualifiedErrorId.Split(',')[0] -eq 'NoServiceFoundForGivenName'`,
		`$_.CategoryInfo.Category -eq [System.Management.Automation.ErrorCategory]::ObjectNotFound`,
		`param([int]$TimeoutSeconds = 30)`,
		`$service.Stop()`,
		`$service.WaitForStatus([System.ServiceProcess.ServiceControllerStatus]::Stopped,`,
		`[TimeSpan]::FromSeconds($TimeoutSeconds))`,
		`$service.Refresh()`,
		`throw "Service '$AgentName' is not confirmed stopped."`,
		`} finally {`,
		`$service.Dispose()`,
		`throw "Failed to delete service '$AgentName': $scOutput"`,
		`$remaining = Get-PulseService`,
		`$remaining.Dispose()`,
		`$elapsed.Elapsed.TotalSeconds -ge $TimeoutSeconds`,
		`throw "Service '$AgentName' is still present after deletion.`,
	} {
		if !strings.Contains(script, needle) {
			return fmt.Errorf("missing removal boundary %q", needle)
		}
	}
	if strings.Contains(script, "Stop-Service") && strings.Contains(script, "Stop-Service $AgentName") {
		return fmt.Errorf("unbounded or suppressed stop bypasses the shared boundary")
	}
	// Pin each real caller's refusal before any remote deregistration, binary
	// mutation or state mutation. A helper test alone cannot prove its wiring.
	for _, boundary := range []struct{ start, end, refusal string }{
		{"# --- Uninstall Logic ---", "# Try to notify the Pulse server", "Cannot uninstall"},
		{"# Confirm existing service stopped and absent", "# Move temp file to final location", "Cannot replace"},
	} {
		start := strings.Index(script, boundary.start)
		end := strings.Index(script, boundary.end)
		if start < 0 || end <= start {
			return fmt.Errorf("missing caller mutation boundary %s", boundary.start)
		}
		gate := script[start:end]
		call := strings.Index(gate, "Remove-PulseService")
		refusal := strings.Index(gate, boundary.refusal)
		if call < 0 || refusal <= call || !strings.Contains(gate[call:refusal], "} catch {") ||
			!strings.Contains(gate[refusal:], "Exit 1") {
			return fmt.Errorf("caller can continue after removal failure: %s", boundary.start)
		}
	}
	return nil
}

func TestInstallPS1ServiceRemovalRefusesUnknownRuntimeBeforeMutation(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "install.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	script := string(content)
	if err := windowsRemovalContract(script); err != nil {
		t.Fatal(err)
	}
	for name, needle := range map[string]string{
		"unknown is not absent":     `return Get-Service -Name $AgentName -ErrorAction Stop`,
		"stop is observed":          `$service.WaitForStatus([System.ServiceProcess.ServiceControllerStatus]::Stopped,`,
		"stopped is rechecked":      `throw "Service '$AgentName' is not confirmed stopped."`,
		"delete exit is retained":   `throw "Failed to delete service '$AgentName': $scOutput"`,
		"delete is not absence":     `$remaining = Get-PulseService`,
		"pending delete is bounded": `$elapsed.Elapsed.TotalSeconds -ge $TimeoutSeconds`,
		"uninstall refuses":         `Cannot uninstall`,
		"replacement refuses":       `Cannot replace`,
	} {
		t.Run(name, func(t *testing.T) {
			if err := windowsRemovalContract(strings.Replace(script, needle, "", 1)); err == nil {
				t.Fatal("accepted a removed safety boundary")
			}
		})
	}
	// An exact-source proof can supply the complete actual parent as an
	// independently hash-bound input. Do not make future CI depend on HEAD^.
	if parentPath := os.Getenv("PULSE_WINDOWS_REMOVAL_PARENT"); parentPath != "" {
		parent, err := os.ReadFile(parentPath)
		if err != nil {
			t.Fatal(err)
		}
		if err := windowsRemovalContract(string(parent)); err == nil {
			t.Fatal("accepted the parent's suppressed stop/delete failures")
		}
		t.Log("rejected the complete supplied parent installer")
	}
}
