package models

import (
	"time"
)

// AgentProfile represents a reusable configuration profile for agents.
type AgentProfile struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Config      AgentConfigMap `json:"config"`
	Version     int            `json:"version"`             // Auto-incremented on each update
	ParentID    string         `json:"parent_id,omitempty"` // For profile inheritance
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	CreatedBy   string         `json:"created_by,omitempty"`
	UpdatedBy   string         `json:"updated_by,omitempty"`
}

// AgentConfigMap represents the key-value configuration overrides
// (e.g., {"interval": "10s", "enable_docker": true})
type AgentConfigMap map[string]interface{}

// AgentProfileAssignment maps an agent to a profile
type AgentProfileAssignment struct {
	AgentID        string    `json:"agent_id"`
	ProfileID      string    `json:"profile_id"`
	ProfileVersion int       `json:"profile_version"` // Version at time of assignment
	UpdatedAt      time.Time `json:"updated_at"`
	AssignedBy     string    `json:"assigned_by,omitempty"`
}

// AgentProfileVersion represents a historical version of a profile.
type AgentProfileVersion struct {
	ProfileID   string         `json:"profile_id"`
	Version     int            `json:"version"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Config      AgentConfigMap `json:"config"`
	ParentID    string         `json:"parent_id,omitempty"`
	CreatedAt   time.Time      `json:"created_at"` // When this version was created
	CreatedBy   string         `json:"created_by,omitempty"`
	ChangeNote  string         `json:"change_note,omitempty"` // Optional note about the change
}

// ProfileDeploymentStatus tracks which version an agent has received.
type ProfileDeploymentStatus struct {
	AgentID          string    `json:"agent_id"`
	ProfileID        string    `json:"profile_id"`
	AssignedVersion  int       `json:"assigned_version"` // Version that should be deployed
	DeployedVersion  int       `json:"deployed_version"` // Version actually deployed
	LastDeployedAt   time.Time `json:"last_deployed_at"`
	DeploymentStatus string    `json:"deployment_status"` // "pending", "deployed", "failed"
	ErrorMessage     string    `json:"error_message,omitempty"`
}

// ProfileChangeLog represents an audit entry for profile changes.
type ProfileChangeLog struct {
	ID          string    `json:"id"`
	ProfileID   string    `json:"profile_id"`
	ProfileName string    `json:"profile_name"`
	Action      string    `json:"action"` // "create", "update", "delete", "assign", "unassign", "rollback"
	OldVersion  int       `json:"old_version,omitempty"`
	NewVersion  int       `json:"new_version,omitempty"`
	AgentID     string    `json:"agent_id,omitempty"` // For assign/unassign actions
	User        string    `json:"user,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
	Details     string    `json:"details,omitempty"`
}

// MergedConfig returns the effective configuration by merging parent configs.
// Parent configs are applied first, then overridden by child configs.
func (p *AgentProfile) MergedConfig(profiles []AgentProfile) AgentConfigMap {
	return p.mergedConfig(profiles, map[string]struct{}{})
}

func (p *AgentProfile) mergedConfig(profiles []AgentProfile, visited map[string]struct{}) AgentConfigMap {
	if p.ParentID == "" {
		return p.Config
	}

	if p.ID != "" {
		if _, seen := visited[p.ID]; seen {
			// Cycle in inheritance chain; stop walking parents and use local config.
			return p.Config
		}
		visited[p.ID] = struct{}{}
		defer delete(visited, p.ID)
	}

	// Find parent profile
	var parent *AgentProfile
	for i := range profiles {
		if profiles[i].ID == p.ParentID {
			parent = &profiles[i]
			break
		}
	}

	if parent == nil {
		return p.Config
	}

	// Get parent's merged config (recursive)
	parentConfig := parent.mergedConfig(profiles, visited)

	// Merge: start with parent config, override with current
	merged := make(AgentConfigMap)
	for k, v := range parentConfig {
		merged[k] = v
	}
	for k, v := range p.Config {
		merged[k] = v
	}

	return merged
}
