//go:build linux

package installtests

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// These are the shipped optional-registration transports and their failure
// branches, not a second installer. The surrounding container, pveum, ACL and
// service mutations are never invoked. Stalls use real curl's production
// limits, including its existing retry counts and backoff.
func TestRootInstallerRegistrationTransferDeadlines(t *testing.T) {
	for _, phase := range []string{"smoke", "setup", "register"} {
		for _, mode := range []string{"healthy", "transient-503", "stalled-headers", "stalled-body", "registered-response-stall"} {
			if mode == "registered-response-stall" && phase != "register" {
				continue // Only completion consumes the setup token and saves a node.
			}
			t.Run(phase+"/"+mode, func(t *testing.T) {
				t.Parallel()
				root := t.TempDir()
				const payload = `{"synthetic":"fixture request"}`
				const response = `{"synthetic":"fixture response"}`
				path, method := "/api/setup-script-url", "POST"
				retries, delay, transferSeconds := 3, 2, 15
				if phase == "register" {
					path, transferSeconds = "/api/auto-register", 60
				} else if phase == "smoke" {
					path, method, retries, delay = "/api2/json/nodes", "GET", 2, 1
				}
				var mu sync.Mutex
				var starts []time.Time
				applied := false
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					body, err := io.ReadAll(r.Body)
					mu.Lock()
					starts = append(starts, time.Now())
					attempt := len(starts)
					mu.Unlock()
					if r.URL.Path != path || r.Method != method || err != nil || r.Header.Get("X-Curlrc-Injected") != "" {
						t.Errorf("unexpected request: %s %s read=%v", r.Method, r.URL.Path, err)
					}
					if phase == "smoke" {
						if len(body) != 0 || r.Header.Get("Authorization") != "PVEAPIToken=synthetic@pve!monitor=synthetic-secret" {
							t.Error("local token smoke credential transport changed")
						}
					} else if string(body) != payload || r.Header.Get("Content-Type") != "application/json" || r.Header.Get("Authorization") != "" {
						t.Error("registration payload or credential transport changed")
					}
					switch mode {
					case "transient-503":
						if attempt == 1 {
							http.Error(w, "synthetic transient unavailability", http.StatusServiceUnavailable)
							return
						}
					case "registered-response-stall":
						if attempt == 1 {
							mu.Lock()
							applied = true // Synthetic remote commit, never a real Pulse mutation.
							mu.Unlock()
							<-r.Context().Done()
						} else {
							http.Error(w, "synthetic consumed setup token", http.StatusUnauthorized)
						}
						return
					case "stalled-headers":
						<-r.Context().Done()
						return
					case "stalled-body":
						w.Header().Set("Content-Length", "100")
						fmt.Fprint(w, "{")
						w.(http.Flusher).Flush()
						<-r.Context().Done()
						return
					}
					fmt.Fprint(w, response)
				}))
				defer server.Close()
				trace := filepath.Join(root, "ambient-trace")
				saved := map[string][]byte{
					".curlrc": []byte("max-time = 900\nretry = 99\nheader = \"X-Curlrc-Injected: synthetic\"\ntrace-ascii = \"" + trace + "\"\n"),
					"pulse":   []byte("saved binary\n"), "system.json": []byte("saved configuration\n"),
					"token": []byte("saved fixture token\n"),
				}
				for name, body := range saved {
					writeTestFile(t, filepath.Join(root, name), body, 0600)
				}
				script := rootRegistrationDeadlineScript(t, phase)
				env := append(rootCurlFixtureEnvironment(t, root, server.URL), "FIXTURE_PAYLOAD="+payload)
				exit := runRootCurlFixtureWithin(t, script, env, 270*time.Second)
				mu.Lock()
				observed := append([]time.Time(nil), starts...)
				wasApplied := applied
				mu.Unlock()
				if len(observed) == 0 {
					t.Fatal("no real request was observed")
				}
				elapsed := time.Since(observed[0])
				failure, err := os.ReadFile(filepath.Join(root, "failure"))
				if exit != 0 || err != nil {
					t.Fatalf("failure branch observation: exit=%d read=%v", exit, err)
				}
				accepted, acceptedErr := os.ReadFile(filepath.Join(root, "accepted"))
				wantRequests := 1
				if strings.HasPrefix(mode, "stalled-") {
					wantRequests = retries + 1
					wantFailure := map[string]string{"smoke": "token smoke check failed", "setup": "setup token request failed", "register": "auto-registration unconfirmed"}[phase]
					if string(failure) != wantFailure || !os.IsNotExist(acceptedErr) {
						t.Errorf("stalled response admitted: failure=%q accepted=%q read=%v", failure, accepted, acceptedErr)
					}
					minimum := time.Duration(wantRequests*transferSeconds)*time.Second - 2*time.Second
					maximum := time.Duration(wantRequests*transferSeconds+retries*delay+12) * time.Second
					if elapsed < minimum || elapsed > maximum {
						t.Errorf("transfer/retry budget changed: %s, expected %s..%s", elapsed, minimum, maximum)
					}
				} else if mode == "registered-response-stall" {
					wantRequests = 2
					if elapsed < 58*time.Second || elapsed > 74*time.Second {
						t.Errorf("unconfirmed completion budget changed: %s", elapsed)
					}
					warnings, err := os.ReadFile(filepath.Join(root, "warnings"))
					if !wasApplied || string(failure) != "auto-registration unconfirmed" || !os.IsNotExist(acceptedErr) || err != nil ||
						!strings.Contains(string(warnings), "the node may already be registered") || !strings.Contains(string(warnings), "before re-running the installer") {
						t.Errorf("lost completion response misrepresented: applied=%t failure=%q warnings=%q read=%v", wasApplied, failure, warnings, err)
					}
				} else {
					if mode == "transient-503" {
						wantRequests = 2
					}
					if len(failure) != 0 || acceptedErr != nil || (phase != "smoke" && string(accepted) != response) {
						t.Errorf("completed request rejected: failure=%q accepted=%q read=%v", failure, accepted, acceptedErr)
					}
				}
				if len(observed) != wantRequests {
					t.Errorf("retry policy changed: got %d requests, want %d", len(observed), wantRequests)
				}
				t.Logf("%s: %d requests, terminal in %s, failure=%q", mode, len(observed), elapsed.Round(time.Millisecond), failure)
				assertRootCurlFixtureUnchanged(t, root, trace, saved)
			})
		}
	}
}

