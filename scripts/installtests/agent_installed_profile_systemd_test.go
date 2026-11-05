//go:build linux

package installtests

// This is a current-source installer regression, not release qualification.
// Only the dedicated fresh GitHub-hosted VM job opts in. It runs real systemd
// services against a local HTTPS server, using an ephemeral SSH signing key.
// Published signatures, appliances and original customer recovery are separate.

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"debug/buildinfo"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

const nativeProfileRoot = "/opt/pulse-agent-profile-native"

var nativeProfileUnits = []string{
	"pulse-agent-runner.service", "pulse-agent-helper.socket",
	"pulse-agent-helper.service", "pulse-agent.service",
}

// All these names are admitted absent before ownership is acquired. Cleanup
// never reaches a pre-existing installation or a different service name.
var nativeProfilePaths = []string{
	nativeProfileRoot, "/opt/pulse", "/etc/pulse", "/etc/pulse-agent",
	"/etc/pulse-agent-runner", "/usr/local/bin/pulse-agent",
	"/usr/local/bin/pulse-agent.last-known-good", "/usr/local/lib/pulse-agent",
	"/var/lib/pulse-agent", "/var/lib/pulse-agent-helper",
	"/var/lib/pulse-agent-runner", "/var/lib/pulse-agent-profile",
	"/run/pulse-agent", "/var/log/pulse-agent.log", "/etc/sudoers.d/pulse-agent",
}

func nativeProfileAdmission(env map[string]string, goos string, uid int, pid1 string, collisions []string) error {
	if goos != "linux" || uid != 0 || strings.TrimSpace(pid1) != "systemd" ||
		env["PULSE_NATIVE_INSTALLED_PROFILE"] != "1" || env["GITHUB_ACTIONS"] != "true" ||
		env["RUNNER_ENVIRONMENT"] != "github-hosted" || env["GITHUB_REPOSITORY"] != "rcourtman/Pulse" ||
		!regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(env["EXPECTED_SOURCE_SHA"]) || len(collisions) != 0 {
		return errors.New("installed/profile regression requires a fresh source-bound public hosted systemd VM")
	}
	return nil
}

func nativeProfileRootInstallArgs() []string {
	return []string{"--enable-commands", "--command-authority", "command-capable"}
}

func nativeProfileRunnerInstallArgs(token string) []string {
	return []string{"--least-privilege", "--enable-privileged-helper", "--enable-action-runner", "--action-token-file", token}
}

func nativeProfileUnitQuiescent(unit, active, mainPID string) bool {
	if active != "inactive" && active != "failed" {
		return false
	}
	switch {
	case strings.HasSuffix(unit, ".socket"):
		// Socket units have no MainPID property; their owning service is
		// independently stopped and checked in the same batch.
		return mainPID == ""
	case strings.HasSuffix(unit, ".service"):
		return mainPID == "0"
	default:
		return false
	}
}

type nativeProfilePhase struct {
	Name     string `json:"name"`
	At       string `json:"observed_at"`
	Evidence any    `json:"evidence"`
}

type nativeProfileReceipt struct {
	Schema              int                  `json:"schema_version"`
	Source              string               `json:"source_sha"`
	Scope               string               `json:"scope"`
	InstallerHash       string               `json:"installer_template_sha256"`
	PinnedInstallerHash string               `json:"pinned_installer_sha256"`
	Artifacts           map[string]string    `json:"artifact_sha256"`
	Phases              []nativeProfilePhase `json:"phases"`
	Commands            []map[string]any     `json:"commands"`
	Passed              bool                 `json:"passed"`
	Cleanup             map[string]any       `json:"cleanup"`
}

// command retains an observed exit, never calls a timed-out command again, and
// kills/joins its owned process group before reporting a deadline failure.
func nativeProfileCommand(t *testing.T, receipt *nativeProfileReceipt, executable string, args ...string) ([]byte, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, executable, args...)
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.Cancel = func() error { return syscall.Kill(-command.Process.Pid, syscall.SIGKILL) }
	command.WaitDelay = 3 * time.Second
	output, err := command.CombinedOutput()
	secureRuntimeAssertNoCredentialExposure(t, output)
	exit := -1
	if command.ProcessState != nil {
		exit = command.ProcessState.ExitCode()
	}
	if receipt != nil {
		receipt.Commands = append(receipt.Commands, map[string]any{
			"executable": filepath.Base(executable), "exit": exit, "deadline": ctx.Err() != nil,
			"command_error": err != nil, "wait_delay_exceeded": errors.Is(err, exec.ErrWaitDelay),
			"output_bytes": len(output), "output_sha256": secureRuntimeHash(output),
		})
	}
	return output, err
}

