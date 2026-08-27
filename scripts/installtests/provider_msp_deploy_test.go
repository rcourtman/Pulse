package installtests

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestProviderMSPControlPlaneImageConsumesExactCandidate(t *testing.T) {
	dockerfileBytes, err := os.ReadFile(repoFile("deploy", "provider-msp", "Dockerfile.control-plane"))
	if err != nil {
		t.Fatalf("read provider MSP control-plane Dockerfile: %v", err)
	}
	publishBytes, err := os.ReadFile(repoFile(".github", "workflows", "publish-docker.yml"))
	if err != nil {
		t.Fatalf("read Docker publisher: %v", err)
	}
	dockerfile := string(dockerfileBytes)
	publish := string(publishBytes)
	assertContainsAll(t, dockerfile,
		"FROM control-plane-runtime-foundation AS control_plane_prebuilt",
		"COPY --from=compiled_payload /binaries/pulse-control-plane-linux-${TARGETARCH:-amd64}",
		"FROM control-plane-runtime-foundation AS runtime",
	)
	assertContainsAll(t, publish,
		"Verify exact-candidate container payload",
		"target: control_plane_prebuilt",
		"compiled_payload=${{ runner.temp }}/release-container-payload/payload/compiled",
	)
	assertNotContainsAny(t, publish,
		"PULSE_LICENSE_PUBLIC_KEY",
		"PULSE_UPDATE_SIGNING_KEY",
		"pulse_license_public_key",
		"pulse_update_signing_key",
	)
}

func TestProviderMSPDeployComposeIsProviderModeAndStripeFree(t *testing.T) {
	composePath := repoFile("deploy", "provider-msp", "docker-compose.yml")
	composeBytes, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatalf("read provider MSP compose: %v", err)
	}
	var compose map[string]any
	if err := yaml.Unmarshal(composeBytes, &compose); err != nil {
		t.Fatalf("provider MSP compose must be valid YAML: %v", err)
	}
	text := string(composeBytes)
	assertContainsAny(t, text,
		"CP_CONTROL_PLANE_MODE=provider_hosted_msp",
		"CP_CONTROL_PLANE_MODE=${CP_CONTROL_PLANE_MODE:-provider_hosted_msp}",
	)
	assertContainsAll(t, text,
		"CP_DATA_DIR=${PULSE_PROVIDER_MSP_DATA_DIR:-/data}",
		"CP_PROVIDER_MSP_LICENSE_FILE=/run/secrets/provider_msp_license",
		"CP_DOCKER_NETWORK=${PULSE_PROVIDER_MSP_DOCKER_NETWORK:-pulse-provider-msp}",
		"CP_TRUSTED_PROXY_CIDRS=${CP_TRUSTED_PROXY_CIDRS}",
		"DOCKER_HOST=tcp://docker-socket-proxy:2375",
		"CP_STORAGE_DATA_PATH=${PULSE_PROVIDER_MSP_DATA_DIR:-/data}",
		"CP_STORAGE_ROOT_PATH=/storage-root-spacecheck",
		"CP_STORAGE_DOCKER_PATH=/storage-docker-spacecheck",
		"${PULSE_PROVIDER_MSP_DATA_DIR:-/data}:${PULSE_PROVIDER_MSP_DATA_DIR:-/data}",
		"DOCKER_SOCKET_PROXY_IMAGE",
		"PING=1",
		"VERSION=1",
		"INFO=1",
		"${PULSE_PROVIDER_MSP_DOCKER_SOCKET:-/var/run/docker.sock}:/var/run/docker.sock:ro",
		"${PULSE_PROVIDER_MSP_ROOT_SPACECHECK_DIR:-/var/lib/pulse-provider-msp/spacecheck/root}:/storage-root-spacecheck:ro",
		"${PULSE_PROVIDER_MSP_DOCKER_SPACECHECK_DIR:-/var/lib/docker/.pulse-provider-msp-spacecheck}:/storage-docker-spacecheck:ro",
		"pulse.provider-msp.role=traefik",
		"pulse.provider-msp.role=control-plane",
		"provider_msp_license:",
		"name: ${PULSE_PROVIDER_MSP_DOCKER_NETWORK:-pulse-provider-msp}",
		"subnet: ${PULSE_PROVIDER_MSP_DOCKER_SUBNET:-172.30.0.0/24}",
	)
	assertNotContainsAny(t, text,
		":/host-root",
		":/host-var-lib-docker",
		"STRIPE_",
		"CP_TRIAL_SIGNUP_PRICE_ID",
		"CP_MSP_STARTER_PRICE_ID",
		"CP_MSP_GROWTH_PRICE_ID",
		"CP_MSP_SCALE_PRICE_ID",
		"CP_PUBLIC_CLOUD_SIGNUP_ENABLED",
	)
}

