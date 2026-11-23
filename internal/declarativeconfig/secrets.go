package declarativeconfig

import (
	"fmt"
	"net/url"
	"os"
	"sort"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
)

const defaultRedactedHint = "configured"

func preservedSecret(hint string) *SecretField {
	if strings.TrimSpace(hint) == "" {
		hint = defaultRedactedHint
	}
	return &SecretField{
		SecretRef:    SecretRefPreserved,
		Configured:   true,
		RedactedHint: hint,
	}
}

func preservedSecretValue(hint string) SecretField {
	return *preservedSecret(hint)
}

// redactWebhookURLHint produces a non-secret display hint for a webhook URL.
// It runs Pulse's canonical webhook redactor first, then strips any remaining
// arbitrary path tokens, query parameters, userinfo, or fragments so custom
// secret paths can never leak into exported YAML or JSON.
func redactWebhookURLHint(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	redacted := notifications.RedactWebhookURLSecrets(trimmed)
	parsed, err := url.Parse(redacted)
	if err != nil || !parsed.IsAbs() || strings.TrimSpace(parsed.Host) == "" {
		return defaultRedactedHint
	}

	scheme := strings.ToLower(parsed.Scheme)
	host := parsed.Host
	hostname := strings.ToLower(parsed.Hostname())

	switch hostname {
	case "hooks.slack.com", "hooks.slack-gov.com":
		if strings.HasPrefix(parsed.Path, "/services/REDACTED") {
			return scheme + "://" + host + "/services/REDACTED"
		}
		return scheme + "://" + host + "/REDACTED"
	case "discord.com", "discordapp.com":
		if idx := strings.Index(parsed.Path, "/webhooks/REDACTED"); idx != -1 {
			return scheme + "://" + host + parsed.Path[:idx] + "/webhooks/REDACTED"
		}
	}

	if parsed.Path != "" && parsed.Path != "/" {
		return scheme + "://" + host + "/REDACTED"
	}
	return scheme + "://" + host
}

// redactSchemeHostHint returns only scheme and host for secret-bearing URLs
// such as dead-man watchdog ping URLs where the path contains the secret token.
func redactSchemeHostHint(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || !parsed.IsAbs() || strings.TrimSpace(parsed.Host) == "" {
		return defaultRedactedHint
	}
	return strings.ToLower(parsed.Scheme) + "://" + parsed.Host
}

// redactAppriseTargetHint returns a safe non-secret hint for an Apprise target.
// Many Apprise schemes place credentials directly in the host component (for
// example tgram://bot_token/chat_id or discord://id/token), so only http, https,
// mailto, and mailtos expose a DNS host while all other schemes expose only the
// scheme name.
func redactAppriseTargetHint(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || strings.TrimSpace(parsed.Scheme) == "" {
		return defaultRedactedHint
	}
	scheme := strings.ToLower(parsed.Scheme)
	switch scheme {
	case "http", "https":
		if strings.TrimSpace(parsed.Host) != "" {
			return scheme + "://" + parsed.Host + "/REDACTED"
		}
	case "mailto", "mailtos":
		if hostname := strings.TrimSpace(parsed.Hostname()); hostname != "" {
			return scheme + "://" + hostname
		}
	}
	return scheme + "://REDACTED"
}

