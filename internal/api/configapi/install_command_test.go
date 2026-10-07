package configapi

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// A real controlling terminal is necessary: stdin must not accidentally become
// a credential channel for a downloaded/piped script. The token travels only
// over the PTY after the prompt appears, not Python/child argv or environment.
const bootstrapPTYRunner = `
import errno, fcntl, json, os, pty, select, signal, subprocess, sys, termios, time
p = json.load(sys.stdin)
master, slave = pty.openpty()
tty_watch = os.dup(slave)
initial_tty = termios.tcgetattr(tty_watch)
def session():
    os.setsid()
    fcntl.ioctl(0, termios.TIOCSCTTY, 0)
history = p.get("history")
env = os.environ.copy()
if history:
    env.update(HISTFILE=history, HISTSIZE="100", HISTFILESIZE="100", HISTCONTROL="", PS1="PULSE_TEST$ ")
program = ["bash", "--noprofile", "--norc", "-i"] if history else ["sh", "-c", p["command"]]
child = subprocess.Popen(program, stdin=slave, stdout=slave, stderr=slave, preexec_fn=session, env=env)
os.close(slave)
output = b""
sent = False
command_sent = False
exit_sent = False
deadline = time.monotonic() + 15
while time.monotonic() < deadline:
    ready, _, _ = select.select([master], [], [], .1)
    if ready:
        try: chunk = os.read(master, 65536)
        except OSError as e:
            if e.errno == errno.EIO: break
            raise
        if not chunk: break
        output += chunk
        if history and output.endswith(b"PULSE_TEST$ "):
            if not command_sent:
                os.write(master, (p["command"] + "\n").encode())
                command_sent = True
            elif not exit_sent:
                # Do not queue shell input while the bootstrap/installer still
                # owns the terminal or may be restoring its input mode.
                os.write(master, b"exit\n")
                exit_sent = True
        # The interactive shell also echoes the copied command, which contains
        # this prompt literal. A PTY read may end there before the command has
        # even run. Never deliver a credential while terminal echo is enabled:
        # the real bootstrap disables it before printing the input prompt.
        silent = not (termios.tcgetattr(tty_watch)[3] & termios.ECHO)
        if output.endswith(b"(paste at this prompt, not in the command): ") and silent and not sent:
            os.write(master, (p["input"] + "\n").encode())
            sent = True
    if child.poll() is not None and not ready: break
else:
    os.killpg(child.pid, signal.SIGKILL)
    output += b"\nPTY fixture deadline expired\n"
code = child.wait()
final_tty = termios.tcgetattr(tty_watch)
mode_mask = termios.ECHO | termios.ICANON
if (initial_tty[3] & mode_mask) != (final_tty[3] & mode_mask):
    output += b"\nBootstrap did not restore terminal echo/canonical mode\n"
    code = 1
os.close(tty_watch)
os.close(master)
sys.stdout.buffer.write(output)
sys.exit(code if code >= 0 else 128 - code)
`

func runBootstrap(t *testing.T, command, token string, env []string, tty bool) ([]byte, error) {
	t.Helper()
	var cmd *exec.Cmd
	if tty {
		payload, err := json.Marshal(map[string]string{"command": command, "input": token})
		if err != nil {
			t.Fatal(err)
		}
		cmd = exec.Command("python3", "-c", bootstrapPTYRunner)
		cmd.Stdin = strings.NewReader(string(payload))
	} else {
		cmd = exec.Command("sh", "-c", command)
	}
	cmd.Env = append(os.Environ(), env...)
	return cmd.CombinedOutput()
}

func writeExecutable(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
}

