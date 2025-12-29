package knownhosts

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func resetKnownHostsFns() {
	mkdirAllFn = defaultMkdirAllFn
	statFn = defaultStatFn
	openFileFn = defaultOpenFileFn
	openFn = defaultOpenFn
	appendOpenFileFn = defaultAppendOpenFileFn
	keyscanCmdRunner = defaultKeyscanCmdRunner
}

func TestEnsureCreatesFileAndCaches(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "known_hosts")

	var calls int
	keyscan := func(ctx context.Context, host string, port int, timeout time.Duration) ([]byte, error) {
		calls++
		return []byte(host + " ssh-ed25519 AAAA"), nil
	}

	mgr, err := NewManager(path, WithKeyscanFunc(keyscan))
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	ctx := context.Background()
	if err := mgr.Ensure(ctx, "example.com"); err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("known_hosts not created: %v", err)
	}

	if err := mgr.Ensure(ctx, "example.com"); err != nil {
		t.Fatalf("Ensure second call: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected keyscan once, got %d", calls)
	}
}

func TestEnsureUsesSanitizedOutput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "known_hosts")

	keyscan := func(ctx context.Context, host string, port int, timeout time.Duration) ([]byte, error) {
		return []byte(`# comment
example.com ssh-ed25519 AAAA
example.com,192.0.2.10 ssh-rsa BBBB
other.com ssh-ed25519 CCCC
`), nil
	}

	mgr, err := NewManager(path, WithKeyscanFunc(keyscan))
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	if err := mgr.Ensure(context.Background(), "example.com"); err != nil {
		t.Fatalf("Ensure: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if want := "example.com ssh-ed25519 AAAA\nexample.com ssh-rsa BBBB\n"; string(data) != want {
		t.Fatalf("unexpected known_hosts contents\nwant:\n%s\ngot:\n%s", want, data)
	}
}

func TestEnsureReturnsErrorWhenNoEntries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "known_hosts")

	mgr, err := NewManager(path, WithKeyscanFunc(func(ctx context.Context, host string, port int, timeout time.Duration) ([]byte, error) {
		return []byte("|1|hash|salt ssh-ed25519 AAAA\n"), nil
	}))
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	err = mgr.Ensure(context.Background(), "example.com")
	if !errors.Is(err, ErrNoHostKeys) {
		t.Fatalf("expected ErrNoHostKeys, got %v", err)
	}
}

func TestEnsureRespectsContextCancellation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "known_hosts")

	keyscan := func(ctx context.Context, host string, port int, timeout time.Duration) ([]byte, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}

	mgr, err := NewManager(path, WithKeyscanFunc(keyscan), WithTimeout(50*time.Millisecond))
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := mgr.Ensure(ctx, "example.com"); err == nil {
		t.Fatalf("expected context error, got nil")
	}
}

func TestNewManagerEmptyPath(t *testing.T) {
	if _, err := NewManager(""); err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestEnsureWithPortMissingHost(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "known_hosts")

	mgr, err := NewManager(path)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	if err := mgr.EnsureWithPort(context.Background(), "", 22); err == nil {
		t.Fatal("expected error for missing host")
	}
}

func TestEnsureWithPortDefaultsPort(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "known_hosts")

	var gotPort int
	keyscan := func(ctx context.Context, host string, port int, timeout time.Duration) ([]byte, error) {
		gotPort = port
		return []byte(host + " ssh-ed25519 AAAA"), nil
	}

	mgr, err := NewManager(path, WithKeyscanFunc(keyscan))
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	if err := mgr.EnsureWithPort(context.Background(), "example.com", 0); err != nil {
		t.Fatalf("EnsureWithPort: %v", err)
	}
	if gotPort != 22 {
		t.Fatalf("expected port 22, got %d", gotPort)
	}
}

func TestEnsureWithPortCustomPort(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "known_hosts")

	keyscan := func(ctx context.Context, host string, port int, timeout time.Duration) ([]byte, error) {
		return []byte("[example.com]:2222 ssh-ed25519 AAAA"), nil
	}

	mgr, err := NewManager(path, WithKeyscanFunc(keyscan))
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	if err := mgr.EnsureWithPort(context.Background(), "example.com", 2222); err != nil {
		t.Fatalf("EnsureWithPort: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if got := strings.TrimSpace(string(data)); got != "[example.com]:2222 ssh-ed25519 AAAA" {
		t.Fatalf("unexpected known_hosts contents: %s", got)
	}
}

