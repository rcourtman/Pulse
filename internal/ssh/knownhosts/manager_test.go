package knownhosts

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

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
