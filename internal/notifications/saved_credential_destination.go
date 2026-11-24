package notifications

import (
	"net/url"
	"strings"
)

// Saved destination credentials are write-only: the API returns them blank or
// masked, and a request that sends them back that way means "keep the saved
// value". A credential was entered for one destination, so it may be kept
// while editing that destination but must never follow an address edit.
// Otherwise a caller who cannot read the credential could have it sent to a
// server of their choice.

// EmailConfigKeepsSMTPServer reports whether requested still delivers to the
// SMTP server saved names: the same host and port, and not in the clear when
// saved encrypts the connection.
func EmailConfigKeepsSMTPServer(saved, requested EmailConfig) bool {
	savedHost := strings.TrimSpace(saved.SMTPHost)
	if savedHost == "" || !strings.EqualFold(savedHost, strings.TrimSpace(requested.SMTPHost)) {
		return false
	}
	savedPort, requestedPort := effectiveSMTPPort(saved.SMTPPort), effectiveSMTPPort(requested.SMTPPort)
	if savedPort != requestedPort {
		return false
	}
	return !smtpTransportEncrypted(saved, savedPort) || smtpTransportEncrypted(requested, requestedPort)
}

// effectiveSMTPPort is the port normalizeEmailConfig settles on.
func effectiveSMTPPort(port int) int {
	if port <= 0 || port > 65535 {
		return defaultSMTPPort
	}
	return port
}

// smtpTransportEncrypted mirrors the transport delivery chooses; see
// EnhancedEmailManager.usesImplicitTLS and sendViaProviderWithAddresses.
func smtpTransportEncrypted(config EmailConfig, port int) bool {
	return config.TLS || config.StartTLS || port == 465
}

// AppriseConfigKeepsServer reports whether requested still calls the Apprise
// API server saved names, and not without the certificate check saved makes.
func AppriseConfigKeepsServer(saved, requested AppriseConfig) bool {
	if requested.SkipTLSVerify && !saved.SkipTLSVerify {
		return false
	}
	return WebhookURLKeepsOrigin(NormalizeAppriseConfig(saved).ServerURL, NormalizeAppriseConfig(requested).ServerURL)
}

// WebhookURLKeepsOrigin reports whether requested is saved itself or has the
// same HTTP origin, the boundary redirects are held to. A URL whose origin is
// templated has no origin to compare and matches only itself.
func WebhookURLKeepsOrigin(saved, requested string) bool {
	saved, requested = strings.TrimSpace(saved), strings.TrimSpace(requested)
	if saved == "" {
		return false
	}
	if saved == requested {
		return true
	}
	savedURL, err := url.Parse(saved)
	if err != nil {
		return false
	}
	requestedURL, err := url.Parse(requested)
	return err == nil && sameNotificationOrigin(savedURL, requestedURL)
}
