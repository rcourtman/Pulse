package api

import (
	"container/heap"
	"sync"
	"time"
)

// agentConfigFetchAuditInterval re-records an unchanged successful config
// delivery at most this often per agent, so the audit trail still shows
// continued access.
const agentConfigFetchAuditInterval = 24 * time.Hour

// maxAgentConfigFetchAudits bounds remembered deliveries. At capacity, only an
// expired entry can be replaced. An unremembered delivery is always audited;
// churn must not evict recent agents and turn their minute polls into writes.
const maxAgentConfigFetchAudits = 4096

// agentConfigFetchAuditTracker decides which successful agent config fetches
// are audit events. Agents poll their config every minute, and recording each
// poll made agent_config_fetch over 99.9% of audit rows (about 1,440 a day per
// agent), burying the logins and failures the log exists to show. Failed
// fetches are always audited and never pass through here.
type agentConfigFetchAuditTracker struct {
	mu     sync.Mutex
	last   map[agentConfigFetchAuditKey]*agentConfigFetchAuditEntry
	oldest agentConfigFetchAuditHeap
}

type agentConfigFetchAuditKey struct {
	orgID   string
	agentID string
}

type agentConfigFetchAuditEntry struct {
	key        agentConfigFetchAuditKey
	tokenID    string
	configHash string
	auditedAt  time.Time
	index      int
}

// The expiry index avoids a full-map scan for every new key at capacity. It
// contains exactly the remembered entries, including after token/config/daily
// updates. Ordering by audit time also handles out-of-order observations.
type agentConfigFetchAuditHeap []*agentConfigFetchAuditEntry

func (h agentConfigFetchAuditHeap) Len() int { return len(h) }
func (h agentConfigFetchAuditHeap) Less(i, j int) bool {
	return h[i].auditedAt.Before(h[j].auditedAt)
}
func (h agentConfigFetchAuditHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index, h[j].index = i, j
}
func (h *agentConfigFetchAuditHeap) Push(value any) {
	entry := value.(*agentConfigFetchAuditEntry)
	entry.index = len(*h)
	*h = append(*h, entry)
}
func (h *agentConfigFetchAuditHeap) Pop() any {
	last := len(*h) - 1
	entry := (*h)[last]
	(*h)[last] = nil
	*h = (*h)[:last]
	entry.index = -1
	return entry
}

func newAgentConfigFetchAuditTracker() *agentConfigFetchAuditTracker {
	return &agentConfigFetchAuditTracker{last: make(map[agentConfigFetchAuditKey]*agentConfigFetchAuditEntry)}
}

// observe records a successful delivery and reports whether it is new audit
// information, with the reason: the agent's first delivery since startup, a
// different token or delivered config, or an unchanged delivery last audited
// at least agentConfigFetchAuditInterval ago. At capacity, unremembered agents
// are audited with reason "capacity" on every delivery until a slot expires.
// A nil tracker audits every delivery.
func (t *agentConfigFetchAuditTracker) observe(orgID, agentID, tokenID, configHash string, now time.Time) (string, bool) {
	if t == nil {
		return "", true
	}
	key := agentConfigFetchAuditKey{orgID: orgID, agentID: agentID}

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
	if seen {
		previous.tokenID, previous.configHash, previous.auditedAt = tokenID, configHash, now
		heap.Fix(&t.oldest, previous.index)
		return reason, true
	}
	if len(t.last) >= maxAgentConfigFetchAudits {
		stale := t.oldest[0]
		if now.Sub(stale.auditedAt) < agentConfigFetchAuditInterval {
			return "capacity", true
		}
		heap.Pop(&t.oldest)
		delete(t.last, stale.key)
	}
	if t.last == nil {
		t.last = make(map[agentConfigFetchAuditKey]*agentConfigFetchAuditEntry)
	}
	entry := &agentConfigFetchAuditEntry{key: key, tokenID: tokenID, configHash: configHash, auditedAt: now}
	heap.Push(&t.oldest, entry)
	t.last[key] = entry
	return reason, true
}
