package system

import (
	"errors"
	"os"
	"testing"
)

func resetSystemFns() {
	envGetFn = os.Getenv
	statFn = os.Stat
	readFileFn = boundedReadFile
	hostnameFn = os.Hostname
}

func TestIsHexString(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"0123456789abcdef", true},
		{"ABCDEF", true},
		{"0123456789ABCDEF", true},
		{"abc123", true},
		{"", true}, // empty string has no non-hex chars
		{"xyz", false},
		{"123g", false},
		{"hello-world", false},
		{"12 34", false},  // space
		{"12\n34", false}, // newline
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := isHexString(tc.input)
			if result != tc.expected {
				t.Errorf("isHexString(%q) = %v, want %v", tc.input, result, tc.expected)
			}
		})
	}
}

func TestIsNumeric(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"123", true},
		{"0", true},
		{"9999999", true},
		{" 123 ", true}, // whitespace trimmed
		{"", false},
		{"   ", false},
		{"abc", false},
		{"12.3", false},
		{"-1", false},
		{"1a2", false},
		{"12 34", false},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := isNumeric(tc.input)
			if result != tc.expected {
				t.Errorf("isNumeric(%q) = %v, want %v", tc.input, result, tc.expected)
			}
		})
	}
}

func TestIsTruthy(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		// Truthy values
		{"1", true},
		{"true", true},
		{"TRUE", true},
		{"True", true},
		{"t", true},
		{"T", true},
		{"yes", true},
		{"YES", true},
		{"Yes", true},
		{"y", true},
		{"Y", true},
		{"on", true},
		{"ON", true},
		{"On", true},
		{" true ", true}, // with whitespace
		{"\ttrue\n", true},

		// Falsy values
		{"0", false},
		{"false", false},
		{"FALSE", false},
		{"no", false},
		{"n", false},
		{"off", false},
		{"", false},
		{"   ", false},
		{"random", false},
		{"2", false},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := isTruthy(tc.input)
			if result != tc.expected {
				t.Errorf("isTruthy(%q) = %v, want %v", tc.input, result, tc.expected)
			}
		})
	}
}

func TestContainerMarkers(t *testing.T) {
	// Verify all expected markers are present
	expectedMarkers := []string{
		"docker",
		"lxc",
		"containerd",
		"kubepods",
		"podman",
		"crio",
		"libpod",
		"lxcfs",
	}

	for _, expected := range expectedMarkers {
		found := false
		for _, marker := range containerMarkers {
			if marker == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected container marker %q not found", expected)
		}
	}
}

