package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/rs/zerolog/log"
)

// CredentialResolver handles resolving credential values from various sources
type CredentialResolver struct {
	// Track which credentials are stored insecurely for warnings
	insecureCredentials []string
}

// NewCredentialResolver creates a new credential resolver
func NewCredentialResolver() *CredentialResolver {
	return &CredentialResolver{
		insecureCredentials: []string{},
	}
}

// ResolveValue resolves a credential value that might be:
// - A literal value (backwards compatible)
// - An environment variable reference: ${VAR_NAME} (for secrets, not node config)
// - A file reference: file:///path/to/secret
// - Future: vault://path/to/secret, keyring://secret-name, etc.
// NOTE: This is for credential values only, not for node configuration which is done via UI
func (cr *CredentialResolver) ResolveValue(value string, fieldName string) (string, error) {
	if value == "" {
		return "", nil
	}

	// Check for environment variable reference
	if strings.HasPrefix(value, "${") && strings.HasSuffix(value, "}") {
		varName := value[2 : len(value)-1]
		resolved := os.Getenv(varName)
		if resolved == "" {
			return "", fmt.Errorf("environment variable %s not set", varName)
		}
		log.Debug().Str("field", fieldName).Str("var", varName).Msg("Resolved credential from environment variable")
		return resolved, nil
	}

	// Check for file reference
	if strings.HasPrefix(value, "file://") {
		filePath := strings.TrimPrefix(value, "file://")
		content, err := os.ReadFile(filePath)
		if err != nil {
			return "", fmt.Errorf("failed to read credential file %s: %w", filePath, err)
		}
		// Trim any whitespace/newlines
		resolved := strings.TrimSpace(string(content))

		// Check file permissions
		if info, err := os.Stat(filePath); err == nil {
			mode := info.Mode()
			if mode&0077 != 0 {
				log.Warn().
					Str("file", filePath).
					Str("permissions", mode.String()).
					Msg("Credential file has overly permissive permissions. Consider: chmod 600 " + filePath)
			}
		}

		log.Debug().Str("field", fieldName).Str("file", filePath).Msg("Resolved credential from file")
		return resolved, nil
	}

	// Check if this looks like a credential (UUID pattern, token pattern, etc)
	if looksLikeCredential(value) {
		cr.insecureCredentials = append(cr.insecureCredentials, fieldName)
	}

	// Return as-is (literal value - backwards compatible)
	return value, nil
}

// CheckConfigSecurity checks the security of the config file and credentials
func (cr *CredentialResolver) CheckConfigSecurity(configPath string) {
	// We now auto-secure the config file, so only log at debug level
	if info, err := os.Stat(configPath); err == nil {
		mode := info.Mode()
		log.Debug().
			Str("file", configPath).
			Str("permissions", mode.String()).
			Int("inline_credentials", len(cr.insecureCredentials)).
			Msg("Config file security check")
	}
}

// looksLikeCredential uses heuristics to detect if a value is likely a credential
func looksLikeCredential(value string) bool {
	// Skip if it's a reference
	if strings.HasPrefix(value, "${") || strings.HasPrefix(value, "file://") {
		return false
	}

	// UUID pattern
	uuidRegex := regexp.MustCompile(`^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$`)
	if uuidRegex.MatchString(value) {
		return true
	}

	// Long random string (likely a token)
	if len(value) > 20 && regexp.MustCompile(`^[a-zA-Z0-9_\-]+$`).MatchString(value) {
		return true
	}

	// Contains words like secret, token, key, password
	lowerValue := strings.ToLower(value)
	if strings.Contains(lowerValue, "secret") || strings.Contains(lowerValue, "token") ||
		strings.Contains(lowerValue, "key") || strings.Contains(lowerValue, "password") {
		return true
	}

	return false
}

// ResolveNodeCredentials resolves all credentials in a node configuration
func (cr *CredentialResolver) ResolveNodeCredentials(node interface{}, nodeName string) error {
	switch n := node.(type) {
	case *PVEInstance:
		var err error
		n.Password, err = cr.ResolveValue(n.Password, fmt.Sprintf("%s.password", nodeName))
		if err != nil {
			return err
		}
		n.TokenValue, err = cr.ResolveValue(n.TokenValue, fmt.Sprintf("%s.token_value", nodeName))
		if err != nil {
			return err
		}
	case *PBSInstance:
		var err error
		n.Password, err = cr.ResolveValue(n.Password, fmt.Sprintf("%s.password", nodeName))
		if err != nil {
			return err
		}
		n.TokenValue, err = cr.ResolveValue(n.TokenValue, fmt.Sprintf("%s.token_value", nodeName))
		if err != nil {
			return err
		}
	}
	return nil
}
