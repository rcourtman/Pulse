package config

import (
	"errors"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

// Canonical API token scope strings.
const (
	ScopeWildcard         = "*"
	ScopeMonitoringRead   = "monitoring:read"
	ScopeMonitoringWrite  = "monitoring:write"
	ScopeDockerReport     = "docker:report"
	ScopeDockerManage     = "docker:manage"
	ScopeKubernetesReport = "kubernetes:report"
	ScopeKubernetesManage = "kubernetes:manage"
	ScopeHostReport       = "host-agent:report"
	ScopeHostConfigRead   = "host-agent:config:read"
	ScopeHostManage       = "host-agent:manage"
	ScopeSettingsRead     = "settings:read"
	ScopeSettingsWrite    = "settings:write"
	ScopeAIExecute        = "ai:execute" // Allows executing AI commands and remediation plans
	ScopeAIChat           = "ai:chat"    // Allows AI chat participation
	ScopeAgentExec        = "agent:exec" // Allows agent execution WebSocket connections
)

// AllKnownScopes enumerates scopes recognized by the backend (excluding the wildcard sentinel).
var AllKnownScopes = []string{
	ScopeMonitoringRead,
	ScopeMonitoringWrite,
	ScopeDockerReport,
	ScopeDockerManage,
	ScopeKubernetesReport,
	ScopeKubernetesManage,
	ScopeHostReport,
	ScopeHostConfigRead,
	ScopeHostManage,
	ScopeSettingsRead,
	ScopeSettingsWrite,
	ScopeAIExecute,
	ScopeAIChat,
	ScopeAgentExec,
}

var scopeLookup = func() map[string]struct{} {
	lookup := make(map[string]struct{}, len(AllKnownScopes))
	for _, scope := range AllKnownScopes {
		lookup[scope] = struct{}{}
	}
	return lookup
}()

// ErrInvalidToken is returned when a token value is empty or malformed.
var ErrInvalidToken = errors.New("invalid API token")

// APITokenRecord stores hashed token metadata.
type APITokenRecord struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Hash       string     `json:"hash"`
	Prefix     string     `json:"prefix,omitempty"`
	Suffix     string     `json:"suffix,omitempty"`
	CreatedAt  time.Time  `json:"createdAt"`
	LastUsedAt *time.Time `json:"lastUsedAt,omitempty"`
	ExpiresAt  *time.Time `json:"expiresAt,omitempty"`
	Scopes     []string   `json:"scopes,omitempty"`

	// OrgID binds this token to a single organization.
	// If set, the token can only access resources within this organization.
	// Empty string means the token is not org-bound (legacy behavior with wildcard access).
	OrgID string `json:"orgId,omitempty"`

	// OrgIDs allows multi-org access for MSP tokens.
	// If set, the token can access resources within any of these organizations.
	// Takes precedence over OrgID if both are set.
	OrgIDs []string `json:"orgIds,omitempty"`

	// Metadata stores arbitrary key-value pairs for token binding.
	// Used to bind tokens to specific resources (e.g., bound_agent_id).
	Metadata map[string]string `json:"metadata,omitempty"`
}

// ensureScopes normalizes the scope slice, applying legacy defaults.
func (r *APITokenRecord) ensureScopes() {
	if len(r.Scopes) == 0 {
		r.Scopes = []string{ScopeWildcard}
		return
	}

	// Copy to avoid shared underlying slice if this record is reused.
	scopes := make([]string, len(r.Scopes))
	copy(scopes, r.Scopes)
	r.Scopes = scopes
}

// IsExpired reports whether the token has passed its expiration time.
func (r *APITokenRecord) IsExpired() bool {
	if r.ExpiresAt == nil {
		return false // No expiration set — token never expires.
	}
	return time.Now().UTC().After(*r.ExpiresAt)
}

// Clone returns a copy of the record with duplicated pointer fields.
func (r *APITokenRecord) Clone() APITokenRecord {
	clone := *r
	if r.LastUsedAt != nil {
		t := *r.LastUsedAt
		clone.LastUsedAt = &t
	}
	if r.ExpiresAt != nil {
		t := *r.ExpiresAt
		clone.ExpiresAt = &t
	}
	clone.ensureScopes()

	// Deep copy OrgIDs slice
	if len(r.OrgIDs) > 0 {
		clone.OrgIDs = make([]string, len(r.OrgIDs))
		copy(clone.OrgIDs, r.OrgIDs)
	}

	return clone
}

