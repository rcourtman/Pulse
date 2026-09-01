package utils

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// UnsupportedContentEncodingError reports a Content-Encoding value that the
// server does not know how to decode. Callers can distinguish this from a
// malformed payload that claims to use a supported encoding.
type UnsupportedContentEncodingError struct {
	Encoding string
}

func (e *UnsupportedContentEncodingError) Error() string {
	return fmt.Sprintf("unsupported Content-Encoding: %s", e.Encoding)
}

// DecompressedBodyTooLargeError reports that a compressed body expanded past
// the decoded payload limit. It is intentionally typed so HTTP handlers do not
// misclassify the read failure as malformed JSON.
type DecompressedBodyTooLargeError struct {
	Limit int64
}

func (e *DecompressedBodyTooLargeError) Error() string {
	return fmt.Sprintf("decompressed payload exceeds %d byte limit", e.Limit)
}

// CompressJSON compresses a JSON payload using gzip BestSpeed.
// Returns the compressed bytes suitable for use as an HTTP request body.
func CompressJSON(payload []byte) ([]byte, error) {
	var buf bytes.Buffer
	gz, err := gzip.NewWriterLevel(&buf, gzip.BestSpeed)
	if err != nil {
		return nil, fmt.Errorf("create gzip writer: %w", err)
	}
	if _, err := gz.Write(payload); err != nil {
		return nil, fmt.Errorf("gzip write: %w", err)
	}
	if err := gz.Close(); err != nil {
		return nil, fmt.Errorf("gzip close: %w", err)
	}
	return buf.Bytes(), nil
}

// DecompressBodyIfGzipped inspects the Content-Encoding header of the request.
// If "gzip", it wraps the body with a gzip reader capped at maxDecompressed bytes.
// If empty (no encoding), it returns the body unchanged.
// If an unsupported encoding is specified, it returns an error.
func DecompressBodyIfGzipped(r *http.Request, maxDecompressed int64) (io.ReadCloser, error) {
	encoding := strings.TrimSpace(strings.ToLower(r.Header.Get("Content-Encoding")))

	switch encoding {
	case "":
		return r.Body, nil
	case "gzip":
		gz, err := gzip.NewReader(r.Body)
		if err != nil {
			return nil, fmt.Errorf("create gzip reader: %w", err)
		}
		limited := io.LimitReader(gz, maxDecompressed+1)
		return &cappedGzipReader{gz: gz, lr: limited, max: maxDecompressed}, nil
	default:
		return nil, &UnsupportedContentEncodingError{Encoding: encoding}
	}
}

// cappedGzipReader wraps a gzip.Reader with a size limit on decompressed output.
type cappedGzipReader struct {
	gz  *gzip.Reader
	lr  io.Reader
	max int64
	n   int64
	err error
}

func (c *cappedGzipReader) Read(p []byte) (int, error) {
	if c.err != nil {
		return 0, c.err
	}
	n, err := c.lr.Read(p)
	if c.n+int64(n) <= c.max {
		c.n += int64(n)
		return n, err
	}

	// LimitReader intentionally permits one byte beyond max so an exact-limit
	// payload remains distinguishable from an oversized one. Do not expose that
	// proof byte to callers: decoders are allowed to accept a complete value
	// even when Read returns data and an error together.
	allowed := c.max - c.n
	if allowed < 0 {
		allowed = 0
	}
	c.n = c.max
	c.err = &DecompressedBodyTooLargeError{Limit: c.max}
	return int(allowed), c.err
}

func (c *cappedGzipReader) Close() error {
	return c.gz.Close()
}
