package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

func TestProxy_cleanupRequestPath_UsesConfiguredWorkDir(t *testing.T) {
	p := &Proxy{workDir: "/tmp/pulse-sensor-proxy-test"}
	got, err := p.cleanupRequestPath()
	if err != nil {
		t.Fatalf("cleanupRequestPath: %v", err)
	}
	want := filepath.Join(p.workDir, cleanupRequestFilename)
	if got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
}

func TestProxy_handleRequestCleanup_WritesValidPayloadAndReplacesExisting(t *testing.T) {
	workDir := t.TempDir()
	p := &Proxy{workDir: workDir}
	logger := zerolog.Nop()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := p.handleRequestCleanup(ctx, &RPCRequest{
		Method: RPCRequestCleanup,
		Params: map[string]interface{}{
			"host":   "pve-1",
			"reason": "testing",
		},
	}, logger)
	if err != nil {
		t.Fatalf("handleRequestCleanup: %v", err)
	}
	respMap, ok := resp.(map[string]any)
	if !ok {
		t.Fatalf("expected map response, got %#v", resp)
	}
	queued, ok := respMap["queued"].(bool)
	if !ok || !queued {
		t.Fatalf("expected queued=true response, got %#v", resp)
	}

	path, err := p.cleanupRequestPath()
	if err != nil {
		t.Fatalf("cleanupRequestPath: %v", err)
	}

	readPayload := func() map[string]any {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%s): %v", path, err)
		}
		var payload map[string]any
		if err := json.Unmarshal(data, &payload); err != nil {
			t.Fatalf("unmarshal payload: %v", err)
		}
		return payload
	}

	payload := readPayload()
	if payload["host"] != "pve-1" {
		t.Fatalf("payload host = %#v, want %q", payload["host"], "pve-1")
	}
	if payload["reason"] != "testing" {
		t.Fatalf("payload reason = %#v, want %q", payload["reason"], "testing")
	}
	if _, ok := payload["requestedAt"].(string); !ok {
		t.Fatalf("payload requestedAt missing or not string: %#v", payload["requestedAt"])
	}
	if ts, ok := payload["requestedAt"].(string); ok {
		if _, err := time.Parse(time.RFC3339, ts); err != nil {
			t.Fatalf("requestedAt not RFC3339: %q: %v", ts, err)
		}
	}

	if runtime.GOOS != "windows" {
		fi, err := os.Stat(path)
		if err != nil {
			t.Fatalf("Stat(%s): %v", path, err)
		}
		if fi.Mode().Perm() != 0o600 {
			t.Fatalf("payload file mode = %v, want %v", fi.Mode().Perm(), os.FileMode(0o600))
		}
	}

	resp, err = p.handleRequestCleanup(ctx, &RPCRequest{
		Method: RPCRequestCleanup,
		Params: map[string]interface{}{
			"host":   "pve-1",
			"reason": "testing-2",
		},
	}, logger)
	if err != nil {
		t.Fatalf("handleRequestCleanup (2): %v", err)
	}
	respMap, ok = resp.(map[string]any)
	if !ok {
		t.Fatalf("expected map response (2), got %#v", resp)
	}
	queued, ok = respMap["queued"].(bool)
	if !ok || !queued {
		t.Fatalf("expected queued=true response (2), got %#v", resp)
	}

	payload2 := readPayload()
	if payload2["reason"] != "testing-2" {
		t.Fatalf("payload reason after replace = %#v, want %q", payload2["reason"], "testing-2")
	}
}
