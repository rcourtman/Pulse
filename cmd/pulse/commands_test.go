package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/pkg/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestEncryptionKey creates a valid base64-encoded encryption key in the temp directory.
// Required before creating .enc files to avoid crypto initialization failures.
func createTestEncryptionKey(t *testing.T, dir string) {
	t.Helper()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	encoded := base64.StdEncoding.EncodeToString(key)
	if err := os.WriteFile(filepath.Join(dir, ".encryption.key"), []byte(encoded), 0600); err != nil {
		t.Fatalf("failed to create test encryption key: %v", err)
	}
}

func TestVersionCmd(t *testing.T) {
	oldVersion := Version
	oldBuildTime := BuildTime
	oldGitCommit := GitCommit
	defer func() {
		Version = oldVersion
		BuildTime = oldBuildTime
		GitCommit = oldGitCommit
	}()

	// Test 1: Full version info
	Version = "1.2.3"
	BuildTime = "2023-01-01"
	GitCommit = "abcdef"

	output := captureOutput(func() {
		rootCmd.SetArgs([]string{"version"})
		rootCmd.Execute()
	})

	assert.Contains(t, output, "Pulse 1.2.3")
	assert.Contains(t, output, "Built: 2023-01-01")
	assert.Contains(t, output, "Commit: abcdef")

	// Test 2: Only version
	BuildTime = "unknown"
	GitCommit = "unknown"
	output = captureOutput(func() {
		rootCmd.SetArgs([]string{"version"})
		rootCmd.Execute()
	})
	assert.Contains(t, output, "Pulse 1.2.3")
	assert.NotContains(t, output, "Built:")
	assert.NotContains(t, output, "Commit:")
}

func TestConfigInfoCmd(t *testing.T) {
	output := captureOutput(func() {
		rootCmd.SetArgs([]string{"config", "info"})
		rootCmd.Execute()
	})

	assert.Contains(t, output, "Pulse Configuration Information")
	assert.Contains(t, output, "Configuration is managed through the web UI")
}

func TestConfigExportCmd(t *testing.T) {
	resetFlags()

	tempDir := t.TempDir()
	os.Setenv("PULSE_DATA_DIR", tempDir)
	defer os.Unsetenv("PULSE_DATA_DIR")
	createTestEncryptionKey(t, tempDir)

	// Set PULSE_PASSPHRASE for non-interactive test
	os.Setenv("PULSE_PASSPHRASE", "testpass")
	defer os.Unsetenv("PULSE_PASSPHRASE")

	outputFile := filepath.Join(tempDir, "export.enc")

	rootCmd.SetArgs([]string{"config", "export", "-o", outputFile})
	err := rootCmd.Execute()
	assert.NoError(t, err)

	_, err = os.Stat(outputFile)
	assert.NoError(t, err)

	// Test without output file (prints to stdout)
	output := captureOutput(func() {
		exportFile = "" // Reset again
		rootCmd.SetArgs([]string{"config", "export"})
		rootCmd.Execute()
	})
	assert.NotEmpty(t, output)
}

func TestConfigImportCmd(t *testing.T) {
	resetFlags()

	tempDir := t.TempDir()
	os.Setenv("PULSE_DATA_DIR", tempDir)
	defer os.Unsetenv("PULSE_DATA_DIR")
	createTestEncryptionKey(t, tempDir)

	os.Setenv("PULSE_PASSPHRASE", "testpass")
	defer os.Unsetenv("PULSE_PASSPHRASE")

	// First export some config to have something to import
	exportFile = filepath.Join(tempDir, "export.enc")
	rootCmd.SetArgs([]string{"config", "export", "-o", exportFile})
	rootCmd.Execute()

	// Now import it
	importFile = exportFile
	forceImport = true
	rootCmd.SetArgs([]string{"config", "import", "-i", exportFile, "--force"})
	err := rootCmd.Execute()
	assert.NoError(t, err)

	// Test missing input file error
	importFile = "" // Reset to trigger error
	rootCmd.SetArgs([]string{"config", "import", "--force"})
	err = rootCmd.Execute()
	assert.Error(t, err)
	if err != nil {
		assert.Contains(t, err.Error(), "import file is required")
	}
}

