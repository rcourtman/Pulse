package api

import (
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// Routine authorization outcomes — an unauthenticated caller reaching a
// protected route, or an authenticated non-admin reaching an admin one — are
// the access control working as designed, not faults. Logging each one at warn
// level meant a healthy instance could not produce a quiet log: a single idle
// non-admin browser tab emitted a steady stream of "Non-admin user attempted to
// access admin endpoint" lines, which #1601's rc.9 reporter read as an RBAC
// regression.
//
// Individually these are debug-level facts. The thing actually worth warning an
// operator about is an abnormal *rate* of refusals from one caller, which is
// what credential stuffing and endpoint probing look like. So denials are
// counted per caller per window and escalated once when they cross the
// threshold, instead of every refusal shouting on its own.
const (
	authDenialWindow        = time.Minute
	authDenialWarnThreshold = 20
	// Cap on tracked callers. A flood of spoofed forwarded-for values must not
	// grow this map without bound; once full, untracked callers still log at
	// debug level, they just do not contribute to the escalation signal.
	authDenialMaxTracked = 4096
)

type authDenialCounter struct {
	windowStart time.Time
	count       int
	warned      bool
}

var (
	authDenialMu sync.Mutex
	authDenials  = make(map[string]*authDenialCounter)
	// Test seam: the tracker is time-based, and tests must not sleep for a
	// minute to exercise window rollover.
	authDenialNow = time.Now
)

// authDenialKey identifies the caller a refusal is attributed to. A known
// username survives IP rotation, so it is preferred; otherwise fall back to the
// client IP.
func authDenialKey(r *http.Request, identity string) string {
	if identity != "" {
		return "user:" + identity
	}
	if r == nil {
		return "ip:unknown"
	}
	return "ip:" + GetClientIP(r)
}

// recordAuthDenial counts one refusal and reports whether this is the call that
// pushed the caller over the probing threshold. It returns true at most once per
// caller per window, so the escalation itself cannot become the new log spam.
func recordAuthDenial(key string) (crossed bool, count int) {
	now := authDenialNow()

	authDenialMu.Lock()
	defer authDenialMu.Unlock()

	entry, ok := authDenials[key]
	if !ok || now.Sub(entry.windowStart) >= authDenialWindow {
		if !ok {
			pruneAuthDenialsLocked(now)
			entry = &authDenialCounter{}
			authDenials[key] = entry
		}
		entry.windowStart = now
		entry.count = 0
		entry.warned = false
	}

	entry.count++
	if entry.count >= authDenialWarnThreshold && !entry.warned {
		entry.warned = true
		return true, entry.count
	}
	return false, entry.count
}

// pruneAuthDenialsLocked drops counters whose window has closed. Callers must
// hold authDenialMu.
func pruneAuthDenialsLocked(now time.Time) {
	for key, entry := range authDenials {
		if now.Sub(entry.windowStart) >= authDenialWindow {
			delete(authDenials, key)
		}
	}
	if len(authDenials) < authDenialMaxTracked {
		return
	}
	// Still over the cap with every window live: evict the oldest windows so a
	// spoofed-key flood cannot pin newer callers out of the map.
	keys := make([]string, 0, len(authDenials))
	for key := range authDenials {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		return authDenials[keys[i]].windowStart.Before(authDenials[keys[j]].windowStart)
	})
	for _, key := range keys[:len(authDenials)-authDenialMaxTracked/2] {
		delete(authDenials, key)
	}
}

// logAuthDenial records an expected authorization refusal. The refusal itself
// goes to debug; only a caller crossing the probing threshold warns.
func logAuthDenial(r *http.Request, identity, message string, fields map[string]string) {
	event := log.Debug()
	if r != nil {
		event = event.
			Str("ip", r.RemoteAddr).
			Str("path", r.URL.Path).
			Str("method", r.Method)
	}
	if identity != "" {
		event = event.Str("username", identity)
	}
	for _, name := range sortedFieldNames(fields) {
		event = event.Str(name, fields[name])
	}
	event.Msg(message)

	crossed, count := recordAuthDenial(authDenialKey(r, identity))
	if !crossed {
		return
	}

	warn := log.Warn().
		Int("denials", count).
		Dur("window", authDenialWindow)
	if r != nil {
		warn = warn.Str("ip", GetClientIP(r)).Str("path", r.URL.Path)
	}
	if identity != "" {
		warn = warn.Str("username", identity)
	}
	warn.Msg("Repeated authorization denials from one caller; possible endpoint probing")
}

func sortedFieldNames(fields map[string]string) []string {
	if len(fields) == 0 {
		return nil
	}
	names := make([]string, 0, len(fields))
	for name := range fields {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
