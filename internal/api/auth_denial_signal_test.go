package api

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// captureAuthDenialLogs resets the denial tracker and captures logs through the
// process-wide synchronized test sink, so each case starts from a known window.
func captureAuthDenialLogs(t *testing.T) *lockedLogBuffer {
	t.Helper()

	buf := captureTestLogs(t)

	authDenialMu.Lock()
	authDenials = make(map[string]*authDenialCounter)
	authDenialMu.Unlock()

	prevNow := authDenialNow
	t.Cleanup(func() {
		authDenialNow = prevNow
		authDenialMu.Lock()
		authDenials = make(map[string]*authDenialCounter)
		authDenialMu.Unlock()
	})

	return buf
}

func countLogEvents(t *testing.T, buf *lockedLogBuffer, level, message string) int {
	t.Helper()

	count := 0
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}
		var event struct {
			Level   string `json:"level"`
			Message string `json:"message"`
		}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("decode captured log event %q: %v", line, err)
		}
		if event.Level == level && event.Message == message {
			count++
		}
	}
	return count
}

// A non-admin browsing a UI that still mounts an admin-only surface produces a
// steady trickle of refusals. Warning on each is what made #1601's rc.9 reporter
// read a working RBAC gate as a break, so ordinary refusals must stay at debug.
func TestAuthDenialBelowThresholdStaysDebug(t *testing.T) {
	buf := captureAuthDenialLogs(t)
	req := httptest.NewRequest("GET", "/api/connections", nil)
	const message = "Non-admin user attempted to access admin endpoint"

	for i := 0; i < authDenialWarnThreshold-1; i++ {
		logAuthDenial(req, "viewer", message, nil)
	}

	if got := countLogEvents(t, buf, "debug", message); got != authDenialWarnThreshold-1 {
		t.Fatalf("debug lines = %d, want %d", got, authDenialWarnThreshold-1)
	}
	if got := countLogEvents(t, buf, "warn", "Repeated authorization denials from one caller; possible endpoint probing"); got != 0 {
		t.Fatalf("warn lines = %d, want 0 below the probing threshold", got)
	}
}

// The signal worth an operator's attention is the rate, not the individual
// refusal — and the escalation must not itself become the new spam.
func TestAuthDenialEscalatesOncePerWindow(t *testing.T) {
	buf := captureAuthDenialLogs(t)
	req := httptest.NewRequest("GET", "/api/connections", nil)

	for i := 0; i < authDenialWarnThreshold*3; i++ {
		logAuthDenial(req, "viewer", "Non-admin user attempted to access admin endpoint", nil)
	}

	output := buf.String()
	if got := countLogEvents(t, buf, "warn", "Repeated authorization denials from one caller; possible endpoint probing"); got != 1 {
		t.Fatalf("warn lines = %d, want exactly 1 for a single window", got)
	}
	if !strings.Contains(output, "possible endpoint probing") {
		t.Fatalf("escalation missing from output: %q", output)
	}
}

func TestAuthDenialWindowRolloverRearmsEscalation(t *testing.T) {
	buf := captureAuthDenialLogs(t)
	req := httptest.NewRequest("GET", "/api/connections", nil)

	base := time.Unix(1_700_000_000, 0)
	authDenialNow = func() time.Time { return base }
	for i := 0; i < authDenialWarnThreshold; i++ {
		logAuthDenial(req, "viewer", "denied", nil)
	}
	if got := countLogEvents(t, buf, "warn", "Repeated authorization denials from one caller; possible endpoint probing"); got != 1 {
		t.Fatalf("first window warn lines = %d, want 1", got)
	}

	// A caller that goes quiet and starts probing again later is a fresh signal,
	// not a permanently silenced one.
	authDenialNow = func() time.Time { return base.Add(authDenialWindow + time.Second) }
	for i := 0; i < authDenialWarnThreshold; i++ {
		logAuthDenial(req, "viewer", "denied", nil)
	}
	if got := countLogEvents(t, buf, "warn", "Repeated authorization denials from one caller; possible endpoint probing"); got != 2 {
		t.Fatalf("warn lines after rollover = %d, want 2", got)
	}
}

