package api

import "time"

// ConnectionState is the derived lifecycle state shown in the unified
// connections ledger. Derivation rules live in connections_aggregator.go; no
// new state is persisted — every value comes from existing runtime signals.
type ConnectionState string

const (
	ConnectionStateActive       ConnectionState = "active"
	ConnectionStatePaused       ConnectionState = "paused"
	ConnectionStateUnauthorized ConnectionState = "unauthorized"
	ConnectionStateUnreachable  ConnectionState = "unreachable"
	ConnectionStateStale        ConnectionState = "stale"
	ConnectionStatePending      ConnectionState = "pending"
)

// ConnectionType is the product family that owns the connection. The value is
// the discriminator the frontend switches on to render per-type credential
// slots in ConnectionEditor.
type ConnectionType string

const (
	ConnectionTypePVE        ConnectionType = "pve"
	ConnectionTypePBS        ConnectionType = "pbs"
	ConnectionTypePMG        ConnectionType = "pmg"
	ConnectionTypeVMware     ConnectionType = "vmware"
	ConnectionTypeTrueNAS    ConnectionType = "truenas"
	ConnectionTypeAgent      ConnectionType = "agent"
	ConnectionTypeDocker     ConnectionType = "docker"
	ConnectionTypeKubernetes ConnectionType = "kubernetes"
)

// ConnectionSource records how a connection entered Pulse.
type ConnectionSource string

const (
	ConnectionSourceManual ConnectionSource = "manual"
	ConnectionSourceAgent  ConnectionSource = "agent"
	ConnectionSourceScript ConnectionSource = "script"
)

// ConnectionCapabilities tells the frontend which controls to render for a
// connection. Agents cannot pause or partial-scope; manual Proxmox/PBS/PMG
// can do both. This avoids putting the decision inside the editor.
type ConnectionCapabilities struct {
	SupportsPause bool `json:"supportsPause"`
	SupportsScope bool `json:"supportsScope"`
	SupportsTest  bool `json:"supportsTest"`
}

// ConnectionError is the runtime error shape surfaced on a connection row.
// Mirrors monitoring.ErrorDetail but lives in the api package so the type
// stays stable if the internal monitoring shape evolves.
type ConnectionError struct {
	At       time.Time `json:"at"`
	Message  string    `json:"message"`
	Category string    `json:"category,omitempty"`
}

// Connection is the unified row the frontend consumes. It replaces the
// per-type shapes that today require separate fetches and separate table
// renderers.
type Connection struct {
	ID           string                 `json:"id"`
	Type         ConnectionType         `json:"type"`
	Name         string                 `json:"name"`
	Address      string                 `json:"address"`
	State        ConnectionState        `json:"state"`
	StateReason  string                 `json:"stateReason,omitempty"`
	Enabled      bool                   `json:"enabled"`
	Surfaces     []string               `json:"surfaces"`
	Scope        map[string]bool        `json:"scope"`
	LastSeen     *time.Time             `json:"lastSeen,omitempty"`
	LastError    *ConnectionError       `json:"lastError,omitempty"`
	Source       ConnectionSource       `json:"source"`
	Capabilities ConnectionCapabilities `json:"capabilities"`
}

// ConnectionsListResponse is the envelope for GET /api/connections.
type ConnectionsListResponse struct {
	Connections []Connection `json:"connections"`
}
