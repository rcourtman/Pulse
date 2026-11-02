package installtests

import (
	"context"
	"crypto/sha256"
	"encoding/pem"
	"fmt"
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
)

// Model tr implementations that treat POSIX classes as literal characters.
// This is deliberately not an assertion about the reporter's native QNAP tr.
func classlessTREnvironment(t *testing.T) []string {
	t.Helper()
	tr, err := exec.LookPath("tr")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	wrapper := `#!/usr/bin/env python3
import os, sys
args = sys.argv[1:]
for name in ("upper", "lower", "space"):
    args = [arg.replace("[:" + name + ":]", "\\[:" + name + ":\\]") for arg in args]
os.execv(` + fmt.Sprintf("%q", tr) + `, ["tr"] + args)
`
	if err := os.WriteFile(filepath.Join(dir, "tr"), []byte(wrapper), 0700); err != nil {
		t.Fatal(err)
	}
	env := append(os.Environ(), "PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	// Admit the model only when it reproduces the original corruption. A
	// passing control using an ordinary class-aware tr would prove nothing.
	cmd := exec.Command("bash", "-c", `printf https:// | tr '[:upper:]' '[:lower:]'`)
	cmd.Env = env
	if out, err := cmd.CombinedOutput(); err != nil || string(out) != "httws://" {
		t.Fatalf("classless tr control: %v, %q", err, out)
	}
	return env
}

func TestAgentInstallerPortableTRPreflight(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix installer")
	}
	if _, err := exec.LookPath("openssl"); err != nil {
		t.Fatal("installer fingerprint prerequisite openssl missing")
	}
	classless := classlessTREnvironment(t)
	for _, tc := range []struct {
		name, scheme, pin string
		classless         bool
		wantExit          int
	}{
		{"ordinary-https", "https", "correct", false, 0},
		{"classless-https", "https", "correct", true, 0},
		{"classless-mixed-https", "hTtPs", "correct", true, 0},
		{"classless-malformed-url", "missing", "none", true, 1},
		{"classless-wrong-pin", "https", "wrong", true, 14},
		{"classless-malformed-pin", "https", "malformed", true, 14},
		{"classless-http-pin-despite-insecure", "http", "correct", true, 14},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var mu sync.Mutex
			var requests []string
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				requests = append(requests, r.Method+" "+r.URL.RequestURI())
				mu.Unlock()
				switch r.URL.Path {
				case "/api/health":
					w.WriteHeader(http.StatusOK)
				case "/download/pulse-agent":
					w.Header().Set("X-Checksum-Sha256", strings.Repeat("a", 64))
					w.WriteHeader(http.StatusOK)
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			dir := t.TempDir()
			caPath := filepath.Join(dir, "saved-ca.pem")
			ca := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})
			saved := map[string][]byte{
				"saved-ca.pem": ca, "agent-id": []byte("saved-id\n"),
				"runtime.token":  []byte("synthetic-runtime-credential\n"),
				"connection.env": []byte("saved connection sentinel\n"),
				"pulse-agent":    []byte("installed predecessor sentinel\n"),
			}
			for name, content := range saved {
				if err := os.WriteFile(filepath.Join(dir, name), content, 0600); err != nil {
					t.Fatal(err)
				}
			}
			url := strings.Replace(server.URL, "https", tc.scheme, 1)
			if tc.scheme == "missing" {
				url = "pulse.invalid"
			}
			args := []string{repoFile("scripts", "install.sh"), "--url", url, "--cacert", caPath,
				"--state-dir", dir, "--preflight-only", "--output", "json", "--non-interactive",
				"--disable-docker", "--disable-kubernetes", "--disable-proxmox"}
			if tc.pin != "none" {
				pin := fmt.Sprintf("%X", sha256.Sum256(server.Certificate().Raw))
				if tc.pin == "wrong" {
					pin = strings.Repeat("0", 64)
				} else if tc.pin == "malformed" {
					pin = strings.Repeat("Z", 64)
				}
				var pairs []string
				for i := 0; i < len(pin); i += 2 {
					pairs = append(pairs, pin[i:i+2])
				}
				args = append(args, "--server-fingerprint", " \t"+strings.Join(pairs, ":")+"\r\n\v\f")
			}
			if tc.scheme == "http" {
				args = append(args, "--insecure")
			}
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, "bash", args...)
			if tc.classless {
				cmd.Env = classless
			}
			out, err := cmd.CombinedOutput()
			exit := 0
			if err != nil {
				if failure, ok := err.(*exec.ExitError); ok {
					exit = failure.ExitCode()
				} else {
					t.Fatal(err)
				}
			}
			if exit != tc.wantExit || ctx.Err() != nil {
				t.Fatalf("preflight exit=%d want=%d timeout=%v\n%s", exit, tc.wantExit, ctx.Err(), out)
			}
			mu.Lock()
			observed := append([]string(nil), requests...)
			mu.Unlock()
			if tc.wantExit == 0 {
				want := "HEAD /download/pulse-agent?arch=" + runtime.GOOS + "-" + runtime.GOARCH
				if strings.Join(observed, "\n") != "GET /api/health\n"+want ||
					!strings.Contains(string(out), `"code":"ok"`) {
					t.Fatalf("wrong OS/download/preflight result: %v\n%s", observed, out)
				}
			} else if len(observed) != 0 || strings.Contains(string(out), `"code":"ok"`) {
				t.Fatalf("refused URL/pin reached HTTP or claimed success: %v\n%s", observed, out)
			}
			for name, want := range saved {
				path := filepath.Join(dir, name)
				got, err := os.ReadFile(path)
				info, statErr := os.Stat(path)
				if err != nil || statErr != nil || string(got) != string(want) || info.Mode().Perm() != 0600 {
					t.Fatalf("preflight mutated saved %s", name)
				}
			}
			if entries, err := os.ReadDir(dir); err != nil || len(entries) != len(saved) {
				t.Fatalf("preflight added saved state: %v %v", entries, err)
			}
		})
	}
}

