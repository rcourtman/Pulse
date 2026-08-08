package installtests

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestUninstallSensorProxyScriptContract(t *testing.T) {
	scriptPath := repoFile("scripts", "uninstall-sensor-proxy.sh")
	content, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("read uninstall-sensor-proxy.sh: %v", err)
	}

	script := string(content)
	required := []string{
		`--remove-proxmox-access`,
		`--local-only`,
		`--ssh-known-hosts`,
		`StrictHostKeyChecking=yes`,
		`UpdateHostKeys=no`,
		`GlobalKnownHostsFile=none`,
		`pulse-sensor-proxy-selfheal.timer`,
		`pulse-sensor-cleanup.path`,
		`remove_managed_keys_from_authorized_keys_file()`,
		`cleanup_stale_sensor_proxy_mounts()`,
		`pulse-monitor@pam`,
		`# pulse-(managed|proxy)-key$`,
	}
	for _, needle := range required {
		if !strings.Contains(script, needle) {
			t.Fatalf("uninstall-sensor-proxy.sh missing cleanup contract: %s", needle)
		}
	}

	if out, err := exec.Command("bash", "-n", scriptPath).CombinedOutput(); err != nil {
		t.Fatalf("bash -n uninstall-sensor-proxy.sh: %v\n%s", err, out)
	}
}

func TestUninstallSensorProxyRemoteCleanupUsesProvisionedHostKeys(t *testing.T) {
	tmpDir := t.TempDir()
	binDir := filepath.Join(tmpDir, "bin")
	argsLog := filepath.Join(tmpDir, "ssh-args")
	knownHosts := filepath.Join(tmpDir, "known_hosts")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin dir: %v", err)
	}
	sshStub := `#!/bin/sh
printf '%s\n' "$@" >"$SSH_ARGS_LOG"
exit "${SSH_EXIT_STATUS:-0}"
`
	if err := os.WriteFile(filepath.Join(binDir, "ssh"), []byte(sshStub), 0o755); err != nil {
		t.Fatalf("write ssh stub: %v", err)
	}
	if err := os.WriteFile(knownHosts, []byte("node ssh-ed25519 AAAAtest\n"), 0o600); err != nil {
		t.Fatalf("write known_hosts: %v", err)
	}
	resolvedKnownHosts, err := filepath.EvalSymlinks(knownHosts)
	if err != nil {
		t.Fatalf("resolve known_hosts: %v", err)
	}

	command := `source "$1"
SSH_KNOWN_HOSTS_PATH="$2"
cleanup_remote_authorized_keys 192.0.2.10
`
	cmd := exec.Command("bash", "-c", command, "bash", repoFile("scripts", "uninstall-sensor-proxy.sh"), knownHosts)
	cmd.Env = append(os.Environ(),
		"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"SSH_ARGS_LOG="+argsLog,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("remote cleanup with provisioned trust failed: %v\n%s", err, out)
	}

	args, err := os.ReadFile(argsLog)
	if err != nil {
		t.Fatalf("read ssh args: %v", err)
	}
	got := string(args)
	for _, option := range []string{
		"StrictHostKeyChecking=yes",
		"UpdateHostKeys=no",
		"BatchMode=yes",
		"ConnectTimeout=5",
		"UserKnownHostsFile=" + resolvedKnownHosts,
		"GlobalKnownHostsFile=none",
		"root@192.0.2.10",
	} {
		if !strings.Contains(got, option) {
			t.Fatalf("ssh invocation missing %q:\n%s", option, got)
		}
	}
}

func TestUninstallSensorProxyRemoteCleanupUsesStrictConfiguredTrustByDefault(t *testing.T) {
	tmpDir := t.TempDir()
	binDir := filepath.Join(tmpDir, "bin")
	argsLog := filepath.Join(tmpDir, "ssh-args")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin dir: %v", err)
	}
	sshStub := `#!/bin/sh
printf '%s\n' "$@" >"$SSH_ARGS_LOG"
exit 0
`
	if err := os.WriteFile(filepath.Join(binDir, "ssh"), []byte(sshStub), 0o755); err != nil {
		t.Fatalf("write ssh stub: %v", err)
	}

	command := `source "$1"
SSH_KNOWN_HOSTS_PATH=""
cleanup_remote_authorized_keys 192.0.2.20
`
	cmd := exec.Command("bash", "-c", command, "bash", repoFile("scripts", "uninstall-sensor-proxy.sh"))
	cmd.Env = append(os.Environ(),
		"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"SSH_ARGS_LOG="+argsLog,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("remote cleanup with configured OpenSSH trust failed: %v\n%s", err, out)
	}

	args, err := os.ReadFile(argsLog)
	if err != nil {
		t.Fatalf("read ssh args: %v", err)
	}
	got := string(args)
	if !strings.Contains(got, "StrictHostKeyChecking=yes") {
		t.Fatalf("default ssh invocation did not require configured trust:\n%s", got)
	}
	if !strings.Contains(got, "UpdateHostKeys=no") {
		t.Fatalf("default ssh invocation could mutate host trust:\n%s", got)
	}
	for _, isolatedOption := range []string{"UserKnownHostsFile=", "GlobalKnownHostsFile="} {
		if strings.Contains(got, isolatedOption) {
			t.Fatalf("default ssh invocation unexpectedly replaced configured OpenSSH trust with %q:\n%s", isolatedOption, got)
		}
	}
}

