//go:build linux

package installtests

import (
	"bytes"
	"context"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// Run the shipped functions with real curl and SSH verification. Only their
// release URLs and signing key are synthetic. No installer is executed and no
// guest, service, production endpoint or installed state is touched.
func TestRootInstallerDownloadCurlConfigIsolation(t *testing.T) {
	assets := t.TempDir()
	key := filepath.Join(assets, "key")
	payload := []byte("inert transport fixture; never executed\n")
	file := filepath.Join(assets, "payload")
	writeTestFile(t, file, payload, 0600)
	for _, args := range [][]string{
		{"-q", "-t", "ed25519", "-N", "", "-f", key},
		{"-q", "-Y", "sign", "-f", key, "-n", "pulse-install", file},
	} {
		if out, err := exec.Command("ssh-keygen", args...).CombinedOutput(); err != nil {
			t.Fatalf("synthetic signature: %v\n%s", err, out)
		}
	}
	public, err := os.ReadFile(key + ".pub")
	if err != nil {
		t.Fatal(err)
	}
	signature, err := os.ReadFile(file + ".sig")
	if err != nil {
		t.Fatal(err)
	}
	for _, phase := range []string{"installer", "helper"} {
		for _, noTimeout := range []bool{false, true} {
			for _, mode := range []string{"trusted", "untrusted-ca", "http-error", "redirect", "bad-signature", "environment-proxy"} {
				t.Run(fmt.Sprintf("%s/no-timeout-%t/%s", phase, noTimeout, mode), func(t *testing.T) {
					root := t.TempDir()
					var mu sync.Mutex
					var requests []string
					injected := false
					handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						mu.Lock()
						requests = append(requests, r.Method+" "+r.URL.Path)
						injected = injected || r.Header.Get("X-Curlrc-Injected") != "" || r.Header.Get("Authorization") != ""
						mu.Unlock()
						if mode == "http-error" {
							http.Error(w, "synthetic unavailability", http.StatusServiceUnavailable)
							return
						}
						if mode == "redirect" && !strings.HasPrefix(r.URL.Path, "/cdn/") {
							http.Redirect(w, r, "/cdn"+r.URL.Path, http.StatusTemporaryRedirect)
							return
						}
						if strings.HasSuffix(r.URL.Path, ".sshsig") {
							if mode == "bad-signature" {
								fmt.Fprint(w, "invalid synthetic signature\n")
							} else {
								_, _ = w.Write(signature)
							}
						} else {
							_, _ = w.Write(payload)
						}
					})
					server := httptest.NewTLSServer(handler)
					defer server.Close()
					origin := server.URL
					ca := filepath.Join(root, "ca.pem")
					writeTestFile(t, ca, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}), 0600)
					proxyEnv := []string{}
					if mode == "environment-proxy" {
						proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
							if r.URL.Host != "server-installer.fixture.invalid" || r.URL.Scheme != "http" {
								t.Errorf("unexpected proxy destination: %s", r.URL)
								http.Error(w, "unexpected destination", http.StatusBadGateway)
								return
							}
							handler.ServeHTTP(w, r)
						}))
						defer proxy.Close()
						origin = "http://server-installer.fixture.invalid"
						proxyEnv = []string{"http_proxy=" + proxy.URL, "NO_PROXY=", "no_proxy="}
					}
					trace := filepath.Join(root, "trace")
					config := "insecure\nretry = 1\nretry-delay = 0\nheader = \"X-Curlrc-Injected: synthetic\"\n" +
						"trace-ascii = \"" + trace + "\"\nurl = \"" + origin + "/ambient\"\n"
					saved := map[string][]byte{
						".curlrc": []byte(config), "system.json": []byte("saved configuration\n"),
						"pulse": []byte("saved binary\n"), "token": []byte("saved synthetic token\n"),
					}
					for name, body := range saved {
						writeTestFile(t, filepath.Join(root, name), body, 0600)
					}
					script := `source "$ROOT_INSTALLER_SOURCE"
PINNED_RELEASE_SSH_PUBLIC_KEY="$FIXTURE_PUBLIC_KEY"
print_info() { :; }
print_warn() { :; }
print_error() { :; }
# Keep script-owned retry counts, without waiting through fixture backoff.
sleep() { :; }
command() {
    if [[ "$*" == '-v timeout' && "$NO_TIMEOUT" == true ]]; then return 1; fi
    builtin command "$@"
}
resolve_install_script_download_url() { printf '%s/install.sh\n' "$FIXTURE_ORIGIN"; }
resolve_release_asset_base_url() { printf '%s\n' "$FIXTURE_ORIGIN"; }
if [[ "$PHASE" == installer ]]; then
    selected=''; staging=''
    trap 'cleanup_container_install_inputs "$staging" "" false' EXIT
    if download_container_installer selected staging; then
        [[ "$selected" == "$staging/install.sh" ]] || exit 91
        cp "$selected" "$FIXTURE_ROOT/accepted"
    else
        [[ -z "$selected" ]] || exit 92
        exit 23
    fi
else
    AUTO_UPDATE_DEST="$FIXTURE_ROOT/fetched-helper"
    if download_auto_update_script; then
        [[ -x "$AUTO_UPDATE_DEST" ]] || exit 93
        cp "$AUTO_UPDATE_DEST" "$FIXTURE_ROOT/accepted"
    else
        exit 23
    fi
fi
`
					env := rootCurlFixtureEnvironment(t, root, origin)
					env = append(env, "PHASE="+phase, fmt.Sprintf("NO_TIMEOUT=%t", noTimeout),
						"FIXTURE_PUBLIC_KEY="+strings.TrimSpace(string(public)), "CURL_CA_BUNDLE="+ca, "SSL_CERT_FILE="+ca)
					if mode == "untrusted-ca" {
						// The standard store does not trust this ephemeral test CA.
						env = append(env, "CURL_CA_BUNDLE=", "SSL_CERT_FILE=")
					}
					env = append(env, proxyEnv...)
					exit := runRootCurlFixture(t, script, env)
					accepted, readErr := os.ReadFile(filepath.Join(root, "accepted"))
					ok := mode != "untrusted-ca" && mode != "http-error" && mode != "bad-signature"
					if ok {
						if exit != 0 || readErr != nil || !bytes.Equal(accepted, payload) {
							t.Fatalf("signed transport rejected: exit=%d read=%v payload=%q", exit, readErr, accepted)
						}
					} else if exit != 23 || !os.IsNotExist(readErr) {
						t.Fatalf("failed transfer/signature admitted: exit=%d read=%v payload=%q", exit, readErr, accepted)
					}
					mu.Lock()
					got := append([]string(nil), requests...)
					hasInjected := injected
					mu.Unlock()
					want := 2
					if mode == "untrusted-ca" {
						want = 0
					} else if mode == "http-error" {
						want = 3 // Existing script-owned attempts, not curlrc retries.
					} else if mode == "redirect" {
						want = 4
					} else if mode == "bad-signature" && phase == "helper" {
						want = 6 // Three existing signed-pair attempts, never acceptance.
					}
					if len(got) != want || hasInjected || strings.Contains(strings.Join(got, "\n"), "/ambient") {
						t.Errorf("implicit requests/headers/retries: %q injected=%t, want %d requests", got, hasInjected, want)
					}
					assertRootCurlFixtureUnchanged(t, root, trace, saved)
					entries, err := os.ReadDir(filepath.Join(root, "temporary"))
					if err != nil || len(entries) != 0 {
						t.Errorf("owned staging/signature inputs survived: %v %v", entries, err)
					}
				})
			}
		}
	}
}

