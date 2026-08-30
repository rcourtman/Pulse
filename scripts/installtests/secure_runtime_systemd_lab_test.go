//go:build !windows

package installtests

// TestSecureRuntimeSystemdLab is intentionally excluded from ordinary test
// runs. It installs services and users into a disposable Linux systemd host.
// Build the release-shaped inputs on the host, then copy or mount them into a
// dedicated Colima profile before opting in:
//
//   mkdir -p .lab-artifacts
//   update_seed="$(openssl rand -base64 32)"
//   update_public_key="$(go run ./scripts/release_update_key.go public-key --private-key "$update_seed")"
//   v1_ldflags="$(./scripts/release_ldflags.sh agent --version 6.2.0-lab.1 --update-public-keys "$update_public_key")"
//   v2_ldflags="$(./scripts/release_ldflags.sh agent --version 6.2.0-lab.2 --update-public-keys "$update_public_key")"
//   v3_ldflags="$(./scripts/release_ldflags.sh agent --version 6.2.0-lab.3 --update-public-keys "$update_public_key")"
//   v4_ldflags="$(./scripts/release_ldflags.sh agent --version 6.2.0-lab.4 --update-public-keys "$update_public_key")"
//   CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags "$v1_ldflags" -o .lab-artifacts/pulse-agent-v1 ./cmd/pulse-agent
//   CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags "$v2_ldflags" -o .lab-artifacts/pulse-agent-v2 ./cmd/pulse-agent
//   CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags "$v3_ldflags" -o .lab-artifacts/pulse-agent-v3 ./cmd/pulse-agent
//   CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags "$v4_ldflags" -o .lab-artifacts/pulse-agent-v4 ./cmd/pulse-agent
//   CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags "$v4_ldflags" -o .lab-artifacts/pulse-agent-helper ./cmd/pulse-agent-helper
//   CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o .lab-artifacts/pulse-agent-runner ./cmd/pulse-agent-runner
//   go run ./scripts/release_update_key.go sign --private-key "$update_seed" --file .lab-artifacts/pulse-agent-v1 > .lab-artifacts/pulse-agent-v1.sig
//   go run ./scripts/release_update_key.go sign --private-key "$update_seed" --file .lab-artifacts/pulse-agent-v2 > .lab-artifacts/pulse-agent-v2.sig
//   go run ./scripts/release_update_key.go sign --private-key "$update_seed" --file .lab-artifacts/pulse-agent-v3 > .lab-artifacts/pulse-agent-v3.sig
//   go run ./scripts/release_update_key.go sign --private-key "$update_seed" --file .lab-artifacts/pulse-agent-v4 > .lab-artifacts/pulse-agent-v4.sig
//   unset update_seed
//   CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go test -c -o .lab-artifacts/installtests-linux-arm64.test ./scripts/installtests
//   colima start pulse-agent-qual --activate=false --mount "$PWD:w"
//   colima ssh -p pulse-agent-qual -- sudo sh -c \
//     'printf "%s\n" "PULSE_SECURE_RUNTIME_SYSTEMD_LAB=disposable-v1" > /etc/pulse-secure-runtime-lab'
//   repo="$PWD"
//   colima ssh -p pulse-agent-qual -- sudo env \
//     PULSE_SECURE_RUNTIME_SYSTEMD_LAB=1 \
//     PULSE_SECURE_RUNTIME_COLLECTOR_V1="$repo/.lab-artifacts/pulse-agent-v1" \
//     PULSE_SECURE_RUNTIME_COLLECTOR_V1_SIGNATURE="$repo/.lab-artifacts/pulse-agent-v1.sig" \
//     PULSE_SECURE_RUNTIME_COLLECTOR_V2="$repo/.lab-artifacts/pulse-agent-v2" \
//     PULSE_SECURE_RUNTIME_COLLECTOR_V2_SIGNATURE="$repo/.lab-artifacts/pulse-agent-v2.sig" \
//     PULSE_SECURE_RUNTIME_COLLECTOR_V3="$repo/.lab-artifacts/pulse-agent-v3" \
//     PULSE_SECURE_RUNTIME_COLLECTOR_V3_SIGNATURE="$repo/.lab-artifacts/pulse-agent-v3.sig" \
//     PULSE_SECURE_RUNTIME_COLLECTOR_V4="$repo/.lab-artifacts/pulse-agent-v4" \
//     PULSE_SECURE_RUNTIME_COLLECTOR_V4_SIGNATURE="$repo/.lab-artifacts/pulse-agent-v4.sig" \
//     PULSE_SECURE_RUNTIME_HELPER="$repo/.lab-artifacts/pulse-agent-helper" \
//     PULSE_SECURE_RUNTIME_RUNNER="$repo/.lab-artifacts/pulse-agent-runner" \
//     PULSE_SECURE_RUNTIME_RECEIPT=/tmp/secure-runtime-receipt.json \
//     PULSE_SECURE_RUNTIME_RECEIPT_RECORD_PATH=docs/release-control/v6/internal/records/secure-agent-runtime-systemd-receipt-v6.json \
//     PULSE_SECURE_RUNTIME_TRANSCRIPT=/tmp/secure-runtime-transcript.jsonl \
//     PULSE_SECURE_RUNTIME_TRANSCRIPT_RECORD_PATH=docs/release-control/v6/internal/records/secure-agent-runtime-systemd-transcript-v6.jsonl \
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
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/actionrunner"
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
	secureRuntimeUpdateStatePath = "/var/lib/pulse-agent-helper/update-activation.json"
	secureRuntimeUpdateHandoff   = "/var/lib/pulse-agent/.pulse-agent-update-pending.json"
	secureRuntimeUpdateLKGPath   = "/usr/local/bin/pulse-agent.last-known-good"
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

var secureRuntimeForbiddenReceiptKeys = map[string]struct{}{
	"api_key":       {},
	"authorization": {},
	"bearer":        {},
	"password":      {},
	"refresh_token": {},
	"secret":        {},
	"token":         {},
}

var secureRuntimeScenarioClaims = map[string][]string{
	"legacy_root_command_capable_install": {"legacy_root_command_authority_observed"},
	"read_only_inspect":                   {"inspection_left_stable_files_unchanged"},
	"drop_in_fail_closed_rehearsal":       {"drop_in_rejected_before_mutation"},
	"helper_service_override_rejection":   {"helper_service_effective_override_detected"},
	"helper_resource_limit_override_rejection": {
		"helper_resource_limits_enforced",
		"helper_resource_limit_override_detected",
	},
	"helper_socket_override_rejection": {"helper_socket_effective_override_detected"},
	"safe_profile_apply": {
		"collector_non_root",
		"collector_monitoring_only",
		"helper_protocol_healthy",
		"collector_authority_reduction_observed",
	},
	"explicit_safe_profile_rollback": {"explicit_rollback_preserved_reduced_authority"},
	"automatic_failure_rollback":     {"failed_activation_restored_prior_runtime"},
	"ordinary_update_non_migration":  {"ordinary_update_preserved_selected_profile"},
	"final_safe_profile_apply":       {"collector_reporting_continued_after_migration"},
	"helper_update_authoritative_commit": {
		"signed_helper_activation_observed",
		"activated_process_digest_bound",
		"accepted_primary_report_gated_commit",
		"update_handoff_cleared_after_commit",
	},
	"helper_update_watchdog_rollback": {
		"helper_watchdog_rollback_observed",
		"prior_active_binary_restored_from_rollback_slot",
		"collector_reporting_resumed_after_watchdog_rollback",
	},
	"helper_update_interrupted_recovery": {
		"helper_restart_recovered_pending_activation",
		"prior_active_binary_restored_from_rollback_slot",
		"collector_reporting_resumed_after_helper_recovery",
	},
	"separate_action_runner_install":   {"action_runner_registered_separately"},
	"action_runner_override_rejection": {"action_runner_effective_override_detected"},
	"helper_network_namespace_isolation": {
		"helper_host_interface_tcp_denied",
		"helper_network_namespace_isolated",
	},
	"typed_action_receipt": {
		"typed_mutation_verified",
		"terminal_receipt_replayed",
		"stale_precondition_refused",
		"generic_command_denied",
	},
	"action_runner_credential_rotation": {"fixture_credential_replacement_observed"},
	"action_runner_self_revoke":         {"exact_runner_binding_revoked"},
}

type secureRuntimeLabReport struct {
	ReceivedAt      time.Time
	AgentID         string
	AgentVersion    string
	UpdatedFrom     string
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
	collectorSignature  string
	helper              []byte
	runner              []byte
	serverVersion       string
	reports             []secureRuntimeLabReport
	reportAttempts      []secureRuntimeLabReport
	rejectedVersions    map[string]bool
	lastSeen            time.Time
	freezeLastSeen      bool
	authFailures        int
	requestFailures     []string
	actionServer        *agentexec.Server
	actionSecret        string
	actionBindingID     string
	actionPending       bool
	actionRevoked       bool
	actionRevokes       int
	actionActivations   int
	authorityReductions int
}

func newSecureRuntimeLabFixture(collector []byte, collectorSignature string, helper, runner []byte, version string) *secureRuntimeLabFixture {
	fixture := &secureRuntimeLabFixture{
		collector: collector, collectorSignature: collectorSignature, helper: helper, runner: runner, serverVersion: version,
		rejectedVersions: make(map[string]bool),
		actionSecret:     secureRuntimeRunnerSecretV1, actionBindingID: secureRuntimeRunnerBindingV1, actionPending: true,
	}
	fixture.actionServer = agentexec.NewServerWithAdmissionValidator(fixture.admitCommandSession, fixture.validateCommandSession)
	return fixture
}

func (f *secureRuntimeLabFixture) admitCommandSession(secret, agentID, hostname string) (agentexec.AgentAdmission, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if strings.TrimSpace(agentID) != secureRuntimeLabAgentID || !strings.EqualFold(strings.TrimSpace(hostname), secureRuntimeLabHostname) {
		return agentexec.AgentAdmission{}, false
	}
	if secret == secureRuntimeLabToken {
		return f.collectorAdmissionLocked(), true
	}
	if f.actionRevoked || secret != f.actionSecret {
		return agentexec.AgentAdmission{}, false
	}
	return f.actionAdmissionLocked(), true
}