func bootstrapFixture(t *testing.T, installer string) (string, []string) {
	t.Helper()
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	if err := os.Mkdir(bin, 0700); err != nil {
		t.Fatal(err)
	}
	writeExecutable(t, filepath.Join(root, "installer.sh"), installer)
	writeExecutable(t, filepath.Join(bin, "curl"), `#!/bin/sh
printf '%s\n' "$@" >> "$FAKE_ROOT/argv"
out=""
while [ "$#" -gt 0 ]; do
    case "$1" in -o) out="$2"; shift 2 ;; *) shift ;; esac
done
printf %s "$out" > "$FAKE_ROOT/download-path"
cp "$FAKE_ROOT/installer.sh" "$out"
exit "${FAKE_CURL_EXIT:-0}"
`)
	writeExecutable(t, filepath.Join(bin, "sudo"), `#!/bin/sh
printf '%s\n' "$@" >> "$FAKE_ROOT/argv"
if [ "$1" = "-v" ]; then exit "${FAKE_SUDO_EXIT:-0}"; fi
exec "$@"
`)
	writeExecutable(t, filepath.Join(bin, "id"), `#!/bin/sh
if [ "$1" = "-u" ]; then printf '%s\n' "$FAKE_ID_UID"; else exec /usr/bin/id "$@"; fi
`)
	writeExecutable(t, filepath.Join(bin, "mktemp"), `#!/bin/sh
path=$(/usr/bin/mktemp "$@") || exit 1
printf '%s\n' "$path" >> "$FAKE_ROOT/private-directories"
printf '%s\n' "$path"
`)
	return root, []string{"PATH=" + bin + ":" + os.Getenv("PATH"), "FAKE_ROOT=" + root, "FAKE_ID_UID=0", "PULSE_SETUP_TOKEN=", "PULSE_TOKEN="}
}

const recordingInstaller = `#!/bin/bash
set -eu
printf '%s\n' "$@" >> "$FAKE_ROOT/argv"
if [[ " $* " == *" --preflight-only "* ]]; then
    printf preflight > "$FAKE_ROOT/preflight"
    exit "${FAKE_PREFLIGHT_EXIT:-0}"
fi
printf '%s:%s' "${PULSE_TOKEN:-}" "${PULSE_SETUP_TOKEN:-}" > "$FAKE_ROOT/secret-env"
token_file="${PULSE_SETUP_TOKEN_FILE:-}"
while [ "$#" -gt 0 ]; do
    case "$1" in --token-file) token_file="$2"; shift 2 ;; *) shift ;; esac
done
[ -n "$token_file" ]
[ "$(stat -c %a "$token_file")" = 600 ]
[ "$(stat -c %a "$(dirname "$token_file")")" = 700 ]
[ "$(stat -c %u "$token_file")" = "$(stat -c %u "$(dirname "$token_file")")" ]
printf %s "$token_file" > "$FAKE_ROOT/token-path"
cat "$token_file" > "$FAKE_ROOT/captured-token"
exit "${FAKE_INSTALL_EXIT:-0}"
`

func TestPrivateBootstrapKeepsSecretsOutOfCommands(t *testing.T) {
	secret := "synthetic-secret-0123456789"
	artifact := BuildSetupScriptInstallArtifact("https://pulse.example/base", "pbs", "https://pbs.example:8007", "https://pulse.example/base", false, secret, 1900000000)
	if artifact.DownloadURL != artifact.URL || strings.Contains(artifact.DownloadURL, secret) {
		t.Fatal("credential-bearing download")
	}
	for name, command := range map[string]string{
		"setup":                   artifact.Command,
		"setup-with-env-alias":    artifact.CommandWithEnv,
		"setup-without-env-alias": artifact.CommandWithoutEnv,
		"agent-pve":               BuildProxmoxAgentInstallCommand(AgentInstallCommandOptions{BaseURL: "https://pulse.example", Token: secret, InstallType: "pve", IncludeInstallType: true}),
		"agent-pbs":               BuildProxmoxAgentInstallCommand(AgentInstallCommandOptions{BaseURL: "https://pulse.example", Token: secret, InstallType: "pbs", IncludeInstallType: true}),
	} {
		t.Run(name, func(t *testing.T) {
			if strings.Contains(command, secret) || strings.ContainsAny(command, "\r\n") || strings.Contains(command, "PULSE_SETUP_TOKEN=") || strings.Contains(command, "--token '") || strings.Contains(command, "| bash") {
				t.Fatal("copied command exposes a credential or executes a partial fetch")
			}
			if !strings.Contains(command, "read -r -s -p") || !strings.Contains(command, "sudo bash -c") || !strings.Contains(command, " -o \"$install_script\"") {
				t.Fatal("missing private prompt, sudo or complete download")
			}
		})
	}
}

