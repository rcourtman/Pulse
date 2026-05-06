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
	ConnectionTypePVE          ConnectionType = "pve"
	ConnectionTypePBS          ConnectionType = "pbs"
	ConnectionTypePMG          ConnectionType = "pmg"
	ConnectionTypeVMware       ConnectionType = "vmware"
	ConnectionTypeTrueNAS      ConnectionType = "truenas"
	ConnectionTypeAgent        ConnectionType = "agent"
	ConnectionTypeDocker       ConnectionType = "docker"
	ConnectionTypeKubernetes   ConnectionType = "kubernetes"
	ConnectionTypeAvailability ConnectionType = "availability"
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

// ConnectionFleetGovernance is the canonical fleet-control projection for a
// connection row. Every field is derived from existing connection/runtime
// signals; the connections ledger does not persist a second fleet registry.
type ConnectionFleetGovernance struct {
	EnrollmentState  string `json:"enrollmentState"`
	LivenessState    string `json:"livenessState"`
	VersionDrift     string `json:"versionDrift"`
	AdapterHealth    string `json:"adapterHealth"`
	ConfigRollout    string `json:"configRollout"`
	CredentialStatus string `json:"credentialStatus"`
	UpdateStatus     string `json:"updateStatus"`
	RemoteControl    string `json:"remoteControl"`
}

// ConnectionError is the runtime error shape surfaced on a connection row.
// Mirrors monitoring.ErrorDetail but lives in the api package so the type
// stays stable if the internal monitoring shape evolves.
type ConnectionError struct {
	At       time.Time `json:"at"`
	Message  string    `json:"message"`
	Category string    `json:"category,omitempty"`
}

// ConnectionAgentIdentity carries host identity facts Pulse already knows for
// an agent-backed source. The infrastructure settings surface uses this to
// render compact standalone-host identity without forcing every consumer to
// query the broader host inventory.
type ConnectionAgentIdentity struct {
	Hostname        string `json:"hostname,omitempty"`
	Platform        string `json:"platform,omitempty"`
	OSName          string `json:"osName,omitempty"`
	OSVersion       string `json:"osVersion,omitempty"`
	KernelVersion   string `json:"kernelVersion,omitempty"`
	Architecture    string `json:"architecture,omitempty"`
	ReportIP        string `json:"reportIp,omitempty"`
	CommandsEnabled bool   `json:"commandsEnabled,omitempty"`
}

// Connection is the unified row the frontend consumes. It replaces the
// per-type shapes that today require separate fetches and separate table
// renderers.
type Connection struct {
	ID                   string                    `json:"id"`
	Type                 ConnectionType            `json:"type"`
	Name                 string                    `json:"name"`
	Address              string                    `json:"address"`
	HostAliases          []string                  `json:"hostAliases,omitempty"`
	State                ConnectionState           `json:"state"`
	StateReason          string                    `json:"stateReason,omitempty"`
	Enabled              bool                      `json:"enabled"`
	Surfaces             []string                  `json:"surfaces"`
	Scope                map[string]bool           `json:"scope"`
	LastSeen             *time.Time                `json:"lastSeen,omitempty"`
	LastError            *ConnectionError          `json:"lastError,omitempty"`
	Source               ConnectionSource          `json:"source"`
	AgentIdentity        *ConnectionAgentIdentity  `json:"agentIdentity,omitempty"`
	AgentVersion         string                    `json:"agentVersion,omitempty"`
	ExpectedAgentVersion string                    `json:"expectedAgentVersion,omitempty"`
	AgentUpdateAvailable bool                      `json:"agentUpdateAvailable,omitempty"`
	Fleet                ConnectionFleetGovernance `json:"fleet"`
	Capabilities         ConnectionCapabilities    `json:"capabilities"`
}

type ConnectionSystemComponentRole string

const (
	ConnectionSystemComponentRolePrimary    ConnectionSystemComponentRole = "primary"
	ConnectionSystemComponentRoleAttachment ConnectionSystemComponentRole = "attachment"
)

// ConnectionSystemComponent identifies one underlying connection that
// contributes to a grouped infrastructure source row in settings.
type ConnectionSystemComponent struct {
	ConnectionID string                        `json:"connectionId"`
	Type         ConnectionType                `json:"type"`
	Role         ConnectionSystemComponentRole `json:"role"`
}

// ConnectionSystemMember identifies one runtime member that composes a grouped
// infrastructure source row. For Proxmox clusters this carries the canonical
// cluster node list so the frontend can render node composition beneath the
// owning cluster source instead of reopening standalone host rows.
type ConnectionSystemMember struct {
	ID                string          `json:"id"`
	Name              string          `json:"name"`
	Endpoint          string          `json:"endpoint,omitempty"`
	HostAliases       []string        `json:"hostAliases,omitempty"`
	State             ConnectionState `json:"state"`
	LastSeen          *time.Time      `json:"lastSeen,omitempty"`
	Primary           bool            `json:"primary,omitempty"`
	AgentConnectionID string          `json:"agentConnectionId,omitempty"`
}

// ConnectionSystem is the source-oriented grouping contract for the settings
// infrastructure manager. One row owns a primary connection and can carry
// attached collection methods such as a unified agent augmenting a Proxmox
// source.
type ConnectionSystem struct {
	ID          string                      `json:"id"`
	Type        ConnectionType              `json:"type"`
	ClusterName string                      `json:"clusterName,omitempty"`
	Components  []ConnectionSystemComponent `json:"components"`
	Members     []ConnectionSystemMember    `json:"members,omitempty"`
}

// ConnectionsListResponse is the envelope for GET /api/connections.
type ConnectionsListResponse struct {
	Connections []Connection       `json:"connections"`
	Systems     []ConnectionSystem `json:"systems,omitempty"`
}