func (f *secureRuntimeLabFixture) validateCommandSession(admission agentexec.AgentAdmission) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if admission.RuntimeRole != agentexec.RuntimeRoleActionRunner {
		return admission == f.collectorAdmissionLocked()
	}
	expected := f.actionAdmissionLocked()
	return !f.actionRevoked &&
		admission.OrganizationID == expected.OrganizationID &&
		admission.TokenID == expected.TokenID &&
		admission.AgentID == expected.AgentID &&
		admission.Hostname == expected.Hostname &&
		admission.RuntimeRole == expected.RuntimeRole &&
		admission.ActionCapability == expected.ActionCapability
}

func (f *secureRuntimeLabFixture) collectorAdmissionLocked() agentexec.AgentAdmission {
	return agentexec.AgentAdmission{
		OrganizationID: secureRuntimeLabOrgID,
		TokenID:        "secure-runtime-collector-binding-v1",
		AgentID:        secureRuntimeLabAgentID,
		Hostname:       secureRuntimeLabHostname,
	}
}

func (f *secureRuntimeLabFixture) actionAdmissionLocked() agentexec.AgentAdmission {
	return agentexec.AgentAdmission{
		OrganizationID: secureRuntimeLabOrgID, TokenID: f.actionBindingID,
		AgentID: secureRuntimeLabAgentID, Hostname: secureRuntimeLabHostname,
		RuntimeRole: agentexec.RuntimeRoleActionRunner, ActionCapability: agentexec.ActionCapabilityTypedV1,
		ActivationPending: f.actionPending,
	}
}

func (f *secureRuntimeLabFixture) replaceActionCredential(secret, bindingID string) agentexec.AgentAdmission {
	f.mu.Lock()
	defer f.mu.Unlock()
	previous := f.actionAdmissionLocked()
	f.actionSecret = secret
	f.actionBindingID = bindingID
	f.actionPending = true
	f.actionRevoked = false
	return previous
}

func (f *secureRuntimeLabFixture) actionSnapshot() (agentexec.AgentAdmission, bool, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.actionAdmissionLocked(), f.actionRevoked, f.actionRevokes
}

func (f *secureRuntimeLabFixture) actionActivationCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.actionActivations
}

func (f *secureRuntimeLabFixture) authorityReductionCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.authorityReductions
}

func (f *secureRuntimeLabFixture) setCollector(artifact []byte, signature string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.collector = append([]byte(nil), artifact...)
	f.collectorSignature = strings.TrimSpace(signature)
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

func (f *secureRuntimeLabFixture) setVersionReportsAccepted(version string, accepted bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rejectedVersions[strings.TrimSpace(version)] = !accepted
}

func (f *secureRuntimeLabFixture) snapshot() ([]secureRuntimeLabReport, time.Time, int, []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]secureRuntimeLabReport(nil), f.reports...), f.lastSeen, f.authFailures, append([]string(nil), f.requestFailures...)
}

func (f *secureRuntimeLabFixture) attemptSnapshot() []secureRuntimeLabReport {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]secureRuntimeLabReport(nil), f.reportAttempts...)
}

