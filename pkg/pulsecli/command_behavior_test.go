package pulsecli

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type testCLI struct {
	t                 *testing.T
	exportFile        string
	importFile        string
	passphrase        string
	forceImport       bool
	readPassword      func(int) ([]byte, error)
	exitCode          int
	mockEnvDefaultDir string
	mockStat          func(string) (os.FileInfo, error)
}

func newTestCLI(t *testing.T) *testCLI {
	t.Helper()
	return &testCLI{
		t:                 t,
		mockEnvDefaultDir: "/opt/pulse",
		mockStat:          os.Stat,
	}
}

func (tc *testCLI) execute(args ...string) (string, error) {
	tc.t.Helper()

	cmd := NewRootCommand(Options{
		Use:       "pulse",
		Short:     "Pulse",
		Long:      "Pulse",
		Version:   "1.2.3",
		Config:    tc.configDeps(),
		Bootstrap: tc.bootstrapDeps(),
		Mock:      tc.mockDeps(),
	})
	cmd.SetArgs(args)

	var err error
	output := captureOutput(tc.t, func() {
		err = cmd.Execute()
	})
	return output, err
}

func (tc *testCLI) configDeps() *ConfigDeps {
	return &ConfigDeps{
		ExportFile:  &tc.exportFile,
		ImportFile:  &tc.importFile,
		Passphrase:  &tc.passphrase,
		ForceImport: &tc.forceImport,
		ReadPassword: func(fd int) ([]byte, error) {
			if tc.readPassword != nil {
				return tc.readPassword(fd)
			}
			return nil, fmt.Errorf("read password not configured")
		},
	}
}

func (tc *testCLI) bootstrapDeps() *BootstrapDeps {
	return &BootstrapDeps{
		Exit: func(code int) {
			tc.exitCode = code
		},
	}
}

func (tc *testCLI) mockDeps() *MockDeps {
	return &MockDeps{
		Exit: func(code int) {
			tc.exitCode = code
		},
		DefaultEnvDir: func() string {
			return tc.mockEnvDefaultDir
		},
		Stat: tc.mockStat,
	}
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

func TestConfigInfoCommand(t *testing.T) {
	tc := newTestCLI(t)

	output, err := tc.execute("config", "info")
	if err != nil {
		t.Fatalf("execute config info: %v", err)
	}
	if output == "" || !containsAll(output, "Pulse Configuration Information", "Configuration is managed through the web UI") {
		t.Fatalf("config info output = %q", output)
	}
}

func TestConfigExportAndImportCommands(t *testing.T) {
	tc := newTestCLI(t)
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)
	t.Setenv("PULSE_PASSPHRASE", "testpass")
	createTestEncryptionKey(t, tempDir)

	outputFile := filepath.Join(tempDir, "export.enc")
	_, err := tc.execute("config", "export", "-o", outputFile)
	if err != nil {
		t.Fatalf("execute config export: %v", err)
	}
	if _, err := os.Stat(outputFile); err != nil {
		t.Fatalf("stat export file: %v", err)
	}

	tc.exportFile = ""
	output, err := tc.execute("config", "export")
	if err != nil {
		t.Fatalf("execute config export stdout: %v", err)
	}
	if output == "" {
		t.Fatal("expected exported configuration on stdout")
	}

	tc.importFile = outputFile
	tc.forceImport = true
	_, err = tc.execute("config", "import", "-i", outputFile, "--force")
	if err != nil {
		t.Fatalf("execute config import: %v", err)
	}
}

func TestConfigAutoImportCommandErrors(t *testing.T) {
	tc := newTestCLI(t)
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)
	t.Setenv("PULSE_INIT_CONFIG_PASSPHRASE", "testpass")
	createTestEncryptionKey(t, tempDir)

	t.Setenv("PULSE_INIT_CONFIG_DATA", "testdata")
	_, err := tc.execute("config", "auto-import")
	if err == nil {
		t.Fatal("expected auto-import with invalid inline data to fail")
	}

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "url-test-data")
	}))
	defer testServer.Close()

	t.Setenv("PULSE_INIT_CONFIG_URL", testServer.URL)
	t.Setenv("PULSE_INIT_CONFIG_DATA", "")
	_, err = tc.execute("config", "auto-import")
	if err == nil {
		t.Fatal("expected auto-import with invalid URL data to fail")
	}

	t.Setenv("PULSE_INIT_CONFIG_URL", "ftp://host/file")
	_, err = tc.execute("config", "auto-import")
	if err == nil || !containsAll(err.Error(), "unsupported URL scheme") {
		t.Fatalf("expected unsupported URL scheme error, got %v", err)
	}
}

