package dockeragent

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

func TestDetermineSelfUpdateArch_Coverage(t *testing.T) {
	t.Run("known arches", func(t *testing.T) {
		swap(t, &goArch, "amd64")
		if got := determineSelfUpdateArch(); got != "linux-amd64" {
			t.Fatalf("expected linux-amd64, got %q", got)
		}

		swap(t, &goArch, "arm64")
		if got := determineSelfUpdateArch(); got != "linux-arm64" {
			t.Fatalf("expected linux-arm64, got %q", got)
		}

		swap(t, &goArch, "arm")
		if got := determineSelfUpdateArch(); got != "linux-armv7" {
			t.Fatalf("expected linux-armv7, got %q", got)
		}
	})

	t.Run("uname fallback", func(t *testing.T) {
		swap(t, &goArch, "other")
		swap(t, &unameMachine, func() (string, error) {
			return "x86_64", nil
		})
		if got := determineSelfUpdateArch(); got != "linux-amd64" {
			t.Fatalf("expected linux-amd64, got %q", got)
		}
	})

	t.Run("uname error", func(t *testing.T) {
		swap(t, &goArch, "other")
		swap(t, &unameMachine, func() (string, error) {
			return "", errors.New("boom")
		})
		if got := determineSelfUpdateArch(); got != "" {
			t.Fatalf("expected empty result, got %q", got)
		}
	})

	t.Run("uname arm variants", func(t *testing.T) {
		swap(t, &goArch, "other")
		swap(t, &unameMachine, func() (string, error) {
			return "armv7l", nil
		})
		if got := determineSelfUpdateArch(); got != "linux-armv7" {
			t.Fatalf("expected linux-armv7, got %q", got)
		}

		swap(t, &unameMachine, func() (string, error) {
			return "aarch64", nil
		})
		if got := determineSelfUpdateArch(); got != "linux-arm64" {
			t.Fatalf("expected linux-arm64, got %q", got)
		}
	})

	t.Run("uname unknown", func(t *testing.T) {
		swap(t, &goArch, "other")
		swap(t, &unameMachine, func() (string, error) {
			return "mips", nil
		})
		if got := determineSelfUpdateArch(); got != "" {
			t.Fatalf("expected empty result, got %q", got)
		}
	})
}

func TestResolveSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, []byte("data"), 0600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	got, err := resolveSymlink(link)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected, err := filepath.EvalSymlinks(target)
	if err != nil {
		t.Fatalf("eval symlinks target: %v", err)
	}
	if got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}

	if _, err := resolveSymlink(filepath.Join(dir, "missing")); err == nil {
		t.Fatal("expected error for missing symlink")
	}
}