func TestProviderMSPDeployEnvExampleMatchesBootstrapPath(t *testing.T) {
	envBytes, err := os.ReadFile(repoFile("deploy", "provider-msp", ".env.example"))
	if err != nil {
		t.Fatalf("read provider MSP env example: %v", err)
	}
	text := string(envBytes)
	assertContainsAll(t, text,
		"CP_ENV=production",
		"PULSE_PROVIDER_MSP_DATA_DIR=/data",
		"PULSE_PROVIDER_MSP_DOCKER_NETWORK=pulse-provider-msp",
		"PULSE_PROVIDER_MSP_DOCKER_SUBNET=172.30.0.0/24",
		"PULSE_PROVIDER_MSP_DOCKER_SOCKET=/var/run/docker.sock",
		"PULSE_PROVIDER_MSP_ROOT_SPACECHECK_DIR=/var/lib/pulse-provider-msp/spacecheck/root",
		"PULSE_PROVIDER_MSP_DOCKER_SPACECHECK_DIR=/var/lib/docker/.pulse-provider-msp-spacecheck",
		"CP_TRUSTED_PROXY_CIDRS=172.30.0.0/24",
		// Ships blank on purpose: blank is evaluation. The exact blank form is
		// asserted in TestProviderMSPSetupScriptSupportsUnlicensedEvaluation.
		"CP_PROVIDER_MSP_LICENSE_FILE=",
		"CP_ENTITLEMENT_SIGNING_PRIVATE_KEY=",
		"ACME_DNS_PROVIDER=cloudflare",
		"sudo -E ./setup.sh",
		"docker compose run --rm control-plane provider-msp bootstrap",
		"docker compose run --rm control-plane provider-msp portal-link",
		"CP_SESSION_TTL",
		"docker compose run --rm control-plane provider-msp preflight",
		"docker compose run --rm control-plane provider-msp status",
		"docker compose run --rm control-plane provider-msp status --require-backup",
		"./upgrade.sh --dry-run",
		"./upgrade.sh",
		"./upgrade.sh --rollout-tenants",
		"./run-install-proof.sh",
		"docker compose run --rm control-plane provider-msp install-proof",
		"docker compose run --rm control-plane provider-msp recover --all-degraded --dry-run",
		"docker compose run --rm control-plane provider-msp recover --all-degraded",
		"docker compose run --rm control-plane provider-msp backup create",
		"docker compose run --rm control-plane provider-msp backup verify",
		"docker compose run --rm control-plane provider-msp backup restore",
		"--target-data-dir",
		"--dry-run",
		"docker compose run --rm control-plane provider-msp proof",
		"--account-name",
		"--owner-email",
		"--cleanup",
	)
	assertNotContainsAny(t, text,
		"STRIPE_",
		"CP_TRIAL_SIGNUP_PRICE_ID",
		"CP_MSP_STARTER_PRICE_ID",
		"CP_MSP_GROWTH_PRICE_ID",
		"CP_MSP_SCALE_PRICE_ID",
		"CP_PUBLIC_CLOUD_SIGNUP_ENABLED",
	)
}

