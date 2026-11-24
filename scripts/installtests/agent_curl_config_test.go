//go:build linux

package installtests

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
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

// Exercise the actual version/download statements and complete typed-artifact
// functions with real curl. All peers, files and credentials are synthetic.
// Typed artifacts deliberately omit checksum headers, so even a successful
// transport must stop at the existing admission check, before executable use.
// This is transport proof, not signature, service or installed-profile proof.
func TestAgentInstallerDownloadCurlConfigIsolation(t *testing.T) {
	for _, phase := range []string{"version", "agent", "helper", "runner"} {
		for _, mode := range []string{"trusted", "untrusted-ca", "http-error", "redirect", "explicit-insecure", "environment-ca", "environment-proxy", "pinned-public-key", "other-pinned-public-key"} {
			t.Run(phase+"/"+mode, func(t *testing.T) {
				// A verified --server-fingerprint leaves INSECURE=true, so the
				// pin is all that identifies the peer of these transfers.
				pinned := strings.HasSuffix(mode, "pinned-public-key")
				root := t.TempDir()
				trace := filepath.Join(root, "ambient-trace")
				var mu sync.Mutex
				var requests []string
				var injected bool
				path := "/api/version"
				if phase != "version" {
					path = "/download/pulse-agent"
					if phase != "agent" {
						path += "-" + phase
					}
				}
				payload := []byte("\x7fELFinert transport fixture, never executable\n")
				handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					mu.Lock()
					requests = append(requests, r.Method+" "+r.URL.Path)
					injected = injected || r.Header.Get("X-Curlrc-Injected") != "" ||
						r.Header.Get("Authorization") != "" || r.Header.Get("X-API-Token") != ""
					mu.Unlock()
					if mode == "http-error" {
						http.Error(w, "fixture unavailable", http.StatusServiceUnavailable)
						return
					}
					if mode == "redirect" && r.URL.Path == path {
						http.Redirect(w, r, "/redirected", http.StatusTemporaryRedirect)
						return
					}
					if phase == "version" {
						fmt.Fprint(w, `{"version":"fixture-version"}`)
					} else {
						w.Write(payload)
					}
				})
				server := httptest.NewTLSServer(handler)
				defer server.Close()
				pulseURL := server.URL
				var proxyEnv []string
				if mode == "environment-proxy" {
					proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if r.URL.Host != "installer-proxy.fixture.invalid" || r.URL.Scheme != "http" {
							t.Errorf("unexpected proxy destination: %s", r.URL)
							http.Error(w, "unexpected target", http.StatusBadGateway)
							return
						}
						handler.ServeHTTP(w, r)
					}))
					defer proxy.Close()
					pulseURL = "http://installer-proxy.fixture.invalid"
					proxyEnv = []string{"http_proxy=" + proxy.URL, "NO_PROXY=", "no_proxy="}
				}
				ca := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})
				if mode == "untrusted-ca" || mode == "explicit-insecure" || pinned {
					// httptest TLS servers share a certificate; create a distinct
					// trust root rather than using another httptest server's CA.
					public, private, err := ed25519.GenerateKey(rand.Reader)
					if err != nil {
						t.Fatal(err)
					}
					template := &x509.Certificate{
						SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "unrelated installer fixture CA"},
						NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
						IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
					}
					der, err := x509.CreateCertificate(rand.Reader, template, template, public, private)
					if err != nil {
						t.Fatal(err)
					}
					ca = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
				}
				caPath := filepath.Join(root, "ca.pem")
				config := "insecure\nretry = 2\nretry-delay = 1\n" +
					"header = \"X-Curlrc-Injected: synthetic-sentinel\"\n" +
					"trace-ascii = \"" + trace + "\"\nurl = \"" + pulseURL + "/ambient\"\n"
				saved := map[string][]byte{
					"ca.pem": ca, ".curlrc": []byte(config), "token": []byte("saved synthetic token\n"),
					"agent-id": []byte("saved identity\n"), "pulse-agent": []byte("installed predecessor\n"),
					"connection.env": []byte("saved trusted connection\n"),
				}
				for name, body := range saved {
					writeTestFile(t, filepath.Join(root, name), body, 0600)
				}
				temporary := filepath.Join(root, "temporary")
				if err := os.Mkdir(temporary, 0700); err != nil {
					t.Fatal(err)
				}
				// Run only the shipped transfer statements, never main(), an
				// installer, a fixture executable, or a service-management tool.
				script := `set -euo pipefail
TMP_FILES=()
EXIT_DOWNLOAD_FAILED=11
EXIT_CHECKSUM_FAILED=12
EXIT_SIGNATURE_FAILED=13
log_info() { :; }
fail() { printf '%s\n' "$1" >&2; exit "$2"; }
TMP_BIN="$FIXTURE_ROOT/downloaded-agent"
TMP_HEADERS="$FIXTURE_ROOT/headers"
PRIVILEGED_HELPER_BINARY_NAME=pulse-agent-helper
ACTION_RUNNER_BINARY_NAME=pulse-agent-runner
BINARY_NAME=pulse-agent
ARCH_PARAM=linux-amd64
DOWNLOAD_QUERY=arch=linux-amd64
` + agentInstallerTransferStatements(t, phase)
				ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
				defer cancel()
				cmd := exec.CommandContext(ctx, "bash", "-c", script)
				cmd.Env = append(os.Environ(), "HOME="+root, "CURL_HOME="+root,
					"XDG_CONFIG_HOME="+root, "TMPDIR="+temporary, "FIXTURE_ROOT="+root,
					"http_proxy=", "https_proxy=", "HTTP_PROXY=", "HTTPS_PROXY=", "ALL_PROXY=", "all_proxy=",
					"NO_PROXY=*", "no_proxy=*", "PULSE_URL="+pulseURL, "INSECURE=false",
					"CURL_CA_BUNDLE="+caPath, "SSL_CERT_FILE="+caPath)
				if mode == "environment-ca" {
					// Omit explicit --cacert without changing production statements.
					// curl must retain the standard SSL_CERT_FILE environment CA.
					cmd.Env = append(cmd.Env, "CURL_CA_BUNDLE=")
				}
				if mode == "explicit-insecure" || pinned {
					cmd.Env = append(cmd.Env, "INSECURE=true")
				}
				if pinned {
					pin := sha256.Sum256(server.Certificate().RawSubjectPublicKeyInfo)
					if mode == "other-pinned-public-key" {
						pin = sha256.Sum256([]byte("public key of another server"))
					}
					cmd.Env = append(cmd.Env, "CURL_PINNED_PUBKEY=sha256//"+base64.StdEncoding.EncodeToString(pin[:]))
				}
				cmd.Env = append(cmd.Env, proxyEnv...)
				// A deadline regression must not strand curl after its shell exits.
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
				exit := 0
				if err != nil {
					if ee, ok := err.(*exec.ExitError); ok {
						exit = ee.ExitCode()
					} else {
						t.Fatal(err)
					}
				}
				transferOK := mode != "untrusted-ca" && mode != "http-error" && mode != "other-pinned-public-key"
				wantExit := 0
				if phase != "version" {
					if !transferOK {
						wantExit = 11
					} else if phase != "agent" {
						wantExit = 12 // The real missing-checksum admission check.
					}
				}
				if ctx.Err() != nil || exit != wantExit {
					t.Errorf("phase exit=%d want=%d observer=%v: %s", exit, wantExit, ctx.Err(), out)
				}
				if phase == "version" {
					want := "VERSION=\n"
					if transferOK {
						want = "VERSION=fixture-version\n"
					}
					if string(out) != want {
						t.Errorf("version result=%q want=%q", out, want)
					}
				} else if transferOK {
					staged := filepath.Join(root, "downloaded-agent")
					if phase != "agent" {
						// Each typed function allocates one body and one header file.
						entries, readErr := os.ReadDir(temporary)
						if readErr != nil || len(entries) != 2 {
							t.Fatalf("typed staging: %v %v", entries, readErr)
						}
						staged = ""
						for _, entry := range entries {
							p := filepath.Join(temporary, entry.Name())
							if body, _ := os.ReadFile(p); bytes.Equal(body, payload) {
								staged = p
							}
						}
					}
					if body, readErr := os.ReadFile(staged); readErr != nil || !bytes.Equal(body, payload) {
						t.Errorf("staged body differs: %v", readErr)
					}
				}
				mu.Lock()
				wantRequests := []string{"GET " + path}
				if mode == "untrusted-ca" || mode == "other-pinned-public-key" {
					wantRequests = nil
				} else if mode == "redirect" {
					wantRequests = append(wantRequests, "GET /redirected")
				}
				if strings.Join(requests, "\n") != strings.Join(wantRequests, "\n") || injected {
					t.Errorf("requests=%v want=%v injected-header=%v", requests, wantRequests, injected)
				}
				mu.Unlock()
				if _, err := os.Stat(trace); !os.IsNotExist(err) {
					t.Errorf("ambient trace created: %v", err)
				}
				for name, body := range saved {
					p := filepath.Join(root, name)
					actual, err := os.ReadFile(p)
					info, statErr := os.Stat(p)
					if err != nil || statErr != nil || !bytes.Equal(actual, body) || info.Mode().Perm() != 0600 {
						t.Errorf("saved %s changed: read=%v stat=%v", name, err, statErr)
					}
				}
				t.Logf("%s/%s exit=%d requests=%v", phase, mode, exit, requests)
			})
		}
	}
}

