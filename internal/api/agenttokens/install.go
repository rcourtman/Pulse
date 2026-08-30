// Package agenttokens owns the security contract for install-token issuance.
package agenttokens

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/actionrunner"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

const (
	IssuedAtMetadataKey                      = "install_issued_at"
	OwnerUserIDMetadataKey                   = "owner_user_id"
	CommandPolicyIntentMetadataKey           = "command_policy_intent"
	CommandPolicyAppliedAgentIDMetadataKey   = "command_policy_applied_agent_id"
	CommandPolicyIntentEnabled               = "enabled"
	CommandPolicyIntentDisabled              = "disabled"
	RuntimeRoleMetadataKey                   = internalauth.RuntimeRoleMetadataKey
	CredentialKindMetadataKey                = RuntimeRoleMetadataKey
	CredentialKindMonitoringCollector        = internalauth.RuntimeRoleMonitoringCollector
	CredentialKindActionRunner               = internalauth.RuntimeRoleActionRunner
	CredentialKindLegacyFullTrust            = internalauth.RuntimeRoleLegacyFullTrust
	ActionCapabilityMetadataKey              = "agent_action_capability"
	ActionCapabilityTypedV1                  = "typed_actions.v1"
	ActionBindingVersionMetadataKey          = "action_runner_binding_version"
	ActionBindingVersion                     = "1"
	ActionRunnerActivationPendingMetadataKey = "action_runner_activation_pending"
	ActionRunnerReplacesTokenIDsMetadataKey  = "action_runner_replaces_token_ids"
	ActionRunnerActivationWindow             = 10 * time.Minute
)

var (
	ErrGeneration = errors.New("agent install token generation failed")
	ErrRecord     = errors.New("agent install token record failed")
	ErrPersist    = errors.New("agent install token persistence failed")
)

type IssueOptions struct {
	TokenName   string
	OrgID       string
	OwnerUserID string
	Metadata    map[string]string
	Scopes      []string
}

// ActionRunnerIssueOptions binds a separately issued remediation credential to
// one tenant and one canonical host identity. The credential is intentionally
// unusable for collector report and configuration endpoints.
type ActionRunnerIssueOptions struct {
	TokenName   string
	OrgID       string
	OwnerUserID string
	AgentID     string
	Hostname    string
}

// ActionRunnerIssueResult carries the newly issued credential together with
// the prior host-bound records it durably replaced. Callers use the replaced
// non-secret identities to invalidate only the superseded live sessions after
// persistence succeeds.
type ActionRunnerIssueResult struct {
	Token    string
	Record   *config.APITokenRecord
	Replaced []config.APITokenRecord
}

func cloneAPITokenRecords(records []config.APITokenRecord) []config.APITokenRecord {
	cloned := make([]config.APITokenRecord, len(records))
	for index := range records {
		cloned[index] = records[index].Clone()
	}
	return cloned
}

func ProxmoxScopes(enableCommands bool) []string {
	scopes := []string{
		config.ScopeAgentReport,
		config.ScopeAgentConfigRead,
	}
	if enableCommands {
		scopes = append(scopes, config.ScopeAgentExec)
	}
	return scopes
}

func HostScopes(enableCommands bool) []string {
	scopes := []string{
		config.ScopeAgentReport,
		config.ScopeAgentConfigRead,
		config.ScopeDockerReport,
		config.ScopeKubernetesReport,
	}
	if enableCommands {
		scopes = append(scopes, config.ScopeAgentExec)
	}
	return scopes
}

// ActionRunnerScopes is the complete authority granted to the separate action
// runtime. Monitoring scopes must be added only by a distinct, explicit grant.
func ActionRunnerScopes() []string {
	return []string{config.ScopeAgentExec}
}

func CommandPolicyIntent(enableCommands bool) string {
	if enableCommands {
		return CommandPolicyIntentEnabled
	}
	return CommandPolicyIntentDisabled
}

func ParseCommandPolicyIntent(record *config.APITokenRecord) (bool, bool) {
	if record == nil {
		return false, false
	}
	switch strings.TrimSpace(record.Metadata[CommandPolicyIntentMetadataKey]) {
	case CommandPolicyIntentEnabled:
		return true, true
	case CommandPolicyIntentDisabled:
		return false, true
	default:
		return false, false
	}
}

func IssueAndPersist(cfg *config.Config, persistence *config.ConfigPersistence, opts IssueOptions) (string, *config.APITokenRecord, error) {
	rawToken, record, _, err := issueAndPersistReplacing(cfg, persistence, opts, nil)
	return rawToken, record, err
}

