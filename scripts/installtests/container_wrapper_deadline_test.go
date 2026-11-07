package installtests

import (
	"encoding/pem"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// Keep routine qualification fast: assert the wrapper's actual 5s/60s curl
// arguments, then scale only the private peer's transfer deadline to 1s. This
// still runs real curl through the complete shipped wrapper, including cleanup
// and prefix nonexecution. Exact unscaled timing is separate one-off evidence.
func TestContainerAgentCompatibilityWrapperBoundsStalledDownload(t *testing.T) {
	realCurl, err := exec.LookPath("curl")
	if err != nil {
		t.Fatal("curl is required for wrapper transport proof")
	}
	for _, tc := range []struct {
		name, body string
		insecure   bool
	}{
		{name: "before_headers"},
		{name: "valid_script_prefix", body: "printf touched > \"$WRAPPER_MARKER\"\n", insecure: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var requests atomic.Int32
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				if tc.body != "" {
					w.Header().Set("Content-Length", "1024")
					_, _ = io.WriteString(w, tc.body)
					w.(http.Flusher).Flush()
				}
				<-r.Context().Done()
			}))
			defer server.Close()
			arguments := []string{"--url", server.URL}
			if tc.insecure {
				arguments = append(arguments, "--insecure")
			}
			cmd, root := containerAgentWrapperCommand(t, arguments...)
			ca := filepath.Join(root, "ca.pem")
			writeTestFile(t, ca, pem.EncodeToMemory(&pem.Block{
				Type: "CERTIFICATE", Bytes: server.Certificate().Raw,
			}), 0o600)
			tools := filepath.Join(root, "tools")
			if err := os.Mkdir(tools, 0o700); err != nil {
				t.Fatal(err)
			}
			shim := `#!/usr/bin/env bash
set -euo pipefail
args=()
connect=false
transfer=false
while (( $# )); do
    case "$1" in
        --connect-timeout)
            [[ "$connect" == false && "${2:-}" == 5 ]] || exit 91
            connect=true
            args+=("$1" "$2"); shift 2 ;;
        --max-time)
            [[ "$transfer" == false && "${2:-}" == 60 ]] || exit 92
            transfer=true
            args+=("$1" 1); shift 2 ;;
        *) args+=("$1"); shift ;;
    esac
done
[[ "$connect" == true && "$transfer" == true ]] || exit 93
printf 'production=5/60 fixture_transfer=1\n' > "$WRAPPER_DEADLINE_ARGS"
exec "$WRAPPER_REAL_CURL" "${args[@]}"
`
			writeTestFile(t, filepath.Join(tools, "curl"), []byte(shim), 0o700)
			admitted := filepath.Join(root, "deadline-arguments")
			cmd.Env = append(cmd.Env, "PATH="+tools+string(os.PathListSeparator)+os.Getenv("PATH"),
				"WRAPPER_REAL_CURL="+realCurl, "WRAPPER_DEADLINE_ARGS="+admitted, "CURL_CA_BUNDLE="+ca, "LC_ALL=C")
			started := time.Now()
			output, runErr := cmd.CombinedOutput()
			elapsed := time.Since(started)
			var exit *exec.ExitError
			if !errors.As(runErr, &exit) || exit.ExitCode() != 1 ||
				!strings.Contains(string(output), "nothing was executed") ||
				!strings.Contains(string(output), "curl: (28)") {
				t.Fatalf("stalled download was not refused by curl: %v\n%s", runErr, output)
			}
			if bytes, err := os.ReadFile(admitted); err != nil || string(bytes) != "production=5/60 fixture_transfer=1\n" {
				t.Fatalf("production deadline arguments changed: %q (%v)", bytes, err)
			}
			if requests.Load() != 1 || elapsed < 900*time.Millisecond || elapsed > 10*time.Second {
				t.Fatalf("unexpected deadline/replay: requests=%d elapsed=%v", requests.Load(), elapsed)
			}
			if _, err := os.Stat(filepath.Join(root, "executed")); !os.IsNotExist(err) {
				t.Fatalf("incomplete installer prefix executed: %v", err)
			}
			assertContainerWrapperDownloadRemoved(t, root)
			t.Logf("production connect=5s/transfer=60s; private real-curl deadline=1s; elapsed=%s", elapsed.Round(time.Millisecond))
		})
	}
}
