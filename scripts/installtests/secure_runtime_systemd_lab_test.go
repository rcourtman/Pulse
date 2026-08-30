//go:build !windows

package installtests

// TestSecureRuntimeSystemdLab is intentionally excluded from ordinary test
// runs. It installs services and users into a disposable Linux systemd host.
// Build the release-shaped inputs on the host, then copy or mount them into a
// dedicated Colima profile before opting in:
//
//   mkdir -p .lab-artifacts
//   v1_ldflags="$(./scripts/release_ldflags.sh agent --version 6.2.0-lab.1)"
//   v2_ldflags="$(./scripts/release_ldflags.sh agent --version 6.2.0-lab.2)"
//   CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags "$v1_ldflags" -o .lab-artifacts/pulse-agent-v1 ./cmd/pulse-agent
//   CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags "$v2_ldflags" -o .lab-artifacts/pulse-agent-v2 ./cmd/pulse-agent
//   CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o .lab-artifacts/pulse-agent-helper ./cmd/pulse-agent-helper
//   CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o .lab-artifacts/pulse-agent-runner ./cmd/pulse-agent-runner
//   CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go test -c -o .lab-artifacts/installtests-linux-arm64.test ./scripts/installtests
//   colima start pulse-agent-qual --activate=false --mount "$PWD:w"
//   colima ssh -p pulse-agent-qual -- sudo sh -c \
//     'printf "%s\n" "PULSE_SECURE_RUNTIME_SYSTEMD_LAB=disposable-v1" > /etc/pulse-secure-runtime-lab'
//   repo="$PWD"
//   colima ssh -p pulse-agent-qual -- sudo env \
//     PULSE_SECURE_RUNTIME_SYSTEMD_LAB=1 \
//     PULSE_SECURE_RUNTIME_COLLECTOR_V1="$repo/.lab-artifacts/pulse-agent-v1" \
//     PULSE_SECURE_RUNTIME_COLLECTOR_V2="$repo/.lab-artifacts/pulse-agent-v2" \
//     PULSE_SECURE_RUNTIME_HELPER="$repo/.lab-artifacts/pulse-agent-helper" \
//     PULSE_SECURE_RUNTIME_RUNNER="$repo/.lab-artifacts/pulse-agent-runner" \
//     PULSE_SECURE_RUNTIME_RECEIPT=/tmp/secure-runtime-receipt.json \
//     sh -c 'cd "$1/scripts/installtests" && exec "$1/.lab-artifacts/installtests-linux-arm64.test" -test.run "^TestSecureRuntimeSystemdLab$" -test.count=1 -test.v' sh "$repo"
//
// Use the VM's GOARCH in place of arm64 when qualifying another architecture.
// The test never creates or deletes a VM; lifecycle remains a host-side choice.

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationreceipt"
)

const (
	secureRuntimeLabOptIn        = "PULSE_SECURE_RUNTIME_SYSTEMD_LAB"
	secureRuntimeLabMarkerPath   = "/etc/pulse-secure-runtime-lab"
	secureRuntimeLabMarkerValue  = "PULSE_SECURE_RUNTIME_SYSTEMD_LAB=disposable-v1"
	secureRuntimeLabToken        = "8fa728ed4cbfa7466947e747627b51e2464ed3f69966ed6394e36a82d85d7d31"
	secureRuntimeLabAgentID      = "secure-runtime-systemd-lab"
	secureRuntimeLabHostname     = "pulse-secure-runtime-lab"
	secureRuntimeLabOrgID        = "secure-runtime-lab-org"
	secureRuntimeRunnerSecretV1  = "3c896fe99e29384a293def92f23c1709fbf429773e0a20ac5d930f2aab62f839"
	secureRuntimeRunnerSecretV2  = "d7f6f2550788213e0595f276b62b1df290f7e018ba74c03068c4afbe6efd7601"
	secureRuntimeRunnerBindingV1 = "secure-runtime-runner-binding-v1"
	secureRuntimeRunnerBindingV2 = "secure-runtime-runner-binding-v2"
)

var secureRuntimeInstalledPaths = []string{
	"/usr/local/bin/pulse-agent",
	"/usr/local/lib/pulse-agent/pulse-agent-helper",
	"/usr/local/lib/pulse-agent/pulse-agent-runner",
	"/etc/systemd/system/pulse-agent.service",
	"/etc/systemd/system/pulse-agent.service.d",
	"/etc/systemd/system/pulse-agent-helper.service",
	"/etc/systemd/system/pulse-agent-helper.socket",
	"/etc/systemd/system/pulse-agent-runner.service",
	"/etc/pulse-agent-runner",
	"/var/lib/pulse-agent",
	"/var/lib/pulse-agent-helper",
	"/var/lib/pulse-agent-runner",
	"/var/lib/pulse-agent-profile",
}

type secureRuntimeLabReport struct {
	ReceivedAt      time.Time
	AgentID         string
	AgentVersion    string
	Hostname        string
	RunningAsRoot   bool
	ServiceUser     string
	Authority       string
	TypedHelper     bool
	CommandsEnabled bool
}

type secureRuntimeLabFixture struct {
	mu                  sync.Mutex
	collector           []byte
	helper              []byte
	runner              []byte
	serverVersion       string
	reports             []secureRuntimeLabReport
	lastSeen            time.Time
	freezeLastSeen      bool
	authFailures        int
	requestFailures     []string
	actionServer        *agentexec.Server
	actionSecret        string
	actionBindingID     string
	actionRevoked       bool
	actionRevokes       int
	authorityReductions int
}

func newSecureRuntimeLabFixture(collector, helper, runner []byte, version string) *secureRuntimeLabFixture {
	fixture := &secureRuntimeLabFixture{
		collector: collector, helper: helper, runner: runner, serverVersion: version,
		actionSecret: secureRuntimeRunnerSecretV1, actionBindingID: secureRuntimeRunnerBindingV1,
	}
	fixture.actionServer = agentexec.NewServerWithAdmissionValidator(fixture.admitActionRunner, fixture.validateActionRunnerSession)
	return fixture
}

func (f *secureRuntimeLabFixture) admitActionRunner(secret, agentID, hostname string) (agentexec.AgentAdmission, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.actionRevoked || secret != f.actionSecret || strings.TrimSpace(agentID) != secureRuntimeLabAgentID || !strings.EqualFold(strings.TrimSpace(hostname), secureRuntimeLabHostname) {
		return agentexec.AgentAdmission{}, false
	}
	return f.actionAdmissionLocked(), true
}

func (f *secureRuntimeLabFixture) validateActionRunnerSession(admission agentexec.AgentAdmission) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return !f.actionRevoked && admission == f.actionAdmissionLocked()
}

func (f *secureRuntimeLabFixture) actionAdmissionLocked() agentexec.AgentAdmission {
	return agentexec.AgentAdmission{
		OrganizationID: secureRuntimeLabOrgID, TokenID: f.actionBindingID,
		AgentID: secureRuntimeLabAgentID, Hostname: secureRuntimeLabHostname,
		RuntimeRole: agentexec.RuntimeRoleActionRunner, ActionCapability: agentexec.ActionCapabilityTypedV1,
	}
}

func (f *secureRuntimeLabFixture) replaceActionCredential(secret, bindingID string) agentexec.AgentAdmission {
	f.mu.Lock()
	defer f.mu.Unlock()
	previous := f.actionAdmissionLocked()
	f.actionSecret = secret
	f.actionBindingID = bindingID
	f.actionRevoked = false
	return previous
}

func (f *secureRuntimeLabFixture) actionSnapshot() (agentexec.AgentAdmission, bool, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.actionAdmissionLocked(), f.actionRevoked, f.actionRevokes
}

func (f *secureRuntimeLabFixture) authorityReductionCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.authorityReductions
}

func (f *secureRuntimeLabFixture) setCollector(artifact []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.collector = append([]byte(nil), artifact...)
}

func (f *secureRuntimeLabFixture) setServerVersion(version string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.serverVersion = version
}

func (f *secureRuntimeLabFixture) setFrozen(frozen bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.freezeLastSeen = frozen
}

func (f *secureRuntimeLabFixture) snapshot() ([]secureRuntimeLabReport, time.Time, int, []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]secureRuntimeLabReport(nil), f.reports...), f.lastSeen, f.authFailures, append([]string(nil), f.requestFailures...)
}

