//go:build linux

package api

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func trackingJoin(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("tracking fixture goroutine did not join")
	}
}

func TestSessionTrackingRepeatedRevocationWaitsForBothStores(t *testing.T) {
	for _, disk := range []string{"session", "csrf"} {
		t.Run(disk, func(t *testing.T) {
			router, tokens := trackingDiskFixture(t)
			var resume func() []byte
			if disk == "session" {
				resume = pauseSessionDiskWrite(t, router.sessionStore, router.sessionStore.save)
			} else {
				resume = pauseCSRFDiskWrite(t, router.csrfStore, router.csrfStore.save)
			}
			first := make(chan struct{})
			go func() { defer close(first); InvalidateUserSessions("retired-user") }()
			// Observe the live withdrawal before starting the no-match caller.
			deadline := time.After(2 * time.Second)
			tick := time.NewTicker(time.Millisecond)
			defer tick.Stop()
			for router.sessionStore.ValidateSession("target-00") || router.csrfStore.ValidateCSRFToken("target-00", tokens["target-00"][0]) {
				select {
				case <-tick.C:
				case <-deadline:
					resume()
					trackingJoin(t, first)
					t.Fatal("both live stores did not withdraw before pending persistence")
				}
			}
			second, started := make(chan struct{}), make(chan struct{})
			go func() { defer close(second); close(started); InvalidateUserSessions("retired-user") }()
			<-started
			select {
			case <-second:
				t.Error("no-match account revocation bypassed a pending store save")
			case <-time.After(100 * time.Millisecond):
			}
			select {
			case <-first:
				t.Error("first account revocation returned before both saves")
			default:
			}
			resume()
			trackingJoin(t, first)
			trackingJoin(t, second)
			loaded := NewSessionStore(router.sessionStore.dataPath)
			t.Cleanup(loaded.Shutdown)
			csrf := loadCSRFDiskStore(t, router.csrfStore.dataPath)
			if loaded.ValidateSession("target-00") || csrf.ValidateCSRFToken("target-00", tokens["target-00"][0]) || !loaded.ValidateSession("control-warm") || !csrf.ValidateCSRFToken("control-warm", tokens["control-warm"][0]) {
				t.Fatal("older snapshot restored authority or removed an unrelated account")
			}
		})
	}
}

func TestSessionTrackingTrackedOnlyAndPrunedSessions(t *testing.T) {
	router, tokens := trackingDiskFixture(t)
	// No persisted session exists for this tracked entry, but its CSRF set
	// remains part of account-wide revocation, including after restart.
	InvalidateUserSessions("legacy-user")
	if GetSessionUsername("legacy-only") != "" {
		t.Fatal("tracked-only attribution survived withdrawal")
	}
	csrf := loadCSRFDiskStore(t, router.csrfStore.dataPath)
	if csrf.ValidateCSRFToken("legacy-only", tokens["legacy-only"][0]) || !csrf.ValidateCSRFToken("control-warm", tokens["control-warm"][0]) {
		t.Fatal("tracked-only withdrawal lost its CSRF or account boundary")
	}
	// An already logged-out entry is pruned before applying the ten-session
	// cap; adding one replacement must not evict the next valid session.
	router.sessionStore.DeleteSession("target-00")
	TrackUserSession("retired-user", "target-new")
	for i := 1; i < maxSessionsPerUser; i++ {
		raw := fmt.Sprintf("target-%02d", i)
		if !router.sessionStore.ValidateSession(raw) || !router.csrfStore.ValidateCSRFToken(raw, tokens[raw][0]) {
			t.Fatal("stale pruning evicted a valid session")
		}
	}
	sessionsMu.RLock()
	count := len(allSessions["retired-user"])
	sessionsMu.RUnlock()
	if count != maxSessionsPerUser || GetSessionUsername("target-new") != "retired-user" {
		t.Fatal("pruned tracking did not retain the existing session bound")
	}
}