func TestVerifyELFMagic(t *testing.T) {
	dir := t.TempDir()
	valid := filepath.Join(dir, "valid")
	if err := os.WriteFile(valid, []byte{0x7f, 'E', 'L', 'F', 0x01}, 0600); err != nil {
		t.Fatalf("write valid: %v", err)
	}
	if err := verifyELFMagic(valid); err != nil {
		t.Fatalf("expected valid ELF, got %v", err)
	}

	invalid := filepath.Join(dir, "invalid")
	if err := os.WriteFile(invalid, []byte("nope"), 0600); err != nil {
		t.Fatalf("write invalid: %v", err)
	}
	if err := verifyELFMagic(invalid); err == nil {
		t.Fatal("expected error for invalid magic")
	}

	partial := filepath.Join(dir, "partial")
	if err := os.WriteFile(partial, []byte{0x7f, 'E'}, 0600); err != nil {
		t.Fatalf("write partial: %v", err)
	}
	if err := verifyELFMagic(partial); err == nil {
		t.Fatal("expected error for short file")
	}

	if err := verifyELFMagic(filepath.Join(dir, "missing")); err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestCheckForUpdates(t *testing.T) {
	swap(t, &selfUpdateRetrySleepFn, func(context.Context, time.Duration) error { return nil })

	t.Run("dev version skips", func(t *testing.T) {
		swap(t, &Version, "dev")
		agent := &Agent{logger: zerolog.Nop()}
		agent.checkForUpdates(context.Background())
	})

	t.Run("no target skips", func(t *testing.T) {
		swap(t, &Version, "1.0.0")
		agent := &Agent{logger: zerolog.Nop()}
		agent.checkForUpdates(context.Background())
	})

	t.Run("request creation error", func(t *testing.T) {
		swap(t, &Version, "1.0.0")
		agent := &Agent{
			logger:  zerolog.Nop(),
			targets: []TargetConfig{{URL: "http://example.com/\x7f"}},
		}
		agent.checkForUpdates(context.Background())
	})

	t.Run("http error", func(t *testing.T) {
		swap(t, &Version, "1.0.0")
		client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("boom")
		})}
		agent := &Agent{
			logger:  zerolog.Nop(),
			targets: []TargetConfig{{URL: "http://example.com", Token: "token"}},
			httpClients: map[bool]*http.Client{
				false: client,
			},
		}
		agent.checkForUpdates(context.Background())
	})

	t.Run("non-200 status", func(t *testing.T) {
		swap(t, &Version, "1.0.0")
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		agent := &Agent{
			logger:  zerolog.Nop(),
			targets: []TargetConfig{{URL: server.URL, Token: "token"}},
			httpClients: map[bool]*http.Client{
				false: server.Client(),
			},
		}
		agent.checkForUpdates(context.Background())
	})

	t.Run("decode error", func(t *testing.T) {
		swap(t, &Version, "1.0.0")
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("{"))
		}))
		defer server.Close()

		agent := &Agent{
			logger:  zerolog.Nop(),
			targets: []TargetConfig{{URL: server.URL, Token: "token"}},
			httpClients: map[bool]*http.Client{
				false: server.Client(),
			},
		}
		agent.checkForUpdates(context.Background())
	})

	t.Run("server dev version", func(t *testing.T) {
		swap(t, &Version, "1.0.0")
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"version":"dev"}`))
		}))
		defer server.Close()

		agent := &Agent{
			logger:  zerolog.Nop(),
			targets: []TargetConfig{{URL: server.URL, Token: "token"}},
			httpClients: map[bool]*http.Client{
				false: server.Client(),
			},
		}
		agent.checkForUpdates(context.Background())
	})

	t.Run("up to date", func(t *testing.T) {
		swap(t, &Version, "v1.2.3")
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"version":"1.2.3"}`))
		}))
		defer server.Close()

		agent := &Agent{
			logger:  zerolog.Nop(),
			targets: []TargetConfig{{URL: server.URL, Token: "token"}},
			httpClients: map[bool]*http.Client{
				false: server.Client(),
			},
		}
		agent.checkForUpdates(context.Background())
	})

	t.Run("server older version skips downgrade", func(t *testing.T) {
		swap(t, &Version, "1.2.4")
		called := false
		swap(t, &selfUpdateFunc, func(*Agent, context.Context) error {
			called = true
			return nil
		})
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"version":"1.2.3"}`))
		}))
		defer server.Close()

		agent := &Agent{
			logger:  zerolog.Nop(),
			targets: []TargetConfig{{URL: server.URL, Token: "token"}},
			httpClients: map[bool]*http.Client{
				false: server.Client(),
			},
		}
		agent.checkForUpdates(context.Background())
		if called {
			t.Fatal("expected selfUpdate not to be called for older server version")
		}
	})

	t.Run("version check retries transient request failures", func(t *testing.T) {
		swap(t, &Version, "1.2.3")
		called := false
		swap(t, &selfUpdateFunc, func(*Agent, context.Context) error {
			called = true
			return nil
		})

		attempts := 0
		client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			attempts++
			if attempts < 3 {
				return nil, errors.New("transient network error")
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"version":"1.2.4"}`)),
				Header:     make(http.Header),
			}, nil
		})}

		agent := &Agent{
			logger:  zerolog.Nop(),
			targets: []TargetConfig{{URL: "http://example.com", Token: "token"}},
			httpClients: map[bool]*http.Client{
				false: client,
			},
		}
		agent.checkForUpdates(context.Background())
		if attempts != 3 {
			t.Fatalf("expected 3 version-check attempts, got %d", attempts)
		}
		if !called {
			t.Fatal("expected selfUpdate to be called after retry recovery")
		}
	})

	t.Run("version check does not retry non-retryable status", func(t *testing.T) {
		swap(t, &Version, "1.2.3")
		called := false
		swap(t, &selfUpdateFunc, func(*Agent, context.Context) error {
			called = true
			return nil
		})

		attempts := 0
		client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			attempts++
			return &http.Response{
				StatusCode: http.StatusBadRequest,
				Status:     "400 Bad Request",
				Body:       io.NopCloser(strings.NewReader(`bad`)),
				Header:     make(http.Header),
			}, nil
		})}

		agent := &Agent{
			logger:  zerolog.Nop(),
			targets: []TargetConfig{{URL: "http://example.com", Token: "token"}},
			httpClients: map[bool]*http.Client{
				false: client,
			},
		}
		agent.checkForUpdates(context.Background())
		if attempts != 1 {
			t.Fatalf("expected 1 version-check attempt for non-retryable status, got %d", attempts)
		}
		if called {
			t.Fatal("expected selfUpdate not to be called")
		}
	})

	t.Run("update success", func(t *testing.T) {
		swap(t, &Version, "1.2.3")
		called := false
		swap(t, &selfUpdateFunc, func(*Agent, context.Context) error {
			called = true
			return nil
		})
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"version":"1.2.4"}`))
		}))
		defer server.Close()

		agent := &Agent{
			logger:  zerolog.Nop(),
			targets: []TargetConfig{{URL: server.URL, Token: "token"}},
			httpClients: map[bool]*http.Client{
				false: server.Client(),
			},
		}
		agent.checkForUpdates(context.Background())
		if !called {
			t.Fatal("expected selfUpdate to be called")
		}
	})

	t.Run("update error", func(t *testing.T) {
		swap(t, &Version, "1.2.3")
		swap(t, &selfUpdateFunc, func(*Agent, context.Context) error {
			return errors.New("update failed")
		})
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"version":"1.2.4"}`))
		}))
		defer server.Close()

		agent := &Agent{
			logger:  zerolog.Nop(),
			targets: []TargetConfig{{URL: server.URL, Token: "token"}},
			httpClients: map[bool]*http.Client{
				false: server.Client(),
			},
		}
		agent.checkForUpdates(context.Background())
	})
}

