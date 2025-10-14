package config

import (
	"errors"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/rcourtman/pulse-go-rewrite/internal/auth"
)

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
}

// Clone returns a copy of the record with duplicated pointer fields.
func (r *APITokenRecord) Clone() APITokenRecord {
	clone := *r
	if r.LastUsedAt != nil {
		t := *r.LastUsedAt
		clone.LastUsedAt = &t
	}
	return clone
}

// NewAPITokenRecord constructs a metadata record from the provided raw token.
func NewAPITokenRecord(rawToken, name string) (*APITokenRecord, error) {
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
	}
	return record, nil
}

// NewHashedAPITokenRecord constructs a record from an already hashed token.
func NewHashedAPITokenRecord(hashedToken, name string, createdAt time.Time) (*APITokenRecord, error) {
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
			now := time.Now().UTC()
			c.APITokens[idx].LastUsedAt = &now
			return &c.APITokens[idx], true
		}
	}
	return nil, false
}

// UpsertAPIToken inserts or replaces a record by ID.
func (c *Config) UpsertAPIToken(record APITokenRecord) {
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

// RemoveAPIToken removes a token by ID.
func (c *Config) RemoveAPIToken(id string) bool {
	for idx, record := range c.APITokens {
		if record.ID == id {
			c.APITokens = append(c.APITokens[:idx], c.APITokens[idx+1:]...)
			return true
		}
	}
	return false
}

// SortAPITokens keeps tokens ordered newest-first and syncs the legacy APIToken field.
func (c *Config) SortAPITokens() {
	sort.SliceStable(c.APITokens, func(i, j int) bool {
		return c.APITokens[i].CreatedAt.After(c.APITokens[j].CreatedAt)
	})

	if len(c.APITokens) > 0 {
		c.APIToken = c.APITokens[0].Hash
		c.APITokenEnabled = true
	} else {
		c.APIToken = ""
	}
}
