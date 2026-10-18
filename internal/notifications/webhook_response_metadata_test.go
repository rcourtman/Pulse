package notifications

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestWebhookResponseMetadataBoundAndSummary(t *testing.T) {
	for _, size := range []int{0, 1, WebhookMaxResponseSize - 1, WebhookMaxResponseSize, WebhookMaxResponseSize + 1000} {
		for _, code := range []int{200, 403} {
			t.Run(fmt.Sprintf("%d/%d", code, size), func(t *testing.T) {
				body := &trackedWebhookResponse{reader: strings.NewReader(strings.Repeat("x", size))}
				result, err := readWebhookResponse(&http.Response{StatusCode: code,
					Header: http.Header{"Retry-After": {"42"}}, Body: body})
				wantBytes := int64(min(size, WebhookMaxResponseSize))
				if (err == nil) != (code == 200) || body.read != wantBytes || result.responseBytes != wantBytes ||
					result.responseLimitReached != (size >= WebhookMaxResponseSize) || result.responseIncomplete ||
					result.statusCode != code || result.headers.Get("Retry-After") != "42" {
					t.Fatalf("response metadata/verdict/bound changed: result=%+v err=%v bytes=%d", result, err, body.read)
				}
				wantSummary := ""
				if size > 0 {
					suffix := ""
					if size >= WebhookMaxResponseSize {
						suffix = "; read limit reached"
					}
					wantSummary = fmt.Sprintf("Response body withheld (%d bytes read%s)", wantBytes, suffix)
				}
				if summary := result.responseSummary(); summary != wantSummary {
					t.Errorf("Test summary = %q; want %q", summary, wantSummary)
				}
			})
		}
	}
}

func TestWebhookResponseMetadataIncompleteSummary(t *testing.T) {
	for _, text := range []string{"", webhookResponsePrivateText} {
		body := &trackedWebhookResponse{reader: strings.NewReader(text), cause: io.ErrUnexpectedEOF}
		result, err := readWebhookResponse(&http.Response{StatusCode: 200, Header: make(http.Header), Body: body})
		if !errors.Is(err, io.ErrUnexpectedEOF) || !result.responseIncomplete || result.responseBytes != int64(len(text)) ||
			result.responseSummary() != fmt.Sprintf("Response body withheld (%d bytes read; read incomplete)", len(text)) {
			t.Fatalf("incomplete response lost cause or Test summary: result=%+v err=%v", result, err)
		}
	}
}
