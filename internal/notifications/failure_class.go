package notifications

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/textproto"
	"syscall"
)

// Delivery failure classification is authoritative where the sender knows the
// answer and heuristic only where it does not.
//
// The original classifier read the class out of the error prose. That is wrong
// in two ways that both showed up in fleet telemetry. It cannot see failures
// whose text carries no recognised token, which is why `unknown` became the
// modal bucket; and the prose it reads includes the destination's own response
// body, so a third party can steer the reason code by putting "rate limit" or
// "unauthorized" in the body of a 500. A declared class removes both: the
// sender states what happened, and the text is only consulted when nobody did.

// NotificationFailureError carries the delivery failure class its sender
// already knew, so the class never has to be inferred from the message.
type NotificationFailureError struct {
	Class NotificationFailureClass
	Err   error
}

func (e *NotificationFailureError) Error() string {
	if e == nil {
		return ""
	}
	if e.Err == nil {
		return string(e.Class)
	}
	return e.Err.Error()
}

func (e *NotificationFailureError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// FailWithClass labels err with an authoritative delivery failure class.
// A nil err yields nil so callers can wrap unconditionally.
func FailWithClass(class NotificationFailureClass, err error) error {
	if err == nil {
		return nil
	}
	return &NotificationFailureError{Class: class, Err: err}
}

// FailfWithClass builds a labelled failure from a format string. The format
// arguments may include destination response text; that text is preserved for
// the operator-facing audit row but can no longer influence the class.
func FailfWithClass(class NotificationFailureClass, format string, args ...any) error {
	return &NotificationFailureError{Class: class, Err: fmt.Errorf(format, args...)}
}

// ClassFromHTTPStatus maps an HTTP response status to its delivery class.
// Status codes are the destination's own structured verdict, so they outrank
// anything the response body happens to say.
func ClassFromHTTPStatus(status int) NotificationFailureClass {
	switch status {
	case 401, 403, 407:
		return NotificationFailureAuthentication
	case 402:
		return NotificationFailureConfiguration
	case 408:
		return NotificationFailureConnectivity
	case 429:
		return NotificationFailureRateLimited
	}
	switch {
	case status >= 500 && status <= 599:
		return NotificationFailureServerError
	case status >= 400 && status <= 499:
		return NotificationFailureRejected
	default:
		return NotificationFailureUnknown
	}
}

// ClassFromSMTPCode maps an SMTP reply code to its delivery class.
//
// SMTP 5xx is not the HTTP 5xx meaning: a 550 is the destination refusing the
// message, not the destination breaking. Only the transient 4xx replies are a
// server-side fault, and the 5xx range splits between our credentials, our
// command, and the recipient.
func ClassFromSMTPCode(code int) NotificationFailureClass {
	switch code {
	case 530, 534, 535, 538:
		return NotificationFailureAuthentication
	case 500, 501, 502, 503, 504:
		// Syntax, bad sequence, or an unimplemented command: the session Pulse
		// built does not match what this server accepts.
		return NotificationFailureConfiguration
	}
	switch {
	case code >= 400 && code <= 499:
		// Transient negative completion. The server responded and asked for
		// this to be tried again later.
		return NotificationFailureServerError
	case code >= 500 && code <= 599:
		return NotificationFailureRejected
	default:
		return NotificationFailureUnknown
	}
}

// ClassifyNotificationFailureError determines the delivery failure class for a
// send error. It prefers what the sender declared, then what Go's own error
// types prove, and only then falls back to reading the message text.
func ClassifyNotificationFailureError(err error) NotificationFailureClass {
	if err == nil {
		return NotificationFailureUnknown
	}

	var declared *NotificationFailureError
	if errors.As(err, &declared) && declared.Class != "" {
		return declared.Class
	}

	// An SMTP reply code is the mail server's structured verdict. net/smtp
	// surfaces it as *textproto.Error however deeply it is wrapped.
	var protoErr *textproto.Error
	if errors.As(err, &protoErr) {
		return ClassFromSMTPCode(protoErr.Code)
	}

	// Certificate and handshake failures, before the generic network checks:
	// a verification failure is also reachable through a dial.
	var certVerifyErr *tls.CertificateVerificationError
	var unknownAuthorityErr x509.UnknownAuthorityError
	var hostnameErr x509.HostnameError
	var certInvalidErr x509.CertificateInvalidError
	var recordHeaderErr tls.RecordHeaderError
	if errors.As(err, &certVerifyErr) ||
		errors.As(err, &unknownAuthorityErr) ||
		errors.As(err, &hostnameErr) ||
		errors.As(err, &certInvalidErr) ||
		errors.As(err, &recordHeaderErr) {
		return NotificationFailureTLS
	}

	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return NotificationFailureConnectivity
	}

	if errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, syscall.ECONNREFUSED) ||
		errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, syscall.EHOSTUNREACH) ||
		errors.Is(err, syscall.ENETUNREACH) ||
		errors.Is(err, syscall.ETIMEDOUT) ||
		errors.Is(err, syscall.EPIPE) {
		return NotificationFailureConnectivity
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return NotificationFailureConnectivity
	}

	return ClassifyNotificationFailure(err.Error())
}

// Retryable reports whether another delivery attempt could plausibly succeed.
//
// Authentication, configuration and rejection are verdicts about the request
// itself: the same payload, sent again to the same destination with the same
// credentials, gets the same answer. Retrying them spends attempts, delays the
// dead letter the operator needs to see, and learns nothing. Connectivity,
// rate limiting and server errors describe conditions that clear on their own,
// and an unclassified failure is retried because nothing proves it will not
// succeed.
//
// This generalises to every destination type the decision webhook delivery
// already made for HTTP 4xx in isRetryableWebhookError.
//
// A dead letter is not the end of the road: once the operator fixes the
// credentials or the configuration, RetryTerminalFailures returns retained
// terminal failures to the queue with a fresh budget.
func (class NotificationFailureClass) Retryable() bool {
	switch class {
	case NotificationFailureAuthentication,
		NotificationFailureConfiguration,
		NotificationFailureRejected:
		return false
	default:
		return true
	}
}
