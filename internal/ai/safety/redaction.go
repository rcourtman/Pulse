package safety

import (
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/pkg/secretredact"
)

const redactedSecretValue = secretredact.Placeholder

// RedactSensitiveText removes likely-secret material from text outputs to reduce accidental
// key/token leakage through AI tool results. It is intentionally conservative: if a value
// looks sensitive, it will be replaced. The rules live in pkg/secretredact so the agent
// applies the same ones on the host.
//
// Returns (redactedText, redactionCount).
func RedactSensitiveText(input string) (string, int) {
	return secretredact.Text(input)
}

// RedactSensitiveFieldValue redacts text with key context. It catches bland
// values that are only secret-shaped because the surrounding field says so,
// such as a tool-call input named api_key.
func RedactSensitiveFieldValue(fieldName string, input string) (string, int) {
	if !IsSensitiveFieldName(fieldName) {
		return RedactSensitiveText(input)
	}
	return RedactSensitiveValue(input)
}

// RedactSensitiveValue redacts a structured value that the caller already
// knows belongs to a sensitive field or schema value slot.
func RedactSensitiveValue(input string) (string, int) {
	redacted, count := RedactSensitiveText(input)
	if strings.TrimSpace(redacted) == "" {
		return redacted, count
	}
	if redacted == redactedSecretValue {
		return redacted, count
	}
	return redactedSecretValue, count + 1
}

// IsSensitiveFieldName reports whether a structured key usually carries a
// credential or secret-bearing value.
func IsSensitiveFieldName(name string) bool {
	normalized := normalizedFieldName(name)
	if normalized == "" {
		return false
	}
	switch normalized {
	case "password", "passwd", "passphrase", "secret", "token", "apikey", "clientsecret",
		"privatekey", "accesstoken", "refreshtoken", "authorization", "xapikey", "credential",
		"credentials":
		return true
	default:
		return false
	}
}

// IsSensitiveValueCarrierFieldName reports whether a nested field commonly
// carries an example, default, or literal value for a sensitive schema property.
func IsSensitiveValueCarrierFieldName(name string) bool {
	normalized := normalizedFieldName(name)
	switch normalized {
	case "value", "values", "default", "defaults", "example", "examples", "enum", "const":
		return true
	default:
		return false
	}
}

func normalizedFieldName(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