func TestReadFileWithLimit(t *testing.T) {
	t.Run("WithinLimit", func(t *testing.T) {
		filePath := t.TempDir() + "/sample.txt"
		if err := os.WriteFile(filePath, []byte("hello"), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		data, err := readFileWithLimit(filePath, 5)
		if err != nil {
			t.Fatalf("readFileWithLimit returned error: %v", err)
		}
		if string(data) != "hello" {
			t.Fatalf("expected %q, got %q", "hello", string(data))
		}
	})

	t.Run("ExceedsLimit", func(t *testing.T) {
		filePath := t.TempDir() + "/sample.txt"
		if err := os.WriteFile(filePath, []byte("hello"), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		data, err := readFileWithLimit(filePath, 4)
		if !errors.Is(err, errFileTooLarge) {
			t.Fatalf("expected errFileTooLarge, got err=%v data=%q", err, string(data))
		}
	})

	t.Run("InvalidLimit", func(t *testing.T) {
		filePath := t.TempDir() + "/sample.txt"
		if err := os.WriteFile(filePath, []byte("hello"), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		_, err := readFileWithLimit(filePath, 0)
		if !errors.Is(err, errInvalidReadMax) {
			t.Fatalf("expected errInvalidReadMax, got %v", err)
		}
	})
}

func TestInContainer(t *testing.T) {
	t.Run("Forced", func(t *testing.T) {
		t.Cleanup(resetSystemFns)
		envGetFn = func(key string) string {
			if key == "PULSE_FORCE_CONTAINER" {
				return "true"
			}
			return ""
		}

		if !InContainer() {
			t.Fatal("expected forced container")
		}
	})

	t.Run("DockerEnvFile", func(t *testing.T) {
		t.Cleanup(resetSystemFns)
		envGetFn = func(string) string { return "" }
		statFn = func(path string) (os.FileInfo, error) {
			if path == "/.dockerenv" {
				return nil, nil
			}
			return nil, errors.New("missing")
		}
		readFileFn = func(string) ([]byte, error) { return nil, errors.New("missing") }

		if !InContainer() {
			t.Fatal("expected docker env file")
		}
	})

	t.Run("ContainerEnvFile", func(t *testing.T) {
		t.Cleanup(resetSystemFns)
		envGetFn = func(string) string { return "" }
		statFn = func(path string) (os.FileInfo, error) {
			if path == "/run/.containerenv" {
				return nil, nil
			}
			return nil, errors.New("missing")
		}
		readFileFn = func(string) ([]byte, error) { return nil, errors.New("missing") }

		if !InContainer() {
			t.Fatal("expected container env file")
		}
	})

	t.Run("ContainerEnvVar", func(t *testing.T) {
		t.Cleanup(resetSystemFns)
		envGetFn = func(key string) string {
			if key == "container" {
				return "lxc"
			}
			return ""
		}
		statFn = func(string) (os.FileInfo, error) { return nil, errors.New("missing") }
		readFileFn = func(string) ([]byte, error) { return nil, errors.New("missing") }

		if !InContainer() {
			t.Fatal("expected container env var")
		}
	})

	t.Run("ProcEnviron", func(t *testing.T) {
		t.Cleanup(resetSystemFns)
		envGetFn = func(string) string { return "" }
		statFn = func(string) (os.FileInfo, error) { return nil, errors.New("missing") }
		readFileFn = func(path string) ([]byte, error) {
			if path == "/proc/1/environ" {
				return []byte("container=lxc\x00"), nil
			}
			return nil, errors.New("missing")
		}

		if !InContainer() {
			t.Fatal("expected container environ")
		}
	})

	t.Run("CgroupMarker", func(t *testing.T) {
		t.Cleanup(resetSystemFns)
		envGetFn = func(string) string { return "" }
		statFn = func(string) (os.FileInfo, error) { return nil, errors.New("missing") }
		readFileFn = func(path string) ([]byte, error) {
			if path == "/proc/1/environ" {
				return []byte("container=host"), nil
			}
			if path == "/proc/1/cgroup" {
				return []byte("0::/docker/abc"), nil
			}
			return nil, errors.New("missing")
		}

		if !InContainer() {
			t.Fatal("expected container cgroup marker")
		}
	})

	t.Run("NotContainer", func(t *testing.T) {
		t.Cleanup(resetSystemFns)
		envGetFn = func(string) string { return "" }
		statFn = func(string) (os.FileInfo, error) { return nil, errors.New("missing") }
		readFileFn = func(path string) ([]byte, error) {
			if path == "/proc/1/environ" {
				return []byte("container=host"), nil
			}
			if path == "/proc/1/cgroup" {
				return []byte("0::/user.slice"), nil
			}
			return nil, errors.New("missing")
		}

		if InContainer() {
			t.Fatal("expected non-container")
		}
	})

	t.Run("OversizedProcFilesIgnored", func(t *testing.T) {
		t.Cleanup(resetSystemFns)
		envGetFn = func(string) string { return "" }
		statFn = func(string) (os.FileInfo, error) { return nil, errors.New("missing") }
		readFileFn = func(path string) ([]byte, error) {
			if path == "/proc/1/environ" || path == "/proc/1/cgroup" {
				return nil, errFileTooLarge
			}
			return nil, errors.New("missing")
		}

		if InContainer() {
			t.Fatal("expected oversized proc files to be treated as non-container")
		}
	})
}

func TestDetectDockerContainerName(t *testing.T) {
	t.Run("HostnameName", func(t *testing.T) {
		t.Cleanup(resetSystemFns)
		hostnameFn = func() (string, error) { return "my-container", nil }

		if got := DetectDockerContainerName(); got != "my-container" {
			t.Fatalf("expected hostname name, got %q", got)
		}
	})

	t.Run("HostnameHexShort", func(t *testing.T) {
		t.Cleanup(resetSystemFns)
		hostnameFn = func() (string, error) { return "abcdef123456", nil }
		readFileFn = func(string) ([]byte, error) { return []byte("0::/docker/abcdef"), nil }

		if got := DetectDockerContainerName(); got != "" {
			t.Fatalf("expected empty name, got %q", got)
		}
	})

	t.Run("HostnameHexLong", func(t *testing.T) {
		t.Cleanup(resetSystemFns)
		hostnameFn = func() (string, error) { return "abcdef1234567890abcdef1234567890", nil }

		if got := DetectDockerContainerName(); got == "" {
			t.Fatal("expected hostname for long hex")
		}
	})

	t.Run("HostnameError", func(t *testing.T) {
		t.Cleanup(resetSystemFns)
		hostnameFn = func() (string, error) { return "", errors.New("fail") }
		readFileFn = func(string) ([]byte, error) { return []byte("0::/docker/abcdef"), nil }

		if got := DetectDockerContainerName(); got != "" {
			t.Fatalf("expected empty name, got %q", got)
		}
	})
}

func TestDetectLXCCTID(t *testing.T) {
	t.Run("FromCgroupLXC", func(t *testing.T) {
		t.Cleanup(resetSystemFns)
		readFileFn = func(path string) ([]byte, error) {
			if path == "/proc/1/cgroup" {
				return []byte("0::/lxc/123"), nil
			}
			return nil, errors.New("missing")
		}
		hostnameFn = func() (string, error) { return "999", nil }

		if got := DetectLXCCTID(); got != "123" {
			t.Fatalf("expected CTID 123, got %q", got)
		}
	})

	t.Run("FromCgroupPayload", func(t *testing.T) {
		t.Cleanup(resetSystemFns)
		readFileFn = func(path string) ([]byte, error) {
			if path == "/proc/1/cgroup" {
				return []byte("0::/lxc.payload.456"), nil
			}
			return nil, errors.New("missing")
		}
		hostnameFn = func() (string, error) { return "999", nil }

		if got := DetectLXCCTID(); got != "456" {
			t.Fatalf("expected CTID 456, got %q", got)
		}
	})

	t.Run("FromMachineLXC", func(t *testing.T) {
		t.Cleanup(resetSystemFns)
		readFileFn = func(path string) ([]byte, error) {
			if path == "/proc/1/cgroup" {
				return []byte("0::/machine.slice/machine-lxc-789"), nil
			}
			return nil, errors.New("missing")
		}
		hostnameFn = func() (string, error) { return "999", nil }

		if got := DetectLXCCTID(); got != "789" {
			t.Fatalf("expected CTID 789, got %q", got)
		}
	})

	t.Run("FromHostname", func(t *testing.T) {
		t.Cleanup(resetSystemFns)
		readFileFn = func(string) ([]byte, error) { return []byte("0::/user.slice"), nil }
		hostnameFn = func() (string, error) { return "321", nil }

		if got := DetectLXCCTID(); got != "321" {
			t.Fatalf("expected CTID 321, got %q", got)
		}
	})

	t.Run("NoMatch", func(t *testing.T) {
		t.Cleanup(resetSystemFns)
		readFileFn = func(string) ([]byte, error) { return []byte("0::/user.slice"), nil }
		hostnameFn = func() (string, error) { return "host", nil }

		if got := DetectLXCCTID(); got != "" {
			t.Fatalf("expected empty CTID, got %q", got)
		}
	})
}