func (f *secureRuntimeLabFixture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == "/api/version":
		f.mu.Lock()
		version := f.serverVersion
		f.mu.Unlock()
		writeSecureRuntimeJSON(w, http.StatusOK, map[string]any{"version": version})
	case r.URL.Path == "/download/pulse-agent":
		f.serveArtifact(w, r, "collector")
	case r.URL.Path == "/download/pulse-agent-helper":
		f.serveArtifact(w, r, "helper")
	case r.URL.Path == "/download/pulse-agent-runner":
		f.serveArtifact(w, r, "runner")
	case r.URL.Path == "/api/agent/ws":
		f.actionServer.HandleWebSocket(w, r)
	case r.URL.Path == "/api/agents/action-runner/credential":
		f.handleActionRunnerSelfRevoke(w, r)
	case r.URL.Path == "/api/agents/collector/reduce-authority":
		f.handleCollectorAuthorityReduction(w, r)
	case r.URL.Path == "/api/health":
		writeSecureRuntimeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	case r.URL.Path == "/api/agents/agent/report":
		f.handleReport(w, r)
	case r.URL.Path == "/api/agents/docker/report":
		if !f.authorized(r) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		writeSecureRuntimeJSON(w, http.StatusOK, map[string]any{"success": true})
	case r.URL.Path == "/api/agents/agent/lookup":
		f.handleLookup(w, r)
	case strings.HasPrefix(r.URL.Path, "/api/agents/agent/") && strings.HasSuffix(r.URL.Path, "/config"):
		if !f.authorized(r) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		writeSecureRuntimeJSON(w, http.StatusOK, map[string]any{
			"success": true,
			"agentId": secureRuntimeLabAgentID,
			"config":  map[string]any{"settings": map[string]any{}},
		})
	default:
		http.NotFound(w, r)
	}
}

func (f *secureRuntimeLabFixture) handleCollectorAuthorityReduction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var request struct {
		AgentID  string `json:"agentId"`
		Hostname string `json:"hostname"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&request); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	valid := strings.TrimSpace(r.Header.Get("Authorization")) == "Bearer "+secureRuntimeLabToken &&
		strings.TrimSpace(request.AgentID) == secureRuntimeLabAgentID &&
		strings.EqualFold(strings.TrimSpace(request.Hostname), secureRuntimeLabHostname)
	if !valid {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	f.mu.Lock()
	f.authorityReductions++
	f.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

func (f *secureRuntimeLabFixture) authorized(r *http.Request) bool {
	bearer := r.Header.Get("Authorization")
	valid := r.Header.Get("X-API-Token") == secureRuntimeLabToken &&
		(bearer == "" || bearer == "Bearer "+secureRuntimeLabToken) &&
		r.URL.Query().Get("token") == ""
	if !valid {
		f.mu.Lock()
		f.authFailures++
		f.requestFailures = append(f.requestFailures, r.Method+" "+r.URL.Path+": invalid credential transport")
		f.mu.Unlock()
	}
	return valid
}

func (f *secureRuntimeLabFixture) serveArtifact(w http.ResponseWriter, r *http.Request, artifactKind string) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	f.mu.Lock()
	artifact := append([]byte(nil), f.collector...)
	switch artifactKind {
	case "helper":
		artifact = append([]byte(nil), f.helper...)
	case "runner":
		artifact = append([]byte(nil), f.runner...)
	}
	f.mu.Unlock()
	sum := sha256.Sum256(artifact)
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("X-Checksum-Sha256", hex.EncodeToString(sum[:]))
	w.Header().Set("Content-Length", strconv.Itoa(len(artifact)))
	if r.Method == http.MethodGet {
		_, _ = w.Write(artifact)
	}
}

func (f *secureRuntimeLabFixture) handleActionRunnerSelfRevoke(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var request struct {
		AgentID  string `json:"agentId"`
		Hostname string `json:"hostname"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&request); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	bearer := strings.TrimPrefix(strings.TrimSpace(r.Header.Get("Authorization")), "Bearer ")
	f.mu.Lock()
	admission := f.actionAdmissionLocked()
	valid := !f.actionRevoked && bearer == f.actionSecret && strings.TrimSpace(request.AgentID) == admission.AgentID && strings.EqualFold(strings.TrimSpace(request.Hostname), admission.Hostname)
	if valid {
		f.actionRevoked = true
		f.actionRevokes++
	}
	f.mu.Unlock()
	if !valid {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	f.actionServer.InvalidateActionRunnerSession(admission)
	writeSecureRuntimeJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (f *secureRuntimeLabFixture) handleReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !f.authorized(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	body := io.Reader(http.MaxBytesReader(w, r.Body, 32<<20))
	if r.Header.Get("Content-Encoding") == "gzip" {
		compressed, err := gzip.NewReader(body)
		if err != nil {
			http.Error(w, "invalid compressed report", http.StatusBadRequest)
			return
		}
		defer compressed.Close()
		body = compressed
	}
	var payload struct {
		Agent struct {
			ID              string `json:"id"`
			Version         string `json:"version"`
			Hostname        string `json:"hostname"`
			CommandsEnabled bool   `json:"commandsEnabled"`
			Privilege       *struct {
				RunningAsRoot bool   `json:"runningAsRoot"`
				ServiceUser   string `json:"serviceUser"`
				Authority     string `json:"commandAuthority"`
				TypedHelper   bool   `json:"typedHelper"`
			} `json:"privilege"`
		} `json:"agent"`
		Host struct {
			Hostname string `json:"hostname"`
		} `json:"host"`
	}
	if err := json.NewDecoder(body).Decode(&payload); err != nil {
		http.Error(w, "invalid report", http.StatusBadRequest)
		return
	}
	report := secureRuntimeLabReport{
		ReceivedAt:      time.Now().UTC(),
		AgentID:         strings.TrimSpace(payload.Agent.ID),
		AgentVersion:    strings.TrimSpace(payload.Agent.Version),
		Hostname:        strings.TrimSpace(payload.Agent.Hostname),
		CommandsEnabled: payload.Agent.CommandsEnabled,
	}
	if report.Hostname == "" {
		report.Hostname = strings.TrimSpace(payload.Host.Hostname)
	}
	if payload.Agent.Privilege != nil {
		report.RunningAsRoot = payload.Agent.Privilege.RunningAsRoot
		report.ServiceUser = payload.Agent.Privilege.ServiceUser
		report.Authority = payload.Agent.Privilege.Authority
		report.TypedHelper = payload.Agent.Privilege.TypedHelper
	}
	f.mu.Lock()
	f.reports = append(f.reports, report)
	if !f.freezeLastSeen {
		f.lastSeen = report.ReceivedAt
	}
	serverVersion := f.serverVersion
	f.mu.Unlock()
	writeSecureRuntimeJSON(w, http.StatusOK, map[string]any{
		"success":       true,
		"agentId":       secureRuntimeLabAgentID,
		"serverVersion": serverVersion,
	})
}

func (f *secureRuntimeLabFixture) handleLookup(w http.ResponseWriter, r *http.Request) {
	if !f.authorized(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	f.mu.Lock()
	lastSeen := f.lastSeen
	f.mu.Unlock()
	if lastSeen.IsZero() {
		writeSecureRuntimeJSON(w, http.StatusNotFound, map[string]any{"success": false})
		return
	}
	writeSecureRuntimeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"agent": map[string]any{
			"id":       secureRuntimeLabAgentID,
			"hostname": secureRuntimeLabHostname,
			"lastSeen": lastSeen.Format(time.RFC3339Nano),
		},
	})
}

func writeSecureRuntimeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

type secureRuntimeFileIdentity struct {
	Present bool
	Mode    os.FileMode
	UID     uint32
	GID     uint32
	Hash    string
}

type secureRuntimeStableIdentity map[string]secureRuntimeFileIdentity

