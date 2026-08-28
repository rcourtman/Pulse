package monitoring

import (
	"fmt"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// revokeAPIToken removes one credential from both live and restart-time state.
// Agent removal itself remains authoritative when persistence is unavailable,
// but the credential must remain live if its reduced inventory cannot commit;
// otherwise a restart would silently reactivate a token Pulse reported revoked.
func (m *Monitor) revokeAPIToken(tokenID string) (*config.APITokenRecord, error) {
	tokenID = strings.TrimSpace(tokenID)
	if tokenID == "" {
		return nil, nil
	}
	if m == nil || m.config == nil {
		return nil, fmt.Errorf("configuration unavailable")
	}

	config.Mu.Lock()
	defer config.Mu.Unlock()

	previousTokens := append([]config.APITokenRecord(nil), m.config.APITokens...)
	removed := m.config.RemoveAPIToken(tokenID)
	if removed == nil {
		return nil, nil
	}
	m.config.SortAPITokens()

	if m.persistence != nil {
		if err := m.persistence.SaveAPITokens(m.config.APITokens); err != nil {
			m.config.APITokens = previousTokens
			m.config.SortAPITokens()
			return nil, fmt.Errorf("persist API token revocation: %w", err)
		}
	}

	return removed, nil
}
