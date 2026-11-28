package installtests

import (
	"bytes"
	"context"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestAutoUpdateAssetByteLimit(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("curl"); err != nil {
		t.Fatal("curl is required for the updater byte-limit controls")
	}
	const limit = 2 * 1024 * 1024
	payload := bytes.Repeat([]byte("x"), limit+64*1024)
	var requests atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		size := limit
		switch r.URL.Path {
		case "/over-length", "/over-chunked", "/over-eof":
			size = len(payload)
		case "/detection-byte":
			size = limit + 1
		case "/partial":
			w.Header().Set("Content-Length", "100")
			_, _ = io.WriteString(w, "partial")
			return
		case "/http-error":
			http.Error(w, "fixture HTTP refusal", http.StatusServiceUnavailable)
			return
		}
		if r.URL.Path == "/over-eof" {
			// A close-delimited response has neither a length nor chunked framing.
			conn, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				t.Error(err)
				return
			}
			defer conn.Close()
			_, _ = io.WriteString(conn, "HTTP/1.0 200 OK\r\nConnection: close\r\n\r\n")
			_, _ = conn.Write(payload[:size])
			return
		}
		if r.URL.Path == "/exact-chunked" || r.URL.Path == "/over-chunked" {
			w.(http.Flusher).Flush() // No Content-Length: curl cannot rely on it.
		} else {
			w.Header().Set("Content-Length", fmt.Sprint(size))
		}
		_, _ = w.Write(payload[:size])
	}))
	defer server.Close()
	dir := t.TempDir()
	ca := filepath.Join(dir, "ca.pem")
	if err := os.WriteFile(ca, pem.EncodeToMemory(&pem.Block{
		Type: "CERTIFICATE", Bytes: server.Certificate().Raw,
	}), 0600); err != nil {
		t.Fatal(err)
	}
	updater, err := filepath.Abs(repoFile("scripts", "pulse-auto-update.sh"))
	if err != nil {
		t.Fatal(err)
	}
	for _, pipefail := range []string{"on", "off"} {
		for _, tc := range []struct {
			name, path string
			ok         bool
		}{
			{"exact_length", "/exact", true},
			{"exact_chunked", "/exact-chunked", true},
			{"one_extra_byte", "/detection-byte", false},
			{"over_length", "/over-length", false},
			{"over_chunked", "/over-chunked", false},
			{"over_without_framing", "/over-eof", false},
			{"partial_curl_failure", "/partial", false},
			{"http_refusal", "/http-error", false},
		} {
			t.Run(pipefail+"/"+tc.name, func(t *testing.T) {
				asset := filepath.Join(t.TempDir(), "asset")
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				cmd := exec.CommandContext(ctx, "bash", "-c", `
source "$PULSE_TEST_UPDATER"
if [[ "$PULSE_TEST_PIPEFAIL" == off ]]; then set +o pipefail; fi
status=0
download_update_asset "$PULSE_TEST_URL" "$PULSE_TEST_ASSET" || status=$?
exit "$status"
`)
				cmd.Env = append(os.Environ(), "PULSE_TEST_UPDATER="+updater,
					"PULSE_TEST_URL="+server.URL+tc.path, "PULSE_TEST_ASSET="+asset,
					"PULSE_TEST_PIPEFAIL="+pipefail, "CURL_CA_BUNDLE="+ca,
					"CURL_HOME="+dir, "HOME="+dir, "NO_PROXY=", "no_proxy=",
					"HTTPS_PROXY=", "https_proxy=", "ALL_PROXY=", "all_proxy=", "SSL_CERT_FILE=")
				cmd.Env = append(cmd.Env, legacyCurlCLIEnvironment(t, t.TempDir())...)
				before := requests.Load()
				out, err := cmd.CombinedOutput()
				if ctx.Err() != nil || (err == nil) != tc.ok {
					t.Fatalf("bounded transport accepted=%t want=%t context=%v error=%v: %s", err == nil, tc.ok, ctx.Err(), err, out)
				}
				if got := requests.Load() - before; got != 1 {
					t.Fatalf("got %d fixture requests, want one with no retry", got)
				}
				data, err := os.ReadFile(asset)
				if err != nil || len(data) > limit+1 {
					t.Fatalf("download bytes=%d exceed bound=%d read=%v", len(data), limit+1, err)
				}
				if tc.ok && !bytes.Equal(data, payload[:limit]) {
					t.Fatal("successful boundary transfer changed the admitted bytes")
				}
				t.Logf("accepted=%t retained_bytes=%d bound=%d requests=1 pipefail=%s", tc.ok, len(data), limit+1, pipefail)
			})
		}
	}
}