func TestBuildProxmoxAgentInstallCommandExecutesTrustedRootAndSudoTokenBootstrap(t *testing.T) {
	for _, kind := range []string{"agent", "setup"} {
		for _, uid := range []string{"0", "1000"} {
			t.Run(kind+"_uid_"+uid, func(t *testing.T) {
				root, env := bootstrapFixture(t, recordingInstaller)
				env = append(env, "FAKE_ID_UID="+uid)
				token := strings.Repeat("a", 32)
				command := BuildSetupScriptCommand("https://pulse.example/api/setup-script?type=pbs", token)
				if kind == "agent" {
					command = BuildProxmoxAgentInstallCommand(AgentInstallCommandOptions{BaseURL: "https://pulse.example", Token: token, InstallType: "pbs", IncludeInstallType: true})
				}
				out, err := runBootstrap(t, command, token, env, true)
				if err != nil {
					t.Fatalf("bootstrap: %v\n%s", err, out)
				}
				captured, err := os.ReadFile(filepath.Join(root, "captured-token"))
				if err != nil {
					t.Fatal(err)
				}
				if string(captured) != token {
					t.Fatal("private-file token differs")
				}
				args, _ := os.ReadFile(filepath.Join(root, "argv"))
				secretEnv, _ := os.ReadFile(filepath.Join(root, "secret-env"))
				if strings.Contains(string(args), token) || strings.Contains(string(out), token) || string(secretEnv) != ":" {
					t.Fatal("token leaked into argv, environment or terminal output")
				}
				for _, record := range []string{"token-path", "download-path"} {
					path, err := os.ReadFile(filepath.Join(root, record))
					if err != nil {
						t.Fatal(err)
					}
					if _, err := os.Stat(string(path)); !os.IsNotExist(err) {
						t.Fatalf("handoff remains: %s", record)
					}
					if _, err := os.Stat(filepath.Dir(string(path))); !os.IsNotExist(err) {
						t.Fatalf("handoff directory remains: %s", record)
					}
				}
				_, err = os.Stat(filepath.Join(root, "preflight"))
				if kind == "agent" && err != nil {
					t.Fatal("agent preflight omitted")
				}
				if kind == "setup" && !os.IsNotExist(err) {
					t.Fatal("native setup ran agent preflight")
				}
			})
		}
	}
}

func TestPrivateBootstrapFailsBeforeInstallAndCleansHandoffs(t *testing.T) {
	for _, tc := range []struct {
		name  string
		env   string
		tty   bool
		input string
	}{
		{"no-terminal", "", false, ""}, {"blank-token", "", true, ""}, {"interrupt", "", true, "\x03"},
		{"partial-download", "FAKE_CURL_EXIT=22", false, ""}, {"wrong-installer-or-preflight", "FAKE_PREFLIGHT_EXIT=44", false, ""},
		{"sudo-refusal", "FAKE_SUDO_EXIT=1", false, ""}, {"install-failure", "FAKE_INSTALL_EXIT=9", true, strings.Repeat("b", 32)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, env := bootstrapFixture(t, recordingInstaller)
			env = append(env, tc.env)
			if tc.name == "sudo-refusal" {
				env = append(env, "FAKE_ID_UID=1000")
			}
			command := BuildProxmoxAgentInstallCommand(AgentInstallCommandOptions{BaseURL: "https://pulse.example", Token: "synthetic-not-in-command", InstallType: "pve", IncludeInstallType: true})
			out, err := runBootstrap(t, command, tc.input, env, tc.tty)
			if err == nil {
				t.Fatalf("failure appeared successful: %s", out)
			}
			if tc.input != "" && tc.input != "\x03" && strings.Contains(string(out), tc.input) {
				t.Fatal("token echoed on failure")
			}
			_, err = os.Stat(filepath.Join(root, "captured-token"))
			if tc.name != "install-failure" && !os.IsNotExist(err) {
				t.Fatal("installer ran before prerequisite succeeded")
			}
			directories, _ := os.ReadFile(filepath.Join(root, "private-directories"))
			for _, path := range strings.Fields(string(directories)) {
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatal("failure left a private bootstrap directory")
				}
			}

			for _, record := range []string{"download-path", "token-path"} {
				p, err := os.ReadFile(filepath.Join(root, record))
				if os.IsNotExist(err) {
					continue
				}
				if err != nil {
					t.Fatal(err)
				}
				if _, err := os.Stat(filepath.Dir(string(p))); !os.IsNotExist(err) {
					t.Fatalf("failure left %s directory", record)
				}
			}
		})
	}
}

