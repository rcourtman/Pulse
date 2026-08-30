// Package agenttokens owns the security contract for install-token issuance.
package agenttokens

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

const (
	IssuedAtMetadataKey                    = "install_issued_at"
	OwnerUserIDMetadataKey                 = "owner_user_id"
	CommandPolicyIntentMetadataKey         = "command_policy_intent"
	CommandPolicyAppliedAgentIDMetadataKey = "command_policy_applied_agent_id"
	CommandPolicyIntentEnabled             = "enabled"
	CommandPolicyIntentDisabled            = "disabled"
	RuntimeRoleMetadataKey                 = "runtime_role"
	CredentialKindMetadataKey              = RuntimeRoleMetadataKey
	CredentialKindMonitoringCollector      = "monitoring-collector"
	CredentialKindActionRunner             = "action-runner"
	CredentialKindLegacyFullTrust          = "legacy-full-trust"
	ActionCapabilityMetadataKey            = "agent_action_capability"
	ActionCapabilityTypedV1                = "typed_actions.v1"
	ActionBindingVersionMetadataKey        = "action_runner_binding_version"
	ActionBindingVersion                   = "1"
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

func ProxmoxScopes(enableCommands bool) []string {
	scopes := []string{
		config.ScopeAgentReport,
		config.ScopeAgentConfigRead,
		config.ScopeAgentManage,
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
		config.ScopeAgentManage,
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

	previousTokens := append([]config.APITokenRecord(nil), cfg.APITokens...)
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

// IssueActionRunnerAndPersistDetailed is the rotation-aware form used by the
// API boundary. Replaced contains records only when the new credential was
// durably committed; persistence failure returns an empty result.
func IssueActionRunnerAndPersistDetailed(cfg *config.Config, persistence *config.ConfigPersistence, opts ActionRunnerIssueOptions) (ActionRunnerIssueResult, error) {
	agentID := strings.TrimSpace(opts.AgentID)
	hostname := unifiedresources.NormalizeFullHostname(opts.Hostname)
	organizationID := strings.TrimSpace(opts.OrgID)
	if organizationID == "" {
		return ActionRunnerIssueResult{}, fmt.Errorf("%w: organization id is required", ErrRecord)
	}
	if agentID == "" {
		return ActionRunnerIssueResult{}, fmt.Errorf("%w: canonical agent id is required", ErrRecord)
	}
	if hostname == "" {
		return ActionRunnerIssueResult{}, fmt.Errorf("%w: canonical hostname is required", ErrRecord)
	}
	if len(agentID) > 128 || len(hostname) > 253 {
		return ActionRunnerIssueResult{}, fmt.Errorf("%w: action runner identity exceeds maximum length", ErrRecord)
	}

	tokenName := strings.TrimSpace(opts.TokenName)
	if tokenName == "" {
		tokenName = "action-runner:" + hostname
	}
	rawToken, record, replaced, err := issueAndPersistReplacing(cfg, persistence, IssueOptions{
		TokenName:   tokenName,
		OrgID:       organizationID,
		OwnerUserID: opts.OwnerUserID,
		Scopes:      ActionRunnerScopes(),
		Metadata: map[string]string{
			CredentialKindMetadataKey:       CredentialKindActionRunner,
			ActionCapabilityMetadataKey:     ActionCapabilityTypedV1,
			ActionBindingVersionMetadataKey: ActionBindingVersion,
			"bound_agent_id":                agentID,
			"bound_hostname":                hostname,
			"bound_at":                      time.Now().UTC().Format(time.RFC3339),
		},
	}, func(record config.APITokenRecord) bool {
		return strings.TrimSpace(record.OrgID) == organizationID &&
			strings.TrimSpace(record.Metadata[CredentialKindMetadataKey]) == CredentialKindActionRunner &&
			strings.TrimSpace(record.Metadata["bound_agent_id"]) == agentID
	})
	if err != nil {
		return ActionRunnerIssueResult{}, err
	}
	return ActionRunnerIssueResult{Token: rawToken, Record: record, Replaced: replaced}, nil
}

func normalizeCredentialKind(record *config.APITokenRecord) error {
	if record == nil {
		return nil
	}
	kind := strings.TrimSpace(record.Metadata[CredentialKindMetadataKey])
	hasExec := record.HasScope(config.ScopeAgentExec)
	switch kind {
	case "":
		if hasExec {
			// Existing combined collector/command issuance remains available only
			// as an explicit compatibility class while deployments migrate.
			record.Metadata[CredentialKindMetadataKey] = CredentialKindLegacyFullTrust
		} else {
			record.Metadata[CredentialKindMetadataKey] = CredentialKindMonitoringCollector
		}
	case CredentialKindMonitoringCollector:
		if hasExec {
			return errors.New("monitoring collector credential cannot carry agent:exec")
		}
	case CredentialKindActionRunner:
		if !hasExec {
			return errors.New("action runner credential requires agent:exec")
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
