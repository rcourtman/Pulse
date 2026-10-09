package notifications

import (
	"fmt"
	"io"
	"net/http"
)

// Receivers can echo credentials, alert content or arbitrary provider text.
// Keep only bounded byte counts and structured delivery facts, not a copy of
// the body that another caller could put in logs, history or a Test response.
func readWebhookResponse(resp *http.Response) (*webhookHTTPResult, error) {
	result := &webhookHTTPResult{statusCode: resp.StatusCode, headers: resp.Header.Clone()}
	var err error
	result.responseBytes, err = io.Copy(io.Discard, io.LimitReader(resp.Body, WebhookMaxResponseSize))
	result.responseLimitReached = result.responseBytes == WebhookMaxResponseSize
	result.responseIncomplete = err != nil
	if err != nil {
		// Preserve identity and classification before hiding the error prose.
		// A body-read error can itself contain the receiver's private text.
		class := ClassifyNotificationFailureError(err)
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			class = ClassFromHTTPStatus(resp.StatusCode)
		}
		return result, FailWithClass(class, &webhookResponseReadError{status: resp.StatusCode, cause: err})
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return result, FailfWithClass(ClassFromHTTPStatus(resp.StatusCode),
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
