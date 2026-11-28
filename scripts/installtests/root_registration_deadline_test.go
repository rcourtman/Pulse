//go:build linux

package installtests

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

// These are the shipped optional-registration transports and their failure
// branches, not a second installer. The surrounding container, pveum, ACL and
// service mutations are never invoked. Stalls use real curl's production
// arguments, including its existing retry counts and backoff. The private curl
// shim admits every original argument before scaling only --max-time to 1s.
// This keeps real header/body timeouts, retries and failure branches in normal
// qualification without repeating minutes of wall-clock waiting per case.
// Unscaled 15s/60s transport timing is separate retained one-off evidence.
func TestRootInstallerRegistrationTransferDeadlines(t *testing.T) {
	t.Parallel()
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
				retries, delay := 3, 2
				if phase == "register" {
					path = "/api/auto-register"
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
				env := rootRegistrationDeadlineEnvironment(t, root, server.URL, phase, payload)
				exit := runRootCurlFixtureWithin(t, script, env, 25*time.Second)
				mu.Lock()
				observed := append([]time.Time(nil), starts...)
				wasApplied := applied
				mu.Unlock()
				if len(observed) == 0 {
					t.Fatal("no real request was observed")
				}
				admission, err := os.ReadFile(filepath.Join(root, "deadline-admitted"))
				if err != nil || string(admission) != "production arguments admitted; private transfer=1s\n" {
					t.Fatalf("original curl arguments were not admitted: %q read=%v", admission, err)
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
					minimum := time.Duration(wantRequests+retries*delay)*time.Second - 500*time.Millisecond
					maximum := time.Duration(wantRequests+retries*delay+5) * time.Second
					if elapsed < minimum || elapsed > maximum {
						t.Errorf("transfer/retry budget changed: %s, expected %s..%s", elapsed, minimum, maximum)
					}
				} else if mode == "registered-response-stall" {
					wantRequests = 2
					if elapsed < 2500*time.Millisecond || elapsed > 8*time.Second {
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
				t.Logf("%s: original argv admitted; private max-time=1s; %d requests, terminal in %s, failure=%q", mode, len(observed), elapsed.Round(time.Millisecond), failure)
				assertRootCurlFixtureUnchanged(t, root, trace, saved)
			})
		}
	}
}

// Inspect actual curl argv at each shipped call, without trusting a static
// substring or reaching a provider. This fails on the old unbounded source;
// real transfers above establish that curl enforces the arguments too.
func TestRootInstallerRegistrationDeadlineArguments(t *testing.T) {
	t.Parallel()
	for _, phase := range []string{"smoke", "setup", "register"} {
		t.Run(phase, func(t *testing.T) {
			t.Parallel()
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
			want := rootRegistrationDeadlineArguments(phase, "http://fixture.invalid", "{}")
			if !reflect.DeepEqual(args, want) {
				t.Errorf("registration transport arguments changed: got %q, want %q", args, want)
			}
		})
	}
}

// The acceleration boundary must reject drift rather than silently restoring
// expected flags and allowing a broken production invocation to pass.
func TestRootInstallerRegistrationDeadlineShimRefusesChangedArguments(t *testing.T) {
	t.Parallel()
	for _, phase := range []string{"smoke", "setup", "register"} {
		for _, fault := range []string{"missing-max-time", "changed-max-time", "duplicate-max-time", "changed-retries", "changed-backoff", "changed-connect-timeout", "missing-config-isolation", "extra-header"} {
			t.Run(phase+"/"+fault, func(t *testing.T) {
				t.Parallel()
				root := t.TempDir()
				env := rootRegistrationDeadlineEnvironment(t, root, "http://fixture.invalid", phase, "{}")
				args := rootRegistrationDeadlineArguments(phase, "http://fixture.invalid", "{}")
				switch fault {
				case "missing-max-time":
					args = append(args[:7], args[9:]...)
				case "changed-max-time":
					args[8] = "900"
				case "duplicate-max-time":
					args = append(args, "--max-time", "900")
				case "changed-retries":
					args[2] = "99"
				case "changed-backoff":
					args[4] = "0"
				case "changed-connect-timeout":
					args[6] = "900"
				case "missing-config-isolation":
					args = args[1:]
				case "extra-header":
					args = append(args, "-H", "X-Unexpected: synthetic")
				}
				script := "curl"
				for _, arg := range args {
					script += " '" + strings.ReplaceAll(arg, "'", "'\"'\"'") + "'"
				}
				if exit := runRootCurlFixture(t, script, env); exit != 91 && exit != 92 {
					t.Errorf("changed original arguments escaped admission: exit=%d", exit)
				}
				if _, err := os.Stat(filepath.Join(root, "deadline-admitted")); !os.IsNotExist(err) {
					t.Errorf("changed arguments reached real curl: %v", err)
				}
			})
		}
	}
}

// Exact argv admission is shared by the non-executing argument control and
// the private real-curl shim. A missing, duplicated or extra option cannot be
// hidden by rewriting just one substring; production never reads these values.
func rootRegistrationDeadlineArguments(phase, origin, payload string) []string {
	retries, delay, maximum := "3", "2", "15"
	if phase == "smoke" {
		retries, delay = "2", "1"
	} else if phase == "register" {
		maximum = "60"
	}
	args := []string{"-q", "--retry", retries, "--retry-delay", delay, "--connect-timeout", "5", "--max-time", maximum}
	if phase == "smoke" {
		return append(args, "-kfsS", "-H", "@-", origin+"/api2/json/nodes")
	}
	path, data := "/api/setup-script-url", payload
	if phase == "register" {
		path, data = "/api/auto-register", "@-"
	}
	return append(args, "-fsS", "-X", "POST", origin+path, "-H", "Content-Type: application/json", "-d", data)
}

func rootRegistrationDeadlineEnvironment(t *testing.T, root, origin, phase, payload string) []string {
	t.Helper()
	realCurl, err := exec.LookPath("curl")
	if err != nil {
		t.Fatal("real curl is required for registration transport proof: ", err)
	}
	tools := filepath.Join(root, "tools")
	if err := os.Mkdir(tools, 0700); err != nil {
		t.Fatal(err)
	}
	// Bash array equality preserves each argument boundary, including stdin
	// payload/header admission. No shell evaluation of fixture strings is used.
	shim := "#!/bin/bash\nset -euo pipefail\nexpected=("
	for _, arg := range rootRegistrationDeadlineArguments(phase, origin, payload) {
		shim += " '" + strings.ReplaceAll(arg, "'", "'\"'\"'") + "'"
	}
	shim += ` )
actual=("$@")
[[ "${#actual[@]}" == "${#expected[@]}" ]] || exit 91
for i in "${!expected[@]}"; do
    [[ "${actual[$i]}" == "${expected[$i]}" ]] || exit 92
done
# Only the admitted transfer timeout's value changes. Keep first-option -q,
# connect timeout, TLS, payload, retry count and backoff exactly as shipped.
actual[8]=1
printf 'production arguments admitted; private transfer=1s\n' > "$FIXTURE_ROOT/deadline-admitted"
exec "$FIXTURE_REAL_CURL" "${actual[@]}"
`
	writeTestFile(t, filepath.Join(tools, "curl"), []byte(shim), 0700)
	return append(rootCurlFixtureEnvironment(t, root, origin), "FIXTURE_PAYLOAD="+payload,
		"PATH="+tools+string(os.PathListSeparator)+os.Getenv("PATH"), "FIXTURE_REAL_CURL="+realCurl)
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