func (f *secureRuntimeLabFixture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == "/api/version" || r.URL.Path == "/api/agent/version":
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
		f.handleActionRunnerCredential(w, r)
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

func (f *secureRuntimeLabFixture) collectorLifecycleAuthorized(r *http.Request) bool {
	bearer := strings.TrimSpace(r.Header.Get("Authorization"))
	legacyHeader := strings.TrimSpace(r.Header.Get("X-API-Token"))
	valid := bearer == "Bearer "+secureRuntimeLabToken &&
		(legacyHeader == "" || legacyHeader == secureRuntimeLabToken) &&
		r.URL.Query().Get("token") == ""
	if !valid {
		f.mu.Lock()
		f.authFailures++
		f.requestFailures = append(f.requestFailures, r.Method+" "+r.URL.Path+": invalid collector lifecycle credential transport")
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
	signature := f.collectorSignature
	switch artifactKind {
	case "helper":
		artifact = append([]byte(nil), f.helper...)
		signature = ""
	case "runner":
		artifact = append([]byte(nil), f.runner...)
		signature = ""
	}
	f.mu.Unlock()
	sum := sha256.Sum256(artifact)
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("X-Checksum-Sha256", hex.EncodeToString(sum[:]))
	if signature != "" {
		w.Header().Set("X-Signature-Ed25519", signature)
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(artifact)))
	if r.Method == http.MethodGet {
		_, _ = w.Write(artifact)
	}
}

func (f *secureRuntimeLabFixture) handleActionRunnerCredential(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPatch:
		f.handleActionRunnerActivation(w, r)
	case http.MethodDelete:
		f.handleActionRunnerSelfRevoke(w, r)
	default:
		w.Header().Set("Allow", http.MethodPatch+", "+http.MethodDelete)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (f *secureRuntimeLabFixture) handleActionRunnerActivation(w http.ResponseWriter, r *http.Request) {
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
	f.mu.Unlock()
	if !valid {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if !f.actionServer.HasActionRunnerSession(admission) {
		http.Error(w, "exact action runner session is not registered", http.StatusConflict)
		return
	}
	if admission.ActivationPending {
		if !f.actionServer.PromoteActionRunnerSession(admission) {
			http.Error(w, "action runner session promotion failed", http.StatusConflict)
			return
		}
		f.mu.Lock()
		if f.actionBindingID != admission.TokenID || f.actionSecret != bearer || f.actionRevoked {
			f.mu.Unlock()
			http.Error(w, "action runner credential changed during activation", http.StatusConflict)
			return
		}
		f.actionPending = false
		f.actionActivations++
		f.mu.Unlock()
	}
	w.WriteHeader(http.StatusNoContent)
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
	w.WriteHeader(http.StatusNoContent)
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
			UpdatedFrom     string `json:"updatedFrom"`
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
		UpdatedFrom:     strings.TrimSpace(payload.Agent.UpdatedFrom),
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
	f.reportAttempts = append(f.reportAttempts, report)
	rejected := f.rejectedVersions[report.AgentVersion]
	serverVersion := f.serverVersion
	if rejected {
		f.mu.Unlock()
		http.Error(w, "report version temporarily rejected by qualification gate", http.StatusServiceUnavailable)
		return
	}
	f.reports = append(f.reports, report)
	if !f.freezeLastSeen {
		f.lastSeen = report.ReceivedAt
	}
	f.mu.Unlock()
	writeSecureRuntimeJSON(w, http.StatusOK, map[string]any{
		"success":       true,
		"agentId":       secureRuntimeLabAgentID,
		"serverVersion": serverVersion,
	})
}

func (f *secureRuntimeLabFixture) handleLookup(w http.ResponseWriter, r *http.Request) {
	if !f.collectorLifecycleAuthorized(r) {
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
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatalf("create disposable apt package cache: %v", err)
	}
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
	Sequence    int                           `json:"sequence"`
	Name        string                        `json:"name"`
	Passed      bool                          `json:"passed"`
	StartedAt   string                        `json:"started_at"`
	CompletedAt string                        `json:"completed_at"`
	Evidence    secureRuntimeScenarioEvidence `json:"evidence"`
}

type secureRuntimeScenarioEvidence struct {
	Kind               string         `json:"kind"`
	Summary            string         `json:"summary"`
	Claims             []string       `json:"claims"`
	Observations       map[string]any `json:"observations"`
	TranscriptEventIDs []string       `json:"transcript_event_ids"`
}

type secureRuntimeTranscriptEvent struct {
	Sequence     int            `json:"sequence"`
	EventID      string         `json:"event_id"`
	ObservedAt   string         `json:"observed_at"`
	Kind         string         `json:"kind"`
	Scenario     string         `json:"scenario,omitempty"`
	Claims       []string       `json:"claims,omitempty"`
	Observations map[string]any `json:"observations,omitempty"`
	Summary      string         `json:"summary,omitempty"`
	Operation    string         `json:"operation,omitempty"`
	Output       string         `json:"output"`
	OutputSHA256 string         `json:"output_sha256,omitempty"`
}

type secureRuntimeSourceManifestBinding struct {
	SchemaVersion int    `json:"schema_version"`
	ManifestID    string `json:"manifest_id"`
	Path          string `json:"path"`
	SHA256        string `json:"sha256"`
	TargetOS      string `json:"target_os"`
	TargetArch    string `json:"target_arch"`
}

type secureRuntimeTranscriptBinding struct {
	Format     string `json:"format"`
	RecordPath string `json:"record_path"`
	SHA256     string `json:"sha256"`
	EventCount int    `json:"event_count"`
}

type secureRuntimeSourceManifest struct {
	SchemaVersion   int      `json:"schema_version"`
	ManifestID      string   `json:"manifest_id"`
	TargetOS        string   `json:"target_os"`
	ExactPaths      []string `json:"exact_paths"`
	RecursiveRoots  []string `json:"recursive_roots"`
	IncludeSuffixes []string `json:"include_suffixes"`
	ExcludeSuffixes []string `json:"exclude_suffixes"`
}

type secureRuntimeLabReceipt struct {
	SchemaVersion                              int                                `json:"schema_version"`
	RecordPath                                 string                             `json:"record_path"`
	StartedAt                                  string                             `json:"started_at"`
	CompletedAt                                string                             `json:"completed_at"`
	SourceManifest                             secureRuntimeSourceManifestBinding `json:"source_manifest"`
	SourceHashes                               map[string]string                  `json:"source_hashes"`
	ArtifactHashes                             map[string]string                  `json:"artifact_hashes"`
	ArtifactVersions                           map[string]string                  `json:"artifact_versions"`
	DisposableVMGuardHash                      string                             `json:"disposable_vm_guard_sha256"`
	OSRelease                                  string                             `json:"os_release"`
	Kernel                                     string                             `json:"kernel"`
	SystemdVersion                             string                             `json:"systemd_version"`
	Architecture                               string                             `json:"architecture"`
	CollectorServiceUser                       string                             `json:"collector_service_user"`
	CollectorProcessUID                        int                                `json:"collector_process_uid"`
	CollectorAuthority                         string                             `json:"collector_authority"`
	AmbientCapabilitiesNone                    bool                               `json:"ambient_capabilities_none"`
	HelperProtocolHealthy                      bool                               `json:"helper_protocol_healthy"`
	StateIdentityPreserved                     bool                               `json:"state_identity_preserved"`
	DockerDegraded                             bool                               `json:"docker_degraded"`
	ActionRunnerQualified                      bool                               `json:"action_runner_qualified"`
	ActionMutationVerified                     bool                               `json:"action_mutation_verified"`
	CollectorAuthorityReductionRequestObserved bool                               `json:"collector_authority_reduction_request_observed"`
	ActionReceiptKind                          string                             `json:"action_receipt_kind,omitempty"`
	CredentialRotated                          bool                               `json:"credential_rotated"`
	SelfRevokeObserved                         bool                               `json:"self_revoke_observed"`
	CollectorContinuity                        bool                               `json:"collector_continuity"`
	ReportCount                                int                                `json:"report_count"`
	FirstReportAt                              string                             `json:"first_report_at"`
	LastReportAt                               string                             `json:"last_report_at"`
	Transcript                                 secureRuntimeTranscriptBinding     `json:"transcript"`
	Scenarios                                  []secureRuntimeScenarioResult      `json:"scenarios"`
}

var secureRuntimeTranscriptRecorder struct {
	sync.Mutex
	enabled bool
	events  []secureRuntimeTranscriptEvent
}

func secureRuntimeResetTranscript(enabled bool) {
	secureRuntimeTranscriptRecorder.Lock()
	defer secureRuntimeTranscriptRecorder.Unlock()
	secureRuntimeTranscriptRecorder.enabled = enabled
	secureRuntimeTranscriptRecorder.events = nil
}

func secureRuntimeRecordTranscriptEvent(event secureRuntimeTranscriptEvent) secureRuntimeTranscriptEvent {
	secureRuntimeTranscriptRecorder.Lock()
	defer secureRuntimeTranscriptRecorder.Unlock()
	if !secureRuntimeTranscriptRecorder.enabled {
		return event
	}
	event.Sequence = len(secureRuntimeTranscriptRecorder.events) + 1
	event.EventID = fmt.Sprintf("event-%04d", event.Sequence)
	if event.ObservedAt == "" {
		event.ObservedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	secureRuntimeTranscriptRecorder.events = append(secureRuntimeTranscriptRecorder.events, event)
	return event
}

func secureRuntimeRecordCommandOutput(operation string, output []byte) {
	secureRuntimeRecordTranscriptEvent(secureRuntimeTranscriptEvent{
		Kind: "command_output", Operation: operation, Output: string(output),
		OutputSHA256: secureRuntimeHash(output),
	})
}

func secureRuntimeTranscriptSnapshot() []secureRuntimeTranscriptEvent {
	secureRuntimeTranscriptRecorder.Lock()
	defer secureRuntimeTranscriptRecorder.Unlock()
	return append([]secureRuntimeTranscriptEvent(nil), secureRuntimeTranscriptRecorder.events...)
}

func secureRuntimeFinalizeTranscript(receipt *secureRuntimeLabReceipt) []secureRuntimeTranscriptEvent {
	events := secureRuntimeTranscriptSnapshot()
	receipt.CompletedAt = time.Now().UTC().Format(time.RFC3339Nano)
	return events
}

func TestSecureRuntimeReceiptCredentialDetection(t *testing.T) {
	safe, err := json.Marshal(map[string]any{
		"source_hashes": map[string]string{
			"internal/api/agenttokens/install.go":      strings.Repeat("a", 64),
			"internal/api/agent_exec_token_binding.go": strings.Repeat("b", 64),
		},
	})
	if err != nil {
		t.Fatalf("marshal safe receipt fixture: %v", err)
	}
	if secureRuntimeReceiptContainsCredential(safe) {
		t.Fatal("public boundary source paths were misclassified as credential material")
	}

	for _, unsafe := range [][]byte{
		[]byte(`{"token":"redacted"}`),
		[]byte(fmt.Sprintf(`{"detail":%q}`, secureRuntimeRunnerSecretV1)),
	} {
		if !secureRuntimeReceiptContainsCredential(unsafe) {
			t.Fatalf("credential-bearing receipt was accepted: %s", unsafe)
		}
	}
}

func TestSecureRuntimeTranscriptPreservesEmptyCommandOutput(t *testing.T) {
	event := secureRuntimeTranscriptEvent{
		Sequence: 1, EventID: "event-0001", ObservedAt: time.Now().UTC().Format(time.RFC3339Nano),
		Kind: "command_output", Operation: "systemctl", OutputSHA256: secureRuntimeHash(nil),
	}
	encoded, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatal(err)
	}
	output, present := payload["output"]
	if !present || output != "" {
		t.Fatalf("empty command output was not represented explicitly: %s", encoded)
	}
}

func TestSecureRuntimeReceiptCompletionEnclosesFinalTranscriptEvent(t *testing.T) {
	secureRuntimeResetTranscript(true)
	t.Cleanup(func() { secureRuntimeResetTranscript(false) })
	event := secureRuntimeRecordTranscriptEvent(secureRuntimeTranscriptEvent{
		Kind: "command_output", Operation: "systemctl", OutputSHA256: secureRuntimeHash(nil),
	})
	receipt := secureRuntimeLabReceipt{}
	events := secureRuntimeFinalizeTranscript(&receipt)
	if len(events) != 1 || events[0].EventID != event.EventID {
		t.Fatalf("final transcript snapshot = %+v", events)
	}
	observedAt, err := time.Parse(time.RFC3339Nano, event.ObservedAt)
	if err != nil {
		t.Fatal(err)
	}
	completedAt, err := time.Parse(time.RFC3339Nano, receipt.CompletedAt)
	if err != nil {
		t.Fatal(err)
	}
	if observedAt.After(completedAt) {
		t.Fatalf("final event %s falls after receipt completion %s", event.ObservedAt, receipt.CompletedAt)
	}
}

func TestSecureRuntimeSourceManifestCoversTransitiveProviders(t *testing.T) {
	_, hashes := secureRuntimeLoadSourceBoundary(t, runtime.GOARCH)
	for _, required := range []string{
		"internal/agenthelper/providers.go",
		"internal/agenthelper/container_inventory.go",
		"internal/agenttls/config.go",
		"internal/hostagent/action_runner_client.go",
		"internal/hostagent/action_runner_health_persistence_unix.go",
		"internal/hostagent/package_updates.go",
		"internal/api/action_runner_credentials.go",
		"internal/api/router_routes_registration.go",
		"internal/agentexec/server.go",
		"internal/securityutil/secure_storage_dir.go",
		"pkg/auth/scopes.go",
		"pkg/securityutil/httpurl.go",
		"scripts/release_control/secure_runtime_attestation.py",
		"scripts/release_control/secure_runtime_attestation_v6.py",
		".github/workflows/build-release-candidate.yml",
		".github/workflows/compile-release-payload.yml",
		".github/workflows/create-release.yml",
		"scripts/build-release-binaries.sh",
		"scripts/build-release.sh",
		"scripts/release_asset_common.sh",
		"scripts/release_build_targets.sh",
		"scripts/release_candidate_manifest.py",
		"scripts/release_ldflags.sh",
		"scripts/release_update_key.go",
		"scripts/require-safe-gh-attestation.sh",
		"scripts/validate-release.sh",
		"scripts/verify-github-release-integrity.sh",
	} {
		if _, ok := hashes[required]; !ok {
			t.Fatalf("secure-runtime source manifest omitted transitive boundary source %s", required)
		}
	}
	for sourcePath := range hashes {
		if strings.HasSuffix(sourcePath, "_test.go") && sourcePath != "scripts/installtests/secure_runtime_systemd_lab_test.go" {
			t.Fatalf("secure-runtime production source manifest included test source %s", sourcePath)
		}
	}
}

func TestSecureRuntimeFixturePromotesPendingRunner(t *testing.T) {
	fixture := newSecureRuntimeLabFixture(nil, "", nil, nil, "fixture")
	defer fixture.actionServer.Shutdown()
	server := httptest.NewServer(fixture)
	defer server.Close()

	stateDir := t.TempDir()
	healthPath := filepath.Join(stateDir, "health.json")
	client := actionrunner.NewClient(actionrunner.TransportConfig{
		PulseURL: server.URL, APIToken: secureRuntimeRunnerSecretV1,
		StateDir: stateDir, HealthPath: healthPath, InsecureSkipVerify: true,
		ActivationNonce: strings.Repeat("a", 64),
	}, secureRuntimeLabAgentID, secureRuntimeLabHostname, "fixture")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- client.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		_ = client.Close()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("timed out stopping fixture action runner")
		}
	})

	secureRuntimeWaitForActionRunner(t, fixture, true, 10*time.Second)
	admission, revoked, _ := fixture.actionSnapshot()
	if revoked || admission.ActivationPending || fixture.actionActivationCount() != 1 {
		t.Fatalf("fixture activation state = admission:%+v revoked:%t activations:%d", admission, revoked, fixture.actionActivationCount())
	}
	var health struct {
		Registered bool `json:"registered"`
		Activated  bool `json:"activated"`
	}
	if err := json.Unmarshal(secureRuntimeReadFile(t, healthPath), &health); err != nil {
		t.Fatalf("decode fixture action-runner health: %v", err)
	}
	if !health.Registered || !health.Activated {
		t.Fatalf("fixture action-runner health = %+v", health)
	}
}

func TestSecureRuntimeFixtureAdmitsCollectorAndRunnerCredentialsSeparately(t *testing.T) {
	fixture := newSecureRuntimeLabFixture(nil, "", nil, nil, "fixture")
	defer fixture.actionServer.Shutdown()

	collector, admitted := fixture.admitCommandSession(secureRuntimeLabToken, secureRuntimeLabAgentID, secureRuntimeLabHostname)
	if !admitted || collector.RuntimeRole == agentexec.RuntimeRoleActionRunner || !fixture.validateCommandSession(collector) {
		t.Fatalf("collector admission = %+v, admitted=%t", collector, admitted)
	}
	actionRunner, admitted := fixture.admitCommandSession(secureRuntimeRunnerSecretV1, secureRuntimeLabAgentID, secureRuntimeLabHostname)
	if !admitted || actionRunner.RuntimeRole != agentexec.RuntimeRoleActionRunner || !fixture.validateCommandSession(actionRunner) {
		t.Fatalf("action-runner admission = %+v, admitted=%t", actionRunner, admitted)
	}
	if collector.TokenID == actionRunner.TokenID {
		t.Fatal("collector and action runner shared a fixture token identity")
	}
	if _, admitted := fixture.admitCommandSession("wrong-secret", secureRuntimeLabAgentID, secureRuntimeLabHostname); admitted {
		t.Fatal("fixture admitted an unknown command credential")
	}
}

