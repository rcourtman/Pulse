package notifications

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/textproto"
	"testing"
	"time"
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

// The declared class has to survive all the way into the audit row that feeds
// both the operator's delivery health card and the telemetry counters.
func TestRecordAuditErrorPersistsDeclaredClass(t *testing.T) {
	nq, err := NewNotificationQueue(t.TempDir())
	if err != nil {
		t.Fatalf("NewNotificationQueue: %v", err)
	}
	defer func() { _ = nq.Stop() }()

	now := time.Now().UTC()
	entries := []*QueuedNotification{
		{ID: "body-says-rate-limit", Type: "webhook", Status: QueueStatusDLQ, Attempts: 3, Config: []byte(`{}`), CreatedAt: now},
		{ID: "smtp-refusal", Type: "email", Status: QueueStatusDLQ, Attempts: 3, Config: []byte(`{}`), CreatedAt: now},
	}
	for _, entry := range entries {
		status := entry.Status
		entry.Status = QueueStatusPending
		if err := nq.Enqueue(entry); err != nil {
			t.Fatalf("enqueue %s: %v", entry.ID, err)
		}
		entry.Status = status
	}

	// A 500 whose body says "rate limit" is a server error, not rate limiting.
	bodySteered := FailfWithClass(
		ClassFromHTTPStatus(500),
		"webhook returned HTTP %d: %s", 500, `{"error":"rate limit exceeded"}`,
	)
	if err := nq.RecordAuditError(entries[0], false, bodySteered); err != nil {
		t.Fatalf("record body-steered audit: %v", err)
	}
	smtpRefusal := fmt.Errorf("failed to send email: %w", &textproto.Error{Code: 550, Msg: "5.7.1 Relay access denied"})
	if err := nq.RecordAuditError(entries[1], false, smtpRefusal); err != nil {
		t.Fatalf("record smtp audit: %v", err)
	}

	stats, err := nq.GetTelemetryStats(now.Add(-time.Hour))
	if err != nil {
		t.Fatalf("GetTelemetryStats: %v", err)
	}
	if stats.Failures != 2 {
		t.Fatalf("failures = %d, want 2", stats.Failures)
	}
	if stats.FailureClasses.ServerError != 1 {
		t.Errorf("server_error = %d, want 1", stats.FailureClasses.ServerError)
	}
	if stats.FailureClasses.Rejected != 1 {
		t.Errorf("rejected = %d, want 1 (SMTP 550 is a refusal)", stats.FailureClasses.Rejected)
	}
	if stats.FailureClasses.RateLimited != 0 {
		t.Errorf("rate_limited = %d, want 0: the response body must not set the class", stats.FailureClasses.RateLimited)
	}
	if stats.FailureClasses.Unknown != 0 {
		t.Errorf("unknown = %d, want 0", stats.FailureClasses.Unknown)
	}
}

func TestFailureClassRetryable(t *testing.T) {
	cases := map[NotificationFailureClass]bool{
		NotificationFailureAuthentication: false,
		NotificationFailureConfiguration:  false,
		NotificationFailureRejected:       false,
		NotificationFailureConnectivity:   true,
		NotificationFailureRateLimited:    true,
		NotificationFailureServerError:    true,
		NotificationFailureTLS:            true,
		NotificationFailureUnknown:        true,
		NotificationFailureClass(""):      true,
	}
	for class, want := range cases {
		if got := class.Retryable(); got != want {
			t.Errorf("%q.Retryable() = %v, want %v", class, got, want)
		}
	}
}

// A deterministic failure must dead-letter on the first attempt rather than
// spend the whole ladder re-asking a question that already has an answer.
func TestProcessNotificationDeadLettersDeterministicFailureImmediately(t *testing.T) {
	cases := []struct {
		name          string
		sendErr       error
		wantStatus    NotificationQueueStatus
		wantCallCount int
	}{
		{
			name:          "authentication does not retry",
			sendErr:       FailWithClass(NotificationFailureAuthentication, errors.New("smtp 535 authentication failed")),
			wantStatus:    QueueStatusDLQ,
			wantCallCount: 1,
		},
		{
			name:          "configuration does not retry",
			sendErr:       FailWithClass(NotificationFailureConfiguration, errors.New("no Apprise targets configured for CLI delivery")),
			wantStatus:    QueueStatusDLQ,
			wantCallCount: 1,
		},
		{
			name:          "rejection does not retry",
			sendErr:       FailfWithClass(ClassFromHTTPStatus(422), "webhook returned HTTP 422: unprocessable"),
			wantStatus:    QueueStatusDLQ,
			wantCallCount: 1,
		},
		{
			name:          "connectivity still retries",
			sendErr:       FailWithClass(NotificationFailureConnectivity, errors.New("dial tcp: connection refused")),
			wantStatus:    QueueStatusPending,
			wantCallCount: 1,
		},
		{
			name:          "server error still retries",
			sendErr:       FailfWithClass(ClassFromHTTPStatus(503), "webhook returned HTTP 503: unavailable"),
			wantStatus:    QueueStatusPending,
			wantCallCount: 1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			nq, err := NewNotificationQueue(t.TempDir())
			if err != nil {
				t.Fatalf("NewNotificationQueue: %v", err)
			}
			defer func() { _ = nq.Stop() }()

			futureRetry := time.Now().Add(time.Hour)
			notif := &QueuedNotification{
				ID:          "deterministic-failure",
				Type:        "webhook",
				Status:      QueueStatusPending,
				MaxAttempts: 3,
				Config:      []byte(`{}`),
				NextRetryAt: &futureRetry,
			}
			if err := nq.Enqueue(notif); err != nil {
				t.Fatalf("enqueue: %v", err)
			}

			calls := 0
			nq.SetProcessor(func(*QueuedNotification) error {
				calls++
				return tc.sendErr
			})

			nq.processNotification(notif)

			if calls != tc.wantCallCount {
				t.Errorf("processor calls = %d, want %d", calls, tc.wantCallCount)
			}

			if notif.Status != tc.wantStatus {
				t.Errorf("status = %q, want %q", notif.Status, tc.wantStatus)
			}
			if notif.Attempts != 1 {
				t.Errorf("attempts = %d, want 1 (one delivery attempt was made)", notif.Attempts)
			}

			stats, err := nq.GetQueueStats()
			if err != nil {
				t.Fatalf("GetQueueStats: %v", err)
			}
			if stats[string(tc.wantStatus)] != 1 {
				t.Errorf("persisted queue stats = %#v, want one row in %q", stats, tc.wantStatus)
			}
		})
	}
}
