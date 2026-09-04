package securityutil

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

type trackingReadCloser struct {
	io.Reader
	closed bool
}

func (r *trackingReadCloser) Close() error {
	r.closed = true
	return nil
}

func TestLimitResponseBody(t *testing.T) {
	t.Run("rejects declared oversize and closes body", func(t *testing.T) {
		body := &trackingReadCloser{Reader: strings.NewReader("oversized")}
		resp := &http.Response{Body: body, ContentLength: 9}

		err := LimitResponseBody(resp, 8)
		if err == nil || !strings.Contains(err.Error(), "response body exceeds 8 bytes") {
			t.Fatalf("LimitResponseBody() error = %v", err)
		}
		if !IsResponseBodyTooLarge(err) {
			t.Fatalf("IsResponseBodyTooLarge(%v) = false, want true", err)
		}
		if !body.closed {
			t.Fatal("oversized response body was not closed")
		}
	})

	t.Run("allows body exactly at limit", func(t *testing.T) {
		resp := &http.Response{
			Body:          io.NopCloser(strings.NewReader("12345678")),
			ContentLength: -1,
		}
		if err := LimitResponseBody(resp, 8); err != nil {
			t.Fatalf("LimitResponseBody() error = %v", err)
		}

		got, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("ReadAll() error = %v", err)
		}
		if string(got) != "12345678" {
			t.Fatalf("ReadAll() = %q", got)
		}
	})

	t.Run("rejects streamed oversize at boundary", func(t *testing.T) {
		resp := &http.Response{
			Body:          io.NopCloser(strings.NewReader("123456789")),
			ContentLength: -1,
		}
		if err := LimitResponseBody(resp, 8); err != nil {
			t.Fatalf("LimitResponseBody() error = %v", err)
		}

		got, err := io.ReadAll(resp.Body)
		if err == nil || !strings.Contains(err.Error(), "response body exceeds 8 bytes") {
			t.Fatalf("ReadAll() error = %v", err)
		}
		if !IsResponseBodyTooLarge(err) {
			t.Fatalf("IsResponseBodyTooLarge(%v) = false, want true", err)
		}
		if string(got) != "12345678" {
			t.Fatalf("ReadAll() returned bytes beyond limit: %q", got)
		}
	})

	t.Run("preserves close", func(t *testing.T) {
		body := &trackingReadCloser{Reader: strings.NewReader("ok")}
		resp := &http.Response{Body: body, ContentLength: 2}
		if err := LimitResponseBody(resp, 8); err != nil {
			t.Fatalf("LimitResponseBody() error = %v", err)
		}
		if err := resp.Body.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
		if !body.closed {
			t.Fatal("underlying response body was not closed")
		}
	})

	t.Run("validates arguments", func(t *testing.T) {
		if err := LimitResponseBody(nil, 8); err == nil {
			t.Fatal("expected nil response error")
		}
		resp := &http.Response{Body: io.NopCloser(strings.NewReader(""))}
		if err := LimitResponseBody(resp, -1); err == nil {
			t.Fatal("expected negative limit error")
		}
		if err := LimitResponseBody(&http.Response{}, 8); err == nil {
			t.Fatal("expected missing body error")
		}
	})
}
