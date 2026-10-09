package notifications

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
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
