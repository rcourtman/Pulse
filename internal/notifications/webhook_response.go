package notifications

import (
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// maxNotificationRetryAfter bounds destination-supplied Retry-After and
// X-RateLimit-Reset-After hints so a misconfigured or hostile receiver cannot
// defer an alert indefinitely.
const maxNotificationRetryAfter = 15 * time.Minute

func parseNotificationRetryAfterSeconds(raw string) (time.Duration, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, false
	}
	secs, err := strconv.ParseFloat(trimmed, 64)
	if err != nil || math.IsNaN(secs) || math.IsInf(secs, 0) || secs <= 0 {
		return 0, false
	}
	if secs >= maxNotificationRetryAfter.Seconds() {
		return maxNotificationRetryAfter, true
	}
	delay := time.Duration(math.Ceil(secs)) * time.Second
	if delay < time.Second {
		delay = time.Second
	}
	if delay > maxNotificationRetryAfter {
		delay = maxNotificationRetryAfter
	}
	return delay, true
}

// parseNotificationRetryAfter parses bounded retry-delay headers from HTTP
// 429/503 responses without reading or logging any response body or echoing
// raw header text.
func parseNotificationRetryAfter(headers http.Header, statusCode int, now time.Time) (time.Duration, bool) {
	if headers == nil {
		return 0, false
	}
	if statusCode != http.StatusTooManyRequests && statusCode != http.StatusServiceUnavailable {
		return 0, false
	}
	if raw := strings.TrimSpace(headers.Get("Retry-After")); raw != "" {
		if delay, ok := parseNotificationRetryAfterSeconds(raw); ok {
			return delay, true
		}
		if retryAt, err := http.ParseTime(raw); err == nil {
			delay := retryAt.Sub(now)
			if delay > 0 {
				if delay < time.Second {
					delay = time.Second
				}
				if delay > maxNotificationRetryAfter {
					delay = maxNotificationRetryAfter
				}
				return delay, true
			}
		}
	}
	if statusCode == http.StatusTooManyRequests {
		if delay, ok := parseNotificationRetryAfterSeconds(headers.Get("X-RateLimit-Reset-After")); ok {
			return delay, true
		}
	}
	return 0, false
}

// Receivers can echo credentials, alert content or arbitrary provider text.
// Keep only bounded byte counts and structured delivery facts, not a copy of
// the body that another caller could put in logs, history or a Test response.
func readWebhookResponse(resp *http.Response) (*webhookHTTPResult, error) {
	result := &webhookHTTPResult{statusCode: resp.StatusCode, headers: resp.Header.Clone()}
	var err error
	result.responseBytes, err = io.Copy(io.Discard, io.LimitReader(resp.Body, WebhookMaxResponseSize))
	result.responseLimitReached = result.responseBytes == WebhookMaxResponseSize
	result.responseIncomplete = err != nil
	retryAfter, _ := parseNotificationRetryAfter(result.headers, resp.StatusCode, time.Now())
	if err != nil {
		// Preserve identity and classification before hiding the error prose.
		// A body-read error can itself contain the receiver's private text.
		class := ClassifyNotificationFailureError(err)
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			class = ClassFromHTTPStatus(resp.StatusCode)
		}
		return result, FailWithClassAndRetryAfter(class, retryAfter, &webhookResponseReadError{status: resp.StatusCode, cause: err})
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return result, FailfWithClassAndRetryAfter(ClassFromHTTPStatus(resp.StatusCode), retryAfter,
			"webhook returned HTTP %d: %s (response body withheld)", resp.StatusCode, http.StatusText(resp.StatusCode))
	}
	return result, nil
}

type webhookResponseReadError struct {
	status int
	cause  error
}

func (e *webhookResponseReadError) Error() string {
	if e.status < 200 || e.status >= 300 {
		return fmt.Sprintf("webhook returned HTTP %d: failed to read response body (details withheld)", e.status)
	}
	return "failed to read webhook response body (details withheld)"
}

func (e *webhookResponseReadError) Unwrap() error { return e.cause }

func (r *webhookHTTPResult) responseSummary() string {
	if r.responseBytes == 0 && !r.responseIncomplete {
		return ""
	}
	suffix := ""
	if r.responseIncomplete {
		suffix = "; read incomplete"
	} else if r.responseLimitReached {
		// Reaching the cap is not proof that the body exceeded it.
		suffix = "; read limit reached"
	}
	return fmt.Sprintf("Response body withheld (%d bytes read%s)", r.responseBytes, suffix)
}
