package notifications

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestWebhookResponseConfidentialityInvalidRetryHint(t *testing.T) {
	for _, finalCode := range []int{http.StatusForbidden, http.StatusAccepted} {
		t.Run(fmt.Sprint(finalCode), func(t *testing.T) {
			captured := captureWebhookResponseLogs(t)
			attempts := 0
			n := responseFixtureManager(t, func(*http.Request) (*http.Response, error) {
				attempts++
				code := http.StatusTooManyRequests
				if attempts > 1 {
					code = finalCode
				}
				return &http.Response{StatusCode: code, Header: http.Header{"Retry-After": {webhookResponsePrivateText}},
					Body: io.NopCloser(strings.NewReader(""))}, nil
			})
			err := n.sendWebhookWithRetry(EnhancedWebhookConfig{
				WebhookConfig: WebhookConfig{Name: "response fixture", URL: "http://127.0.0.1/hook"}, RetryCount: 1,
			}, []byte(`{}`), "fixture:alert")
			if attempts != 2 || (err == nil) != (finalCode == http.StatusAccepted) ||
				!strings.Contains(captured.String(), "invalid Retry-After header; falling back to exponential backoff") {
				t.Fatalf("invalid hint changed fallback or verdict: attempts=%d error=%v", attempts, err)
			}
			assertWithheldWebhookResponse(t, captured, err)
		})
	}
}

func TestReadWebhookResponsePropagatesRetryAfterAndDiscordResetHeader(t *testing.T) {
	now := time.Date(2026, time.November, 21, 9, 0, 0, 0, time.UTC)

	t.Run("parses integer, fractional, HTTP-date, and Discord reset headers with 15m cap", func(t *testing.T) {
		cases := []struct {
			name       string
			statusCode int
			headers    http.Header
			want       time.Duration
			wantOK     bool
		}{
			{
				name:       "429 integer Retry-After",
				statusCode: http.StatusTooManyRequests,
				headers:    http.Header{"Retry-After": {"45"}},
				want:       45 * time.Second,
				wantOK:     true,
			},
			{
				name:       "429 fractional Retry-After rounds up to whole seconds",
				statusCode: http.StatusTooManyRequests,
				headers:    http.Header{"Retry-After": {"1.25"}},
				want:       2 * time.Second,
				wantOK:     true,
			},
			{
				name:       "429 Discord X-RateLimit-Reset-After fallback",
				statusCode: http.StatusTooManyRequests,
				headers:    http.Header{"X-Ratelimit-Reset-After": {"12.4"}},
				want:       13 * time.Second,
				wantOK:     true,
			},
			{
				name:       "503 HTTP-date Retry-After",
				statusCode: http.StatusServiceUnavailable,
				headers:    http.Header{"Retry-After": {now.Add(90 * time.Second).UTC().Format(http.TimeFormat)}},
				want:       90 * time.Second,
				wantOK:     true,
			},
			{
				name:       "clamps excessive Retry-After at 15m",
				statusCode: http.StatusTooManyRequests,
				headers:    http.Header{"Retry-After": {"86400"}},
				want:       maxNotificationRetryAfter,
				wantOK:     true,
			},
			{
				name:       "ignores Retry-After on non-429/503 status",
				statusCode: http.StatusBadRequest,
				headers:    http.Header{"Retry-After": {"45"}},
				want:       0,
				wantOK:     false,
			},
			{
				name:       "ignores invalid Retry-After safely",
				statusCode: http.StatusTooManyRequests,
				headers:    http.Header{"Retry-After": {webhookResponsePrivateText}},
				want:       0,
				wantOK:     false,
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				got, ok := parseNotificationRetryAfter(tc.headers, tc.statusCode, now)
				if ok != tc.wantOK || got != tc.want {
					t.Fatalf("parseNotificationRetryAfter() = (%v, %v), want (%v, %v)", got, ok, tc.want, tc.wantOK)
				}
			})
		}
	})

	t.Run("attaches Retry-After onto readWebhookResponse error and survives safeAppriseError wrapping", func(t *testing.T) {
		resp := &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Header:     http.Header{"Retry-After": {"42"}},
			Body:       io.NopCloser(strings.NewReader(webhookResponsePrivateText)),
		}
		_, err := readWebhookResponse(resp)
		if err == nil {
			t.Fatal("expected error for HTTP 429")
		}
		if strings.Contains(err.Error(), webhookResponsePrivateText) {
			t.Fatalf("response body leaked into error: %v", err)
		}
		got, ok := RetryAfterFromError(err)
		if !ok || got != 42*time.Second {
			t.Fatalf("RetryAfterFromError(readWebhookResponse) = (%v, %v), want (42s, true)", got, ok)
		}

		wrapped := safeAppriseError("apprise server returned HTTP 429", err)
		gotWrapped, okWrapped := RetryAfterFromError(wrapped)
		if !okWrapped || gotWrapped != 42*time.Second {
			t.Fatalf("RetryAfterFromError(safeAppriseError) = (%v, %v), want (42s, true)", gotWrapped, okWrapped)
		}
	})
}
