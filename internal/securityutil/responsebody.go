package securityutil

import (
	"fmt"
	"io"
	"net/http"
)

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
		return fmt.Errorf("response body exceeds %d bytes", limit)
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
		return 0, fmt.Errorf("response body exceeds %d bytes", r.limit)
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
		return 0, fmt.Errorf("response body exceeds %d bytes", r.limit)
	}
	return 0, err
}

func (r *limitedResponseBody) Close() error {
	return r.body.Close()
}
