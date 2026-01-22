package models

import "time"

// Organization represents a distinct tenant in the system.
type Organization struct {
	// ID is the unique identifier for the organization (e.g., "customer-a").
	// It is used as the directory name for data isolation.
	ID string `json:"id"`

	// DisplayName is the human-readable name of the organization.
	DisplayName string `json:"displayName"`

	// CreatedAt is when the organization was registered.
	CreatedAt time.Time `json:"createdAt"`

	// EncryptionKeyID refers to the specific encryption key used for this org's data
	// (Future proofing for per-tenant encryption keys)
	EncryptionKeyID string `json:"encryptionKeyId,omitempty"`
}