func secureRuntimeStableSnapshot(t *testing.T) secureRuntimeStableIdentity {
	t.Helper()
	paths := []string{
		"/usr/local/bin/pulse-agent",
		"/etc/systemd/system/pulse-agent.service",
		"/var/lib/pulse-agent/agent-id",
		"/var/lib/pulse-agent/connection.env",
		"/var/lib/pulse-agent/token",
		"/var/lib/pulse-agent/runtime.token",
	}
	result := make(secureRuntimeStableIdentity, len(paths))
	for _, path := range paths {
		info, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			result[path] = secureRuntimeFileIdentity{}
			continue
		}
		if err != nil {
			t.Fatalf("inspect stable identity %s: %v", path, err)
		}
		if !info.Mode().IsRegular() {
			t.Fatalf("stable identity path is not a regular file: %s (%s)", path, info.Mode())
		}
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read stable identity %s: %v", path, err)
		}
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok {
			t.Fatalf("read ownership for %s", path)
		}
		sum := sha256.Sum256(content)
		result[path] = secureRuntimeFileIdentity{
			Present: true,
			Mode:    info.Mode().Perm(),
			UID:     stat.Uid,
			GID:     stat.Gid,
			Hash:    hex.EncodeToString(sum[:]),
		}
	}
	return result
}

func secureRuntimeSeedAPTPackageCache(t *testing.T) (string, string) {
	t.Helper()
	const cacheDir = "/var/cache/apt/archives"
	path := filepath.Join(cacheDir, "pulse-secure-runtime-qualification.deb")
	content := bytes.Repeat([]byte("pulse-secure-runtime\n"), 4096)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("seed disposable apt package cache: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })
	hasher := sha256.New()
	err := filepath.WalkDir(cacheDir, func(entryPath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".deb") {
			return nil
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			return infoErr
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		relative, relativeErr := filepath.Rel(cacheDir, entryPath)
		if relativeErr != nil {
			return relativeErr
		}
		_, _ = fmt.Fprintf(hasher, "%s\x00%d\x00%d\n", filepath.ToSlash(relative), info.Size(), info.ModTime().UnixNano())
		return nil
	})
	if err != nil {
		t.Fatalf("fingerprint disposable apt package cache: %v", err)
	}
	return path, "sha256:" + hex.EncodeToString(hasher.Sum(nil))
}

type secureRuntimeScenarioResult struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Detail string `json:"detail,omitempty"`
}

type secureRuntimeLabReceipt struct {
	SchemaVersion                              int                           `json:"schema_version"`
	CompletedAt                                string                        `json:"completed_at"`
	SourceHashes                               map[string]string             `json:"source_hashes"`
	ArtifactHashes                             map[string]string             `json:"artifact_hashes"`
	ArtifactVersions                           map[string]string             `json:"artifact_versions"`
	DisposableVMGuardHash                      string                        `json:"disposable_vm_guard_sha256"`
	OSRelease                                  string                        `json:"os_release"`
	Kernel                                     string                        `json:"kernel"`
	SystemdVersion                             string                        `json:"systemd_version"`
	Architecture                               string                        `json:"architecture"`
	CollectorServiceUser                       string                        `json:"collector_service_user"`
	CollectorProcessUID                        int                           `json:"collector_process_uid"`
	CollectorAuthority                         string                        `json:"collector_authority"`
	AmbientCapabilitiesNone                    bool                          `json:"ambient_capabilities_none"`
	HelperProtocolHealthy                      bool                          `json:"helper_protocol_healthy"`
	StateIdentityPreserved                     bool                          `json:"state_identity_preserved"`
	DockerDegraded                             bool                          `json:"docker_degraded"`
	ActionRunnerQualified                      bool                          `json:"action_runner_qualified"`
	ActionMutationVerified                     bool                          `json:"action_mutation_verified"`
	CollectorAuthorityReductionRequestObserved bool                          `json:"collector_authority_reduction_request_observed"`
	ActionReceiptKind                          string                        `json:"action_receipt_kind,omitempty"`
	CredentialRotated                          bool                          `json:"credential_rotated"`
	SelfRevokeObserved                         bool                          `json:"self_revoke_observed"`
	CollectorContinuity                        bool                          `json:"collector_continuity"`
	ReportCount                                int                           `json:"report_count"`
	FirstReportAt                              string                        `json:"first_report_at"`
	LastReportAt                               string                        `json:"last_report_at"`
	Scenarios                                  []secureRuntimeScenarioResult `json:"scenarios"`
}