func issueAndPersistReplacing(
	cfg *config.Config,
	persistence *config.ConfigPersistence,
	opts IssueOptions,
	replace func(config.APITokenRecord) bool,
) (string, *config.APITokenRecord, []config.APITokenRecord, error) {
	if cfg == nil {
		return "", nil, nil, fmt.Errorf("config is required")
	}

	rawToken, err := internalauth.GenerateAPIToken()
	if err != nil {
		return "", nil, nil, fmt.Errorf("%w: %w", ErrGeneration, err)
	}

	scopes := opts.Scopes
	if len(scopes) == 0 {
		// A caller that does not declare an authority profile receives a
		// monitoring-only credential. Command authority must always be an
		// explicit install-time choice.
		scopes = ProxmoxScopes(false)
	}
	record, err := config.NewAPITokenRecord(rawToken, opts.TokenName, scopes)
	if err != nil {
		return "", nil, nil, fmt.Errorf("%w: %w", ErrRecord, err)
	}

	record.OrgID = strings.TrimSpace(opts.OrgID)
	setOwnerUserID(record, opts.OwnerUserID)
	if err := mergeMetadata(record, opts.Metadata); err != nil {
		return "", nil, nil, fmt.Errorf("%w: %w", ErrRecord, err)
	}
	if record.Metadata == nil {
		record.Metadata = make(map[string]string)
	}
	if err := normalizeCredentialKind(record); err != nil {
		return "", nil, nil, fmt.Errorf("%w: %w", ErrRecord, err)
	}
	record.Metadata[IssuedAtMetadataKey] = record.CreatedAt.UTC().Format(time.RFC3339)

	config.Mu.Lock()
	defer config.Mu.Unlock()

	previousTokens := cloneAPITokenRecords(cfg.APITokens)
	replaced := make([]config.APITokenRecord, 0, 1)
	if replace == nil {
		cfg.APITokens = append(cfg.APITokens, *record)
	} else {
		nextTokens := make([]config.APITokenRecord, 0, len(cfg.APITokens)+1)
		for _, existing := range cfg.APITokens {
			if replace(existing) {
				replaced = append(replaced, existing.Clone())
			} else {
				nextTokens = append(nextTokens, existing)
			}
		}
		cfg.APITokens = append(nextTokens, *record)
	}
	cfg.SortAPITokens()
	if persistence != nil {
		if err := persistence.SaveAPITokens(cfg.APITokens); err != nil {
			// The generated record sorts newest-first, so truncating the sorted
			// inventory would discard an older valid token and keep the secret
			// that the failed request never returned. Restore the full snapshot.
			cfg.APITokens = previousTokens
			cfg.SortAPITokens()
			return "", nil, nil, fmt.Errorf("%w: %w", ErrPersist, err)
		}
	}

	return rawToken, record, replaced, nil
}

// IssueActionRunnerAndPersist mints the host-bound credential used by the
// separate action runner. It fails closed on missing tenant/host identity and
// never grants collector report, lookup, configuration, or management scopes.
func IssueActionRunnerAndPersist(cfg *config.Config, persistence *config.ConfigPersistence, opts ActionRunnerIssueOptions) (string, *config.APITokenRecord, error) {
	result, err := IssueActionRunnerAndPersistDetailed(cfg, persistence, opts)
	return result.Token, result.Record, err
}

