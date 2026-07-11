package auth

// Canonical API token scopes shared across Pulse repos.
const (
	ScopeWildcard          = "*"
	ScopeMonitoringRead    = "monitoring:read"
	ScopeMonitoringWrite   = "monitoring:write"
	ScopeDockerReport      = "docker:report"
	ScopeDockerManage      = "docker:manage"
	ScopeKubernetesReport  = "kubernetes:report"
	ScopeKubernetesManage  = "kubernetes:manage"
	ScopeAgentReport       = "agent:report"
	ScopeAgentConfigRead   = "agent:config:read"
	ScopeAgentManage       = "agent:manage"
	ScopeSettingsRead      = "settings:read"
	ScopeSettingsWrite     = "settings:write"
	ScopeAuditRead         = "audit:read"
	ScopeAIExecute         = "ai:execute"
	ScopeAIChat            = "ai:chat"
	ScopeRelayMobileAccess = "relay:mobile:access"
	ScopeAgentExec         = "agent:exec"
	ScopeAgentEnroll       = "agent:enroll"
	ScopeActionsPlan       = "actions:plan"
	ScopeActionsApprove    = "actions:approve"
	ScopeActionsExecute    = "actions:execute"
)

// AllKnownScopes enumerates canonical scopes, excluding the wildcard sentinel.
var AllKnownScopes = []string{
	ScopeMonitoringRead,
	ScopeMonitoringWrite,
	ScopeDockerReport,
	ScopeDockerManage,
	ScopeKubernetesReport,
	ScopeKubernetesManage,
	ScopeAgentReport,
	ScopeAgentConfigRead,
	ScopeAgentManage,
	ScopeSettingsRead,
	ScopeSettingsWrite,
	ScopeAuditRead,
	ScopeAIExecute,
	ScopeAIChat,
	ScopeRelayMobileAccess,
	ScopeAgentExec,
	ScopeAgentEnroll,
	ScopeActionsPlan,
	ScopeActionsApprove,
	ScopeActionsExecute,
}