func TestSecureRuntimeSystemdLab(t *testing.T) {
	if os.Getenv(secureRuntimeLabOptIn) != "1" {
		t.Skip("set PULSE_SECURE_RUNTIME_SYSTEMD_LAB=1 only inside a disposable systemd VM")
	}
	secureRuntimeRequireDisposableHost(t)

	collectorV1 := secureRuntimeReadArtifact(t, "PULSE_SECURE_RUNTIME_COLLECTOR_V1")
	collectorV2 := secureRuntimeReadArtifact(t, "PULSE_SECURE_RUNTIME_COLLECTOR_V2")
	helper := secureRuntimeReadArtifact(t, "PULSE_SECURE_RUNTIME_HELPER")
	runner := secureRuntimeReadArtifact(t, "PULSE_SECURE_RUNTIME_RUNNER")
	collectorV1Version := secureRuntimeArtifactVersion(t, "PULSE_SECURE_RUNTIME_COLLECTOR_V1")
	collectorV2Version := secureRuntimeArtifactVersion(t, "PULSE_SECURE_RUNTIME_COLLECTOR_V2")
	if collectorV1Version == collectorV2Version {
		t.Fatalf("collector V1 and V2 must have distinct --version output, both reported %q", collectorV1Version)
	}
	installerPath, err := filepath.Abs(repoFile("scripts", "install.sh"))
	if err != nil {
		t.Fatalf("resolve installer path: %v", err)
	}
	installer := secureRuntimeReadFile(t, installerPath)

	fixture := newSecureRuntimeLabFixture(collectorV1, helper, runner, collectorV1Version)
	defer fixture.actionServer.Shutdown()
	server := httptest.NewServer(fixture)
	defer server.Close()

	receipt := secureRuntimeLabReceipt{
		SchemaVersion: 3,
		SourceHashes: map[string]string{
			"scripts/install.sh": secureRuntimeHash(installer),
			"scripts/installtests/secure_runtime_systemd_lab_test.go": secureRuntimeHash(secureRuntimeReadFile(t, repoFile("scripts", "installtests", "secure_runtime_systemd_lab_test.go"))),
			"scripts/release_control/secure_runtime_attestation.py":   secureRuntimeHash(secureRuntimeReadFile(t, repoFile("scripts", "release_control", "secure_runtime_attestation.py"))),
			"internal/agenthelper/update_activation.go":               secureRuntimeHash(secureRuntimeReadFile(t, repoFile("internal", "agenthelper", "update_activation.go"))),
			"internal/agenthelper/server.go":                          secureRuntimeHash(secureRuntimeReadFile(t, repoFile("internal", "agenthelper", "server.go"))),
			"internal/agentexec/server.go":                            secureRuntimeHash(secureRuntimeReadFile(t, repoFile("internal", "agentexec", "server.go"))),
			"internal/agentexec/types.go":                             secureRuntimeHash(secureRuntimeReadFile(t, repoFile("internal", "agentexec", "types.go"))),
			"internal/agentexec/verifier_postconditions.go":           secureRuntimeHash(secureRuntimeReadFile(t, repoFile("internal", "agentexec", "verifier_postconditions.go"))),
			"internal/api/collector_authority.go":                     secureRuntimeHash(secureRuntimeReadFile(t, repoFile("internal", "api", "collector_authority.go"))),
			"internal/api/action_runner_credentials.go":               secureRuntimeHash(secureRuntimeReadFile(t, repoFile("internal", "api", "action_runner_credentials.go"))),
			"internal/api/agenttokens/install.go":                     secureRuntimeHash(secureRuntimeReadFile(t, repoFile("internal", "api", "agenttokens", "install.go"))),
			"internal/api/agent_exec_token_binding.go":                secureRuntimeHash(secureRuntimeReadFile(t, repoFile("internal", "api", "agent_exec_token_binding.go"))),
			"internal/actionrunner/runner.go":                         secureRuntimeHash(secureRuntimeReadFile(t, repoFile("internal", "actionrunner", "runner.go"))),
			"internal/actionrunner/types.go":                          secureRuntimeHash(secureRuntimeReadFile(t, repoFile("internal", "actionrunner", "types.go"))),
			"internal/actionrunner/transport.go":                      secureRuntimeHash(secureRuntimeReadFile(t, repoFile("internal", "actionrunner", "transport.go"))),
			"internal/hostagent/storage_cleanup.go":                   secureRuntimeHash(secureRuntimeReadFile(t, repoFile("internal", "hostagent", "storage_cleanup.go"))),
			"internal/operationreceipt/store.go":                      secureRuntimeHash(secureRuntimeReadFile(t, repoFile("internal", "operationreceipt", "store.go"))),
			"internal/operationreceipt/types.go":                      secureRuntimeHash(secureRuntimeReadFile(t, repoFile("internal", "operationreceipt", "types.go"))),
			"internal/agentupdate/update.go":                          secureRuntimeHash(secureRuntimeReadFile(t, repoFile("internal", "agentupdate", "update.go"))),
			"internal/agentupdate/privileged_update.go":               secureRuntimeHash(secureRuntimeReadFile(t, repoFile("internal", "agentupdate", "privileged_update.go"))),
			"cmd/pulse-agent/main.go":                                 secureRuntimeHash(secureRuntimeReadFile(t, repoFile("cmd", "pulse-agent", "main.go"))),
			"cmd/pulse-agent-helper/main.go":                          secureRuntimeHash(secureRuntimeReadFile(t, repoFile("cmd", "pulse-agent-helper", "main.go"))),
			"cmd/pulse-agent-runner/main.go":                          secureRuntimeHash(secureRuntimeReadFile(t, repoFile("cmd", "pulse-agent-runner", "main.go"))),
		},
		ArtifactHashes:        map[string]string{"collector_v1": secureRuntimeHash(collectorV1), "collector_v2": secureRuntimeHash(collectorV2), "helper": secureRuntimeHash(helper), "runner": secureRuntimeHash(runner)},
		ArtifactVersions:      map[string]string{"collector_v1": collectorV1Version, "collector_v2": collectorV2Version},
		DisposableVMGuardHash: secureRuntimeHash([]byte(secureRuntimeLabMarkerValue + "\n")),
		Architecture:          runtime.GOARCH,
	}
	receipt.OSRelease = strings.TrimSpace(string(secureRuntimeReadFile(t, "/etc/os-release")))
	receipt.Kernel = secureRuntimeCommand(t, 10*time.Second, "uname", "-srvmo")
	receipt.SystemdVersion = strings.SplitN(secureRuntimeCommand(t, 10*time.Second, "systemctl", "--version"), "\n", 2)[0]
	pass := func(name, detail string) {
		receipt.Scenarios = append(receipt.Scenarios, secureRuntimeScenarioResult{Name: name, Passed: true, Detail: detail})
	}

	initialArgs := []string{"--enable-commands", "--command-authority", "command-capable"}
	dockerInitiallyAvailable := secureRuntimeRootfulDockerAvailable()
	if dockerInitiallyAvailable {
		initialArgs = append(initialArgs, "--enable-docker")
	}
	secureRuntimeRunInstaller(t, installerPath, server.URL, initialArgs...)
	secureRuntimeWaitForReports(t, fixture, 1, 45*time.Second)
	secureRuntimeAssertRootCommandProfile(t)
	dockerInitiallyEnabled := secureRuntimeCollectorHasArgument("--enable-docker")
	if dockerInitiallyAvailable && !dockerInitiallyEnabled {
		t.Fatal("rootful Docker was available but the requested legacy profile did not enable Docker monitoring")
	}
	pass("legacy_root_command_capable_install", fmt.Sprintf("root collector installed; docker_enabled=%t", dockerInitiallyEnabled))

	beforeInspect := secureRuntimeStableSnapshot(t)
	inspectOutput := secureRuntimeRunInstaller(t, installerPath, server.URL, "--safe-profile-inspect")
	if !strings.Contains(inspectOutput, "current_profile=legacy-root-command-capable") ||
		!strings.Contains(inspectOutput, "target_profile=typed-helper-monitoring-only") {
		t.Fatalf("read-only inspection did not describe legacy and target profiles:\n%s", inspectOutput)
	}
	if afterInspect := secureRuntimeStableSnapshot(t); !secureRuntimeIdentitiesEqual(beforeInspect, afterInspect) {
		t.Fatal("--safe-profile-inspect mutated installer-owned stable files")
	}
	pass("read_only_inspect", "stable installer-owned files unchanged")

	dropInDir := "/etc/systemd/system/pulse-agent.service.d"
	dropInPath := filepath.Join(dropInDir, "secure-runtime-lab.conf")
	if err := os.MkdirAll(dropInDir, 0o755); err != nil {
		t.Fatalf("create systemd drop-in directory: %v", err)
	}
	if err := os.WriteFile(dropInPath, []byte("[Service]\nUser=root\n"), 0o644); err != nil {
		t.Fatalf("write systemd drop-in: %v", err)
	}
	dropInPresent := true
	t.Cleanup(func() {
		if dropInPresent {
			_ = os.Remove(dropInPath)
			_ = os.Remove(dropInDir)
			_ = exec.Command("systemctl", "daemon-reload").Run()
		}
	})
	secureRuntimeCommand(t, 20*time.Second, "systemctl", "daemon-reload")
	beforeDropIn := secureRuntimeStableSnapshot(t)
	failedOutput, failedErr := secureRuntimeRunInstallerError(t, installerPath, server.URL, "--safe-profile-apply")
	if failedErr == nil || !strings.Contains(failedOutput, "systemd drop-in override") {
		t.Fatalf("safe apply did not fail closed on a systemd drop-in: err=%v\n%s", failedErr, failedOutput)
	}
	if afterDropIn := secureRuntimeStableSnapshot(t); !secureRuntimeIdentitiesEqual(beforeDropIn, afterDropIn) {
		t.Fatal("drop-in rejection mutated installer-owned stable files")
	}
	if err := os.Remove(dropInPath); err != nil {
		t.Fatalf("remove systemd drop-in: %v", err)
	}
	_ = os.Remove(dropInDir)
	dropInPresent = false
	secureRuntimeCommand(t, 20*time.Second, "systemctl", "daemon-reload")
	pass("drop_in_fail_closed_rehearsal", "migration rejected before stable installer-owned files changed")

	legacyBaseline := secureRuntimeStableSnapshot(t)
	reportsBeforeApply, preApplyLastSeen, _, _ := fixture.snapshot()
	fixture.setCollector(collectorV2)
	applyOutput := secureRuntimeRunInstaller(t, installerPath, server.URL, "--safe-profile-apply")
	secureRuntimeWaitForReports(t, fixture, len(reportsBeforeApply)+1, 45*time.Second)
	_, postApplyLastSeen, _, _ := fixture.snapshot()
	if !postApplyLastSeen.After(preApplyLastSeen) {
		t.Fatalf("safe apply committed without fresh server registration: before=%s after=%s", preApplyLastSeen, postApplyLastSeen)
	}
	secureRuntimeAssertSafeProfile(t)
	secureRuntimeAssertHelperProtocol(t)
	if fixture.authorityReductionCount() < 1 {
		t.Fatal("safe apply did not durably reduce the collector credential authority")
	}
	dockerDegraded := dockerInitiallyEnabled && !secureRuntimeCollectorHasArgument("--enable-docker")
	if dockerInitiallyEnabled && !secureRuntimeCollectorOwnedRootlessAvailable(t) {
		if !dockerDegraded || !strings.Contains(applyOutput, "disabled rootful Docker monitoring") {
			t.Fatalf("safe migration did not make rootful Docker degradation explicit:\n%s", applyOutput)
		}
	}
	pass("safe_profile_apply", "fresh server lastSeen, least-privilege identity, typed helper health")

	reportsBeforeRollback, _, _, _ := fixture.snapshot()
	secureRuntimeRunInstaller(t, installerPath, server.URL, "--safe-profile-rollback")
	secureRuntimeWaitForReports(t, fixture, len(reportsBeforeRollback)+1, 45*time.Second)
	secureRuntimeAssertRootMonitoringProfile(t)
	secureRuntimeWaitForStableIdentity(t, secureRuntimeWithoutCollectorUnit(legacyBaseline), 10*time.Second, "explicit rollback")
	pass("explicit_safe_profile_rollback", "legacy binary and service identity restored without resurrecting collector command authority")

	automaticRollbackBaseline := secureRuntimeStableSnapshot(t)
	fixture.setFrozen(true)
	failedOutput, failedErr = secureRuntimeRunInstallerError(t, installerPath, server.URL, "--safe-profile-apply")
	fixture.setFrozen(false)
	if failedErr == nil || !strings.Contains(failedOutput, "restoring the previous profile") {
		t.Fatalf("stale lastSeen safe apply unexpectedly succeeded: err=%v\n%s", failedErr, failedOutput)
	}
	secureRuntimeWaitForStableIdentity(t, secureRuntimeWithoutCollectorUnit(automaticRollbackBaseline), 10*time.Second, "automatic failure rollback")
	secureRuntimeAssertRootMonitoringProfile(t)
	pass("automatic_failure_rollback", "frozen lastSeen prevented commit and restored binary/state identity without command authority")

	fixture.setCollector(collectorV2)
	reportsBeforeUpdate, _, _, _ := fixture.snapshot()
	secureRuntimeRunInstaller(t, installerPath, server.URL, "--update")
	secureRuntimeWaitForReports(t, fixture, len(reportsBeforeUpdate)+1, 45*time.Second)
	secureRuntimeAssertRootMonitoringProfile(t)
	if got := secureRuntimeHash(secureRuntimeReadFile(t, "/usr/local/bin/pulse-agent")); got != secureRuntimeHash(collectorV2) {
		t.Fatalf("ordinary update collector hash = %s, want v2 hash", got)
	}
	pass("ordinary_update_non_migration", "binary updated while the downgraded root monitoring profile remained unchanged")

	fixture.setServerVersion(collectorV2Version)
	reportsBeforeFinalApply, preFinalLastSeen, _, _ := fixture.snapshot()
	secureRuntimeRunInstaller(t, installerPath, server.URL, "--safe-profile-apply")
	secureRuntimeWaitForReports(t, fixture, len(reportsBeforeFinalApply)+1, 45*time.Second)
	_, finalLastSeen, authFailures, requestFailures := fixture.snapshot()
	if !finalLastSeen.After(preFinalLastSeen) {
		t.Fatalf("final safe apply did not advance server lastSeen: before=%s after=%s", preFinalLastSeen, finalLastSeen)
	}
	if authFailures != 0 || len(requestFailures) != 0 {
		t.Fatalf("fixture observed credential/request failures: auth=%d failures=%v", authFailures, requestFailures)
	}
	secureRuntimeAssertSafeProfile(t)
	secureRuntimeAssertHelperProtocol(t)
	reportsBeforeContinuity, _, _, _ := fixture.snapshot()
	secureRuntimeWaitForReports(t, fixture, len(reportsBeforeContinuity)+1, 20*time.Second)
	pass("final_safe_profile_apply", "collector continued reporting after committed migration")

	reportsBeforeRunner, _, _, _ := fixture.snapshot()
	secureRuntimeRunInstallerWithActionCredential(t, installerPath, server.URL, secureRuntimeRunnerSecretV1,
		"--least-privilege", "--enable-privileged-helper", "--enable-action-runner")
	secureRuntimeWaitForActionRunner(t, fixture, true, 30*time.Second)
	secureRuntimeAssertActionRunnerInstalled(t)
	secureRuntimeWaitForReports(t, fixture, len(reportsBeforeRunner)+1, 20*time.Second)
	pass("separate_action_runner_install", "root action runner registered independently while the collector remained non-root and reporting")

	actionContext, cancelAction := context.WithTimeout(agentexec.WithOrganizationID(context.Background(), secureRuntimeLabOrgID), 30*time.Second)
	defer cancelAction()
	cachePath, cacheFingerprint := secureRuntimeSeedAPTPackageCache(t)
	cleanupRequest := agentexec.HostStorageCleanupPayload{
		RequestID: "secure-runtime.receipt.1", ActionID: "secure-runtime-receipt",
		Operation:           agentexec.HostStorageCleanupOperationPackageCache,
		ExpectedFingerprint: cacheFingerprint, Timeout: 15,
	}
	if err := agentexec.BindHostStorageCleanupPayload(&cleanupRequest); err != nil {
		t.Fatalf("bind typed receipt qualification request: %v", err)
	}
	cleanupResult, err := fixture.actionServer.ExecuteHostStorageCleanup(actionContext, secureRuntimeLabAgentID, cleanupRequest)
	if err != nil {
		t.Fatalf("execute typed receipt qualification request: %v", err)
	}
	if cleanupResult == nil || !cleanupResult.Success || !cleanupResult.MutationStarted ||
		cleanupResult.Verification != agentexec.HostStorageCleanupVerificationVerified || cleanupResult.ReclaimedBytes <= 0 {
		t.Fatalf("typed receipt qualification did not complete a verified host mutation: %+v", cleanupResult)
	}
	if _, err := os.Stat(cachePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("typed package-cache cleanup did not remove seeded archive: %v", err)
	}
	receiptIdentity := agentexec.HostStorageCleanupOperationIdentity(secureRuntimeLabAgentID, cleanupRequest)
	query, err := fixture.actionServer.QueryAgentOperation(actionContext, secureRuntimeLabAgentID, receiptIdentity)
	if err != nil || query.Status != operationreceipt.QueryFoundTerminal || query.Record == nil || query.Record.ResultKind != agentexec.HostStorageCleanupReceiptKind {
		t.Fatalf("durable typed receipt query = %+v, err=%v", query, err)
	}
	refusalRequest := agentexec.HostStorageCleanupPayload{
		RequestID: "secure-runtime.receipt.2", ActionID: "secure-runtime-refusal",
		Operation:           agentexec.HostStorageCleanupOperationPackageCache,
		ExpectedFingerprint: "sha256:" + strings.Repeat("f", 64), Timeout: 15,
	}
	if err := agentexec.BindHostStorageCleanupPayload(&refusalRequest); err != nil {
		t.Fatalf("bind typed refusal qualification request: %v", err)
	}
	refusalResult, err := fixture.actionServer.ExecuteHostStorageCleanup(actionContext, secureRuntimeLabAgentID, refusalRequest)
	if err != nil || refusalResult == nil || refusalResult.MutationStarted {
		t.Fatalf("typed refusal request was not rejected before mutation: result=%+v err=%v", refusalResult, err)
	}
	if _, err := fixture.actionServer.ExecuteCommand(actionContext, secureRuntimeLabAgentID, agentexec.ExecuteCommandPayload{
		RequestID: "secure-runtime-shell-denial", Command: "true", TargetType: "agent", Trusted: true,
	}); err == nil || !strings.Contains(err.Error(), "typed action-runner") {
		t.Fatalf("action runner did not reject generic command dispatch: %v", err)
	}
	pass("typed_action_receipt", "verified apt cache mutation and durable terminal receipt; stale fingerprint refused before mutation; generic command dispatch denied")

	currentAdmission, _, _ := fixture.actionSnapshot()
	wrongAdmission := currentAdmission
	wrongAdmission.TokenID = secureRuntimeRunnerBindingV2
	if fixture.actionServer.InvalidateActionRunnerSession(wrongAdmission) {
		t.Fatal("mismatched replacement binding evicted the current action-runner session")
	}
	previousAdmission := fixture.replaceActionCredential(secureRuntimeRunnerSecretV2, secureRuntimeRunnerBindingV2)
	if previousAdmission != currentAdmission || !fixture.actionServer.InvalidateActionRunnerSession(previousAdmission) {
		t.Fatal("credential rotation did not invalidate the exact superseded action-runner session")
	}
	secureRuntimeWaitForActionRunner(t, fixture, false, 10*time.Second)
	secureRuntimeRunInstallerWithActionCredential(t, installerPath, server.URL, secureRuntimeRunnerSecretV2,
		"--least-privilege", "--enable-privileged-helper", "--enable-action-runner")
	secureRuntimeWaitForActionRunner(t, fixture, true, 30*time.Second)
	rotatedAdmission, revoked, _ := fixture.actionSnapshot()
	if revoked || rotatedAdmission.TokenID != secureRuntimeRunnerBindingV2 {
		t.Fatalf("rotated action-runner admission = %+v revoked=%t", rotatedAdmission, revoked)
	}
	pass("action_runner_credential_rotation", "mismatched invalidation was rejected, exact superseded session closed, replacement credential registered")

	reportsBeforeRemoval, _, _, _ := fixture.snapshot()
	uninstallOutput := secureRuntimeRunStandaloneInstaller(t, installerPath, "--uninstall-action-runner")
	if !strings.Contains(uninstallOutput, "Revoked the action-runner credential") {
		t.Fatalf("runner uninstall did not confirm self-revocation:\n%s", uninstallOutput)
	}
	secureRuntimeWaitForActionRunner(t, fixture, false, 10*time.Second)
	secureRuntimeAssertActionRunnerRemoved(t)
	_, revoked, revokeCount := fixture.actionSnapshot()
	if !revoked || revokeCount != 1 {
		t.Fatalf("runner self-revoke state: revoked=%t count=%d", revoked, revokeCount)
	}
	secureRuntimeAssertSafeProfile(t)
	secureRuntimeAssertHelperProtocol(t)
	secureRuntimeWaitForReports(t, fixture, len(reportsBeforeRemoval)+1, 20*time.Second)
	pass("action_runner_self_revoke", "uninstall revoked the exact host binding and removed only runner state; collector/helper continuity remained healthy")

	reports, _, _, _ := fixture.snapshot()
	if len(reports) == 0 {
		t.Fatal("no reports recorded")
	}
	for i, report := range reports {
		if report.AgentID != secureRuntimeLabAgentID {
			t.Fatalf("report %d changed collector identity: %q", i, report.AgentID)
		}
		if report.Hostname != secureRuntimeLabHostname {
			t.Fatalf("report %d changed collector hostname: %q", i, report.Hostname)
		}
	}
	latest := reports[len(reports)-1]
	if latest.RunningAsRoot || latest.ServiceUser != "pulse-agent" || latest.Authority != "monitoring-only" || !latest.TypedHelper || latest.CommandsEnabled {
		t.Fatalf("final report privilege posture = %+v", latest)
	}

	receipt.CompletedAt = time.Now().UTC().Format(time.RFC3339Nano)
	receipt.CollectorServiceUser = secureRuntimeSystemdProperty(t, "User")
	receipt.CollectorProcessUID = secureRuntimeCollectorProcessUID(t)
	receipt.CollectorAuthority = latest.Authority
	receipt.AmbientCapabilitiesNone = strings.TrimSpace(secureRuntimeSystemdProperty(t, "AmbientCapabilities")) == ""
	receipt.HelperProtocolHealthy = true
	receipt.StateIdentityPreserved = true
	receipt.DockerDegraded = dockerDegraded
	receipt.ActionRunnerQualified = true
	receipt.ActionMutationVerified = true
	receipt.CollectorAuthorityReductionRequestObserved = fixture.authorityReductionCount() > 0
	receipt.ActionReceiptKind = agentexec.HostStorageCleanupReceiptKind
	receipt.CredentialRotated = true
	receipt.SelfRevokeObserved = true
	receipt.CollectorContinuity = true
	receipt.ReportCount = len(reports)
	receipt.FirstReportAt = reports[0].ReceivedAt.Format(time.RFC3339Nano)
	receipt.LastReportAt = reports[len(reports)-1].ReceivedAt.Format(time.RFC3339Nano)
	secureRuntimeWriteReceipt(t, receipt)
}

