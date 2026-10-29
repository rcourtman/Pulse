package installtests

import (
	"context"
	"encoding/pem"
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

// Actual curl/TLS/deadline behavior, without contacting a provider or running an
// installer. The complete transaction/failure controls below remain separate.
func TestAutoUpdateAssetTransport(t *testing.T) {
	// Private peers, files and child environments do not need to serialise
	// the real sixty-second curl deadline with independent agent controls.
	t.Parallel()
	if _, err := exec.LookPath("curl"); err != nil {
		t.Fatal("curl is required for the updater transport controls")
	}
	var insecureRequests atomic.Int32
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
			// --disable were absent or ceased to be curl's first argument.
			if err := os.WriteFile(filepath.Join(dir, ".curlrc"), []byte("proxy = http://127.0.0.1:1\n"), 0600); err != nil {
				t.Fatal(err)
			}
			updater, err := filepath.Abs(repoFile("scripts", "pulse-auto-update.sh"))
			if err != nil {
				t.Fatal(err)
			}
			asset := filepath.Join(dir, "asset")
			ctx, cancel := context.WithTimeout(context.Background(), 80*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, "bash", "-c",
				`source "$PULSE_TEST_UPDATER"; download_update_asset "$PULSE_TEST_URL" "$PULSE_TEST_ASSET"`)
			cmd.Env = append(os.Environ(), "PULSE_TEST_UPDATER="+updater,
				"PULSE_TEST_URL="+server.URL+tc.path, "PULSE_TEST_ASSET="+asset,
				"CURL_HOME="+dir, "HOME="+dir, "NO_PROXY=", "no_proxy=",
				"HTTPS_PROXY=", "https_proxy=", "ALL_PROXY=", "all_proxy=", "SSL_CERT_FILE=")
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
			bytes, readErr := os.ReadFile(asset)
			if tc.exit == 0 && (readErr != nil || string(bytes) != "complete signed-asset fixture\n") {
				t.Fatalf("successful transfer bytes: %v %q", readErr, bytes)
			}
			if tc.path == "/stalled" {
				if elapsed < 55*time.Second || elapsed > 75*time.Second || string(bytes) != "partial bytes" {
					t.Fatalf("stalled transfer elapsed=%v bytes=%q read=%v", elapsed, bytes, readErr)
				}
			}
			t.Logf("curl exit=%d elapsed=%s asset_bytes=%d", exit, elapsed.Round(time.Millisecond), len(bytes))
		})
	}
	if got := insecureRequests.Load(); got != 0 {
		t.Fatalf("HTTPS downgrade reached the HTTP destination %d times", got)
	}
}
