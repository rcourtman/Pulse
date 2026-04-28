package cloudcp

import (
	"net/url"
	"strings"
)

func buildCPURL(baseURL, path string, query url.Values) string {
	base := strings.TrimSpace(baseURL)
	if base == "" {
		return path
	}
	parsed, err := url.Parse(base)
	if err != nil {
		return path
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + path
	parsed.RawQuery = query.Encode()
	return parsed.String()
}
