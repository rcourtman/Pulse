package portal

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/crypto"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
	"github.com/rs/zerolog/log"
)

type WorkspaceSetupFacts struct {
	AgentCount                  *int
	AgentTokenCount             *int
	UnusedAgentTokenCount       *int
	LastAgentSeenAt             *time.Time
	AlertRouteCount             *int
	DisabledAlertRouteCount     *int
	ActiveCriticalAlertCount    *int
	ActiveWarningAlertCount     *int
	ActiveAlertsUpdatedAt       *time.Time
	ReportScheduleCount         *int
	DisabledReportScheduleCount *int
}

type WorkspaceSetupFactReader interface {
	FactsForWorkspace(tenantID string) WorkspaceSetupFacts
}

type tenantDirWorkspaceSetupFactReader struct {
	tenantsDir string
}

func NewTenantDirWorkspaceSetupFactReader(tenantsDir string) WorkspaceSetupFactReader {
	return tenantDirWorkspaceSetupFactReader{tenantsDir: strings.TrimSpace(tenantsDir)}
}

func (r tenantDirWorkspaceSetupFactReader) FactsForWorkspace(tenantID string) WorkspaceSetupFacts {
	tenantDataDir, orgDir, ok := r.workspaceConfigDirs(tenantID)
	if !ok {
		return WorkspaceSetupFacts{}
	}
	return readWorkspaceSetupFacts(tenantID, tenantDataDir, orgDir)
}

func (r tenantDirWorkspaceSetupFactReader) workspaceConfigDirs(tenantID string) (tenantDataDir string, orgDir string, ok bool) {
	tenantID = strings.TrimSpace(tenantID)
	if r.tenantsDir == "" || tenantID == "" || filepath.Base(tenantID) != tenantID {
		return "", "", false
	}
	tenantDataDir = filepath.Join(r.tenantsDir, tenantID)
	return tenantDataDir, filepath.Join(tenantDataDir, "orgs", tenantID), true
}

func readWorkspaceSetupFacts(tenantID, tenantDataDir, orgDir string) WorkspaceSetupFacts {
	facts := WorkspaceSetupFacts{}
	if orgDir == "" {
		return facts
	}

	facts.AgentCount, facts.AgentTokenCount, facts.UnusedAgentTokenCount, facts.LastAgentSeenAt = readAgentSetupFacts(tenantID, tenantDataDir, orgDir)
	facts.AlertRouteCount, facts.DisabledAlertRouteCount = readAlertRouteFacts(orgDir)
	facts.ActiveCriticalAlertCount, facts.ActiveWarningAlertCount, facts.ActiveAlertsUpdatedAt = readActiveAlertFacts(tenantDataDir)
	facts.ReportScheduleCount, facts.DisabledReportScheduleCount = readReportScheduleFacts(orgDir)
	return facts
}

func intPtr(value int) *int {
	return &value
}

func readAgentSetupFacts(tenantID, tenantDataDir, orgDir string) (*int, *int, *int, *time.Time) {
	tokens := make([]config.APITokenRecord, 0)
	seen := map[string]struct{}{}
	appendTokens := func(dir, source string, filter func(config.APITokenRecord) bool) {
		if dir == "" {
			return
		}
		var sourceTokens []config.APITokenRecord
		ok, err := readMaybeEncryptedJSON(dir, "api_tokens.json", &sourceTokens)
		if err != nil {
			log.Warn().Err(err).Str("dir", dir).Str("source", source).Msg("cloudcp.portal.setup_facts: read api tokens")
			return
		}
		if !ok {
			return
		}
		for _, token := range sourceTokens {
			if filter != nil && !filter(token) {
				continue
			}
			key := agentSetupTokenKey(token)
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			tokens = append(tokens, token)
		}
	}

	// Tokens created inside the org-specific config directory belong to that
	// workspace by construction.
	appendTokens(orgDir, "org", nil)
	// Hosted tenant install commands are persisted in the tenant runtime's
	// shared token store, with OrgID/OrgIDs carrying the workspace boundary.
	appendTokens(tenantDataDir, "tenant-root", func(token config.APITokenRecord) bool {
		return agentSetupTokenMatchesWorkspace(token, tenantID)
	})

	count := 0
	tokenCount := 0
	unusedCount := 0
	var lastSeen *time.Time
	for i := range tokens {
		if tokens[i].IsExpired() || !tokens[i].HasScope(config.ScopeAgentReport) {
			continue
		}
		tokenCount++
		if tokens[i].LastUsedAt == nil {
			unusedCount++
			continue
		}
		count++
		if lastSeen == nil || tokens[i].LastUsedAt.After(*lastSeen) {
			seenAt := tokens[i].LastUsedAt.UTC()
			lastSeen = &seenAt
		}
	}
	return intPtr(count), intPtr(tokenCount), intPtr(unusedCount), lastSeen
}

