package installtests

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	host "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	packet "github.com/rcourtman/pulse-go-rewrite/scripts/installtests/windowsupdatepacket"
)

// The fixtures below are public synthetic identities, never project credentials.
const signedWindowsID = "windows-signed-update-fixture"
const signedWindowsToken = "windows-signed-update-fixture-not-a-secret"

func signedWindowsPS(t *testing.T, script string) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-Command",
		"$ErrorActionPreference = 'Stop'; "+script)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("owned Windows fixture operation failed: %v\n%s", err, output)
	}
	return output
}

func signedWindowsQuote(value string) string { return "'" + strings.ReplaceAll(value, "'", "''") + "'" }

type signedWindowsService struct {
	PID       uint32 `json:"ProcessId"`
	Path      string `json:"PathName"`
	State     string `json:"State"`
	StartMode string `json:"StartMode"`
}

func signedWindowsServiceState(t *testing.T) signedWindowsService {
	t.Helper()
	output := signedWindowsPS(t, `$s = Get-CimInstance Win32_Service -Filter "Name='PulseAgent'" -ErrorAction Stop;
if ($null -eq $s) { throw 'Owned service is missing.' };
$s | Select-Object ProcessId,PathName,State,StartMode | ConvertTo-Json -Compress`)
	var service signedWindowsService
	if err := json.Unmarshal(output, &service); err != nil {
		t.Fatalf("decode independent SCM observation: %v", err)
	}
	return service
}

type signedWindowsCase struct {
	Name          string `json:"name"`
	Downloads     int    `json:"canonical_downloads"`
	BeforePID     uint32 `json:"before_pid"`
	AfterPID      uint32 `json:"after_pid"`
	Executable    string `json:"executable_sha256"`
	SavedRetained bool   `json:"saved_state_retained"`
	Connected     bool   `json:"authenticated_exact_identity_report"`
	UpdatedFrom   string `json:"updated_from,omitempty"`
}

func signedWindowsResult(refusal string, report host.AgentInfo) bool {
	if refusal != "" {
		return report.Version == packet.Before && report.Update != nil && report.Update.State == "error" && strings.Contains(report.Update.LastError, refusal)
	}
	return report.Version == packet.After && report.UpdatedFrom == packet.Before
}

