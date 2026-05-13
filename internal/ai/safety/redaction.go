package safety

import (
	"regexp"
	"strings"
)

var (
	// Match a PEM block header/footer. Redact the whole block because it is almost always sensitive.
	pemBeginRE = regexp.MustCompile(`(?m)^-----BEGIN [A-Z0-9 ][A-Z0-9 ]+-----\s*$`)
	pemEndRE   = regexp.MustCompile(`(?m)^-----END [A-Z0-9 ][A-Z0-9 ]+-----\s*$`)

	// Common secret-bearing key/value patterns.
	kvSecretRE = regexp.MustCompile(`(?i)\b(password|passwd|passphrase|secret|token|api[_-]?key|client[_-]?secret|private[_-]?key|access[_-]?token|refresh[_-]?token)\b\s*[:=]\s*(.+)$`)

	// Structured, URL, and header forms that commonly appear in prompts,
	// provider errors, tool schemas, and operator-entered handoff context.
	quotedKVSecretRE = regexp.MustCompile(`(?i)("(?:password|passwd|passphrase|secret|token|api[_-]?key|apikey|client[_-]?secret|private[_-]?key|access[_-]?token|refresh[_-]?token|authorization|x-api-key|credential)"\s*:\s*")[^"]+`)
	querySecretRE    = regexp.MustCompile(`(?i)([?&](?:key|api[_-]?key|apikey|access[_-]?token|refresh[_-]?token|token|client[_-]?secret|secret)=)[^\s&"']+`)
	bearerRE         = regexp.MustCompile(`(?i)(\bauthorization\s*:\s*bearer\s+)([A-Za-z0-9\-._~+/]+=*)`)
	xAPIKeyHeaderRE  = regexp.MustCompile(`(?i)(\bx-api-key\s*:\s*)[^\s,;]+`)
	urlUserInfoRE    = regexp.MustCompile(`(?i)(https?://)[^\s/@:]+:[^\s/@]+@`)

	// Common token formats to reduce accidental leakage even when not in k=v form.
	awsAccessKeyRE = regexp.MustCompile(`\b(AKIA|ASIA)[0-9A-Z]{16}\b`)
	jwtRE          = regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\b`)
	openAIKeyRE    = regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{8,}\b`)
	googleAPIKeyRE = regexp.MustCompile(`\bAIza[0-9A-Za-z_-]{10,}\b`)
	githubTokenRE  = regexp.MustCompile(`\bgh[opsur]_[A-Za-z0-9_]{10,}\b`)
)

const redactedSecretValue = "[REDACTED]"

// RedactSensitiveText removes likely-secret material from text outputs to reduce accidental
// key/token leakage through AI tool results. It is intentionally conservative: if a value
// looks sensitive, it will be replaced.
//
// Returns (redactedText, redactionCount).
func RedactSensitiveText(input string) (string, int) {
	if input == "" {
		return input, 0
	}

	lines := strings.Split(input, "\n")
	redactions := 0

	inPEM := false
	for i, line := range lines {
		if !inPEM && pemBeginRE.MatchString(line) {
			inPEM = true
			lines[i] = "[REDACTED PEM BLOCK]"
			redactions++
			continue
		}
		if inPEM {
			// Keep replacing until end marker; we preserve only a single marker line.
			if pemEndRE.MatchString(line) {
				inPEM = false
			}
			lines[i] = ""
			continue
		}

		if m := kvSecretRE.FindStringSubmatchIndex(line); m != nil {
			// Replace only the value portion with a marker.
			// submatch 2 is the value.
			valueStart, valueEnd := m[4], m[5]
			if valueStart >= 0 && valueEnd >= 0 && valueEnd > valueStart {
				lines[i] = line[:valueStart] + "[REDACTED]"
				redactions++
				continue
			}
		}

		lines[i], redactions = redactLineSecretPatterns(lines[i], redactions)
	}

	// Drop empty lines introduced by PEM redaction.
	outLines := make([]string, 0, len(lines))
	for _, l := range lines {
		if l == "" {
			continue
		}
		outLines = append(outLines, l)
	}
	return strings.Join(outLines, "\n"), redactions
}

func redactLineSecretPatterns(line string, redactions int) (string, int) {
	var count int
	line, count = replaceAllCounting(quotedKVSecretRE, line, `${1}`+redactedSecretValue)
	redactions += count
	line, count = replaceAllCounting(querySecretRE, line, `${1}`+redactedSecretValue)
	redactions += count
	line, count = replaceAllCounting(bearerRE, line, `${1}`+redactedSecretValue)
	redactions += count
	line, count = replaceAllCounting(xAPIKeyHeaderRE, line, `${1}`+redactedSecretValue)
	redactions += count
	line, count = replaceAllCounting(urlUserInfoRE, line, `${1}`+redactedSecretValue+"@")
	redactions += count
	line, count = replaceAllCounting(awsAccessKeyRE, line, "[REDACTED_AWS_ACCESS_KEY]")
	redactions += count
	line, count = replaceAllCounting(jwtRE, line, "[REDACTED_JWT]")
	redactions += count
	line, count = replaceAllCounting(openAIKeyRE, line, "[REDACTED_PROVIDER_KEY]")
	redactions += count
	line, count = replaceAllCounting(googleAPIKeyRE, line, "[REDACTED_PROVIDER_KEY]")
	redactions += count
	line, count = replaceAllCounting(githubTokenRE, line, "[REDACTED_PROVIDER_TOKEN]")
	redactions += count
	return line, redactions
}

func replaceAllCounting(re *regexp.Regexp, input string, replacement string) (string, int) {
	matches := re.FindAllStringIndex(input, -1)
	if len(matches) == 0 {
		return input, 0
	}
	return re.ReplaceAllString(input, replacement), len(matches)
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