func TestProviderMSPSetupScriptMatchesProviderContract(t *testing.T) {
	scriptPath := repoFile("deploy", "provider-msp", "setup.sh")
	result := exec.Command("bash", "-n", scriptPath)
	if output, err := result.CombinedOutput(); err != nil {
		t.Fatalf("provider MSP setup shell syntax failed: %v\n%s", err, output)
	}
	scriptBytes, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("read provider MSP setup: %v", err)
	}
	text := string(scriptBytes)
	assertContainsAll(t, text,
		"PULSE_PROVIDER_MSP_INSTALL_DIR",
		"PULSE_PROVIDER_MSP_DATA_DIR",
		"PULSE_PROVIDER_MSP_DOCKER_NETWORK",
		"PULSE_PROVIDER_MSP_BUNDLE_URL",
		"docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin",
		"docker-compose.yml",
		"traefik.yml",
		"traefik-dynamic.yml",
		".env.example",
		"run-install-proof.sh",
		"upgrade.sh",
		"CP_PROVIDER_MSP_LICENSE_FILE",
		"CP_ENTITLEMENT_SIGNING_PRIVATE_KEY",
		"PULSE_PROVIDER_MSP_DATA_DIR",
		"PULSE_PROVIDER_MSP_DOCKER_NETWORK",
		"PULSE_PROVIDER_MSP_DOCKER_SUBNET",
		"PULSE_PROVIDER_MSP_DOCKER_SOCKET",
		"PULSE_PROVIDER_MSP_ROOT_SPACECHECK_DIR",
		"PULSE_PROVIDER_MSP_DOCKER_SPACECHECK_DIR",
		"CP_TRUSTED_PROXY_CIDRS",
		"DOCKER_SOCKET_PROXY_IMAGE",
		"must be an absolute path",
		"must point to a reachable Docker socket",
		"CP_ADMIN_KEY must be at least 32 characters",
		"CP_TRUSTED_PROXY_CIDRS must include PULSE_PROVIDER_MSP_DOCKER_SUBNET",
		"169.254.169.254/32",
		"will be created by compose with subnet",
		"must not be configured in provider-hosted MSP mode",
		"CP_ALLOW_DOCKERLESS_PROVISIONING must be false",
		"CP_STORAGE_GUARDRAILS_ENABLED must be true",
		"docker compose config --quiet",
		`docker pull "${image_ref}"`,
		"PULSE_PROVIDER_MSP_ACCOUNT_NAME",
		"PULSE_PROVIDER_MSP_OWNER_EMAIL",
		"./run-install-proof.sh",
		"Next step: create your operator account",
		"provider-msp portal-link --email",
	)
	assertNotContainsAny(t, text, "docker network create --subnet")
}

func TestProviderMSPUpgradeRunnerMatchesComposeContract(t *testing.T) {
	scriptPath := repoFile("deploy", "provider-msp", "upgrade.sh")
	result := exec.Command("bash", "-n", scriptPath)
	if output, err := result.CombinedOutput(); err != nil {
		t.Fatalf("provider MSP upgrade runner shell syntax failed: %v\n%s", err, output)
	}
	scriptBytes, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("read provider MSP upgrade runner: %v", err)
	}
	text := string(scriptBytes)
	assertContainsAll(t, text,
		"PROVIDER_MSP_UPGRADE_DRY_RUN",
		"PROVIDER_MSP_UPGRADE_ROLLOUT_TENANTS",
		"PROVIDER_MSP_UPGRADE_PRUNE_PREVIOUS",
		"PROVIDER_MSP_UPGRADE_BACKUP_OUTPUT",
		"PROVIDER_MSP_UPGRADE_RESTORE_TARGET",
		"PROVIDER_MSP_UPGRADE_RUN_ID",
		"PROVIDER_MSP_UPGRADE_HEALTH_TIMEOUT",
		"docker compose config --quiet",
		"docker version >/dev/null",
		"provider-msp preflight",
		"provider-msp status",
		"provider-msp status --require-backup",
		"provider-msp backup create",
		"provider-msp backup verify",
		"provider-msp backup restore",
		"--target-data-dir",
		"tenant-runtime rollout --all --image",
		"tenant-runtime rollout",
		"--all",
		"--image",
		"--run-id",
		"--health-timeout",
		"--prune-previous",
		"docker compose pull traefik docker-socket-proxy control-plane",
		"docker compose up -d traefik docker-socket-proxy control-plane",
		"docker compose run --rm --no-deps control-plane",
		"provider_msp_upgrade_ok=true",
		"tenant_runtime_rollout_applied=true",
		"tenant_runtime_rollout_applied=false",
	)
	assertNotContainsAny(t, text,
		"STRIPE_",
		"CP_PUBLIC_CLOUD_SIGNUP_ENABLED",
	)
}

