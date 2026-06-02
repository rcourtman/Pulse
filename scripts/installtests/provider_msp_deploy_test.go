package installtests

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

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
	assertContainsAll(t, text,
		"CP_CONTROL_PLANE_MODE=provider_hosted_msp",
		"CP_DATA_DIR=${PULSE_PROVIDER_MSP_DATA_DIR:-/data}",
		"CP_PROVIDER_MSP_LICENSE_FILE=/run/secrets/provider_msp_license",
		"CP_DOCKER_NETWORK=${PULSE_PROVIDER_MSP_DOCKER_NETWORK:-pulse-provider-msp}",
		"CP_STORAGE_DATA_PATH=${PULSE_PROVIDER_MSP_DATA_DIR:-/data}",
		"${PULSE_PROVIDER_MSP_DATA_DIR:-/data}:${PULSE_PROVIDER_MSP_DATA_DIR:-/data}",
		"${PULSE_PROVIDER_MSP_DOCKER_SOCKET:-/var/run/docker.sock}:/var/run/docker.sock",
		"provider_msp_license:",
		"name: ${PULSE_PROVIDER_MSP_DOCKER_NETWORK:-pulse-provider-msp}",
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
		"PULSE_PROVIDER_MSP_DOCKER_SOCKET=/var/run/docker.sock",
		"PULSE_PROVIDER_MSP_HOST_ROOT=/",
		"PULSE_PROVIDER_MSP_DOCKER_DATA_DIR=/var/lib/docker",
		"CP_PROVIDER_MSP_LICENSE_FILE=./provider-msp-license.jwt",
		"CP_TRIAL_ACTIVATION_PRIVATE_KEY=",
		"sudo -E ./setup.sh",
		"docker compose run --rm control-plane provider-msp bootstrap",
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
		"CP_TRIAL_ACTIVATION_PRIVATE_KEY",
		"PULSE_PROVIDER_MSP_DATA_DIR",
		"PULSE_PROVIDER_MSP_DOCKER_NETWORK",
		"PULSE_PROVIDER_MSP_DOCKER_SOCKET",
		"PULSE_PROVIDER_MSP_HOST_ROOT",
		"PULSE_PROVIDER_MSP_DOCKER_DATA_DIR",
		"must be an absolute path",
		"must point to a reachable Docker socket",
		"must point to the host Docker data directory",
		"must not be configured in provider-hosted MSP mode",
		"CP_ALLOW_DOCKERLESS_PROVISIONING must be false",
		"CP_STORAGE_GUARDRAILS_ENABLED must be true",
		"docker compose config --quiet",
		"docker compose pull traefik control-plane",
		"PULSE_PROVIDER_MSP_ACCOUNT_NAME",
		"PULSE_PROVIDER_MSP_OWNER_EMAIL",
		"./run-install-proof.sh",
	)
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
		"docker compose pull traefik control-plane",
		"docker compose up -d traefik control-plane",
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
		"docker compose pull traefik control-plane",
		"provider-msp install-proof",
		"--account-name",
		"--owner-email",
		"--workspace-count",
		"--install-type",
		"--target-path",
		"--skip-image-pull",
		"${#extra_install_args[@]}",
		"docker compose run --rm --no-deps control-plane",
		"docker compose up -d traefik control-plane",
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
		"FROM --platform=$BUILDPLATFORM golang:1.25.9-alpine@sha256:5caaf1cca9dc351e13deafbc3879fd4754801acba8653fa9540cea125d01a71f AS builder",
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

func assertNotContainsAny(t *testing.T, text string, forbidden ...string) {
	t.Helper()
	for _, needle := range forbidden {
		if strings.Contains(text, needle) {
			t.Fatalf("forbidden %q found in:\n%s", needle, text)
		}
	}
}
