package configapi

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type portableBootstrapCaller struct {
	name  string
	build func(string) string
}

func portableBootstrapCallers() []portableBootstrapCaller {
	return []portableBootstrapCaller{
		{"proxmox", func(token string) string {
			return BuildProxmoxAgentInstallCommand(AgentInstallCommandOptions{BaseURL: "https://pulse.example", Token: token, InstallType: "pbs", IncludeInstallType: true})
		}},
		{"setup", func(token string) string {
			return BuildSetupScriptCommand("https://pulse.example/api/setup-script?type=pbs", token)
		}},
		{"container", func(token string) string {
			return BuildContainerRuntimeAgentInstallCommand("https://pulse.example", token, false)
		}},
	}
}

// PATH contains only the prerequisite tools and our closed download/privilege
// fixtures. The real controlling PTY is never a customer or appliance terminal.
// Explicit proof inputs can supply the exact static BusyBox used on CI hosts.
func portableBootstrapFixture(t *testing.T, mode string) (string, []string) {
	t.Helper()
	root, env := bootstrapFixture(t, recordingInstaller)
	bin := filepath.Join(root, "bin")
	for _, name := range []string{"sh", "bash", "cp", "rm", "rmdir", "cat", "stat", "dirname", "touch"} {
		path, err := exec.LookPath(name)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(path, filepath.Join(bin, name)); err != nil {
			t.Fatal(err)
		}
	}
	if mode == "standalone" || mode == "prefer-standalone" {
		stty, err := exec.LookPath("stty")
		if err != nil {
			t.Fatal(err)
		}
		writeExecutable(t, filepath.Join(bin, "stty"), `#!/bin/sh
printf '%s\n' "$*" >> "$FAKE_ROOT/tty-calls"
case "${FAKE_TTY_FAILURE:-}:$1" in
  empty:-g) exit 0 ;;
  snapshot:-g) exit 1 ;;
  echo:-echo) exit 1 ;;
esac
if [ "${FAKE_TTY_FAILURE:-}" = restore ] && [ "$1" != -g ] && [ "$1" != -echo ] && [ ! -e "$FAKE_ROOT/restore-failed" ]; then
    touch "$FAKE_ROOT/restore-failed"
    exit 1
fi
exec `+posixShellQuote(stty)+` "$@"
`)
	}
	if mode == "busybox" {
		path := os.Getenv("PULSE_TEST_BUSYBOX")
		if path == "" {
			var err error
			path, err = exec.LookPath("busybox")
			if err != nil {
				t.Skip("real BusyBox input unavailable; modeled tool and standalone controls still run")
			}
		}
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(filepath.Join(root, "native"), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "native", "busybox"), body, 0700); err != nil {
			t.Fatal(err)
		}
		writeExecutable(t, filepath.Join(bin, "busybox"), `#!/bin/sh
printf '%s\n' "$*" >> "$FAKE_ROOT/tty-calls"
if [ "$1" = stty ] && [ "$2" = -echo ]; then printf '%s\n' "$PPID" > "$FAKE_ROOT/reader-pid"; fi
exec "$FAKE_ROOT/native/busybox" "$@"
`)
	} else if mode == "unsupported-busybox" || mode == "prefer-standalone" {
		writeExecutable(t, filepath.Join(bin, "busybox"), `#!/bin/sh
printf '%s\n' "$*" >> "$FAKE_ROOT/busybox-calls"
exit 97
`)
	}
	return root, append(env, "PATH="+bin)
}

func runPortableBootstrap(t *testing.T, command, input string, env []string, history, signalName string) ([]byte, error) {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"command": command, "input": input, "history": history,
		"signal_on_prompt": signalName, "alternate_tty": true,
	})
	if err != nil {
		t.Fatal(err)
	}
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(python, "-c", bootstrapPTYRunner)
	cmd.Stdin = strings.NewReader(string(payload))
	cmd.Env = append(os.Environ(), env...)
	return cmd.CombinedOutput()
}

func assertPortableBootstrapPrivate(t *testing.T, root, token string, out []byte) {
	t.Helper()
	for _, name := range []string{"argv", "shell-history", "secret-env"} {
		body, err := os.ReadFile(filepath.Join(root, name))
		if err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		if token != "" && strings.Contains(string(body), token) {
			t.Fatalf("private input reached %s", name)
		}
		if name == "secret-env" && err == nil && string(body) != ":" {
			t.Fatal("installer received a credential through the environment")
		}
	}
	if token != "" && strings.Contains(string(out), token) {
		t.Fatal("private input was echoed")
	}
	if strings.Contains(string(out), "Bootstrap did not restore") || strings.Contains(string(out), "PTY fixture deadline expired") {
		t.Fatal("complete terminal restoration or bounded execution failed")
	}
	assertContainerBootstrapCleanup(t, root)
}

