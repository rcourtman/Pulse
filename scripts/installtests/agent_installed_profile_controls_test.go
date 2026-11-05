//go:build linux

package installtests

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestAgentInstalledProfileAdmission(t *testing.T) {
	env := map[string]string{"PULSE_NATIVE_INSTALLED_PROFILE": "1", "GITHUB_ACTIONS": "true", "RUNNER_ENVIRONMENT": "github-hosted", "GITHUB_REPOSITORY": "rcourtman/Pulse", "EXPECTED_SOURCE_SHA": strings.Repeat("a", 40)}
	if err := nativeProfileAdmission(env, "linux", 0, "systemd\n", nil); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"PULSE_NATIVE_INSTALLED_PROFILE", "GITHUB_ACTIONS", "RUNNER_ENVIRONMENT", "GITHUB_REPOSITORY", "EXPECTED_SOURCE_SHA"} {
		t.Run(name, func(t *testing.T) {
			changed := make(map[string]string)
			for key, value := range env {
				changed[key] = value
			}
			changed[name] = "unbound"
			if err := nativeProfileAdmission(changed, "linux", 0, "systemd", nil); err == nil {
				t.Fatal("accepted an unbound host")
			}
		})
	}
	for _, host := range []struct {
		os         string
		uid        int
		pid1       string
		collisions []string
	}{
		{"darwin", 0, "systemd", nil}, {"linux", 1000, "systemd", nil}, {"linux", 0, "bash", nil},
	} {
		if err := nativeProfileAdmission(env, host.os, host.uid, host.pid1, host.collisions); err == nil {
			t.Fatal("accepted a worker/non-systemd host")
		}
	}
	for _, path := range append(append([]string{}, nativeProfilePaths...), "/.dockerenv") {
		t.Run(path, func(t *testing.T) {
			if err := nativeProfileAdmission(env, "linux", 0, "systemd", []string{path}); err == nil {
				t.Fatal("accepted an existing host")
			}
		})
	}
}

func TestAgentInstalledProfileSnapshotsBindBytesAndMetadata(t *testing.T) {
	path := filepath.Join(t.TempDir(), "saved-state")
	if err := os.WriteFile(path, []byte("original-history"), 0o600); err != nil {
		t.Fatal(err)
	}
	before := nativeProfileFiles(t, path)
	if before[path].Hash == "" {
		t.Fatal("snapshot omitted content identity")
	}
	if !reflect.DeepEqual(before, nativeProfileFiles(t, path)) {
		t.Fatal("unchanged snapshot differed")
	}
	if err := os.WriteFile(path, []byte("replaced-history"), 0o600); err != nil {
		t.Fatal(err)
	}
	if reflect.DeepEqual(before, nativeProfileFiles(t, path)) {
		t.Fatal("accepted changed bytes with unchanged ownership/mode")
	}
	before = nativeProfileTree(t, filepath.Dir(path))
	if err := os.Chmod(path, 0o640); err != nil {
		t.Fatal(err)
	}
	if reflect.DeepEqual(before, nativeProfileTree(t, filepath.Dir(path))) {
		t.Fatal("accepted changed snapshot file metadata")
	}
}

func TestAgentInstalledProfileFixtureVerifiesTLS(t *testing.T) {
	fixture := newSecureRuntimeLabFixture(nil, "", nil, nil, "fixture")
	t.Cleanup(fixture.actionServer.Shutdown)
	server := httptest.NewTLSServer(fixture)
	t.Cleanup(server.Close)
	for name, ca := range map[string][]byte{"trusted": pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}), "unrelated": nativeProfileUntrustedCA(t)} {
		t.Run(name, func(t *testing.T) {
			roots := x509.NewCertPool()
			if !roots.AppendCertsFromPEM(ca) {
				t.Fatal("fixture CA is not valid PEM")
			}
			transport := &http.Transport{TLSClientConfig: &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}}
			t.Cleanup(transport.CloseIdleConnections)
			client := &http.Client{Transport: transport, Timeout: 3 * time.Second}
			response, err := client.Get(server.URL + "/api/health")
			if name == "unrelated" {
				if err == nil {
					response.Body.Close()
					t.Fatal("fixture accepted an unrelated CA")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			body, err := io.ReadAll(response.Body)
			if err != nil || response.StatusCode != 200 || !strings.Contains(string(body), `"status":"ok"`) {
				t.Fatalf("verified TLS fixture health: %s, %v", body, err)
			}
		})
	}
}

