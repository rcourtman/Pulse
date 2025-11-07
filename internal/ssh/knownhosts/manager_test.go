package knownhosts

import (
	"context"
	"errors"
	"os"
	"path/filepath"
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
	if want := "example.com ssh-ed25519 AAAA\nexample.com,192.0.2.10 ssh-rsa BBBB\n"; string(data) != want {
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