func TestSecureRuntimeFixtureAcceptsBearerOnlyCollectorLifecycleLookup(t *testing.T) {
	fixture := newSecureRuntimeLabFixture(nil, "", nil, nil, "fixture")
	defer fixture.actionServer.Shutdown()
	fixture.mu.Lock()
	fixture.lastSeen = time.Now().UTC()
	fixture.mu.Unlock()

	request := httptest.NewRequest(http.MethodGet, "/api/agents/agent/lookup?agentId="+secureRuntimeLabAgentID, nil)
	request.Header.Set("Authorization", "Bearer "+secureRuntimeLabToken)
	recorder := httptest.NewRecorder()
	fixture.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("bearer-only lifecycle lookup status = %d, body=%s", recorder.Code, recorder.Body.String())
	}

	request = httptest.NewRequest(http.MethodGet, "/api/agents/agent/lookup?agentId="+secureRuntimeLabAgentID, nil)
	recorder = httptest.NewRecorder()
	fixture.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated lifecycle lookup status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}

func TestSecureRuntimeFixtureSelfRevokeUsesBodylessNoContentResponse(t *testing.T) {
	fixture := newSecureRuntimeLabFixture(nil, "", nil, nil, "fixture")
	defer fixture.actionServer.Shutdown()
	body := fmt.Sprintf(`{"agentId":%q,"hostname":%q}`, secureRuntimeLabAgentID, secureRuntimeLabHostname)
	request := httptest.NewRequest(http.MethodDelete, "/api/agents/action-runner/credential", strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+secureRuntimeRunnerSecretV1)
	recorder := httptest.NewRecorder()
	fixture.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNoContent || recorder.Body.Len() != 0 {
		t.Fatalf("self-revoke response = status %d body %q, want bodyless 204", recorder.Code, recorder.Body.String())
	}
	_, revoked, revokeCount := fixture.actionSnapshot()
	if !revoked || revokeCount != 1 {
		t.Fatalf("self-revoke state = revoked:%t count:%d", revoked, revokeCount)
	}
}

