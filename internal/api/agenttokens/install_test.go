package agenttokens

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestIssueAndPersistInstallToken(t *testing.T) {
	cfg := &config.Config{DataPath: t.TempDir()}
	raw, record, err := IssueAndPersist(cfg, nil, IssueOptions{
		TokenName:   "host-agent",
		OwnerUserID: "operator",
		Scopes:      HostScopes(true),
		Metadata:    map[string]string{"install_type": "host"},
	})
	if err != nil {
		t.Fatalf("IssueAndPersist: %v", err)
	}
	if raw == "" || record == nil || len(cfg.APITokens) != 1 {
		t.Fatalf("issued token = (%q, %#v), persisted=%d", raw, record, len(cfg.APITokens))
	}
	if OwnerUserID(*record) != "operator" || record.Metadata[IssuedAtMetadataKey] == "" {
		t.Fatalf("issued metadata = %#v", record.Metadata)
	}
	if !record.HasScope(config.ScopeAgentExec) {
		t.Fatalf("commands-enabled host scopes = %v", record.Scopes)
	}
	if got := record.Metadata[RuntimeRoleMetadataKey]; got != CredentialKindLegacyFullTrust {
		t.Fatalf("combined install runtime role = %q, want %q", got, CredentialKindLegacyFullTrust)
	}
}

func TestIssueActionRunnerAndPersistIsHostBoundAndExecOnly(t *testing.T) {
	cfg := &config.Config{DataPath: t.TempDir()}
	raw, record, err := IssueActionRunnerAndPersist(cfg, nil, ActionRunnerIssueOptions{
		OrgID: "org-a", AgentID: "machine-123", Hostname: " Node.EXAMPLE. ", OwnerUserID: "operator",
	})
	if err != nil {
		t.Fatalf("IssueActionRunnerAndPersist: %v", err)
	}
	if raw == "" || record == nil {
		t.Fatalf("issued action credential = (%q, %#v)", raw, record)
	}
	if len(record.Scopes) != 1 || record.Scopes[0] != config.ScopeAgentExec {
		t.Fatalf("action credential scopes = %v, want only %q", record.Scopes, config.ScopeAgentExec)
	}
	if record.HasScope(config.ScopeAgentReport) || record.HasScope(config.ScopeAgentConfigRead) || record.HasScope(config.ScopeAgentManage) {
		t.Fatalf("action credential inherited collector authority: %v", record.Scopes)
	}
	if record.OrgID != "org-a" || record.Metadata["bound_agent_id"] != "machine-123" || record.Metadata["bound_hostname"] != "node.example" {
		t.Fatalf("action credential binding = org=%q metadata=%#v", record.OrgID, record.Metadata)
	}
	if record.Metadata[RuntimeRoleMetadataKey] != CredentialKindActionRunner ||
		record.Metadata[ActionCapabilityMetadataKey] != ActionCapabilityTypedV1 ||
		record.Metadata[ActionBindingVersionMetadataKey] != ActionBindingVersion {
		t.Fatalf("action credential authority metadata = %#v", record.Metadata)
	}
}

func TestIssueActionRunnerAndPersistRejectsIncompleteBinding(t *testing.T) {
	for _, tc := range []ActionRunnerIssueOptions{
		{AgentID: "machine", Hostname: "node"},
		{OrgID: "org", Hostname: "node"},
		{OrgID: "org", AgentID: "machine"},
	} {
		if raw, record, err := IssueActionRunnerAndPersist(&config.Config{}, nil, tc); !errors.Is(err, ErrRecord) || raw != "" || record != nil {
			t.Fatalf("incomplete binding %#v = (%q, %#v, %v), want ErrRecord", tc, raw, record, err)
		}
	}
}

func TestIssueActionRunnerAndPersistReplacesMatchingBoundCredential(t *testing.T) {
	cfg := &config.Config{DataPath: t.TempDir()}
	_, first, err := IssueActionRunnerAndPersist(cfg, nil, ActionRunnerIssueOptions{
		OrgID: "org-a", AgentID: "machine-123", Hostname: "Node.EXAMPLE",
	})
	if err != nil {
		t.Fatalf("first IssueActionRunnerAndPersist: %v", err)
	}
	_, otherHost, err := IssueActionRunnerAndPersist(cfg, nil, ActionRunnerIssueOptions{
		OrgID: "org-a", AgentID: "machine-456", Hostname: "other.example",
	})
	if err != nil {
		t.Fatalf("other-host IssueActionRunnerAndPersist: %v", err)
	}
	_, replacement, err := IssueActionRunnerAndPersist(cfg, nil, ActionRunnerIssueOptions{
		// Canonical agent identity survives a host rename. The old binding can no
		// longer admit that host and must not remain as an orphaned secret.
		OrgID: "org-a", AgentID: "machine-123", Hostname: "renamed.example",
	})
	if err != nil {
		t.Fatalf("replacement IssueActionRunnerAndPersist: %v", err)
	}
	if replacement.ID == first.ID {
		t.Fatalf("replacement reused token id %q", replacement.ID)
	}
	if len(cfg.APITokens) != 2 {
		t.Fatalf("token inventory = %#v, want one replacement plus other host", cfg.APITokens)
	}
	foundReplacement, foundOther, foundFirst := false, false, false
	for _, record := range cfg.APITokens {
		foundReplacement = foundReplacement || record.ID == replacement.ID
		foundOther = foundOther || record.ID == otherHost.ID
		foundFirst = foundFirst || record.ID == first.ID
	}
	if !foundReplacement || !foundOther || foundFirst {
		t.Fatalf("replacement inventory = %#v", cfg.APITokens)
	}
}

