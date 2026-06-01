package portal

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/crypto"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
	"github.com/rs/zerolog/log"
)

type WorkspaceSetupFacts struct {
	AgentCount          *int
	LastAgentSeenAt     *time.Time
	AlertRouteCount     *int
	ReportScheduleCount *int
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
	orgDir, ok := r.orgConfigDir(tenantID)
	if !ok {
		return WorkspaceSetupFacts{}
	}
	return readWorkspaceSetupFacts(orgDir)
}

func (r tenantDirWorkspaceSetupFactReader) orgConfigDir(tenantID string) (string, bool) {
	tenantID = strings.TrimSpace(tenantID)
	if r.tenantsDir == "" || tenantID == "" || filepath.Base(tenantID) != tenantID {
		return "", false
	}
	return filepath.Join(r.tenantsDir, tenantID, "orgs", tenantID), true
}

func readWorkspaceSetupFacts(orgDir string) WorkspaceSetupFacts {
	facts := WorkspaceSetupFacts{}
	if orgDir == "" {
		return facts
	}

	facts.AgentCount, facts.LastAgentSeenAt = readAgentSetupFacts(orgDir)
	facts.AlertRouteCount = intPtr(readAlertRouteCount(orgDir))
	facts.ReportScheduleCount = intPtr(readReportScheduleCount(orgDir))
	return facts
}

func intPtr(value int) *int {
	return &value
}

func readAgentSetupFacts(orgDir string) (*int, *time.Time) {
	var tokens []config.APITokenRecord
	ok, err := readMaybeEncryptedJSON(orgDir, "api_tokens.json", &tokens)
	if err != nil {
		log.Warn().Err(err).Str("org_dir", orgDir).Msg("cloudcp.portal.setup_facts: read api tokens")
		return nil, nil
	}
	if !ok {
		return intPtr(0), nil
	}

	count := 0
	var lastSeen *time.Time
	for i := range tokens {
		if tokens[i].IsExpired() || !tokens[i].HasScope(config.ScopeAgentReport) || tokens[i].LastUsedAt == nil {
			continue
		}
		count++
		if lastSeen == nil || tokens[i].LastUsedAt.After(*lastSeen) {
			seenAt := tokens[i].LastUsedAt.UTC()
			lastSeen = &seenAt
		}
	}
	return intPtr(count), lastSeen
}

func readAlertRouteCount(orgDir string) int {
	count := 0

	var email notifications.EmailConfig
	if ok, err := readMaybeEncryptedJSON(orgDir, "email.enc", &email); err != nil {
		log.Warn().Err(err).Str("org_dir", orgDir).Msg("cloudcp.portal.setup_facts: read email config")
	} else if ok && email.Enabled {
		for _, recipient := range email.To {
			if strings.TrimSpace(recipient) != "" {
				count++
			}
		}
	}

	var apprise notifications.AppriseConfig
	if ok, err := readMaybeEncryptedJSON(orgDir, "apprise.enc", &apprise); err != nil {
		log.Warn().Err(err).Str("org_dir", orgDir).Msg("cloudcp.portal.setup_facts: read apprise config")
	} else if ok && apprise.Enabled {
		for _, target := range apprise.Targets {
			if strings.TrimSpace(target) != "" {
				count++
			}
		}
	}

	var webhooks []notifications.WebhookConfig
	if ok, err := readMaybeEncryptedJSON(orgDir, "webhooks.enc", &webhooks); err != nil {
		log.Warn().Err(err).Str("org_dir", orgDir).Msg("cloudcp.portal.setup_facts: read webhook config")
	} else if !ok {
		_, _ = readMaybeEncryptedJSON(orgDir, "webhooks.json", &webhooks)
	}
	for _, webhook := range webhooks {
		if webhook.Enabled && strings.TrimSpace(webhook.URL) != "" {
			count++
		}
	}

	return count
}

func readReportScheduleCount(orgDir string) int {
	for _, leaf := range []string{"report_schedules.json", filepath.Join("reports", "schedules.json")} {
		var raw json.RawMessage
		ok, err := readMaybeEncryptedJSON(orgDir, leaf, &raw)
		if err != nil {
			log.Warn().Err(err).Str("org_dir", orgDir).Str("file", leaf).Msg("cloudcp.portal.setup_facts: read report schedules")
			continue
		}
		if ok {
			return countReportSchedules(raw)
		}
	}
	return 0
}

func countReportSchedules(raw json.RawMessage) int {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return 0
	}

	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err == nil {
		return countEnabledScheduleItems(items)
	}

	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil || len(object) == 0 {
		return 0
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
		return 1
	}
	return 0
}

func countEnabledScheduleItems(items []json.RawMessage) int {
	count := 0
	for _, item := range items {
		if isScheduleItemEnabled(item) {
			count++
		}
	}
	return count
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