func TestSessionTrackingColdFallbackAndConcurrentEviction(t *testing.T) {
	router, _ := trackingDiskFixture(t)
	for round := 0; round < 24; round++ {
		raw := fmt.Sprintf("cold-race-%02d", round)
		router.sessionStore.CreateSession(raw, time.Hour, "fixture", "192.0.2.1", "cold-user")
		start := make(chan struct{})
		var wg sync.WaitGroup
		for reader := 0; reader < 8; reader++ {
			wg.Add(1)
			go func() { defer wg.Done(); <-start; GetSessionUsername(raw) }()
		}
		wg.Add(1)
		go func() { defer wg.Done(); <-start; InvalidateUserSessions("cold-user") }()
		close(start)
		wg.Wait()
		if router.sessionStore.ValidateSession(raw) || GetSessionUsername(raw) != "" {
			t.Fatal("cold rehydration cached a withdrawn session snapshot")
		}
	}
	// Concurrent admissions have an unspecified order, but the final ten and
	// only those ten must retain both live and restart authority.
	const total = 24
	tokens := make(map[string]string)
	for i := 0; i < total; i++ {
		raw := fmt.Sprintf("parallel-%02d", i)
		router.sessionStore.CreateSession(raw, time.Hour, "fixture", "192.0.2.1", "parallel-user")
		tokens[raw] = router.csrfStore.GenerateCSRFToken(raw)
	}
	start := make(chan struct{})
	var wg sync.WaitGroup
	for raw := range tokens {
		wg.Add(1)
		go func() { defer wg.Done(); <-start; TrackUserSession("parallel-user", raw) }()
	}
	close(start)
	wg.Wait()
	sessionsMu.RLock()
	tracked := make(map[string]bool)
	for _, raw := range allSessions["parallel-user"] {
		tracked[raw] = true
	}
	sessionsMu.RUnlock()
	if len(tracked) != maxSessionsPerUser {
		t.Fatal("concurrent eviction lost the ten-session bound")
	}
	loaded := NewSessionStore(router.sessionStore.dataPath)
	t.Cleanup(loaded.Shutdown)
	csrf := loadCSRFDiskStore(t, router.csrfStore.dataPath)
	for raw, token := range tokens {
		if router.sessionStore.ValidateSession(raw) != tracked[raw] || router.csrfStore.ValidateCSRFToken(raw, token) != tracked[raw] || loaded.ValidateSession(raw) != tracked[raw] || csrf.ValidateCSRFToken(raw, token) != tracked[raw] {
			t.Fatal("concurrent eviction crossed its live/persisted session-CSRF boundary")
		}
	}
}

func TestSessionTrackingSaveFailureKeepsLiveWithdrawal(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("requires ordinary-user filesystem permissions")
	}
	router, tokens := trackingDiskFixture(t)
	dir := router.sessionStore.dataPath
	read := func(name string) []byte {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	sessionBefore, csrfBefore := read("sessions.json"), read("csrf_tokens.json")
	t.Cleanup(func() { _ = os.Chmod(dir, 0700) })
	if err := os.Chmod(dir, 0500); err != nil {
		t.Fatal(err)
	}
	InvalidateUserSessions("retired-user")
	if router.sessionStore.ValidateSession("target-00") || router.csrfStore.ValidateCSRFToken("target-00", tokens["target-00"][0]) || GetSessionUsername("target-00") != "" || GetSessionUsername("control-warm") != "synthetic-admin" {
		t.Fatal("failed saves restored live authority or blocked the unrelated account")
	}
	if !bytes.Equal(sessionBefore, read("sessions.json")) || !bytes.Equal(csrfBefore, read("csrf_tokens.json")) {
		t.Fatal("failed saves changed previous disk bytes")
	}
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	// A second no-match revocation must recover BOTH stores after repair.
	InvalidateUserSessions("retired-user")
	loaded := NewSessionStore(dir)
	t.Cleanup(loaded.Shutdown)
	csrf := loadCSRFDiskStore(t, dir)
	if loaded.ValidateSession("target-00") || csrf.ValidateCSRFToken("target-00", tokens["target-00"][0]) || !loaded.ValidateSession("control-warm") || !csrf.ValidateCSRFToken("control-warm", tokens["control-warm"][0]) {
		t.Fatal("no-match recovery failed to retire both saved authority sets")
	}
}
