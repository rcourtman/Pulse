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