// --server-fingerprint must hold for every connection the installer opens, not
// only for its openssl probe. The relay stands for a peer on the network path:
// it hands the first connections to the pinned server and every later one to a
// server with another self-signed certificate. Preflight only: nothing is
// downloaded, installed or run.
func TestAgentInstallerServerFingerprintBindsEveryCurlConnection(t *testing.T) {
	if _, err := exec.LookPath("openssl"); err != nil {
		t.Fatal("installer fingerprint prerequisite openssl missing")
	}
	for _, tc := range []struct {
		name     string
		relayed  int // connections that reach the pinned server before the swap
		wantExit int
	}{
		{"every-connection-reaches-pinned-server", 1 << 30, 0},
		{"only-fingerprint-probe-reaches-pinned-server", 1, 11},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var mu sync.Mutex
			var pinnedRequests, otherRequests []string
			handler := func(requests *[]string) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					mu.Lock()
					*requests = append(*requests, r.Method+" "+r.URL.Path)
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
				})
			}
			pinned := httptest.NewTLSServer(handler(&pinnedRequests))
			defer pinned.Close()
			// httptest TLS servers share a certificate; the other peer needs
			// a key of its own.
			key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
			if err != nil {
				t.Fatal(err)
			}
			template := &x509.Certificate{
				SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "unpinned installer fixture"},
				NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
				KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
				IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
			}
			der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
			if err != nil {
				t.Fatal(err)
			}
			other := httptest.NewUnstartedServer(handler(&otherRequests))
			other.TLS = &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}}}
			other.StartTLS()
			defer other.Close()

			relay, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer relay.Close()
			go func() {
				for accepted := 0; ; accepted++ {
					client, err := relay.Accept()
					if err != nil {
						return
					}
					target := pinned.Listener.Addr().String()
					if accepted >= tc.relayed {
						target = other.Listener.Addr().String()
					}
					go func() {
						defer client.Close()
						upstream, err := net.Dial("tcp", target)
						if err != nil {
							return
						}
						defer upstream.Close()
						go func() { _, _ = io.Copy(upstream, client) }()
						_, _ = io.Copy(client, upstream)
					}()
				}
			}()

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, "bash", repoFile("scripts", "install.sh"),
				"--url", "https://"+relay.Addr().String(), "--state-dir", t.TempDir(),
				"--server-fingerprint", fmt.Sprintf("%x", sha256.Sum256(pinned.Certificate().Raw)),
				"--preflight-only", "--output", "json", "--non-interactive",
				"--disable-docker", "--disable-kubernetes", "--disable-proxmox")
			out, err := cmd.CombinedOutput()
			exit := 0
			if err != nil {
				ee, ok := err.(*exec.ExitError)
				if !ok {
					t.Fatal(err)
				}
				exit = ee.ExitCode()
			}
			if ctx.Err() != nil || exit != tc.wantExit {
				t.Fatalf("preflight exit=%d want=%d observer=%v: %s", exit, tc.wantExit, ctx.Err(), out)
			}
			mu.Lock()
			defer mu.Unlock()
			if len(otherRequests) != 0 {
				t.Fatalf("installer sent requests to a server without the pinned certificate: %v\n%s", otherRequests, out)
			}
			wantRequests := []string{"GET /api/health", "HEAD /download/pulse-agent"}
			if tc.wantExit != 0 {
				wantRequests = nil
			}
			reported := strings.Contains(string(out), `"code":"pulse_unreachable"`) && strings.Contains(string(out), `"code":"agent_download_unavailable"`)
			if strings.Join(pinnedRequests, "\n") != strings.Join(wantRequests, "\n") || reported != (tc.wantExit != 0) {
				t.Fatalf("requests=%v want=%v refusal-reported=%v: %s", pinnedRequests, wantRequests, reported, out)
			}
		})
	}
}