func agentSetupTokenKey(token config.APITokenRecord) string {
	if strings.TrimSpace(token.ID) != "" {
		return "id:" + strings.TrimSpace(token.ID)
	}
	if strings.TrimSpace(token.Hash) != "" {
		return "hash:" + strings.TrimSpace(token.Hash)
	}
	return "token:" + strings.TrimSpace(token.Name) + ":" + token.CreatedAt.UTC().Format(time.RFC3339Nano) + ":" + strings.Join(token.Scopes, ",")
}

func agentSetupTokenMatchesWorkspace(token config.APITokenRecord, tenantID string) bool {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return false
	}
	if strings.TrimSpace(token.OrgID) == tenantID {
		return true
	}
	for _, orgID := range token.OrgIDs {
		if strings.TrimSpace(orgID) == tenantID {
			return true
		}
	}
	return false
}

func readAlertRouteFacts(orgDir string) (*int, *int) {
	enabledCount := 0
	disabledCount := 0

	var email notifications.EmailConfig
	if ok, err := readMaybeEncryptedJSON(orgDir, "email.enc", &email); err != nil {
		log.Warn().Err(err).Str("org_dir", orgDir).Msg("cloudcp.portal.setup_facts: read email config")
	} else if ok {
		emailRecipients := nonBlankCount(email.To)
		if email.Enabled {
			enabledCount += emailRecipients
		} else {
			disabledCount += emailRecipients
		}
	}

	var apprise notifications.AppriseConfig
	if ok, err := readMaybeEncryptedJSON(orgDir, "apprise.enc", &apprise); err != nil {
		log.Warn().Err(err).Str("org_dir", orgDir).Msg("cloudcp.portal.setup_facts: read apprise config")
	} else if ok {
		appriseTargets := nonBlankCount(apprise.Targets)
		if apprise.Enabled {
			enabledCount += appriseTargets
		} else {
			disabledCount += appriseTargets
		}
	}

	var webhooks []notifications.WebhookConfig
	if ok, err := readMaybeEncryptedJSON(orgDir, "webhooks.enc", &webhooks); err != nil {
		log.Warn().Err(err).Str("org_dir", orgDir).Msg("cloudcp.portal.setup_facts: read webhook config")
	} else if !ok {
		_, _ = readMaybeEncryptedJSON(orgDir, "webhooks.json", &webhooks)
	}
	for _, webhook := range webhooks {
		if strings.TrimSpace(webhook.URL) == "" {
			continue
		}
		if webhook.Enabled {
			enabledCount++
		} else {
			disabledCount++
		}
	}

	return intPtr(enabledCount), intPtr(disabledCount)
}

func nonBlankCount(values []string) int {
	count := 0
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			count++
		}
	}
	return count
}

func readActiveAlertFacts(tenantDataDir string) (*int, *int, *time.Time) {
	path, ok := safeConfigLeafPath(tenantDataDir, filepath.Join("alerts", "active-alerts.json"))
	if !ok {
		return nil, nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			log.Warn().Err(err).Str("tenant_data_dir", tenantDataDir).Msg("cloudcp.portal.setup_facts: read active alerts")
		}
		return nil, nil, nil
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, nil, nil
	}

	var active []alerts.Alert
	if err := json.Unmarshal(data, &active); err != nil {
		log.Warn().Err(err).Str("tenant_data_dir", tenantDataDir).Msg("cloudcp.portal.setup_facts: parse active alerts")
		return nil, nil, nil
	}

	criticalCount := 0
	warningCount := 0
	for _, alert := range active {
		switch strings.ToLower(strings.TrimSpace(string(alert.Level))) {
		case "critical":
			criticalCount++
		case "warning":
			warningCount++
		}
	}

	var updatedAt *time.Time
	if info, err := os.Stat(path); err == nil {
		ts := info.ModTime().UTC()
		updatedAt = &ts
	}
	return intPtr(criticalCount), intPtr(warningCount), updatedAt
}

