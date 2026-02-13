package sensors

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestCollectLocal_EmptyFallbackReturnsError(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "sensors", "#!/bin/sh\necho '{}'\n")
	writeScript(t, dir, "cat", "#!/bin/sh\necho ''\n")
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	_, err := CollectLocal(context.Background())
	if err == nil {
		t.Fatalf("expected error when sensors and Pi fallback are empty")
	}
	if !strings.Contains(err.Error(), "empty output") {
		t.Fatalf("expected empty output error, got %v", err)
	}
}

func TestCollectLocal_ContextCanceled(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "sensors", "#!/bin/sh\necho '{\"chip\":{\"temp\":{\"temp1_input\":42}}}'\n")
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := CollectLocal(ctx)
	if err == nil {
		t.Fatalf("expected command execution error for canceled context")
	}
	if !strings.Contains(err.Error(), "failed to execute sensors") {
		t.Fatalf("expected command execution error, got %v", err)
	}
}