// The existing PVE smoke operation deliberately opts into -k and sends its
// token through stdin. Disabling curlrc must preserve both explicit choices,
// while preventing implicit tracing, headers and extra destinations.
func TestRootInstallerTokenSmokeCurlConfigIsolation(t *testing.T) {
	root := t.TempDir()
	var mu sync.Mutex
	var requests []string
	const authorization = "PVEAPIToken=synthetic@pve!monitor=synthetic-secret"
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests = append(requests, r.Method+" "+r.URL.Path)
		mu.Unlock()
		if r.URL.Path != "/api2/json/nodes" || r.Header.Get("Authorization") != authorization || r.Header.Get("X-Curlrc-Injected") != "" {
			t.Errorf("unexpected synthetic credential request: path=%q injected=%t", r.URL.Path, r.Header.Get("X-Curlrc-Injected") != "")
		}
		fmt.Fprint(w, `{"data":[]}`)
	}))
	defer server.Close()
	trace := filepath.Join(root, "trace")
	saved := map[string][]byte{".curlrc": []byte("header = \"X-Curlrc-Injected: synthetic\"\ntrace-ascii = \"" + trace + "\"\nurl = \"" + server.URL + "/ambient\"\n")}
	writeTestFile(t, filepath.Join(root, ".curlrc"), saved[".curlrc"], 0600)
	exit := runRootCurlFixture(t, `source "$ROOT_INSTALLER_SOURCE"
smoke_test_pve_auto_register_token "$FIXTURE_ORIGIN" 'synthetic@pve!monitor' 'synthetic-secret'
`, rootCurlFixtureEnvironment(t, root, server.URL))
	mu.Lock()
	got := strings.Join(requests, "\n")
	mu.Unlock()
	if exit != 0 || got != "GET /api2/json/nodes" {
		t.Errorf("explicit PVE transport changed: exit=%d requests=%q", exit, got)
	}
	assertRootCurlFixtureUnchanged(t, root, trace, saved)
}

