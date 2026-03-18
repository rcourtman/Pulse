package installtests

import (
	"os"
	"strings"
	"testing"
)

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
		`if (-not [string]::IsNullOrWhiteSpace($Token)) { $ServiceArgs += @("--token", "` + "`" + `"$Token` + "`" + `"") }`,
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
		`Set-Content -Path $ConnectionStatePath -Value ($lines -join "` + "`n" + `") -Encoding UTF8`,
		`$Url = Get-ConnectionStateValue "PULSE_URL"`,
		`$Token = Get-ConnectionStateValue "PULSE_TOKEN"`,
		`$AgentId = Get-ConnectionStateValue "PULSE_AGENT_ID"`,
		`$Hostname = Get-ConnectionStateValue "PULSE_HOSTNAME"`,
		`$Insecure = Parse-Bool (Get-ConnectionStateValue "PULSE_INSECURE_SKIP_VERIFY") $Insecure`,
		`$CACertPath = Get-ConnectionStateValue "PULSE_CACERT"`,
		`$lines += "PULSE_INSECURE_SKIP_VERIFY='true'"`,
		`$lines += "PULSE_CACERT='$CACertPath'"`,
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