func TestEnsureWithPortKeyscanError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "known_hosts")

	keyscan := func(ctx context.Context, host string, port int, timeout time.Duration) ([]byte, error) {
		return nil, errors.New("scan failed")
	}

	mgr, err := NewManager(path, WithKeyscanFunc(keyscan))
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	if err := mgr.EnsureWithPort(context.Background(), "example.com", 22); err == nil {
		t.Fatal("expected keyscan error")
	}
}

func TestEnsureWithEntriesMissingHost(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "known_hosts")

	mgr, err := NewManager(path)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	if err := mgr.EnsureWithEntries(context.Background(), "", 22, [][]byte{[]byte("example.com ssh-ed25519 AAAA")}); err == nil {
		t.Fatal("expected missing host error")
	}
}

func TestEnsureWithEntriesNoEntries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "known_hosts")

	mgr, err := NewManager(path)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	if err := mgr.EnsureWithEntries(context.Background(), "example.com", 22, nil); err == nil {
		t.Fatal("expected no entries error")
	}
}

func TestEnsureWithEntriesNormalizeError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "known_hosts")

	mgr, err := NewManager(path)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	if err := mgr.EnsureWithEntries(context.Background(), "example.com", 22, [][]byte{[]byte("invalid")}); err == nil {
		t.Fatal("expected normalize error")
	}
}

func TestEnsureWithEntriesEnsureKnownHostsFileError(t *testing.T) {
	t.Cleanup(resetKnownHostsFns)
	mkdirAllFn = func(string, os.FileMode) error {
		return errors.New("mkdir failed")
	}

	mgr, err := NewManager(filepath.Join(t.TempDir(), "known_hosts"))
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	if err := mgr.EnsureWithEntries(context.Background(), "example.com", 22, [][]byte{[]byte("example.com ssh-ed25519 AAAA")}); err == nil {
		t.Fatal("expected ensureKnownHostsFile error")
	}
}

func TestEnsureWithEntriesFindHostKeyLineError(t *testing.T) {
	t.Cleanup(resetKnownHostsFns)
	openFn = func(string) (*os.File, error) {
		return nil, errors.New("open failed")
	}

	mgr, err := NewManager(filepath.Join(t.TempDir(), "known_hosts"))
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	if err := mgr.EnsureWithEntries(context.Background(), "example.com", 22, [][]byte{[]byte("example.com ssh-ed25519 AAAA")}); err == nil {
		t.Fatal("expected open error")
	}
}

func TestHostCandidates(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"example.com", []string{"example.com"}},
		{"example.com:2222", []string{"example.com:2222", "example.com"}},
		{"[example.com]:2222", []string{"[example.com]:2222", "example.com"}},
	}

	for _, tt := range tests {
		got := hostCandidates(tt.input)
		if len(got) != len(tt.want) {
			t.Fatalf("hostCandidates(%q) len = %d, want %d", tt.input, len(got), len(tt.want))
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Fatalf("hostCandidates(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

func TestEnsureWithEntriesDetectsChange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "known_hosts")

	mgr, err := NewManager(path)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	entry := []byte("example.com ssh-ed25519 AAAA")
	if err := mgr.EnsureWithEntries(context.Background(), "example.com", 22, [][]byte{entry}); err != nil {
		t.Fatalf("EnsureWithEntries: %v", err)
	}

	// Same entry should be a no-op
	if err := mgr.EnsureWithEntries(context.Background(), "example.com", 22, [][]byte{entry}); err != nil {
		t.Fatalf("EnsureWithEntries repeat: %v", err)
	}

	// Different key should trigger change error
	changeEntry := []byte("example.com ssh-ed25519 BBBB")
	err = mgr.EnsureWithEntries(context.Background(), "example.com", 22, [][]byte{changeEntry})
	var changeErr *HostKeyChangeError
	if !errors.As(err, &changeErr) {
		t.Fatalf("expected HostKeyChangeError, got %v", err)
	}
}

func TestEnsureWithEntriesAppendsNewKeyTypes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "known_hosts")

	mgr, err := NewManager(path)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	ctx := context.Background()
	if err := mgr.EnsureWithEntries(ctx, "example.com", 22, [][]byte{[]byte("example.com ssh-ed25519 AAAA")}); err != nil {
		t.Fatalf("EnsureWithEntries ed25519: %v", err)
	}
	if err := mgr.EnsureWithEntries(ctx, "example.com", 22, [][]byte{[]byte("example.com ssh-rsa BBBB")}); err != nil {
		t.Fatalf("EnsureWithEntries rsa: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	got := string(data)
	if !strings.Contains(got, "ssh-ed25519 AAAA") || !strings.Contains(got, "ssh-rsa BBBB") {
		t.Fatalf("expected both key types, got %s", got)
	}
}

func TestHostKeyChangeErrorError(t *testing.T) {
	tests := []struct {
		host string
		want string
	}{
		{"example.com", "knownhosts: host key for example.com changed"},
		{"192.168.1.1", "knownhosts: host key for 192.168.1.1 changed"},
		{"[example.com]:2222", "knownhosts: host key for [example.com]:2222 changed"},
		{"", "knownhosts: host key for  changed"},
	}

	for _, tt := range tests {
		err := &HostKeyChangeError{Host: tt.host}
		if got := err.Error(); got != tt.want {
			t.Errorf("HostKeyChangeError{Host: %q}.Error() = %q, want %q", tt.host, got, tt.want)
		}
	}
}

func TestHostKeyChangeErrorUnwrap(t *testing.T) {
	err := &HostKeyChangeError{
		Host:     "example.com",
		Existing: "example.com ssh-ed25519 AAAA",
		Provided: "example.com ssh-ed25519 BBBB",
	}

	if !errors.Is(err, ErrHostKeyChanged) {
		t.Error("errors.Is(HostKeyChangeError, ErrHostKeyChanged) = false, want true")
	}

	unwrapped := err.Unwrap()
	if unwrapped != ErrHostKeyChanged {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, ErrHostKeyChanged)
	}
}

