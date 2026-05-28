package api

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/updates"
)

const (
	updateReadinessReady     = "ready"
	updateReadinessAttention = "attention"
	updateReadinessBlocked   = "blocked"

	updateReadinessCheckPass    = "pass"
	updateReadinessCheckWarning = "warning"
	updateReadinessCheckBlocked = "blocked"
)

type updateReadinessInputs struct {
	cfg           *config.Config
	hosts         []models.Host
	targetVersion string
	plan          updates.UpdatePlan
	now           time.Time
}

func buildUpdateReadiness(in updateReadinessInputs) *updates.UpdateReadiness {
	now := in.now
	if now.IsZero() {
		now = time.Now()
	}

	checks := []updates.UpdateReadinessCheck{
		buildUpdatePathReadinessCheck(in.plan),
		buildAgentContinuityReadinessCheck(in.hosts, in.targetVersion, now),
		buildAgentTokenReadinessCheck(in.cfg, len(in.hosts), now),
	}

	status := updateReadinessReady
	for _, check := range checks {
		switch check.Status {
		case updateReadinessCheckBlocked:
			status = updateReadinessBlocked
		case updateReadinessCheckWarning:
			if status != updateReadinessBlocked {
				status = updateReadinessAttention
			}
		}
	}

	return &updates.UpdateReadiness{
		Status:  status,
		Summary: updateReadinessSummary(status, checks),
		Checks:  checks,
	}
}

func updateReadinessSummary(status string, checks []updates.UpdateReadinessCheck) string {
	blocked := 0
	warnings := 0
	for _, check := range checks {
		switch check.Status {
		case updateReadinessCheckBlocked:
			blocked++
		case updateReadinessCheckWarning:
			warnings++
		}
	}

	switch status {
	case updateReadinessBlocked:
		return fmt.Sprintf("Resolve %d blocked upgrade %s before installing this update.", blocked, pluralNoun(blocked, "check", "checks"))
	case updateReadinessAttention:
		return fmt.Sprintf("Update can proceed, but review %d upgrade %s before installing.", warnings, pluralNoun(warnings, "warning", "warnings"))
	default:
		return "Upgrade checks passed for this Pulse instance."
	}
}

func buildUpdatePathReadinessCheck(plan updates.UpdatePlan) updates.UpdateReadinessCheck {
	if plan.CanAutoUpdate && plan.RollbackSupport {
		return updates.UpdateReadinessCheck{
			ID:      "server-update-path",
			Status:  updateReadinessCheckPass,
			Title:   "Server update path",
			Summary: "Automatic install and rollback support are available for this deployment.",
			Details: []string{
				"Pulse will stage the update through the configured updater.",
				"A rollback-capable backup is part of this update path.",
			},
		}
	}

	if plan.CanAutoUpdate {
		return updates.UpdateReadinessCheck{
			ID:      "server-update-path",
			Status:  updateReadinessCheckWarning,
			Title:   "Server update path",
			Summary: "Automatic install is available, but rollback support is not advertised by this updater.",
			Details: []string{"Keep console access and an external backup available before installing."},
		}
	}

	return updates.UpdateReadinessCheck{
		ID:      "server-update-path",
		Status:  updateReadinessCheckWarning,
		Title:   "Server update path",
		Summary: "This deployment uses a manual update path.",
		Details: []string{"Follow the generated update instructions and keep a backup available before restarting Pulse."},
	}
}

func buildAgentContinuityReadinessCheck(hosts []models.Host, targetVersion string, now time.Time) updates.UpdateReadinessCheck {
	if len(hosts) == 0 {
		return updates.UpdateReadinessCheck{
			ID:      "agent-continuity",
			Status:  updateReadinessCheckPass,
			Title:   "Agent continuity",
			Summary: "No installed Pulse agents are currently registered.",
		}
	}

	connections := buildConnections(aggregatorInputs{
		hosts:                hosts,
		expectedAgentVersion: strings.TrimSpace(targetVersion),
		now:                  now,
	})

	active := 0
	pendingOrStale := 0
	behind := 0
	legacy := 0
	unknownVersion := 0
	for _, conn := range connections {
		if conn.Type != ConnectionTypeAgent {
			continue
		}
		switch conn.State {
		case ConnectionStateActive:
			active++
		case ConnectionStatePending, ConnectionStateStale:
			pendingOrStale++
		}
		if conn.AgentUpdateAvailable {
			behind++
		}
		if strings.TrimSpace(conn.AgentVersion) == "" {
			unknownVersion++
		}
	}
	for _, host := range hosts {
		if host.IsLegacy || looksLikePreV6Version(host.AgentVersion) {
			legacy++
		}
	}

	details := []string{fmt.Sprintf("%s currently registered.", countWithNoun(len(hosts), "agent", "agents"))}
	if active > 0 {
		details = append(details, fmt.Sprintf("%s have a recent heartbeat.", countWithNoun(active, "agent", "agents")))
	}
	if legacy > 0 {
		details = append(details, fmt.Sprintf("%s can continue reporting through the v6 compatibility route.", countWithNoun(legacy, "v5 or legacy agent", "v5 or legacy agents")))
	}
	if behind > 0 {
		details = append(details, fmt.Sprintf("%s should move toward %s after the server update.", countWithNoun(behind, "agent", "agents"), targetVersionLabel(targetVersion)))
	}
	if unknownVersion > 0 {
		details = append(details, fmt.Sprintf("%s did not report a version.", countWithNoun(unknownVersion, "agent", "agents")))
	}

	if pendingOrStale > 0 {
		return updates.UpdateReadinessCheck{
			ID:      "agent-continuity",
			Status:  updateReadinessCheckWarning,
			Title:   "Agent continuity",
			Summary: fmt.Sprintf("%s %s not recently connected, so Pulse cannot prove they will update cleanly.", countWithNoun(pendingOrStale, "agent", "agents"), pluralVerb(pendingOrStale, "is", "are")),
			Details: details,
		}
	}
	if unknownVersion > 0 {
		return updates.UpdateReadinessCheck{
			ID:      "agent-continuity",
			Status:  updateReadinessCheckWarning,
			Title:   "Agent continuity",
			Summary: "Some agents are connected but did not report a version.",
			Details: details,
		}
	}

	return updates.UpdateReadinessCheck{
		ID:      "agent-continuity",
		Status:  updateReadinessCheckPass,
		Title:   "Agent continuity",
		Summary: "Registered agents have recent heartbeats and can continue through the v6 compatibility path.",
		Details: details,
	}
}

