package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadBoundedRegularFileSuccess(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.enc")
	want := []byte("encrypted-config")
	if err := os.WriteFile(path, want, 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := readBoundedRegularFile(path, int64(len(want)))
	if err != nil {
		t.Fatalf("readBoundedRegularFile: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("got %q, want %q", string(got), string(want))
	}
}

func TestReadBoundedRegularFileRejectsOversized(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.enc")
	if err := os.WriteFile(path, []byte("0123456789"), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := readBoundedRegularFile(path, 8)
	if err == nil {
		t.Fatal("expected oversized file error")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReadBoundedRegularFileRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.enc")
	if err := os.WriteFile(target, []byte("ok"), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	link := filepath.Join(dir, "config.enc")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}

	_, err := readBoundedRegularFile(link, 1024)
	if err == nil {
		t.Fatal("expected non-regular file error")
	}
	if !strings.Contains(err.Error(), "regular file") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReadBoundedHTTPBodySuccess(t *testing.T) {
	data := []byte("payload")
	got, err := readBoundedHTTPBody(bytes.NewReader(data), int64(len(data)), int64(len(data)), "configuration response")
	if err != nil {
		t.Fatalf("readBoundedHTTPBody: %v", err)
	}
	if string(got) != string(data) {
		t.Fatalf("got %q, want %q", string(got), string(data))
	}
}

func TestReadBoundedHTTPBodyRejectsOversizedContentLength(t *testing.T) {
	_, err := readBoundedHTTPBody(bytes.NewReader([]byte("ok")), 9, 8, "configuration response")
	if err == nil {
		t.Fatal("expected oversized content-length error")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReadBoundedHTTPBodyRejectsOversizedStream(t *testing.T) {
	_, err := readBoundedHTTPBody(bytes.NewReader([]byte("0123456789")), -1, 8, "configuration response")
	if err == nil {
		t.Fatal("expected oversized stream error")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("unexpected error: %v", err)
	}
}