func TestManagerPath(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{"simple path", "/tmp/known_hosts"},
		{"nested path", "/home/user/.ssh/known_hosts"},
		{"relative-like path", "/opt/pulse/data/known_hosts"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr, err := NewManager(tt.path)
			if err != nil {
				t.Fatalf("NewManager(%q): %v", tt.path, err)
			}
			if got := mgr.Path(); got != tt.path {
				t.Errorf("Path() = %q, want %q", got, tt.path)
			}
		})
	}
}

func TestHostFieldMatches(t *testing.T) {
	tests := []struct {
		host  string
		field string
		want  bool
	}{
		// Exact matches
		{"example.com", "example.com", true},
		{"192.168.1.1", "192.168.1.1", true},

		// Case insensitive
		{"EXAMPLE.COM", "example.com", true},
		{"example.com", "EXAMPLE.COM", true},

		// Comma-separated hosts
		{"example.com", "example.com,192.168.1.1", true},
		{"192.168.1.1", "example.com,192.168.1.1", true},
		{"other.com", "example.com,192.168.1.1", false},

		// Bracketed hosts with ports
		{"[example.com]:2222", "[example.com]:2222", true},
		{"example.com", "[example.com]:2222", true},

		// Host:port format
		{"example.com:2222", "example.com:2222", true},
		{"example.com", "example.com:2222", true},

		// No match
		{"other.com", "example.com", false},
		{"example.org", "example.com", false},

		// Empty cases
		{"example.com", "", false},
		{"", "example.com", false},
		{"", "", false},
	}

	for _, tt := range tests {
		name := tt.host + "_" + tt.field
		t.Run(name, func(t *testing.T) {
			if got := HostFieldMatches(tt.host, tt.field); got != tt.want {
				t.Errorf("HostFieldMatches(%q, %q) = %v, want %v", tt.host, tt.field, got, tt.want)
			}
		})
	}
}

func TestEnsureKnownHostsFileMkdirError(t *testing.T) {
	t.Cleanup(resetKnownHostsFns)
	mkdirAllFn = func(string, os.FileMode) error {
		return errors.New("mkdir failed")
	}

	m := &manager{path: filepath.Join(t.TempDir(), "known_hosts")}
	if err := m.ensureKnownHostsFile(); err == nil {
		t.Fatal("expected mkdir error")
	}
}

func TestEnsureKnownHostsFileStatError(t *testing.T) {
	t.Cleanup(resetKnownHostsFns)
	statFn = func(string) (os.FileInfo, error) {
		return nil, errors.New("stat failed")
	}

	m := &manager{path: filepath.Join(t.TempDir(), "known_hosts")}
	if err := m.ensureKnownHostsFile(); err == nil {
		t.Fatal("expected stat error")
	}
}

func TestEnsureKnownHostsFileCreateError(t *testing.T) {
	t.Cleanup(resetKnownHostsFns)
	statFn = func(string) (os.FileInfo, error) {
		return nil, os.ErrNotExist
	}
	openFileFn = func(string, int, os.FileMode) (*os.File, error) {
		return nil, errors.New("open failed")
	}

	m := &manager{path: filepath.Join(t.TempDir(), "known_hosts")}
	if err := m.ensureKnownHostsFile(); err == nil {
		t.Fatal("expected create error")
	}
}

