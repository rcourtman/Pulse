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