// Attribution follows the identity when there is one, so a probing account is
// still caught after rotating source addresses.
func TestAuthDenialTracksIdentityAcrossAddresses(t *testing.T) {
	buf := captureAuthDenialLogs(t)

	for i := 0; i < authDenialWarnThreshold; i++ {
		req := httptest.NewRequest("GET", "/api/connections", nil)
		req.RemoteAddr = "10.0.0." + string(rune('1'+i%9)) + ":5000"
		logAuthDenial(req, "viewer", "denied", nil)
	}

	if got := countLogEvents(t, buf, "warn", "Repeated authorization denials from one caller; possible endpoint probing"); got != 1 {
		t.Fatalf("warn lines = %d, want 1 — identity should aggregate across addresses", got)
	}
}

// Separate callers must not pool into one another's budget, or one busy client
// would silence the signal for everybody else.
func TestAuthDenialSeparatesCallers(t *testing.T) {
	buf := captureAuthDenialLogs(t)
	req := httptest.NewRequest("GET", "/api/connections", nil)

	for i := 0; i < authDenialWarnThreshold-1; i++ {
		logAuthDenial(req, "viewer-a", "denied", nil)
		logAuthDenial(req, "viewer-b", "denied", nil)
	}

	if got := countLogEvents(t, buf, "warn", "Repeated authorization denials from one caller; possible endpoint probing"); got != 0 {
		t.Fatalf("warn lines = %d, want 0 — neither caller crossed the threshold", got)
	}
}

// The API package can have background monitor goroutines logging while these
// tests run, and production handlers may record denials concurrently. Exercise
// that concurrency directly so an unsynchronized capture sink or tracker state
// fails deterministically under go test -race.
func TestAuthDenialConcurrentLoggingIsRaceFreeAndWarnsOnce(t *testing.T) {
	buf := captureAuthDenialLogs(t)

	base := time.Unix(1_700_000_000, 0)
	authDenialNow = func() time.Time { return base }
	const (
		identity = "concurrent-viewer"
		message  = "concurrent denial"
		attempts = authDenialWarnThreshold * 8
	)

	var wg sync.WaitGroup
	wg.Add(attempts)
	for i := 0; i < attempts; i++ {
		go func() {
			defer wg.Done()
			req := httptest.NewRequest("GET", "/api/connections", nil)
			logAuthDenial(req, identity, message, nil)
		}()
	}
	wg.Wait()

	if got := countLogEvents(t, buf, "debug", message); got != attempts {
		t.Fatalf("debug lines = %d, want %d", got, attempts)
	}
	if got := countLogEvents(t, buf, "warn", "Repeated authorization denials from one caller; possible endpoint probing"); got != 1 {
		t.Fatalf("warn lines = %d, want exactly 1 for concurrent denials in one window", got)
	}

	authDenialMu.Lock()
	entry := *authDenials["user:"+identity]
	authDenialMu.Unlock()
	if entry.count != attempts || !entry.warned {
		t.Fatalf("counter = {count:%d warned:%t}, want {count:%d warned:true}", entry.count, entry.warned, attempts)
	}
}

func TestAuthDenialPruneKeepsTrackedSetBounded(t *testing.T) {
	captureAuthDenialLogs(t)

	base := time.Unix(1_700_000_000, 0)
	authDenialNow = func() time.Time { return base }
	for i := 0; i < authDenialMaxTracked+64; i++ {
		req := httptest.NewRequest("GET", "/api/connections", nil)
		logAuthDenial(req, "spoofed-"+strings.Repeat("x", i%7)+string(rune(i)), "denied", nil)
	}

	authDenialMu.Lock()
	tracked := len(authDenials)
	authDenialMu.Unlock()

	if tracked > authDenialMaxTracked {
		t.Fatalf("tracked callers = %d, want <= %d", tracked, authDenialMaxTracked)
	}
}
