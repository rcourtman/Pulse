package notifications

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os/exec"
	"strings"
	"syscall"
)

// Apprise targets use many credential-bearing URL schemes, and both CLI output
// and HTTP responses can echo the targets, API key, config key or alert body.
// Do not try to recognise every provider's secret syntax. Diagnostics retain
// structured outcomes, never endpoint paths or arbitrary provider/error text.
func appriseDiagnosticURL(string) string { return "[Apprise endpoint]" }

type appriseDiagnosticError struct {
	message string
	cause   error
}

func (e *appriseDiagnosticError) Error() string { return e.message }
func (e *appriseDiagnosticError) Unwrap() error { return e.cause }

func safeAppriseError(operation string, err error) error {
	if err == nil {
		return nil
	}
	// Classify before hiding prose so existing retry decisions remain intact.
	// Retain the cause for errors.Is/As, but never format it in diagnostics.
	class := ClassifyNotificationFailureError(fmt.Errorf("%s: %w", operation, err))
	reason := "delivery failed (details withheld)"
	switch class {
	case NotificationFailureAuthentication:
		reason = "authentication failed"
	case NotificationFailureRateLimited:
		reason = "rate limited"
	case NotificationFailureConnectivity:
		reason = "server connection failed"
	case NotificationFailureTLS:
		reason = "TLS certificate or handshake failed"
	case NotificationFailureConfiguration:
		reason = "invalid configuration"
	case NotificationFailureRejected:
		reason = "request rejected"
	case NotificationFailureServerError:
		reason = "server unavailable"
	}
	var dnsErr *net.DNSError
	var netErr net.Error
	switch {
	case errors.Is(err, exec.ErrNotFound):
		reason = "executable file not found in PATH"
	case errors.Is(err, syscall.EACCES):
		reason = "permission denied"
	case errors.As(err, &dnsErr):
		reason = "no such host or DNS lookup failed"
	case errors.Is(err, syscall.ECONNREFUSED):
		reason = "connection refused"
	case errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &netErr) && netErr.Timeout()):
		reason = "connection timed out (deadline exceeded)"
	case errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF):
		reason = "unexpected EOF from server"
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		reason = fmt.Sprintf("exit status %d (output withheld)", exitErr.ExitCode())
	}
	// These literal validation reasons are actionable and contain no input.
	// Inspect wrapped redirect refusals too; unknown reasons stay withheld.
	for cause := err; cause != nil; cause = errors.Unwrap(cause) {
		switch cause.Error() {
		case "URL userinfo is not allowed", "base URL must not include query or fragment",
			"URL host is required", "URL hostname is required", "base URL path must be host-local",
			"webhook URL userinfo is not allowed", "webhook URL missing hostname",
			"webhook URLs pointing to unspecified addresses are not allowed",
			"webhook URLs pointing to localhost are not allowed for security reasons",
			"webhook URLs pointing to link-local addresses are not allowed",
			"webhook URLs pointing to cloud metadata services are not allowed":
			reason = cause.Error()
		}
		if strings.Contains(cause.Error(), "private networks are not allowed for security (configure allowlist in System Settings)") {
			reason = "private networks are not allowed for security (configure allowlist in System Settings)"
		}
	}
	return FailWithClass(class, &appriseDiagnosticError{message: operation + ": " + reason, cause: err})
}