func TestConfigCommandErrorPaths(t *testing.T) {
	tc := newTestCLI(t)
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	tc.readPassword = func(fd int) ([]byte, error) {
		return nil, fmt.Errorf("read error")
	}
	_, err := tc.execute("config", "export")
	if err == nil || !containsAll(err.Error(), "passphrase is required") {
		t.Fatalf("expected export passphrase error, got %v", err)
	}

	createTestEncryptionKey(t, tempDir)
	outputDir := filepath.Join(tempDir, "is_dir")
	if err := os.Mkdir(outputDir, 0o755); err != nil {
		t.Fatalf("mkdir output dir: %v", err)
	}

	tc.passphrase = "test"
	_, err = tc.execute("config", "export", "--passphrase", "test", "-o", outputDir)
	if err == nil || !containsAll(err.Error(), "failed to write export file") {
		t.Fatalf("expected export write error, got %v", err)
	}

	importPath := filepath.Join(tempDir, "import.enc")
	if err := os.WriteFile(importPath, []byte("data"), 0o644); err != nil {
		t.Fatalf("write import file: %v", err)
	}

	tc.importFile = importPath
	tc.passphrase = ""
	tc.readPassword = func(fd int) ([]byte, error) {
		return nil, fmt.Errorf("read error")
	}
	_, err = tc.execute("config", "import", "-i", importPath)
	if err == nil || !containsAll(err.Error(), "passphrase is required") {
		t.Fatalf("expected import passphrase error, got %v", err)
	}

	tc.readPassword = func(fd int) ([]byte, error) {
		return []byte("pass"), nil
	}
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdin = r
	_, _ = w.Write([]byte("no\n"))
	_ = w.Close()
	output, err := tc.execute("config", "import", "-i", importPath)
	os.Stdin = oldStdin
	if err != nil {
		t.Fatalf("expected cancelled import to return nil, got %v", err)
	}
	if !containsAll(output, "Import cancelled") {
		t.Fatalf("expected cancelled import output, got %q", output)
	}

	tc.passphrase = "pass"
	tc.forceImport = true
	_, err = tc.execute("config", "import", "-i", importPath, "--force", "--passphrase", "pass")
	if err == nil || !containsAll(err.Error(), "failed to import configuration") {
		t.Fatalf("expected import failure for invalid data, got %v", err)
	}
}

func TestBootstrapTokenCommandAndErrors(t *testing.T) {
	tc := newTestCLI(t)
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	tokenFile := filepath.Join(tempDir, ".bootstrap_token")
	if err := os.WriteFile(tokenFile, []byte("test-token"), 0o644); err != nil {
		t.Fatalf("write bootstrap token: %v", err)
	}

	output, err := tc.execute("bootstrap-token")
	if err != nil {
		t.Fatalf("execute bootstrap-token: %v", err)
	}
	if !containsAll(output, "test-token", tokenFile) {
		t.Fatalf("bootstrap-token output = %q", output)
	}

	if err := os.Remove(tokenFile); err != nil {
		t.Fatalf("remove bootstrap token: %v", err)
	}
	tc.exitCode = 0
	output, err = tc.execute("bootstrap-token")
	if err != nil {
		t.Fatalf("missing bootstrap token should not return cobra error: %v", err)
	}
	if tc.exitCode != 1 || !containsAll(output, "NO BOOTSTRAP TOKEN FOUND") {
		t.Fatalf("missing token output = %q exit=%d", output, tc.exitCode)
	}
}

func TestMockCommands(t *testing.T) {
	tc := newTestCLI(t)
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	output, err := tc.execute("mock", "status")
	if err != nil {
		t.Fatalf("execute mock status: %v", err)
	}
	if !containsAll(output, "Mock mode: DISABLED") {
		t.Fatalf("mock status output = %q", output)
	}

	envPath := filepath.Join(tempDir, "mock.env")
	if err := os.WriteFile(envPath, []byte("PULSE_MOCK_MODE=true\nEXTRA_KEY=value\n"), 0o644); err != nil {
		t.Fatalf("write mock.env: %v", err)
	}

	output, err = tc.execute("mock", "status")
	if err != nil {
		t.Fatalf("execute mock status enabled: %v", err)
	}
	if !containsAll(output, "Mock mode: ENABLED") {
		t.Fatalf("mock enabled output = %q", output)
	}

	output, err = tc.execute("mock", "enable")
	if err != nil {
		t.Fatalf("execute mock enable: %v", err)
	}
	if !containsAll(output, "Mock mode enabled") {
		t.Fatalf("mock enable output = %q", output)
	}
	content, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("read mock.env: %v", err)
	}
	if !containsAll(string(content), "EXTRA_KEY=value") {
		t.Fatalf("mock.env content = %q", string(content))
	}

	output, err = tc.execute("mock", "disable")
	if err != nil {
		t.Fatalf("execute mock disable: %v", err)
	}
	if !containsAll(output, "Mock mode disabled") {
		t.Fatalf("mock disable output = %q", output)
	}
}

func TestMockCommandErrorPaths(t *testing.T) {
	tc := newTestCLI(t)
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	if err := os.Mkdir(filepath.Join(tempDir, "mock.env"), 0o755); err != nil {
		t.Fatalf("mkdir mock.env: %v", err)
	}

	_, err := tc.execute("mock", "enable")
	if err != nil {
		t.Fatalf("mock enable should exit via deps, not cobra error: %v", err)
	}
	if tc.exitCode != 1 {
		t.Fatalf("mock enable exit code = %d, want 1", tc.exitCode)
	}

	tc.exitCode = 0
	_, err = tc.execute("mock", "disable")
	if err != nil {
		t.Fatalf("mock disable should exit via deps, not cobra error: %v", err)
	}
	if tc.exitCode != 1 {
		t.Fatalf("mock disable exit code = %d, want 1", tc.exitCode)
	}
}

func containsAll(s string, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(s, part) {
			return false
		}
	}
	return true
}