// resolveSecretField resolves a single SecretField against an optional saved
// secret value. It fails closed on unresolved env refs, missing saved values,
// contradictory inputs, or unknown secretRef values.
func resolveSecretField(field *SecretField, saved string, allowPreserve bool, denyPreserveReason string, fieldPath string) (string, error) {
	if field == nil {
		return "", nil
	}
	ref := strings.TrimSpace(field.SecretRef)
	val := field.Value

	if ref != "" && val != "" {
		return "", fmt.Errorf("%w: %s cannot set both secretRef and value", ErrInvalidDocument, fieldPath)
	}
	if strings.HasPrefix(ref, SecretRefEnvPrefix) {
		return "", fmt.Errorf("%w: %s references %q and must be resolved before submission", ErrUnresolvedSecretRef, fieldPath, ref)
	}
	if ref == SecretRefPreserved {
		if !allowPreserve {
			if denyPreserveReason != "" {
				return "", fmt.Errorf("%w: %s %s", ErrSecretMissing, fieldPath, denyPreserveReason)
			}
			return "", fmt.Errorf("%w: %s cannot preserve a secret without an existing saved value", ErrSecretMissing, fieldPath)
		}
		if saved == "" {
			return "", fmt.Errorf("%w: %s has no saved value to preserve", ErrSecretMissing, fieldPath)
		}
		return saved, nil
	}
	if ref != "" {
		return "", fmt.Errorf("%w: %s has unsupported secretRef %q", ErrInvalidDocument, fieldPath, ref)
	}
	if val != "" {
		return val, nil
	}
	if field.Configured {
		return "", fmt.Errorf("%w: %s is marked configured without secretRef or value", ErrInvalidDocument, fieldPath)
	}
	return "", nil
}

// sameAvailabilityHTTPOrigin reports whether two availability targets point at
// the exact same scheme, host, and effective port. Write-only HTTP secrets may
// only be preserved when the origin matches so an address edit cannot replay
// saved credentials to another endpoint.
func sameAvailabilityHTTPOrigin(previous, next config.AvailabilityTarget) bool {
	previousURL, err := previous.HTTPURL()
	if err != nil {
		return false
	}
	nextURL, err := next.HTTPURL()
	if err != nil {
		return false
	}
	previousPort := previousURL.Port()
	if previousPort == "" {
		previousPort = map[string]string{"http": "80", "https": "443"}[strings.ToLower(previousURL.Scheme)]
	}
	nextPort := nextURL.Port()
	if nextPort == "" {
		nextPort = map[string]string{"http": "80", "https": "443"}[strings.ToLower(nextURL.Scheme)]
	}
	return strings.EqualFold(previousURL.Scheme, nextURL.Scheme) &&
		strings.EqualFold(previousURL.Hostname(), nextURL.Hostname()) &&
		previousPort == nextPort
}

