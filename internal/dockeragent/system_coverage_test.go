package dockeragent

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
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

	t.Run("formats 32-char hex as uuid", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "machine-id")
		if err := os.WriteFile(path, []byte("0123456789abcdef0123456789abcdef\n"), 0600); err != nil {
			t.Fatalf("write machine-id: %v", err)
		}
		swap(t, &machineIDPaths, []string{path})

		got, err := readMachineID()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := "01234567-89ab-cdef-0123-456789abcdef"
		if got != want {
			t.Fatalf("expected %s, got %q", want, got)
		}
	})

	t.Run("keeps non-hex 32-char machine id unchanged", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "machine-id")
		raw := "0123456789abcdef0123456789abcdeg"
		if err := os.WriteFile(path, []byte(raw+"\n"), 0600); err != nil {
			t.Fatalf("write machine-id: %v", err)
		}
		swap(t, &machineIDPaths, []string{path})

		got, err := readMachineID()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != raw {
			t.Fatalf("expected machine-id %q, got %q", raw, got)
		}
	})

	t.Run("not found", func(t *testing.T) {
		swap(t, &machineIDPaths, []string{filepath.Join(t.TempDir(), "missing")})

		if _, err := readMachineID(); err == nil {
			t.Fatal("expected error for missing machine-id")
		}
	})
}

func TestIsHexString(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "lowercase", value: "abcdef012345", want: true},
		{name: "uppercase", value: "ABCDEF012345", want: true},
		{name: "mixed case", value: "aBcDeF012345", want: true},
		{name: "invalid character", value: "abcdxef0zz", want: false},
		{name: "empty string", value: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := utils.IsHexString(tt.value); got != tt.want {
				t.Fatalf("utils.IsHexString(%q) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
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
