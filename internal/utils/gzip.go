package utils

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"strings"
)

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
		return nil, fmt.Errorf("unsupported Content-Encoding: %s", encoding)
	}
}

// cappedGzipReader wraps a gzip.Reader with a size limit on decompressed output.
type cappedGzipReader struct {
	gz  *gzip.Reader
	lr  io.Reader
	max int64
	n   int64
}

func (c *cappedGzipReader) Read(p []byte) (int, error) {
	n, err := c.lr.Read(p)
	c.n += int64(n)
	if c.n > c.max {
		return n, fmt.Errorf("decompressed payload exceeds %d byte limit", c.max)
	}
	return n, err
}

func (c *cappedGzipReader) Close() error {
	return c.gz.Close()
}
