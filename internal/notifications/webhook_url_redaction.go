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
	case "discord.com", "discordapp.com":
		// Discord webhook IDs and tokens follow /webhooks/ in both
		// unversioned and versioned API paths. Mask the entire suffix,
		// including escaped credentials and compatibility endpoint paths.
		if idx := strings.Index(parsed.Path, "/webhooks/"); idx != -1 {
			parsed.Path = parsed.Path[:idx] + "/webhooks/REDACTED"
			parsed.RawPath = ""
			urlString = parsed.String()
		}
	}

	// Telegram also supports local API servers, so retain host-independent
	// masking, but inspect only the decoded path. Searching the whole URL
	// misses escaped prefixes and can mistake hostnames or query URLs for
	// bot credentials. Clear RawPath to prevent escaped secrets resurfacing.
	if idx := strings.Index(parsed.Path, "/bot"); idx != -1 {
		end := len(parsed.Path)
		if suffix := strings.Index(parsed.Path[idx+4:], "/"); suffix != -1 {
			end = idx + 4 + suffix
		}
		parsed.Path = parsed.Path[:idx+4] + "REDACTED" + parsed.Path[end:]
		parsed.RawPath = ""
		urlString = parsed.String()
	}

	// Decode names exactly once, as net/url does, but retain the original
	// spelling, order and unrelated values in diagnostic URLs. Inspect every
	// occurrence rather than Query().Get(), which would miss repeated keys.
	parts := strings.Split(parsed.RawQuery, "&")
	changed := false
	for i, part := range parts {
		name, _, hasValue := strings.Cut(part, "=")
		decoded, err := url.QueryUnescape(name)
		if err != nil {
			return invalidWebhookURLDiagnostic
		}
		switch decoded {
		case "token", "apikey", "api_key", "key", "secret", "password":
			if hasValue {
				parts[i] = name + "=REDACTED"
				changed = true
			}
		}
	}
	if changed {
		parsed.RawQuery = strings.Join(parts, "&")
		return parsed.String()
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