func agentInstallerTransferStatements(t *testing.T, phase string) string {
	t.Helper()
	if phase == "helper" || phase == "runner" {
		function := "download_verified_privileged_helper"
		if phase == "runner" {
			function = "download_verified_action_runner"
		}
		return extractInstallShellFunction(t, "read_magic_hex") + "\n" +
			extractInstallShellFunction(t, "final_response_header_value") + "\n" +
			extractInstallShellFunction(t, function) + "\n" + function + "\n"
	}
	source, err := os.ReadFile(repoFile("scripts", "install.sh"))
	if err != nil {
		t.Fatal(err)
	}
	between := func(start, end string) string {
		t.Helper()
		if strings.Count(string(source), start) != 1 || strings.Count(string(source), end) != 1 {
			t.Fatalf("ambiguous production transport boundaries: %q / %q", start, end)
		}
		_, suffix, _ := strings.Cut(string(source), start)
		body, _, found := strings.Cut(suffix, end)
		if !found {
			t.Fatal("production transfer boundary is reversed")
		}
		return start + body
	}
	if phase == "version" {
		return between("SERVER_VERSION=\"\"\n", "DOWNLOAD_QUERY=\"arch=${ARCH_PARAM}\"\n") +
			"printf 'VERSION=%s\\n' \"$SERVER_VERSION\"\n"
	}
	return between("# Build curl arguments as array for proper quoting\n", "SERVER_VERSION=\"\"\n") +
		between("DOWNLOAD_URL=\"${PULSE_URL}/download/${BINARY_NAME}?${DOWNLOAD_QUERY}\"\n", "# Verify downloaded binary\n")
}

