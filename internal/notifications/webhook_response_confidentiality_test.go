package notifications

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// The receiver may echo arbitrary headers, custom fields, URLs or alert text.
// None of these fixtures opens a listener, contacts a provider or uses a queue.
const webhookResponsePrivateText = "synthetic-header-token synthetic-payload-private synthetic-provider-private"

type trackedWebhookResponse struct {
	reader io.Reader
	cause  error
	read   int64
	closed bool
}

func (r *trackedWebhookResponse) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	r.read += int64(n)
	if err == io.EOF && r.cause != nil {
		err = r.cause
	}
	return n, err
}

func (r *trackedWebhookResponse) Close() error { r.closed = true; return nil }

func captureWebhookResponseLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var captured bytes.Buffer
	previous := log.Logger
	log.Logger = zerolog.New(&captured).Level(zerolog.DebugLevel)
	t.Cleanup(func() { log.Logger = previous })
	return &captured
}

func responseFixtureManager(t *testing.T, transport confidentialityTransport) *NotificationManager {
	t.Helper()
	n := &NotificationManager{webhookClient: &http.Client{Transport: transport},
		webhookRateLimits: make(map[string]*webhookRateLimit)}
	if err := n.UpdateAllowedPrivateCIDRs("127.0.0.1/32"); err != nil {
		t.Fatal(err)
	}
	return n
}

func assertWithheldWebhookResponse(t *testing.T, captured *bytes.Buffer, err error) {
	t.Helper()
	for _, private := range strings.Fields(webhookResponsePrivateText) {
		if strings.Contains(captured.String(), private) || (err != nil && strings.Contains(err.Error(), private)) {
			t.Errorf("provider text reached diagnostics: %s", private)
		}
	}
}

func TestWebhookResponseConfidentialitySendPaths(t *testing.T) {
	for _, path := range []string{"ordinary", "enhanced", "ntfy-resolved"} {
		for _, code := range []int{200, 400, 401, 503} {
			t.Run(fmt.Sprintf("%s/%d", path, code), func(t *testing.T) {
				captured := captureWebhookResponseLogs(t)
				body := &trackedWebhookResponse{reader: strings.NewReader(webhookResponsePrivateText)}
				requests := 0
				n := responseFixtureManager(t, func(r *http.Request) (*http.Response, error) {
					requests++
					if r.URL.Query().Get("token") != "synthetic-header-token" || r.Header.Get("Authorization") != "Bearer synthetic-header-token" {
						t.Error("withholding response text changed the request credential")
					}
					return &http.Response{StatusCode: code, Header: http.Header{"Retry-After": {"0"}}, Body: body}, nil
				})
				webhook := WebhookConfig{Name: "response fixture", URL: "http://127.0.0.1/hook?token=synthetic-header-token",
					Headers: map[string]string{"Authorization": "Bearer synthetic-header-token"}}
				var err error
				switch path {
				case "ordinary":
					err = n.sendWebhookRequest(webhook, []byte(`{"unchanged":true}`), "alert", "fixture:alert")
				case "enhanced":
					var result *webhookHTTPResult
					result, err = n.executeEnhancedWebhookRequest(EnhancedWebhookConfig{WebhookConfig: webhook, ResponseLogging: true},
						[]byte(`{"unchanged":true}`), WebhookTimeout, "response-fixture", "fixture:alert")
					if result == nil || result.statusCode != code || result.headers.Get("Retry-After") != "0" {
						t.Error("structured status or retry hint changed")
					}
				case "ntfy-resolved":
					err = n.sendResolvedWebhookNtfy(webhook, nil, time.Unix(1, 0))
				}
				if (err == nil) != (code == 200) || requests != 1 || !body.closed {
					t.Fatalf("verdict/request/cleanup changed: error=%v requests=%d closed=%t", err, requests, body.closed)
				}
				if err != nil && ClassifyNotificationFailureError(err) != ClassFromHTTPStatus(code) {
					t.Error("HTTP rejection lost its authoritative failure class")
				}
				assertWithheldWebhookResponse(t, captured, err)
			})
		}
	}
}