func TestBootstrapTokenCmd(t *testing.T) {
	tempDir := t.TempDir()
	os.Setenv("PULSE_DATA_DIR", tempDir)
	defer os.Unsetenv("PULSE_DATA_DIR")

	tokenFile := filepath.Join(tempDir, ".bootstrap_token")
	err := os.WriteFile(tokenFile, []byte("test-token"), 0644)
	assert.NoError(t, err)

	output := captureOutput(func() {
		rootCmd.SetArgs([]string{"bootstrap-token"})
		rootCmd.Execute()
	})

	assert.Contains(t, output, "test-token")
	assert.Contains(t, output, tokenFile)
}

func TestBootstrapTokenEdgeCases(t *testing.T) {
	tempDir := t.TempDir()
	os.Setenv("PULSE_DATA_DIR", tempDir)
	defer os.Unsetenv("PULSE_DATA_DIR")

	oldExit := osExit
	defer func() { osExit = oldExit }()

	exitCode := 0
	osExit = func(code int) { exitCode = code }

	// 1. Token file not found
	captureOutput(func() {
		showBootstrapToken()
	})
	assert.Equal(t, 1, exitCode)

	// 2. Token file empty
	tokenFile := filepath.Join(tempDir, ".bootstrap_token")
	os.WriteFile(tokenFile, []byte(""), 0644)
	captureOutput(func() {
		showBootstrapToken()
	})
	assert.Equal(t, 1, exitCode)

	// 3. Other read error (e.g. is a directory)
	dirToken := filepath.Join(tempDir, "is_a_dir")
	os.Mkdir(dirToken, 0755)
	os.Setenv("PULSE_DATA_DIR", tempDir)
	// We need to trick it to use this path
	// showBootstrapToken uses filepath.Join(dataPath, ".bootstrap_token")
	// So we make .bootstrap_token a directory
	os.Remove(tokenFile)
	os.Mkdir(tokenFile, 0755)
	captureOutput(func() {
		showBootstrapToken()
	})
	assert.Equal(t, 1, exitCode)
	os.RemoveAll(tokenFile)

	// 4. Test data paths
	os.Setenv("PULSE_DOCKER", "true")
	os.Unsetenv("PULSE_DATA_DIR")
	captureOutput(func() {
		showBootstrapToken()
	})
	assert.Equal(t, 1, exitCode)
	os.Unsetenv("PULSE_DOCKER")

	// 5. Test default data path (/etc/pulse)
	os.Unsetenv("PULSE_DATA_DIR")
	captureOutput(func() {
		showBootstrapToken()
	})
	assert.Equal(t, 1, exitCode)
}

