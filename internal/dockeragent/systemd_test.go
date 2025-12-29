package dockeragent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSystemctl(t *testing.T, script string) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "systemctl")
	content := "#!/bin/sh\n" + script + "\n"
	if err := os.WriteFile(path, []byte(content), 0755); err != nil {
		t.Fatalf("write systemctl: %v", err)
	}

	prevPath := os.Getenv("PATH")
	if err := os.Setenv("PATH", dir); err != nil {
		t.Fatalf("set PATH: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Setenv("PATH", prevPath)
	})

	return path
}

func TestDisableSystemdService(t *testing.T) {
	t.Run("no systemctl", func(t *testing.T) {
		prev := os.Getenv("PATH")
		_ = os.Setenv("PATH", "")
		t.Cleanup(func() {
			_ = os.Setenv("PATH", prev)
		})

		if err := disableSystemdService(context.Background(), "pulse-docker-agent"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("success", func(t *testing.T) {
		writeSystemctl(t, "exit 0")
		if err := disableSystemdService(context.Background(), "pulse-docker-agent"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("not found exit code", func(t *testing.T) {
		writeSystemctl(t, "echo 'unit not-found' >&2\nexit 5")
		if err := disableSystemdService(context.Background(), "pulse-docker-agent"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("access denied", func(t *testing.T) {
		writeSystemctl(t, "echo 'access denied' >&2\nexit 1")
		err := disableSystemdService(context.Background(), "pulse-docker-agent")
		if err == nil || !strings.Contains(err.Error(), "access denied") {
			t.Fatalf("expected access denied error, got %v", err)
		}
	})

	t.Run("other error", func(t *testing.T) {
		writeSystemctl(t, "echo 'boom' >&2\nexit 2")
		if err := disableSystemdService(context.Background(), "pulse-docker-agent"); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestStopSystemdService(t *testing.T) {
	t.Run("no systemctl", func(t *testing.T) {
		prev := os.Getenv("PATH")
		_ = os.Setenv("PATH", "")
		t.Cleanup(func() {
			_ = os.Setenv("PATH", prev)
		})

		if err := stopSystemdService(context.Background(), "pulse-docker-agent"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("success", func(t *testing.T) {
		writeSystemctl(t, "exit 0")
		if err := stopSystemdService(context.Background(), "pulse-docker-agent"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("not found exit code", func(t *testing.T) {
		writeSystemctl(t, "echo 'could not be found' >&2\nexit 5")
		if err := stopSystemdService(context.Background(), "pulse-docker-agent"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("permission denied", func(t *testing.T) {
		writeSystemctl(t, "echo 'permission denied' >&2\nexit 1")
		err := stopSystemdService(context.Background(), "pulse-docker-agent")
		if err == nil || !strings.Contains(err.Error(), "access denied") {
			t.Fatalf("expected access denied error, got %v", err)
		}
	})

	t.Run("other error", func(t *testing.T) {
		writeSystemctl(t, "echo 'boom' >&2\nexit 2")
		if err := stopSystemdService(context.Background(), "pulse-docker-agent"); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestRemoveFileIfExists(t *testing.T) {
	t.Run("missing file", func(t *testing.T) {
		if err := removeFileIfExists(filepath.Join(t.TempDir(), "missing")); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("removes file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "file")
		if err := os.WriteFile(path, []byte("data"), 0600); err != nil {
			t.Fatalf("write file: %v", err)
		}
		if err := removeFileIfExists(path); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("expected file to be removed")
		}
	})

	t.Run("remove error", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "dir")
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		nested := filepath.Join(dir, "file")
		if err := os.WriteFile(nested, []byte("data"), 0600); err != nil {
			t.Fatalf("write nested file: %v", err)
		}
		if err := removeFileIfExists(dir); err == nil {
			t.Fatal("expected error")
		}
	})
}
