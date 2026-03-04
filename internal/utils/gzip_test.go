package utils

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestCompressJSON_RoundTrip(t *testing.T) {
	original := []byte(`{"hostname":"test-host","cpu":42.5,"disks":[{"mount":"/","used":80}]}`)

	compressed, err := CompressJSON(original)
	if err != nil {
		t.Fatalf("CompressJSON: %v", err)
	}

	if len(compressed) >= len(original) {
		t.Logf("warning: compressed (%d) >= original (%d) for small payload (expected for tiny inputs)", len(compressed), len(original))
	}

	// Decompress and verify round-trip
	gz, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatalf("gzip.NewReader: %v", err)
	}
	defer gz.Close()

	decompressed, err := io.ReadAll(gz)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}

	if !bytes.Equal(original, decompressed) {
		t.Fatalf("round-trip mismatch:\n  got:  %s\n  want: %s", decompressed, original)
	}
}

func TestCompressJSON_LargePayload(t *testing.T) {
	// Simulate a realistic agent report (~100KB of repetitive JSON)
	var buf bytes.Buffer
	buf.WriteString(`{"containers":[`)
	for i := 0; i < 500; i++ {
		if i > 0 {
			buf.WriteByte(',')
		}
		buf.WriteString(`{"id":"abc123","name":"container-name","cpu":12.5,"memory":{"used":1073741824,"total":8589934592}}`)
	}
	buf.WriteString(`]}`)

	original := buf.Bytes()
	compressed, err := CompressJSON(original)
	if err != nil {
		t.Fatalf("CompressJSON: %v", err)
	}

	ratio := float64(len(compressed)) / float64(len(original)) * 100
	t.Logf("compression: %d -> %d bytes (%.1f%%)", len(original), len(compressed), ratio)

	if ratio > 30 {
		t.Errorf("expected >70%% compression on repetitive JSON, got %.1f%% size remaining", ratio)
	}
}

func TestDecompressBodyIfGzipped_Uncompressed(t *testing.T) {
	original := `{"test":"data"}`
	req, _ := http.NewRequest("POST", "/", strings.NewReader(original))

	body, err := DecompressBodyIfGzipped(req, 1024)
	if err != nil {
		t.Fatalf("DecompressBodyIfGzipped: %v", err)
	}
	defer body.Close()

	data, _ := io.ReadAll(body)
	if string(data) != original {
		t.Fatalf("got %q, want %q", data, original)
	}
}

func TestDecompressBodyIfGzipped_Gzipped(t *testing.T) {
	original := []byte(`{"hostname":"test","cpu":99.9}`)

	compressed, err := CompressJSON(original)
	if err != nil {
		t.Fatalf("CompressJSON: %v", err)
	}

	req, _ := http.NewRequest("POST", "/", bytes.NewReader(compressed))
	req.Header.Set("Content-Encoding", "gzip")

	body, err := DecompressBodyIfGzipped(req, 1024)
	if err != nil {
		t.Fatalf("DecompressBodyIfGzipped: %v", err)
	}
	defer body.Close()

	data, _ := io.ReadAll(body)
	if !bytes.Equal(data, original) {
		t.Fatalf("round-trip mismatch: got %q, want %q", data, original)
	}
}

func TestDecompressBodyIfGzipped_UnsupportedEncoding(t *testing.T) {
	req, _ := http.NewRequest("POST", "/", strings.NewReader("data"))
	req.Header.Set("Content-Encoding", "deflate")

	_, err := DecompressBodyIfGzipped(req, 1024)
	if err == nil {
		t.Fatal("expected error for unsupported encoding")
	}
	if !strings.Contains(err.Error(), "unsupported Content-Encoding") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDecompressBodyIfGzipped_BombProtection(t *testing.T) {
	// Create a gzip payload that decompresses to more than the limit
	var buf bytes.Buffer
	gz, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	// Write 2KB of zeros (compresses very well)
	payload := make([]byte, 2048)
	gz.Write(payload)
	gz.Close()

	req, _ := http.NewRequest("POST", "/", bytes.NewReader(buf.Bytes()))
	req.Header.Set("Content-Encoding", "gzip")

	// Set limit to 1KB — decompressed output is 2KB, should fail
	body, err := DecompressBodyIfGzipped(req, 1024)
	if err != nil {
		t.Fatalf("DecompressBodyIfGzipped: %v", err)
	}
	defer body.Close()

	_, readErr := io.ReadAll(body)
	if readErr == nil {
		t.Fatal("expected error when decompressed size exceeds limit")
	}
	if !strings.Contains(readErr.Error(), "exceeds") {
		t.Fatalf("unexpected error: %v", readErr)
	}
}
