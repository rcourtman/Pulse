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

func TestInstallPS1AllowsMissingTokenForOptionalAuth(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "install.ps1"))
	if err != nil {
		t.Fatalf("read install.ps1: %v", err)
	}

	script := string(content)
	required := []string{
		`if (-not [string]::IsNullOrWhiteSpace($Token) -and -not (Test-ValidToken $Token)) {`,
		`function Write-RuntimeTokenFile {`,
		`if (-not [string]::IsNullOrWhiteSpace($Token)) { $ServiceArgs += @("--token-file", "` + "`" + `"$TokenFilePath` + "`" + `"") }`,
	}
	for _, needle := range required {
		if !strings.Contains(script, needle) {
			t.Fatalf("install.ps1 missing optional-token handling: %s", needle)
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
		`if ($AllowInsecure -or $null -ne $CustomCaCertificate) {`,
		`if ($AllowInsecure) {`,
		`return Test-CertificateTrustedByCustomCa -Certificate $certificate -CustomCaCertificate $CustomCaCertificate`,
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
		`$lookupResult = Invoke-RestMethod @lookupArgs`,
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
		`$serverSshSignature = $webClient.ResponseHeaders[$InstallerSignatureHeaderName]`,
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
