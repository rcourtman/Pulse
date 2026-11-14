//go:build linux

package installtests

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// Exercise the production registration function with actual curl and loopback
// HTTP, not real Proxmox credentials, services, ACLs, guests or enrollment.
// Credential creation is an inert provider adapter; it does not qualify pveum.
func TestRootInstallAutoRegisterTransport(t *testing.T) {
	for _, tc := range []struct {
		name, stage, fault, wantError string
		wantRegistered                bool
	}{
		{"success", "", "", "", true},
		{"setup-http-error", "setup", "http", "setup token request failed", false},
		{"smoke-http-error", "smoke", "http", "token smoke check failed", false},
		{"register-http-error", "register", "http", "auto-register request failed", false},
		{"setup-stall", "setup", "stall", "setup token request failed", false},
		{"smoke-stall", "smoke", "stall", "token smoke check failed", false},
		{"register-stall", "register", "stall", "auto-register request failed", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			var mu sync.Mutex
			var stages []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				origin := "http://" + r.Host
				stage := map[string]string{"/api/setup-script-url": "setup", "/api2/json/nodes": "smoke", "/api/auto-register": "register"}[r.URL.Path]
				mu.Lock()
				stages = append(stages, stage)
				mu.Unlock()
				if stage == "" {
					t.Errorf("unexpected fixture request %s", r.URL.Path)
					http.Error(w, "unexpected route", 400)
					return
				}
				if stage == "smoke" && (r.Method != "GET" || r.Header.Get("Authorization") != "PVEAPIToken=pulse-monitor@pve!pulse-127-0-0-1=fixture-pve-secret") {
					t.Error("smoke check did not use the exact created credential")
				}
				if stage != "smoke" && r.Method != "POST" {
					t.Errorf("%s method = %s", stage, r.Method)
				}
				if stage == tc.stage {
					if tc.fault == "stall" {
						// Send no headers at all: a connection timeout alone cannot
						// bound this peer. curl must cancel the accepted request.
						_, _ = io.Copy(io.Discard, r.Body)
						<-r.Context().Done()
						return
					}
					http.Error(w, "fixture rejection", 400)
					return
				}
				switch stage {
				case "setup":
					setupURL := origin + "/api/setup-script?backup_perms=true&host=" + url.QueryEscape(origin) + "&pulse_url=" + url.QueryEscape(origin) + "&type=pve"
					command := "curl '" + setupURL + "' | { if [ \"$(id -u)\" -eq 0 ]; then env PULSE_SETUP_TOKEN_FILE=/fixture/token bash; elif command -v sudo >/dev/null 2>&1; then sudo env PULSE_SETUP_TOKEN_FILE=/fixture/token bash; fi; }"
					_ = json.NewEncoder(w).Encode(map[string]any{
						"setupToken": "fixture-setup-secret", "type": "pve", "host": origin,
						"url": setupURL, "downloadURL": setupURL, "scriptFileName": "pulse-setup-pve.sh",
						"command": command, "commandWithEnv": command, "commandWithoutEnv": command,
						"tokenHint": "fixture-hint", "expires": time.Now().Add(5 * time.Minute).Unix(),
					})
				case "smoke":
					_, _ = w.Write([]byte(`{"data":[]}`))
				case "register":
					var payload map[string]string
					if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
						t.Error(err)
					}
					if payload["host"] != origin || payload["authToken"] != "fixture-setup-secret" || payload["tokenValue"] != "fixture-pve-secret" {
						t.Error("registration did not retain the admitted setup and monitoring identities")
					}
					_ = json.NewEncoder(w).Encode(map[string]string{
						"status": "success", "action": "use_token", "type": "pve", "source": "script",
						"host": origin, "tokenId": payload["tokenId"], "tokenValue": payload["tokenValue"],
						"nodeId": "fixture-pve", "nodeName": "fixture-pve",
					})
				}
			}))
			defer server.Close()
			origin := server.URL
			_, port, err := net.SplitHostPort(strings.TrimPrefix(origin, "http://"))
			if err != nil {
				t.Fatal(err)
			}
			curl, err := exec.LookPath("curl")
			if err != nil {
				t.Fatal(err)
			}
			// A hostile ambient config would break every successful request if
			// -q stopped being curl's first argument.
			if err := os.WriteFile(filepath.Join(dir, ".curlrc"), []byte("proxy = http://127.0.0.1:1\nnoproxy = \"\"\n"), 0600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, "bash", "-c", `
source "$INSTALLER"
print_info() { :; }
print_success() { :; }
print_warn() { printf '%s\n' "$*" >&2; }
hostname() { printf 'fixture-pve\n'; }
pveum() {
    printf '%s %s\n' "$1" "${2:-}" >>"$FIXTURE/provider-calls"
    if [[ "$*" == 'role list' ]]; then
        printf 'Sys.Audit\nVM.GuestAgent.Audit\nVM.GuestAgent.FileRead\n'
    fi
}
create_pve_auto_register_token() {
    printf -v "$2" '%s' '{"value":"fixture-pve-secret"}'
    printf -v "$3" '%s' 0
}
curl() {
    printf '%s\t' "$@" >>"$FIXTURE/curl-args"
    printf '\n' >>"$FIXTURE/curl-args"
    "$REAL_CURL" "$@"
}
IN_CONTAINER=false
PULSE_PVE_HOST_URL="$ORIGIN"
PULSE_AUTO_BACKUP_PERMS=true
AUTO_NODE_REGISTERED=false
AUTO_NODE_REGISTERED_NAME=unchanged
AUTO_NODE_REGISTER_ERROR=
auto_register_pve_node 600 127.0.0.1 "$PORT"
printf '\nRESULT:%s:%s:%s\n' "$AUTO_NODE_REGISTERED" "$AUTO_NODE_REGISTERED_NAME" "$AUTO_NODE_REGISTER_ERROR"
`)
			cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
			cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
			cmd.WaitDelay = time.Second
			installer := repoFile("install.sh")
			// The explicit baseline input lets offline proof run these same
			// controls against the containing commit's parent without editing it.
			if before := os.Getenv("PULSE_INSTALLER_UNDER_TEST"); before != "" {
				installer = before
			}
			cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + dir, "LC_ALL=C", "NO_PROXY=127.0.0.1", "FIXTURE=" + dir, "INSTALLER=" + installer, "REAL_CURL=" + curl, "ORIGIN=" + origin, "PORT=" + port}
			started := time.Now()
			output, err := cmd.CombinedOutput()
			elapsed := time.Since(started)
			if err != nil {
				t.Fatalf("registration did not return to manual completion: %v, elapsed=%s\n%s", err, elapsed, output)
			}
			name := "unchanged"
			if tc.wantRegistered {
				name = "fixture-pve"
			}
			want := fmt.Sprintf("RESULT:%t:%s:%s", tc.wantRegistered, name, tc.wantError)
			if !strings.Contains(string(output), want) {
				t.Fatalf("want %s in %s", want, output)
			}
			if strings.Contains(string(output), "fixture-setup-secret") || strings.Contains(string(output), "fixture-pve-secret") {
				t.Fatal("credential appeared in installer diagnostics")
			}
			args, err := os.ReadFile(filepath.Join(dir, "curl-args"))
			if err != nil {
				t.Fatal(err)
			}
			for _, line := range strings.Split(strings.TrimSpace(string(args)), "\n") {
				argv := strings.Split(strings.TrimSuffix(line, "\t"), "\t")
				if argv[0] != "-q" {
					t.Fatal("curl default configuration was not disabled first")
				}
				for option, value := range map[string]string{"--connect-timeout": "5", "--max-time": "10", "--retry-max-time": "20"} {
					count := 0
					for i, arg := range argv {
						if arg == option {
							count++
							if i+1 == len(argv) || argv[i+1] != value {
								t.Fatalf("%s missing its fixed bound", option)
							}
						}
					}
					if count != 1 {
						t.Fatalf("%s occurs %d times", option, count)
					}
				}
			}
			mu.Lock()
			observed := append([]string(nil), stages...)
			mu.Unlock()
			if tc.stage == "setup" {
				if _, err := os.Stat(filepath.Join(dir, "provider-calls")); !os.IsNotExist(err) {
					t.Fatal("setup failure reached the credential provider")
				}
			}
			for _, stage := range observed {
				if (tc.stage == "setup" && stage != "setup") || (tc.stage == "smoke" && stage == "register") {
					t.Fatalf("failed %s continued to %s", tc.stage, stage)
				}
			}
			if tc.wantRegistered && strings.Join(observed, ",") != "setup,smoke,register" {
				t.Fatalf("registration order: %v", observed)
			}
			if tc.fault == "stall" && (elapsed < 10*time.Second || elapsed > 32*time.Second) {
				t.Fatalf("actual accepted-connection stall not bounded: %s", elapsed)
			}
			t.Logf("elapsed=%s requests=%v result=%s", elapsed.Round(time.Millisecond), observed, want)
		})
	}
}
