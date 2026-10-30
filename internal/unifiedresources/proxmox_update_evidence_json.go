package unifiedresources

import (
	"encoding/json"
	"time"
)

// MarshalJSON keeps an explicit update verdict atomic on incremental readers.
// Omitting a measured zero, a cleared reason or an absent origin would let a
// generic facet merge retain the preceding positive/stale observation instead.
// Guests and legacy sources without a verdict retain their existing payload.
func (p *ProxmoxData) MarshalJSON() ([]byte, error) {
	type plain ProxmoxData
	if p == nil || p.PendingUpdatesStatus == "" {
		return json.Marshal((*plain)(p))
	}
	return json.Marshal(struct {
		*plain
		PendingUpdates          int        `json:"pendingUpdates"`
		PendingUpdatesCheckedAt *time.Time `json:"pendingUpdatesCheckedAt"`
		PendingUpdatesReason    string     `json:"pendingUpdatesReason"`
	}{
		plain:                   (*plain)(p),
		PendingUpdates:          p.PendingUpdates,
		PendingUpdatesCheckedAt: p.PendingUpdatesCheckedAt,
		PendingUpdatesReason:    p.PendingUpdatesReason,
	})
}
