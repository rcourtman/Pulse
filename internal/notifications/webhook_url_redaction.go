package notifications

import (
	"errors"
	"net/url"
	"strings"
)

const invalidWebhookURLDiagnostic = "[invalid webhook URL]"

// RedactWebhookURLSecrets masks credentials commonly embedded in webhook URLs
// while preserving the URL shape needed for operator diagnostics.
func RedactWebhookURLSecrets(urlString string) string {
	// Userinfo can contain a password or a credential used as the username.
	// Do not use URL.Redacted: it preserves usernames. Fail closed on invalid
	// URLs rather than returning unparsed credentials to diagnostic callers.
	parsed, err := url.Parse(urlString)
	if err != nil {
		return invalidWebhookURLDiagnostic
	}
	if parsed.User != nil {
		parsed.User = url.User("REDACTED")
		urlString = parsed.String()
	}

	// Slack incoming webhook paths are credentials, including legacy paths.
	// Match the parsed host, not a substring, and discard RawPath so escaped
	// credentials cannot survive URL.String(). Do not change the destination.
	switch strings.ToLower(parsed.Hostname()) {
	case "hooks.slack.com", "hooks.slack-gov.com":
		if strings.HasPrefix(parsed.Path, "/services/") {
			parsed.Path = "/services/REDACTED"
		} else {
			parsed.Path = "/REDACTED"
		}
		parsed.RawPath = ""
		urlString = parsed.String()
	}

	// Telegram bot credentials are path components rather than query values.
	if idx := strings.Index(urlString, "/bot"); idx != -1 {
		if endIdx := strings.Index(urlString[idx+4:], "/"); endIdx != -1 {
			urlString = urlString[:idx+4] + "REDACTED" + urlString[idx+4+endIdx:]
		} else if queryIdx := strings.Index(urlString[idx+4:], "?"); queryIdx != -1 {
			urlString = urlString[:idx+4] + "REDACTED" + urlString[idx+4+queryIdx:]
		} else {
			urlString = urlString[:idx+4] + "REDACTED"
		}
	}

	queryIndex := strings.Index(urlString, "?")
	if queryIndex == -1 {
		return urlString
	}

	for _, parameter := range []string{"token", "apikey", "api_key", "key", "secret", "password"} {
		pattern := parameter + "="
		searchStart := queryIndex
		for {
			parameterIndex := strings.Index(urlString[searchStart:], pattern)
			if parameterIndex == -1 {
				break
			}
			parameterIndex += searchStart

			if parameterIndex > 0 {
				previous := urlString[parameterIndex-1]
				if previous != '?' && previous != '&' {
					searchStart = parameterIndex + len(pattern)
					continue
				}
			}

			valueStart := parameterIndex + len(pattern)
			valueEnd := valueStart
			for valueEnd < len(urlString) && urlString[valueEnd] != '&' && urlString[valueEnd] != '#' {
				valueEnd++
			}
			urlString = urlString[:valueStart] + "REDACTED" + urlString[valueEnd:]
			searchStart = valueStart + len("REDACTED")
		}
	}

	return urlString
}

// RedactWebhookDiagnosticSecrets masks webhook URLs embedded in diagnostic
// text while retaining the non-secret context around them. A malformed URL
// still fails closed rather than returning potentially sensitive text.
func RedactWebhookDiagnosticSecrets(message string) string {
	lowerMessage := strings.ToLower(message)
	cursor := 0
	foundURL := false
	var redacted strings.Builder

	for cursor < len(message) {
		httpOffset := strings.Index(lowerMessage[cursor:], "http://")
		httpsOffset := strings.Index(lowerMessage[cursor:], "https://")
		offset := httpOffset
		if offset == -1 || (httpsOffset != -1 && httpsOffset < offset) {
			offset = httpsOffset
		}
		if offset == -1 {
			break
		}

		start := cursor + offset
		end := len(message)
		if whitespace := strings.IndexAny(message[start:], " \t\r\n"); whitespace != -1 {
			end = start + whitespace
		}

		redactedURL := RedactWebhookURLSecrets(message[start:end])
		if redactedURL == invalidWebhookURLDiagnostic {
			return invalidWebhookURLDiagnostic
		}
		redacted.WriteString(message[cursor:start])
		redacted.WriteString(redactedURL)
		cursor = end
		foundURL = true
	}

	if !foundURL {
		return RedactWebhookURLSecrets(message)
	}
	redacted.WriteString(message[cursor:])
	return redacted.String()
}

func redactWebhookTransportError(err error) error {
	if err == nil {
		return nil
	}

	var urlError *url.Error
	if !errors.As(err, &urlError) {
		return err
	}

	redacted := *urlError
	redacted.URL = RedactWebhookURLSecrets(urlError.URL)
	return &redacted
}