func TestAgentInstallerPortableTRIdentityAndTransport(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix installer")
	}
	script := "set -euo pipefail\n"
	for _, name := range []string{"resolve_safe_profile_hostname", "pulse_url_uses_plain_http",
		"action_runner_url_uses_loopback_http", "action_runner_url_transport_allowed",
		"generate_action_runner_activation_nonce", "systemd_effective_words_equal"} {
		script += extractInstallShellFunction(t, name) + "\n"
	}
	script += `
HOSTNAME_OVERRIDE='NODE-UPPER.Example.'
resolve_safe_profile_hostname
[[ "$HOSTNAME_OVERRIDE" == node-upper.example ]]
pulse_url_uses_plain_http 'HtTp://pulse.invalid'
! pulse_url_uses_plain_http 'HTTPS://pulse.invalid'
action_runner_url_transport_allowed 'HTTPS://pulse.invalid'
action_runner_url_transport_allowed 'HtTp://LOCALHOST:7655'
action_runner_url_transport_allowed 'http://127.2.3.4:7655'
action_runner_url_transport_allowed 'http://[::1]:7655'
for url in 'http://pulse.invalid' 'http://192.168.1.2' 'http://127.999.1.2' \
    'https://user@pulse.invalid' 'https://pulse.invalid/with space' 'pulse.invalid'; do
    ! action_runner_url_transport_allowed "$url"
done
nonce=$(generate_action_runner_activation_nonce)
[[ "$nonce" =~ ^[a-f0-9]{64}$ ]]
systemd_effective_unit_property() { printf 'unix\tinet\r\ninet6\vunix\f'; }
systemd_effective_words_equal unit RestrictAddressFamilies inet6 unix inet
! systemd_effective_words_equal unit RestrictAddressFamilies unix inet
printf 'portable identity, nonce, word-set and transport controls passed\n'
`
	cmd := exec.Command("bash", "-c", script)
	cmd.Env = classlessTREnvironment(t)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("classless downstream controls: %v\n%s", err, out)
	}
}
