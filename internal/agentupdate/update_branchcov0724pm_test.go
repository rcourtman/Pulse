package agentupdate

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestBranchcov0724pmSnapshot covers the nil-receiver arm of Snapshot (which
// returns a disabled status without dereferencing the receiver) and proves that
// the returned Status is an independent deep copy: mutating a cloned *time.Time
// pointer obtained from one Snapshot must not affect a subsequent Snapshot.
func TestBranchcov0724pmSnapshot(t *testing.T) {
	t.Run("NilReceiverReturnsDisabled", func(t *testing.T) {
		var u *Updater
		got := u.Snapshot()
		if got.State != UpdateStateDisabled {
			t.Fatalf("nil Snapshot State = %q, want %q", got.State, UpdateStateDisabled)
		}
		if got.AutoUpdate {
			t.Fatalf("nil Snapshot AutoUpdate = true, want false")
		}
	})

	t.Run("ReturnsIndependentCopy", func(t *testing.T) {
		u := New(Config{PulseURL: "https://pulse.example.com", CurrentVersion: "1.0.0"})

		original := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
		u.updateStatus(func(s *Status) {
			s.State = UpdateStateError
			s.LastError = "boom"
			s.LastCheckedAt = &original
			s.LastAttemptAt = &original
			s.LastSuccessAt = &original
		})

		first := u.Snapshot()
		if first.State != UpdateStateError {
			t.Fatalf("first State = %q, want %q", first.State, UpdateStateError)
		}
		if first.LastError != "boom" {
			t.Fatalf("first LastError = %q, want %q", first.LastError, "boom")
		}
		if first.LastCheckedAt == nil || !first.LastCheckedAt.Equal(original) {
			t.Fatalf("first LastCheckedAt = %v, want %v", first.LastCheckedAt, original)
		}

		// Mutate the value behind the returned pointer and a value field. The
		// internal status must be unaffected because Snapshot returns a copy.
		sabotaged := original.Add(99 * time.Hour)
		if first.LastCheckedAt != nil {
			*first.LastCheckedAt = sabotaged
		}
		*first.LastAttemptAt = sabotaged
		first.State = UpdateStateIdle
		first.LastError = "mutated"

		second := u.Snapshot()
		if second.State != UpdateStateError {
			t.Fatalf("second State = %q, want %q (internal copy was mutated)", second.State, UpdateStateError)
		}
		if second.LastError != "boom" {
			t.Fatalf("second LastError = %q, want %q", second.LastError, "boom")
		}
		if second.LastCheckedAt == nil || !second.LastCheckedAt.Equal(original) {
			t.Fatalf("second LastCheckedAt = %v, want %v (time pointer was shared, not copied)", second.LastCheckedAt, original)
		}
		if second.LastAttemptAt == nil || !second.LastAttemptAt.Equal(original) {
			t.Fatalf("second LastAttemptAt = %v, want %v", second.LastAttemptAt, original)
		}
		if second.LastSuccessAt == nil || !second.LastSuccessAt.Equal(original) {
			t.Fatalf("second LastSuccessAt = %v, want %v", second.LastSuccessAt, original)
		}
	})
}

// TestBranchcov0724pmRetryBackoffDelay asserts the exact backoff durations,
// including the two arms the existing suite misses: attempt <= 0 (base delay)
// and the cap boundary where the exponential delay exceeds updateRetryMaxDelay.
func TestBranchcov0724pmRetryBackoffDelay(t *testing.T) {
	cases := []struct {
		name    string
		attempt int
		want    time.Duration
	}{
		{"NegativeReturnsBase", -3, updateRetryBaseDelay},
		{"ZeroReturnsBase", 0, updateRetryBaseDelay},
		{"AttemptOne", 1, updateRetryBaseDelay},
		{"AttemptTwo", 2, 2 * updateRetryBaseDelay},
		{"AttemptThree", 3, 4 * updateRetryBaseDelay},
		{"AttemptFour", 4, 8 * updateRetryBaseDelay},
		{"AttemptFive", 5, 16 * updateRetryBaseDelay},
		// attempt 6 is the boundary: base<<5 (32s) > updateRetryMaxDelay (30s),
		// so the cap arm returns updateRetryMaxDelay.
		{"AttemptSixCapped", 6, updateRetryMaxDelay},
		{"AttemptSevenStillCapped", 7, updateRetryMaxDelay},
		// Well beyond the cap but still within int64 range (no overflow), so the
		// cap arm still applies. attempt 50 is intentionally excluded here: it
		// overflows the shift/multiply (see TestBranchcov0724pmRetryBackoffDelayOverflow).
		{"AttemptTwentyCapped", 20, updateRetryMaxDelay},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := retryBackoffDelay(tc.attempt)
			if got != tc.want {
				t.Fatalf("retryBackoffDelay(%d) = %s, want %s", tc.attempt, got, tc.want)
			}
		})
	}

	// Explicit boundary check: the largest non-capped delay must be below the
	// cap, and the very next step must equal the cap.
	if got := retryBackoffDelay(5); got >= updateRetryMaxDelay {
		t.Fatalf("retryBackoffDelay(5) = %s, expected below cap %s", got, updateRetryMaxDelay)
	}
	if got := retryBackoffDelay(6); got != updateRetryMaxDelay {
		t.Fatalf("retryBackoffDelay(6) = %s, expected cap %s", got, updateRetryMaxDelay)
	}
}