func TestMockCmds(t *testing.T) {
	tempDir := t.TempDir()
	os.Setenv("PULSE_DATA_DIR", tempDir)
	defer os.Unsetenv("PULSE_DATA_DIR")

	// Test status (disabled initially)
	output := captureOutput(func() {
		rootCmd.SetArgs([]string{"mock", "status"})
		rootCmd.Execute()
	})
	assert.Contains(t, output, "Mock mode: DISABLED")

	// Create a mock.env with extra keys
	envPath := filepath.Join(tempDir, "mock.env")
	os.WriteFile(envPath, []byte("PULSE_MOCK_MODE=true\nEXTRA_KEY=value\n"), 0644)

	// Test status (enabled)
	output = captureOutput(func() {
		rootCmd.SetArgs([]string{"mock", "status"})
		rootCmd.Execute()
	})
	assert.Contains(t, output, "Mock mode: ENABLED")

	// Test enable (should preserve EXTRA_KEY)
	output = captureOutput(func() {
		rootCmd.SetArgs([]string{"mock", "enable"})
		rootCmd.Execute()
	})
	assert.Contains(t, output, "Mock mode enabled")
	content, _ := os.ReadFile(envPath)
	assert.Contains(t, string(content), "EXTRA_KEY=value")

	// Test disable
	output = captureOutput(func() {
		rootCmd.SetArgs([]string{"mock", "disable"})
		rootCmd.Execute()
	})
	assert.Contains(t, output, "Mock mode disabled")

	// Test getMockEnvPath branch (no env var)
	os.Unsetenv("PULSE_DATA_DIR")
	path := getMockEnvPath()
	assert.NotEmpty(t, path)

	// Test getMockEnvPath branch (fallback when PULSE_DATA_DIR is empty and mock.env exists in default dir)
	oldDefault := mockEnvDefaultDir
	oldStat := mockEnvStat
	t.Cleanup(func() {
		mockEnvDefaultDir = oldDefault
		mockEnvStat = oldStat
	})
	mockEnvDefaultDir = t.TempDir()
	mockEnvStat = os.Stat
	require.NoError(t, os.WriteFile(filepath.Join(mockEnvDefaultDir, "mock.env"), []byte("PULSE_MOCK_MODE=false\n"), 0644))

	os.Unsetenv("PULSE_DATA_DIR")
	path = getMockEnvPath()
	assert.Equal(t, filepath.Join(mockEnvDefaultDir, "mock.env"), path)
}

func TestGetMockEnvPath_DefaultFallback(t *testing.T) {
	oldDefault := mockEnvDefaultDir
	oldStat := mockEnvStat
	t.Cleanup(func() {
		mockEnvDefaultDir = oldDefault
		mockEnvStat = oldStat
	})

	// Cover fallback: PULSE_DATA_DIR empty and mock.env does not exist in default dir.
	mockEnvDefaultDir = filepath.Join(t.TempDir(), "does-not-exist")
	mockEnvStat = os.Stat
	os.Unsetenv("PULSE_DATA_DIR")

	path := getMockEnvPath()
	assert.Equal(t, filepath.Join(mockEnvDefaultDir, "mock.env"), path)
}

func TestMockEnable_Error(t *testing.T) {
	resetFlags()
	// Force setMockMode to fail by using a read-only directory
	tempDir := t.TempDir()
	os.Setenv("PULSE_DATA_DIR", tempDir)
	defer os.Unsetenv("PULSE_DATA_DIR")

	// Make directory read-only so file creation fails?
	// Or make the mock.env a directory?
	os.Mkdir(filepath.Join(tempDir, "mock.env"), 0755)

	oldExit := osExit
	defer func() { osExit = oldExit }()
	exitCode := 0
	osExit = func(code int) { exitCode = code }

	captureOutput(func() {
		rootCmd.SetArgs([]string{"mock", "enable"})
		rootCmd.Execute()
	})
	assert.Equal(t, 1, exitCode)
}

func TestMockDisable_Error(t *testing.T) {
	resetFlags()
	tempDir := t.TempDir()
	os.Setenv("PULSE_DATA_DIR", tempDir)
	defer os.Unsetenv("PULSE_DATA_DIR")

	// Make mock.env a directory
	os.Mkdir(filepath.Join(tempDir, "mock.env"), 0755)

	oldExit := osExit
	defer func() { osExit = oldExit }()
	exitCode := 0
	osExit = func(code int) { exitCode = code }

	captureOutput(func() {
		rootCmd.SetArgs([]string{"mock", "disable"})
		rootCmd.Execute()
	})
	assert.Equal(t, 1, exitCode)
}