func TestSecureRuntimeSystemdLab(t *testing.T) {
	if os.Getenv(secureRuntimeLabOptIn) != "1" {
		t.Skip("set PULSE_SECURE_RUNTIME_SYSTEMD_LAB=1 only inside a disposable systemd VM")
	}
	secureRuntimeRequireDisposableHost(t)

	collectorV1 := secureRuntimeReadArtifact(t, "PULSE_SECURE_RUNTIME_COLLECTOR_V1")
	collectorV1Signature := secureRuntimeReadSignature(t, "PULSE_SECURE_RUNTIME_COLLECTOR_V1_SIGNATURE")
	collectorV2 := secureRuntimeReadArtifact(t, "PULSE_SECURE_RUNTIME_COLLECTOR_V2")
	collectorV2Signature := secureRuntimeReadSignature(t, "PULSE_SECURE_RUNTIME_COLLECTOR_V2_SIGNATURE")
	collectorV3 := secureRuntimeReadArtifact(t, "PULSE_SECURE_RUNTIME_COLLECTOR_V3")
	collectorV3Signature := secureRuntimeReadSignature(t, "PULSE_SECURE_RUNTIME_COLLECTOR_V3_SIGNATURE")
	collectorV4 := secureRuntimeReadArtifact(t, "PULSE_SECURE_RUNTIME_COLLECTOR_V4")
	collectorV4Signature := secureRuntimeReadSignature(t, "PULSE_SECURE_RUNTIME_COLLECTOR_V4_SIGNATURE")
	helper := secureRuntimeReadArtifact(t, "PULSE_SECURE_RUNTIME_HELPER")
	runner := secureRuntimeReadArtifact(t, "PULSE_SECURE_RUNTIME_RUNNER")
	collectorV1Version := secureRuntimeArtifactVersion(t, "PULSE_SECURE_RUNTIME_COLLECTOR_V1")
	collectorV2Version := secureRuntimeArtifactVersion(t, "PULSE_SECURE_RUNTIME_COLLECTOR_V2")
	collectorV3Version := secureRuntimeArtifactVersion(t, "PULSE_SECURE_RUNTIME_COLLECTOR_V3")
	collectorV4Version := secureRuntimeArtifactVersion(t, "PULSE_SECURE_RUNTIME_COLLECTOR_V4")
	if collectorV1Version == collectorV2Version {
		t.Fatalf("collector V1 and V2 must have distinct --version output, both reported %q", collectorV1Version)
	}
	installerPath, err := filepath.Abs(repoFile("scripts", "install.sh"))
	if err != nil {
		t.Fatalf("resolve installer path: %v", err)
	}

	fixture := newSecureRuntimeLabFixture(collectorV1, collectorV1Signature, helper, runner, collectorV1Version)
	defer fixture.actionServer.Shutdown()
	server := httptest.NewServer(fixture)
	defer server.Close()

	recordPath := strings.TrimSpace(os.Getenv("PULSE_SECURE_RUNTIME_RECEIPT_RECORD_PATH"))
	if recordPath == "" || filepath.IsAbs(recordPath) || filepath.ToSlash(filepath.Clean(recordPath)) != recordPath || strings.Contains(recordPath, "..") {
		t.Fatalf("PULSE_SECURE_RUNTIME_RECEIPT_RECORD_PATH must be a canonical repository-relative path: %q", recordPath)
	}
	startedAt := time.Now().UTC()
	sourceManifest, sourceHashes := secureRuntimeLoadSourceBoundary(t, runtime.GOARCH)
	receipt := secureRuntimeLabReceipt{
		SchemaVersion:         6,
		RecordPath:            recordPath,
		StartedAt:             startedAt.Format(time.RFC3339Nano),
		SourceManifest:        sourceManifest,
		SourceHashes:          sourceHashes,
		ArtifactHashes:        map[string]string{"collector_v1": secureRuntimeHash(collectorV1), "collector_v2": secureRuntimeHash(collectorV2), "collector_v3": secureRuntimeHash(collectorV3), "collector_v4": secureRuntimeHash(collectorV4), "helper": secureRuntimeHash(helper), "runner": secureRuntimeHash(runner)},
		ArtifactVersions:      map[string]string{"collector_v1": collectorV1Version, "collector_v2": collectorV2Version, "collector_v3": collectorV3Version, "collector_v4": collectorV4Version},
		DisposableVMGuardHash: secureRuntimeHash([]byte(secureRuntimeLabMarkerValue + "\n")),
		Architecture:          runtime.GOARCH,
	}
	secureRuntimeResetTranscript(true)
	t.Cleanup(func() { secureRuntimeResetTranscript(false) })
	receipt.OSRelease = strings.TrimSpace(string(secureRuntimeReadFile(t, "/etc/os-release")))
	receipt.Kernel = secureRuntimeCommand(t, 10*time.Second, "uname", "-srvmo")
	receipt.SystemdVersion = strings.SplitN(secureRuntimeCommand(t, 10*time.Second, "systemctl", "--version"), "\n", 2)[0]
	scenarioStartedAt := startedAt
	pass := func(name, detail string, observations map[string]any) {
		claims, ok := secureRuntimeScenarioClaims[name]
		if !ok {
			t.Fatalf("scenario %q has no governed causal claims", name)
		}
		completedAt := time.Now().UTC()
		sequence := len(receipt.Scenarios) + 1
		claims = append([]string(nil), claims...)
		event := secureRuntimeRecordTranscriptEvent(secureRuntimeTranscriptEvent{
			ObservedAt: completedAt.Format(time.RFC3339Nano), Kind: "scenario_result",
			Scenario: name, Claims: claims, Observations: observations, Summary: detail,
		})
		receipt.Scenarios = append(receipt.Scenarios, secureRuntimeScenarioResult{
			Sequence: sequence, Name: name, Passed: true,
			StartedAt: scenarioStartedAt.Format(time.RFC3339Nano), CompletedAt: completedAt.Format(time.RFC3339Nano),
			Evidence: secureRuntimeScenarioEvidence{
				Kind: "runtime-observation-v1", Summary: detail, Claims: claims,
				Observations:       observations,
				TranscriptEventIDs: []string{event.EventID},
			},
		})
		scenarioStartedAt = completedAt
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
	pass("legacy_root_command_capable_install", fmt.Sprintf("root collector installed; docker_enabled=%t", dockerInitiallyEnabled), map[string]any{"collector_process_uid": 0, "commands_enabled": true})

	beforeInspect := secureRuntimeStableSnapshot(t)
	inspectOutput := secureRuntimeRunInstaller(t, installerPath, server.URL, "--safe-profile-inspect")
	if !strings.Contains(inspectOutput, "current_profile=legacy-root-command-capable") ||
		!strings.Contains(inspectOutput, "target_profile=typed-helper-monitoring-only") {
		t.Fatalf("read-only inspection did not describe legacy and target profiles:\n%s", inspectOutput)
	}
	if afterInspect := secureRuntimeStableSnapshot(t); !secureRuntimeIdentitiesEqual(beforeInspect, afterInspect) {
		t.Fatal("--safe-profile-inspect mutated installer-owned stable files")
	}
	pass("read_only_inspect", "stable installer-owned files unchanged", map[string]any{"stable_files_unchanged": true})

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
	pass("drop_in_fail_closed_rehearsal", "migration rejected before stable installer-owned files changed", map[string]any{"rejected_before_mutation": true})

	legacyBaseline := secureRuntimeStableSnapshot(t)
	reportsBeforeApply, preApplyLastSeen, _, _ := fixture.snapshot()
	fixture.setCollector(collectorV2, collectorV2Signature)
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
		if dockerDegraded || !strings.Contains(applyOutput, "typed helper in summary-only mode") {
			t.Fatalf("safe migration did not preserve explicitly reduced helper inventory:\n%s", applyOutput)
		}
	}
	pass("safe_profile_apply", "fresh server lastSeen, least-privilege identity, typed helper health", map[string]any{"collector_service_user": "pulse-agent", "collector_authority": "monitoring-only", "helper_status": "ok"})

	reportsBeforeRollback, _, _, _ := fixture.snapshot()
	secureRuntimeRunInstaller(t, installerPath, server.URL, "--safe-profile-rollback")
	secureRuntimeWaitForReports(t, fixture, len(reportsBeforeRollback)+1, 45*time.Second)
	secureRuntimeAssertRootMonitoringProfile(t)
	secureRuntimeWaitForStableIdentity(t, secureRuntimeWithoutCollectorUnit(legacyBaseline), 10*time.Second, "explicit rollback")
	pass("explicit_safe_profile_rollback", "legacy binary and service identity restored without resurrecting collector command authority", map[string]any{"restored_profile": "root-monitoring", "commands_enabled": false})

	automaticRollbackBaseline := secureRuntimeStableSnapshot(t)
	fixture.setFrozen(true)
	failedOutput, failedErr = secureRuntimeRunInstallerError(t, installerPath, server.URL, "--safe-profile-apply")
	fixture.setFrozen(false)
	if failedErr == nil || !strings.Contains(failedOutput, "restoring the previous profile") {
		t.Fatalf("stale lastSeen safe apply unexpectedly succeeded: err=%v\n%s", failedErr, failedOutput)
	}
	secureRuntimeWaitForStableIdentity(t, secureRuntimeWithoutCollectorUnit(automaticRollbackBaseline), 10*time.Second, "automatic failure rollback")
	secureRuntimeAssertRootMonitoringProfile(t)
	pass("automatic_failure_rollback", "frozen lastSeen prevented commit and restored binary/state identity without command authority", map[string]any{"activation_committed": false, "restored_profile": "root-monitoring"})

	fixture.setCollector(collectorV2, collectorV2Signature)
	reportsBeforeUpdate, _, _, _ := fixture.snapshot()
	secureRuntimeRunInstaller(t, installerPath, server.URL, "--update")
	secureRuntimeWaitForReports(t, fixture, len(reportsBeforeUpdate)+1, 45*time.Second)
	secureRuntimeAssertRootMonitoringProfile(t)
	if got := secureRuntimeHash(secureRuntimeReadFile(t, "/usr/local/bin/pulse-agent")); got != secureRuntimeHash(collectorV2) {
		t.Fatalf("ordinary update collector hash = %s, want v2 hash", got)
	}
	pass("ordinary_update_non_migration", "binary updated while the downgraded root monitoring profile remained unchanged", map[string]any{"collector_v2_installed": true, "selected_profile": "root-monitoring"})

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
	pass("final_safe_profile_apply", "collector continued reporting after committed migration", map[string]any{"collector_service_user": "pulse-agent", "continuity_report_observed": true})

	helperServiceRejection := secureRuntimeExerciseUnitOverrideDetection(t,
		"pulse-agent-helper.service", "Service", "PrivateNetwork=false", false)
	pass("helper_service_override_rejection", "effective helper service validation rejected an existing systemd drop-in", map[string]any{
		"override_directive": "PrivateNetwork=false",
		"validation_error":   helperServiceRejection,
	})
	helperResourceRejection := secureRuntimeExerciseUnitOverrideDetection(t,
		"pulse-agent-helper.service", "Service", "TasksMax=infinity", false)
	pass("helper_resource_limit_override_rejection", "effective helper service validation enforced bounded task, descriptor, and memory resources and rejected an unbounded task override", map[string]any{
		"override_directive": "TasksMax=infinity",
		"tasks_max":          "64",
		"limit_nofile":       "256",
		"memory_max_bytes":   "268435456",
		"validation_error":   helperResourceRejection,
	})
	helperSocketRejection := secureRuntimeExerciseUnitOverrideDetection(t,
		"pulse-agent-helper.socket", "Socket", "SocketMode=0666", false)
	pass("helper_socket_override_rejection", "effective helper socket validation rejected an existing systemd drop-in", map[string]any{
		"override_directive": "SocketMode=0666",
		"validation_error":   helperSocketRejection,
	})
	helperNetworkObservations := secureRuntimeAssertHelperOutboundNetworkDenied(t)
	pass("helper_network_namespace_isolation", "the helper network namespace could not establish TCP to a host-interface canary that was reachable from the host namespace", helperNetworkObservations)

	collectorV2SHA256 := secureRuntimeHash(collectorV2)
	collectorV3SHA256 := secureRuntimeHash(collectorV3)
	collectorV4SHA256 := secureRuntimeHash(collectorV4)
	preUpdatePID := secureRuntimeCollectorMainPID(t)
	fixture.setVersionReportsAccepted(collectorV3Version, false)
	fixture.setCollector(collectorV3, collectorV3Signature)
	fixture.setServerVersion(collectorV3Version)
	pendingV3 := secureRuntimeWaitForUpdateState(t, "pending", collectorV3SHA256, collectorV2SHA256, 45*time.Second)
	secureRuntimeWaitForReportAttempt(t, fixture, collectorV3Version, collectorV2Version, 45*time.Second)
	if pendingV3.ActivatorPID != preUpdatePID {
		t.Fatalf("V3 activation peer PID = %d, want pre-exec collector PID %d", pendingV3.ActivatorPID, preUpdatePID)
	}
	postExecPID := secureRuntimeCollectorMainPID(t)
	if postExecPID != preUpdatePID {
		t.Fatalf("helper-backed exec changed systemd MainPID from %d to %d", preUpdatePID, postExecPID)
	}
	if got := secureRuntimeProcessExecutableHash(t, postExecPID); got != collectorV3SHA256 {
		t.Fatalf("/proc/%d/exe digest = %s, want V3 %s", postExecPID, got, collectorV3SHA256)
	}
	secureRuntimeAssertUpdateBinaryIdentities(t, collectorV3SHA256, collectorV2SHA256)
	if _, err := os.Stat(secureRuntimeUpdateHandoff); err != nil {
		t.Fatalf("pending helper update handoff unavailable: %v", err)
	}
	fixture.setVersionReportsAccepted(collectorV3Version, true)
	secureRuntimeWaitForAcceptedVersion(t, fixture, collectorV3Version, 1, 30*time.Second)
	committedV3 := secureRuntimeWaitForUpdateState(t, "committed", collectorV3SHA256, collectorV2SHA256, 30*time.Second)
	secureRuntimeWaitForFileAbsent(t, secureRuntimeUpdateHandoff, 15*time.Second)
	if committedV3.ActivatorPID != postExecPID {
		t.Fatalf("committed V3 activator PID = %d, want current collector PID %d", committedV3.ActivatorPID, postExecPID)
	}
	secureRuntimeAssertUpdateBinaryIdentities(t, collectorV3SHA256, collectorV2SHA256)
	secureRuntimeAssertSafeProfile(t)
	pass("helper_update_authoritative_commit", "signed helper activation retained the systemd process identity and committed only after a freshly accepted primary report", map[string]any{
		"signature_verified":      true,
		"candidate_sha256":        collectorV3SHA256,
		"prior_sha256":            collectorV2SHA256,
		"target_sha256":           collectorV3SHA256,
		"last_known_good_sha256":  collectorV2SHA256,
		"activator_pid":           pendingV3.ActivatorPID,
		"committer_pid":           committedV3.ActivatorPID,
		"accepted_primary_report": true,
		"update_action":           "committed",
		"handoff_cleared":         true,
		"reporting_continuity":    true,
	})

	fixture.setVersionReportsAccepted(collectorV4Version, false)
	fixture.setCollector(collectorV4, collectorV4Signature)
	fixture.setServerVersion(collectorV4Version)
	pendingV4Watchdog := secureRuntimeWaitForUpdateState(t, "pending", collectorV4SHA256, collectorV3SHA256, 45*time.Second)
	secureRuntimeWaitForReportAttempt(t, fixture, collectorV4Version, collectorV3Version, 45*time.Second)
	watchdogPID := secureRuntimeCollectorMainPID(t)
	if pendingV4Watchdog.ActivatorPID != watchdogPID || secureRuntimeProcessExecutableHash(t, watchdogPID) != collectorV4SHA256 {
		t.Fatalf("watchdog candidate identity mismatch: state=%+v pid=%d", pendingV4Watchdog, watchdogPID)
	}
	acceptedV3BeforeWatchdog := len(secureRuntimeWaitForAcceptedVersion(t, fixture, collectorV3Version, 1, 5*time.Second))
	fixture.setServerVersion(collectorV3Version)
	secureRuntimeCommand(t, 10*time.Second, "systemctl", "kill", "--kill-who=main", "--signal=STOP", "pulse-agent.service")
	secureRuntimeWaitForUpdateState(t, "rolled_back", collectorV3SHA256, collectorV4SHA256, 150*time.Second)
	secureRuntimeAssertUpdateBinaryIdentities(t, collectorV3SHA256, collectorV4SHA256)
	secureRuntimeCommand(t, 10*time.Second, "systemctl", "kill", "--kill-who=main", "--signal=KILL", "pulse-agent.service")
	secureRuntimeWaitForAcceptedVersion(t, fixture, collectorV3Version, acceptedV3BeforeWatchdog+1, 45*time.Second)
	secureRuntimeWaitForFileAbsent(t, secureRuntimeUpdateHandoff, 30*time.Second)
	secureRuntimeAssertSafeProfile(t)
	pass("helper_update_watchdog_rollback", "the independent helper watchdog restored V3 after the unresponsive V4 collector missed its production rollback deadline", map[string]any{
		"candidate_sha256":       collectorV4SHA256,
		"prior_sha256":           collectorV3SHA256,
		"target_sha256":          collectorV3SHA256,
		"last_known_good_sha256": collectorV4SHA256,
		"update_action":          "rolled_back",
		"rollback_trigger":       "watchdog",
		"reporting_continuity":   true,
	})

	fixture.setServerVersion(collectorV4Version)
	pendingV4Recovery := secureRuntimeWaitForUpdateState(t, "pending", collectorV4SHA256, collectorV3SHA256, 45*time.Second)
	recoveryPID := secureRuntimeCollectorMainPID(t)
	if pendingV4Recovery.ActivatorPID != recoveryPID || secureRuntimeProcessExecutableHash(t, recoveryPID) != collectorV4SHA256 {
		t.Fatalf("interrupted-recovery candidate identity mismatch: state=%+v pid=%d", pendingV4Recovery, recoveryPID)
	}
	fixture.setServerVersion(collectorV3Version)
	secureRuntimeCommand(t, 10*time.Second, "systemctl", "kill", "--kill-who=main", "--signal=STOP", "pulse-agent.service")
	acceptedV3BeforeRecovery := len(secureRuntimeWaitForAcceptedVersion(t, fixture, collectorV3Version, acceptedV3BeforeWatchdog+1, 5*time.Second))
	secureRuntimeCommand(t, 20*time.Second, "systemctl", "restart", "pulse-agent-helper.service")
	secureRuntimeWaitForUpdateState(t, "rolled_back", collectorV3SHA256, collectorV4SHA256, 20*time.Second)
	secureRuntimeAssertUpdateBinaryIdentities(t, collectorV3SHA256, collectorV4SHA256)
	secureRuntimeCommand(t, 10*time.Second, "systemctl", "kill", "--kill-who=main", "--signal=KILL", "pulse-agent.service")
	secureRuntimeWaitForAcceptedVersion(t, fixture, collectorV3Version, acceptedV3BeforeRecovery+1, 45*time.Second)
	secureRuntimeWaitForFileAbsent(t, secureRuntimeUpdateHandoff, 30*time.Second)
	secureRuntimeAssertSafeProfile(t)
	pass("helper_update_interrupted_recovery", "restarting only the helper recovered the durable pending activation before the stopped collector could participate", map[string]any{
		"candidate_sha256":       collectorV4SHA256,
		"prior_sha256":           collectorV3SHA256,
		"target_sha256":          collectorV3SHA256,
		"last_known_good_sha256": collectorV4SHA256,
		"update_action":          "rolled_back",
		"rollback_trigger":       "helper-restart",
		"reporting_continuity":   true,
	})

	fixture.setCollector(collectorV3, collectorV3Signature)
	reportsBeforeRunner, _, _, _ := fixture.snapshot()
	secureRuntimeRunInstallerWithActionCredential(t, installerPath, server.URL, secureRuntimeRunnerSecretV1,
		"--least-privilege", "--enable-privileged-helper", "--enable-action-runner")
	secureRuntimeWaitForActionRunner(t, fixture, true, 30*time.Second)
	secureRuntimeAssertActionRunnerInstalled(t)
	if activations := fixture.actionActivationCount(); activations != 1 {
		t.Fatalf("initial action-runner activation requests = %d, want 1", activations)
	}
	secureRuntimeWaitForReports(t, fixture, len(reportsBeforeRunner)+1, 20*time.Second)
	pass("separate_action_runner_install", "root action runner registered and activated independently while the collector remained non-root and reporting", map[string]any{"runner_service_user": "root", "collector_service_user": "pulse-agent", "fixture_activation_requests": 1})
	runnerOverrideRejection := secureRuntimeExerciseUnitOverrideDetection(t,
		"pulse-agent-runner.service", "Service", "EnvironmentFile=-/tmp/unsafe-runner.env", true)
	pass("action_runner_override_rejection", "effective action-runner validation rejected an existing systemd drop-in", map[string]any{
		"override_directive": "EnvironmentFile=-/tmp/unsafe-runner.env",
		"validation_error":   runnerOverrideRejection,
	})

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
	pass("typed_action_receipt", "verified apt cache mutation and durable terminal receipt; stale fingerprint refused before mutation; generic command dispatch denied", map[string]any{"action_receipt_kind": agentexec.HostStorageCleanupReceiptKind, "mutation_started": true, "verification": "verified", "stale_precondition_mutation_started": false, "generic_command_denied": true})

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
	if activations := fixture.actionActivationCount(); activations != 2 {
		t.Fatalf("action-runner activation requests after rotation = %d, want 2", activations)
	}
	pass("action_runner_credential_rotation", "mismatched invalidation was rejected, exact superseded session closed, replacement credential registered and activated", map[string]any{"proof_scope": "in-memory-fixture", "superseded_session_invalidated": true, "replacement_registered": true, "fixture_activation_requests": 2})

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
	pass("action_runner_self_revoke", "uninstall revoked the exact host binding and removed only runner state; collector/helper continuity remained healthy", map[string]any{"revocation_count": revokeCount, "collector_continuity": true})

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
	secureRuntimeWriteEvidence(t, &receipt, secureRuntimeFinalizeTranscript(&receipt))
}