func secureRuntimeRequireDisposableHost(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Fatalf("destructive secure-runtime lab requires Linux, got %s", runtime.GOOS)
	}
	if os.Geteuid() != 0 {
		t.Fatal("destructive secure-runtime lab requires EUID 0")
	}
	comm, err := os.ReadFile("/proc/1/comm")
	if err != nil || strings.TrimSpace(string(comm)) != "systemd" {
		t.Fatalf("destructive secure-runtime lab requires systemd as PID 1: comm=%q err=%v", strings.TrimSpace(string(comm)), err)
	}
	marker, err := os.ReadFile(secureRuntimeLabMarkerPath)
	if err != nil || strings.TrimSpace(string(marker)) != secureRuntimeLabMarkerValue {
		t.Fatalf("refusing host mutation: %s must contain exactly %q", secureRuntimeLabMarkerPath, secureRuntimeLabMarkerValue)
	}
	for _, path := range secureRuntimeInstalledPaths {
		if _, err := os.Lstat(path); err == nil {
			t.Fatalf("refusing already-installed host: %s exists", path)
		} else if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("inspect existing Pulse path %s: %v", path, err)
		}
	}
	if out, err := exec.Command("systemctl", "show", "pulse-agent.service", "--property=LoadState", "--value").CombinedOutput(); err == nil && strings.TrimSpace(string(out)) != "not-found" {
		t.Fatalf("refusing host with loaded pulse-agent.service: %s", strings.TrimSpace(string(out)))
	}
}

