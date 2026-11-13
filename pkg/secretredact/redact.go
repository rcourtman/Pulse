// Package secretredact removes likely secret material from free text. It has
// no dependencies beyond the standard library, so the agent can apply the
// same rules on the host that the server applies before text reaches a model.
package secretredact

import (
	"regexp"
	"strings"
)

var (
	// Match a PEM block header/footer. Redact the whole block because it is almost always sensitive.
	pemBeginRE = regexp.MustCompile(`(?m)^-----BEGIN [A-Z0-9 ][A-Z0-9 ]+-----\s*$`)
	pemEndRE   = regexp.MustCompile(`(?m)^-----END [A-Z0-9 ][A-Z0-9 ]+-----\s*$`)

	// Common secret-bearing key/value patterns. The key may carry an
	// identifier prefix, because environment-style names (DB_PASSWORD,
	// POSTGRES_PASSWORD, PULSE_TOKEN, JWT_SECRET) are the commonest shape in
	// container inspection, unit files and process listings. The sensitive
	// word must end the key, so counters such as token_count or max_tokens
	// are left alone.
	kvSecretRE = regexp.MustCompile(`(?i)([A-Za-z0-9_.-]*?(?:` + secretKeyWords + `))\s*[:=]\s*(.+)$`)

	// Structured, URL, and header forms that commonly appear in prompts,
	// provider errors, tool schemas, and operator-entered handoff context.
	quotedKVSecretRE = regexp.MustCompile(`(?i)("[A-Za-z0-9_.-]*?(?:` + secretKeyWords + `|authorization|credential)"\s*:\s*")[^"]+`)
	querySecretRE    = regexp.MustCompile(`(?i)([?&](?:key|api[_-]?key|apikey|access[_-]?token|refresh[_-]?token|token|client[_-]?secret|secret|password|passwd)=)[^\s&"']+`)
	bearerRE         = regexp.MustCompile(`(?i)(\bauthorization\s*:\s*bearer\s+)([A-Za-z0-9\-._~+/]+=*)`)
	// Command-line flags that take a credential as the next word, as seen in
	// process listings and unit files: --token VALUE, --db-password VALUE.
	// The flag must start a word, so prose such as "csrf-token mismatch" stays.
	cliFlagSecretRE = regexp.MustCompile(`(?i)((?:^|\s)--?[A-Za-z0-9_.-]*?(?:` + secretKeyWords + `)[ \t]+)[^\s-]\S*`)
	basicAuthRE     = regexp.MustCompile(`(?i)(\bauthorization\s*:\s*basic\s+)[A-Za-z0-9+/=._-]+`)
	xAPIKeyHeaderRE = regexp.MustCompile(`(?i)(\bx-api-key\s*:\s*)[^\s,;]+`)
	// Any URL scheme: database DSNs (postgres://, mysql://, redis://,
	// amqp://) carry credentials the same way HTTP URLs do.
	urlUserInfoRE = regexp.MustCompile(`(?i)\b([a-z][a-z0-9+.-]*://)[^\s/@:]+:[^\s/@]+@`)

	// Common token formats to reduce accidental leakage even when not in k=v form.
	awsAccessKeyRE = regexp.MustCompile(`\b(AKIA|ASIA)[0-9A-Z]{16}\b`)
	jwtRE          = regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\b`)
	openAIKeyRE    = regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{8,}\b`)
	googleAPIKeyRE = regexp.MustCompile(`\bAIza[0-9A-Za-z_-]{10,}\b`)
	githubTokenRE  = regexp.MustCompile(`\bgh[opsur]_[A-Za-z0-9_]{10,}\b`)
)

// Placeholder replaces a redacted value.
const Placeholder = "[REDACTED]"

const redactedSecretValue = Placeholder

// secretKeyWords are the key endings that mark a credential value.
const secretKeyWords = `password|passwd|passphrase|secret|token|api[_-]?key|apikey|access[_-]?key|secret[_-]?key(?:[_-]?base)?|auth[_-]?key|client[_-]?secret|private[_-]?key|access[_-]?token|refresh[_-]?token|credentials?`

// Text removes likely-secret material from text outputs to reduce accidental
// key/token leakage through AI tool results. It is intentionally conservative: if a value
// looks sensitive, it will be replaced.
//
// Returns (redactedText, redactionCount).
func Text(input string) (string, int) {
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
				// The text before the key can still hold other secret forms,
				// such as a proxy URL with credentials earlier on the line.
				var prefix string
				prefix, redactions = redactLineSecretPatterns(line[:valueStart], redactions)
				lines[i] = prefix + "[REDACTED]"
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
	line, count = replaceAllCounting(basicAuthRE, line, `${1}`+redactedSecretValue)
	redactions += count
	line, count = replaceAllCounting(cliFlagSecretRE, line, `${1}`+redactedSecretValue)
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
