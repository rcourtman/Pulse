package api

import (
	"fmt"
	"net/url"
	"strings"
)

// validateCustomURL validates a custom URL string.
// Returns an error message if invalid, or empty string if valid or empty input.
func validateCustomURL(rawURL string) string {
	if rawURL == "" {
		return ""
	}
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "Invalid URL format: " + err.Error()
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return "URL must use http:// or https:// scheme"
	}
	if parsedURL.Host == "" {
		return "Invalid URL: missing host/domain (e.g., use https://192.168.1.100:8006 or https://myhost.local)"
	}
	if strings.HasSuffix(parsedURL.Host, ".") && !strings.Contains(parsedURL.Host, "..") {
		return fmt.Sprintf("Incomplete URL: '%s' - please enter a complete domain or IP address", rawURL)
	}
	return ""
}

// metadataSaveErrorMessage returns a user-friendly error message for metadata save failures.
func metadataSaveErrorMessage(err error) string {
	if strings.Contains(err.Error(), "permission") {
		return "Permission denied - check file permissions"
	}
	if strings.Contains(err.Error(), "no space") {
		return "Disk full - cannot save metadata"
	}
	return "Failed to save metadata"
}
