package configapi

import (
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestContainerBootstrapHistoryAndShellData(t *testing.T) {
	root, env := bootstrapFixture(t, recordingInstaller)
	marker := filepath.Join(root, "must-not-exist")
	baseURL := `https://pulse.example/base/' ; touch ` + marker + ` ; #`
	token := `synthetic-' ; touch ` + marker + ` ; #`
	command := BuildContainerRuntimeAgentInstallCommand(baseURL, token, false)
	history := filepath.Join(root, "shell-history")
	payload, err := json.Marshal(map[string]string{"command": command, "input": token, "history": history})
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("python3", "-c", bootstrapPTYRunner)
	cmd.Stdin = strings.NewReader(string(payload))
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("interactive container bootstrap: %v\n%s", err, out)
	}
	hist, err := os.ReadFile(history)
	if err != nil || !strings.Contains(string(hist), command) {
		t.Fatal("history control did not record the copied command")
	}
	args, err := os.ReadFile(filepath.Join(root, "argv"))
	if err != nil || !strings.Contains(string(args), "--url\n"+baseURL+"\n") {
		t.Fatal("shell quoting changed the installer destination")
	}
	if strings.Contains(string(hist), token) || strings.Contains(string(args), token) || strings.Contains(string(out), token) {
		t.Fatal("private input reached history, argv or terminal output")
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("shell data was executed as source")
	}
	assertContainerBootstrapCleanup(t, root)
}

func TestContainerBootstrapOverflowDoesNotBecomeShellHistory(t *testing.T) {
	root, env := bootstrapFixture(t, recordingInstaller)
	token := strings.Repeat("x", 10000) + "-OVERFLOW_HISTORY_MARKER"
	command := BuildContainerRuntimeAgentInstallCommand("https://pulse.example", "separate-issued-token", true)
	history := filepath.Join(root, "shell-history")
	payload, err := json.Marshal(map[string]string{"command": command, "input": token, "history": history})
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("python3", "-c", bootstrapPTYRunner)
	cmd.Stdin = strings.NewReader(string(payload))
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("overflowed input was accepted")
	}
	hist, err := os.ReadFile(history)
	if err != nil || !strings.Contains(string(hist), command) {
		t.Fatal("history control did not record the copied command")
	}
	for _, output := range []string{string(out), string(hist)} {
		if strings.Contains(output, "OVERFLOW_HISTORY_MARKER") || strings.Contains(output, strings.Repeat("x", 512)) {
			t.Fatal("discarded credential input was echoed or returned to shell history")
		}
	}
	if _, err := os.Stat(filepath.Join(root, "captured-token")); !os.IsNotExist(err) {
		t.Fatal("installer ran after overflowed input")
	}
	assertContainerBootstrapCleanup(t, root)
}

func TestContainerBootstrapPreservesMaximumAcceptedInput(t *testing.T) {
	root, env := bootstrapFixture(t, recordingInstaller)
	token := strings.Repeat("a", 4096)
	command := BuildContainerRuntimeAgentInstallCommand("https://pulse.example", "separate-issued-token", true)
	out, err := runBootstrap(t, command, token, env, true)
	if err != nil {
		t.Fatalf("maximum-size private input: %v\n%s", err, out)
	}
	captured, err := os.ReadFile(filepath.Join(root, "captured-token"))
	if err != nil || string(captured) != token {
		t.Fatal("terminal silently truncated an accepted token")
	}
	assertContainerBootstrapCleanup(t, root)
}