func TestPrivateBootstrapPortableToolFailures(t *testing.T) {
	for _, caller := range portableBootstrapCallers() {
		for _, tc := range []struct {
			name, mode, fault, message string
			tty                        bool
		}{
			{"no-terminal", "absent", "", "A terminal is required to enter the token.", false},
			{"no-tool", "absent", "", "Silent token input requires stty or BusyBox stty.", true},
			{"unsupported-applet", "unsupported-busybox", "", "Cannot read terminal settings with stty or BusyBox stty.", true},
			{"empty-state", "standalone", "empty", "Cannot read terminal settings with stty or BusyBox stty.", true},
			{"failed-state", "standalone", "snapshot", "Cannot read terminal settings with stty or BusyBox stty.", true},
			{"failed-echo", "standalone", "echo", "Cannot disable terminal echo.", true},
			{"failed-restore", "standalone", "restore", "Cannot restore terminal settings.", true},
		} {
			t.Run(caller.name+"/"+tc.name, func(t *testing.T) {
				root, env := portableBootstrapFixture(t, tc.mode)
				env = append(env, "FAKE_TTY_FAILURE="+tc.fault)
				command := caller.build("issued-but-not-in-command")
				var out []byte
				var err error
				if tc.tty {
					out, err = runPortableBootstrap(t, command, "synthetic-private-token", env, "", "")
				} else {
					out, err = runBootstrap(t, command, "", env, false)
				}
				if err == nil || !strings.Contains(string(out), tc.message) {
					t.Fatalf("expected distinct fail-closed %s diagnostic; exit=%v, matched=%t\n%s", tc.name, err, strings.Contains(string(out), tc.message), out)
				}
				if _, err := os.Stat(filepath.Join(root, "captured-token")); !os.IsNotExist(err) {
					t.Fatal("installation preceded safe terminal restoration")
				}
				if tc.name != "failed-restore" && strings.Contains(string(out), "(paste at this prompt, not in the command): ") {
					t.Fatal("credential requested before silent input was available")
				}
				if tc.name == "failed-restore" {
					calls, err := os.ReadFile(filepath.Join(root, "tty-calls"))
					lines := strings.Split(strings.TrimSpace(string(calls)), "\n")
					if err != nil || len(lines) != 4 || lines[0] != "-g" || lines[1] != "-echo" || lines[2] != lines[3] {
						t.Fatal("cleanup did not retry the same full state with the selected tool")
					}
				}
				assertPortableBootstrapPrivate(t, root, "synthetic-private-token", out)
			})
		}
	}
}

func TestPrivateBootstrapRealBusyBoxInput(t *testing.T) {
	for _, caller := range portableBootstrapCallers() {
		for _, tc := range []struct {
			name, input, signalName string
			failure                 bool
		}{
			{"ordinary", strings.Repeat("p", 32), "", false},
			{"maximum", strings.Repeat("q", 4096), "", false},
			{"overflow", strings.Repeat("r", 10000) + "-DISCARDED-CREDENTIAL", "", true},
			{"empty", "", "", true},
			{"interrupt", "\x03", "", true},
			{"hangup", "", "HUP", true},
			{"terminate", "", "TERM", true},
		} {
			t.Run(caller.name+"/"+tc.name, func(t *testing.T) {
				root, env := portableBootstrapFixture(t, "busybox")
				command := caller.build("issued-separately")
				out, err := runPortableBootstrap(t, command, tc.input, env, "", tc.signalName)
				if tc.signalName != "" {
					body, readErr := os.ReadFile(filepath.Join(root, "signal-observation"))
					if readErr != nil {
						t.Fatal("signal execution observation missing")
					}
					var observation struct {
						Sent                   bool  `json:"sent"`
						Exit                   int   `json:"exit"`
						CompleteTTYRestored    bool  `json:"complete_tty_restored"`
						InputChildren          []int `json:"input_children"`
						RemainingInputChildren []int `json:"remaining_input_children"`
					}
					if json.Unmarshal(body, &observation) != nil || !observation.Sent {
						t.Fatal("requested reader signal was not actually sent")
					}
					// This fixed-field record contains no command, input or environment.
					// Keep delivery separate from a successful cleanup assertion.
					t.Logf("reader signal observation: %s", body)
					if !observation.CompleteTTYRestored || observation.Exit != map[string]int{"HUP": 129, "TERM": 143}[tc.signalName] {
						t.Fatal("prompt-time signal did not retain its exit and complete terminal restoration")
					}
					if runtime.GOOS == "linux" && (len(observation.InputChildren) != 1 || len(observation.RemainingInputChildren) != 0) {
						t.Fatal("input was not supervised and reaped before cleanup completed")
					}
				}
				if (err != nil) != tc.failure {
					t.Fatalf("BusyBox bootstrap result=%v, expected failure=%t\n%s", err, tc.failure, out)
				}
				captured, captureErr := os.ReadFile(filepath.Join(root, "captured-token"))
				if tc.failure {
					if !os.IsNotExist(captureErr) {
						t.Fatal("invalid or interrupted entry reached installation")
					}
				} else if captureErr != nil || string(captured) != tc.input {
					t.Fatal("BusyBox reader lost accepted input")
				}
				calls, err := os.ReadFile(filepath.Join(root, "tty-calls"))
				lines := strings.Split(strings.TrimSpace(string(calls)), "\n")
				if err != nil || len(lines) != 3 || lines[0] != "stty -g" || lines[1] != "stty -echo" || !strings.HasPrefix(lines[2], "stty ") {
					t.Fatalf("snapshot, silent mode and full restoration did not use the selected real BusyBox applet: %q\n%s", lines, out)
				}
				assertPortableBootstrapPrivate(t, root, tc.input, out)
			})
		}
	}
}