func buildAgentTokenReadinessCheck(cfg *config.Config, agentCount int, now time.Time) updates.UpdateReadinessCheck {
	if cfg == nil {
		return updates.UpdateReadinessCheck{
			ID:      "agent-token-scopes",
			Status:  updateReadinessCheckWarning,
			Title:   "Agent token scopes",
			Summary: "Pulse could not read the loaded API token set for this readiness check.",
		}
	}

	if agentCount == 0 {
		return updates.UpdateReadinessCheck{
			ID:      "agent-token-scopes",
			Status:  updateReadinessCheckPass,
			Title:   "Agent token scopes",
			Summary: "No registered agents need an agent reporting token during this upgrade.",
		}
	}

	agentScoped := 0
	expired := 0
	expiring := 0
	for _, record := range cfg.APITokens {
		if !record.HasScope(config.ScopeAgentReport) {
			continue
		}
		agentScoped++
		if record.ExpiresAt != nil {
			if now.After(*record.ExpiresAt) {
				expired++
			} else if record.ExpiresAt.Sub(now) <= 14*24*time.Hour {
				expiring++
			}
		}
	}

	if agentScoped == 0 {
		return updates.UpdateReadinessCheck{
			ID:      "agent-token-scopes",
			Status:  updateReadinessCheckBlocked,
			Title:   "Agent token scopes",
			Summary: "Registered agents exist, but no loaded API token grants agent reporting scope.",
			Details: []string{"Existing v5 host-agent scopes are normally normalized to agent:report on load."},
		}
	}
	if expired >= agentScoped {
		return updates.UpdateReadinessCheck{
			ID:      "agent-token-scopes",
			Status:  updateReadinessCheckBlocked,
			Title:   "Agent token scopes",
			Summary: "All loaded agent reporting tokens are expired.",
			Details: []string{"Create or refresh an agent install token before relying on agent reconnect after the update."},
		}
	}
	if expired > 0 || expiring > 0 {
		return updates.UpdateReadinessCheck{
			ID:      "agent-token-scopes",
			Status:  updateReadinessCheckWarning,
			Title:   "Agent token scopes",
			Summary: "At least one agent reporting token is expired or expires soon.",
			Details: []string{
				fmt.Sprintf("%s loaded.", countWithNoun(agentScoped, "agent reporting token", "agent reporting tokens")),
				fmt.Sprintf("%s expired, %s expiring within 14 days.", countWithNoun(expired, "token", "tokens"), countWithNoun(expiring, "token", "tokens")),
			},
		}
	}

	return updates.UpdateReadinessCheck{
		ID:      "agent-token-scopes",
		Status:  updateReadinessCheckPass,
		Title:   "Agent token scopes",
		Summary: "Loaded API tokens include agent reporting scope for registered agents.",
		Details: []string{fmt.Sprintf("%s available.", countWithNoun(agentScoped, "agent reporting token", "agent reporting tokens"))},
	}
}

func looksLikePreV6Version(version string) bool {
	version = strings.TrimSpace(strings.TrimPrefix(version, "v"))
	if version == "" {
		return false
	}
	major, _, ok := strings.Cut(version, ".")
	if !ok || major == "" {
		return false
	}
	majorNumber, err := strconv.Atoi(major)
	return err == nil && majorNumber < 6
}

func targetVersionLabel(version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		return "the target server version"
	}
	return version
}

func pluralNoun(count int, singular, plural string) string {
	if count == 1 {
		return singular
	}
	return plural
}

func countWithNoun(count int, singular, plural string) string {
	return fmt.Sprintf("%d %s", count, pluralNoun(count, singular, plural))
}
