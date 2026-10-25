package monitoring

import (
	"fmt"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rs/zerolog/log"
)

// APITokenInventory is one token store a monitor's agents authenticate
// with, together with the persistence that commits changes to it.
type APITokenInventory struct {
	Config      *config.Config
	Persistence *config.ConfigPersistence
	// OrgID, when set, limits the inventory to records bound only to that
	// organization: the instance inventory is shared by every org.
	OrgID string
}

// Owns reports whether record belongs to this inventory's monitor.
func (inv APITokenInventory) Owns(record *config.APITokenRecord) bool {
	return record != nil && (inv.OrgID == "" || record.BoundOnlyToOrg(inv.OrgID))
}

func (inv APITokenInventory) findLocked(tokenID string) *config.APITokenRecord {
	if inv.Config == nil {
		return nil
	}
	for i := range inv.Config.APITokens {
		if record := &inv.Config.APITokens[i]; record.ID == tokenID && inv.Owns(record) {
			return record
		}
	}
	return nil
}

// FindAPITokenLocked returns the live record for tokenID and the inventory
// holding it, so a change can be persisted where the record lives. The
// caller holds config.Mu.
func FindAPITokenLocked(inventories []APITokenInventory, tokenID string) (*config.APITokenRecord, APITokenInventory, bool) {
	for _, inventory := range inventories {
		if record := inventory.findLocked(tokenID); record != nil {
			return record, inventory, true
		}
	}
	return nil, APITokenInventory{}, false
}

// SetInstanceAPITokenInventory gives a tenant monitor the instance token
// inventory. Agent install tokens for a tenant org live there, bound to the
// org, because agents never name an organization and authentication finds
// header-less tokens only in the instance inventory. Call it before Start.
func (m *Monitor) SetInstanceAPITokenInventory(cfg *config.Config, persistence *config.ConfigPersistence) {
	if m == nil || cfg == nil || cfg == m.config {
		return
	}
	orgID := m.GetOrgID()
	if orgID == "" || orgID == "default" {
		return
	}
	m.instanceTokens = APITokenInventory{Config: cfg, Persistence: persistence, OrgID: orgID}
}

// APITokenInventories returns the inventories holding this monitor's agent
// credentials: the org's own config, then for a tenant org the instance
// records bound to it. Read and change a credential in the inventory that
// holds it. Own config comes first so a duplicate persisted there by an
// earlier version keeps the grant state it was given, and so a revocation
// that cannot commit leaves the instance credential, which agents
// authenticate with, still live.
func (m *Monitor) APITokenInventories() []APITokenInventory {
	if m == nil || m.config == nil {
		return nil
	}
	own := APITokenInventory{Config: m.config, Persistence: m.persistence}
	if m.instanceTokens.Config == nil {
		return []APITokenInventory{own}
	}
	return []APITokenInventory{own, m.instanceTokens}
}

// APITokenSnapshot copies the records of every inventory this monitor's
// agents authenticate with, once per token ID. It takes config.Mu itself:
// callers must not hold it.
func (m *Monitor) APITokenSnapshot() []config.APITokenRecord {
	config.Mu.RLock()
	defer config.Mu.RUnlock()
	var records []config.APITokenRecord
	seen := make(map[string]struct{})
	for _, inventory := range m.APITokenInventories() {
		for i := range inventory.Config.APITokens {
			record := &inventory.Config.APITokens[i]
			if !inventory.Owns(record) {
				continue
			}
			if id := record.ID; id != "" {
				if _, duplicate := seen[id]; duplicate {
					continue
				}
				seen[id] = struct{}{}
			}
			records = append(records, record.Clone())
		}
	}
	return records
}

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

	// A credential can sit in more than one inventory (a duplicate an earlier
	// version persisted in the org's own config). Revoke it everywhere or
	// nowhere: a half-committed revocation could leave the copy without its
	// consumed install grant as the only one.
	type revokedFrom struct {
		inventory      APITokenInventory
		previousTokens []config.APITokenRecord
	}
	var committed []revokedFrom
	restore := func() {
		for i := len(committed) - 1; i >= 0; i-- {
			cfg := committed[i].inventory.Config
			cfg.APITokens = committed[i].previousTokens
			cfg.SortAPITokens()
			if persistence := committed[i].inventory.Persistence; persistence != nil {
				if err := persistence.SaveAPITokens(cfg.APITokens); err != nil {
					log.Error().Err(err).Str("tokenID", tokenID).Msg("Failed to restore API token after a partial revocation")
				}
			}
		}
	}

	var removed *config.APITokenRecord
	for _, inventory := range m.APITokenInventories() {
		if inventory.findLocked(tokenID) == nil {
			continue
		}
		cfg := inventory.Config
		previousTokens := append([]config.APITokenRecord(nil), cfg.APITokens...)
		record := cfg.RemoveAPIToken(tokenID)
		cfg.SortAPITokens()
		if inventory.Persistence != nil {
			if err := inventory.Persistence.SaveAPITokens(cfg.APITokens); err != nil {
				cfg.APITokens = previousTokens
				cfg.SortAPITokens()
				restore()
				return nil, fmt.Errorf("persist API token revocation: %w", err)
			}
		}
		committed = append(committed, revokedFrom{inventory: inventory, previousTokens: previousTokens})
		if removed == nil {
			removed = record
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