type sizeReadCloser struct {
	remaining int64
}

func (s *sizeReadCloser) Read(p []byte) (int, error) {
	if s.remaining <= 0 {
		return 0, io.EOF
	}
	if int64(len(p)) > s.remaining {
		p = p[:s.remaining]
	}
	for i := range p {
		p[i] = 0
	}
	s.remaining -= int64(len(p))
	return len(p), nil
}

func (s *sizeReadCloser) Close() error {
	return nil
}

func elfBytes() []byte {
	return []byte{0x7f, 'E', 'L', 'F', 0x01, 0x02, 0x03}
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func TestSelfUpdate(t *testing.T) {
	swap(t, &selfUpdateRetrySleepFn, func(context.Context, time.Duration) error { return nil })

	t.Run("no target", func(t *testing.T) {
		agent := &Agent{logger: zerolog.Nop()}
		if err := agent.selfUpdate(context.Background()); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("executable error", func(t *testing.T) {
		agent := &Agent{
			logger:  zerolog.Nop(),
			targets: []TargetConfig{{URL: "http://example.com", Token: "token"}},
		}
		swap(t, &osExecutableFn, func() (string, error) {
			return "", errors.New("no exec")
		})

		if err := agent.selfUpdate(context.Background()); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("request creation error", func(t *testing.T) {
		agent := &Agent{
			logger:  zerolog.Nop(),
			targets: []TargetConfig{{URL: "http://example.com/\x7f", Token: "token"}},
		}
		dir := t.TempDir()
		execPath := filepath.Join(dir, "exec")
		if err := os.WriteFile(execPath, elfBytes(), 0700); err != nil {
			t.Fatalf("write exec: %v", err)
		}
		swap(t, &osExecutableFn, func() (string, error) {
			return execPath, nil
		})

		if err := agent.selfUpdate(context.Background()); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("request error", func(t *testing.T) {
		agent := &Agent{
			logger:  zerolog.Nop(),
			targets: []TargetConfig{{URL: "http://example.com", Token: "token"}},
			httpClients: map[bool]*http.Client{
				false: {Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					return nil, errors.New("send failed")
				})},
			},
		}
		dir := t.TempDir()
		execPath := filepath.Join(dir, "exec")
		if err := os.WriteFile(execPath, elfBytes(), 0700); err != nil {
			t.Fatalf("write exec: %v", err)
		}
		swap(t, &osExecutableFn, func() (string, error) {
			return execPath, nil
		})

		if err := agent.selfUpdate(context.Background()); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("download retries transient request failures", func(t *testing.T) {
		body := elfBytes()
		attempts := 0
		client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			attempts++
			if attempts < 3 {
				return nil, errors.New("temporary transport failure")
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(body)),
				Header:     http.Header{"X-Checksum-Sha256": []string{sha256Hex(body)}},
			}, nil
		})}

		agent := &Agent{
			logger:  zerolog.Nop(),
			targets: []TargetConfig{{URL: "http://example.com", Token: "token"}},
			httpClients: map[bool]*http.Client{
				false: client,
			},
		}

		dir := t.TempDir()
		execPath := filepath.Join(dir, "exec")
		if err := os.WriteFile(execPath, elfBytes(), 0700); err != nil {
			t.Fatalf("write exec: %v", err)
		}
		swap(t, &osExecutableFn, func() (string, error) {
			return execPath, nil
		})
		swap(t, &syscallExecFn, func(string, []string, []string) error {
			return errors.New("exec failed")
		})
		swap(t, &execCommandContextFn, func(ctx context.Context, name string, arg ...string) *exec.Cmd {
			return exec.Command("echo", "ok")
		})

		if err := agent.selfUpdate(context.Background()); err == nil {
			t.Fatal("expected error from exec failure after successful retry")
		}
		if attempts != 3 {
			t.Fatalf("expected 3 download attempts, got %d", attempts)
		}
	})

	t.Run("symlink resolved", func(t *testing.T) {
		client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("send failed")
		})}

		agent := &Agent{
			logger:  zerolog.Nop(),
			targets: []TargetConfig{{URL: "http://example.com", Token: "token"}},
			httpClients: map[bool]*http.Client{
				false: client,
			},
		}

		dir := t.TempDir()
		target := filepath.Join(dir, "exec-target")
		if err := os.WriteFile(target, elfBytes(), 0700); err != nil {
			t.Fatalf("write target: %v", err)
		}
		link := filepath.Join(dir, "exec-link")
		if err := os.Symlink(target, link); err != nil {
			t.Fatalf("symlink: %v", err)
		}
		swap(t, &osExecutableFn, func() (string, error) {
			return link, nil
		})

		if err := agent.selfUpdate(context.Background()); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("status error", func(t *testing.T) {
		client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusInternalServerError,
				Status:     http.StatusText(http.StatusInternalServerError),
				Body:       io.NopCloser(strings.NewReader("fail")),
				Header:     make(http.Header),
			}, nil
		})}

		agent := &Agent{
			logger:  zerolog.Nop(),
			targets: []TargetConfig{{URL: "http://example.com", Token: "token"}},
			httpClients: map[bool]*http.Client{
				false: client,
			},
		}
		dir := t.TempDir()
		execPath := filepath.Join(dir, "exec")
		if err := os.WriteFile(execPath, elfBytes(), 0700); err != nil {
			t.Fatalf("write exec: %v", err)
		}
		swap(t, &osExecutableFn, func() (string, error) {
			return execPath, nil
		})

		if err := agent.selfUpdate(context.Background()); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("create temp error", func(t *testing.T) {
		client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			body := elfBytes()
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(body)),
				Header:     http.Header{"X-Checksum-Sha256": []string{sha256Hex(body)}},
			}, nil
		})}

		agent := &Agent{
			logger:  zerolog.Nop(),
			targets: []TargetConfig{{URL: "http://example.com", Token: "token"}},
			httpClients: map[bool]*http.Client{
				false: client,
			},
		}

		swap(t, &osExecutableFn, func() (string, error) {
			return filepath.Join(t.TempDir(), "missing", "exec"), nil
		})

		if err := agent.selfUpdate(context.Background()); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("copy error", func(t *testing.T) {
		client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       errReadCloser{err: errors.New("read failed")},
				Header:     http.Header{"X-Checksum-Sha256": []string{"ignored"}},
			}, nil
		})}

		agent := &Agent{
			logger:  zerolog.Nop(),
			targets: []TargetConfig{{URL: "http://example.com", Token: "token"}},
			httpClients: map[bool]*http.Client{
				false: client,
			},
		}
		dir := t.TempDir()
		execPath := filepath.Join(dir, "exec")
		if err := os.WriteFile(execPath, elfBytes(), 0700); err != nil {
			t.Fatalf("write exec: %v", err)
		}
		swap(t, &osExecutableFn, func() (string, error) {
			return execPath, nil
		})

		if err := agent.selfUpdate(context.Background()); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("too large", func(t *testing.T) {
		client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       &sizeReadCloser{remaining: (100 * 1024 * 1024) + 1},
				Header:     http.Header{"X-Checksum-Sha256": []string{"ignored"}},
			}, nil
		})}

		agent := &Agent{
			logger:  zerolog.Nop(),
			targets: []TargetConfig{{URL: "http://example.com", Token: "token"}},
			httpClients: map[bool]*http.Client{
				false: client,
			},
		}
		dir := t.TempDir()
		execPath := filepath.Join(dir, "exec")
		if err := os.WriteFile(execPath, elfBytes(), 0700); err != nil {
			t.Fatalf("write exec: %v", err)
		}
		swap(t, &osExecutableFn, func() (string, error) {
			return execPath, nil
		})

		if err := agent.selfUpdate(context.Background()); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("close error", func(t *testing.T) {
		client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			body := elfBytes()
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(body)),
				Header:     http.Header{"X-Checksum-Sha256": []string{sha256Hex(body)}},
			}, nil
		})}

		agent := &Agent{
			logger:  zerolog.Nop(),
			targets: []TargetConfig{{URL: "http://example.com", Token: "token"}},
			httpClients: map[bool]*http.Client{
				false: client,
			},
		}
		dir := t.TempDir()
		execPath := filepath.Join(dir, "exec")
		if err := os.WriteFile(execPath, elfBytes(), 0700); err != nil {
			t.Fatalf("write exec: %v", err)
		}
		swap(t, &osExecutableFn, func() (string, error) {
			return execPath, nil
		})
		swap(t, &closeFileFn, func(*os.File) error {
			return errors.New("close failed")
		})

		if err := agent.selfUpdate(context.Background()); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("invalid elf", func(t *testing.T) {
		body := []byte("bad")
		client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(body)),
				Header:     http.Header{"X-Checksum-Sha256": []string{sha256Hex(body)}},
			}, nil
		})}

		agent := &Agent{
			logger:  zerolog.Nop(),
			targets: []TargetConfig{{URL: "http://example.com", Token: "token"}},
			httpClients: map[bool]*http.Client{
				false: client,
			},
		}
		dir := t.TempDir()
		execPath := filepath.Join(dir, "exec")
		if err := os.WriteFile(execPath, elfBytes(), 0700); err != nil {
			t.Fatalf("write exec: %v", err)
		}
		swap(t, &osExecutableFn, func() (string, error) {
			return execPath, nil
		})

		if err := agent.selfUpdate(context.Background()); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("missing checksum", func(t *testing.T) {
		body := elfBytes()
		client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(body)),
				Header:     make(http.Header),
			}, nil
		})}

		agent := &Agent{
			logger:  zerolog.Nop(),
			targets: []TargetConfig{{URL: "http://example.com", Token: "token"}},
			httpClients: map[bool]*http.Client{
				false: client,
			},
		}
		dir := t.TempDir()
		execPath := filepath.Join(dir, "exec")
		if err := os.WriteFile(execPath, elfBytes(), 0700); err != nil {
			t.Fatalf("write exec: %v", err)
		}
		swap(t, &osExecutableFn, func() (string, error) {
			return execPath, nil
		})

		if err := agent.selfUpdate(context.Background()); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("checksum mismatch", func(t *testing.T) {
		body := elfBytes()
		client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(body)),
				Header:     http.Header{"X-Checksum-Sha256": []string{"bad"}},
			}, nil
		})}

		agent := &Agent{
			logger:  zerolog.Nop(),
			targets: []TargetConfig{{URL: "http://example.com", Token: "token"}},
			httpClients: map[bool]*http.Client{
				false: client,
			},
		}
		dir := t.TempDir()
		execPath := filepath.Join(dir, "exec")
		if err := os.WriteFile(execPath, elfBytes(), 0700); err != nil {
			t.Fatalf("write exec: %v", err)
		}
		swap(t, &osExecutableFn, func() (string, error) {
			return execPath, nil
		})

		if err := agent.selfUpdate(context.Background()); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("chmod error", func(t *testing.T) {
		body := elfBytes()
		client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(body)),
				Header:     http.Header{"X-Checksum-Sha256": []string{sha256Hex(body)}},
			}, nil
		})}

		agent := &Agent{
			logger:  zerolog.Nop(),
			targets: []TargetConfig{{URL: "http://example.com", Token: "token"}},
			httpClients: map[bool]*http.Client{
				false: client,
			},
		}
		dir := t.TempDir()
		execPath := filepath.Join(dir, "exec")
		if err := os.WriteFile(execPath, elfBytes(), 0700); err != nil {
			t.Fatalf("write exec: %v", err)
		}
		swap(t, &osExecutableFn, func() (string, error) {
			return execPath, nil
		})
		swap(t, &osChmodFn, func(string, os.FileMode) error {
			return errors.New("chmod failed")
		})

		if err := agent.selfUpdate(context.Background()); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("rename backup error", func(t *testing.T) {
		body := elfBytes()
		client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(body)),
				Header:     http.Header{"X-Checksum-Sha256": []string{sha256Hex(body)}},
			}, nil
		})}

		agent := &Agent{
			logger:  zerolog.Nop(),
			targets: []TargetConfig{{URL: "http://example.com", Token: "token"}},
			httpClients: map[bool]*http.Client{
				false: client,
			},
		}
		dir := t.TempDir()
		execPath := filepath.Join(dir, "exec")
		if err := os.WriteFile(execPath, elfBytes(), 0700); err != nil {
			t.Fatalf("write exec: %v", err)
		}
		swap(t, &osExecutableFn, func() (string, error) {
			return execPath, nil
		})
		swap(t, &osRenameFn, func(string, string) error {
			return errors.New("rename failed")
		})

		if err := agent.selfUpdate(context.Background()); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("rename replace error", func(t *testing.T) {
		body := elfBytes()
		client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(body)),
				Header:     http.Header{"X-Checksum-Sha256": []string{sha256Hex(body)}},
			}, nil
		})}

		agent := &Agent{
			logger:  zerolog.Nop(),
			targets: []TargetConfig{{URL: "http://example.com", Token: "token"}},
			httpClients: map[bool]*http.Client{
				false: client,
			},
		}
		dir := t.TempDir()
		execPath := filepath.Join(dir, "exec")
		if err := os.WriteFile(execPath, elfBytes(), 0700); err != nil {
			t.Fatalf("write exec: %v", err)
		}
		swap(t, &osExecutableFn, func() (string, error) {
			return execPath, nil
		})

		calls := 0
		swap(t, &osRenameFn, func(old, new string) error {
			calls++
			if calls == 2 {
				return errors.New("rename failed")
			}
			return os.Rename(old, new)
		})

		if err := agent.selfUpdate(context.Background()); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("unraid read error", func(t *testing.T) {
		body := elfBytes()
		client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(body)),
				Header:     http.Header{"X-Checksum-Sha256": []string{sha256Hex(body)}},
			}, nil
		})}

		agent := &Agent{
			logger:  zerolog.Nop(),
			targets: []TargetConfig{{URL: "http://example.com", Token: "token"}},
			httpClients: map[bool]*http.Client{
				false: client,
			},
		}
		dir := t.TempDir()
		execPath := filepath.Join(dir, "exec")
		if err := os.WriteFile(execPath, elfBytes(), 0700); err != nil {
			t.Fatalf("write exec: %v", err)
		}
		swap(t, &osExecutableFn, func() (string, error) {
			return execPath, nil
		})
		unraidPath := filepath.Join(dir, "unraid-version")
		if err := os.WriteFile(unraidPath, []byte("1"), 0600); err != nil {
			t.Fatalf("write unraid: %v", err)
		}
		swap(t, &unraidVersionPath, unraidPath)
		persist := filepath.Join(dir, "persist")
		if err := os.WriteFile(persist, []byte("old"), 0600); err != nil {
			t.Fatalf("write persist: %v", err)
		}
		swap(t, &unraidPersistPath, persist)
		swap(t, &osReadFileFn, func(string) ([]byte, error) {
			return nil, errors.New("read failed")
		})
		swap(t, &syscallExecFn, func(string, []string, []string) error {
			return errors.New("exec failed")
		})

		_ = agent.selfUpdate(context.Background())
	})

	t.Run("unraid write error", func(t *testing.T) {
		body := elfBytes()
		client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(body)),
				Header:     http.Header{"X-Checksum-Sha256": []string{sha256Hex(body)}},
			}, nil
		})}

		agent := &Agent{
			logger:  zerolog.Nop(),
			targets: []TargetConfig{{URL: "http://example.com", Token: "token"}},
			httpClients: map[bool]*http.Client{
				false: client,
			},
		}
		dir := t.TempDir()
		execPath := filepath.Join(dir, "exec")
		if err := os.WriteFile(execPath, elfBytes(), 0700); err != nil {
			t.Fatalf("write exec: %v", err)
		}
		swap(t, &osExecutableFn, func() (string, error) {
			return execPath, nil
		})
		unraidPath := filepath.Join(dir, "unraid-version")
		if err := os.WriteFile(unraidPath, []byte("1"), 0600); err != nil {
			t.Fatalf("write unraid: %v", err)
		}
		swap(t, &unraidVersionPath, unraidPath)
		persist := filepath.Join(dir, "persist")
		if err := os.WriteFile(persist, []byte("old"), 0600); err != nil {
			t.Fatalf("write persist: %v", err)
		}
		swap(t, &unraidPersistPath, persist)
		swap(t, &osWriteFileFn, func(string, []byte, os.FileMode) error {
			return errors.New("write failed")
		})
		swap(t, &syscallExecFn, func(string, []string, []string) error {
			return errors.New("exec failed")
		})

		_ = agent.selfUpdate(context.Background())
	})

	t.Run("unraid rename error", func(t *testing.T) {
		body := elfBytes()
		client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(body)),
				Header:     http.Header{"X-Checksum-Sha256": []string{sha256Hex(body)}},
			}, nil
		})}

		agent := &Agent{
			logger:  zerolog.Nop(),
			targets: []TargetConfig{{URL: "http://example.com", Token: "token"}},
			httpClients: map[bool]*http.Client{
				false: client,
			},
		}
		dir := t.TempDir()
		execPath := filepath.Join(dir, "exec")
		if err := os.WriteFile(execPath, elfBytes(), 0700); err != nil {
			t.Fatalf("write exec: %v", err)
		}
		swap(t, &osExecutableFn, func() (string, error) {
			return execPath, nil
		})
		unraidPath := filepath.Join(dir, "unraid-version")
		if err := os.WriteFile(unraidPath, []byte("1"), 0600); err != nil {
			t.Fatalf("write unraid: %v", err)
		}
		swap(t, &unraidVersionPath, unraidPath)
		persist := filepath.Join(dir, "persist")
		if err := os.WriteFile(persist, []byte("old"), 0600); err != nil {
			t.Fatalf("write persist: %v", err)
		}
		swap(t, &unraidPersistPath, persist)
		swap(t, &osRenameFn, func(old, new string) error {
			if strings.HasSuffix(new, ".tmp") {
				return os.Rename(old, new)
			}
			return errors.New("rename failed")
		})
		swap(t, &syscallExecFn, func(string, []string, []string) error {
			return errors.New("exec failed")
		})

		_ = agent.selfUpdate(context.Background())
	})

	t.Run("unraid rename persist error", func(t *testing.T) {
		body := elfBytes()
		client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(body)),
				Header:     http.Header{"X-Checksum-Sha256": []string{sha256Hex(body)}},
			}, nil
		})}

		agent := &Agent{
			logger:  zerolog.Nop(),
			targets: []TargetConfig{{URL: "http://example.com", Token: "token"}},
			httpClients: map[bool]*http.Client{
				false: client,
			},
		}
		dir := t.TempDir()
		execPath := filepath.Join(dir, "exec")
		if err := os.WriteFile(execPath, elfBytes(), 0700); err != nil {
			t.Fatalf("write exec: %v", err)
		}
		swap(t, &osExecutableFn, func() (string, error) {
			return execPath, nil
		})
		unraidPath := filepath.Join(dir, "unraid-version")
		if err := os.WriteFile(unraidPath, []byte("1"), 0600); err != nil {
			t.Fatalf("write unraid: %v", err)
		}
		swap(t, &unraidVersionPath, unraidPath)
		persist := filepath.Join(dir, "persist")
		if err := os.WriteFile(persist, []byte("old"), 0600); err != nil {
			t.Fatalf("write persist: %v", err)
		}
		swap(t, &unraidPersistPath, persist)

		swap(t, &osRenameFn, func(old, new string) error {
			if new == persist {
				return errors.New("rename failed")
			}
			return os.Rename(old, new)
		})
		swap(t, &syscallExecFn, func(string, []string, []string) error {
			return errors.New("exec failed")
		})

		_ = agent.selfUpdate(context.Background())
	})

	t.Run("unraid persist success", func(t *testing.T) {
		body := elfBytes()
		client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(body)),
				Header:     http.Header{"X-Checksum-Sha256": []string{sha256Hex(body)}},
			}, nil
		})}

		agent := &Agent{
			logger:  zerolog.Nop(),
			targets: []TargetConfig{{URL: "http://example.com", Token: "token"}},
			httpClients: map[bool]*http.Client{
				false: client,
			},
		}
		dir := t.TempDir()
		execPath := filepath.Join(dir, "exec")
		if err := os.WriteFile(execPath, elfBytes(), 0700); err != nil {
			t.Fatalf("write exec: %v", err)
		}
		swap(t, &osExecutableFn, func() (string, error) {
			return execPath, nil
		})
		unraidPath := filepath.Join(dir, "unraid-version")
		if err := os.WriteFile(unraidPath, []byte("1"), 0600); err != nil {
			t.Fatalf("write unraid: %v", err)
		}
		swap(t, &unraidVersionPath, unraidPath)
		persist := filepath.Join(dir, "persist")
		if err := os.WriteFile(persist, []byte("old"), 0600); err != nil {
			t.Fatalf("write persist: %v", err)
		}
		swap(t, &unraidPersistPath, persist)
		swap(t, &syscallExecFn, func(string, []string, []string) error {
			return errors.New("exec failed")
		})

		_ = agent.selfUpdate(context.Background())
	})

	t.Run("exec error", func(t *testing.T) {
		body := elfBytes()
		client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(body)),
				Header:     http.Header{"X-Checksum-Sha256": []string{sha256Hex(body)}},
			}, nil
		})}

		agent := &Agent{
			logger:  zerolog.Nop(),
			targets: []TargetConfig{{URL: "http://example.com", Token: "token"}},
			httpClients: map[bool]*http.Client{
				false: client,
			},
		}
		dir := t.TempDir()
		execPath := filepath.Join(dir, "exec")
		if err := os.WriteFile(execPath, elfBytes(), 0700); err != nil {
			t.Fatalf("write exec: %v", err)
		}
		swap(t, &osExecutableFn, func() (string, error) {
			return execPath, nil
		})
		swap(t, &syscallExecFn, func(string, []string, []string) error {
			return errors.New("exec failed")
		})

		if err := agent.selfUpdate(context.Background()); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("exec success", func(t *testing.T) {
		body := elfBytes()
		client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(body)),
				Header:     http.Header{"X-Checksum-Sha256": []string{sha256Hex(body)}},
			}, nil
		})}

		agent := &Agent{
			logger:  zerolog.Nop(),
			targets: []TargetConfig{{URL: "http://example.com", Token: "token"}},
			httpClients: map[bool]*http.Client{
				false: client,
			},
		}
		dir := t.TempDir()
		execPath := filepath.Join(dir, "exec")
		if err := os.WriteFile(execPath, elfBytes(), 0700); err != nil {
			t.Fatalf("write exec: %v", err)
		}
		swap(t, &osExecutableFn, func() (string, error) {
			return execPath, nil
		})
		swap(t, &syscallExecFn, func(string, []string, []string) error {
			return nil
		})
		// Mock pre-flight check to succeed
		swap(t, &execCommandContextFn, func(ctx context.Context, name string, arg ...string) *exec.Cmd {
			// echo returns 0 exit code
			return exec.Command("echo", "ok")
		})

		if err := agent.selfUpdate(context.Background()); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("arch fallback to default", func(t *testing.T) {
		body := elfBytes()
		client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if strings.Contains(req.URL.RawQuery, "arch=") {
				return &http.Response{
					StatusCode: http.StatusNotFound,
					Status:     "404",
					Body:       io.NopCloser(strings.NewReader("missing")),
					Header:     make(http.Header),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(body)),
				Header:     http.Header{"X-Checksum-Sha256": []string{sha256Hex(body)}},
			}, nil
		})}

		agent := &Agent{
			logger:  zerolog.Nop(),
			targets: []TargetConfig{{URL: "http://example.com", Token: "token"}},
			httpClients: map[bool]*http.Client{
				false: client,
			},
		}
		dir := t.TempDir()
		execPath := filepath.Join(dir, "exec")
		if err := os.WriteFile(execPath, elfBytes(), 0700); err != nil {
			t.Fatalf("write exec: %v", err)
		}
		swap(t, &osExecutableFn, func() (string, error) {
			return execPath, nil
		})
		swap(t, &goArch, "amd64")
		swap(t, &syscallExecFn, func(string, []string, []string) error {
			return errors.New("exec failed")
		})
		// Mock pre-flight check to succeed
		swap(t, &execCommandContextFn, func(ctx context.Context, name string, arg ...string) *exec.Cmd {
			return exec.Command("echo", "ok")
		})

		if err := agent.selfUpdate(context.Background()); err == nil {
			t.Fatal("expected error from exec")
		}
	})
}
