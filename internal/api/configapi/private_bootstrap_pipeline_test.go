package configapi

import (
	"bytes"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// Real TLS/curl, issuance, downloaded native PBS script, private handoff and
// registration. Only the appliance CLI and EUID guard are modeled: Core's
// ordinary proof guest is deliberately not a system-install/root capability.
func TestPrivatePBSBootstrapPipelineTLSAndRegistration(t *testing.T) {
	curlPath, err := exec.LookPath("curl")
	if err != nil {
		t.Fatal("curl required for the TLS bootstrap acceptance test")
	}
	for _, status := range []int{200, 401, 403, 503} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			root, env := bootstrapFixture(t, "")
			cfg := &config.Config{DataPath: t.TempDir()}
			handler := newTestConfigHandlers(t, cfg)
			var registration AutoRegisterRequest
			var registrationMu sync.Mutex
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/setup-script" {
					if r.URL.Query().Has("setup_token") {
						t.Error("download URL contains setup token")
					}
					rr := httptest.NewRecorder()
					handler.HandleSetupScript(rr, r)
					if rr.Code != 200 {
						t.Errorf("download status %d", rr.Code)
					}
					for k, values := range rr.Header() {
						w.Header()[k] = values
					}
					// Preserve every operation except the unavailable actual root
					// identity check. All PBS CLI calls below are local fixtures.
					script := strings.Replace(rr.Body.String(), `if [ "$EUID" -ne 0 ]; then`, `if false; then`, 1)
					w.Write([]byte(script))
					return
				}
				if r.URL.Path == "/api/auto-register" {
					var input AutoRegisterRequest
					if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
						t.Error(err)
					}
					registrationMu.Lock()
					registration = input
					registrationMu.Unlock()
					body, _ := json.Marshal(input)
					r.Body = http.NoBody
					if status != 200 {
						w.WriteHeader(status)
						// An adverse server response must not turn the request's
						// credential into diagnostic output on the terminal.
						w.Write(body)
						return
					}
					r.Body = io.NopCloser(bytes.NewReader(body))
					handler.HandleAutoRegister(w, r)
					return
				}
				w.WriteHeader(404)
			}))
			defer server.Close()
			cfg.PublicURL = server.URL
			ca := filepath.Join(root, "ca.pem")
			if err := os.WriteFile(ca, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}), 0600); err != nil {
				t.Fatal(err)
			}
			request := httptest.NewRequest(http.MethodPost, "/api/setup-script-url", strings.NewReader(`{"type":"pbs","host":"`+server.URL+`"}`))
			rr := httptest.NewRecorder()
			handler.HandleSetupScriptURL(rr, request)
			if rr.Code != 200 {
				t.Fatalf("issuance status %d", rr.Code)
			}
			var artifact SetupScriptInstallArtifact
			if err := json.Unmarshal(rr.Body.Bytes(), &artifact); err != nil {
				t.Fatal(err)
			}
			if strings.Contains(artifact.Command, artifact.SetupToken) || artifact.DownloadURL != artifact.URL {
				t.Fatal("unsafe issued artifact")
			}
			// Record only process argument/secret-export presence; never dump
			// the environment, credential file, or registration body to logs.
			writeExecutable(t, filepath.Join(root, "bin", "curl"), "#!/bin/sh\nprintf '%s\\n' \"$@\" >> \"$FAKE_ROOT/argv\"\nexec "+posixShellQuote(curlPath)+" \"$@\"\n")
			writeExecutable(t, filepath.Join(root, "bin", "proxmox-backup-manager"), `#!/bin/sh
printf '%s\n' "$@" >> "$FAKE_ROOT/native-argv"
case "$*" in
  "user generate-token "*) printf '%s\n' '{"value":"synthetic-native-PBS-secret"}' ;;
esac
`)
			if status == 200 {
				out, err := runBootstrap(t, artifact.Command, "", env, false)
				if err == nil {
					t.Fatal("untrusted TLS was accepted")
				}
				if _, err := os.Stat(filepath.Join(root, "native-argv")); !os.IsNotExist(err) {
					t.Fatal("native mutation preceded trusted download")
				}
				if strings.Contains(string(out), artifact.SetupToken) {
					t.Fatal("setup credential leaked on TLS failure")
				}
			}
			env = append(env, "CURL_CA_BUNDLE="+ca)
			out, err := runBootstrap(t, artifact.Command, artifact.SetupToken, env, true)
			if status == 200 && err != nil {
				t.Fatalf("trusted bootstrap: %v\n%s", err, out)
			}
			if status != 200 && err == nil {
				t.Fatalf("HTTP %d appeared successful", status)
			}
			args, _ := os.ReadFile(filepath.Join(root, "argv"))
			if strings.Contains(string(args), artifact.SetupToken) || strings.Contains(string(args), "synthetic-native-PBS-secret") || strings.Contains(string(out), artifact.SetupToken) {
				t.Fatal("credential leaked through args or failed-response output")
			}
			registrationMu.Lock()
			defer registrationMu.Unlock()
			config.Mu.Lock()
			defer config.Mu.Unlock()
			if registration.AuthToken != artifact.SetupToken || registration.TokenValue != "synthetic-native-PBS-secret" || registration.Type != "pbs" || registration.Host != server.URL {
				t.Fatal("registration lost credential, host or product binding")
			}
			if status == 200 && len(cfg.PBSInstances) != 1 {
				t.Fatal("registration did not create one PBS source")
			}
			if status != 200 && len(cfg.PBSInstances) != 0 {
				t.Fatal("failed registration created a source")
			}
		})
	}
}
