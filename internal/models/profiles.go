package models

import (
	"time"
)

// AgentProfile represents a reusable configuration profile for agents.
type AgentProfile struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Config    AgentConfigMap `json:"config"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

// AgentConfigMap represents the key-value configuration overrides
// (e.g., {"interval": "10s", "enable_docker": true})
type AgentConfigMap map[string]interface{}

// AgentProfileAssignment maps an agent to a profile
type AgentProfileAssignment struct {
	AgentID   string    `json:"agent_id"`
	ProfileID string    `json:"profile_id"`
	UpdatedAt time.Time `json:"updated_at"`
}