func secureRuntimeReadArtifact(t *testing.T, envName string) []byte {
	t.Helper()
	path := strings.TrimSpace(os.Getenv(envName))
	if path == "" || !filepath.IsAbs(path) {
		t.Fatalf("%s must name an absolute caller-built artifact path", envName)
	}
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("inspect %s: %v", envName, err)
	}
	if !info.Mode().IsRegular() || info.Mode()&0o111 == 0 {
		t.Fatalf("%s must be a regular executable file: %s (%s)", envName, path, info.Mode())
	}
	artifact := secureRuntimeReadFile(t, path)
	if len(artifact) < 4 || !bytes.Equal(artifact[:4], []byte{0x7f, 'E', 'L', 'F'}) {
		t.Fatalf("%s is not an ELF executable: %s", envName, path)
	}
	return artifact
}

func secureRuntimeArtifactVersion(t *testing.T, envName string) string {
	t.Helper()
	path := strings.TrimSpace(os.Getenv(envName))
	if path == "" || !filepath.IsAbs(path) {
		t.Fatalf("%s must name an absolute caller-built artifact path", envName)
	}
	version := secureRuntimeCommand(t, 10*time.Second, path, "--version")
	version = strings.TrimSpace(strings.SplitN(version, "\n", 2)[0])
	if version == "" || version == "unknown" {
		t.Fatalf("%s returned unusable --version output %q", envName, version)
	}
	return version
}

func secureRuntimeReadFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}

func secureRuntimeHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func secureRuntimeRunInstaller(t *testing.T, installerPath, fixtureURL string, scenarioArgs ...string) string {
	t.Helper()
	out, err := secureRuntimeRunInstallerErrorWithActionCredential(t, installerPath, fixtureURL, "", scenarioArgs...)
	if err != nil {
		t.Fatalf("installer %s failed: %v\n%s", strings.Join(scenarioArgs, " "), err, out)
	}
	return out
}

func secureRuntimeRunInstallerWithActionCredential(t *testing.T, installerPath, fixtureURL, actionCredential string, scenarioArgs ...string) string {
	t.Helper()
	out, err := secureRuntimeRunInstallerErrorWithActionCredential(t, installerPath, fixtureURL, actionCredential, scenarioArgs...)
	if err != nil {
		t.Fatalf("installer %s failed: %v\n%s", strings.Join(scenarioArgs, " "), err, out)
	}
	return out
}

func secureRuntimeRunInstallerError(t *testing.T, installerPath, fixtureURL string, scenarioArgs ...string) (string, error) {
	t.Helper()
	return secureRuntimeRunInstallerErrorWithActionCredential(t, installerPath, fixtureURL, "", scenarioArgs...)
}

func secureRuntimeRunInstallerErrorWithActionCredential(t *testing.T, installerPath, fixtureURL, actionCredential string, scenarioArgs ...string) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	args := []string{installerPath}
	switch {
	case len(scenarioArgs) == 1 && scenarioArgs[0] == "--safe-profile-inspect":
		args = append(args, scenarioArgs...)
	case len(scenarioArgs) == 1 && scenarioArgs[0] == "--safe-profile-rollback":
		args = append(args, scenarioArgs...)
	default:
		tokenFile := filepath.Join(t.TempDir(), "monitoring-token")
		if err := os.WriteFile(tokenFile, []byte(secureRuntimeLabToken+"\n"), 0o600); err != nil {
			t.Fatalf("write private fixture token: %v", err)
		}
		args = append(args,
			"--url", fixtureURL,
			"--token-file", tokenFile,
			"--interval", "2s",
			"--agent-id", secureRuntimeLabAgentID,
			"--hostname", secureRuntimeLabHostname,
			"--state-dir", "/var/lib/pulse-agent",
			"--insecure",
			"--non-interactive",
		)
		if actionCredential != "" {
			actionCredentialFile := filepath.Join(t.TempDir(), "action-credential")
			if err := os.WriteFile(actionCredentialFile, []byte(actionCredential+"\n"), 0o600); err != nil {
				t.Fatalf("write private action-runner credential: %v", err)
			}
			args = append(args, "--action-token-file", actionCredentialFile)
		}
		args = append(args, scenarioArgs...)
	}
	cmd := exec.CommandContext(ctx, "bash", args...)
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return string(out), fmt.Errorf("installer timed out: %w", ctx.Err())
	}
	secureRuntimeAssertNoCredentialExposure(t, out)
	return string(out), err
}

