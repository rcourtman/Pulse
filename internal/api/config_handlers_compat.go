package api

import (
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/api/configapi"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/websocket"
)

const agentInstallTokenIssuedAtKey = "install_issued_at"

const proxmoxInstallBootstrapGrantTTL = configapi.ProxmoxInstallBootstrapGrantTTL
const proxmoxInstallGrantExpiredMessage = configapi.ProxmoxInstallGrantExpiredMessage

type ConfigHandlers = configapi.ConfigHandlers
type SetupTokenRecord = configapi.SetupTokenRecord
type RecentSetupTokenRecord = configapi.RecentSetupTokenRecord
type NodeConfigRequest = configapi.NodeConfigRequest
type ClusterEndpointOverrideRequest = configapi.ClusterEndpointOverrideRequest
type ClusterNodeDisplayNameOverrideRequest = configapi.ClusterNodeDisplayNameOverrideRequest
type NodeResponse = configapi.NodeResponse
type ClusterEndpointResponse = configapi.ClusterEndpointResponse
type AutoRegisterRequest = configapi.AutoRegisterRequest
type AutoRegisterResponse = configapi.AutoRegisterResponse
type AutoUnregisterRequest = configapi.AutoUnregisterRequest
type AutoUnregisterResponse = configapi.AutoUnregisterResponse
type SSHKeyPair = configapi.SSHKeyPair
type AgentInstallCommandRequest = configapi.AgentInstallCommandRequest
type AgentInstallCommandResponse = configapi.AgentInstallCommandResponse
type ExportConfigRequest = configapi.ExportConfigRequest
type ImportConfigRequest = configapi.ImportConfigRequest
type setupScriptRenderContext = configapi.SetupScriptRenderContext

func NewConfigHandlers(mtp *config.MultiTenantPersistence, mtm *monitoring.MultiTenantMonitor, reloadFunc func() error, wsHub *websocket.Hub, guestMetadataHandler *GuestMetadataHandler, reloadSystemSettingsFunc func(), hostedMode bool) *ConfigHandlers {
	handler := configapi.NewConfigHandlers(mtp, mtm, reloadFunc, wsHub, guestMetadataHandler, reloadSystemSettingsFunc, hostedMode)
	handler.SetRuntimeDependencies(configapi.RuntimeDependencies{
		AuditEvent:       LogAuditEventForTenant,
		ClientIP:         GetClientIP,
		AuthUsername:     getAuthUsername,
		TokenOwnerUserID: apiTokenOwnerUserIDForRequest,
		AuthConfigured:   authConfiguredForAgentLifecycle,
		ResolvePublicURL: resolveConfiguredPublicBaseURL,
	})
	return handler
}

func pulseTokenHostCandidate(candidate string) string {
	return configapi.PulseTokenHostCandidate(candidate)
}

func buildPulseMonitorTokenName(candidates ...string) string {
	return configapi.BuildPulseMonitorTokenName(candidates...)
}

func hostsShareResolvedIdentity(existingHost, candidateHost string) bool {
	return configapi.HostsShareResolvedIdentity(existingHost, candidateHost)
}

func isCanonicalAutoRegisterType(nodeType string) bool {
	return configapi.IsCanonicalAutoRegisterType(nodeType)
}

func isCanonicalAutoRegisterTokenID(nodeType, tokenID string) bool {
	return configapi.IsCanonicalAutoRegisterTokenID(nodeType, tokenID)
}

func isCanonicalAutoRegisterSource(source string) bool {
	return configapi.IsCanonicalAutoRegisterSource(source)
}

func canonicalAutoRegisterMatchMessage(reason string) string {
	return configapi.CanonicalAutoRegisterMatchMessage(reason)
}

func canonicalAutoRegisterCompletionPayloadMessage() string {
	return configapi.CanonicalAutoRegisterCompletionPayloadMessage()
}

func canonicalAutoRegisterMissingFieldsMessage(typeValue, host string, hasTokenID bool, serverName string) string {
	return configapi.CanonicalAutoRegisterMissingFieldsMessage(typeValue, host, hasTokenID, serverName)
}

func canonicalAutoUnregisterMissingFieldsMessage(typeValue, host, serverName string) string {
	return configapi.CanonicalAutoUnregisterMissingFieldsMessage(typeValue, host, serverName)
}

func normalizePBSUser(user string) string { return configapi.NormalizePBSUser(user) }
func normalizePMGUser(user string) string { return configapi.NormalizePMGUser(user) }

func canonicalAutoRegisterCheckMissingFieldsMessage(typeValue, host, serverName string) string {
	return configapi.CanonicalAutoRegisterCheckMissingFieldsMessage(typeValue, host, serverName)
}

func canBootstrapProxmoxInstallRegistrationAt(record *config.APITokenRecord, req *AutoRegisterRequest, now time.Time) bool {
	return configapi.CanBootstrapProxmoxInstallRegistrationAt(record, req, now)
}

func proxmoxInstallGrantEligible(record *config.APITokenRecord, req *AutoRegisterRequest) bool {
	return configapi.ProxmoxInstallGrantEligible(record, req)
}

func deriveSetupScriptServerName(serverHost string) string {
	return configapi.DeriveSetupScriptServerName(serverHost)
}

func renderSetupScript(serverType string, ctx setupScriptRenderContext) string {
	return configapi.RenderSetupScript(serverType, ctx)
}