// IssueActionRunnerAndPersistDetailed prepares a bounded action-runner
// activation. The prior active credential remains valid until the replacement
// runner explicitly commits activation after writing its durable health proof.
// Replaced contains only older unactivated credentials removed by this prepare
// step; persistence failure returns an empty result.
func IssueActionRunnerAndPersistDetailed(cfg *config.Config, persistence *config.ConfigPersistence, opts ActionRunnerIssueOptions) (ActionRunnerIssueResult, error) {
	agentID := strings.TrimSpace(opts.AgentID)
	hostname := unifiedresources.NormalizeFullHostname(opts.Hostname)
	organizationID := strings.TrimSpace(opts.OrgID)
	if organizationID == "" {
		return ActionRunnerIssueResult{}, fmt.Errorf("%w: organization id is required", ErrRecord)
	}
	if !actionrunner.IsValidBoundedID(agentID) {
		return ActionRunnerIssueResult{}, fmt.Errorf("%w: canonical agent id is invalid", ErrRecord)
	}
	if hostname == "" {
		return ActionRunnerIssueResult{}, fmt.Errorf("%w: canonical hostname is required", ErrRecord)
	}
	if len(hostname) > 253 {
		return ActionRunnerIssueResult{}, fmt.Errorf("%w: action runner identity exceeds maximum length", ErrRecord)
	}

	tokenName := strings.TrimSpace(opts.TokenName)
	if tokenName == "" {
		tokenName = "action-runner:" + hostname
	}
	rawToken, err := internalauth.GenerateAPIToken()
	if err != nil {
		return ActionRunnerIssueResult{}, fmt.Errorf("%w: %w", ErrGeneration, err)
	}
	record, err := config.NewAPITokenRecord(rawToken, tokenName, ActionRunnerScopes())
	if err != nil {
		return ActionRunnerIssueResult{}, fmt.Errorf("%w: %w", ErrRecord, err)
	}
	record.OrgID = organizationID
	setOwnerUserID(record, opts.OwnerUserID)
	record.Metadata = map[string]string{
		CredentialKindMetadataKey:                CredentialKindActionRunner,
		ActionCapabilityMetadataKey:              ActionCapabilityTypedV1,
		ActionBindingVersionMetadataKey:          ActionBindingVersion,
		ActionRunnerActivationPendingMetadataKey: "true",
		"bound_agent_id":                         agentID,
		"bound_hostname":                         hostname,
		"bound_at":                               time.Now().UTC().Format(time.RFC3339),
		IssuedAtMetadataKey:                      record.CreatedAt.UTC().Format(time.RFC3339),
	}
	if err := normalizeCredentialKind(record); err != nil {
		return ActionRunnerIssueResult{}, fmt.Errorf("%w: %w", ErrRecord, err)
	}
	deadline := time.Now().UTC().Add(ActionRunnerActivationWindow)
	record.ExpiresAt = &deadline

	config.Mu.Lock()
	defer config.Mu.Unlock()
	previousTokens := cloneAPITokenRecords(cfg.APITokens)
	nextTokens := make([]config.APITokenRecord, 0, len(cfg.APITokens)+1)
	replaced := make([]config.APITokenRecord, 0, 1)
	activeIDs := make([]string, 0, 1)
	for _, existing := range cfg.APITokens {
		matchingBinding := strings.TrimSpace(existing.OrgID) == organizationID &&
			strings.TrimSpace(existing.Metadata[CredentialKindMetadataKey]) == CredentialKindActionRunner &&
			strings.TrimSpace(existing.Metadata["bound_agent_id"]) == agentID
		if !matchingBinding {
			nextTokens = append(nextTokens, existing)
			continue
		}
		if strings.TrimSpace(existing.Metadata[ActionRunnerActivationPendingMetadataKey]) == "true" {
			replaced = append(replaced, existing.Clone())
			continue
		}
		activeIDs = append(activeIDs, strings.TrimSpace(existing.ID))
		nextTokens = append(nextTokens, existing)
	}
	if len(activeIDs) > 0 {
		record.Metadata[ActionRunnerReplacesTokenIDsMetadataKey] = strings.Join(activeIDs, ",")
	}
	cfg.APITokens = append(nextTokens, *record)
	cfg.SortAPITokens()
	if persistence != nil {
		if err := persistence.SaveAPITokens(cfg.APITokens); err != nil {
			cfg.APITokens = previousTokens
			cfg.SortAPITokens()
			return ActionRunnerIssueResult{}, fmt.Errorf("%w: %w", ErrPersist, err)
		}
	}

	return ActionRunnerIssueResult{Token: rawToken, Record: record, Replaced: replaced}, nil
}

