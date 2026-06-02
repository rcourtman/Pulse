package installtests

import (
	"os"
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
		"CP_PROVIDER_MSP_LICENSE_FILE=/run/secrets/provider_msp_license",
		"CP_DOCKER_NETWORK=pulse-provider-msp",
		"provider_msp_license:",
		"pulse-provider-msp:",
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
		"CP_PROVIDER_MSP_LICENSE_FILE=./provider-msp-license.jwt",
		"CP_TRIAL_ACTIVATION_PRIVATE_KEY=",
		"docker compose run --rm control-plane provider-msp bootstrap",
		"docker compose run --rm control-plane provider-msp preflight",
		"docker compose run --rm control-plane provider-msp status",
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
