package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectAppRoot(t *testing.T) {
	// 1. Test PULSE_APP_ROOT
	expectedRoot := "/custom/root"
	t.Setenv("PULSE_APP_ROOT", expectedRoot)

	if root := detectAppRoot(); root != expectedRoot {
		t.Errorf("Expected root %q from env, got %q", expectedRoot, root)
	}

	// 2. Test fallback (unset env)
	os.Unsetenv("PULSE_APP_ROOT")

	// Determining expected fallback is tricky because "go test" compiles a binary to a temp location.
	// detectAppRoot logic handles "go run" temp dirs by falling back to CWD using os.Getwd().
	// So we expect it to return CWD.

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get cwd: %v", err)
	}

	// Depending on how "go test" is run, os.Executable might be in a temp dir.
	// If detectAppRoot detects temp dir, it returns cwd.
	// If it doesn't detect temp dir, it returns dirname(executable).

	root := detectAppRoot()

	// Verify it returns a valid directory
	if stat, err := os.Stat(root); err != nil || !stat.IsDir() {
		t.Errorf("detectAppRoot returned invalid directory: %q", root)
	}

	// We can't strictly assert it equals CWD because in some CI envs the test binary location might not trigger the temp dir check.
	// But mostly it should be CWD or the dir of the binary.

	t.Logf("Detected root: %s", root)
	t.Logf("CWD: %s", cwd)

	// Just ensure it's not empty
	if root == "" {
		t.Error("detectAppRoot returned empty string")
	}
}

func TestDetectAppRoot_Scenarios(t *testing.T) {
	// Restore mocks after tests
	originalOsExecutable := osExecutable
	originalOsGetwd := osGetwd
	defer func() {
		osExecutable = originalOsExecutable
		osGetwd = originalOsGetwd
	}()

	tests := []struct {
		name           string
		envRoot        string
		mockExec       string
		mockExecErr    error
		mockGetwd      string
		mockGetwdErr   error
		expectedResult string
	}{
		{
			name:           "Env var set",
			envRoot:        "/custom/root",
			expectedResult: "/custom/root",
		},
		{
			name:           "Executable normal",
			mockExec:       "/opt/pulse/pulse-server",
			expectedResult: "/opt/pulse",
		},
		{
			name:           "Executable in temp (go run)",
			mockExec:       os.TempDir() + "/go-build123/exe",
			mockGetwd:      "/home/user/pulse",
			expectedResult: "/home/user/pulse",
		},
		{
			name:           "Executable error, use cwd",
			mockExecErr:    os.ErrNotExist,
			mockGetwd:      "/home/user/pulse",
			expectedResult: "/home/user/pulse",
		},
		{
			name:           "Executable in temp, getwd error",
			mockExec:       os.TempDir() + "/go-build123/exe",
			mockGetwdErr:   os.ErrPermission,
			expectedResult: filepath.Join(os.TempDir(), "go-build123"), // Falls back to exe dir
		},
		{
			name:           "Executable error, getwd error",
			mockExecErr:    os.ErrNotExist,
			mockGetwdErr:   os.ErrPermission,
			expectedResult: ".",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envRoot != "" {
				t.Setenv("PULSE_APP_ROOT", tt.envRoot)
			} else {
				os.Unsetenv("PULSE_APP_ROOT")
			}

			osExecutable = func() (string, error) {
				return tt.mockExec, tt.mockExecErr
			}
			osGetwd = func() (string, error) {
				return tt.mockGetwd, tt.mockGetwdErr
			}

			result := detectAppRoot()
			if result != tt.expectedResult {
				t.Errorf("Expected %q, got %q", tt.expectedResult, result)
			}
		})
	}
}