// CanAccessOrg checks if this token is authorized to access the specified organization.
// Returns true if:
// - Token has no org binding (legacy compatibility): default org only
// - Token's OrgID matches the requested orgID
// - Token's OrgIDs contains the requested orgID
// - Requested orgID is "default" and token is explicitly bound to "default"
func (r *APITokenRecord) CanAccessOrg(orgID string) bool {
	// Legacy tokens (no org binding) are restricted to the default org only.
	// This prevents cross-tenant access by unscoped tokens.
	if r.OrgID == "" && len(r.OrgIDs) == 0 {
		return orgID == "" || orgID == "default"
	}

	// Check multi-org binding first (takes precedence)
	if len(r.OrgIDs) > 0 {
		for _, boundOrgID := range r.OrgIDs {
			if boundOrgID == orgID {
				return true
			}
		}
		return false
	}

	// Check single-org binding
	return r.OrgID == orgID
}

// IsLegacyToken returns true if this token has no org binding (wildcard access).
func (r *APITokenRecord) IsLegacyToken() bool {
	return r.OrgID == "" && len(r.OrgIDs) == 0
}

// GetBoundOrgs returns all organizations this token is bound to.
// Returns nil for legacy tokens with wildcard access.
func (r *APITokenRecord) GetBoundOrgs() []string {
	if len(r.OrgIDs) > 0 {
		return r.OrgIDs
	}
	if r.OrgID != "" {
		return []string{r.OrgID}
	}
	return nil
}

// NewAPITokenRecord constructs a metadata record from the provided raw token.
func NewAPITokenRecord(rawToken, name string, scopes []string) (*APITokenRecord, error) {
	if rawToken == "" {
		return nil, ErrInvalidToken
	}

	now := time.Now().UTC()
	record := &APITokenRecord{
		ID:        uuid.NewString(),
		Name:      name,
		Hash:      auth.HashAPIToken(rawToken),
		Prefix:    tokenPrefix(rawToken),
		Suffix:    tokenSuffix(rawToken),
		CreatedAt: now,
		Scopes:    normalizeScopes(scopes),
	}
	return record, nil
}

// NewHashedAPITokenRecord constructs a record from an already hashed token.
func NewHashedAPITokenRecord(hashedToken, name string, createdAt time.Time, scopes []string) (*APITokenRecord, error) {
	if hashedToken == "" {
		return nil, ErrInvalidToken
	}
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}

	return &APITokenRecord{
		ID:        uuid.NewString(),
		Name:      name,
		Hash:      hashedToken,
		Prefix:    tokenPrefix(hashedToken),
		Suffix:    tokenSuffix(hashedToken),
		CreatedAt: createdAt,
		Scopes:    normalizeScopes(scopes),
	}, nil
}

// tokenPrefix returns the first six characters suitable for hints.
func tokenPrefix(value string) string {
	if len(value) >= 6 {
		return value[:6]
	}
	return value
}

// tokenSuffix returns the last four characters suitable for hints.
func tokenSuffix(value string) string {
	if len(value) >= 4 {
		return value[len(value)-4:]
	}
	return value
}

// HasAPITokens reports whether any API tokens are configured.
func (c *Config) HasAPITokens() bool {
	return len(c.APITokens) > 0
}

// APITokenCount returns the number of configured tokens.
func (c *Config) APITokenCount() int {
	return len(c.APITokens)
}

// ActiveAPITokenHashes returns all stored token hashes.
func (c *Config) ActiveAPITokenHashes() []string {
	hashes := make([]string, 0, len(c.APITokens))
	for _, record := range c.APITokens {
		if record.Hash != "" {
			hashes = append(hashes, record.Hash)
		}
	}
	return hashes
}

// HasAPITokenHash returns true when the hash already exists.
func (c *Config) HasAPITokenHash(hash string) bool {
	for _, record := range c.APITokens {
		if record.Hash == hash {
			return true
		}
	}
	return false
}

// IsEnvMigrationSuppressed returns true if the given hash was a migrated env token
// that the user explicitly deleted (and should not be re-migrated on restart).
func (c *Config) IsEnvMigrationSuppressed(hash string) bool {
	for _, h := range c.SuppressedEnvMigrations {
		if h == hash {
			return true
		}
	}
	return false
}

