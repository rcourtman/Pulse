package safety

import (
	"strings"
	"testing"
)

func TestRedactSensitiveTextCoversStructuredPromptSecrets(t *testing.T) {
	input := strings.Join([]string{
		`password: hunter2`,
		`{"api_key":"plain-json-secret","access_token":"access-token-value"}`,
		`GET https://example.test/v1?key=AIzaSySecretTokenValue&region=us`,
		`Authorization: Bearer sk-live-secret-token`,
		`x-api-key: sk-provider-secret`,
		`https://operator:password@example.test/v1`,
		`github=ghp_abcdefghijklmnopqrstuvwxyz`,
	}, "\n")

	redacted, count := RedactSensitiveText(input)
	if count == 0 {
		t.Fatal("expected redactions")
	}
	for _, forbidden := range []string{
		"hunter2",
		"plain-json-secret",
		"access-token-value",
		"AIzaSySecretTokenValue",
		"sk-live-secret-token",
		"sk-provider-secret",
		"operator:password@",
		"ghp_abcdefghijklmnopqrstuvwxyz",
	} {
		if strings.Contains(redacted, forbidden) {
			t.Fatalf("redacted text leaked %q:\n%s", forbidden, redacted)
		}
	}
	for _, retained := range []string{
		"password:",
		`"api_key":"[REDACTED]"`,
		"Authorization: Bearer [REDACTED]",
		"x-api-key: [REDACTED]",
		"https://[REDACTED]@example.test/v1",
	} {
		if !strings.Contains(redacted, retained) {
			t.Fatalf("redacted text missing retained context %q:\n%s", retained, redacted)
		}
	}
}

func TestRedactSensitiveFieldValueUsesKeyContext(t *testing.T) {
	redacted, count := RedactSensitiveFieldValue("client_secret", "plain-value-without-token-shape")
	if redacted != "[REDACTED]" {
		t.Fatalf("redacted value = %q, want marker", redacted)
	}
	if count == 0 {
		t.Fatal("expected key-context redaction count")
	}

	unchanged, count := RedactSensitiveFieldValue("display_name", "plain-value-without-token-shape")
	if unchanged != "plain-value-without-token-shape" || count != 0 {
		t.Fatalf("non-sensitive field redaction = %q count=%d", unchanged, count)
	}
}

func TestRedactSensitiveTextCoversEnvironmentStyleKeys(t *testing.T) {
	input := strings.Join([]string{
		`DB_PASSWORD=hunter2`,
		`        "POSTGRES_PASSWORD=hunter3",`,
		`PULSE_TOKEN=0123456789abcdef0123`,
		`SECRET_KEY_BASE=railsbasevalue`,
		`AWS_SECRET_ACCESS_KEY=awssecretvalue`,
		`{"db_password":"hunter4"}`,
		`Authorization: Basic dXNlcjpodW50ZXI1`,
		`DATABASE_URL=postgres://app:hunter6@db:5432/app`,
		`root  812  /usr/bin/pulse-agent --url https://pulse --token flagtokenvalue`,
	}, "\n")
	redacted, count := RedactSensitiveText(input)
	if count == 0 {
		t.Fatal("expected redactions")
	}
	for _, forbidden := range []string{
		"hunter2", "hunter3", "0123456789abcdef0123", "railsbasevalue", "awssecretvalue",
		"hunter4", "dXNlcjpodW50ZXI1", "hunter6", "flagtokenvalue",
	} {
		if strings.Contains(redacted, forbidden) {
			t.Fatalf("redacted text leaked %q:\n%s", forbidden, redacted)
		}
	}
	for _, retained := range []string{"DB_PASSWORD=", "PULSE_TOKEN=", "--token [REDACTED]", "postgres://[REDACTED]@db:5432/app"} {
		if !strings.Contains(redacted, retained) {
			t.Fatalf("redacted text missing retained context %q:\n%s", retained, redacted)
		}
	}

	// Counters and prose that only mention a sensitive word stay readable.
	for _, line := range []string{
		`prompt_tokens=120`,
		`max_tokens: 4096`,
		`token_count: 5`,
		`csrf-token mismatch for request`,
		`Failed password for root from 192.0.2.1 port 22 ssh2`,
	} {
		if out, n := RedactSensitiveText(line); out != line || n != 0 {
			t.Errorf("RedactSensitiveText(%q) = (%q, %d), want unchanged", line, out, n)
		}
	}
}
