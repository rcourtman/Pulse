package notifications

import (
	"errors"
	"net/url"
	"strings"
)

// RedactWebhookURLSecrets masks credentials commonly embedded in webhook URLs
// while preserving the URL shape needed for operator diagnostics.
func RedactWebhookURLSecrets(urlString string) string {
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