// Exercise the two shipped POST assignments, without invoking the surrounding
// host/guest/ACL mutation. Keep their real payload mechanism and retry policy;
// the function's other validation and state transitions are not claimed here.
func TestRootInstallerRegistrationCurlConfigIsolation(t *testing.T) {
	content, err := os.ReadFile(rootCurlFixtureSource(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, phase := range []string{"setup", "register"} {
		prefix := "if ! " + phase + "_response=$("
		assignment := ""
		for _, line := range strings.Split(string(content), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, prefix) && strings.HasSuffix(line, "; then") {
				if assignment != "" {
					t.Fatal("duplicate registered transport assignment: ", phase)
				}
				assignment = strings.TrimSuffix(strings.TrimPrefix(line, "if ! "), "; then")
			}
		}
		if assignment == "" {
			t.Fatal("missing registered transport assignment: ", phase)
		}
		for _, unavailable := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/unavailable-%t", phase, unavailable), func(t *testing.T) {
				root := t.TempDir()
				path := "/api/setup-script-url"
				if phase == "register" {
					path = "/api/auto-register"
				}
				const body = `{"synthetic":"private fixture token"}`
				var mu sync.Mutex
				var requests []string
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					payload, err := io.ReadAll(r.Body)
					mu.Lock()
					requests = append(requests, r.Method+" "+r.URL.Path)
					mu.Unlock()
					if err != nil || r.Method != "POST" || r.URL.Path != path || string(payload) != body ||
						r.Header.Get("Content-Type") != "application/json" || r.Header.Get("X-Curlrc-Injected") != "" {
						t.Errorf("registered transport changed or config added a request/header: method=%q path=%q read=%v", r.Method, r.URL.Path, err)
					}
					if unavailable {
						http.Error(w, "synthetic unavailability", http.StatusServiceUnavailable)
					} else {
						fmt.Fprint(w, `{"synthetic":"private response"}`)
					}
				}))
				defer server.Close()
				trace := filepath.Join(root, "trace")
				saved := map[string][]byte{".curlrc": []byte("header = \"X-Curlrc-Injected: synthetic\"\ntrace-ascii = \"" + trace + "\"\nurl = \"" + server.URL + "/ambient\"\n")}
				writeTestFile(t, filepath.Join(root, ".curlrc"), saved[".curlrc"], 0600)
				env := append(rootCurlFixtureEnvironment(t, root, server.URL), "FIXTURE_PAYLOAD="+body)
				exit := runRootCurlFixture(t, "set -euo pipefail\npulse_url=\"$FIXTURE_ORIGIN\"\nsetup_payload=\"$FIXTURE_PAYLOAD\"\nregister_payload=\"$FIXTURE_PAYLOAD\"\n"+assignment+"\n", env)
				mu.Lock()
				got := append([]string(nil), requests...)
				mu.Unlock()
				want, wantExit := 1, 0
				if unavailable {
					want, wantExit = 4, 22 // The existing explicit --retry 3 and -f.
				}
				if exit != wantExit || len(got) != want {
					t.Errorf("registered transport exit=%d requests=%q, want exit=%d requests=%d", exit, got, wantExit, want)
				}
				assertRootCurlFixtureUnchanged(t, root, trace, saved)
			})
		}
	}
}