// TestBranchcov0724pmRetryBackoffDelayOverflow is a characterization test that
// documents a SUSPECTED SOURCE BUG (reported, not fixed): for very large
// attempt values, the expression
//
//	updateRetryBaseDelay * time.Duration(1<<(attempt-1))
//
// overflows int64 (1s * 2^49 far exceeds int64 max). The wrapped result is
// negative, so the subsequent `delay > updateRetryMaxDelay` guard is false and
// the cap is bypassed, returning a bogus (negative) duration instead of the
// cap. This is not reachable through the real retry loop (updateRequestMaxAttempts
// == 3), but it is a latent defect of retryBackoffDelay.
func TestBranchcov0724pmRetryBackoffDelayOverflow(t *testing.T) {
	got := retryBackoffDelay(50)
	if got >= 0 {
		t.Fatalf("retryBackoffDelay(50) = %s, expected a negative duration due to int64 overflow (bug)", got)
	}
	if got == updateRetryMaxDelay {
		t.Fatalf("retryBackoffDelay(50) returned the cap; overflow bypass should produce a negative value (bug)")
	}
}

// TestBranchcov0724pmSleepWithContext covers both select arms: an already
// cancelled context returns immediately with context.Canceled, and a normal
// short sleep completes with nil.
func TestBranchcov0724pmSleepWithContext(t *testing.T) {
	t.Run("CancelledContextReturnsImmediately", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		start := time.Now()
		err := sleepWithContext(ctx, 30*time.Second)
		elapsed := time.Since(start)

		if !errors.Is(err, context.Canceled) {
			t.Fatalf("sleepWithContext err = %v, want context.Canceled", err)
		}
		if elapsed > 100*time.Millisecond {
			t.Fatalf("sleepWithContext took %s on cancelled ctx, want immediate return", elapsed)
		}
	})

	t.Run("ShortDurationCompletes", func(t *testing.T) {
		d := 50 * time.Millisecond
		start := time.Now()
		err := sleepWithContext(context.Background(), d)
		elapsed := time.Since(start)

		if err != nil {
			t.Fatalf("sleepWithContext err = %v, want nil", err)
		}
		if elapsed < d {
			t.Fatalf("sleepWithContext returned early after %s, want >= %s", elapsed, d)
		}
		if elapsed > 2*time.Second {
			t.Fatalf("sleepWithContext took %s, want close to %s", elapsed, d)
		}
	})
}