func TestGetPassphrase(t *testing.T) {
	oldRead := readPassword
	defer func() { readPassword = oldRead }()

	// 1. Flag
	passphrase = "flag-pass"
	assert.Equal(t, "flag-pass", getPassphrase("test", false))
	passphrase = ""

	// 2. Interactive
	os.Unsetenv("PULSE_PASSPHRASE")
	readPassword = func(fd int) ([]byte, error) {
		return []byte("inter-pass"), nil
	}
	assert.Equal(t, "inter-pass", getPassphrase("test", false))

	// 3. Confirmation match
	callCount := 0
	readPassword = func(fd int) ([]byte, error) {
		callCount++
		return []byte("match"), nil
	}
	assert.Equal(t, "match", getPassphrase("test", true))
	assert.Equal(t, 2, callCount)

	// 4. Confirmation mismatch
	callCount = 0
	readPassword = func(fd int) ([]byte, error) {
		callCount++
		if callCount == 1 {
			return []byte("pass1"), nil
		}
		return []byte("pass2"), nil
	}
	assert.Equal(t, "", getPassphrase("test", true))

	// 5. Error
	readPassword = func(fd int) ([]byte, error) {
		return nil, fmt.Errorf("error")
	}
	assert.Equal(t, "", getPassphrase("test", false))

	// 6. Error in confirm
	callCount = 0
	readPassword = func(fd int) ([]byte, error) {
		callCount++
		if callCount == 1 {
			return []byte("pass1"), nil
		}
		return nil, fmt.Errorf("error")
	}
	assert.Equal(t, "", getPassphrase("test", true))
}

func TestConfigAutoImportCmd(t *testing.T) {
	tempDir := t.TempDir()
	os.Setenv("PULSE_DATA_DIR", tempDir)
	defer os.Unsetenv("PULSE_DATA_DIR")
	createTestEncryptionKey(t, tempDir)

	os.Setenv("PULSE_INIT_CONFIG_PASSPHRASE", "testpass")
	defer os.Unsetenv("PULSE_INIT_CONFIG_PASSPHRASE")

	// Test with data
	os.Setenv("PULSE_INIT_CONFIG_DATA", "testdata")
	defer os.Unsetenv("PULSE_INIT_CONFIG_DATA")

	// This might fail because 'testdata' is not a valid encrypted config,
	// but we want to see it try. ImportConfig will probably fail.
	rootCmd.SetArgs([]string{"config", "auto-import"})
	err := rootCmd.Execute()
	// It should fail because "testdata" is not valid encrypted config
	assert.Error(t, err)

	// Test with URL
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "url-test-data")
	}))
	defer server.Close()

	os.Setenv("PULSE_INIT_CONFIG_URL", server.URL)
	defer os.Unsetenv("PULSE_INIT_CONFIG_URL")
	os.Unsetenv("PULSE_INIT_CONFIG_DATA")

	rootCmd.SetArgs([]string{"config", "auto-import"})
	err = rootCmd.Execute()
	assert.Error(t, err) // Still invalid data, but covered the URL path
}

func TestConfigAutoImport_Errors(t *testing.T) {
	tempDir := t.TempDir()
	os.Setenv("PULSE_DATA_DIR", tempDir)
	defer os.Unsetenv("PULSE_DATA_DIR")

	os.Setenv("PULSE_INIT_CONFIG_PASSPHRASE", "testpass")
	defer os.Unsetenv("PULSE_INIT_CONFIG_PASSPHRASE")

	// 1. Invalid URL scheme
	os.Setenv("PULSE_INIT_CONFIG_URL", "ftp://host/file")
	rootCmd.SetArgs([]string{"config", "auto-import"})
	err := rootCmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported URL scheme")

	// 2. Invalid URL
	os.Setenv("PULSE_INIT_CONFIG_URL", "http:// invalid")
	err = rootCmd.Execute()
	assert.Error(t, err)

	// 3. 404 from URL
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	os.Setenv("PULSE_INIT_CONFIG_URL", server.URL)
	err = rootCmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to fetch configuration")

	// 4. Empty body from URL
	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server2.Close()
	os.Setenv("PULSE_INIT_CONFIG_URL", server2.URL)
	err = rootCmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "configuration response from URL was empty")
}