func secureRuntimeRunStandaloneInstaller(t *testing.T, installerPath string, scenarioArgs ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bash", append([]string{installerPath}, scenarioArgs...)...)
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("standalone installer timed out: %v\n%s", ctx.Err(), out)
	}
	secureRuntimeAssertNoCredentialExposure(t, out)
	if err != nil {
		t.Fatalf("standalone installer %s failed: %v\n%s", strings.Join(scenarioArgs, " "), err, out)
	}
	return string(out)
}

func secureRuntimeAssertNoCredentialExposure(t *testing.T, output []byte) {
	t.Helper()
	for _, credential := range []string{secureRuntimeLabToken, secureRuntimeRunnerSecretV1, secureRuntimeRunnerSecretV2} {
		if bytes.Contains(output, []byte(credential)) {
			t.Fatal("installer output exposed a runtime credential")
		}
	}
}

func secureRuntimeWaitForReports(t *testing.T, fixture *secureRuntimeLabFixture, count int, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		reports, _, _, _ := fixture.snapshot()
		if len(reports) >= count {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	reports, _, _, failures := fixture.snapshot()
	t.Fatalf("timed out waiting for %d reports; got %d (fixture failures: %v)", count, len(reports), failures)
}

func secureRuntimeWaitForActionRunner(t *testing.T, fixture *secureRuntimeLabFixture, connected bool, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fixture.actionServer.IsAgentConnectedForOrganization(secureRuntimeLabOrgID, secureRuntimeLabAgentID) == connected {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for action-runner connected=%t", connected)
}

func secureRuntimeCommand(t *testing.T, timeout time.Duration, name string, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s: %v\n%s", name, strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func secureRuntimeSystemdProperty(t *testing.T, property string) string {
	t.Helper()
	return secureRuntimeCommand(t, 10*time.Second, "systemctl", "show", "pulse-agent.service", "--property="+property, "--value")
}

func secureRuntimeActionRunnerSystemdProperty(t *testing.T, property string) string {
	t.Helper()
	return secureRuntimeCommand(t, 10*time.Second, "systemctl", "show", "pulse-agent-runner.service", "--property="+property, "--value")
}

func secureRuntimeAssertActionRunnerInstalled(t *testing.T) {
	t.Helper()
	secureRuntimeCommand(t, 10*time.Second, "systemctl", "is-active", "pulse-agent-runner.service")
	if user := secureRuntimeActionRunnerSystemdProperty(t, "User"); user != "root" {
		t.Fatalf("action-runner systemd User = %q, want root", user)
	}
	if ambient := strings.TrimSpace(secureRuntimeActionRunnerSystemdProperty(t, "AmbientCapabilities")); ambient != "" {
		t.Fatalf("action-runner AmbientCapabilities = %q, want none", ambient)
	}
	for path, mode := range map[string]os.FileMode{
		"/usr/local/lib/pulse-agent/pulse-agent-runner": 0o755,
		"/etc/pulse-agent-runner/runner.env":            0o600,
		"/etc/pulse-agent-runner/token":                 0o600,
		"/var/lib/pulse-agent-runner/health.json":       0o600,
	} {
		identity := secureRuntimeStableFileIdentity(t, path)
		if identity.UID != 0 || identity.GID != 0 || identity.Mode != mode {
			t.Fatalf("%s identity = uid:%d gid:%d mode:%#o, want root:root %#o", path, identity.UID, identity.GID, identity.Mode, mode)
		}
	}
	secureRuntimeAssertSafeProfile(t)
}

func secureRuntimeAssertActionRunnerRemoved(t *testing.T) {
	t.Helper()
	for _, path := range []string{
		"/usr/local/lib/pulse-agent/pulse-agent-runner",
		"/etc/systemd/system/pulse-agent-runner.service",
		"/etc/pulse-agent-runner",
		"/var/lib/pulse-agent-runner",
	} {
		if _, err := os.Lstat(path); err == nil {
			t.Fatalf("runner-only uninstall left %s", path)
		} else if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("inspect removed action-runner path %s: %v", path, err)
		}
	}
	if loadState := secureRuntimeCommand(t, 10*time.Second, "systemctl", "show", "pulse-agent-runner.service", "--property=LoadState", "--value"); loadState != "not-found" {
		t.Fatalf("removed action-runner unit LoadState = %q, want not-found", loadState)
	}
}

func secureRuntimeCollectorHasArgument(argument string) bool {
	out, err := exec.Command("systemctl", "show", "pulse-agent.service", "--property=ExecStart", "--value").Output()
	if err != nil {
		return false
	}
	for _, field := range strings.Fields(string(out)) {
		if strings.Trim(field, `{ };"`) == argument {
			return true
		}
	}
	return false
}

func secureRuntimeCollectorProcessUID(t *testing.T) int {
	t.Helper()
	pidText := secureRuntimeSystemdProperty(t, "MainPID")
	pid, err := strconv.Atoi(strings.TrimSpace(pidText))
	if err != nil || pid <= 1 {
		t.Fatalf("invalid pulse-agent MainPID %q", pidText)
	}
	status := string(secureRuntimeReadFile(t, fmt.Sprintf("/proc/%d/status", pid)))
	for _, line := range strings.Split(status, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "Uid:" {
			uid, err := strconv.Atoi(fields[1])
			if err != nil {
				t.Fatalf("parse collector UID from %q: %v", line, err)
			}
			return uid
		}
	}
	t.Fatalf("collector process status omitted UID: %s", status)
	return -1
}

func secureRuntimeAssertRootCommandProfile(t *testing.T) {
	t.Helper()
	if user := secureRuntimeSystemdProperty(t, "User"); user != "root" && user != "" {
		t.Fatalf("legacy collector systemd User = %q, want root", user)
	}
	if !secureRuntimeCollectorHasArgument("--enable-commands") {
		t.Fatal("legacy collector is not command-capable")
	}
	if uid := secureRuntimeCollectorProcessUID(t); uid != 0 {
		t.Fatalf("legacy collector process UID = %d, want 0", uid)
	}
	secureRuntimeCommand(t, 10*time.Second, "systemctl", "is-active", "pulse-agent.service")
}

func secureRuntimeAssertRootMonitoringProfile(t *testing.T) {
	t.Helper()
	if user := secureRuntimeSystemdProperty(t, "User"); user != "root" && user != "" {
		t.Fatalf("rolled-back collector systemd User = %q, want root", user)
	}
	if secureRuntimeCollectorHasArgument("--enable-commands") {
		t.Fatal("rolled-back collector resurrected --enable-commands after irreversible credential downgrade")
	}
	if uid := secureRuntimeCollectorProcessUID(t); uid != 0 {
		t.Fatalf("rolled-back collector process UID = %d, want 0", uid)
	}
	secureRuntimeCommand(t, 10*time.Second, "systemctl", "is-active", "pulse-agent.service")
}

func secureRuntimeAssertSafeProfile(t *testing.T) {
	t.Helper()
	if user := secureRuntimeSystemdProperty(t, "User"); user != "pulse-agent" {
		t.Fatalf("safe collector systemd User = %q, want pulse-agent", user)
	}
	if ambient := strings.TrimSpace(secureRuntimeSystemdProperty(t, "AmbientCapabilities")); ambient != "" {
		t.Fatalf("safe collector AmbientCapabilities = %q, want none", ambient)
	}
	if secureRuntimeCollectorHasArgument("--enable-commands") {
		t.Fatal("safe collector retained --enable-commands")
	}
	if environment := secureRuntimeSystemdProperty(t, "Environment"); !strings.Contains(environment, "PULSE_AGENT_HELPER_SOCKET=/run/pulse-agent/helper.sock") {
		t.Fatalf("safe collector lacks typed-helper environment: %s", environment)
	}
	uidText := secureRuntimeCommand(t, 10*time.Second, "id", "-u", "pulse-agent")
	wantUID, err := strconv.Atoi(uidText)
	if err != nil {
		t.Fatalf("parse pulse-agent UID %q: %v", uidText, err)
	}
	if uid := secureRuntimeCollectorProcessUID(t); uid != wantUID {
		t.Fatalf("safe collector process UID = %d, want %d", uid, wantUID)
	}
	groups := strings.Fields(secureRuntimeCommand(t, 10*time.Second, "id", "-nG", "pulse-agent"))
	for _, group := range groups {
		if group == "docker" {
			t.Fatal("safe collector retained rootful docker group membership")
		}
	}
	for _, path := range []string{"/usr/local/bin/pulse-agent", "/usr/local/lib/pulse-agent/pulse-agent-helper"} {
		identity := secureRuntimeStableFileIdentity(t, path)
		if identity.UID != 0 || identity.GID != 0 || identity.Mode != 0o755 {
			t.Fatalf("%s identity = uid:%d gid:%d mode:%#o, want root:root 0755", path, identity.UID, identity.GID, identity.Mode)
		}
	}
	secureRuntimeCommand(t, 10*time.Second, "systemctl", "is-active", "pulse-agent.service")
	secureRuntimeCommand(t, 10*time.Second, "systemctl", "is-active", "pulse-agent-helper.socket")
}

func secureRuntimeStableFileIdentity(t *testing.T, path string) secureRuntimeFileIdentity {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		t.Fatalf("read ownership for %s", path)
	}
	return secureRuntimeFileIdentity{Present: true, Mode: info.Mode().Perm(), UID: stat.Uid, GID: stat.Gid}
}

