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

// agentTokenUsedByLiveResource reports whether any currently monitored agent
// module still authenticates with tokenID. Unified-agent installs legitimately
// share one credential across host, Docker, and Kubernetes reports, so removing
// one module must not revoke the credential out from under its siblings.
// Callers that are removing a resource should delete it from state first.
func (m *Monitor) agentTokenUsedByLiveResource(tokenID string) bool {
	tokenID = strings.TrimSpace(tokenID)
	if tokenID == "" || m == nil || m.state == nil {
		return false
	}
	for _, host := range m.state.GetHosts() {
		if strings.TrimSpace(host.TokenID) == tokenID {
			return true
		}
	}
	for _, host := range m.state.GetDockerHosts() {
		if strings.TrimSpace(host.TokenID) == tokenID {
			return true
		}
	}
	for _, cluster := range m.state.GetKubernetesClusters() {
		if strings.TrimSpace(cluster.TokenID) == tokenID {
			return true
		}
	}
	return false
}