func TestNormalizeImportPayload(t *testing.T) {
	// Empty case
	_, err := server.NormalizeImportPayload([]byte("  "))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "configuration payload is empty")

	// Base64 case (where decoded doesn't look like base64)
	// base64("!!") = "ISE="
	s, err := server.NormalizeImportPayload([]byte(" ISE= "))
	assert.NoError(t, err)
	assert.Equal(t, "ISE=", s)

	// Base64-of-Base64 case (unwraps)
	// base64("test") = "dGVzdA=="
	// test also looks like base64 (4 chars, alphanumeric)
	s, err = server.NormalizeImportPayload([]byte(" dGVzdA== "))
	assert.NoError(t, err)
	assert.Equal(t, "test", s)

	// Plain case (not base64)
	s, err = server.NormalizeImportPayload([]byte("!!"))
	assert.NoError(t, err)
	// Should be base64 encoded
	assert.Equal(t, base64.StdEncoding.EncodeToString([]byte("!!")), s)
}

func TestConfigExport_ErrorPaths(t *testing.T) {
	resetFlags()
	tempDir := t.TempDir()
	os.Setenv("PULSE_DATA_DIR", tempDir)
	defer os.Unsetenv("PULSE_DATA_DIR")

	// 1. Passphrase required error
	// Set passphrase to empty by making getPassphrase return ""
	// getPassphrase returns "" if terminal read fails
	oldRead := readPassword
	readPassword = func(fd int) ([]byte, error) { return nil, fmt.Errorf("read error") }
	defer func() { readPassword = oldRead }()

	rootCmd.SetArgs([]string{"config", "export"})
	err := rootCmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "passphrase is required")
}

func TestConfigExport_WriteError(t *testing.T) {
	resetFlags()
	tempDir := t.TempDir()
	os.Setenv("PULSE_DATA_DIR", tempDir)
	defer os.Unsetenv("PULSE_DATA_DIR")

	// Create a directory where the output file should be, to cause write error
	outputFile := filepath.Join(tempDir, "is_dir")
	os.Mkdir(outputFile, 0755)

	rootCmd.SetArgs([]string{"config", "export", "--passphrase", "test", "-o", outputFile})
	err := rootCmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to write export file")
}

func TestConfigImport_Errors(t *testing.T) {
	resetFlags()
	resetReadPassword := readPassword
	defer func() { readPassword = resetReadPassword }()

	tempDir := t.TempDir()
	os.Setenv("PULSE_DATA_DIR", tempDir)
	defer os.Unsetenv("PULSE_DATA_DIR")

	// Create dummy import file
	importFile := filepath.Join(tempDir, "import.enc")
	os.WriteFile(importFile, []byte("data"), 0644)

	// 1. Passphrase required error
	readPassword = func(fd int) ([]byte, error) { return nil, fmt.Errorf("read error") }
	rootCmd.SetArgs([]string{"config", "import", "-i", importFile})
	err := rootCmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "passphrase is required")

	// 2. Import cancelled
	readPassword = func(fd int) ([]byte, error) { return []byte("pass"), nil }

	// Mock stdin for confirmation "no"
	oldStdin := os.Stdin
	r, w, _ := os.Pipe()
	os.Stdin = r
	w.Write([]byte("no\n"))
	w.Close()

	rootCmd.SetArgs([]string{"config", "import", "-i", importFile})
	captureOutput(func() {
		err = rootCmd.Execute()
	})
	assert.NoError(t, err)

	os.Stdin = oldStdin

	// 3. Failed to import configuration (invalid data)
	// We need to force import to skip confirmation
	rootCmd.SetArgs([]string{"config", "import", "-i", importFile, "--force", "--passphrase", "pass"})
	err = rootCmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to import configuration")
}

func TestCaptureOutput(t *testing.T) {
	output := captureOutput(func() {
		fmt.Print("hello")
		fmt.Fprint(os.Stderr, "world")
	})
	assert.Equal(t, "helloworld", output)
}

// Helper to capture stdout and stderr
func captureOutput(f func()) string {
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stdout = w
	os.Stderr = w

	f()

	w.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

func resetFlags() {
	exportFile = ""
	importFile = ""
	passphrase = ""
	forceImport = false
}