func TestSetupPrivateFileRejectsUnsafeInputsBeforeNativeMutation(t *testing.T) {
	for _, tc := range []string{"valid", "public-mode", "public-parent", "symlink", "oversized", "malformed", "missing", "directory"} {
		t.Run(tc, func(t *testing.T) {
			root := t.TempDir()
			if err := os.Chmod(root, 0700); err != nil {
				t.Fatal(err)
			}
			file := filepath.Join(root, "token")
			token := strings.Repeat("c", 32)
			body := token
			if tc == "oversized" {
				body = strings.Repeat("c", 4097)
			}
			if tc == "malformed" {
				body = "do not echo this credential"
			}
			if err := os.WriteFile(file, []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
			switch tc {
			case "public-mode":
				os.Chmod(file, 0644)
			case "public-parent":
				os.Chmod(root, 0755)
			case "symlink":
				file = filepath.Join(root, "link")
				if err := os.Symlink(filepath.Join(root, "token"), file); err != nil {
					t.Fatal(err)
				}
			case "missing":
				file = filepath.Join(root, "absent")
			case "directory":
				file = root
			}
			cmd := exec.Command("bash", "-c", setupTokenFilePrelude+`printf %s "$PULSE_SETUP_TOKEN" > "$FAKE_CAPTURE"`)
			capture := filepath.Join(root, "native-mutation-marker")
			cmd.Env = append(os.Environ(), "PULSE_SETUP_TOKEN_FILE="+file, "FAKE_CAPTURE="+capture)
			out, err := cmd.CombinedOutput()
			if strings.Contains(string(out), body) {
				t.Fatal("invalid secret echoed")
			}
			if tc == "valid" {
				if err != nil {
					t.Fatalf("valid file: %v %s", err, out)
				}
				data, e := os.ReadFile(capture)
				if e != nil || string(data) != token {
					t.Fatal("private input lost")
				}
			} else {
				if err == nil {
					t.Fatal("unsafe file accepted")
				}
				if _, e := os.Stat(capture); !os.IsNotExist(e) {
					t.Fatal("native action followed unsafe input")
				}
			}
		})
	}
}

func TestPrivateBootstrapShellQuotesDataWithoutPuttingTokenInSource(t *testing.T) {
	root, env := bootstrapFixture(t, recordingInstaller)
	injected := filepath.Join(root, "must-not-exist")
	base := `https://pulse.example/' ; touch ` + injected + ` ; #`
	token := `tok'en-with-shell-' ; touch ` + injected
	command := BuildProxmoxAgentInstallCommand(AgentInstallCommandOptions{BaseURL: base, Token: token, InstallType: "pbs", IncludeInstallType: true})
	if strings.Contains(command, token) {
		t.Fatal("credential interpolated")
	}
	out, err := runBootstrap(t, command, token, env, true)
	if err != nil {
		t.Fatalf("escaped bootstrap failed: %v %s", err, out)
	}
	args, err := os.ReadFile(filepath.Join(root, "argv"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(args), "--url\n"+base+"\n") || !strings.Contains(string(args), "--proxmox-type\npbs\n") {
		t.Fatal("quoted arguments changed")
	}
	if _, err := os.Stat(injected); !os.IsNotExist(err) {
		t.Fatal("argument was executed as shell source")
	}
}

func TestPrivateBootstrapInteractiveHistoryContainsCommandButNotToken(t *testing.T) {
	root, env := bootstrapFixture(t, recordingInstaller)
	token := strings.Repeat("d", 32)
	command := BuildProxmoxAgentInstallCommand(AgentInstallCommandOptions{BaseURL: "https://pulse.example", Token: token, InstallType: "pve", IncludeInstallType: true})
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
		t.Fatalf("interactive bootstrap: %v %s", err, out)
	}
	hist, err := os.ReadFile(history)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(hist), command) {
		t.Fatal("history control did not record the copied command")
	}
	if strings.Contains(string(hist), token) || strings.Contains(string(out), token) {
		t.Fatal("silent input entered shell history or terminal output")
	}
}