func assertContainerBootstrapCleanup(t *testing.T, root string) {
	t.Helper()
	directories, err := os.ReadFile(filepath.Join(root, "private-directories"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	for _, path := range strings.Fields(string(directories)) {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatal("container bootstrap left a private handoff directory")
		}
	}
}

func TestContainerBootstrapPrivateRootAndSudoModes(t *testing.T) {
	for _, enableHost := range []bool{true, false} {
		for _, uid := range []string{"0", "1000"} {
			name := "workload-only"
			if enableHost {
				name = "host-and-workload"
			}
			t.Run(name+"_uid_"+uid, func(t *testing.T) {
				root, env := bootstrapFixture(t, recordingInstaller)
				env = append(env, "FAKE_ID_UID="+uid)
				token := strings.Repeat("e", 32)
				command := BuildContainerRuntimeAgentInstallCommand("https://pulse.example/base///", token, enableHost)
				out, err := runBootstrap(t, command, token, env, true)
				if err != nil {
					t.Fatalf("container bootstrap: %v\n%s", err, out)
				}
				captured, err := os.ReadFile(filepath.Join(root, "captured-token"))
				if err != nil || string(captured) != token {
					t.Fatal("private handoff lost the issued token")
				}
				args, err := os.ReadFile(filepath.Join(root, "argv"))
				if err != nil {
					t.Fatal(err)
				}
				secretEnv, err := os.ReadFile(filepath.Join(root, "secret-env"))
				if err != nil || string(secretEnv) != ":" || strings.Contains(string(out), token) || strings.Contains(string(args), token) || strings.Contains(command, token) {
					t.Fatal("credential reached source, argv, environment or terminal output")
				}
				hostArg := "--enable-host=false\n"
				if enableHost {
					hostArg = "--enable-host\n"
				}
				for _, arg := range []string{"--url\nhttps://pulse.example/base\n", "--enable-docker\n", hostArg, "--interval\n30s\n", "--non-interactive\n", "--preflight-only\n"} {
					if !strings.Contains(string(args), arg) {
						t.Fatalf("installer lost required argument %q", arg)
					}
				}
				if strings.Contains(string(args), "--enable-commands") || strings.Contains(string(args), "--insecure") {
					t.Fatal("container monitoring silently broadened execution or TLS policy")
				}
				assertContainerBootstrapCleanup(t, root)
			})
		}
	}
}

func TestContainerBootstrapFailuresStopAndClean(t *testing.T) {
	for _, tc := range []struct {
		name, env, input string
		tty              bool
	}{
		{"no-terminal", "", "", false},
		{"blank-token", "", "", true},
		{"oversized-token", "", strings.Repeat("x", 4097), true},
		{"interrupt-input", "", "\x03", true},
		{"partial-download", "FAKE_CURL_EXIT=22", "", false},
		{"preflight-failure", "FAKE_PREFLIGHT_EXIT=44", "", false},
		{"sudo-refusal", "FAKE_SUDO_EXIT=1", "", false},
		{"install-failure", "FAKE_INSTALL_EXIT=9", strings.Repeat("f", 32), true},
		{"terminate", "FAKE_INSTALL_SIGNAL=TERM", strings.Repeat("f", 32), true},
		{"hangup", "FAKE_INSTALL_SIGNAL=HUP", strings.Repeat("f", 32), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			installer := strings.Replace(recordingInstaller, `exit "${FAKE_INSTALL_EXIT:-0}"`, `if [ -n "${FAKE_INSTALL_SIGNAL:-}" ]; then kill -s "$FAKE_INSTALL_SIGNAL" "$PPID"; fi
exit "${FAKE_INSTALL_EXIT:-0}"`, 1)
			root, env := bootstrapFixture(t, installer)
			env = append(env, tc.env)
			if tc.name == "sudo-refusal" {
				env = append(env, "FAKE_ID_UID=1000")
			}
			command := BuildContainerRuntimeAgentInstallCommand("https://pulse.example", "synthetic-not-in-command", true)
			out, err := runBootstrap(t, command, tc.input, env, tc.tty)
			if err == nil {
				t.Fatal("failed or interrupted bootstrap appeared successful")
			}
			if tc.input != "" && tc.input != "\x03" && strings.Contains(string(out), tc.input) {
				t.Fatal("token echoed on failure")
			}
			if tc.name != "install-failure" && tc.name != "terminate" && tc.name != "hangup" {
				if _, err := os.Stat(filepath.Join(root, "captured-token")); !os.IsNotExist(err) {
					t.Fatal("installer ran before its prerequisite succeeded")
				}
			}
			assertContainerBootstrapCleanup(t, root)
		})
	}
}

func TestContainerBootstrapOptionalAuthNeedsNoTerminal(t *testing.T) {
	root, env := bootstrapFixture(t, `#!/bin/bash
set -eu
printf '%s\n' "$@" >> "$FAKE_ROOT/argv"
[[ " $* " != *" --token"* ]]
if [[ " $* " != *" --preflight-only "* ]]; then touch "$FAKE_ROOT/installed"; fi
`)
	command := BuildContainerRuntimeAgentInstallCommand("http://pulse.example:7655/", "", false)
	out, err := runBootstrap(t, command, "", env, false)
	if err != nil {
		t.Fatalf("optional-auth bootstrap: %v\n%s", err, out)
	}
	if strings.Contains(command, "read -r") || !strings.Contains(command, "--insecure") || strings.Contains(command, "-kfs") {
		t.Fatal("optional auth changed credential or HTTP trust policy")
	}
	if _, err := os.Stat(filepath.Join(root, "installed")); err != nil {
		t.Fatal("token-optional installer did not run")
	}
	assertContainerBootstrapCleanup(t, root)
}

// Exercise real curl/TLS download and the executable private handoff, not a
// system installation or a real Docker/Podman estate. The installer is modeled.
func TestContainerBootstrapTLSDownload(t *testing.T) {
	curlPath, err := exec.LookPath("curl")
	if err != nil {
		t.Fatal("curl required for transport proof")
	}
	for _, status := range []int{200, 401, 403, 503} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			root, env := bootstrapFixture(t, recordingInstaller)
			token := strings.Repeat("a", 32)
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/install.sh" || r.URL.RawQuery != "" || r.Header.Get("X-API-Token") != "" || r.Header.Get("Authorization") != "" {
					t.Error("download carried credentials or an unexpected target")
				}
				w.WriteHeader(status)
				w.Write([]byte(recordingInstaller))
			}))
			defer server.Close()
			writeExecutable(t, filepath.Join(root, "bin", "curl"), "#!/bin/sh\nprintf '%s\\n' \"$@\" >> \"$FAKE_ROOT/argv\"\nexec "+posixShellQuote(curlPath)+" \"$@\"\n")
			command := BuildContainerRuntimeAgentInstallCommand(server.URL, token, true)
			if status == 200 {
				if _, err := runBootstrap(t, command, "", env, false); err == nil {
					t.Fatal("untrusted TLS was accepted")
				}
			}
			ca := filepath.Join(root, "ca.pem")
			if err := os.WriteFile(ca, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}), 0600); err != nil {
				t.Fatal(err)
			}
			env = append(env, "CURL_CA_BUNDLE="+ca)
			out, err := runBootstrap(t, command, token, env, true)
			if (status == 200) != (err == nil) {
				t.Fatalf("trusted TLS download status %d: %v\n%s", status, err, out)
			}
			args, _ := os.ReadFile(filepath.Join(root, "argv"))
			if strings.Contains(string(out), token) || strings.Contains(string(args), token) {
				t.Fatal("transport exposed the credential")
			}
			if status != 200 {
				if _, err := os.Stat(filepath.Join(root, "captured-token")); !os.IsNotExist(err) {
					t.Fatal("installer ran after a failed download")
				}
			}
			assertContainerBootstrapCleanup(t, root)
		})
	}
}