// Inspect actual curl argv at each shipped call, without trusting a static
// substring or reaching a provider. This fails on the old unbounded source;
// real transfers above establish that curl enforces the arguments too.
func TestRootInstallerRegistrationDeadlineArguments(t *testing.T) {
	for _, phase := range []string{"smoke", "setup", "register"} {
		t.Run(phase, func(t *testing.T) {
			root := t.TempDir()
			script := `curl() { printf '%s\n' "$@" > "$FIXTURE_ROOT/argv"; cat >/dev/null; printf '{}'; }
` + rootRegistrationDeadlineScript(t, phase)
			env := append(rootCurlFixtureEnvironment(t, root, "http://fixture.invalid"), "FIXTURE_PAYLOAD={}")
			if exit := runRootCurlFixture(t, script, env); exit != 0 {
				t.Fatal("transport argument probe failed: ", exit)
			}
			body, err := os.ReadFile(filepath.Join(root, "argv"))
			if err != nil {
				t.Fatal(err)
			}
			args := strings.Split(strings.TrimSpace(string(body)), "\n")
			encoded, _ := json.Marshal(args)
			wantMax := `"--max-time","15"`
			if phase == "register" {
				wantMax = `"--max-time","60"`
			}
			if args[0] != "-q" || !strings.Contains(string(encoded), `"--connect-timeout","5"`) || !strings.Contains(string(encoded), wantMax) {
				t.Errorf("registration transfer must be independently bounded: %s", encoded)
			}
		})
	}
}

func rootRegistrationDeadlineScript(t *testing.T, phase string) string {
	t.Helper()
	call := `if smoke_test_pve_auto_register_token "$FIXTURE_ORIGIN" 'synthetic@pve!monitor' 'synthetic-secret'; then
    : > "$FIXTURE_ROOT/accepted"
fi`
	if phase != "smoke" {
		content, err := os.ReadFile(rootCurlFixtureSource(t))
		if err != nil {
			t.Fatal(err)
		}
		marker := "    if ! " + phase + "_response=$("
		start := strings.Index(string(content), marker)
		if start < 0 || strings.Count(string(content), marker) != 1 {
			t.Fatal("missing or duplicated registration transport block: ", phase)
		}
		end := strings.Index(string(content)[start:], "\n    fi")
		if end < 0 {
			t.Fatal("unterminated registration transport block: ", phase)
		}
		block := string(content)[start : start+end+len("\n    fi")]
		call = "registration_transport() {\nlocal " + phase + "_response\n" + block + "\nprintf '%s' \"$" + phase + "_response\" > \"$FIXTURE_ROOT/accepted\"\n}\nregistration_transport"
	}
	return `source "$ROOT_INSTALLER_SOURCE"
print_warn() { printf '%s\n' "$*" >> "$FIXTURE_ROOT/warnings"; }
pulse_url="$FIXTURE_ORIGIN"
setup_payload="$FIXTURE_PAYLOAD"
register_payload="$FIXTURE_PAYLOAD"
` + call + `
printf '%s' "$AUTO_NODE_REGISTER_ERROR" > "$FIXTURE_ROOT/failure"
[[ "$AUTO_NODE_REGISTERED" == false ]]
`
}
