package hostagent

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
)

const (
	guestExecConfigLimit     = 64 << 10
	guestExecCooldown        = time.Minute
	maxGuestExecGuardEntries = 4096
)

type guestExecEntry struct {
	done  chan struct{}
	until time.Time
}

type guestExecGuard struct {
	mu         sync.Mutex
	entries    map[string]guestExecEntry
	readConfig func(context.Context, string) ([]byte, error)
	now        func() time.Time
}

// Shared across reconnects and CommandClients in this agent process. This is
// not a lock shared with the Pulse HTTP poller, other processes, or PVE/QGA.
var localGuestExecGuard = newGuestExecGuard(readLocalGuestExecConfig)

func newGuestExecGuard(readConfig func(context.Context, string) ([]byte, error)) *guestExecGuard {
	return &guestExecGuard{entries: make(map[string]guestExecEntry), readConfig: readConfig, now: time.Now}
}

func canonicalGuestExecVMID(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if !safeTargetIDPattern.MatchString(raw) {
		return "", false
	}
	id, err := strconv.ParseUint(raw, 10, 32)
	if err != nil || id == 0 {
		return "", false
	}
	return strconv.FormatUint(id, 10), true
}

// Wait only for another local command, never poll QGA or retry a dispatched
// command. A waiter rechecks cooldown and the authoritative config on admission.
func (g *guestExecGuard) acquire(ctx context.Context, vmid string) (func(bool), error) {
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		g.mu.Lock()
		now := g.now()
		entry, exists := g.entries[vmid]
		if entry.done != nil {
			done := entry.done
			g.mu.Unlock()
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-done:
				continue
			}
		}
		if now.Before(entry.until) {
			g.mu.Unlock()
			return nil, agentexec.GuestExecDeferred(agentexec.GuestExecCooldown)
		}
		if !exists && len(g.entries) >= maxGuestExecGuardEntries {
			for key, e := range g.entries {
				if e.done == nil && !now.Before(e.until) {
					delete(g.entries, key)
				}
			}
			if len(g.entries) >= maxGuestExecGuardEntries {
				g.mu.Unlock()
				return nil, agentexec.GuestExecDeferred(agentexec.GuestExecCapacity)
			}
		}
		done := make(chan struct{})
		g.entries[vmid] = guestExecEntry{done: done}
		g.mu.Unlock()
		return func(uncertain bool) {
			g.mu.Lock()
			defer g.mu.Unlock()
			if uncertain {
				g.entries[vmid] = guestExecEntry{until: g.now().Add(guestExecCooldown)}
			} else {
				delete(g.entries, vmid)
			}
			close(done)
		}, nil
	}
}

func (g *guestExecGuard) verifyUnlocked(ctx context.Context, vmid string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	config, err := g.readConfig(ctx, vmid)
	if err != nil {
		return agentexec.GuestExecDeferred(agentexec.GuestExecLockUnverified)
	}
	lock, valid := guestExecConfigLock(config)
	if !valid {
		return agentexec.GuestExecDeferred(agentexec.GuestExecLockUnverified)
	}
	if lock != "" {
		return agentexec.GuestExecDeferred(agentexec.GuestExecVMLocked)
	}
	return nil
}

// qm config --current emits one key/value dictionary, not pending/snapshot
// sections. Reject empty, malformed, oversized or duplicate/case-conflicting
// fields rather than allowing last-key-wins to erase a lock.
func guestExecConfigLock(config []byte) (string, bool) {
	if len(config) == 0 || len(config) > guestExecConfigLimit || !utf8.Valid(config) {
		return "", false
	}
	fields := make(map[string]bool)
	lock := ""
	for _, line := range strings.Split(string(config), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok || key == "" || strings.TrimSpace(key) != key || fields[key] {
			return "", false
		}
		for _, c := range key {
			if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_' || c == '-') {
				return "", false
			}
		}
		fields[key] = true
		if key == "lock" {
			lock = strings.TrimSpace(value)
			if lock == "" {
				return "", false
			}
		}
	}
	return lock, len(fields) > 0
}