func secureRuntimeAssertHelperProtocol(t *testing.T) {
	t.Helper()
	requestID := "secure-runtime-lab-health"
	request, err := json.Marshal(map[string]any{
		"protocolVersion":  1,
		"requestId":        requestID,
		"operation":        "helper.health",
		"operationVersion": 1,
		"deadlineMillis":   2000,
		"payload":          map[string]any{},
	})
	if err != nil {
		t.Fatalf("marshal helper health request: %v", err)
	}
	frame := make([]byte, 4+len(request))
	binary.BigEndian.PutUint32(frame[:4], uint32(len(request)))
	copy(frame[4:], request)
	requestHandle, err := os.CreateTemp("/tmp", "pulse-helper-health-*.frame")
	if err != nil {
		t.Fatalf("create helper health frame: %v", err)
	}
	requestFile := requestHandle.Name()
	t.Cleanup(func() { _ = os.Remove(requestFile) })
	if _, err := requestHandle.Write(frame); err != nil {
		_ = requestHandle.Close()
		t.Fatalf("write helper health frame: %v", err)
	}
	if err := requestHandle.Chmod(0o644); err != nil {
		_ = requestHandle.Close()
		t.Fatalf("make helper health frame collector-readable: %v", err)
	}
	if err := requestHandle.Close(); err != nil {
		t.Fatalf("close helper health frame: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "runuser", "-u", "pulse-agent", "--", "curl", "-sS", "--max-time", "5", "--unix-socket", "/run/pulse-agent/helper.sock", "--upload-file", requestFile, "telnet://localhost").Output()
	if err != nil {
		t.Fatalf("typed helper protocol health: %v", err)
	}
	if len(output) < 5 {
		t.Fatalf("typed helper returned short frame: %d bytes", len(output))
	}
	length := int(binary.BigEndian.Uint32(output[:4]))
	if length != len(output)-4 {
		t.Fatalf("typed helper frame length = %d, payload bytes = %d", length, len(output)-4)
	}
	var response struct {
		ProtocolVersion  int    `json:"protocolVersion"`
		RequestID        string `json:"requestId"`
		Operation        string `json:"operation"`
		OperationVersion int    `json:"operationVersion"`
		Success          bool   `json:"success"`
		Result           struct {
			Status string `json:"status"`
		} `json:"result"`
	}
	if err := json.Unmarshal(output[4:], &response); err != nil {
		t.Fatalf("decode typed helper health: %v", err)
	}
	if response.ProtocolVersion != 1 || response.RequestID != requestID || response.Operation != "helper.health" || response.OperationVersion != 1 || !response.Success || response.Result.Status != "ok" {
		t.Fatalf("typed helper health response failed correlation/health: %+v", response)
	}
	secureRuntimeCommand(t, 10*time.Second, "systemctl", "is-active", "pulse-agent-helper.service")
}

func secureRuntimeIdentitiesEqual(left, right secureRuntimeStableIdentity) bool {
	if len(left) != len(right) {
		return false
	}
	for path, identity := range left {
		if right[path] != identity {
			return false
		}
	}
	return true
}

func secureRuntimeWithoutCollectorUnit(identity secureRuntimeStableIdentity) secureRuntimeStableIdentity {
	result := make(secureRuntimeStableIdentity, len(identity)-1)
	for path, value := range identity {
		if path != "/etc/systemd/system/pulse-agent.service" {
			result[path] = value
		}
	}
	return result
}

func secureRuntimeWaitForStableIdentity(t *testing.T, want secureRuntimeStableIdentity, timeout time.Duration, scenario string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var got secureRuntimeStableIdentity
	for time.Now().Before(deadline) {
		got = secureRuntimeStableSnapshot(t)
		if _, compareUnit := want["/etc/systemd/system/pulse-agent.service"]; !compareUnit {
			delete(got, "/etc/systemd/system/pulse-agent.service")
		}
		if secureRuntimeIdentitiesEqual(want, got) {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("%s did not converge to the pre-migration binary/unit/state identity: want=%#v got=%#v", scenario, want, got)
}

func secureRuntimeRootfulDockerAvailable() bool {
	if info, err := os.Stat("/var/run/docker.sock"); err != nil || info.Mode()&os.ModeSocket == 0 {
		return false
	}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", "/var/run/docker.sock")
		},
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 2 * time.Second}
	response, err := client.Get("http://docker/_ping")
	if err != nil {
		return false
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 16))
	return err == nil && response.StatusCode == http.StatusOK && strings.TrimSpace(string(body)) == "OK"
}

func secureRuntimeCollectorOwnedRootlessAvailable(t *testing.T) bool {
	t.Helper()
	uidText, err := exec.Command("id", "-u", "pulse-agent").Output()
	if err != nil {
		return false
	}
	uid := strings.TrimSpace(string(uidText))
	for _, path := range []string{filepath.Join("/run/user", uid, "docker.sock"), filepath.Join("/run/user", uid, "podman", "podman.sock")} {
		if info, err := os.Stat(path); err == nil && info.Mode()&os.ModeSocket != 0 {
			return true
		}
	}
	return false
}

func secureRuntimeWriteReceipt(t *testing.T, receipt secureRuntimeLabReceipt) {
	t.Helper()
	path := strings.TrimSpace(os.Getenv("PULSE_SECURE_RUNTIME_RECEIPT"))
	if path == "" {
		return
	}
	if !filepath.IsAbs(path) {
		t.Fatalf("PULSE_SECURE_RUNTIME_RECEIPT must be an absolute path: %s", path)
	}
	encoded, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		t.Fatalf("marshal secure-runtime receipt: %v", err)
	}
	if bytes.Contains(encoded, []byte(secureRuntimeLabToken)) ||
		bytes.Contains(encoded, []byte(secureRuntimeRunnerSecretV1)) ||
		bytes.Contains(encoded, []byte(secureRuntimeRunnerSecretV2)) ||
		bytes.Contains(bytes.ToLower(encoded), []byte("token")) {
		t.Fatal("refusing to write a receipt containing credential material or token-labelled fields")
	}
	encoded = append(encoded, '\n')
	temporary := path + ".tmp"
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create receipt directory: %v", err)
	}
	if err := os.WriteFile(temporary, encoded, 0o600); err != nil {
		t.Fatalf("write receipt temporary file: %v", err)
	}
	if err := os.Rename(temporary, path); err != nil {
		t.Fatalf("publish receipt: %v", err)
	}
}
