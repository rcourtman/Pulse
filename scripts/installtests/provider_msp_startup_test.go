package installtests

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
)

// Explicitly opt in: ordinary unit runs neither build another daemon nor use
// Docker. The hosted job requires Compose, not a skip or a parser-only pass.
// "process" also permits the same real release daemon in an offline proof VM;
// it does NOT establish Docker secret-mount or installed provider acceptance.
func TestProviderMSPProductionStartup(t *testing.T) {
	transport := os.Getenv("PULSE_TEST_PROVIDER_MSP_STARTUP")
	if transport == "" {
		t.Skip("set PULSE_TEST_PROVIDER_MSP_STARTUP=process or compose for real daemon acceptance")
	}
	if runtime.GOOS != "linux" || (transport != "process" && transport != "compose") {
		t.Fatalf("unsupported startup acceptance transport/platform: %q/%s", transport, runtime.GOOS)
	}
	root := repoFile()
	if assets, err := filepath.Glob(filepath.Join(root, "internal/api/frontend-modern/dist/assets/*.js")); err != nil || len(assets) == 0 {
		t.Fatal("startup acceptance requires the real frontend build, not the unit-test embed stub")
	}
	issuerPublic, issuerPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	leasePublic, leasePrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	binaryDir := t.TempDir()
	binary := filepath.Join(binaryDir, "binaries", "pulse-control-plane-linux-amd64")
	if err := os.MkdirAll(filepath.Dir(binary), 0o700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	build := exec.CommandContext(ctx, "go", "build", "-tags=release", "-trimpath", "-buildvcs=false",
		"-ldflags=-X main.Version=msp-startup-integration -X github.com/rcourtman/pulse-go-rewrite/pkg/licensing.EmbeddedPublicKey="+base64.StdEncoding.EncodeToString(issuerPublic),
		"-o", binary, "./cmd/pulse-control-plane")
	build.Dir = root
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build exact-source release daemon: %v\n%s", err, output)
	}
	payload, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("release_daemon_sha256=%x transport=%s", sha256.Sum256(payload), transport)

	dockerEnv := []string{"PATH=" + os.Getenv("PATH"), "HOME=" + t.TempDir(), "DOCKER_CONFIG=" + t.TempDir()}
	nonce := fmt.Sprintf("%x", sha256.Sum256(issuerPublic))[:16]
	image := "pulse-msp-startup-test:" + nonce
	if transport == "compose" {
		startupCommand(t, dockerEnv, root, "docker", "compose", "version")
		startupCommand(t, dockerEnv, root, "docker", "version")
		startupCommand(t, dockerEnv, root, "docker", "build", "--build-context", "compiled_payload="+binaryDir,
			"--target", "control_plane_prebuilt", "-f", "deploy/provider-msp/Dockerfile.control-plane", "-t", image, ".")
		t.Cleanup(func() { startupCommand(t, dockerEnv, root, "docker", "image", "rm", image) })
	}
	claims := pkglicensing.Claims{
		LicenseID: "lic_synthetic_startup", Email: "provider@example.invalid", Tier: pkglicensing.TierMSP,
		PlanVersion: "msp_growth", IssuedAt: time.Now().Add(-time.Minute).Unix(), ExpiresAt: time.Now().Add(24 * time.Hour).Unix(),
		EntitlementSigningPublicKey: base64.StdEncoding.EncodeToString(leasePublic),
	}
	payloadJSON, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	signingInput := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"EdDSA","typ":"JWT"}`)) + "." + base64.RawURLEncoding.EncodeToString(payloadJSON)
	license := signingInput + "." + base64.RawURLEncoding.EncodeToString(ed25519.Sign(issuerPrivate, []byte(signingInput)))

	for _, kind := range []string{"unlicensed", "licensed", "renewed", "empty", "truncated", "unsigned_paid", "pulse_hosted"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			dataDir := filepath.Join(dir, "data")
			if err := os.MkdirAll(filepath.Join(dataDir, "control-plane"), 0o700); err != nil {
				t.Fatal(err)
			}
			licensePath := ""
			plan, source, limit := "msp_eval", "environment_fallback", 2
			mode, configuredPlan, refusal := "provider_hosted_msp", "", ""
			switch kind {
			case "licensed", "empty", "truncated":
				licensePath = filepath.Join(dir, "license.jwt")
				content := license
				if kind == "empty" {
					content, refusal = "", "CP_PROVIDER_MSP_LICENSE_FILE is empty"
				} else if kind == "truncated" {
					content, refusal = "truncated.jwt", "validate CP_PROVIDER_MSP_LICENSE_FILE"
				} else {
					plan, source, limit = "msp_growth", "license_file", 15
				}
				startupWrite(t, licensePath, content)
			case "renewed":
				startupWrite(t, cloudcp.ProviderMSPRenewedLicensePath(dataDir), license)
				plan, source, limit = "msp_growth", "renewed_license", 15
			case "unsigned_paid":
				configuredPlan, refusal = "msp_scale", "must be msp_eval without a signed provider MSP license"
			case "pulse_hosted":
				mode, refusal = "pulse_hosted_msp", "CP_PROVIDER_MSP_LICENSE_FILE is required in production"
			}
			env := []string{
				"PATH=" + os.Getenv("PATH"), "CP_ENV=production", "CP_DATA_DIR=" + dataDir,
				"CP_CONTROL_PLANE_MODE=" + mode, "CP_ADMIN_KEY=" + strings.Repeat("synthetic-startup-", 3),
				"CP_BASE_URL=https://msp.example.invalid", "CP_REQUIRE_EMAIL_PROVIDER=false",
				"CP_PROVIDER_MSP_LICENSE_FILE=" + licensePath, "CP_PROVIDER_MSP_PLAN_VERSION=" + configuredPlan,
				"CP_ENTITLEMENT_SIGNING_PRIVATE_KEY=" + base64.StdEncoding.EncodeToString(leasePrivate),
				"PULSE_LICENSE_SERVER_URL=http://127.0.0.1:1", "CP_ALLOW_DOCKERLESS_PROVISIONING=false",
				"CP_STORAGE_GUARDRAILS_ENABLED=true", "CP_STORAGE_ROOT_PATH=" + dir, "CP_STORAGE_DATA_PATH=" + dataDir,
				"CP_STORAGE_DOCKER_PATH=" + dir, "CP_STORAGE_MIN_ROOT_AVAILABLE=1MiB",
				"CP_STORAGE_MIN_DATA_AVAILABLE=1MiB", "CP_STORAGE_MIN_DOCKER_AVAILABLE=1MiB",
			}
			var run func(...string) string
			var start func() (string, func())
			if transport == "process" {
				listener, err := net.Listen("tcp", "127.0.0.1:0")
				if err != nil {
					t.Fatal(err)
				}
				port := listener.Addr().(*net.TCPAddr).Port
				if err := listener.Close(); err != nil {
					t.Fatal(err)
				}
				env = append(env, "CP_BIND_ADDRESS=127.0.0.1", "CP_PORT="+strconv.Itoa(port), "DOCKER_HOST=tcp://127.0.0.1:1")
				run = func(args ...string) string { return startupCommand(t, env, dir, binary, args...) }
				if refusal != "" {
					startupRefusal(t, env, dir, refusal, binary)
					return
				}
				start = func() (string, func()) {
					return startupProcess(t, binary, dir, env, "http://127.0.0.1:"+strconv.Itoa(port))
				}
			} else {
				composeBytes, err := os.ReadFile(repoFile("deploy", "provider-msp", "docker-compose.yml"))
				if err != nil {
					t.Fatal(err)
				}
				startupWrite(t, filepath.Join(dir, "docker-compose.yml"), string(composeBytes))
				startupWrite(t, filepath.Join(dir, "startup.yml"), "services:\n  control-plane:\n    ports: ['127.0.0.1::8443']\n    restart: 'no'\n")
				env = append(env,
					"DOMAIN=msp.example.invalid", "ACME_EMAIL=operator@example.invalid", "CP_TRUSTED_PROXY_CIDRS=172.30.0.0/24",
					"CONTROL_PLANE_IMAGE="+image, "CP_PULSE_IMAGE="+image, "TRAEFIK_IMAGE="+image, "DOCKER_SOCKET_PROXY_IMAGE="+image,
					"PULSE_PROVIDER_MSP_DATA_DIR="+dataDir, "PULSE_PROVIDER_MSP_ROOT_SPACECHECK_DIR="+dir,
					"PULSE_PROVIDER_MSP_DOCKER_SPACECHECK_DIR="+dir, "PULSE_PROVIDER_MSP_DOCKER_NETWORK=msp-startup-"+nonce+"-"+kind,
				)
				startupWrite(t, filepath.Join(dir, ".env"), strings.Join(env, "\n")+"\n")
				project := "msp-startup-" + nonce + "-" + kind
				prefix := []string{"compose", "--project-name", project, "-f", "docker-compose.yml", "-f", "startup.yml"}
				compose := func(args ...string) string {
					return startupCommand(t, dockerEnv, dir, "docker", append(append([]string{}, prefix...), args...)...)
				}
				t.Cleanup(func() {
					compose("down", "--volumes", "--remove-orphans")
					for _, resource := range []string{"container", "network", "volume"} {
						args := []string{resource, "ls", "-q", "--filter", "label=com.docker.compose.project=" + project}
						if resource == "container" {
							args = append(args, "-a")
						}
						if remaining := startupCommand(t, dockerEnv, dir, "docker", args...); strings.TrimSpace(remaining) != "" {
							t.Errorf("owned %s remain after cleanup: %s", resource, remaining)
						}
					}
				})
				run = func(args ...string) string {
					return compose(append([]string{"run", "--rm", "--no-deps", "control-plane"}, args...)...)
				}
				compose("config", "--quiet")
				if refusal != "" {
					startupRefusal(t, dockerEnv, dir, refusal, "docker", append(prefix, "run", "--rm", "--no-deps", "control-plane")...)
					return
				}
				start = func() (string, func()) {
					compose("up", "--no-deps", "-d", "control-plane")
					id := strings.TrimSpace(compose("ps", "-q", "control-plane"))
					if id == "" {
						t.Fatal("Compose did not return the daemon container identity")
					}
					if licensePath == "" {
						// This is the actual /dev/null secret bind mount, not config text.
						startupCommand(t, dockerEnv, dir, "docker", "exec", id, "sh", "-c", "test -c /run/secrets/provider_msp_license && test $(wc -c < /run/secrets/provider_msp_license) -eq 0")
					} else {
						mountHash := startupCommand(t, dockerEnv, dir, "docker", "exec", id, "sha256sum", "/run/secrets/provider_msp_license")
						if !strings.HasPrefix(mountHash, fmt.Sprintf("%x", sha256.Sum256([]byte(license)))) {
							t.Fatal("mounted synthetic licence does not match the host file")
						}
					}
					address := strings.TrimSpace(compose("port", "control-plane", "8443"))
					stopped := false
					return "http://" + address, func() {
						if stopped {
							return
						}
						stopped = true
						compose("stop", "--timeout", "10", "control-plane")
						state := startupCommand(t, dockerEnv, dir, "docker", "inspect", "--format", "{{.State.Running}} {{.State.ExitCode}}", id)
						if strings.TrimSpace(state) != "false 0" {
							t.Errorf("daemon stop must be clean, got %s", state)
						}
					}
				}
			}
			accountID, ownerID := "", ""
			for restart := 0; restart < 2; restart++ {
				address, stop := start()
				// Cleanup is retained even when readiness or readback fails.
				t.Cleanup(stop)
				startupReady(t, address)
				startupStatus(t, address, plan, source, limit)
				args := []string{"provider-msp", "bootstrap", "--account-name=Startup acceptance", "--owner-email=operator@example.invalid", "--magic-link=false"}
				if accountID != "" {
					args = append(args, "--account-id="+accountID)
				}
				facts := startupFacts(run(args...))
				if facts["provider_msp_bootstrap_ok"] != "true" || facts["plan_version"] != plan || facts["plan_source"] != source || facts["workspace_limit"] != strconv.Itoa(limit) || facts["account_id"] == "" || facts["owner_user_id"] == "" {
					t.Fatalf("bootstrap readback differs: %v", facts)
				}
				if restart == 1 && (facts["account_id"] != accountID || facts["owner_user_id"] != ownerID) {
					t.Fatal("account/owner identity did not persist across daemon restart")
				}
				accountID, ownerID = facts["account_id"], facts["owner_user_id"]
				for observation := 0; observation < 3; observation++ {
					startupReady(t, address)
					startupStatus(t, address, plan, source, limit)
				}
				stop()
			}
		})
	}
}

func startupWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func startupCommand(t *testing.T, env []string, dir, program string, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, program, args...)
	cmd.Dir, cmd.Env = dir, env
	// Compose may warn on stderr even for a successful ps/port/config read.
	// Only stdout is machine readback: a warning must never become a container
	// identity, URL, mount digest, bootstrap fact or owned-resource observation.
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if err != nil {
		// Environment and licence contents are never diagnostic output.
		t.Fatalf("%s command failed: %v\nstdout:\n%s\nstderr:\n%s", filepath.Base(program), err, stdout.String(), stderr.String())
	}
	if stderr.Len() != 0 {
		t.Logf("%s stderr: %s", filepath.Base(program), stderr.String())
	}
	return stdout.String()
}

func startupRefusal(t *testing.T, env []string, dir, want, program string, args ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, program, args...)
	cmd.Dir, cmd.Env = dir, env
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil || err == nil || !strings.Contains(string(output), want) {
		t.Fatalf("expected bounded daemon refusal containing %q, err=%v timeout=%v\n%s", want, err, ctx.Err(), output)
	}
}

func startupProcess(t *testing.T, binary, dir string, env []string, address string) (string, func()) {
	t.Helper()
	cmd := exec.Command(binary)
	cmd.Dir, cmd.Env = dir, env
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	stopped := false
	return address, func() {
		if stopped {
			return
		}
		stopped = true
		if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
			t.Errorf("signal owned daemon: %v", err)
		}
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("daemon exit: %v\n%s", err, output.String())
			}
		case <-time.After(10 * time.Second):
			_ = cmd.Process.Kill()
			<-done
			t.Error("owned daemon did not terminate gracefully within 10 seconds")
		}
	}
}

func startupReady(t *testing.T, address string) {
	t.Helper()
	client := &http.Client{Timeout: time.Second}
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := client.Get(address + "/readyz")
		if err == nil {
			body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1024))
			resp.Body.Close()
			if readErr == nil && resp.StatusCode == http.StatusOK && string(body) == "ready" {
				return
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("daemon did not become persistently ready within 20 seconds")
}

func startupStatus(t *testing.T, address, plan, source string, limit int) {
	t.Helper()
	client := &http.Client{Timeout: time.Second}
	for _, authenticated := range []bool{false, true} {
		req, err := http.NewRequest(http.MethodGet, address+"/status", nil)
		if err != nil {
			t.Fatal(err)
		}
		if authenticated {
			req.Header.Set("Authorization", "Bearer "+strings.Repeat("synthetic-startup-", 3))
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 16*1024))
		resp.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if !authenticated {
			if resp.StatusCode != http.StatusUnauthorized {
				t.Fatalf("operator status must remain private, got %d", resp.StatusCode)
			}
			continue
		}
		var status struct {
			Version string `json:"version"`
			Mode    string `json:"control_plane_mode"`
			Plan    string `json:"provider_msp_plan_version"`
			Source  string `json:"provider_msp_plan_source"`
			Limit   int    `json:"provider_msp_workspace_limit"`
		}
		if err := json.Unmarshal(body, &status); err != nil || resp.StatusCode != http.StatusOK || status.Version != "msp-startup-integration" || status.Mode != "provider_hosted_msp" || status.Plan != plan || status.Source != source || status.Limit != limit {
			t.Fatalf("daemon status=%+v HTTP=%d err=%v", status, resp.StatusCode, err)
		}
	}
}

func startupFacts(output string) map[string]string {
	facts := make(map[string]string)
	for _, line := range strings.Split(output, "\n") {
		if key, value, ok := strings.Cut(line, "="); ok {
			facts[key] = value
		}
	}
	return facts
}