func readReportScheduleFacts(orgDir string) (*int, *int) {
	for _, leaf := range []string{"report_schedules.json", filepath.Join("reports", "schedules.json")} {
		var raw json.RawMessage
		ok, err := readMaybeEncryptedJSON(orgDir, leaf, &raw)
		if err != nil {
			log.Warn().Err(err).Str("org_dir", orgDir).Str("file", leaf).Msg("cloudcp.portal.setup_facts: read report schedules")
			continue
		}
		if ok {
			enabled, disabled := countReportSchedules(raw)
			return intPtr(enabled), intPtr(disabled)
		}
	}
	return intPtr(0), intPtr(0)
}

func countReportSchedules(raw json.RawMessage) (int, int) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return 0, 0
	}

	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err == nil {
		return countEnabledScheduleItems(items)
	}

	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil || len(object) == 0 {
		return 0, 0
	}

	for _, key := range []string{"schedules", "reports", "items"} {
		if value, ok := object[key]; ok {
			var nested []json.RawMessage
			if err := json.Unmarshal(value, &nested); err == nil {
				return countEnabledScheduleItems(nested)
			}
		}
	}

	if isScheduleItemEnabled(raw) {
		return 1, 0
	}
	if isScheduleItemDisabled(raw) {
		return 0, 1
	}
	return 0, 0
}

func countEnabledScheduleItems(items []json.RawMessage) (int, int) {
	enabledCount := 0
	disabledCount := 0
	for _, item := range items {
		if isScheduleItemEnabled(item) {
			enabledCount++
		} else if isScheduleItemDisabled(item) {
			disabledCount++
		}
	}
	return enabledCount, disabledCount
}

func isScheduleItemEnabled(raw json.RawMessage) bool {
	var item map[string]any
	if err := json.Unmarshal(raw, &item); err != nil {
		return false
	}
	if enabled, ok := item["enabled"].(bool); ok && !enabled {
		return false
	}
	if disabled, ok := item["disabled"].(bool); ok && disabled {
		return false
	}
	return len(item) > 0
}

func isScheduleItemDisabled(raw json.RawMessage) bool {
	var item map[string]any
	if err := json.Unmarshal(raw, &item); err != nil || len(item) == 0 {
		return false
	}
	if enabled, ok := item["enabled"].(bool); ok && !enabled {
		return true
	}
	if disabled, ok := item["disabled"].(bool); ok && disabled {
		return true
	}
	return false
}

func readMaybeEncryptedJSON(configDir string, leaf string, out any) (bool, error) {
	path, ok := safeConfigLeafPath(configDir, leaf)
	if !ok {
		return false, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return true, nil
	}

	if err := json.Unmarshal(data, out); err == nil {
		return true, nil
	}

	keyPath := filepath.Join(configDir, ".encryption.key")
	if _, err := os.Stat(keyPath); err != nil {
		return false, err
	}

	manager, err := crypto.NewCryptoManagerAt(configDir)
	if err != nil {
		return false, err
	}
	decrypted, err := manager.Decrypt(data)
	if err != nil {
		return false, err
	}
	if err := json.Unmarshal(decrypted, out); err != nil {
		return false, err
	}
	return true, nil
}

func safeConfigLeafPath(configDir string, leaf string) (string, bool) {
	configDir = strings.TrimSpace(configDir)
	leaf = strings.TrimSpace(leaf)
	if configDir == "" || leaf == "" || filepath.IsAbs(leaf) {
		return "", false
	}
	cleanLeaf := filepath.Clean(leaf)
	if cleanLeaf == "." || cleanLeaf == ".." || strings.HasPrefix(cleanLeaf, ".."+string(os.PathSeparator)) {
		return "", false
	}
	return filepath.Join(configDir, cleanLeaf), true
}