// This is NOT a published installer/release qualification or an original-client
// replay. A new, otherwise empty hosted Windows VM registers one test-owned SCM
// service. Its actual auto-updater receives exact Linux production responses
// through a TLS-only transport shim, then SCM must run the replacement itself.
func TestWindowsSignedAutomaticUpdate(t *testing.T) {
	if os.Getenv("PULSE_WINDOWS_SIGNED_UPDATE") != "1" {
		t.Skip("dedicated native Windows hosted fixture required")
	}
	if runtime.GOOS != "windows" || os.Getenv("GITHUB_ACTIONS") != "true" || os.Getenv("RUNNER_ENVIRONMENT") != "github-hosted" {
		t.Fatal("signed Windows fixture requires its new GitHub-hosted Windows VM")
	}
	source := os.Getenv("EXPECTED_SOURCE_SHA")
	input, bodies, err := packet.Load(os.Getenv("PULSE_WINDOWS_UPDATE_PACKET"), source)
	if err != nil {
		t.Fatal(err)
	}
	// Refuse a failed query or pre-existing service BEFORE creating state.
	signedWindowsPS(t, `if ($PSVersionTable.PSEdition -ne 'Desktop' -or $PSVersionTable.PSVersion.ToString() -notlike '5.1.*') { throw 'Windows PowerShell 5.1 required.' };
$identity = [Security.Principal.WindowsIdentity]::GetCurrent();
if (-not ([Security.Principal.WindowsPrincipal]$identity).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) { throw 'Elevated hosted fixture required.' };
try { $s = Get-Service -Name PulseAgent -ErrorAction Stop } catch {
 if ($_.FullyQualifiedErrorId.Split(',')[0] -ne 'NoServiceFoundForGivenName' -or $_.CategoryInfo.Category -ne [System.Management.Automation.ErrorCategory]::ObjectNotFound) { throw }; $s = $null
};
if ($null -ne $s) { throw 'Pre-existing service: no mutation permitted.' };
if (-not [string]::IsNullOrWhiteSpace([Environment]::GetEnvironmentVariable('PULSE_DISABLE_AUTO_UPDATE','Machine'))) { throw 'Pre-existing machine auto-update override.' }`)
	root, err := os.MkdirTemp("", "pulse-signed-windows-update-")
	if err != nil {
		t.Fatal(err)
	}
	// Do not use TempDir's unconditional erasure if owned SCM cleanup fails.
	owned := false
	var peer *httptest.Server
	serviceCommand := ""
	receipt := struct {
		Source  string              `json:"source"`
		Scope   string              `json:"scope"`
		Cases   []signedWindowsCase `json:"cases"`
		Cleanup bool                `json:"owned_scm_cleanup"`
		Passed  bool                `json:"passed"`
	}{Source: source, Scope: "current-source-production-response-verified-TLS-native-Windows-SCM-not-release-or-customer-recovery"}
	// Retain the final result after owned service/directory cleanup.
	t.Cleanup(func() {
		receipt.Passed = !t.Failed() && receipt.Cleanup && len(receipt.Cases) == 3
		encoded, err := json.MarshalIndent(receipt, "", "  ")
		if err != nil {
			t.Error(err)
			return
		}
		path := os.Getenv("PULSE_WINDOWS_UPDATE_RECEIPT")
		if path == "" {
			t.Error("native receipt destination is required")
			return
		}
		file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			t.Error(err)
			return
		}
		_, writeErr := file.Write(encoded)
		closeErr := file.Close()
		if writeErr != nil || closeErr != nil {
			t.Error("native receipt write failed")
		}
	})
	t.Cleanup(func() {
		if owned {
			// An unknown/changed service is never stopped or deleted. Retain its
			// fixture bytes on failure rather than converting uncertainty to cleanup.
			signedWindowsPS(t, `$s = Get-CimInstance Win32_Service -Filter "Name='PulseAgent'" -ErrorAction Stop;
if ($null -eq $s) { return };
if ($s.PathName -cne `+signedWindowsQuote(serviceCommand)+`) { throw 'Owned service identity changed; cleanup stopped.' };
$process = $null;
if ($s.ProcessId -ne 0) {
 $process = Get-Process -Id $s.ProcessId -ErrorAction Stop;
 if ($process.Path -ine `+signedWindowsQuote(filepath.Join(root, "pulse-agent.exe"))+`) { throw 'Owned cleanup process identity differs.' }
};
if ($s.State -ne 'Stopped') { Stop-Service PulseAgent -ErrorAction Stop; (Get-Service PulseAgent -ErrorAction Stop).WaitForStatus([System.ServiceProcess.ServiceControllerStatus]::Stopped,[TimeSpan]::FromSeconds(30)) };
if ($null -ne $process) { if (-not $process.WaitForExit(10000)) { throw 'Owned cleanup process remains.' }; $process.Dispose() };
& sc.exe delete PulseAgent | Out-Null; if ($LASTEXITCODE -ne 0) { throw 'Owned SCM deletion failed.' };
for ($i=0; $i -lt 30; $i++) {
 try { $remaining = Get-Service PulseAgent -ErrorAction Stop; $remaining.Dispose() } catch {
  if ($_.FullyQualifiedErrorId.Split(',')[0] -eq 'NoServiceFoundForGivenName' -and $_.CategoryInfo.Category -eq [System.Management.Automation.ErrorCategory]::ObjectNotFound) { return }; throw
 }; Start-Sleep -Milliseconds 200
}; throw 'Owned SCM service remains after cleanup.'`)
		}
		if peer != nil {
			peer.Close()
		}
		if err := os.RemoveAll(root); err != nil {
			t.Error("owned fixture directory cleanup failed")
		} else {
			receipt.Cleanup = true
		}
	})
	var mu sync.Mutex
	mode := ""
	var versionReady chan struct{}
	downloads, unexpected := 0, 0
	var reports []host.AgentInfo
	peer = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Hold only the first version response until the observer has the
		// original SCM PID. A slow PowerShell query cannot miss the replacement.
		if r.URL.Path == "/api/version" && r.Header.Get("X-API-Token") == signedWindowsToken {
			mu.Lock()
			ready := versionReady
			mu.Unlock()
			select {
			case <-ready:
			case <-r.Context().Done():
				return
			case <-time.After(30 * time.Second):
				http.Error(w, "baseline observation missing", http.StatusServiceUnavailable)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"version": packet.After})
			return
		}
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("X-API-Token") != signedWindowsToken {
			unexpected++
			http.Error(w, "fixture requires the saved token", http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/download/pulse-agent":
			if r.Method != http.MethodGet || r.URL.Query().Get("arch") != "windows-amd64" || r.URL.Query().Get("serverVersion") != packet.After {
				unexpected++
				http.NotFound(w, r)
				return
			}
			responseName := mode
			if mode == "wrong-identity" {
				responseName = "initial"
			}
			response := input.Responses[responseName]
			downloads++
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("X-Checksum-Sha256", response.Checksum)
			w.Header().Set("X-Signature-Ed25519", response.Signature)
			w.Header().Set("X-Signature-SSHSIG", response.SSHSIG)
			_, _ = w.Write(bodies[responseName])
		case "/api/agents/agent/report":
			// Exercise real collector reports (which are gzip-compressed), not
			// a fixture's hand-written uncompressed acknowledgement witness.
			reader, err := gzip.NewReader(http.MaxBytesReader(w, r.Body, 1024*1024))
			if err != nil {
				unexpected++
				http.Error(w, "invalid fixture report compression", http.StatusBadRequest)
				return
			}
			defer reader.Close()
			var report host.Report
			if r.Method != http.MethodPost || r.Header.Get("Content-Encoding") != "gzip" || json.NewDecoder(io.LimitReader(reader, 4*1024*1024)).Decode(&report) != nil || report.Agent.ID != signedWindowsID || report.Host.Hostname != signedWindowsID {
				unexpected++
				http.Error(w, "fixture identity differs", http.StatusBadRequest)
				return
			}
			reports = append(reports, report.Agent)
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "agentId": signedWindowsID})
		default:
			// Remote-config polling is not part of this updater fixture.
			http.NotFound(w, r)
		}
	}))
	ca := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: peer.Certificate().Raw})
	binary := filepath.Join(root, "pulse-agent.exe")
	state := filepath.Join(root, "state")
	if err := os.Mkdir(state, 0700); err != nil {
		t.Fatal(err)
	}
	saved := map[string][]byte{
		filepath.Join(root, "fixture-ca.pem"):  ca,
		filepath.Join(state, "token"):          []byte(signedWindowsToken),
		filepath.Join(state, "agent-id"):       []byte(signedWindowsID),
		filepath.Join(state, "retained-state"): []byte("saved Windows state must survive refusal and update"),
	}
	for path, body := range saved {
		if err := os.WriteFile(path, body, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(binary, bodies["initial"], 0700); err != nil {
		t.Fatal(err)
	}
	serviceCommand = fmt.Sprintf(`"%s" --url "%s" --token-file "%s" --cacert "%s" --insecure=false --state-dir "%s" --agent-id %s --hostname %s --interval 1s --health-addr 127.0.0.1:0 --enable-host --enable-docker=false --enable-kubernetes=false --enable-proxmox=false --enable-commands=false --command-authority monitoring-only --log-file "%s"`,
		binary, peer.URL, filepath.Join(state, "token"), filepath.Join(root, "fixture-ca.pem"), state, signedWindowsID, signedWindowsID, filepath.Join(state, "agent.log"))
	owned = true // intent: cleanup may touch only an absent or exact-command service.
	signedWindowsPS(t, `New-Service -Name PulseAgent -BinaryPathName `+signedWindowsQuote(serviceCommand)+` -StartupType Automatic | Out-Null;
& sc.exe failure PulseAgent reset= 86400 actions= restart/1000/restart/1000/restart/1000 | Out-Null; if ($LASTEXITCODE -ne 0) { throw 'SCM recovery setup failed.' };
& sc.exe failureflag PulseAgent 1 | Out-Null; if ($LASTEXITCODE -ne 0) { throw 'SCM non-crash recovery setup failed.' }`)
	for _, scenario := range []struct{ name, refusal string }{
		{"bad-signature", "signature verification failed"}, {"wrong-identity", "downloaded executable identity:"}, {"valid", ""},
	} {
		mu.Lock()
		mode, downloads, unexpected, reports = scenario.name, 0, 0, nil
		versionReady = make(chan struct{})
		mu.Unlock()
		signedWindowsPS(t, `Start-Service PulseAgent -ErrorAction Stop`)
		before := signedWindowsServiceState(t)
		if before.PID == 0 || before.Path != serviceCommand || before.State != "Running" || before.StartMode != "Auto" {
			t.Fatal("SCM did not start the exact saved connection")
		}
		close(versionReady)
		deadline := time.Now().Add(100 * time.Second)
		var observed host.AgentInfo
		complete := false
		for time.Now().Before(deadline) {
			mu.Lock()
			for _, report := range reports {
				if signedWindowsResult(scenario.refusal, report) {
					observed, complete = report, true
				}
			}
			mu.Unlock()
			if complete {
				break
			}
			time.Sleep(250 * time.Millisecond)
		}
		if !complete {
			t.Fatalf("%s: no exact connected updater result before deadline", scenario.name)
		}
		after := signedWindowsServiceState(t)
		if after.Path != serviceCommand || after.State != "Running" || after.StartMode != "Auto" || after.PID == 0 {
			t.Fatal("running SCM identity or saved connection differs")
		}
		if (after.PID == before.PID) != (scenario.refusal != "") {
			t.Fatal("SCM process replacement/refusal result differs")
		}
		body, err := os.ReadFile(binary)
		if err != nil {
			t.Fatal(err)
		}
		expected := input.Responses["initial"].Checksum
		if scenario.refusal == "" {
			expected = input.Responses["valid"].Checksum
		}
		if packet.Digest(body) != expected {
			t.Fatal("installed executable bytes differ")
		}
		if scenario.refusal != "" {
			for _, path := range []string{binary + ".backup", filepath.Join(root, ".pulse-update-info")} {
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatal("refused update left replacement/provenance state")
				}
			}
			staging, err := filepath.Glob(filepath.Join(root, "pulse-agent-*.tmp"))
			if err != nil || len(staging) != 0 {
				t.Fatal("refused update did not clean its staged download")
			}
		}
		for path, original := range saved {
			body, err := os.ReadFile(path)
			if err != nil || string(body) != string(original) {
				t.Fatal("saved identity, token, CA or seeded state changed")
			}
		}
		mu.Lock()
		count, wrong := downloads, unexpected
		mu.Unlock()
		if count != 1 || wrong != 0 {
			t.Fatal("canonical request/authentication count differs; fallback is not acceptance")
		}
		receipt.Cases = append(receipt.Cases, signedWindowsCase{Name: scenario.name, Downloads: count, BeforePID: before.PID, AfterPID: after.PID, Executable: expected, SavedRetained: true, Connected: true, UpdatedFrom: observed.UpdatedFrom})
		t.Logf("%s: one authenticated canonical download, exact SCM executable and connected identity; saved token/CA/state retained", scenario.name)
		signedWindowsPS(t, `$s = Get-CimInstance Win32_Service -Filter "Name='PulseAgent'" -ErrorAction Stop;
if ($null -eq $s -or $s.PathName -cne `+signedWindowsQuote(serviceCommand)+` -or $s.ProcessId -eq 0) { throw 'Owned process is unknown.' };
$process = Get-Process -Id $s.ProcessId -ErrorAction Stop;
if ($process.Path -ine `+signedWindowsQuote(binary)+`) { throw 'Owned process executable differs.' };
Stop-Service PulseAgent -ErrorAction Stop; (Get-Service PulseAgent -ErrorAction Stop).WaitForStatus([System.ServiceProcess.ServiceControllerStatus]::Stopped,[TimeSpan]::FromSeconds(30));
if (-not $process.WaitForExit(10000)) { throw 'Owned process did not quiesce.' }; $process.Dispose()`)
	}
}
