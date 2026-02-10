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
	kvSecretRE = regexp.MustCompile(`(?i)\b(password|passwd|passphrase|secret|token|api[_-]?key|client[_-]?secret|private[_-]?key)\b\s*[:=]\s*(.+)$`)

	// Authorization bearer header.
	bearerRE = regexp.MustCompile(`(?i)\bauthorization\s*:\s*bearer\s+([A-Za-z0-9\-._~+/]+=*)`)

	// Common token formats to reduce accidental leakage even when not in k=v form.
	awsAccessKeyRE = regexp.MustCompile(`\b(AKIA|ASIA)[0-9A-Z]{16}\b`)
	jwtRE          = regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\b`)
)

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

		if bearerRE.MatchString(line) {
			lines[i] = bearerRE.ReplaceAllString(line, "Authorization: Bearer [REDACTED]")
			redactions++
			continue
		}

		if awsAccessKeyRE.MatchString(line) {
			lines[i] = awsAccessKeyRE.ReplaceAllString(line, "[REDACTED_AWS_ACCESS_KEY]")
			redactions++
		}
		if jwtRE.MatchString(lines[i]) {
			lines[i] = jwtRE.ReplaceAllString(lines[i], "[REDACTED_JWT]")
			redactions++
		}
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