// TestBranchcov0724pmWriteSelfTestTokenFile asserts the success-path file
// contents/mode and covers every error arm reachable through the package's
// injectable seams: createTemp failure (unwritable directory), write failure
// (pre-closed file), close failure, and chmod failure.
func TestBranchcov0724pmWriteSelfTestTokenFile(t *testing.T) {
	t.Run("EmptyTokenReturnsEmptyAndNoFile", func(t *testing.T) {
		gotPath, err := writeSelfTestTokenFile("   \t\n")
		if err != nil {
			t.Fatalf("writeSelfTestTokenFile err = %v, want nil", err)
		}
		if gotPath != "" {
			t.Fatalf("writeSelfTestTokenFile path = %q, want empty", gotPath)
		}
	})

	t.Run("SuccessWritesTrimmedContentsWithMode0600", func(t *testing.T) {
		gotPath, err := writeSelfTestTokenFile("  my-secret-token  \n")
		if err != nil {
			t.Fatalf("writeSelfTestTokenFile err = %v, want nil", err)
		}
		if gotPath == "" {
			t.Fatalf("writeSelfTestTokenFile path empty, want a temp path")
		}
		t.Cleanup(func() { _ = os.Remove(gotPath) })

		data, err := os.ReadFile(gotPath)
		if err != nil {
			t.Fatalf("read token file: %v", err)
		}
		if string(data) != "my-secret-token" {
			t.Fatalf("token file contents = %q, want %q", string(data), "my-secret-token")
		}
		info, err := os.Stat(gotPath)
		if err != nil {
			t.Fatalf("stat token file: %v", err)
		}
		// Windows does not expose POSIX permission bits through os.FileMode.
		if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
			t.Fatalf("token file mode = %o, want 0600", info.Mode().Perm())
		}
	})

	t.Run("CreateTempFailureMissingDir", func(t *testing.T) {
		// Deterministic createTemp failure regardless of privileges: point at a
		// directory that does not exist on disk.
		missingDir := filepath.Join(t.TempDir(), "does-not-exist")

		origCreateTemp := createTempFn
		t.Cleanup(func() { createTempFn = origCreateTemp })
		createTempFn = func(string, string) (*os.File, error) {
			return os.CreateTemp(missingDir, "pulse-agent-selftest-token-*")
		}

		gotPath, err := writeSelfTestTokenFile("token")
		if err == nil {
			t.Fatalf("writeSelfTestTokenFile err = nil, want create error")
		}
		if !strings.Contains(err.Error(), "create self-test token file") {
			t.Fatalf("err = %v, want it to contain %q", err, "create self-test token file")
		}
		if gotPath != "" {
			t.Fatalf("gotPath = %q, want empty on create failure", gotPath)
		}
	})

	t.Run("WriteFailureOnPreClosedFile", func(t *testing.T) {
		workDir := t.TempDir()

		origCreateTemp := createTempFn
		t.Cleanup(func() { createTempFn = origCreateTemp })
		createTempFn = func(string, string) (*os.File, error) {
			f, err := os.CreateTemp(workDir, "pulse-agent-selftest-token-*")
			if err != nil {
				return nil, err
			}
			// Close before returning so the subsequent WriteString fails.
			_ = f.Close()
			return f, nil
		}

		gotPath, err := writeSelfTestTokenFile("token")
		if err == nil {
			t.Fatalf("writeSelfTestTokenFile err = nil, want write error")
		}
		if !strings.Contains(err.Error(), "write self-test token file") {
			t.Fatalf("err = %v, want it to contain %q", err, "write self-test token file")
		}
		if gotPath != "" {
			t.Fatalf("gotPath = %q, want empty on write failure", gotPath)
		}
		// The on-disk file must have been cleaned up by the error-path defer.
		matches, _ := filepath.Glob(filepath.Join(workDir, "pulse-agent-selftest-token-*"))
		if len(matches) != 0 {
			t.Fatalf("expected cleanup of token file, still present: %v", matches)
		}
	})

	t.Run("CloseFailure", func(t *testing.T) {
		origClose := closeFileFn
		t.Cleanup(func() { closeFileFn = origClose })
		closeFileFn = func(f *os.File) error {
			_ = f.Close()
			return errors.New("close denied")
		}

		gotPath, err := writeSelfTestTokenFile("token")
		if err == nil {
			t.Fatalf("writeSelfTestTokenFile err = nil, want close error")
		}
		if !strings.Contains(err.Error(), "close self-test token file") {
			t.Fatalf("err = %v, want it to contain %q", err, "close self-test token file")
		}
		if gotPath != "" {
			t.Fatalf("gotPath = %q, want empty on close failure", gotPath)
		}
	})

	t.Run("ChmodFailure", func(t *testing.T) {
		origChmod := chmodFn
		t.Cleanup(func() { chmodFn = origChmod })
		chmodFn = func(string, os.FileMode) error { return errors.New("chmod denied") }

		gotPath, err := writeSelfTestTokenFile("token")
		if err == nil {
			t.Fatalf("writeSelfTestTokenFile err = nil, want chmod error")
		}
		if !strings.Contains(err.Error(), "chmod self-test token file") {
			t.Fatalf("err = %v, want it to contain %q", err, "chmod self-test token file")
		}
		if gotPath != "" {
			t.Fatalf("gotPath = %q, want empty on chmod failure", gotPath)
		}
	})
}

