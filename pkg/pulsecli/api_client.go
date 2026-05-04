package pulsecli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

const (
	defaultPulseAPIURL        = "http://127.0.0.1:7655"
	maxPulseAPIResponseBytes  = 1 << 20
	maxPulseAPIErrorBodyChars = 4096
)

type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

func cliHTTPClient(client HTTPDoer) HTTPDoer {
	if client != nil {
		return client
	}
	return http.DefaultClient
}

func cliGetenv(getenv func(string) string, key string) string {
	if getenv != nil {
		return getenv(key)
	}
	return os.Getenv(key)
}

func pulseAPIEndpoint(raw, apiPath string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("api url is required (use --api-url or PULSE_API_URL)")
	}
	if !strings.HasPrefix(apiPath, "/") {
		return nil, fmt.Errorf("api path must start with /")
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid api url: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("invalid api url: scheme must be http or https")
	}
	if parsed.Host == "" {
		return nil, fmt.Errorf("invalid api url: host is required")
	}

	apiPath = strings.TrimRight(apiPath, "/")
	fullPath := "/api" + apiPath
	path := strings.TrimRight(parsed.Path, "/")
	switch {
	case path == "":
		parsed.Path = fullPath
	case path == fullPath || strings.HasSuffix(path, fullPath):
		parsed.Path = path
	case path == "/api" || strings.HasSuffix(path, "/api"):
		parsed.Path = path + apiPath
	default:
		parsed.Path = path + fullPath
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed, nil
}

func apiStatusError(operation, status string, body []byte) error {
	if strings.TrimSpace(operation) == "" {
		operation = "request"
	}
	message := strings.TrimSpace(string(body))
	if message == "" {
		return fmt.Errorf("%s failed: %s", operation, status)
	}
	if len(message) > maxPulseAPIErrorBodyChars {
		message = message[:maxPulseAPIErrorBodyChars] + "..."
	}
	return fmt.Errorf("%s failed: %s: %s", operation, status, message)
}

func decodeJSONBytes(data []byte, out any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(out); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return fmt.Errorf("invalid trailing JSON content")
		}
		return err
	}
	return nil
}

func decodeJSONString(data string, out any) error {
	decoder := json.NewDecoder(strings.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(out); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return fmt.Errorf("invalid trailing JSON content")
		}
		return err
	}
	return nil
}