func TestAppendHostKeyOpenError(t *testing.T) {
	t.Cleanup(resetKnownHostsFns)
	appendOpenFileFn = func(string) (io.WriteCloser, error) {
		return nil, errors.New("open failed")
	}

	if err := appendHostKey("ignored", [][]byte{[]byte("example.com ssh-ed25519 AAAA")}); err == nil {
		t.Fatal("expected open error")
	}
}

func TestAppendHostKeyWriteError(t *testing.T) {
	t.Cleanup(resetKnownHostsFns)
	appendOpenFileFn = func(string) (io.WriteCloser, error) {
		return errWriteCloser{err: errors.New("write failed")}, nil
	}

	if err := appendHostKey("ignored", [][]byte{[]byte("example.com ssh-ed25519 AAAA")}); err == nil {
		t.Fatal("expected write error")
	}
}

func TestNormalizeHostEntryWithComment(t *testing.T) {
	entry := []byte("example.com ssh-ed25519 AAAA comment here")
	normalized, keyType, err := normalizeHostEntry("example.com", entry)
	if err != nil {
		t.Fatalf("normalizeHostEntry error: %v", err)
	}
	if keyType != "ssh-ed25519" {
		t.Fatalf("expected key type ssh-ed25519, got %s", keyType)
	}
	if string(normalized) != "example.com ssh-ed25519 AAAA comment here" {
		t.Fatalf("unexpected normalized entry: %s", string(normalized))
	}
}

func TestFindHostKeyLineNotExists(t *testing.T) {
	line, err := findHostKeyLine(filepath.Join(t.TempDir(), "missing"), "example.com", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if line != "" {
		t.Fatalf("expected empty line, got %q", line)
	}
}

func TestFindHostKeyLineOpenError(t *testing.T) {
	t.Cleanup(resetKnownHostsFns)
	openFn = func(string) (*os.File, error) {
		return nil, errors.New("open failed")
	}

	if _, err := findHostKeyLine("ignored", "example.com", ""); err == nil {
		t.Fatal("expected open error")
	}
}

func TestFindHostKeyLineScannerError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "known_hosts")
	longLine := strings.Repeat("a", 70000)
	if err := os.WriteFile(path, []byte(longLine+"\n"), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	if _, err := findHostKeyLine(path, "example.com", ""); err == nil {
		t.Fatal("expected scanner error")
	}
}

func TestHostLineMatchesSkips(t *testing.T) {
	if hostLineMatches("example.com", "") {
		t.Fatal("expected empty line to be false")
	}
	if hostLineMatches("example.com", "# comment") {
		t.Fatal("expected comment line to be false")
	}
	if hostLineMatches("example.com", "|1|hash|salt ssh-ed25519 AAAA") {
		t.Fatal("expected hashed entry to be false")
	}
}

func TestDefaultKeyscanSuccess(t *testing.T) {
	t.Cleanup(resetKnownHostsFns)
	keyscanCmdRunner = func(ctx context.Context, args ...string) ([]byte, error) {
		return []byte("example.com ssh-ed25519 AAAA"), nil
	}

	out, err := defaultKeyscan(context.Background(), "example.com", 22, time.Second)
	if err != nil {
		t.Fatalf("defaultKeyscan error: %v", err)
	}
	if string(out) != "example.com ssh-ed25519 AAAA" {
		t.Fatalf("unexpected output: %s", string(out))
	}
}

func TestDefaultKeyscanError(t *testing.T) {
	t.Cleanup(resetKnownHostsFns)
	keyscanCmdRunner = func(ctx context.Context, args ...string) ([]byte, error) {
		return []byte("boom"), errors.New("scan failed")
	}

	if _, err := defaultKeyscan(context.Background(), "example.com", 22, time.Second); err == nil {
		t.Fatal("expected error")
	}
}

func TestEnsureWithEntriesDefaultsPort(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "known_hosts")

	mgr, err := NewManager(path)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	if err := mgr.EnsureWithEntries(context.Background(), "example.com", 0, [][]byte{[]byte("example.com ssh-ed25519 AAAA")}); err != nil {
		t.Fatalf("EnsureWithEntries: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if got := strings.TrimSpace(string(data)); got != "example.com ssh-ed25519 AAAA" {
		t.Fatalf("unexpected known_hosts contents: %s", got)
	}
}