func TestAgentInstalledProfileFixtureEnforcesRealInstallerSignatures(t *testing.T) {
	dir := t.TempDir()
	key := filepath.Join(dir, "fixture-key")
	nativeProfileMust(t, nil, "ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", key)
	public := strings.TrimSpace(string(secureRuntimeReadFile(t, key+".pub")))
	template := secureRuntimeReadFile(t, repoFile("scripts", "install.sh"))
	pinned, err := nativeProfilePin(template, public)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(pinned, "__PULSE_INSTALLER_SSH_PUBLIC_KEY__") != 1 {
		t.Fatal("pinning removed the verifier sentinel comparison")
	}
	for _, malformed := range []string{"", strings.ReplaceAll(string(template), "__PULSE_INSTALLER_SSH_PUBLIC_KEY__", "removed")} {
		if _, err := nativeProfilePin([]byte(malformed), public); err == nil {
			t.Fatal("accepted a changed template")
		}
	}
	path := filepath.Join(dir, "helper")
	if err := os.WriteFile(path, []byte("signed helper bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	signature := nativeProfileSign(t, key, path)
	// Extract the actual unchanged verification functions from the pin-templated
	// installer. The positive and changed-byte cases execute ssh-keygen, not a
	// scripted verifier or a string assertion about its intended behaviour.
	functions := ""
	for _, name := range []string{"has_pinned_installer_signature_key", "decode_base64_to_file", "verify_download_signature"} {
		functions += extractInstallShellFunction(t, name) + "\n"
	}
	harness := `set -euo pipefail
TMP_FILES=()
export TMPDIR="$4"
EXIT_SIGNATURE_FAILED=93
INSTALL_SIGNATURE_IDENTITY=pulse-installer
INSTALL_SIGNATURE_NAMESPACE=pulse-install
PINNED_INSTALLER_SSH_PUBLIC_KEY="$1"
fail() { exit "$2"; }
log_info() { :; }
json_event() { :; }
` + functions + `
has_pinned_installer_signature_key
verify_download_signature "$2" "$3"
`
	for name, content := range map[string]string{"valid": "signed helper bytes", "replaced": "different helper bytes"} {
		t.Run(name, func(t *testing.T) {
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := nativeProfileCommand(t, nil, "bash", "-c", harness, "native-signature-control", public, path, signature, dir)
			if (name == "valid") != (err == nil) {
				t.Fatalf("actual installer signature verdict: %v", err)
			}
		})
	}
}

func nativeProfileWorkflowSteps(t *testing.T) []nativeAgentFrontendStep {
	t.Helper()
	content := secureRuntimeReadFile(t, repoFile(".github", "workflows", "unified-agent-native.yml"))
	var workflow struct {
		Jobs map[string]struct {
			Runner   string `yaml:"runs-on"`
			Timeout  int    `yaml:"timeout-minutes"`
			Continue bool   `yaml:"continue-on-error"`
			Steps    []nativeAgentFrontendStep
		}
	}
	if err := yaml.Unmarshal(content, &workflow); err != nil {
		t.Fatal(err)
	}
	job, ok := workflow.Jobs["installed-profile-systemd"]
	if !ok || job.Runner != "ubuntu-24.04" || job.Timeout != 25 || job.Continue {
		t.Fatal("installed profile job must be a bounded required fresh public VM")
	}
	return job.Steps
}

func TestNativeInstalledProfileWorkflow(t *testing.T) {
	steps := nativeProfileWorkflowSteps(t)
	var build, execute, retain *nativeAgentFrontendStep
	for i := range steps {
		step := &steps[i]
		if step.ContinueOnError {
			t.Fatal("native profile failures must remain failures")
		}
		switch step.Name {
		case "Build exact-source installed profile inputs":
			build = step
		case "Exercise signed TLS installed profile forward and reverse":
			execute = step
		case "Retain native installed profile observations":
			retain = step
		}
	}
	if build == nil || execute == nil || retain == nil || build.If != "" || execute.If != "" || execute.WorkingDirectory != "scripts/installtests" || retain.If != "always()" {
		t.Fatal("native profile execution/retention cannot be skipped")
	}
	for _, required := range []string{`test "$(git rev-parse HEAD)" = "$EXPECTED_SOURCE_SHA"`, "git diff --exit-code HEAD", "for version in 1 2", "-buildvcs=true", "./cmd/pulse-agent", "./cmd/pulse-agent-helper", "./cmd/pulse-agent-runner", "go test -c", "./scripts/installtests", "$RUNNER_TEMP/pulse-native-profile-inputs."} {
		if !strings.Contains(build.Run, required) {
			t.Fatalf("native build omitted %s", required)
		}
	}
	for _, required := range []string{`test "$(git rev-parse HEAD)" = "$EXPECTED_SOURCE_SHA"`, "git diff --exit-code HEAD", "sudo env -i", "RUNNER_ENVIRONMENT=github-hosted", "GITHUB_REPOSITORY=rcourtman/Pulse", "PULSE_NATIVE_INSTALLED_PROFILE=1", `PULSE_NATIVE_PROFILE_INPUTS="$PULSE_NATIVE_PROFILE_INPUTS"`, "'^TestAgentInstalledProfileSystemd$'", "-test.timeout=11m"} {
		if !strings.Contains(execute.Run, required) {
			t.Fatalf("native execution omitted %s", required)
		}
	}
	if retain.With["path"] != "${{ runner.temp }}/pulse-installed-profile.json" || retain.With["retention-days"] != "7" {
		t.Fatal("native receipts must be bounded and always retained")
	}
}