func TestProviderMSPInstallProofRunnerMatchesComposeContract(t *testing.T) {
	scriptPath := repoFile("deploy", "provider-msp", "run-install-proof.sh")
	result := exec.Command("bash", "-n", scriptPath)
	if output, err := result.CombinedOutput(); err != nil {
		t.Fatalf("provider MSP install-proof runner shell syntax failed: %v\n%s", err, output)
	}
	scriptBytes, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("read provider MSP install-proof runner: %v", err)
	}
	text := string(scriptBytes)
	assertContainsAll(t, text,
		"docker compose config --quiet",
		"docker version >/dev/null",
		"docker compose pull traefik docker-socket-proxy control-plane",
		"docker compose up -d traefik docker-socket-proxy",
		"provider-msp install-proof",
		"--account-name",
		"--owner-email",
		"--workspace-count",
		"--install-type",
		"--target-path",
		"--skip-image-pull",
		"${#extra_install_args[@]}",
		"docker compose run --rm --no-deps control-plane",
		"docker compose up -d traefik docker-socket-proxy control-plane",
		"provider-msp status",
	)
	assertNotContainsAny(t, text,
		"STRIPE_",
		"CP_PUBLIC_CLOUD_SIGNUP_ENABLED",
	)
}

func TestProviderMSPTraefikUsesProviderNetwork(t *testing.T) {
	traefikBytes, err := os.ReadFile(repoFile("deploy", "provider-msp", "traefik.yml"))
	if err != nil {
		t.Fatalf("read provider MSP Traefik config: %v", err)
	}
	var cfg map[string]any
	if err := yaml.Unmarshal(traefikBytes, &cfg); err != nil {
		t.Fatalf("provider MSP Traefik config must be valid YAML: %v", err)
	}
	assertContainsAll(t, string(traefikBytes),
		"network: pulse-provider-msp",
		"certificatesResolvers:",
		"letsencrypt:",
		"le:",
	)
}