func nativeProfileMust(t *testing.T, receipt *nativeProfileReceipt, executable string, args ...string) []byte {
	t.Helper()
	output, err := nativeProfileCommand(t, receipt, executable, args...)
	if err != nil {
		t.Fatalf("native %s failed: %v\n%s", filepath.Base(executable), err, output)
	}
	return output
}

func nativeProfileFiles(t *testing.T, paths ...string) secureRuntimeStableIdentity {
	t.Helper()
	result := make(secureRuntimeStableIdentity)
	for _, path := range paths {
		info, err := os.Lstat(path)
		if err != nil {
			t.Fatal(err)
		}
		if !info.Mode().IsRegular() {
			t.Fatalf("snapshot is not a regular file: %s", path)
		}
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok {
			t.Fatalf("snapshot ownership is unavailable: %s", path)
		}
		result[path] = secureRuntimeFileIdentity{Present: true, Mode: info.Mode().Perm(), UID: stat.Uid, GID: stat.Gid,
			Hash: secureRuntimeHash(secureRuntimeReadFile(t, path))}
	}
	return result
}

func nativeProfileDirectory(t *testing.T, path string) secureRuntimeFileIdentity {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() {
		t.Fatalf("snapshot is not a directory: %s", path)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		t.Fatalf("snapshot ownership is unavailable: %s", path)
	}
	return secureRuntimeFileIdentity{Present: true, Mode: info.Mode().Perm(), UID: stat.Uid, GID: stat.Gid}
}

func nativeProfileOptionalFiles(t *testing.T, paths ...string) secureRuntimeStableIdentity {
	t.Helper()
	result := make(secureRuntimeStableIdentity)
	for _, path := range paths {
		if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
			result[path] = secureRuntimeFileIdentity{}
		} else {
			result[path] = nativeProfileFiles(t, path)[path]
		}
	}
	return result
}

// Mutable descendants are handed to the collector, not restored recursively
// to root on rollback. Still bind their bytes, mode and exact service owner;
// protected identity/connection files and the state root are checked separately.
func nativeProfileHistoryHandedOff(before, after secureRuntimeFileIdentity, uid, gid uint32) bool {
	if !before.Present || before.Hash == "" || uid == 0 || gid == 0 {
		return false
	}
	want := before
	want.UID, want.GID = uid, gid
	return after == want
}

func nativeProfileState(t *testing.T, receipt *nativeProfileReceipt) map[string]string {
	t.Helper()
	result := make(map[string]string)
	for _, unit := range nativeProfileUnits {
		result[unit] = strings.TrimSpace(string(nativeProfileMust(t, receipt, "systemctl", "show", unit,
			"--property=LoadState,ActiveState,UnitFileState,MainPID")))
	}
	return result
}

func nativeProfileServiceFlags(t *testing.T, receipt *nativeProfileReceipt, active, enabled bool) {
	t.Helper()
	activity, enablement := "stop", "disable"
	if active {
		activity = "start"
	}
	if enabled {
		enablement = "enable"
	}
	nativeProfileMust(t, receipt, "systemctl", activity, "pulse-agent.service")
	nativeProfileMust(t, receipt, "systemctl", enablement, "pulse-agent.service")
}

func nativeProfileAssertFlags(t *testing.T, receipt *nativeProfileReceipt, active, enabled bool) {
	t.Helper()
	gotActive := strings.TrimSpace(string(nativeProfileMust(t, receipt, "systemctl", "show", "pulse-agent.service", "--property=ActiveState", "--value")))
	gotEnabled := strings.TrimSpace(string(nativeProfileMust(t, receipt, "systemctl", "show", "pulse-agent.service", "--property=UnitFileState", "--value")))
	wantActive, wantEnabled := "inactive", "disabled"
	if active {
		wantActive = "active"
	}
	if enabled {
		wantEnabled = "enabled"
	}
	if gotActive != wantActive || gotEnabled != wantEnabled {
		t.Fatalf("restored intent = %s/%s, want %s/%s", gotActive, gotEnabled, wantActive, wantEnabled)
	}
}