func TestUninstallSensorProxyRemoteCleanupFailsWithoutProvisionedTrust(t *testing.T) {
	tmpDir := t.TempDir()
	binDir := filepath.Join(tmpDir, "bin")
	argsLog := filepath.Join(tmpDir, "ssh-args")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin dir: %v", err)
	}
	sshStub := `#!/bin/sh
printf '%s\n' "$@" >"$SSH_ARGS_LOG"
exit 0
`
	if err := os.WriteFile(filepath.Join(binDir, "ssh"), []byte(sshStub), 0o755); err != nil {
		t.Fatalf("write ssh stub: %v", err)
	}

	missingKnownHosts := filepath.Join(tmpDir, "missing-known-hosts")
	command := `source "$1"
SSH_KNOWN_HOSTS_PATH="$2"
cleanup_remote_authorized_keys 192.0.2.11
`
	cmd := exec.Command("bash", "-c", command, "bash", repoFile("scripts", "uninstall-sensor-proxy.sh"), missingKnownHosts)
	cmd.Env = append(os.Environ(),
		"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"SSH_ARGS_LOG="+argsLog,
	)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("remote cleanup unexpectedly accepted missing trust material:\n%s", out)
	}
	if !strings.Contains(string(out), "known_hosts file is missing, unreadable, or empty") {
		t.Fatalf("missing trust failure was not actionable:\n%s", out)
	}
	if _, statErr := os.Stat(argsLog); !os.IsNotExist(statErr) {
		t.Fatalf("ssh ran before provisioned trust was validated, stat err=%v", statErr)
	}
}

func TestUninstallSensorProxyRemoteCleanupRejectsHostKeyMismatch(t *testing.T) {
	tmpDir := t.TempDir()
	binDir := filepath.Join(tmpDir, "bin")
	knownHosts := filepath.Join(tmpDir, "known_hosts")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(binDir, "ssh"), []byte("#!/bin/sh\nexit 255\n"), 0o755); err != nil {
		t.Fatalf("write ssh stub: %v", err)
	}
	if err := os.WriteFile(knownHosts, []byte("node ssh-ed25519 AAAAold\n"), 0o600); err != nil {
		t.Fatalf("write known_hosts: %v", err)
	}

	command := `source "$1"
SSH_KNOWN_HOSTS_PATH="$2"
cleanup_remote_authorized_keys 192.0.2.12
`
	cmd := exec.Command("bash", "-c", command, "bash", repoFile("scripts", "uninstall-sensor-proxy.sh"), knownHosts)
	cmd.Env = append(os.Environ(), "PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("remote cleanup unexpectedly ignored SSH host-key rejection:\n%s", out)
	}
	if !strings.Contains(string(out), "Unable to verify 192.0.2.12's SSH host key") {
		t.Fatalf("host-key rejection was not actionable:\n%s", out)
	}
}

func TestUninstallSensorProxyLocalOnlyDoesNotUseSSH(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "cleanup-marker")
	command := `source "$1"
LOCAL_ONLY=true
cleanup_local_authorized_keys() { printf 'local\n' >"$LOCAL_CLEANUP_MARKER"; }
cleanup_remote_authorized_keys() { return 99; }
cleanup_cluster_authorized_keys
`
	cmd := exec.Command("bash", "-c", command, "bash", repoFile("scripts", "uninstall-sensor-proxy.sh"))
	cmd.Env = append(os.Environ(), "LOCAL_CLEANUP_MARKER="+marker)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("local-only cleanup failed: %v\n%s", err, out)
	}
	content, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("local-only cleanup did not run local key removal: %v", err)
	}
	if string(content) != "local\n" {
		t.Fatalf("unexpected local-only marker: %q", content)
	}
}