// SuppressEnvMigration adds a hash to the suppression list to prevent re-migration.
func (c *Config) SuppressEnvMigration(hash string) {
	if c.IsEnvMigrationSuppressed(hash) {
		return
	}
	c.SuppressedEnvMigrations = append(c.SuppressedEnvMigrations, hash)
}

// PrimaryAPITokenHash returns the newest token hash, if any.
func (c *Config) PrimaryAPITokenHash() string {
	if len(c.APITokens) == 0 {
		return ""
	}
	return c.APITokens[0].Hash
}

// PrimaryAPITokenHint provides a human-friendly token hint for UI display.
func (c *Config) PrimaryAPITokenHint() string {
	if len(c.APITokens) == 0 {
		return ""
	}
	token := c.APITokens[0]
	if token.Prefix != "" && token.Suffix != "" {
		return token.Prefix + "..." + token.Suffix
	}
	if len(token.Hash) >= 8 {
		return token.Hash[:4] + "..." + token.Hash[len(token.Hash)-4:]
	}
	return ""
}

// ValidateAPIToken compares the raw token against stored hashes and updates metadata.
func (c *Config) ValidateAPIToken(rawToken string) (*APITokenRecord, bool) {
	if rawToken == "" {
		return nil, false
	}

	for idx, record := range c.APITokens {
		if auth.CompareAPIToken(rawToken, record.Hash) {
			if c.APITokens[idx].IsExpired() {
				return nil, false
			}
			now := time.Now().UTC()
			c.APITokens[idx].LastUsedAt = &now
			c.APITokens[idx].ensureScopes()
			return &c.APITokens[idx], true
		}
	}
	return nil, false
}

// IsValidAPIToken checks if a token is valid without mutating any metadata.
// Use this for read-only checks like admin verification where you don't need
// to update LastUsedAt or get the full record. Safe to call under RLock.
func (c *Config) IsValidAPIToken(rawToken string) bool {
	if rawToken == "" {
		return false
	}

	for _, record := range c.APITokens {
		if auth.CompareAPIToken(rawToken, record.Hash) {
			if record.IsExpired() {
				return false
			}
			return true
		}
	}
	return false
}

// UpsertAPIToken inserts or replaces a record by ID.
func (c *Config) UpsertAPIToken(record APITokenRecord) {
	record.ensureScopes()
	for idx, existing := range c.APITokens {
		if existing.ID == record.ID {
			c.APITokens[idx] = record
			c.SortAPITokens()
			return
		}
	}
	c.APITokens = append(c.APITokens, record)
	c.SortAPITokens()
}

// RemoveAPIToken removes a token by ID and returns the removed record (if any).
func (c *Config) RemoveAPIToken(tokenID string) *APITokenRecord {
	for idx, record := range c.APITokens {
		if record.ID == tokenID {
			removed := record
			c.APITokens = append(c.APITokens[:idx], c.APITokens[idx+1:]...)
			return &removed
		}
	}
	return nil
}

// SortAPITokens keeps tokens ordered newest-first and syncs the legacy APIToken field.
func (c *Config) SortAPITokens() {
	for i := range c.APITokens {
		c.APITokens[i].ensureScopes()
	}
	sort.SliceStable(c.APITokens, func(i, j int) bool {
		return c.APITokens[i].CreatedAt.After(c.APITokens[j].CreatedAt)
	})

	if len(c.APITokens) > 0 {
		c.APIToken = c.APITokens[0].Hash
	} else {
		c.APIToken = ""
	}
}

// normalizeScopes applies defaults and returns a safe copy of the input slice.
func normalizeScopes(scopes []string) []string {
	if len(scopes) == 0 {
		return []string{ScopeWildcard}
	}
	result := make([]string, len(scopes))
	copy(result, scopes)
	return result
}

// HasScope reports whether the record grants the requested scope or wildcard access.
func (r *APITokenRecord) HasScope(scope string) bool {
	if scope == "" {
		return true
	}
	r.ensureScopes()
	for _, candidate := range r.Scopes {
		if candidate == ScopeWildcard || candidate == scope {
			return true
		}
	}
	return false
}

// IsKnownScope reports whether the provided string matches a supported scope identifier.
func IsKnownScope(scope string) bool {
	if scope == ScopeWildcard {
		return true
	}
	_, ok := scopeLookup[scope]
	return ok
}