func TestIssueActionRunnerAndPersistDetailedReturnsOnlyDurablyReplacedRecords(t *testing.T) {
	cfg := &config.Config{DataPath: t.TempDir()}
	_, prior, err := IssueActionRunnerAndPersist(cfg, nil, ActionRunnerIssueOptions{
		OrgID: "org-a", AgentID: "machine-123", Hostname: "node.example",
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := IssueActionRunnerAndPersistDetailed(cfg, config.NewConfigPersistence(cfg.DataPath), ActionRunnerIssueOptions{
		OrgID: "org-a", AgentID: "machine-123", Hostname: "renamed.example",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Token == "" || result.Record == nil || len(result.Replaced) != 1 || result.Replaced[0].ID != prior.ID {
		t.Fatalf("detailed issue result = %#v", result)
	}
	if result.Record.ID == prior.ID || len(cfg.APITokens) != 1 || cfg.APITokens[0].ID != result.Record.ID {
		t.Fatalf("persisted replacement = %#v", cfg.APITokens)
	}
}

func TestIssueActionRunnerAndPersistRestoresReplacedCredentialOnPersistenceFailure(t *testing.T) {
	cfg := &config.Config{DataPath: t.TempDir()}
	_, prior, err := IssueActionRunnerAndPersist(cfg, nil, ActionRunnerIssueOptions{
		OrgID: "org-a", AgentID: "machine-123", Hostname: "node.example",
	})
	if err != nil {
		t.Fatalf("seed action credential: %v", err)
	}
	_, unrelated, err := IssueAndPersist(cfg, nil, IssueOptions{TokenName: "collector", OrgID: "org-a"})
	if err != nil {
		t.Fatalf("seed unrelated credential: %v", err)
	}

	statePath := filepath.Join(t.TempDir(), "blocked-state")
	persistence := config.NewConfigPersistence(statePath)
	if err := os.RemoveAll(statePath); err != nil {
		t.Fatalf("remove persistence directory: %v", err)
	}
	if err := os.WriteFile(statePath, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("create persistence blocker: %v", err)
	}
	raw, replacement, err := IssueActionRunnerAndPersist(cfg, persistence, ActionRunnerIssueOptions{
		OrgID: "org-a", AgentID: "machine-123", Hostname: "node.example",
	})
	if !errors.Is(err, ErrPersist) {
		t.Fatalf("replacement error = %v, want ErrPersist", err)
	}
	if raw != "" || replacement != nil {
		t.Fatalf("failed replacement returned credential: raw=%q record=%#v", raw, replacement)
	}
	if len(cfg.APITokens) != 2 {
		t.Fatalf("restored inventory = %#v", cfg.APITokens)
	}
	foundPrior, foundUnrelated := false, false
	for _, record := range cfg.APITokens {
		foundPrior = foundPrior || record.ID == prior.ID
		foundUnrelated = foundUnrelated || record.ID == unrelated.ID
	}
	if !foundPrior || !foundUnrelated {
		t.Fatalf("full prior inventory was not restored: %#v", cfg.APITokens)
	}
}

func TestIssueAndPersistRejectsMonitoringRoleWithExecAuthority(t *testing.T) {
	_, _, err := IssueAndPersist(&config.Config{}, nil, IssueOptions{
		TokenName: "invalid",
		Scopes:    []string{config.ScopeAgentReport, config.ScopeAgentExec},
		Metadata:  map[string]string{RuntimeRoleMetadataKey: CredentialKindMonitoringCollector},
	})
	if !errors.Is(err, ErrRecord) {
		t.Fatalf("monitoring role with exec authority error = %v, want ErrRecord", err)
	}
}

func TestProxmoxScopesRequireExplicitCommandAuthority(t *testing.T) {
	monitoringScopes := ProxmoxScopes(false)
	if (&config.APITokenRecord{Scopes: monitoringScopes}).HasScope(config.ScopeAgentExec) {
		t.Fatalf("monitoring-only Proxmox scopes include %s: %v", config.ScopeAgentExec, monitoringScopes)
	}
	if (&config.APITokenRecord{Scopes: monitoringScopes}).HasScope(config.ScopeAgentManage) {
		t.Fatalf("monitoring-only Proxmox scopes include cross-host management: %v", monitoringScopes)
	}

	commandScopes := ProxmoxScopes(true)
	if !(&config.APITokenRecord{Scopes: commandScopes}).HasScope(config.ScopeAgentExec) {
		t.Fatalf("explicit command profile is missing %s: %v", config.ScopeAgentExec, commandScopes)
	}
	if (&config.APITokenRecord{Scopes: commandScopes}).HasScope(config.ScopeAgentManage) {
		t.Fatalf("collector command profile includes cross-host management: %v", commandScopes)
	}
}

func TestIssueAndPersistDefaultsToMonitoringOnlyScopes(t *testing.T) {
	cfg := &config.Config{DataPath: t.TempDir()}
	_, record, err := IssueAndPersist(cfg, nil, IssueOptions{TokenName: "default-agent"})
	if err != nil {
		t.Fatalf("IssueAndPersist: %v", err)
	}
	if record.HasScope(config.ScopeAgentExec) {
		t.Fatalf("implicit install scopes include %s: %v", config.ScopeAgentExec, record.Scopes)
	}
}

func TestIssueAndPersistRollsBackCompleteInventoryWhenPersistenceFails(t *testing.T) {
	now := time.Now().UTC()
	tokens := []config.APITokenRecord{
		{ID: "newest", Name: "newest", Hash: "hash-newest", CreatedAt: now, Scopes: []string{config.ScopeWildcard}},
		{ID: "oldest", Name: "oldest", Hash: "hash-oldest", CreatedAt: now.Add(-time.Minute), Scopes: []string{config.ScopeWildcard}},
	}
	cfg := &config.Config{APITokens: append([]config.APITokenRecord(nil), tokens...)}
	cfg.SortAPITokens()

	stateDir := filepath.Join(t.TempDir(), "state")
	persistence := config.NewConfigPersistence(stateDir)
	if err := os.RemoveAll(stateDir); err != nil {
		t.Fatalf("remove persistence directory: %v", err)
	}
	if err := os.WriteFile(stateDir, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("create persistence blocker: %v", err)
	}

	raw, record, err := IssueAndPersist(cfg, persistence, IssueOptions{TokenName: "must-not-survive"})
	if !errors.Is(err, ErrPersist) {
		t.Fatalf("IssueAndPersist error = %v, want ErrPersist", err)
	}
	if raw != "" || record != nil {
		t.Fatalf("failed issue returned credential material: raw=%q record=%#v", raw, record)
	}
	if len(cfg.APITokens) != len(tokens) {
		t.Fatalf("token count = %d, want %d: %#v", len(cfg.APITokens), len(tokens), cfg.APITokens)
	}
	for index, want := range tokens {
		if cfg.APITokens[index].ID != want.ID {
			t.Fatalf("token[%d].ID = %q, want %q", index, cfg.APITokens[index].ID, want.ID)
		}
	}
	if cfg.APIToken != "hash-newest" {
		t.Fatalf("legacy primary token = %q, want rollback to %q", cfg.APIToken, "hash-newest")
	}
	for _, token := range cfg.APITokens {
		if token.Name == "must-not-survive" {
			t.Fatalf("failed issue left generated install token active: %#v", token)
		}
	}
}

func TestIssueAndPersistRejectsReservedOwnerMetadata(t *testing.T) {
	_, _, err := IssueAndPersist(&config.Config{}, nil, IssueOptions{
		TokenName: "invalid",
		Metadata:  map[string]string{OwnerUserIDMetadataKey: "forged"},
	})
	if !errors.Is(err, ErrRecord) {
		t.Fatalf("reserved owner metadata error = %v, want ErrRecord", err)
	}
}

func TestCommandPolicyIntentRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name    string
		value   string
		enabled bool
		valid   bool
	}{
		{name: "enabled", value: CommandPolicyIntent(true), enabled: true, valid: true},
		{name: "disabled", value: CommandPolicyIntent(false), enabled: false, valid: true},
		{name: "missing", value: "", enabled: false, valid: false},
		{name: "invalid", value: "yes", enabled: false, valid: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			record := &config.APITokenRecord{Metadata: map[string]string{
				CommandPolicyIntentMetadataKey: tc.value,
			}}
			enabled, valid := ParseCommandPolicyIntent(record)
			if enabled != tc.enabled || valid != tc.valid {
				t.Fatalf("ParseCommandPolicyIntent() = (%v, %v), want (%v, %v)", enabled, valid, tc.enabled, tc.valid)
			}
		})
	}

	if _, valid := ParseCommandPolicyIntent(nil); valid {
		t.Fatal("nil record unexpectedly carried command-policy intent")
	}
}
