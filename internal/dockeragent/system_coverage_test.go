package dockeragent

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadProcUptime(t *testing.T) {
	tests := []struct {
		name      string
		reader    func() (io.ReadCloser, error)
		wantError bool
	}{
		{
			name: "open error",
			reader: func() (io.ReadCloser, error) {
				return nil, errors.New("open failed")
			},
			wantError: true,
		},
		{
			name: "empty file",
			reader: func() (io.ReadCloser, error) {
				return io.NopCloser(strings.NewReader("")), nil
			},
			wantError: true,
		},
		{
			name: "invalid contents",
			reader: func() (io.ReadCloser, error) {
				return io.NopCloser(strings.NewReader("   ")), nil
			},
			wantError: true,
		},
		{
			name: "parse error",
			reader: func() (io.ReadCloser, error) {
				return io.NopCloser(strings.NewReader("notnum 0")), nil
			},
			wantError: true,
		},
		{
			name: "scanner error",
			reader: func() (io.ReadCloser, error) {
				return errReadCloser{err: errors.New("read failed")}, nil
			},
			wantError: true,
		},
		{
			name: "success",
			reader: func() (io.ReadCloser, error) {
				return io.NopCloser(strings.NewReader("123.45 0.00")), nil
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			swap(t, &openProcUptime, tt.reader)

			value, err := readProcUptime()
			if tt.wantError {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if value <= 0 {
				t.Fatalf("expected uptime > 0, got %f", value)
			}
		})
	}
}

func TestReadSystemUptime(t *testing.T) {
	t.Run("error returns zero", func(t *testing.T) {
		swap(t, &openProcUptime, func() (io.ReadCloser, error) {
			return nil, errors.New("boom")
		})

		if got := readSystemUptime(); got != 0 {
			t.Fatalf("expected 0, got %d", got)
		}
	})

	t.Run("success returns seconds", func(t *testing.T) {
		swap(t, &openProcUptime, func() (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("42.3 0.00")), nil
		})

		if got := readSystemUptime(); got != 42 {
			t.Fatalf("expected 42, got %d", got)
		}
	})
}

func TestReadMachineID(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "machine-id")
		if err := os.WriteFile(path, []byte("abc123\n"), 0600); err != nil {
			t.Fatalf("write machine-id: %v", err)
		}
		swap(t, &machineIDPaths, []string{path})

		got, err := readMachineID()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "abc123" {
			t.Fatalf("expected abc123, got %q", got)
		}
	})

	t.Run("not found", func(t *testing.T) {
		swap(t, &machineIDPaths, []string{filepath.Join(t.TempDir(), "missing")})

		if _, err := readMachineID(); err == nil {
			t.Fatal("expected error for missing machine-id")
		}
	})
}

func TestIsUnraid(t *testing.T) {
	t.Run("true when file exists", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "unraid-version")
		if err := os.WriteFile(path, []byte("6.12.0"), 0600); err != nil {
			t.Fatalf("write unraid-version: %v", err)
		}
		swap(t, &unraidVersionPath, path)

		if !isUnraid() {
			t.Fatal("expected isUnraid to be true")
		}
	})

	t.Run("false when missing", func(t *testing.T) {
		swap(t, &unraidVersionPath, filepath.Join(t.TempDir(), "missing"))
		if isUnraid() {
			t.Fatal("expected isUnraid to be false")
		}
	})
}