func TestEnsureWithEntriesAppendError(t *testing.T) {
	t.Cleanup(resetKnownHostsFns)
	appendOpenFileFn = func(string) (io.WriteCloser, error) {
		return nil, errors.New("open failed")
	}

	mgr, err := NewManager(filepath.Join(t.TempDir(), "known_hosts"))
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	if err := mgr.EnsureWithEntries(context.Background(), "example.com", 22, [][]byte{[]byte("example.com ssh-ed25519 AAAA")}); err == nil {
		t.Fatal("expected append error")
	}
}

func TestAppendHostKeySkipsEmptyEntry(t *testing.T) {
	t.Cleanup(resetKnownHostsFns)
	buf := &bufferWriteCloser{}
	appendOpenFileFn = func(string) (io.WriteCloser, error) {
		return buf, nil
	}

	if err := appendHostKey("ignored", [][]byte{nil, []byte("example.com ssh-ed25519 AAAA")}); err != nil {
		t.Fatalf("appendHostKey error: %v", err)
	}
	if !strings.Contains(buf.String(), "example.com ssh-ed25519 AAAA") {
		t.Fatalf("expected entry to be written, got %q", buf.String())
	}
}

func TestFindHostKeyLineSkipsInvalidLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "known_hosts")
	contents := strings.Join([]string{
		"other.com ssh-ed25519 AAAA",
		"example.com ssh-ed25519",
		"example.com ssh-ed25519 AAAA",
	}, "\n") + "\n"
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	line, err := findHostKeyLine(path, "example.com", "ssh-ed25519")
	if err != nil {
		t.Fatalf("findHostKeyLine error: %v", err)
	}
	if line != "example.com ssh-ed25519 AAAA" {
		t.Fatalf("unexpected line: %q", line)
	}
}

func TestDefaultKeyscanArgs(t *testing.T) {
	t.Cleanup(resetKnownHostsFns)

	var gotArgs []string
	keyscanCmdRunner = func(ctx context.Context, args ...string) ([]byte, error) {
		gotArgs = append([]string{}, args...)
		return []byte("ok"), nil
	}

	if _, err := defaultKeyscan(context.Background(), "example.com", 0, 0); err != nil {
		t.Fatalf("defaultKeyscan error: %v", err)
	}
	for _, arg := range gotArgs {
		if arg == "-p" {
			t.Fatal("did not expect -p for default port")
		}
	}
	if len(gotArgs) < 3 || gotArgs[len(gotArgs)-1] != "example.com" {
		t.Fatalf("unexpected args: %v", gotArgs)
	}

	keyscanCmdRunner = func(ctx context.Context, args ...string) ([]byte, error) {
		gotArgs = append([]string{}, args...)
		return []byte("ok"), nil
	}
	if _, err := defaultKeyscan(context.Background(), "example.com", 2222, time.Second); err != nil {
		t.Fatalf("defaultKeyscan error: %v", err)
	}
	hasPort := false
	for i := 0; i < len(gotArgs)-1; i++ {
		if gotArgs[i] == "-p" && gotArgs[i+1] == "2222" {
			hasPort = true
			break
		}
	}
	if !hasPort {
		t.Fatalf("expected -p 2222 in args, got %v", gotArgs)
	}
}

func TestKeyscanCmdRunnerDefault(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("ssh-keyscan helper script requires sh")
	}
	t.Cleanup(resetKnownHostsFns)

	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "ssh-keyscan")
	script := []byte("#!/bin/sh\necho example.com ssh-ed25519 AAAA\n")
	if err := os.WriteFile(scriptPath, script, 0700); err != nil {
		t.Fatalf("failed to write script: %v", err)
	}

	oldPath := os.Getenv("PATH")
	if err := os.Setenv("PATH", dir+string(os.PathListSeparator)+oldPath); err != nil {
		t.Fatalf("failed to set PATH: %v", err)
	}
	t.Cleanup(func() { _ = os.Setenv("PATH", oldPath) })

	output, err := keyscanCmdRunner(context.Background(), "example.com")
	if err != nil {
		t.Fatalf("keyscanCmdRunner error: %v", err)
	}
	if strings.TrimSpace(string(output)) != "example.com ssh-ed25519 AAAA" {
		t.Fatalf("unexpected output: %s", string(output))
	}
}

type errWriteCloser struct {
	err error
}

func (e errWriteCloser) Write(p []byte) (int, error) {
	return 0, e.err
}

func (e errWriteCloser) Close() error {
	return nil
}

type bufferWriteCloser struct {
	bytes.Buffer
}

func (b *bufferWriteCloser) Close() error {
	return nil
}