func TestProviderMSPControlPlaneDockerfileBuildsReleaseLicenseBinary(t *testing.T) {
	dockerfileBytes, err := os.ReadFile(repoFile("deploy", "provider-msp", "Dockerfile.control-plane"))
	if err != nil {
		t.Fatalf("read control-plane Dockerfile: %v", err)
	}
	text := string(dockerfileBytes)
	assertContainsAll(t, text,
		"# syntax=docker/dockerfile:1.7",
		"FROM --platform=linux/amd64 node:20-alpine@sha256:fb4cd12c85ee03686f6af5362a0b0d56d50c58a04632e6c0fb8363f609372293 AS frontend-builder",
		"npm ci",
		"npm run build",
		"FROM --platform=$BUILDPLATFORM golang:1.26.7-alpine@sha256:28d89ee9cc0ff9fec75c82ca201e6bf7fdf9a679d4b7b24dfa04f2bb766bb468 AS builder",
		"FROM alpine:3.20@sha256:d9e853e87e55526f6b2917df91a2115c36dd7c696a35be12163d44e6e2a4b6bc",
		"ARG PULSE_LICENSE_PUBLIC_KEY_SHA256",
		"ARG TARGETOS",
		"ARG TARGETARCH",
		"--mount=type=secret,id=pulse_license_public_key,required=false",
		"PULSE_LICENSE_PUBLIC_KEY_SHA256 is required for control-plane release image builds.",
		"PULSE_LICENSE_PUBLIC_KEY_SHA256 was provided but no license public key was mounted.",
		`LICENSE_PUBLIC_KEY="$(tr -d '\r\n' < /run/secrets/pulse_license_public_key)"`,
		"mounted license public key must decode to 32 bytes.",
		"mounted license public key does not match PULSE_LICENSE_PUBLIC_KEY_SHA256.",
		"COPY --from=frontend-builder /app/internal/api/frontend-modern/dist ./internal/api/frontend-modern/dist",
		`TARGET_GOOS="${TARGETOS:-linux}"`,
		`TARGET_GOARCH="${TARGETARCH:-$(go env GOARCH)}"`,
		`./scripts/release_ldflags.sh server --version "${VERSION}" --build-time "${BUILD_TIME}" --git-commit "${GIT_COMMIT}" --license-public-key "${LICENSE_PUBLIC_KEY}"`,
		`CGO_ENABLED=0 GOOS="${TARGET_GOOS}" GOARCH="${TARGET_GOARCH}" go build \`,
		"-tags release",
		"-buildvcs=false",
		"-trimpath",
		"-o /pulse-control-plane ./cmd/pulse-control-plane",
	)
	assertNotContainsAny(t, text,
		"golang:1.25.7-alpine AS builder",
		"FROM alpine:3.21",
		"CGO_ENABLED=0 go build -o /pulse-control-plane ./cmd/pulse-control-plane",
	)
}

func assertContainsAll(t *testing.T, text string, required ...string) {
	t.Helper()
	for _, needle := range required {
		if !strings.Contains(text, needle) {
			t.Fatalf("missing %q in:\n%s", needle, text)
		}
	}
}

func assertContainsAny(t *testing.T, text string, allowed ...string) {
	t.Helper()
	for _, needle := range allowed {
		if strings.Contains(text, needle) {
			return
		}
	}
	t.Fatalf("missing any of %q in:\n%s", allowed, text)
}

func assertNotContainsAny(t *testing.T, text string, forbidden ...string) {
	t.Helper()
	for _, needle := range forbidden {
		if strings.Contains(text, needle) {
			t.Fatalf("forbidden %q found in:\n%s", needle, text)
		}
	}
}

// The unlicensed path is both a commercial boundary and the whole conversion
// path, so it is pinned here rather than left to whoever next edits setup.sh.
//
// Before this, setup.sh required a licence file and four hand-supplied image
// digests shipped as unfillable "<pin>" placeholders. That put two human
// round-trips in front of a provider's first screen, for no technical reason:
// all four images are public, and the control plane already ran unlicensed on
// the environment fallback.
func TestProviderMSPSetupScriptSupportsUnlicensedEvaluation(t *testing.T) {
	scriptBytes, err := os.ReadFile(repoFile("deploy", "provider-msp", "setup.sh"))
	if err != nil {
		t.Fatalf("read provider MSP setup: %v", err)
	}
	script := string(scriptBytes)

	assertContainsAll(t, script,
		"evaluation mode, 2 client workspaces",
		"ensure_image_pins",
		"default_image_ref",
		"resolve_image_digest",
		"buildx imagetools inspect",
		`--format '{{json .Manifest}}'`,
		`jq -r 'if type == "object" then .digest // empty else empty end'`,
		`awk '$1 == "Digest:" {print $2; exit}'`,
		"validate_compose_config --allow-missing-evaluation-license",
		`CP_PROVIDER_MSP_LICENSE_FILE=/dev/null docker compose config --quiet`,
		`docker pull "${image_ref}"`,
		`if [[ "${current}" == *@sha256:*`,
		`ref="${current}"`,
		// Self-issue, and the three ways it must degrade instead of blocking.
		"ensure_eval_license",
		"/v1/provider-msp/eval-license",
		"PULSE_PROVIDER_MSP_SKIP_EVAL_LICENSE",
		"reusing existing evaluation license",
		"could not reach the license server",
		"PULSE_PROVIDER_MSP_EVAL_EMAIL",
		"PULSE_PROVIDER_MSP_SIGNUP_SOURCE",
		`setup_stage: "images_ready"`,
		"eval_license_id=",
	)
	if strings.LastIndex(script, "pull_provider_images\n") > strings.LastIndex(script, "ensure_eval_license\n") {
		t.Fatal("evaluation must be issued only after pinned provider images are reachable")
	}
	setupSequence := "validate_compose_config --allow-missing-evaluation-license\n  pull_provider_images\n  # Issue the evaluation only after the host is configured and the immutable\n  # images are reachable. This makes an issued evaluation a useful activation\n  # signal rather than a record created before setup can succeed.\n  ensure_eval_license\n  validate_compose_config"
	if !strings.Contains(script, setupSequence) {
		t.Fatal("setup must validate compose, pull pinned images, issue the evaluation, then validate compose with the installed licence")
	}

	// The install must never abort because an evaluation licence could not be
	// obtained. `|| true` inside the substitution does not achieve that: the
	// derive helper calls die, and exit in a subshell is not a catchable
	// status, so setup.sh would abort under set -e.
	if strings.Contains(script, "$(derive_lease_signing_public_key 2>/dev/null || true)") {
		t.Fatal("eval license key derivation uses `|| true` inside a command substitution, which cannot catch die/exit under set -e")
	}
	if !strings.Contains(script, "if ! public_key=\"$(derive_lease_signing_public_key 2>/dev/null)\"") {
		t.Fatal("eval license key derivation must be guarded so a failure degrades to unlicensed rather than aborting setup")
	}

	// A blank licence path means evaluation. If it returns to the
	// must-be-non-empty list, setup.sh refuses to start unlicensed again.
	if strings.Contains(script, "CP_TRUSTED_PROXY_CIDRS CP_PROVIDER_MSP_LICENSE_FILE") {
		t.Fatal("CP_PROVIDER_MSP_LICENSE_FILE is back in the required non-empty list; blank must mean evaluation")
	}

	envBytes, err := os.ReadFile(repoFile("deploy", "provider-msp", ".env.example"))
	if err != nil {
		t.Fatalf("read provider MSP env example: %v", err)
	}
	env := string(envBytes)

	if strings.Contains(env, "<pin>") {
		t.Fatal(".env.example still ships <pin> image placeholders; setup.sh resolves blank pins from published tags instead")
	}
	if !strings.Contains(env, "\nCP_PROVIDER_MSP_LICENSE_FILE=\n") {
		t.Fatal(".env.example must ship a blank CP_PROVIDER_MSP_LICENSE_FILE so the shipped default is evaluation")
	}
	for _, key := range []string{"TRAEFIK_IMAGE", "DOCKER_SOCKET_PROXY_IMAGE", "CONTROL_PLANE_IMAGE", "CP_PULSE_IMAGE"} {
		if !strings.Contains(env, "\n"+key+"=\n") {
			t.Fatalf(".env.example must ship %s blank so setup.sh resolves its digest", key)
		}
	}
}

func TestProviderMSPResolveImageDigestAcceptsManifestJSON(t *testing.T) {
	scriptBytes, err := os.ReadFile(repoFile("deploy", "provider-msp", "setup.sh"))
	if err != nil {
		t.Fatalf("read provider MSP setup: %v", err)
	}
	script := strings.Replace(string(scriptBytes), `main "$@"`, "", 1)
	if script == string(scriptBytes) {
		t.Fatal("provider MSP setup main invocation not found")
	}

	tempDir := t.TempDir()
	digest := "sha256:" + strings.Repeat("a", 64)
	fakeDocker := filepath.Join(tempDir, "docker")
	if err := os.WriteFile(fakeDocker, []byte("#!/bin/sh\nprintf '%s\\n' '{\"digest\":\""+digest+"\"}'\n"), 0o755); err != nil {
		t.Fatalf("write fake docker: %v", err)
	}
	runner := filepath.Join(tempDir, "resolve-image-digest.sh")
	if err := os.WriteFile(runner, []byte(script+"\nresolve_image_digest example.invalid/provider:v1\n"), 0o755); err != nil {
		t.Fatalf("write setup runner: %v", err)
	}

	cmd := exec.Command("bash", runner)
	cmd.Env = append(os.Environ(), "PATH="+tempDir+":"+os.Getenv("PATH"))
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("resolve image digest: %v\n%s", err, output)
	}
	want := "example.invalid/provider@" + digest
	if got := strings.TrimSpace(string(output)); got != want {
		t.Fatalf("resolved image = %q, want %q", got, want)
	}
}

func TestProviderMSPEvaluationDocsUsePublishedSignedBundle(t *testing.T) {
	repoDocBytes, err := os.ReadFile(repoFile("docs", "MSP.md"))
	if err != nil {
		t.Fatalf("read repo MSP guide: %v", err)
	}
	shippedDocBytes, err := os.ReadFile(repoFile("frontend-modern", "public", "docs", "MSP.md"))
	if err != nil {
		t.Fatalf("read shipped MSP guide: %v", err)
	}
	if string(repoDocBytes) != string(shippedDocBytes) {
		t.Fatal("repo and shipped MSP guides must remain byte-synchronized")
	}

	doc := string(repoDocBytes)
	assertContainsAll(t, doc,
		"signed provider bundle published",
		"with Pulse v6.2.1",
		"**not** download the moving `main` branch archive",
		`export PULSE_VERSION=v6.2.1`,
		`PULSE_MSP_BUNDLE="pulse-provider-msp-${PULSE_VERSION}.tar.gz"`,
		`releases/download/${PULSE_VERSION}`,
		"ssh-keygen -Y verify",
		`-s "${PULSE_MSP_BUNDLE}.sshsig" < "${PULSE_MSP_BUNDLE}"`,
		`sha256sum -c "${PULSE_MSP_BUNDLE}.sha256"`,
		"The v6.2.1 evaluation request is anonymous",
		"contact request remains separate from the licence activation",
		`sudo -E bash ./setup.sh`,
	)
	assertNotContainsAny(t, doc,
		"Pulse/archive/refs/heads/main.tar.gz",
		"cd Pulse-main/deploy/provider-msp",
		"sudo -E ./setup.sh",
	)
}

// Traefik terminates TLS at the internet edge. Handing it the whole operator
// .env put CP_ADMIN_KEY and the entitlement signing private key one edge CVE
// away from disclosure, so the compose contract pins the minimal wiring: only
// ACME/DNS material reaches the traefik container, and the DNS-01 provider is
// overridable for operators whose DNS is not on Cloudflare.
func TestProviderMSPTraefikEnvIsMinimalAndDNSProviderOverridable(t *testing.T) {
	composeBytes, err := os.ReadFile(repoFile("deploy", "provider-msp", "docker-compose.yml"))
	if err != nil {
		t.Fatalf("read provider MSP compose: %v", err)
	}
	var compose struct {
		Services map[string]struct {
			EnvFile any `yaml:"env_file"`
		} `yaml:"services"`
	}
	if err := yaml.Unmarshal(composeBytes, &compose); err != nil {
		t.Fatalf("provider MSP compose must be valid YAML: %v", err)
	}
	traefik, ok := compose.Services["traefik"]
	if !ok {
		t.Fatal("compose must define a traefik service")
	}
	var envFiles []string
	switch v := traefik.EnvFile.(type) {
	case nil:
	case string:
		envFiles = append(envFiles, v)
	case []any:
		for _, entry := range v {
			switch e := entry.(type) {
			case string:
				envFiles = append(envFiles, e)
			case map[string]any:
				if p, _ := e["path"].(string); p != "" {
					envFiles = append(envFiles, p)
				}
			}
		}
	}
	for _, f := range envFiles {
		if strings.HasSuffix(f, ".env") && !strings.HasSuffix(f, "dns-credentials.env") {
			t.Fatalf("traefik env_file %q would leak the operator .env (CP_ADMIN_KEY, entitlement signing key) into the edge container", f)
		}
	}

	text := string(composeBytes)
	assertContainsAll(t, text,
		"TRAEFIK_CERTIFICATESRESOLVERS_LETSENCRYPT_ACME_DNSCHALLENGE_PROVIDER=${ACME_DNS_PROVIDER:-cloudflare}",
		"TRAEFIK_CERTIFICATESRESOLVERS_LE_ACME_DNSCHALLENGE_PROVIDER=${ACME_DNS_PROVIDER:-cloudflare}",
		"CF_DNS_API_TOKEN=${CF_DNS_API_TOKEN:-}",
		"path: ./dns-credentials.env",
		"required: false",
	)

	scriptBytes, err := os.ReadFile(repoFile("deploy", "provider-msp", "setup.sh"))
	if err != nil {
		t.Fatalf("read provider MSP setup: %v", err)
	}
	assertContainsAll(t, string(scriptBytes),
		"ensure_dns_credentials_file",
		"CF_DNS_API_TOKEN is required with the default ACME_DNS_PROVIDER=cloudflare",
		"put that provider's credential variables in",
	)
	// CF_DNS_API_TOKEN must not return to the unconditional required list; it
	// is only required when ACME_DNS_PROVIDER resolves to cloudflare.
	if strings.Contains(string(scriptBytes), "ACME_EMAIL CF_DNS_API_TOKEN CP_ENV") {
		t.Fatal("CF_DNS_API_TOKEN is back in the unconditional required-env list; it must be required only when ACME_DNS_PROVIDER is cloudflare")
	}
}