// ActivateActionRunnerAndPersist commits a prepared credential and revokes the
// exact predecessor set in the same durable token-inventory transaction.
func ActivateActionRunnerAndPersist(cfg *config.Config, persistence *config.ConfigPersistence, tokenID, agentID, hostname string) (*config.APITokenRecord, []config.APITokenRecord, bool, error) {
	if cfg == nil {
		return nil, nil, false, fmt.Errorf("%w: config is required", ErrRecord)
	}
	tokenID = strings.TrimSpace(tokenID)
	agentID = strings.TrimSpace(agentID)
	hostname = unifiedresources.NormalizeFullHostname(hostname)
	if tokenID == "" || agentID == "" || hostname == "" {
		return nil, nil, false, fmt.Errorf("%w: complete activation identity is required", ErrRecord)
	}

	config.Mu.Lock()
	defer config.Mu.Unlock()
	index := -1
	for candidateIndex := range cfg.APITokens {
		candidate := &cfg.APITokens[candidateIndex]
		if candidate.ID == tokenID {
			index = candidateIndex
			break
		}
	}
	if index < 0 {
		return nil, nil, false, fmt.Errorf("%w: action runner credential not found", ErrRecord)
	}
	record := &cfg.APITokens[index]
	if record.IsExpired() || strings.TrimSpace(record.Metadata[CredentialKindMetadataKey]) != CredentialKindActionRunner ||
		strings.TrimSpace(record.Metadata["bound_agent_id"]) != agentID ||
		!unifiedresources.HostnamesEquivalent(record.Metadata["bound_hostname"], hostname) {
		return nil, nil, false, fmt.Errorf("%w: action runner activation binding mismatch", ErrRecord)
	}
	if strings.TrimSpace(record.Metadata[ActionRunnerActivationPendingMetadataKey]) != "true" {
		clone := record.Clone()
		return &clone, nil, false, nil
	}

	previousTokens := cloneAPITokenRecords(cfg.APITokens)
	replaceIDs := make(map[string]struct{})
	for _, replacedID := range strings.Split(record.Metadata[ActionRunnerReplacesTokenIDsMetadataKey], ",") {
		if replacedID = strings.TrimSpace(replacedID); replacedID != "" && replacedID != tokenID {
			replaceIDs[replacedID] = struct{}{}
		}
	}
	delete(record.Metadata, ActionRunnerActivationPendingMetadataKey)
	delete(record.Metadata, ActionRunnerReplacesTokenIDsMetadataKey)
	record.ExpiresAt = nil
	activated := record.Clone()
	revoked := make([]config.APITokenRecord, 0, len(replaceIDs))
	nextTokens := make([]config.APITokenRecord, 0, len(cfg.APITokens))
	for _, existing := range cfg.APITokens {
		if _, remove := replaceIDs[existing.ID]; remove {
			revoked = append(revoked, existing.Clone())
			continue
		}
		nextTokens = append(nextTokens, existing)
	}
	cfg.APITokens = nextTokens
	cfg.SortAPITokens()
	if persistence != nil {
		if err := persistence.SaveAPITokens(cfg.APITokens); err != nil {
			cfg.APITokens = previousTokens
			cfg.SortAPITokens()
			return nil, nil, false, fmt.Errorf("%w: %w", ErrPersist, err)
		}
	}
	return &activated, revoked, true, nil
}

func normalizeCredentialKind(record *config.APITokenRecord) error {
	if record == nil {
		return nil
	}
	kind := strings.TrimSpace(record.Metadata[CredentialKindMetadataKey])
	hasExec := record.HasScope(config.ScopeAgentExec)
	if kind == "" {
		if hasExec {
			// Existing combined collector/command issuance remains available only
			// as an explicit compatibility class while deployments migrate.
			kind = CredentialKindLegacyFullTrust
		} else {
			kind = CredentialKindMonitoringCollector
		}
		record.Metadata[CredentialKindMetadataKey] = kind
	}
	switch kind {
	case CredentialKindMonitoringCollector:
		if err := internalauth.ValidateRoleScopes(kind, record.Scopes); err != nil {
			return err
		}
	case CredentialKindActionRunner:
		if err := internalauth.ValidateRoleScopes(kind, record.Scopes); err != nil {
			return err
		}
		if strings.TrimSpace(record.Metadata[ActionCapabilityMetadataKey]) != ActionCapabilityTypedV1 {
			return errors.New("action runner credential requires the typed action capability")
		}
	case CredentialKindLegacyFullTrust:
		if !hasExec {
			return errors.New("legacy full-trust credential requires agent:exec")
		}
	default:
		return fmt.Errorf("unsupported agent credential kind %q", kind)
	}
	return nil
}

func OwnerUserID(record config.APITokenRecord) string {
	return strings.TrimSpace(record.Metadata[OwnerUserIDMetadataKey])
}

func setOwnerUserID(record *config.APITokenRecord, ownerUserID string) {
	ownerUserID = strings.TrimSpace(ownerUserID)
	if record == nil || ownerUserID == "" || strings.HasPrefix(ownerUserID, "token:") {
		return
	}
	if record.Metadata == nil {
		record.Metadata = make(map[string]string)
	}
	record.Metadata[OwnerUserIDMetadataKey] = ownerUserID
}

func mergeMetadata(record *config.APITokenRecord, metadata map[string]string) error {
	if record == nil || len(metadata) == 0 {
		return nil
	}
	if record.Metadata == nil {
		record.Metadata = make(map[string]string)
	}
	for key, value := range metadata {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if key == OwnerUserIDMetadataKey {
			return fmt.Errorf("reserved token metadata key %q cannot be supplied by caller metadata", OwnerUserIDMetadataKey)
		}
		record.Metadata[key] = value
	}
	return nil
}
