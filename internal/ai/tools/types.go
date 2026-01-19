package tools

import "context"

// AgentProfileManager manages centralized agent profiles and assignments.
type AgentProfileManager interface {
	ApplyAgentScope(ctx context.Context, agentID, agentLabel string, settings map[string]interface{}) (profileID, profileName string, created bool, err error)
	AssignProfile(ctx context.Context, agentID, profileID string) (profileName string, err error)
	GetAgentScope(ctx context.Context, agentID string) (*AgentScope, error)
}

// AgentScope summarizes profile scope applied to an agent.
type AgentScope struct {
	AgentID        string
	ProfileID      string
	ProfileName    string
	ProfileVersion int
	Settings       map[string]interface{}
}