func nativeProfileArtifact(t *testing.T, dir, name, source, mainPath string) []byte {
	t.Helper()
	path := filepath.Join(dir, name)
	file, err := os.Lstat(path)
	if err != nil || !file.Mode().IsRegular() || file.Mode()&0o111 == 0 {
		t.Fatalf("native input is not a regular executable: %s", name)
	}
	info, err := buildinfo.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	settings := make(map[string]string)
	for _, setting := range info.Settings {
		settings[setting.Key] = setting.Value
	}
	if info.Path != mainPath || settings["vcs.revision"] != source || settings["vcs.modified"] != "false" || settings["GOOS"] != "linux" || settings["GOARCH"] != runtime.GOARCH {
		t.Fatalf("native artifact %s does not match the clean assigned source/platform", name)
	}
	return secureRuntimeReadFile(t, path)
}

type nativeProfileDownloads struct {
	sync.Mutex
	signatures map[string]string
	badHelper  bool
}

func (d *nativeProfileDownloads) wrap(fixture *secureRuntimeLabFixture) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		d.Lock()
		signature := d.signatures[r.URL.Path]
		if d.badHelper && r.URL.Path == "/download/pulse-agent-helper" {
			signature = d.signatures["/download/pulse-agent"]
		}
		d.Unlock()
		if signature != "" {
			w.Header().Set("X-Signature-SSHSIG", signature)
		}
		fixture.ServeHTTP(w, r)
	})
}

func nativeProfileSign(t *testing.T, key, path string) string {
	t.Helper()
	nativeProfileMust(t, nil, "ssh-keygen", "-q", "-Y", "sign", "-f", key, "-n", "pulse-install", path)
	return base64.StdEncoding.EncodeToString(secureRuntimeReadFile(t, path+".sig"))
}

func nativeProfilePin(template []byte, publicKey string) (string, error) {
	const slot = `PINNED_INSTALLER_SSH_PUBLIC_KEY="__PULSE_INSTALLER_SSH_PUBLIC_KEY__"`
	if strings.Count(string(template), slot) != 1 {
		return "", errors.New("installer signature pin template changed")
	}
	// Leave has_pinned_installer_signature_key's sentinel comparison intact.
	return strings.Replace(string(template), slot, `PINNED_INSTALLER_SSH_PUBLIC_KEY="`+publicKey+`"`, 1), nil
}