type secureRuntimeUpdateState struct {
	Action           string    `json:"action"`
	ActivationID     string    `json:"activationId"`
	ActiveSHA256     string    `json:"activeSha256"`
	RollbackSHA256   string    `json:"rollbackSha256"`
	RollbackDeadline time.Time `json:"rollbackDeadline"`
	ActivatorPID     int       `json:"activatorPid"`
}

func secureRuntimeWaitForUpdateState(t *testing.T, action, activeSHA256, rollbackSHA256 string, timeout time.Duration) secureRuntimeUpdateState {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var (
		state   secureRuntimeUpdateState
		lastErr error
	)
	for time.Now().Before(deadline) {
		raw, err := os.ReadFile(secureRuntimeUpdateStatePath)
		if err == nil {
			err = json.Unmarshal(raw, &state)
		}
		lastErr = err
		if err == nil && state.Action == action && strings.EqualFold(state.ActiveSHA256, activeSHA256) && strings.EqualFold(state.RollbackSHA256, rollbackSHA256) {
			return state
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for helper update state action=%s active=%s rollback=%s; got=%+v err=%v", action, activeSHA256, rollbackSHA256, state, lastErr)
	return secureRuntimeUpdateState{}
}

func secureRuntimeWaitForFileAbsent(t *testing.T, path string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	if _, err := os.Lstat(path); err == nil {
		t.Fatalf("timed out waiting for %s to be removed", path)
	} else {
		t.Fatalf("inspect %s while waiting for removal: %v", path, err)
	}
}

func secureRuntimeWaitForReportAttempt(t *testing.T, fixture *secureRuntimeLabFixture, version, updatedFrom string, timeout time.Duration) secureRuntimeLabReport {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		for _, report := range fixture.attemptSnapshot() {
			if report.AgentVersion == version && report.UpdatedFrom == updatedFrom {
				return report
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for report attempt version=%q updatedFrom=%q", version, updatedFrom)
	return secureRuntimeLabReport{}
}

func secureRuntimeWaitForAcceptedVersion(t *testing.T, fixture *secureRuntimeLabFixture, version string, minimumCount int, timeout time.Duration) []secureRuntimeLabReport {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var matched []secureRuntimeLabReport
	for time.Now().Before(deadline) {
		reports, _, _, _ := fixture.snapshot()
		matched = matched[:0]
		for _, report := range reports {
			if report.AgentVersion == version {
				matched = append(matched, report)
			}
		}
		if len(matched) >= minimumCount {
			return append([]secureRuntimeLabReport(nil), matched...)
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d accepted reports from version %q; got %d", minimumCount, version, len(matched))
	return nil
}

func secureRuntimeCollectorMainPID(t *testing.T) int {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	stablePID := 0
	stableSince := time.Time{}
	lastPIDText := ""
	for time.Now().Before(deadline) {
		out, err := exec.Command("systemctl", "show", "pulse-agent.service", "--property=MainPID", "--value").Output()
		lastPIDText = strings.TrimSpace(string(out))
		pid, parseErr := strconv.Atoi(lastPIDText)
		if err == nil && parseErr == nil && pid > 1 {
			if _, statErr := os.Stat(fmt.Sprintf("/proc/%d/status", pid)); statErr == nil {
				if pid != stablePID {
					stablePID = pid
					stableSince = time.Now()
				} else if time.Since(stableSince) >= time.Second {
					return pid
				}
				time.Sleep(100 * time.Millisecond)
				continue
			}
		}
		stablePID = 0
		stableSince = time.Time{}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("pulse-agent MainPID did not remain positive and stable: last=%q", lastPIDText)
	return 0
}

func secureRuntimeProcessExecutableHash(t *testing.T, pid int) string {
	t.Helper()
	return secureRuntimeHash(secureRuntimeReadFile(t, fmt.Sprintf("/proc/%d/exe", pid)))
}

func secureRuntimeAssertUpdateBinaryIdentities(t *testing.T, targetSHA256, lastKnownGoodSHA256 string) {
	t.Helper()
	if got := secureRuntimeHash(secureRuntimeReadFile(t, "/usr/local/bin/pulse-agent")); !strings.EqualFold(got, targetSHA256) {
		t.Fatalf("installed collector hash = %s, want %s", got, targetSHA256)
	}
	if got := secureRuntimeHash(secureRuntimeReadFile(t, secureRuntimeUpdateLKGPath)); !strings.EqualFold(got, lastKnownGoodSHA256) {
		t.Fatalf("last-known-good collector hash = %s, want %s", got, lastKnownGoodSHA256)
	}
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

func secureRuntimeReadSignature(t *testing.T, envName string) string {
	t.Helper()
	path := strings.TrimSpace(os.Getenv(envName))
	if path == "" || !filepath.IsAbs(path) {
		t.Fatalf("%s must name an absolute caller-built signature path", envName)
	}
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("inspect %s: %v", envName, err)
	}
	if !info.Mode().IsRegular() {
		t.Fatalf("%s must be a regular signature file: %s (%s)", envName, path, info.Mode())
	}
	signature := strings.TrimSpace(string(secureRuntimeReadFile(t, path)))
	if signature == "" || len(signature) > 4096 || strings.ContainsAny(signature, "\x00\r\n") {
		t.Fatalf("%s contains an invalid detached signature", envName)
	}
	return signature
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

func secureRuntimeLoadSourceBoundary(t *testing.T, targetArch string) (secureRuntimeSourceManifestBinding, map[string]string) {
	t.Helper()
	repoRoot, err := filepath.Abs(repoFile())
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	manifestRelative := "scripts/release_control/secure_runtime_source_manifest_v6.json"
	manifestRaw := secureRuntimeReadFile(t, filepath.Join(repoRoot, filepath.FromSlash(manifestRelative)))
	var manifest secureRuntimeSourceManifest
	if err := json.Unmarshal(manifestRaw, &manifest); err != nil {
		t.Fatalf("decode secure-runtime source manifest: %v", err)
	}
	if manifest.SchemaVersion != 1 || manifest.ManifestID != "secure-runtime-linux-v6" || manifest.TargetOS != "linux" {
		t.Fatalf("unsupported secure-runtime source manifest: %+v", manifest)
	}
	sourceHashes := make(map[string]string)
	addSource := func(relative string) {
		if relative == "" || filepath.IsAbs(relative) || strings.Contains(relative, "..") || filepath.ToSlash(filepath.Clean(relative)) != relative {
			t.Fatalf("source manifest contains non-canonical path %q", relative)
		}
		sourceHashes[relative] = secureRuntimeHash(secureRuntimeReadFile(t, filepath.Join(repoRoot, filepath.FromSlash(relative))))
	}
	for _, relative := range manifest.ExactPaths {
		addSource(relative)
	}
	if _, ok := sourceHashes[manifestRelative]; !ok {
		t.Fatal("secure-runtime source manifest does not include itself")
	}
	for _, relativeRoot := range manifest.RecursiveRoots {
		root := filepath.Join(repoRoot, filepath.FromSlash(relativeRoot))
		matched := 0
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			relative, err := filepath.Rel(repoRoot, path)
			if err != nil {
				return err
			}
			relative = filepath.ToSlash(relative)
			included := false
			for _, suffix := range manifest.IncludeSuffixes {
				included = included || strings.HasSuffix(relative, suffix)
			}
			for _, suffix := range manifest.ExcludeSuffixes {
				if strings.HasSuffix(relative, suffix) {
					included = false
				}
			}
			if included {
				addSource(relative)
				matched++
			}
			return nil
		})
		if err != nil {
			t.Fatalf("expand secure-runtime source root %s: %v", relativeRoot, err)
		}
		if matched == 0 {
			t.Fatalf("secure-runtime source root %s matched no production source", relativeRoot)
		}
	}
	return secureRuntimeSourceManifestBinding{
		SchemaVersion: manifest.SchemaVersion,
		ManifestID:    manifest.ManifestID,
		Path:          manifestRelative,
		SHA256:        secureRuntimeHash(manifestRaw),
		TargetOS:      manifest.TargetOS,
		TargetArch:    targetArch,
	}, sourceHashes
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
	secureRuntimeRecordCommandOutput("installer "+strings.Join(scenarioArgs, " "), out)
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
	secureRuntimeRecordCommandOutput("standalone installer "+strings.Join(scenarioArgs, " "), out)
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
	secureRuntimeAssertNoCredentialExposure(t, out)
	secureRuntimeRecordCommandOutput(name, out)
	return strings.TrimSpace(string(out))
}

func secureRuntimeSystemdProperty(t *testing.T, property string) string {
	t.Helper()
	return secureRuntimeUnitProperty(t, "pulse-agent.service", property)
}

func secureRuntimeActionRunnerSystemdProperty(t *testing.T, property string) string {
	t.Helper()
	return secureRuntimeUnitProperty(t, "pulse-agent-runner.service", property)
}

func secureRuntimeUnitProperty(t *testing.T, unit, property string) string {
	t.Helper()
	return secureRuntimeCommand(t, 10*time.Second, "systemctl", "show", unit, "--property="+property, "--value")
}

func secureRuntimeReadUnitProperty(unit, property string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "systemctl", "show", unit, "--property="+property, "--value").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("systemctl show %s %s: %w: %s", unit, property, err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

func secureRuntimeCheckUnitProperty(unit, property, expected string) error {
	actual, err := secureRuntimeReadUnitProperty(unit, property)
	if err != nil {
		return err
	}
	if actual != expected {
		return fmt.Errorf("%s effective %s = %q, want %q", unit, property, actual, expected)
	}
	return nil
}

func secureRuntimeCheckUnitWordSet(unit, property string, expected ...string) error {
	actual, err := secureRuntimeReadUnitProperty(unit, property)
	if err != nil {
		return err
	}
	actualWords := strings.Fields(actual)
	expectedWords := append([]string(nil), expected...)
	sort.Strings(actualWords)
	sort.Strings(expectedWords)
	if strings.Join(actualWords, "\x00") != strings.Join(expectedWords, "\x00") {
		return fmt.Errorf("%s effective %s words = %q, want %q", unit, property, actualWords, expectedWords)
	}
	return nil
}

func secureRuntimeExecStartArgv(value string) []string {
	const marker = "argv[]="
	start := strings.Index(value, marker)
	if start < 0 {
		return nil
	}
	value = value[start+len(marker):]
	if end := strings.Index(value, " ;"); end >= 0 {
		value = value[:end]
	}
	return strings.Fields(strings.TrimSpace(value))
}

func secureRuntimeCheckExactExecStart(unit, expectedBinary string) error {
	actual, err := secureRuntimeReadUnitProperty(unit, "ExecStart")
	if err != nil {
		return err
	}
	argv := secureRuntimeExecStartArgv(actual)
	if len(argv) != 1 || argv[0] != expectedBinary {
		return fmt.Errorf("%s effective ExecStart argv = %q, want only %q", unit, argv, expectedBinary)
	}
	return nil
}

func secureRuntimeCheckInstallerUnitBoundary(unit, expectedFragment string) error {
	if err := secureRuntimeCheckUnitProperty(unit, "FragmentPath", expectedFragment); err != nil {
		return err
	}
	return secureRuntimeCheckUnitProperty(unit, "DropInPaths", "")
}

func secureRuntimeCheckSafeProfileSystemd(includeRunner bool) error {
	for _, check := range []struct {
		unit, property, expected string
	}{
		{"pulse-agent.service", "FragmentPath", "/etc/systemd/system/pulse-agent.service"},
		{"pulse-agent.service", "DropInPaths", ""},
		{"pulse-agent.service", "User", "pulse-agent"},
		{"pulse-agent.service", "AmbientCapabilities", ""},
		{"pulse-agent.service", "UMask", "0077"},
		{"pulse-agent.service", "NoNewPrivileges", "yes"},
		{"pulse-agent.service", "PrivateTmp", "yes"},
		{"pulse-agent.service", "PrivateDevices", "no"},
		{"pulse-agent.service", "ProtectKernelTunables", "yes"},
		{"pulse-agent.service", "ProtectKernelModules", "yes"},
		{"pulse-agent.service", "ProtectControlGroups", "yes"},
		{"pulse-agent.service", "LockPersonality", "yes"},
		{"pulse-agent.service", "RestrictSUIDSGID", "yes"},
		{"pulse-agent.service", "SystemCallArchitectures", "native"},
		{"pulse-agent-helper.service", "FragmentPath", "/etc/systemd/system/pulse-agent-helper.service"},
		{"pulse-agent-helper.service", "DropInPaths", ""},
		{"pulse-agent-helper.service", "User", "root"},
		{"pulse-agent-helper.service", "Group", "root"},
		{"pulse-agent-helper.service", "AmbientCapabilities", ""},
		{"pulse-agent-helper.service", "UMask", "0077"},
		{"pulse-agent-helper.service", "NoNewPrivileges", "yes"},
		{"pulse-agent-helper.service", "PrivateTmp", "yes"},
		{"pulse-agent-helper.service", "PrivateDevices", "no"},
		{"pulse-agent-helper.service", "PrivateNetwork", "yes"},
		{"pulse-agent-helper.service", "ProtectSystem", "strict"},
		{"pulse-agent-helper.service", "ProtectHome", "yes"},
		{"pulse-agent-helper.service", "ProtectKernelTunables", "yes"},
		{"pulse-agent-helper.service", "ProtectKernelModules", "yes"},
		{"pulse-agent-helper.service", "ProtectControlGroups", "yes"},
		{"pulse-agent-helper.service", "LockPersonality", "yes"},
		{"pulse-agent-helper.service", "RestrictSUIDSGID", "yes"},
		{"pulse-agent-helper.service", "SystemCallArchitectures", "native"},
		{"pulse-agent-helper.service", "TasksMax", "64"},
		{"pulse-agent-helper.service", "LimitNOFILE", "256"},
		{"pulse-agent-helper.service", "MemoryMax", "268435456"},
		{"pulse-agent-helper.socket", "FragmentPath", "/etc/systemd/system/pulse-agent-helper.socket"},
		{"pulse-agent-helper.socket", "DropInPaths", ""},
		{"pulse-agent-helper.socket", "SocketUser", "root"},
		{"pulse-agent-helper.socket", "SocketGroup", "pulse-agent"},
		{"pulse-agent-helper.socket", "SocketMode", "0660"},
		{"pulse-agent-helper.socket", "DirectoryMode", "0755"},
		{"pulse-agent-helper.socket", "RemoveOnStop", "yes"},
	} {
		if err := secureRuntimeCheckUnitProperty(check.unit, check.property, check.expected); err != nil {
			return err
		}
	}
	collectorExec, err := secureRuntimeReadUnitProperty("pulse-agent.service", "ExecStart")
	if err != nil {
		return err
	}
	collectorArgv := secureRuntimeExecStartArgv(collectorExec)
	if len(collectorArgv) == 0 || collectorArgv[0] != "/usr/local/bin/pulse-agent" {
		return fmt.Errorf("pulse-agent.service effective ExecStart argv = %q", collectorArgv)
	}
	for _, argument := range collectorArgv[1:] {
		if argument == "--enable-commands" {
			return errors.New("pulse-agent.service effective ExecStart retained --enable-commands")
		}
	}
	collectorEnvironment, err := secureRuntimeReadUnitProperty("pulse-agent.service", "Environment")
	if err != nil {
		return err
	}
	if !strings.Contains(collectorEnvironment, "PULSE_AGENT_HELPER_SOCKET=/run/pulse-agent/helper.sock") {
		return fmt.Errorf("pulse-agent.service effective Environment lacks the typed-helper socket: %q", collectorEnvironment)
	}
	if err := secureRuntimeCheckExactExecStart("pulse-agent-helper.service", "/usr/local/lib/pulse-agent/pulse-agent-helper"); err != nil {
		return err
	}
	if err := secureRuntimeCheckUnitWordSet("pulse-agent-helper.service", "RestrictAddressFamilies", "AF_UNIX"); err != nil {
		return err
	}
	listen, err := secureRuntimeReadUnitProperty("pulse-agent-helper.socket", "Listen")
	if err != nil {
		return err
	}
	if !strings.Contains(listen, "/run/pulse-agent/helper.sock") || !strings.Contains(listen, "Stream") {
		return fmt.Errorf("pulse-agent-helper.socket effective Listen = %q", listen)
	}
	if !includeRunner {
		return nil
	}
	for _, check := range []struct {
		property, expected string
	}{
		{"FragmentPath", "/etc/systemd/system/pulse-agent-runner.service"},
		{"DropInPaths", ""},
		{"User", "root"},
		{"Group", "root"},
		{"AmbientCapabilities", ""},
		{"UMask", "0077"},
		{"NoNewPrivileges", "yes"},
		{"PrivateTmp", "yes"},
		{"PrivateDevices", "no"},
		{"PrivateNetwork", "no"},
		{"ProtectHome", "yes"},
		{"ProtectSystem", "no"},
		{"ProtectKernelTunables", "yes"},
		{"ProtectKernelModules", "yes"},
		{"ProtectControlGroups", "yes"},
		{"LockPersonality", "yes"},
		{"RestrictSUIDSGID", "yes"},
		{"SystemCallArchitectures", "native"},
	} {
		if err := secureRuntimeCheckUnitProperty("pulse-agent-runner.service", check.property, check.expected); err != nil {
			return err
		}
	}
	if err := secureRuntimeCheckExactExecStart("pulse-agent-runner.service", "/usr/local/lib/pulse-agent/pulse-agent-runner"); err != nil {
		return err
	}
	if err := secureRuntimeCheckUnitWordSet("pulse-agent-runner.service", "RestrictAddressFamilies", "AF_UNIX", "AF_INET", "AF_INET6"); err != nil {
		return err
	}
	environmentFiles, err := secureRuntimeReadUnitProperty("pulse-agent-runner.service", "EnvironmentFiles")
	if err != nil {
		return err
	}
	if environmentFiles != "/etc/pulse-agent-runner/runner.env (ignore_errors=no)" {
		return fmt.Errorf("pulse-agent-runner.service effective EnvironmentFiles = %q", environmentFiles)
	}
	return nil
}

func secureRuntimeAssertSafeProfileSystemd(t *testing.T, includeRunner bool) {
	t.Helper()
	if err := secureRuntimeCheckSafeProfileSystemd(includeRunner); err != nil {
		t.Fatal(err)
	}
}

func secureRuntimeAssertActionRunnerInstalled(t *testing.T) {
	t.Helper()
	secureRuntimeCommand(t, 10*time.Second, "systemctl", "is-active", "pulse-agent-runner.service")
	secureRuntimeAssertSafeProfileSystemd(t, true)
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
	pid := secureRuntimeCollectorMainPID(t)
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
	secureRuntimeAssertSafeProfileSystemd(t, false)
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

func secureRuntimeExerciseUnitOverrideDetection(t *testing.T, unit, section, directive string, includeRunner bool) string {
	t.Helper()
	dropInDir := filepath.Join("/etc/systemd/system", unit+".d")
	dropInPath := filepath.Join(dropInDir, "secure-runtime-adversarial.conf")
	if err := os.MkdirAll(dropInDir, 0o755); err != nil {
		t.Fatalf("create %s drop-in directory: %v", unit, err)
	}
	if err := os.WriteFile(dropInPath, []byte("["+section+"]\n"+directive+"\n"), 0o644); err != nil {
		t.Fatalf("write %s adversarial drop-in: %v", unit, err)
	}
	removed := false
	t.Cleanup(func() {
		if !removed {
			_ = os.Remove(dropInPath)
			_ = os.Remove(dropInDir)
			_ = exec.Command("systemctl", "daemon-reload").Run()
		}
	})
	secureRuntimeCommand(t, 20*time.Second, "systemctl", "daemon-reload")
	err := secureRuntimeCheckSafeProfileSystemd(includeRunner)
	if err == nil {
		t.Fatalf("effective systemd validation accepted %s drop-in %q", unit, directive)
	}
	dropIns, propertyErr := secureRuntimeReadUnitProperty(unit, "DropInPaths")
	if propertyErr != nil {
		t.Fatal(propertyErr)
	}
	if !strings.Contains(dropIns, dropInPath) {
		t.Fatalf("%s effective DropInPaths = %q, want %s", unit, dropIns, dropInPath)
	}
	if err := os.Remove(dropInPath); err != nil {
		t.Fatalf("remove %s adversarial drop-in: %v", unit, err)
	}
	_ = os.Remove(dropInDir)
	removed = true
	secureRuntimeCommand(t, 20*time.Second, "systemctl", "daemon-reload")
	secureRuntimeAssertSafeProfileSystemd(t, includeRunner)
	return err.Error()
}

func secureRuntimeNonLoopbackIPv4(t *testing.T) string {
	t.Helper()
	interfaces, err := net.Interfaces()
	if err != nil {
		t.Fatalf("list host interfaces: %v", err)
	}
	for _, networkInterface := range interfaces {
		if networkInterface.Flags&net.FlagUp == 0 || networkInterface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addresses, err := networkInterface.Addrs()
		if err != nil {
			continue
		}
		for _, address := range addresses {
			var ip net.IP
			switch typed := address.(type) {
			case *net.IPNet:
				ip = typed.IP
			case *net.IPAddr:
				ip = typed.IP
			}
			if ipv4 := ip.To4(); ipv4 != nil && !ipv4.IsLoopback() {
				return ipv4.String()
			}
		}
	}
	t.Fatal("disposable systemd host has no non-loopback IPv4 address for the helper network canary")
	return ""
}

func secureRuntimeAssertHelperOutboundNetworkDenied(t *testing.T) map[string]any {
	t.Helper()
	secureRuntimeAssertHelperProtocol(t)
	hostIP := secureRuntimeNonLoopbackIPv4(t)
	listener, err := net.Listen("tcp4", "0.0.0.0:0")
	if err != nil {
		t.Fatalf("listen for helper network-isolation canary: %v", err)
	}
	server := &http.Server{Handler: http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusNoContent)
	})}
	serveDone := make(chan error, 1)
	go func() { serveDone <- server.Serve(listener) }()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
		<-serveDone
	})
	port := listener.Addr().(*net.TCPAddr).Port
	canaryURL := fmt.Sprintf("http://%s/secure-runtime-network-canary", net.JoinHostPort(hostIP, strconv.Itoa(port)))
	hostClient := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			Proxy: nil,
		},
	}
	response, err := hostClient.Get(canaryURL)
	if err != nil {
		t.Fatalf("host network could not reach its canary %s: %v", canaryURL, err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("host network canary status = %s", response.Status)
	}
	mainPID := secureRuntimeUnitProperty(t, "pulse-agent-helper.service", "MainPID")
	if mainPID == "" || mainPID == "0" {
		t.Fatalf("typed helper has no live MainPID: %q", mainPID)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx,
		"nsenter", "--target", mainPID, "--net", "--",
		"curl", "--noproxy", "*", "-fsS", "--connect-timeout", "2", "--max-time", "3", canaryURL,
	).CombinedOutput()
	secureRuntimeAssertNoCredentialExposure(t, out)
	secureRuntimeRecordCommandOutput("helper namespace outbound TCP canary", out)
	if err == nil {
		t.Fatalf("helper network namespace reached host-interface canary %s", canaryURL)
	}
	return map[string]any{
		"canary_scope":                "host-interface-tcp",
		"host_canary_reachable":       true,
		"helper_namespace_connection": "denied",
		"helper_main_pid":             mainPID,
	}
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

func secureRuntimeWriteEvidence(t *testing.T, receipt *secureRuntimeLabReceipt, events []secureRuntimeTranscriptEvent) {
	t.Helper()
	path := strings.TrimSpace(os.Getenv("PULSE_SECURE_RUNTIME_RECEIPT"))
	if path == "" {
		return
	}
	if !filepath.IsAbs(path) {
		t.Fatalf("PULSE_SECURE_RUNTIME_RECEIPT must be an absolute path: %s", path)
	}
	transcriptPath := strings.TrimSpace(os.Getenv("PULSE_SECURE_RUNTIME_TRANSCRIPT"))
	if transcriptPath == "" || !filepath.IsAbs(transcriptPath) {
		t.Fatalf("PULSE_SECURE_RUNTIME_TRANSCRIPT must be an absolute path: %s", transcriptPath)
	}
	transcriptRecordPath := strings.TrimSpace(os.Getenv("PULSE_SECURE_RUNTIME_TRANSCRIPT_RECORD_PATH"))
	if transcriptRecordPath == "" || filepath.IsAbs(transcriptRecordPath) || filepath.ToSlash(filepath.Clean(transcriptRecordPath)) != transcriptRecordPath || strings.Contains(transcriptRecordPath, "..") {
		t.Fatalf("PULSE_SECURE_RUNTIME_TRANSCRIPT_RECORD_PATH must be a canonical repository-relative path: %q", transcriptRecordPath)
	}
	var transcript bytes.Buffer
	for _, event := range events {
		encodedEvent, err := json.Marshal(event)
		if err != nil {
			t.Fatalf("marshal secure-runtime transcript event: %v", err)
		}
		if secureRuntimeReceiptContainsCredential(encodedEvent) {
			t.Fatal("refusing to write a transcript containing credential material or token-labelled fields")
		}
		transcript.Write(encodedEvent)
		transcript.WriteByte('\n')
	}
	transcriptBytes := transcript.Bytes()
	receipt.Transcript = secureRuntimeTranscriptBinding{
		Format: "jsonl-v1", RecordPath: transcriptRecordPath,
		SHA256: secureRuntimeHash(transcriptBytes), EventCount: len(events),
	}
	if err := secureRuntimeWritePrivateAtomic(transcriptPath, transcriptBytes); err != nil {
		t.Fatalf("publish secure-runtime transcript: %v", err)
	}

	encoded, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		t.Fatalf("marshal secure-runtime receipt: %v", err)
	}
	if secureRuntimeReceiptContainsCredential(encoded) {
		t.Fatal("refusing to write a receipt containing credential material or token-labelled fields")
	}
	encoded = append(encoded, '\n')
	if err := secureRuntimeWritePrivateAtomic(path, encoded); err != nil {
		t.Fatalf("publish receipt: %v", err)
	}
}

func secureRuntimeWritePrivateAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, data, 0o600); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}

func secureRuntimeReceiptContainsCredential(encoded []byte) bool {
	for _, credential := range [][]byte{
		[]byte(secureRuntimeLabToken),
		[]byte(secureRuntimeRunnerSecretV1),
		[]byte(secureRuntimeRunnerSecretV2),
	} {
		if bytes.Contains(encoded, credential) {
			return true
		}
	}
	var value any
	if err := json.Unmarshal(encoded, &value); err != nil {
		return true
	}
	return secureRuntimeContainsForbiddenReceiptKey(value)
}

func secureRuntimeContainsForbiddenReceiptKey(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if _, forbidden := secureRuntimeForbiddenReceiptKeys[strings.ToLower(strings.TrimSpace(key))]; forbidden {
				return true
			}
			if secureRuntimeContainsForbiddenReceiptKey(child) {
				return true
			}
		}
	case []any:
		for _, child := range typed {
			if secureRuntimeContainsForbiddenReceiptKey(child) {
				return true
			}
		}
	}
	return false
}
