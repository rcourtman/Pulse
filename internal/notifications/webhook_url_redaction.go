package notifications

import (
	"errors"
	"net/url"
	"strings"
)

// RedactWebhookURLSecrets masks credentials commonly embedded in webhook URLs
// while preserving the URL shape needed for operator diagnostics.
func RedactWebhookURLSecrets(urlString string) string {
	// Userinfo can contain a password or a credential used as the username.
	// Do not use URL.Redacted: it preserves usernames. Fail closed on invalid
	// URLs rather than returning unparsed credentials to diagnostic callers.
	parsed, err := url.Parse(urlString)
	if err != nil {
		return "[invalid webhook URL]"
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