func TestPrivateBootstrapStandalonePreferredAndBusyBoxHistory(t *testing.T) {
	for _, caller := range portableBootstrapCallers() {
		for _, mode := range []string{"prefer-standalone", "busybox"} {
			for _, uid := range []string{"0", "1000"} {
				t.Run(fmt.Sprintf("%s/%s/uid-%s", caller.name, mode, uid), func(t *testing.T) {
					root, env := portableBootstrapFixture(t, mode)
					env = append(env, "FAKE_ID_UID="+uid)
					token := "synthetic-history-token-0123456789"
					command := caller.build(token)
					out, err := runPortableBootstrap(t, command, token, env, filepath.Join(root, "shell-history"), "")
					if err != nil {
						t.Fatalf("interactive portable bootstrap failed: %v", err)
					}
					history, err := os.ReadFile(filepath.Join(root, "shell-history"))
					if err != nil || !strings.Contains(string(history), command) || !strings.HasSuffix(string(history), "exit\n") {
						t.Fatal("caller did not record the complete command and regain the terminal")
					}
					captured, err := os.ReadFile(filepath.Join(root, "captured-token"))
					if err != nil || string(captured) != token {
						t.Fatal("interactive private handoff lost input")
					}
					if _, err := os.Stat(filepath.Join(root, "busybox-calls")); !os.IsNotExist(err) {
						t.Fatal("failed standalone tool selection fell back to another utility")
					}
					assertPortableBootstrapPrivate(t, root, token, out)
				})
			}
		}
	}
}

func TestPrivateBootstrapSinglePrivilegedBody(t *testing.T) {
	for _, caller := range portableBootstrapCallers() {
		t.Run(caller.name, func(t *testing.T) {
			command := caller.build("private-not-in-command")
			if strings.Count(command, "bash -c ") != 1 || !strings.Contains(command, `then set --; else set -- sudo; fi;`) || !strings.Contains(command, `"$@" bash -c `) {
				t.Fatal("private reader is duplicated or root/sudo prefix escaped its subshell")
			}
			if len(command) >= 4096 || strings.ContainsAny(command, "\r\n") {
				t.Fatalf("ordinary single-line command does not fit the common terminal limit: %d bytes", len(command))
			}
			t.Logf("ordinary %s command: %d bytes", caller.name, len(command))
		})
	}
}

// The browser uses these actual builder fields, never a handwritten command.
// All credentials/endpoints here are inert synthetic values, not issued tokens.
func TestPrivateBootstrapConsumerPayload(t *testing.T) {
	const base = "http://127.0.0.1:5237"
	payload := make(map[string]any)
	for _, kind := range []string{"pve", "pbs"} {
		token := strings.Repeat("a", 32)
		payload[kind+"-setup"] = BuildSetupScriptInstallArtifact(base, kind, "https://"+kind+".example:"+map[string]string{"pve": "8006", "pbs": "8007"}[kind], base, false, token, 1900000000)
		payload[kind+"-agent"] = map[string]string{
			"command": BuildProxmoxAgentInstallCommand(AgentInstallCommandOptions{BaseURL: base, Token: strings.Repeat("b", 32), InstallType: kind, IncludeInstallType: true}),
			"token":   strings.Repeat("b", 32),
		}
	}
	payload["container"] = map[string]string{"command": BuildContainerRuntimeAgentInstallCommand(base, strings.Repeat("c", 32), false), "token": strings.Repeat("c", 32)}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	if os.Getenv("PULSE_EMIT_BOOTSTRAP_CONSUMER") == "1" {
		t.Logf("PRIVATE_BOOTSTRAP_PAYLOAD=%s", base64.StdEncoding.EncodeToString(body))
	}
}