func TestAutoUpdateAssetWriterFailures(t *testing.T) {
	t.Parallel()
	updater, err := filepath.Abs(repoFile("scripts", "pulse-auto-update.sh"))
	if err != nil {
		t.Fatal(err)
	}
	for _, pipefail := range []string{"on", "off"} {
		for _, fault := range []string{"writer", "destination", "size-read"} {
			t.Run(pipefail+"/"+fault, func(t *testing.T) {
				t.Parallel()
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				dir := t.TempDir()
				cmd := exec.CommandContext(ctx, "bash", "-c", `
source "$PULSE_TEST_UPDATER"
if [[ "$PULSE_TEST_PIPEFAIL" == off ]]; then set +o pipefail; fi
curl() { printf 'small complete fixture'; }
case "$PULSE_TEST_FAULT" in
  writer) head() { command head "$@"; return 73; } ;;
  destination) mkdir "$PULSE_TEST_ASSET" ;;
  size-read) wc() { return 74; } ;;
esac
if download_update_asset https://fixture.invalid/asset "$PULSE_TEST_ASSET"; then
  echo 'unsafe successful download'; exit 99
fi
echo 'refused incomplete writer or byte observation'
`)
				cmd.Env = append(os.Environ(), "PULSE_TEST_UPDATER="+updater,
					"PULSE_TEST_ASSET="+filepath.Join(dir, "asset"), "PULSE_TEST_FAULT="+fault,
					"PULSE_TEST_PIPEFAIL="+pipefail)
				out, err := cmd.CombinedOutput()
				if err != nil || ctx.Err() != nil || !bytes.Contains(out, []byte("refused incomplete writer or byte observation")) {
					t.Fatalf("accepted failed %s or lost result: %v context=%v: %s", fault, err, ctx.Err(), out)
				}
			})
		}
	}
}

// This adapter is sourced by the stalled loopback case only. Match the actual
// updater invocation before replacing just the wall-clock cost of the wait.
// "command curl" still reaches the legacy-option shim and then real curl.
const stalledUpdaterCurlFixture = `
curl() {
    local -a args=("$@")
    local -a expected=(-q -fsSL --connect-timeout 5 --max-time 60 --proto '=https' --proto-redir '=https' "$PULSE_TEST_URL" -o -)
    local i status=0
    if [[ "${#args[@]}" != "${#expected[@]}" ]]; then
        echo 'unexpected shipped updater curl arguments' >&2
        return 97
    fi
    for i in "${!expected[@]}"; do
        if [[ "${args[i]}" != "${expected[i]}" ]]; then
            echo 'unexpected shipped updater curl arguments' >&2
            return 97
        fi
    done
    args[5]=5
    command curl "${args[@]}" || status=$?
    printf '%s\n' "$status" > "$PULSE_TEST_CURL_EXIT" || return 97
    return "$status"
}
`