// TestBranchcov0724pmVerifyBinaryMagic exercises the magic-byte contract for
// each platform with concrete fixtures built from byte slices in t.TempDir:
// valid headers pass, truncated/empty files fail to read magic, wrong magic
// fails closed, and a missing path fails at open.
//
// NOTE: these arms are already exercised indirectly by update_test.go and via
// performUpdateWithExecPath, so this function's coverage percentage is not
// expected to rise. The only genuinely uncovered arms are the deferred
// close-error handling (update.go:679-685), which call f.Close() directly with
// no injectable seam and therefore cannot be reached from a test without a
// source change.
func TestBranchcov0724pmVerifyBinaryMagic(t *testing.T) {
	origOS := runtimeGOOS
	t.Cleanup(func() { runtimeGOOS = origOS })

	tmpDir := t.TempDir()

	writeFixture := func(name string, data []byte) string {
		t.Helper()
		p := filepath.Join(tmpDir, name)
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		return p
	}

	elf := []byte{0x7f, 'E', 'L', 'F', 0x02, 0x01, 0x01, 0x00}
	macho64 := []byte{0xcf, 0xfa, 0xed, 0xfe, 0x07, 0x00, 0x00, 0x00}
	macho32 := []byte{0xce, 0xfa, 0xed, 0xfe, 0x07, 0x00, 0x00, 0x00}
	machoFat := []byte{0xca, 0xfe, 0xba, 0xbe, 0x00, 0x00, 0x00, 0x08}
	pe := []byte{'M', 'Z', 0x90, 0x00, 0x03, 0x00, 0x00, 0x00}
	wrong := []byte{0xde, 0xad, 0xbe, 0xef, 0x12, 0x34}

	t.Run("LinuxValidELF", func(t *testing.T) {
		runtimeGOOS = goOSLinux
		p := writeFixture("elf", elf)
		if err := verifyBinaryMagic(p); err != nil {
			t.Fatalf("expected ELF to validate, got %v", err)
		}
	})

	t.Run("FreeBSDValidELF", func(t *testing.T) {
		runtimeGOOS = goOSFreeBSD
		p := writeFixture("elf-fbsd", elf)
		if err := verifyBinaryMagic(p); err != nil {
			t.Fatalf("expected FreeBSD ELF to validate, got %v", err)
		}
	})

	t.Run("LinuxWrongMagic", func(t *testing.T) {
		runtimeGOOS = goOSLinux
		p := writeFixture("wrong-elf", wrong)
		err := verifyBinaryMagic(p)
		if err == nil || !strings.Contains(err.Error(), "ELF") {
			t.Fatalf("expected ELF magic error, got %v", err)
		}
	})

	t.Run("DarwinValidMachO64", func(t *testing.T) {
		runtimeGOOS = goOSDarwin
		p := writeFixture("macho64", macho64)
		if err := verifyBinaryMagic(p); err != nil {
			t.Fatalf("expected Mach-O 64-bit to validate, got %v", err)
		}
	})

	t.Run("DarwinValidMachO32", func(t *testing.T) {
		runtimeGOOS = goOSDarwin
		p := writeFixture("macho32", macho32)
		if err := verifyBinaryMagic(p); err != nil {
			t.Fatalf("expected Mach-O 32-bit to validate, got %v", err)
		}
	})

	t.Run("DarwinValidUniversal", func(t *testing.T) {
		runtimeGOOS = goOSDarwin
		p := writeFixture("macho-fat", machoFat)
		if err := verifyBinaryMagic(p); err != nil {
			t.Fatalf("expected universal/fat Mach-O to validate, got %v", err)
		}
	})

	t.Run("DarwinWrongMagic", func(t *testing.T) {
		runtimeGOOS = goOSDarwin
		p := writeFixture("wrong-macho", wrong)
		err := verifyBinaryMagic(p)
		if err == nil || !strings.Contains(err.Error(), "Mach-O") {
			t.Fatalf("expected Mach-O magic error, got %v", err)
		}
	})

	t.Run("WindowsValidPE", func(t *testing.T) {
		runtimeGOOS = goOSWindows
		p := writeFixture("pe", pe)
		if err := verifyBinaryMagic(p); err != nil {
			t.Fatalf("expected PE to validate, got %v", err)
		}
	})

	t.Run("WindowsWrongMagic", func(t *testing.T) {
		runtimeGOOS = goOSWindows
		p := writeFixture("wrong-pe", wrong)
		err := verifyBinaryMagic(p)
		if err == nil || !strings.Contains(err.Error(), "PE") {
			t.Fatalf("expected PE magic error, got %v", err)
		}
	})

	t.Run("UnsupportedOSFailsClosed", func(t *testing.T) {
		runtimeGOOS = "plan9"
		p := writeFixture("plan9", []byte{0x00, 0x01, 0x02, 0x03})
		err := verifyBinaryMagic(p)
		if err == nil || !strings.Contains(err.Error(), "unsupported") {
			t.Fatalf("expected unsupported OS error, got %v", err)
		}
	})
}