func nativeProfileUntrustedCA(t *testing.T) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	certificate := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Unrelated fixture CA"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IsCA: true,
		BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	der, err := x509.CreateCertificate(rand.Reader, certificate, certificate, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func TestAgentInstalledProfileSystemd(t *testing.T) {
	if os.Getenv("PULSE_NATIVE_INSTALLED_PROFILE") != "1" {
		t.Skip("dedicated fresh public hosted VM only; not an installed acceptance result")
	}
	env := make(map[string]string)
	for _, name := range []string{"PULSE_NATIVE_INSTALLED_PROFILE", "GITHUB_ACTIONS", "RUNNER_ENVIRONMENT", "GITHUB_REPOSITORY", "EXPECTED_SOURCE_SHA"} {
		env[name] = os.Getenv(name)
	}
	pid1 := secureRuntimeReadFile(t, "/proc/1/comm")
	var collisions []string
	for _, path := range append(append([]string{}, nativeProfilePaths...), "/.dockerenv") {
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			collisions = append(collisions, path)
		}
	}
	if err := nativeProfileAdmission(env, runtime.GOOS, os.Geteuid(), string(pid1), collisions); err != nil {
		t.Fatal(err)
	}
	for _, unit := range nativeProfileUnits {
		load := strings.TrimSpace(string(nativeProfileMust(t, nil, "systemctl", "show", unit, "--property=LoadState", "--value")))
		if load != "not-found" {
			t.Fatalf("refusing pre-existing unit %s", unit)
		}
		for _, path := range []string{filepath.Join("/etc/systemd/system", unit), filepath.Join("/etc/systemd/system", unit+".d")} {
			if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("refusing pre-existing unit file %s", path)
			}
		}
	}
	for _, database := range []string{"passwd", "group"} {
		_, err := nativeProfileCommand(t, nil, "getent", database, "pulse-agent")
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 2 {
			t.Fatalf("cannot confirm absent pulse-agent %s entry", database)
		}
	}
	source := env["EXPECTED_SOURCE_SHA"]
	// The ordinary checkout owner verifies HEAD and cleanliness immediately
	// before sudo. Root does not alter Git trust/configuration to read a
	// checkout owned by a different user. Every supplied application binary is
	// also independently bound to this revision by its clean VCS build record.
	dir := os.Getenv("PULSE_NATIVE_PROFILE_INPUTS")
	if !filepath.IsAbs(dir) {
		t.Fatal("native source-built inputs directory must be absolute")
	}
	artifacts := make(map[string][]byte)
	for name, mainPath := range map[string]string{"collector-v1": "cmd/pulse-agent", "collector-v2": "cmd/pulse-agent", "helper": "cmd/pulse-agent-helper", "runner": "cmd/pulse-agent-runner"} {
		artifacts[name] = nativeProfileArtifact(t, dir, name, source, "github.com/rcourtman/pulse-go-rewrite/"+mainPath)
	}
	if secureRuntimeHash(artifacts["collector-v1"]) == secureRuntimeHash(artifacts["collector-v2"]) {
		t.Fatal("native forward/reverse requires different executable bytes")
	}
	versions := make(map[string]string)
	for _, name := range []string{"collector-v1", "collector-v2"} {
		versions[name] = strings.TrimSpace(string(nativeProfileMust(t, nil, filepath.Join(dir, name), "--version")))
	}
	if versions["collector-v1"] == versions["collector-v2"] {
		t.Fatal("native inputs have indistinguishable versions")
	}
	keyDir := t.TempDir()
	key := filepath.Join(keyDir, "fixture-key")
	nativeProfileMust(t, nil, "ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", key)
	publicKey := strings.TrimSpace(string(secureRuntimeReadFile(t, key+".pub")))
	signatures := make(map[string]string)
	for name := range artifacts {
		path := filepath.Join(keyDir, name)
		if err := os.WriteFile(path, artifacts[name], 0o600); err != nil {
			t.Fatal(err)
		}
		signatures[name] = nativeProfileSign(t, key, path)
	}
	template := secureRuntimeReadFile(t, repoFile("scripts", "install.sh"))
	// Use the existing release/server templating slot, not a verifier bypass.
	installer, err := nativeProfilePin(template, publicKey)
	if err != nil {
		t.Fatal(err)
	}
	installerPath := filepath.Join(keyDir, "install.sh")
	if err := os.WriteFile(installerPath, []byte(installer), 0o700); err != nil {
		t.Fatal(err)
	}
	fixture := newSecureRuntimeLabFixture(artifacts["collector-v1"], "", artifacts["helper"], artifacts["runner"], versions["collector-v1"])
	t.Cleanup(fixture.actionServer.Shutdown)
	downloads := &nativeProfileDownloads{signatures: map[string]string{
		"/download/pulse-agent": signatures["collector-v1"], "/download/pulse-agent-helper": signatures["helper"], "/download/pulse-agent-runner": signatures["runner"],
	}}
	server := httptest.NewTLSServer(downloads.wrap(fixture))
	t.Cleanup(server.Close)
	receipt := &nativeProfileReceipt{Schema: 1, Source: source, Scope: "current-source-signed-installer-verified-TLS-native-systemd-not-published-release",
		InstallerHash: secureRuntimeHash(template), PinnedInstallerHash: secureRuntimeHash([]byte(installer)), Artifacts: make(map[string]string), Cleanup: map[string]any{"complete": false}}
	for name, artifact := range artifacts {
		receipt.Artifacts[name] = secureRuntimeHash(artifact)
	}
	phase := func(name string, evidence any) {
		receipt.Phases = append(receipt.Phases, nativeProfilePhase{name, time.Now().UTC().Format(time.RFC3339Nano), evidence})
	}
	// From this point every fixed install path was observed absent and is owned
	// solely by this lab. Failed stop or remaining process prevents deletion.
	if err := os.Mkdir(nativeProfileRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { nativeProfileCleanup(t, receipt) })
	caPath := filepath.Join(nativeProfileRoot, "ca.pem")
	if err := os.WriteFile(caPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}), 0o644); err != nil {
		t.Fatal(err)
	}
	token := filepath.Join(keyDir, "monitoring-token")
	runnerToken := filepath.Join(keyDir, "runner-token")
	for path, value := range map[string]string{token: secureRuntimeLabToken, runnerToken: secureRuntimeRunnerSecretV1} {
		if err := os.WriteFile(path, []byte(value+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	install := func(ca string, args ...string) ([]byte, error) {
		commandArgs := []string{installerPath, "--url", server.URL, "--token-file", token, "--cacert", ca,
			"--interval", "2s", "--agent-id", secureRuntimeLabAgentID, "--hostname", secureRuntimeLabHostname, "--non-interactive"}
		return nativeProfileCommand(t, receipt, "bash", append(commandArgs, args...)...)
	}
	if output, err := install(caPath, nativeProfileRootInstallArgs()...); err != nil {
		t.Fatalf("initial signed TLS install: %v\n%s", err, output)
	}
	secureRuntimeWaitForReports(t, fixture, 1, 45*time.Second)
	secureRuntimeAssertRootCommandProfile(t)
	historyPath := "/var/lib/pulse-agent/native-history"
	if err := os.WriteFile(historyPath, []byte("existing-history-must-survive\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	statePaths := []string{caPath, "/var/lib/pulse-agent/agent-id", "/var/lib/pulse-agent/connection.env",
		"/var/lib/pulse-agent/token"}
	baselineState := nativeProfileFiles(t, statePaths...)
	// Protected lifecycle files can be absent in the original legacy profile;
	// rollback must restore that absence too, not just compare present files.
	lifecyclePaths := []string{"/etc/pulse-agent/connection.env", "/etc/pulse-agent/install.sh", "/etc/pulse-agent/install.sh.sha256"}
	baselineLifecycle := nativeProfileOptionalFiles(t, lifecyclePaths...)
	baselineRoot := nativeProfileDirectory(t, "/var/lib/pulse-agent")
	baselineHistory := nativeProfileFiles(t, historyPath)[historyPath]
	phase("signed-TLS-root-install", map[string]any{"units": nativeProfileState(t, receipt), "state": baselineState, "lifecycle": baselineLifecycle, "state_root": baselineRoot, "history": baselineHistory})
	// The installer correctly forbids a legacy command collector and a new
	// action runner in one invocation. Establish the independent runner only
	// under its supported safe profile, then use the real saved rollback to
	// recover the legacy collector (without resurrecting command authority).
	// All four measured forward/reverse pairs below start with this runner.
	if output, err := install(caPath, "--safe-profile-apply"); err != nil {
		t.Fatalf("runner setup safe-profile apply: %v\n%s", err, output)
	}
	secureRuntimeAssertSafeProfile(t)
	collectorOwner := func(flag string) uint32 {
		id := strings.TrimSpace(string(nativeProfileMust(t, receipt, "id", flag, "pulse-agent")))
		value, err := strconv.ParseUint(id, 10, 32)
		if err != nil || value == 0 {
			t.Fatal("dedicated collector ownership is unavailable or root")
		}
		return uint32(value)
	}
	collectorUID, collectorGID := collectorOwner("-u"), collectorOwner("-g")
	assertHistory := func() secureRuntimeFileIdentity {
		history := nativeProfileFiles(t, historyPath)[historyPath]
		if !nativeProfileHistoryHandedOff(baselineHistory, history, collectorUID, collectorGID) {
			t.Fatal("mutable history lost bytes/mode or its dedicated collector ownership")
		}
		return history
	}
	assertRestoredState := func() {
		if !reflect.DeepEqual(baselineState, nativeProfileFiles(t, statePaths...)) ||
			!reflect.DeepEqual(baselineLifecycle, nativeProfileOptionalFiles(t, lifecyclePaths...)) ||
			baselineRoot != nativeProfileDirectory(t, "/var/lib/pulse-agent") {
			t.Fatal("reverse changed protected CA/identity/lifecycle files or state-root metadata")
		}
		assertHistory()
	}
	phase("runner-setup-safe-profile", map[string]any{"history": assertHistory(), "state_root": nativeProfileDirectory(t, "/var/lib/pulse-agent")})
	if output, err := install(caPath, nativeProfileRunnerInstallArgs(runnerToken)...); err != nil {
		t.Fatalf("separate signed TLS runner install: %v\n%s", err, output)
	}
	secureRuntimeWaitForActionRunner(t, fixture, true, 30*time.Second)
	secureRuntimeAssertActionRunnerInstalled(t)
	reportsBeforeReverse, _, _, _ := fixture.snapshot()
	nativeProfileMust(t, receipt, "bash", "/etc/pulse-agent/install.sh", "--safe-profile-rollback")
	secureRuntimeWaitForReports(t, fixture, len(reportsBeforeReverse)+1, 45*time.Second)
	secureRuntimeAssertRootMonitoringProfile(t)
	assertRestoredState()
	runnerPaths := []string{"/usr/local/lib/pulse-agent/pulse-agent-runner", "/etc/systemd/system/pulse-agent-runner.service", "/etc/pulse-agent-runner/runner.env", "/etc/pulse-agent-runner/token"}
	runnerState := nativeProfileFiles(t, runnerPaths...)
	runnerPID := strings.TrimSpace(string(nativeProfileMust(t, receipt, "systemctl", "show", "pulse-agent-runner.service", "--property=MainPID", "--value")))
	if runnerPID == "0" || runnerPID == "" {
		t.Fatal("independent action runner has no process")
	}
	runnerPIDNumber, err := strconv.Atoi(runnerPID)
	if err != nil || secureRuntimeProcessExecutableHash(t, runnerPIDNumber) != receipt.Artifacts["runner"] {
		t.Fatal("running action runner is not the signed input")
	}
	checkRunner := func() {
		if !reflect.DeepEqual(runnerState, nativeProfileFiles(t, runnerPaths...)) {
			t.Fatal("collector migration changed the independent action runner files")
		}
		pid := strings.TrimSpace(string(nativeProfileMust(t, receipt, "systemctl", "show", "pulse-agent-runner.service", "--property=MainPID", "--value")))
		if pid != runnerPID {
			t.Fatal("collector migration restarted the independent action runner")
		}
		secureRuntimeWaitForActionRunner(t, fixture, true, 5*time.Second)
	}
	assertProcess := func(want string) {
		if got := secureRuntimeProcessExecutableHash(t, secureRuntimeCollectorMainPID(t)); got != receipt.Artifacts[want] {
			t.Fatalf("running collector digest is not %s", want)
		}
		if got := secureRuntimeHash(secureRuntimeReadFile(t, "/usr/local/bin/pulse-agent")); got != receipt.Artifacts[want] {
			t.Fatalf("installed collector digest is not %s", want)
		}
	}
	assertSavedTrust := func() {
		environment := strings.TrimSpace(string(nativeProfileMust(t, receipt, "systemctl", "show", "pulse-agent.service", "--property=Environment", "--value")))
		if !strings.Contains(environment, "SSL_CERT_FILE="+caPath) || secureRuntimeCollectorHasArgument("--insecure") {
			t.Fatal("installed collector lost its CA or disabled TLS verification")
		}
		if !reflect.DeepEqual(baselineState[caPath], nativeProfileFiles(t, caPath)[caPath]) {
			t.Fatal("saved CA bytes/metadata changed")
		}
	}
	waitReport := func(version string, after time.Time, safe bool) secureRuntimeLabReport {
		deadline := time.Now().Add(45 * time.Second)
		for time.Now().Before(deadline) {
			reports, _, _, _ := fixture.snapshot()
			for _, report := range reports {
				if !report.ReceivedAt.After(after) || report.AgentVersion != version || report.CommandsEnabled {
					continue
				}
				if safe && !report.RunningAsRoot && report.ServiceUser == "pulse-agent" && report.Authority == "monitoring-only" && report.TypedHelper {
					return report
				}
				if !safe && report.RunningAsRoot {
					return report
				}
			}
			time.Sleep(200 * time.Millisecond)
		}
		t.Fatal("no fresh authenticated HTTPS report from the selected executable/profile")
		return secureRuntimeLabReport{}
	}
	assertProcess("collector-v1")
	assertSavedTrust()
	phase("independent-runner-setup", map[string]any{"units": nativeProfileState(t, receipt), "state": nativeProfileFiles(t, statePaths...), "lifecycle": nativeProfileOptionalFiles(t, lifecyclePaths...), "state_root": nativeProfileDirectory(t, "/var/lib/pulse-agent"), "history": assertHistory(), "runner": runnerState, "runner_pid": runnerPID})
	// Wrong local CA and a correctly checksummed helper with another artifact's
	// valid signature must fail before stopping services or replacing files.
	wrongCAPath := filepath.Join(keyDir, "untrusted-ca.pem")
	if err := os.WriteFile(wrongCAPath, nativeProfileUntrustedCA(t), 0o600); err != nil {
		t.Fatal(err)
	}
	beforeRefusal := secureRuntimeStableSnapshot(t)
	beforeUnits := nativeProfileState(t, receipt)
	if _, err := install(wrongCAPath, "--safe-profile-apply"); err == nil {
		t.Fatal("installer accepted an invalid CA")
	}
	if !reflect.DeepEqual(beforeRefusal, secureRuntimeStableSnapshot(t)) || !reflect.DeepEqual(beforeUnits, nativeProfileState(t, receipt)) {
		t.Fatal("TLS refusal changed installed state")
	}
	phase("TLS-refusal", map[string]any{"files_unchanged": true, "units_unchanged": true})
	fixture.setCollector(artifacts["collector-v2"], "")
	fixture.setServerVersion(versions["collector-v2"])
	downloads.Lock()
	downloads.signatures["/download/pulse-agent"] = signatures["collector-v2"]
	downloads.badHelper = true
	downloads.Unlock()
	if output, err := install(caPath, "--safe-profile-apply"); err == nil || !strings.Contains(string(output), "signature verification failed") {
		t.Fatalf("mismatched helper signature not refused: %v\n%s", err, output)
	}
	if !reflect.DeepEqual(beforeRefusal, secureRuntimeStableSnapshot(t)) || !reflect.DeepEqual(beforeUnits, nativeProfileState(t, receipt)) {
		t.Fatal("signature refusal changed installed state")
	}
	downloads.Lock()
	downloads.badHelper = false
	downloads.Unlock()
	phase("helper-signature-refusal", map[string]any{"files_unchanged": true, "units_unchanged": true})
	for _, flags := range []struct{ active, enabled bool }{{true, true}, {true, false}, {false, true}, {false, false}} {
		nativeProfileServiceFlags(t, receipt, flags.active, flags.enabled)
		baseline := secureRuntimeStableSnapshot(t)
		started := time.Now()
		if output, err := install(caPath, "--safe-profile-apply"); err != nil {
			t.Fatalf("signed TLS safe-profile apply: %v\n%s", err, output)
		}
		forwardReport := waitReport(versions["collector-v2"], started, true)
		secureRuntimeAssertSafeProfile(t)
		secureRuntimeAssertHelperProtocol(t)
		assertProcess("collector-v2")
		assertSavedTrust()
		if secureRuntimeHash(secureRuntimeReadFile(t, "/usr/local/lib/pulse-agent/pulse-agent-helper")) != receipt.Artifacts["helper"] {
			t.Fatal("installed helper differs from signed source input")
		}
		helperPIDText := strings.TrimSpace(string(nativeProfileMust(t, receipt, "systemctl", "show", "pulse-agent-helper.service", "--property=MainPID", "--value")))
		helperPID, err := strconv.Atoi(helperPIDText)
		if err != nil || helperPID <= 1 || secureRuntimeProcessExecutableHash(t, helperPID) != receipt.Artifacts["helper"] {
			t.Fatal("running helper differs from signed source input")
		}
		checkRunner()
		phase("forward-"+strconv.FormatBool(flags.active)+"-"+strconv.FormatBool(flags.enabled), map[string]any{"units": nativeProfileState(t, receipt), "collector_sha256": receipt.Artifacts["collector-v2"], "helper_sha256": receipt.Artifacts["helper"], "history": assertHistory(), "runner_unchanged": true, "authenticated_report": forwardReport})
		// The saved installer invocation recovers the endpoint and CA itself.
		reverseStarted := time.Now()
		nativeProfileMust(t, receipt, "bash", "/etc/pulse-agent/install.sh", "--safe-profile-rollback")
		nativeProfileAssertFlags(t, receipt, flags.active, flags.enabled)
		secureRuntimeWaitForStableIdentity(t, secureRuntimeWithoutCollectorUnit(baseline), 10*time.Second, "native reverse")
		assertRestoredState()
		if secureRuntimeCollectorHasArgument("--enable-commands") {
			t.Fatal("reverse restored withdrawn collector command authority")
		}
		if flags.active {
			assertProcess("collector-v1")
			secureRuntimeAssertRootMonitoringProfile(t)
			waitReport(versions["collector-v1"], reverseStarted, false)
		}
		assertSavedTrust()
		checkRunner()
		phase("reverse-"+strconv.FormatBool(flags.active)+"-"+strconv.FormatBool(flags.enabled), map[string]any{"units": nativeProfileState(t, receipt), "state": nativeProfileFiles(t, statePaths...), "lifecycle": nativeProfileOptionalFiles(t, lifecyclePaths...), "state_root": nativeProfileDirectory(t, "/var/lib/pulse-agent"), "history": assertHistory(), "collector_sha256": receipt.Artifacts["collector-v1"], "runner_unchanged": true})
	}
	// A real manager refusal must retain the migrated executable and the whole
	// recovery snapshot. This final failure is not retried by this lab.
	nativeProfileServiceFlags(t, receipt, true, true)
	if output, err := install(caPath, "--safe-profile-apply"); err != nil {
		t.Fatalf("final profile apply: %v\n%s", err, output)
	}
	secureRuntimeAssertHelperProtocol(t)
	dropin := "/etc/systemd/system/pulse-agent-helper.socket.d"
	if err := os.Mkdir(dropin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dropin, "native-refusal.conf"), []byte("[Unit]\nRefuseManualStop=yes\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	nativeProfileMust(t, receipt, "systemctl", "daemon-reload")
	before := nativeProfileFiles(t, "/usr/local/bin/pulse-agent", "/usr/local/lib/pulse-agent/pulse-agent-helper", "/var/lib/pulse-agent-profile/current.env")
	snapshotBefore := nativeProfileTree(t, "/var/lib/pulse-agent-profile")
	if _, err := nativeProfileCommand(t, receipt, "bash", "/etc/pulse-agent/install.sh", "--safe-profile-rollback"); err == nil {
		t.Fatal("native rollback ignored refused helper stop")
	}
	if !reflect.DeepEqual(before, nativeProfileFiles(t, "/usr/local/bin/pulse-agent", "/usr/local/lib/pulse-agent/pulse-agent-helper", "/var/lib/pulse-agent-profile/current.env")) || !reflect.DeepEqual(snapshotBefore, nativeProfileTree(t, "/var/lib/pulse-agent-profile")) {
		t.Fatal("refused quiescence replaced executables or recovery files")
	}
	checkRunner()
	phase("rollback-stop-refusal", map[string]any{"units": nativeProfileState(t, receipt), "recovery_files_retained": true, "runner_unchanged": true})
	receipt.Passed = true
}

func nativeProfileTree(t *testing.T, root string) secureRuntimeStableIdentity {
	t.Helper()
	result := make(secureRuntimeStableIdentity)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("snapshot contains a symlink: %s", path)
		}
		if entry.Type().IsRegular() {
			result[strings.TrimPrefix(path, root)] = nativeProfileFiles(t, path)[path]
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func nativeProfileCleanup(t *testing.T, receipt *nativeProfileReceipt) {
	t.Helper()
	// Remove only this test's override before stopping its owned services. No
	// production rollback or failed application operation is retried here.
	_ = os.Remove("/etc/systemd/system/pulse-agent-helper.socket.d/native-refusal.conf")
	_ = os.Remove("/etc/systemd/system/pulse-agent-helper.socket.d")
	_, reloadErr := nativeProfileCommand(t, receipt, "systemctl", "daemon-reload")
	complete := reloadErr == nil
	for _, unit := range nativeProfileUnits {
		output, err := nativeProfileCommand(t, receipt, "systemctl", "show", unit, "--property=LoadState", "--value")
		if err != nil {
			complete = false
			continue
		}
		if strings.TrimSpace(string(output)) != "not-found" {
			if _, err := nativeProfileCommand(t, receipt, "systemctl", "disable", "--now", unit); err != nil {
				complete = false
			}
		}
	}
	// The same-unit batch is checked after every stop, not just individually.
	for _, unit := range nativeProfileUnits {
		output, err := nativeProfileCommand(t, receipt, "systemctl", "show", unit, "--property=ActiveState", "--value")
		active := strings.TrimSpace(string(output))
		mainPID := ""
		if strings.HasSuffix(unit, ".service") {
			pidOutput, pidErr := nativeProfileCommand(t, receipt, "systemctl", "show", unit, "--property=MainPID", "--value")
			mainPID = strings.TrimSpace(string(pidOutput))
			if pidErr != nil {
				complete = false
			}
		}
		if err != nil || !nativeProfileUnitQuiescent(unit, active, mainPID) {
			complete = false
		}
	}
	quiescent := complete
	if quiescent {
		for _, unit := range nativeProfileUnits {
			if err := os.Remove(filepath.Join("/etc/systemd/system", unit)); err != nil && !errors.Is(err, os.ErrNotExist) {
				complete = false
			}
		}
		for _, path := range nativeProfilePaths {
			if path == "/opt/pulse" || path == "/etc/pulse" {
				continue
			}
			if err := os.RemoveAll(path); err != nil {
				complete = false
			}
		}
		if _, err := nativeProfileCommand(t, receipt, "systemctl", "daemon-reload"); err != nil {
			complete = false
		}
	}
	receipt.Cleanup = map[string]any{"complete": complete, "quiescence_confirmed": quiescent,
		"scope": "owned service stops and installed paths; fixture account and full VM teardown remain with the hosted executor",
		"at":    time.Now().UTC().Format(time.RFC3339Nano)}
	if !complete {
		t.Error("native cleanup incomplete; inspect retained phase and command observations")
	}
	if t.Failed() {
		receipt.Passed = false
	}
	data, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		t.Error(err)
		return
	}
	output := filepath.Join(os.Getenv("RUNNER_TEMP"), "pulse-installed-profile.json")
	// Only secret-free source/digest/service observations are published here;
	// the ordinary artifact uploader must be able to read root's receipt.
	if err := os.WriteFile(output, append(data, '\n'), 0o644); err != nil {
		t.Error(err)
	}
}