// ResolveEnvSecretRefs walks every SecretField in doc and expands any
// secretRef prefixed with "env:" using getenv (defaulting to os.Getenv).
// Resolved values are stored in Value with SecretRef cleared so clients can
// submit them over HTTPS. Missing or empty variables fail closed with
// ErrUnresolvedSecretRef without partially mutating doc and never include
// secret values in error text.
func ResolveEnvSecretRefs(doc *Document, getenv func(string) string) error {
	if doc == nil {
		return nil
	}
	if getenv == nil {
		getenv = os.Getenv
	}

	type pendingUpdate struct {
		apply func()
	}
	updates := make([]pendingUpdate, 0)

	checkField := func(field SecretField, fieldPath string) (SecretField, bool, error) {
		ref := strings.TrimSpace(field.SecretRef)
		if !strings.HasPrefix(ref, SecretRefEnvPrefix) {
			return field, false, nil
		}
		if field.Value != "" {
			return field, false, fmt.Errorf("%w: %s cannot set both secretRef and value", ErrInvalidDocument, fieldPath)
		}
		envName := strings.TrimSpace(strings.TrimPrefix(ref, SecretRefEnvPrefix))
		if envName == "" || strings.ContainsAny(envName, " \t\r\n=") {
			return field, false, fmt.Errorf("%w: %s has an invalid environment variable name", ErrUnresolvedSecretRef, fieldPath)
		}
		val := getenv(envName)
		if val == "" {
			return field, false, fmt.Errorf("%w: environment variable %s for %s is unset or empty", ErrUnresolvedSecretRef, envName, fieldPath)
		}
		field.Value = val
		field.SecretRef = ""
		field.Configured = true
		field.RedactedHint = ""
		return field, true, nil
	}

	resolvePtr := func(field *SecretField, fieldPath string) error {
		if field == nil {
			return nil
		}
		updated, changed, err := checkField(*field, fieldPath)
		if err != nil {
			return err
		}
		if changed {
			target := field
			val := updated
			updates = append(updates, pendingUpdate{apply: func() { *target = val }})
		}
		return nil
	}

	if doc.AvailabilityTargets != nil {
		targets := *doc.AvailabilityTargets
		for i := range targets {
			target := &targets[i]
			if target.HTTP == nil {
				continue
			}
			prefix := fmt.Sprintf("availabilityTargets[%d]", i)
			if strings.TrimSpace(target.ID) != "" {
				prefix = fmt.Sprintf("availabilityTargets[%s]", strings.TrimSpace(target.ID))
			}
			if err := resolvePtr(target.HTTP.Body, prefix+".http.body"); err != nil {
				return err
			}
			if err := resolvePtr(target.HTTP.Authentication.Password, prefix+".http.authentication.password"); err != nil {
				return err
			}
			if err := resolvePtr(target.HTTP.Authentication.BearerToken, prefix+".http.authentication.bearerToken"); err != nil {
				return err
			}
			for j := range target.HTTP.Headers {
				hdrPath := fmt.Sprintf("%s.http.headers[%d].value", prefix, j)
				if err := resolvePtr(target.HTTP.Headers[j].Value, hdrPath); err != nil {
					return err
				}
			}
		}
	}

	if doc.Notifications != nil {
		if doc.Notifications.Email != nil {
			if err := resolvePtr(doc.Notifications.Email.Password, "notifications.email.password"); err != nil {
				return err
			}
		}
		if doc.Notifications.Webhooks != nil {
			webhooks := *doc.Notifications.Webhooks
			for i := range webhooks {
				hook := &webhooks[i]
				prefix := fmt.Sprintf("notifications.webhooks[%d]", i)
				if strings.TrimSpace(hook.ID) != "" {
					prefix = fmt.Sprintf("notifications.webhooks[%s]", strings.TrimSpace(hook.ID))
				}
				if err := resolvePtr(hook.URL, prefix+".url"); err != nil {
					return err
				}
				if err := resolvePtr(hook.SigningSecret, prefix+".signingSecret"); err != nil {
					return err
				}
				if len(hook.Headers) > 0 {
					keys := sortedMapKeys(hook.Headers)
					for _, k := range keys {
						updated, changed, err := checkField(hook.Headers[k], fmt.Sprintf("%s.headers[%s]", prefix, k))
						if err != nil {
							return err
						}
						if changed {
							m := hook.Headers
							key := k
							val := updated
							updates = append(updates, pendingUpdate{apply: func() { m[key] = val }})
						}
					}
				}
				if len(hook.CustomFields) > 0 {
					keys := sortedMapKeys(hook.CustomFields)
					for _, k := range keys {
						updated, changed, err := checkField(hook.CustomFields[k], fmt.Sprintf("%s.customFields[%s]", prefix, k))
						if err != nil {
							return err
						}
						if changed {
							m := hook.CustomFields
							key := k
							val := updated
							updates = append(updates, pendingUpdate{apply: func() { m[key] = val }})
						}
					}
				}
			}
		}
		if doc.Notifications.Apprise != nil {
			apprise := doc.Notifications.Apprise
			if err := resolvePtr(apprise.ConfigKey, "notifications.apprise.configKey"); err != nil {
				return err
			}
			if err := resolvePtr(apprise.APIKey, "notifications.apprise.apiKey"); err != nil {
				return err
			}
			for i := range apprise.Targets {
				updated, changed, err := checkField(apprise.Targets[i], fmt.Sprintf("notifications.apprise.targets[%d]", i))
				if err != nil {
					return err
				}
				if changed {
					slice := apprise.Targets
					idx := i
					val := updated
					updates = append(updates, pendingUpdate{apply: func() { slice[idx] = val }})
				}
			}
		}
		if doc.Notifications.DeadMan != nil {
			if err := resolvePtr(doc.Notifications.DeadMan.PingURL, "notifications.deadMan.pingUrl"); err != nil {
				return err
			}
		}
	}

	for _, u := range updates {
		u.apply()
	}
	return nil
}

func sortedMapKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
