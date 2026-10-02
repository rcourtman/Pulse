package notifications

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/textproto"
	"testing"
)

func TestClassFromHTTPStatus(t *testing.T) {
	cases := map[int]NotificationFailureClass{
		401: NotificationFailureAuthentication,
		403: NotificationFailureAuthentication,
		407: NotificationFailureAuthentication,
		402: NotificationFailureConfiguration,
		408: NotificationFailureConnectivity,
		429: NotificationFailureRateLimited,
		400: NotificationFailureRejected,
		404: NotificationFailureRejected,
		422: NotificationFailureRejected,
		500: NotificationFailureServerError,
		502: NotificationFailureServerError,
		503: NotificationFailureServerError,
		200: NotificationFailureUnknown,
	}
	for status, want := range cases {
		if got := ClassFromHTTPStatus(status); got != want {
			t.Errorf("ClassFromHTTPStatus(%d) = %q, want %q", status, got, want)
		}
	}
}

func TestClassFromSMTPCode(t *testing.T) {
	// SMTP 5xx is a refusal, not a server fault: only the transient 4xx range
	// means the destination broke.
	cases := map[int]NotificationFailureClass{
		421: NotificationFailureServerError,
		450: NotificationFailureServerError,
		451: NotificationFailureServerError,
		452: NotificationFailureServerError,
		530: NotificationFailureAuthentication,
		535: NotificationFailureAuthentication,
		538: NotificationFailureAuthentication,
		500: NotificationFailureConfiguration,
		501: NotificationFailureConfiguration,
		503: NotificationFailureConfiguration,
		550: NotificationFailureRejected,
		552: NotificationFailureRejected,
		554: NotificationFailureRejected,
		250: NotificationFailureUnknown,
	}
	for code, want := range cases {
		if got := ClassFromSMTPCode(code); got != want {
			t.Errorf("ClassFromSMTPCode(%d) = %q, want %q", code, got, want)
		}
	}
}

// A destination controls its own response body. It must not be able to choose
// the reason code Pulse records and shows the operator.
func TestClassifyNotificationFailureError_ResponseBodyCannotSteerClass(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		want   NotificationFailureClass
	}{
		{"server error body claiming rate limit", 500, `{"error":"rate limit exceeded"}`, NotificationFailureServerError},
		{"server error body claiming unauthorized", 503, `unauthorized`, NotificationFailureServerError},
		{"rejection body mentioning certificate", 404, `no certificate route found`, NotificationFailureRejected},
		{"rejection body mentioning connection refused", 400, `upstream connection refused`, NotificationFailureRejected},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := FailfWithClass(
				ClassFromHTTPStatus(tc.status),
				"webhook returned HTTP %d: %s", tc.status, tc.body,
			)
			if got := ClassifyNotificationFailureError(err); got != tc.want {
				t.Errorf("class = %q, want %q", got, tc.want)
			}
			// The prose classifier is the one that gets this wrong; that is
			// precisely why the declared class has to win.
			if prose := ClassifyNotificationFailure(err.Error()); prose == tc.want {
				t.Logf("prose classifier happened to agree for %q", tc.name)
			}
		})
	}
}

func TestClassifyNotificationFailureError_DeclaredClassSurvivesWrapping(t *testing.T) {
	base := FailWithClass(NotificationFailureConfiguration, errors.New("no Apprise targets configured for CLI delivery"))
	wrapped := fmt.Errorf("apprise CLI send failed: %w", base)
	if got := ClassifyNotificationFailureError(wrapped); got != NotificationFailureConfiguration {
		t.Fatalf("class = %q, want %q", got, NotificationFailureConfiguration)
	}
}

func TestClassifyNotificationFailureError_SMTPReplyCode(t *testing.T) {
	// net/smtp surfaces the server's reply as *textproto.Error. Before this
	// path existed every one of these landed in the unknown bucket.
	cases := []struct {
		code int
		msg  string
		want NotificationFailureClass
	}{
		{550, "5.7.1 Relay access denied", NotificationFailureRejected},
		{535, "5.7.8 Authentication credentials invalid", NotificationFailureAuthentication},
		{451, "4.3.0 Temporary local problem", NotificationFailureServerError},
		{501, "5.5.4 Syntax error in parameters", NotificationFailureConfiguration},
	}
	for _, tc := range cases {
		err := fmt.Errorf("failed to send email: %w", &textproto.Error{Code: tc.code, Msg: tc.msg})
		if got := ClassifyNotificationFailureError(err); got != tc.want {
			t.Errorf("SMTP %d: class = %q, want %q", tc.code, got, tc.want)
		}
	}
}

func TestClassifyNotificationFailureError_StructuralNetworkErrors(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want NotificationFailureClass
	}{
		{
			name: "dns failure",
			err:  fmt.Errorf("post webhook: %w", &net.DNSError{Err: "server misbehaving", Name: "hooks.example"}),
			want: NotificationFailureConnectivity,
		},
		{
			name: "context deadline",
			err:  fmt.Errorf("post webhook: %w", context.DeadlineExceeded),
			want: NotificationFailureConnectivity,
		},
		{
			name: "unknown certificate authority",
			err:  fmt.Errorf("post webhook: %w", x509.UnknownAuthorityError{}),
			want: NotificationFailureTLS,
		},
		{
			name: "certificate hostname mismatch",
			err:  fmt.Errorf("post webhook: %w", x509.HostnameError{Host: "hooks.example"}),
			want: NotificationFailureTLS,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ClassifyNotificationFailureError(tc.err); got != tc.want {
				t.Errorf("class = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestClassifyNotificationFailureError_FallsBackToProse(t *testing.T) {
	err := errors.New("webhook returned HTTP 429 Too Many Requests")
	if got := ClassifyNotificationFailureError(err); got != NotificationFailureRateLimited {
		t.Fatalf("class = %q, want %q", got, NotificationFailureRateLimited)
	}
}

func TestClassifyNotificationFailureError_NilIsUnknown(t *testing.T) {
	if got := ClassifyNotificationFailureError(nil); got != NotificationFailureUnknown {
		t.Fatalf("class = %q, want %q", got, NotificationFailureUnknown)
	}
}

func TestFailWithClassPreservesMessageAndUnwrap(t *testing.T) {
	cause := errors.New("dial tcp 10.0.0.1:465: connect: connection refused")
	err := FailWithClass(NotificationFailureConnectivity, cause)
	if err.Error() != cause.Error() {
		t.Errorf("Error() = %q, want %q", err.Error(), cause.Error())
	}
	if !errors.Is(err, cause) {
		t.Error("wrapped error does not unwrap to its cause")
	}
	if FailWithClass(NotificationFailureConnectivity, nil) != nil {
		t.Error("FailWithClass(nil) should be nil")
	}
}