func TestRootInstallerEveryCurlCallDisablesDefaultConfigFirst(t *testing.T) {
	content, err := os.ReadFile(rootCurlFixtureSource(t))
	if err != nil {
		t.Fatal(err)
	}
	// This deliberately includes timeout, pipe, array and elif call shapes.
	// A first URL/variable instead of flags is a new unisolated call too.
	call := regexp.MustCompile(`\bcurl\s+(-\S+|["'$]\S+|https?://\S+)`)
	count := 0
	for _, line := range strings.Split(string(content), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "echo ") {
			continue
		}
		for _, match := range call.FindAllStringSubmatch(line, -1) {
			count++
			if match[1] != "-q" {
				t.Errorf("curl must use standalone first-option -q, including on legacy hosts: %s", line)
			}
		}
	}
	if count != 18 {
		t.Fatalf("curl call inventory changed: got %d, want 18; review every new/deleted transfer", count)
	}
}

func rootCurlFixtureSource(t *testing.T) string {
	t.Helper()
	source := os.Getenv("PULSE_INSTALLER_UNDER_TEST")
	if source == "" {
		source = repoFile("install.sh")
	}
	source, err := filepath.Abs(source)
	if err != nil {
		t.Fatal(err)
	}
	return source
}

func rootCurlFixtureEnvironment(t *testing.T, root, origin string) []string {
	t.Helper()
	temporary := filepath.Join(root, "temporary")
	if err := os.Mkdir(temporary, 0700); err != nil {
		t.Fatal(err)
	}
	return append(os.Environ(), "ROOT_INSTALLER_SOURCE="+rootCurlFixtureSource(t),
		"HOME="+root, "CURL_HOME="+root, "XDG_CONFIG_HOME="+root, "TMPDIR="+temporary,
		"FIXTURE_ROOT="+root, "FIXTURE_ORIGIN="+origin,
		"PULSE_INSTALL_DIR="+root, "PULSE_CONFIG_DIR="+root,
		"http_proxy=", "https_proxy=", "HTTP_PROXY=", "HTTPS_PROXY=", "ALL_PROXY=", "all_proxy=",
		"CURL_CA_BUNDLE=", "SSL_CERT_FILE=", "NO_PROXY=*", "no_proxy=*")
}

func runRootCurlFixture(t *testing.T, script string, env []string) int {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bash", "-c", script)
	cmd.Env = env
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
	cmd.WaitDelay = 2 * time.Second
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("bounded curl fixture exceeded deadline: %v\n%s", ctx.Err(), out)
	}
	if err == nil {
		return 0
	}
	if exit, ok := err.(*exec.ExitError); ok {
		return exit.ExitCode()
	}
	t.Fatalf("curl fixture did not execute: %v\n%s", err, out)
	return -1
}

func assertRootCurlFixtureUnchanged(t *testing.T, root, trace string, saved map[string][]byte) {
	t.Helper()
	if _, err := os.Stat(trace); !os.IsNotExist(err) {
		t.Errorf("implicit curl trace created or stat failed: %v", err)
	}
	for name, want := range saved {
		got, err := os.ReadFile(filepath.Join(root, name))
		if err != nil || !bytes.Equal(got, want) {
			t.Errorf("saved fixture %s changed: %v", name, err)
		}
	}
}
