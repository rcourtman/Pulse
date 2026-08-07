package api

import (
	"bytes"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// captureAuthDenialLogs swaps the global logger for a buffer at debug level and
// resets the denial tracker, so each case starts from a known window.
func captureAuthDenialLogs(t *testing.T) *bytes.Buffer {
	t.Helper()

	var buf bytes.Buffer
	prevLogger := log.Logger
	prevLevel := zerolog.GlobalLevel()
	log.Logger = zerolog.New(&buf).Level(zerolog.DebugLevel)
	zerolog.SetGlobalLevel(zerolog.DebugLevel)

	authDenialMu.Lock()
	authDenials = make(map[string]*authDenialCounter)
	authDenialMu.Unlock()

	prevNow := authDenialNow
	t.Cleanup(func() {
		log.Logger = prevLogger
		zerolog.SetGlobalLevel(prevLevel)
		authDenialNow = prevNow
		authDenialMu.Lock()
		authDenials = make(map[string]*authDenialCounter)
		authDenialMu.Unlock()
	})

	return &buf
}

func countLevel(output, level string) int {
	return strings.Count(output, `"level":"`+level+`"`)
}

// A non-admin browsing a UI that still mounts an admin-only surface produces a
// steady trickle of refusals. Warning on each is what made #1601's rc.9 reporter
// read a working RBAC gate as a break, so ordinary refusals must stay at debug.
func TestAuthDenialBelowThresholdStaysDebug(t *testing.T) {
	buf := captureAuthDenialLogs(t)
	req := httptest.NewRequest("GET", "/api/connections", nil)

	for i := 0; i < authDenialWarnThreshold-1; i++ {
		logAuthDenial(req, "viewer", "Non-admin user attempted to access admin endpoint", nil)
	}

	output := buf.String()
	if got := countLevel(output, "debug"); got != authDenialWarnThreshold-1 {
		t.Fatalf("debug lines = %d, want %d", got, authDenialWarnThreshold-1)
	}
	if got := countLevel(output, "warn"); got != 0 {
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
	if got := countLevel(output, "warn"); got != 1 {
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
	if got := countLevel(buf.String(), "warn"); got != 1 {
		t.Fatalf("first window warn lines = %d, want 1", got)
	}

	// A caller that goes quiet and starts probing again later is a fresh signal,
	// not a permanently silenced one.
	authDenialNow = func() time.Time { return base.Add(authDenialWindow + time.Second) }
	for i := 0; i < authDenialWarnThreshold; i++ {
		logAuthDenial(req, "viewer", "denied", nil)
	}
	if got := countLevel(buf.String(), "warn"); got != 2 {
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

	if got := countLevel(buf.String(), "warn"); got != 1 {
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

	if got := countLevel(buf.String(), "warn"); got != 0 {
		t.Fatalf("warn lines = %d, want 0 — neither caller crossed the threshold", got)
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