// Cover the local readiness, Docker/helper socket and dormant private-config
// helper too, without inducing a service transition or running as another user.
// Existing lifecycle suites exercise their consumers separately.
// Require short -q: curl before 7.50.0 has no --disable long option.
// This inventory prevents a modern-only spelling from breaking legacy hosts;
// the real-curl tests establish transport behavior, not an old-binary install.
func TestAgentInstallerEveryCurlCallDisablesDefaultConfigFirst(t *testing.T) {
	source, err := os.ReadFile(repoFile("scripts", "install.sh"))
	if err != nil {
		t.Fatal(err)
	}
	// Match command positions, not a mention in help text or an inline comment:
	// a direct/if call, command substitution, or runuser's explicit separator.
	call := regexp.MustCompile(`(?:^[ \t]*(?:if[ \t]+(?:![ \t]+)?)?|\$\(|--[ \t]+)curl[ \t]+(.+)`)
	count := 0
	for _, line := range strings.Split(string(source), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") || strings.Contains(line, "command -v curl") {
			continue
		}
		match := call.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		count++
		if strings.HasPrefix(match[1], "-q ") {
			continue
		}
		// Passive preflight must use the same legacy-compatible first option.
		array := ""
		for _, name := range []string{"CURL_TEST_ARGS", "CURL_DOWNLOAD_CHECK_ARGS"} {
			if strings.HasPrefix(match[1], `"${`+name+`[@]}"`) {
				array = name
			}
		}
		if array == "" || !strings.Contains(string(source), array+"=(-q ") {
			t.Errorf("curl must ignore default config with first-option -q, including on legacy hosts: %s", line)
		}
	}
	if count != 14 {
		t.Fatalf("curl inventory unexpectedly incomplete: %d", count)
	}
	t.Logf("all %d installer curl calls use first-option -q (including preflight arrays)", count)
}