// A complete, matching prompt printed before private input is ready must not
// release the fixture's credential. This makes the echoed-command chunk race
// deterministic, without relaxing the real output/history/argv leak checks.
func TestPrivateBootstrapPTYWaitsForSilentPrompt(t *testing.T) {
	root, env := bootstrapFixture(t, recordingInstaller)
	token := strings.Repeat("g", 32)
	command := BuildProxmoxAgentInstallCommand(AgentInstallCommandOptions{BaseURL: "https://pulse.example", Token: token, InstallType: "pve", IncludeInstallType: true})
	prompt := "Pulse agent token (paste at this prompt, not in the command): "
	// The fixture sees a prompt-shaped write while ECHO is still enabled, then
	// the actual bootstrap downloads/preflights and owns the private prompt.
	command = "printf %s " + posixShellQuote(prompt) + "; sleep 0.2; " + command
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
		t.Fatalf("private prompt bootstrap: %v\n%s", err, out)
	}
	hist, err := os.ReadFile(history)
	if err != nil || !strings.Contains(string(hist), command) {
		t.Fatal("history control did not record the copied command")
	}
	args, err := os.ReadFile(filepath.Join(root, "argv"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), token) || strings.Contains(string(hist), token) || strings.Contains(string(args), token) {
		t.Fatal("PTY fixture sent credential before the silent prompt")
	}
	captured, err := os.ReadFile(filepath.Join(root, "captured-token"))
	if err != nil || string(captured) != token {
		t.Fatal("private prompt did not receive the complete credential")
	}
	assertContainerBootstrapCleanup(t, root)
}

func TestPrivateBootstrapPTYWaitsForCallerBeforeExit(t *testing.T) {
	installer := strings.Replace(recordingInstaller, `exit "${FAKE_INSTALL_EXIT:-0}"`, `
if IFS= read -r -t 0.2 </dev/tty; then
    echo "PTY fixture queued shell input while installer owns terminal" >&2
    exit 55
fi
exit "${FAKE_INSTALL_EXIT:-0}"`, 1)
	root, env := bootstrapFixture(t, installer)
	token := strings.Repeat("h", 32)
	command := BuildProxmoxAgentInstallCommand(AgentInstallCommandOptions{BaseURL: "https://pulse.example", Token: token, InstallType: "pve", IncludeInstallType: true})
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
		t.Fatalf("caller hand-off bootstrap: %v\n%s", err, out)
	}
	hist, err := os.ReadFile(history)
	if err != nil || !strings.Contains(string(hist), command) || !strings.HasSuffix(string(hist), "exit\n") {
		t.Fatal("calling shell did not receive the command and subsequent exit")
	}
	if strings.Contains(string(out), token) || strings.Contains(string(hist), token) {
		t.Fatal("private input reached terminal output or history")
	}
	assertContainerBootstrapCleanup(t, root)
}

// The public server installer is not the telemetry installer. If it is served
// accidentally at the agent path, its real parser rejects --url before token
// input or privileged installation; a complete but wrong fetch is not success.
func TestPrivateAgentBootstrapRejectsActualServerInstaller(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "..", "..", "install.sh"))
	if err != nil {
		t.Fatal(err)
	}
	root, env := bootstrapFixture(t, string(source))
	command := BuildProxmoxAgentInstallCommand(AgentInstallCommandOptions{BaseURL: "https://pulse.example", Token: "synthetic-token", InstallType: "pve", IncludeInstallType: true})
	out, err := runBootstrap(t, command, "", env, false)
	if err == nil || !strings.Contains(string(out), "Unknown option: --url") {
		t.Fatal("wrong installer was not rejected by its real parser")
	}
	dirs, _ := os.ReadFile(filepath.Join(root, "private-directories"))
	for _, path := range strings.Fields(string(dirs)) {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatal("wrong-installer failure left a private directory")
		}
	}
}
