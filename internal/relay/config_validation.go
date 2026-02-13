package relay

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

func normalizeClientInputs(cfg Config, deps ClientDeps) (Config, ClientDeps, []string, error) {
	cfg.ServerURL = strings.TrimSpace(cfg.ServerURL)
	cfg.InstanceSecret = strings.TrimSpace(cfg.InstanceSecret)
	cfg.IdentityPrivateKey = strings.TrimSpace(cfg.IdentityPrivateKey)
	cfg.IdentityPublicKey = strings.TrimSpace(cfg.IdentityPublicKey)
	cfg.IdentityFingerprint = strings.TrimSpace(cfg.IdentityFingerprint)

	deps.LocalAddr = strings.TrimSpace(deps.LocalAddr)
	deps.ServerVersion = strings.TrimSpace(deps.ServerVersion)
	deps.IdentityPubKey = strings.TrimSpace(deps.IdentityPubKey)
	deps.IdentityPrivateKey = strings.TrimSpace(deps.IdentityPrivateKey)

	var warnings []string
	var issues []string

	if cfg.ServerURL == "" {
		cfg.ServerURL = DefaultServerURL
		warnings = append(warnings, "server_url is empty; defaulting to production relay endpoint")
	}
	if err := validateRelayServerURL(cfg.ServerURL); err != nil {
		issues = append(issues, fmt.Sprintf("server_url %q is invalid: %v", cfg.ServerURL, err))
	}

	if deps.LicenseTokenFunc == nil {
		issues = append(issues, "license token function is required")
		deps.LicenseTokenFunc = func() string { return "" }
	}
	if deps.TokenValidator == nil {
		issues = append(issues, "token validator function is required")
		deps.TokenValidator = func(string) bool { return false }
	}

	if deps.LocalAddr == "" {
		issues = append(issues, "local relay proxy address is required")
	} else if err := validateLocalAddr(deps.LocalAddr); err != nil {
		issues = append(issues, fmt.Sprintf("local relay proxy address %q is invalid: %v", deps.LocalAddr, err))
	}

	if len(issues) > 0 {
		return cfg, deps, warnings, fmt.Errorf("invalid relay client configuration: %s", strings.Join(issues, "; "))
	}

	return cfg, deps, warnings, nil
}

func validateRelayServerURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("parse error: %w", err)
	}

	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "ws" && scheme != "wss" {
		return fmt.Errorf("scheme must be ws or wss")
	}
	if parsed.Host == "" {
		return fmt.Errorf("host is required")
	}
	if parsed.User != nil {
		return fmt.Errorf("userinfo is not allowed")
	}
	if parsed.RawQuery != "" {
		return fmt.Errorf("query parameters are not allowed")
	}
	if parsed.Fragment != "" {
		return fmt.Errorf("fragment is not allowed")
	}

	if port := parsed.Port(); port != "" {
		portNum, err := strconv.Atoi(port)
		if err != nil || portNum < 1 || portNum > 65535 {
			return fmt.Errorf("port must be in range 1-65535")
		}
	}

	return nil
}

func validateLocalAddr(localAddr string) error {
	host, port, err := net.SplitHostPort(localAddr)
	if err != nil {
		return fmt.Errorf("must be host:port")
	}

	if strings.TrimSpace(host) == "" {
		return fmt.Errorf("host is required")
	}

	portNum, err := strconv.Atoi(port)
	if err != nil || portNum < 1 || portNum > 65535 {
		return fmt.Errorf("port must be in range 1-65535")
	}

	return nil
}