// Actual curl/TLS/deadline behavior, without contacting a provider or running an
// installer. The complete transaction/failure controls remain separate.
func TestAutoUpdateAssetTransport(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("curl"); err != nil {
		t.Fatal("curl is required for the updater transport controls")
	}
	var insecureRequests, stalledRequests atomic.Int32
	insecure := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		insecureRequests.Add(1)
		_, _ = io.WriteString(w, "must not be downloaded")
	}))
	defer insecure.Close()
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/redirect":
			http.Redirect(w, r, "/asset", http.StatusFound)
		case "/downgrade":
			http.Redirect(w, r, insecure.URL, http.StatusFound)
		case "/stalled":
			stalledRequests.Add(1)
			w.Header().Set("Content-Length", "100")
			_, _ = io.WriteString(w, "partial bytes")
			w.(http.Flusher).Flush()
			<-r.Context().Done()
		default:
			_, _ = io.WriteString(w, "complete signed-asset fixture\n")
		}
	}))
	defer server.Close()
	for _, tc := range []struct {
		name, path string
		trusted    bool
		exit       int
	}{
		{"complete", "/asset", true, 0},
		{"https_redirect", "/redirect", true, 0},
		{"untrusted_certificate", "/asset", false, 60},
		{"http_downgrade", "/downgrade", true, 1},
		{"stalled_transfer", "/stalled", true, 28},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			ca := filepath.Join(dir, "ca.pem")
			if err := os.WriteFile(ca, pem.EncodeToMemory(&pem.Block{
				Type: "CERTIFICATE", Bytes: server.Certificate().Raw,
			}), 0600); err != nil {
				t.Fatal(err)
			}
			// A broken ambient proxy would fail even the complete control if
			// -q were absent or ceased to be curl's first argument.
			if err := os.WriteFile(filepath.Join(dir, ".curlrc"), []byte("proxy = http://127.0.0.1:1\n"), 0600); err != nil {
				t.Fatal(err)
			}
			updater, err := filepath.Abs(repoFile("scripts", "pulse-auto-update.sh"))
			if err != nil {
				t.Fatal(err)
			}
			asset := filepath.Join(dir, "asset")
			curlResult := filepath.Join(dir, "curl-exit")
			script := "source \"$PULSE_TEST_UPDATER\"\n"
			deadline := 80 * time.Second
			if tc.path == "/stalled" {
				script += stalledUpdaterCurlFixture
				deadline = 20 * time.Second
			}
			script += `download_update_asset "$PULSE_TEST_URL" "$PULSE_TEST_ASSET"`
			ctx, cancel := context.WithTimeout(context.Background(), deadline)
			defer cancel()
			cmd := exec.CommandContext(ctx, "bash", "-c", script)
			cmd.WaitDelay = 2 * time.Second
			cmd.Env = append(os.Environ(), "PULSE_TEST_UPDATER="+updater,
				"PULSE_TEST_URL="+server.URL+tc.path, "PULSE_TEST_ASSET="+asset, "PULSE_TEST_CURL_EXIT="+curlResult,
				"CURL_HOME="+dir, "HOME="+dir, "NO_PROXY=", "no_proxy=",
				"HTTPS_PROXY=", "https_proxy=", "ALL_PROXY=", "all_proxy=", "SSL_CERT_FILE=")
			cmd.Env = append(cmd.Env, legacyCurlCLIEnvironment(t, dir)...)
			if tc.trusted {
				cmd.Env = append(cmd.Env, "CURL_CA_BUNDLE="+ca)
			} else {
				cmd.Env = append(cmd.Env, "CURL_CA_BUNDLE=")
			}
			started := time.Now()
			out, err := cmd.CombinedOutput()
			elapsed := time.Since(started)
			exit := 0
			if err != nil {
				failure, ok := err.(*exec.ExitError)
				if !ok {
					t.Fatalf("transport command result unavailable: %v", err)
				}
				exit = failure.ExitCode()
			}
			if ctx.Err() != nil || exit != tc.exit {
				t.Fatalf("transport exit=%d want=%d context=%v: %s", exit, tc.exit, ctx.Err(), out)
			}
			data, readErr := os.ReadFile(asset)
			if tc.exit == 0 && (readErr != nil || string(data) != "complete signed-asset fixture\n") {
				t.Fatalf("successful transfer bytes: %v %q", readErr, data)
			}
			if tc.path == "/stalled" {
				curlExit, resultErr := os.ReadFile(curlResult)
				if resultErr != nil || string(curlExit) != "28\n" || stalledRequests.Load() != 1 ||
					readErr != nil || string(data) != "partial bytes" {
					t.Fatalf("stalled transfer elapsed=%v requests=%d bytes=%q read=%v curl_exit=%q result=%v",
						elapsed, stalledRequests.Load(), data, readErr, curlExit, resultErr)
				}
				t.Log("one partial TLS response and curl exit 28; shipped 60s transfer bound checked before applying the 5s fixture clock")
			}
			t.Logf("updater exit=%d elapsed=%s asset_bytes=%d", exit, elapsed.Round(time.Millisecond), len(data))
		})
	}
	if got := insecureRequests.Load(); got != 0 {
		t.Fatalf("HTTPS downgrade reached the HTTP destination %d times", got)
	}
}
