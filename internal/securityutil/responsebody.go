package securityutil

import (
	"errors"
	"fmt"
	"io"
	"net/http"
)

// ResponseBodyTooLargeError reports that an HTTP response exceeded its
// caller-defined byte limit. The concrete type lets callers choose a bounded
// fallback without treating malformed or truncated responses the same way.
type ResponseBodyTooLargeError struct {
	Limit int64
}

func (e *ResponseBodyTooLargeError) Error() string {
	return fmt.Sprintf("response body exceeds %d bytes", e.Limit)
}

// IsResponseBodyTooLarge reports whether err was caused by a response crossing
// the limit enforced by LimitResponseBody.
func IsResponseBodyTooLarge(err error) bool {
	var target *ResponseBodyTooLargeError
	return errors.As(err, &target)
}

// LimitResponseBody bounds the bytes a caller can read from an HTTP response.
// It closes responses whose declared size already exceeds the limit. Responses
// without a trustworthy Content-Length remain bounded while they are read.
func LimitResponseBody(resp *http.Response, limit int64) error {
	if resp == nil || resp.Body == nil {
		return fmt.Errorf("response body is required")
	}
	if limit < 0 {
		return fmt.Errorf("response body limit must not be negative")
	}
	if resp.ContentLength > limit {
		_ = resp.Body.Close()
		return &ResponseBodyTooLargeError{Limit: limit}
	}

	resp.Body = &limitedResponseBody{
		body:      resp.Body,
		remaining: limit,
		limit:     limit,
	}
	return nil
}

type limitedResponseBody struct {
	body      io.ReadCloser
	remaining int64
	limit     int64
	exceeded  bool
}

func (r *limitedResponseBody) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if r.exceeded {
		return 0, &ResponseBodyTooLargeError{Limit: r.limit}
	}
	if r.remaining > 0 {
		if int64(len(p)) > r.remaining {
			p = p[:r.remaining]
		}
		n, err := r.body.Read(p)
		r.remaining -= int64(n)
		return n, err
	}

	// Probe for one additional byte. This distinguishes a body exactly at the
	// limit from an oversized body without exposing bytes beyond the boundary.
	var probe [1]byte
	n, err := r.body.Read(probe[:])
	if n > 0 {
		r.exceeded = true
		return 0, &ResponseBodyTooLargeError{Limit: r.limit}
	}
	return 0, err
}

func (r *limitedResponseBody) Close() error {
	return r.body.Close()
}
