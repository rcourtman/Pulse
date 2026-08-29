// Package agenttokens owns the security contract for install-token issuance.
package agenttokens

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

const (
	IssuedAtMetadataKey                    = "install_issued_at"
	OwnerUserIDMetadataKey                 = "owner_user_id"
	CommandPolicyIntentMetadataKey         = "command_policy_intent"
	CommandPolicyAppliedAgentIDMetadataKey = "command_policy_applied_agent_id"
	CommandPolicyIntentEnabled             = "enabled"
	CommandPolicyIntentDisabled            = "disabled"
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
	if cfg == nil {
		return "", nil, fmt.Errorf("config is required")
	}

	rawToken, err := internalauth.GenerateAPIToken()
	if err != nil {
		return "", nil, fmt.Errorf("%w: %w", ErrGeneration, err)
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
		return "", nil, fmt.Errorf("%w: %w", ErrRecord, err)
	}

	record.OrgID = strings.TrimSpace(opts.OrgID)
	setOwnerUserID(record, opts.OwnerUserID)
	if err := mergeMetadata(record, opts.Metadata); err != nil {
		return "", nil, fmt.Errorf("%w: %w", ErrRecord, err)
	}
	if record.Metadata == nil {
		record.Metadata = make(map[string]string)
	}
	record.Metadata[IssuedAtMetadataKey] = record.CreatedAt.UTC().Format(time.RFC3339)

	config.Mu.Lock()
	defer config.Mu.Unlock()

	previousTokens := append([]config.APITokenRecord(nil), cfg.APITokens...)
	cfg.APITokens = append(cfg.APITokens, *record)
	cfg.SortAPITokens()
	if persistence != nil {
		if err := persistence.SaveAPITokens(cfg.APITokens); err != nil {
			// The generated record sorts newest-first, so truncating the sorted
			// inventory would discard an older valid token and keep the secret
			// that the failed request never returned. Restore the full snapshot.
			cfg.APITokens = previousTokens
			cfg.SortAPITokens()
			return "", nil, fmt.Errorf("%w: %w", ErrPersist, err)
		}
	}

	return rawToken, record, nil
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
