package api

import (
	"sync"
	"time"
)

// agentConfigFetchAuditInterval re-records an unchanged successful config
// delivery at most this often per agent, so the audit trail still shows
// continued access.
const agentConfigFetchAuditInterval = 24 * time.Hour

// maxAgentConfigFetchAudits bounds remembered deliveries. Beyond it, entries
// older than agentConfigFetchAuditInterval are dropped; they would be
// re-recorded on their next fetch anyway.
const maxAgentConfigFetchAudits = 4096

// agentConfigFetchAuditTracker decides which successful agent config fetches
// are audit events. Agents poll their config every minute, and recording each
// poll made agent_config_fetch over 99.9% of audit rows (about 1,440 a day per
// agent), burying the logins and failures the log exists to show. Failed
// fetches are always audited and never pass through here.
type agentConfigFetchAuditTracker struct {
	mu   sync.Mutex
	last map[agentConfigFetchAuditKey]agentConfigFetchAuditEntry
}

type agentConfigFetchAuditKey struct {
	orgID   string
	agentID string
}

type agentConfigFetchAuditEntry struct {
	tokenID    string
	configHash string
	auditedAt  time.Time
}

func newAgentConfigFetchAuditTracker() *agentConfigFetchAuditTracker {
	return &agentConfigFetchAuditTracker{last: make(map[agentConfigFetchAuditKey]agentConfigFetchAuditEntry)}
}

// observe records a successful delivery and reports whether it is new audit
// information, with the reason: the agent's first delivery since startup, a
// different token or delivered config, or an unchanged delivery last audited
// at least agentConfigFetchAuditInterval ago. A nil tracker audits every
// delivery.
func (t *agentConfigFetchAuditTracker) observe(orgID, agentID, tokenID, configHash string, now time.Time) (string, bool) {
	if t == nil {
		return "", true
	}
	key := agentConfigFetchAuditKey{orgID: orgID, agentID: agentID}
	entry := agentConfigFetchAuditEntry{tokenID: tokenID, configHash: configHash, auditedAt: now}

	t.mu.Lock()
	defer t.mu.Unlock()
	previous, seen := t.last[key]
	reason := ""
	switch {
	case !seen:
		reason = "first_since_start"
	case previous.tokenID != tokenID:
		reason = "token_changed"
	case previous.configHash != configHash:
		reason = "config_changed"
	case now.Sub(previous.auditedAt) >= agentConfigFetchAuditInterval:
		reason = "daily"
	default:
		return "", false
	}
	if !seen && len(t.last) >= maxAgentConfigFetchAudits {
		for staleKey, stale := range t.last {
			if now.Sub(stale.auditedAt) >= agentConfigFetchAuditInterval {
				delete(t.last, staleKey)
			}
		}
	}
	t.last[key] = entry
	return reason, true
}
