package main

import (
	"bytes"
	"encoding/base64"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/pkg/pulsecli"
)

func newTestCLIEnv() *pulsecli.Env {
	return pulsecli.NewEnv()
}

func newTestCLIProcess() pulsecli.Process {
	process := pulsecli.NewProcess()
	process.Exit = func(int) {}
	return process
}

func createTestEncryptionKey(t *testing.T, dir string) {
	t.Helper()

	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	encoded := base64.StdEncoding.EncodeToString(key)
	if err := os.WriteFile(filepath.Join(dir, ".encryption.key"), []byte(encoded), 0o600); err != nil {
		t.Fatalf("failed to create test encryption key: %v", err)
	}
}

func captureOutput(f func()) string {
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stdout = w
	os.Stderr = w

	f()

	_ = w.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String()
}