func TestWebhookResponseConfidentialityRetryHistory(t *testing.T) {
	for _, finalCode := range []int{http.StatusForbidden, http.StatusAccepted} {
		t.Run(fmt.Sprint(finalCode), func(t *testing.T) {
			captured := captureWebhookResponseLogs(t)
			attempts := 0
			payload := []byte(`{"occurrence":"unchanged"}`)
			n := responseFixtureManager(t, func(r *http.Request) (*http.Response, error) {
				attempts++
				sent, err := io.ReadAll(r.Body)
				if err != nil || !bytes.Equal(sent, payload) || r.Header.Get("X-Pulse-Event-ID") != "fixture:alert" {
					t.Error("retry changed occurrence or payload")
				}
				code := http.StatusTooManyRequests
				if attempts > 1 {
					code = finalCode
				}
				return &http.Response{StatusCode: code, Header: http.Header{"Retry-After": {"0"}},
					Body: io.NopCloser(strings.NewReader(webhookResponsePrivateText))}, nil
			})
			err := n.sendWebhookWithRetry(EnhancedWebhookConfig{
				WebhookConfig: WebhookConfig{Name: "response fixture", URL: "http://127.0.0.1/hook"}, RetryCount: 2,
			}, payload, "fixture:alert")
			if attempts != 2 || (err == nil) != (finalCode == http.StatusAccepted) {
				t.Fatalf("retry verdict changed: attempts=%d error=%v", attempts, err)
			}
			history := n.GetWebhookHistory()
			if len(history) != 1 || history[0].StatusCode != finalCode || history[0].RetryAttempts != 1 ||
				history[0].Success != (finalCode == http.StatusAccepted) || history[0].PayloadSize != len(payload) {
				t.Fatalf("delivery history lost final status or attempt accounting: %+v", history)
			}
			assertWithheldWebhookResponse(t, captured, err)
			assertWithheldWebhookResponse(t, captured, errors.New(history[0].ErrorMessage))
		})
	}
}

func TestWebhookResponseConfidentialityReadFailure(t *testing.T) {
	for _, path := range []string{"shared", "ntfy-resolved"} {
		for _, code := range []int{200, 401, 503} {
			t.Run(fmt.Sprintf("%s/%d", path, code), func(t *testing.T) {
				captured := captureWebhookResponseLogs(t)
				cause := fmt.Errorf("%s: %w", webhookResponsePrivateText, io.ErrUnexpectedEOF)
				body := &trackedWebhookResponse{reader: strings.NewReader(webhookResponsePrivateText), cause: cause}
				n := responseFixtureManager(t, func(*http.Request) (*http.Response, error) {
					return &http.Response{StatusCode: code, Header: make(http.Header), Body: body}, nil
				})
				webhook := WebhookConfig{Name: "response fixture", URL: "http://127.0.0.1/hook"}
				var err error
				if path == "shared" {
					_, err = n.executeWebhookRequest(webhook, nil, webhookRequestOptions{})
				} else {
					err = n.sendResolvedWebhookNtfy(webhook, nil, time.Unix(1, 0))
				}
				if !errors.Is(err, cause) || !errors.Is(err, io.ErrUnexpectedEOF) || !body.closed {
					t.Fatalf("incomplete reply became success or lost its cause/cleanup: %v closed=%t", err, body.closed)
				}
				wantClass := ClassFromHTTPStatus(code)
				if code == 200 {
					wantClass = NotificationFailureConnectivity
				}
				if ClassifyNotificationFailureError(err) != wantClass || isRetryableWebhookError(err) != (code != 401) {
					t.Error("read error changed HTTP verdict or retry policy")
				}
				assertWithheldWebhookResponse(t, captured, err)
			})
		}
	}
}