func TestUninstallSensorProxyReportsRemoteTrustFailureAfterLocalCleanup(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "cleanup-marker")
	command := `source "$1"
disable_legacy_units() { :; }
cleanup_cluster_authorized_keys() { return 23; }
cleanup_stale_sensor_proxy_mounts() { printf 'mounts\n' >>"$LOCAL_CLEANUP_MARKER"; }
remove_legacy_files() { printf 'files\n' >>"$LOCAL_CLEANUP_MARKER"; }
remove_proxmox_access() { printf 'access\n' >>"$LOCAL_CLEANUP_MARKER"; }
systemctl_if_available() { :; }
main --quiet
`
	cmd := exec.Command("bash", "-c", command, "bash", repoFile("scripts", "uninstall-sensor-proxy.sh"))
	cmd.Env = append(os.Environ(), "LOCAL_CLEANUP_MARKER="+marker)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("main unexpectedly hid remote trust failure:\n%s", out)
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok || exitErr.ExitCode() != 23 {
		t.Fatalf("main returned %v, want remote cleanup status 23:\n%s", err, out)
	}
	if !strings.Contains(string(out), "Local cleanup completed") {
		t.Fatalf("remote trust failure did not explain local cleanup state:\n%s", out)
	}
	content, readErr := os.ReadFile(marker)
	if readErr != nil {
		t.Fatalf("read local cleanup marker: %v", readErr)
	}
	if string(content) != "mounts\nfiles\naccess\n" {
		t.Fatalf("local cleanup did not complete before remote failure was returned: %q", content)
	}
}

func TestUninstallSensorProxyScriptRemovesTempFootprintAndManagedKeys(t *testing.T) {
	tmpDir := t.TempDir()
	binDir := filepath.Join(tmpDir, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatalf("mkdir bin dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(binDir, "systemctl"), []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatalf("write systemctl stub: %v", err)
	}
	if err := os.WriteFile(filepath.Join(binDir, "pct"), []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatalf("write pct stub: %v", err)
	}

	binaryPath := filepath.Join(tmpDir, "pulse-sensor-proxy")
	installRoot := filepath.Join(tmpDir, "install-root")
	servicePath := filepath.Join(tmpDir, "pulse-sensor-proxy.service")
	runtimeDir := filepath.Join(tmpDir, "run")
	socketPath := filepath.Join(runtimeDir, "pulse-sensor-proxy.sock")
	workDir := filepath.Join(tmpDir, "work")
	configDir := filepath.Join(tmpDir, "config")
	logDir := filepath.Join(tmpDir, "logs")
	authKeys := filepath.Join(tmpDir, "authorized_keys")

	for _, dir := range []string{installRoot, runtimeDir, workDir, configDir, logDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	for _, path := range []string{binaryPath, servicePath, socketPath, filepath.Join(installRoot, "bin", "pulse-sensor-proxy")} {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("mkdir parent for %s: %v", path, err)
		}
		if err := os.WriteFile(path, []byte("legacy"), 0644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	if err := os.WriteFile(authKeys, []byte(strings.Join([]string{
		"ssh-ed25519 AAAAlegacy1 pulse # pulse-managed-key",
		"ssh-ed25519 AAAAkeep user-key",
		"ssh-ed25519 AAAAlegacy2 pulse # pulse-proxy-key",
		"",
	}, "\n")), 0600); err != nil {
		t.Fatalf("write authorized_keys: %v", err)
	}

	cmd := exec.Command("bash", repoFile("scripts", "uninstall-sensor-proxy.sh"), "--uninstall", "--purge", "--quiet")
	cmd.Env = append(os.Environ(),
		"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"PULSE_SENSOR_PROXY_BINARY_PATH="+binaryPath,
		"PULSE_SENSOR_PROXY_INSTALL_ROOT="+installRoot,
		"PULSE_SENSOR_PROXY_SERVICE_PATH="+servicePath,
		"PULSE_SENSOR_PROXY_RUNTIME_DIR="+runtimeDir,
		"PULSE_SENSOR_PROXY_SOCKET_PATH="+socketPath,
		"PULSE_SENSOR_PROXY_WORK_DIR="+workDir,
		"PULSE_SENSOR_PROXY_CONFIG_DIR="+configDir,
		"PULSE_SENSOR_PROXY_LOG_DIR="+logDir,
		"PULSE_SENSOR_PROXY_SERVICE_USER=pulse-sensor-proxy-test-user",
		"PULSE_SENSOR_PROXY_AUTHORIZED_KEYS_PATH="+authKeys,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("uninstall-sensor-proxy.sh failed: %v\n%s", err, out)
	}

	for _, path := range []string{binaryPath, installRoot, servicePath, runtimeDir, workDir, configDir, logDir} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("expected %s to be removed, stat err=%v", path, err)
		}
	}

	content, err := os.ReadFile(authKeys)
	if err != nil {
		t.Fatalf("read authorized_keys: %v", err)
	}
	got := string(content)
	if strings.Contains(got, "pulse-managed-key") || strings.Contains(got, "pulse-proxy-key") {
		t.Fatalf("managed Pulse SSH keys were not removed:\n%s", got)
	}
	if !strings.Contains(got, "AAAAkeep") {
		t.Fatalf("non-Pulse SSH key was not preserved:\n%s", got)
	}
}
