package configapi

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/api/agenttokens"
	"github.com/rcourtman/pulse-go-rewrite/internal/api/apicontext"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

const (
	OrgIDContextKey                    = apicontext.OrgIDContextKey
	agentInstallIssuedViaConfig        = "config_agent_install_command"
	agentInstallIssuedViaHosted        = "hosted_agent_install_command"
	agentInstallTypeHost               = "host"
	agentInstallCommandPolicyIntentKey = agenttokens.CommandPolicyIntentMetadataKey
)

var (
	errAgentInstallTokenGeneration = agenttokens.ErrGeneration
	errAgentInstallTokenRecord     = agenttokens.ErrRecord
	errAgentInstallTokenPersist    = agenttokens.ErrPersist
)

type GuestMetadataReloader interface {
	Reload(context.Context) error
}

type RuntimeDependencies struct {
	AuditEvent       func(orgID, event, user, ip, path string, success bool, details string)
	ClientIP         func(*http.Request) string
	AuthUsername     func(*config.Config, *http.Request) string
	TokenOwnerUserID func(*config.Config, *http.Request) string
	AuthConfigured   func(*config.Config) bool
	ResolvePublicURL func(*http.Request, *config.Config, bool) string
}

func defaultRuntimeDependencies() RuntimeDependencies {
	return RuntimeDependencies{
		AuditEvent: func(string, string, string, string, string, bool, string) {},
		ClientIP: func(r *http.Request) string {
			if r == nil {
				return ""
			}
			host, _, err := net.SplitHostPort(r.RemoteAddr)
			if err == nil {
				return host
			}
			return strings.Trim(r.RemoteAddr, "[]")
		},
		AuthUsername: func(_ *config.Config, r *http.Request) string {
			if r == nil {
				return ""
			}
			return internalauth.GetUser(r.Context())
		},
		TokenOwnerUserID: func(_ *config.Config, r *http.Request) string {
			if r == nil {
				return ""
			}
			return internalauth.GetUser(r.Context())
		},
		AuthConfigured: func(cfg *config.Config) bool {
			return cfg != nil && ((strings.TrimSpace(cfg.AuthUser) != "" && strings.TrimSpace(cfg.AuthPass) != "") ||
				cfg.HasAPITokens() || strings.TrimSpace(cfg.ProxyAuthSecret) != "")
		},
		ResolvePublicURL: defaultPublicURL,
	}
}

func (h *ConfigHandlers) SetRuntimeDependencies(deps RuntimeDependencies) {
	defaults := defaultRuntimeDependencies()
	if deps.AuditEvent == nil {
		deps.AuditEvent = defaults.AuditEvent
	}
	if deps.ClientIP == nil {
		deps.ClientIP = defaults.ClientIP
	}
	if deps.AuthUsername == nil {
		deps.AuthUsername = defaults.AuthUsername
	}
	if deps.TokenOwnerUserID == nil {
		deps.TokenOwnerUserID = defaults.TokenOwnerUserID
	}
	if deps.AuthConfigured == nil {
		deps.AuthConfigured = defaults.AuthConfigured
	}
	if deps.ResolvePublicURL == nil {
		deps.ResolvePublicURL = defaults.ResolvePublicURL
	}
	h.stateMu.Lock()
	h.runtime = deps
	h.stateMu.Unlock()
}

type apiTokenDTO struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Prefix      string     `json:"prefix"`
	Suffix      string     `json:"suffix"`
	CreatedAt   time.Time  `json:"createdAt"`
	LastUsedAt  *time.Time `json:"lastUsedAt,omitempty"`
	ExpiresAt   *time.Time `json:"expiresAt,omitempty"`
	Scopes      []string   `json:"scopes"`
	OwnerUserID string     `json:"ownerUserId,omitempty"`
}

type issueAgentInstallTokenOptions = agenttokens.IssueOptions

func issueAndPersistAgentInstallToken(cfg *config.Config, persistence *config.ConfigPersistence, opts issueAgentInstallTokenOptions) (string, *config.APITokenRecord, error) {
	return agenttokens.IssueAndPersist(cfg, persistence, opts)
}

func hostAgentInstallScopes(enableCommands bool) []string {
	return agenttokens.HostScopes(enableCommands)
}

func proxmoxAgentInstallScopes(enableCommands bool) []string {
	return agenttokens.ProxmoxScopes(enableCommands)
}

func agentInstallCommandPolicyIntent(enableCommands bool) string {
	return agenttokens.CommandPolicyIntent(enableCommands)
}

func (h *ConfigHandlers) authConfiguredForAgentLifecycle(cfg *config.Config) bool {
	return h.runtimeDependencies().AuthConfigured(cfg)
}

func (h *ConfigHandlers) apiTokenOwnerUserIDForRequest(cfg *config.Config, r *http.Request) string {
	return h.runtimeDependencies().TokenOwnerUserID(cfg, r)
}

func toAPITokenDTO(record config.APITokenRecord) apiTokenDTO {
	return apiTokenDTO{
		ID:          record.ID,
		Name:        record.Name,
		Prefix:      record.Prefix,
		Suffix:      record.Suffix,
		CreatedAt:   record.CreatedAt,
		LastUsedAt:  record.LastUsedAt,
		ExpiresAt:   record.ExpiresAt,
		Scopes:      append([]string{}, record.Scopes...),
		OwnerUserID: agenttokens.OwnerUserID(record),
	}
}

func (h *ConfigHandlers) resolveConfigAgentInstallBaseURL(req *http.Request, cfg *config.Config) string {
	return h.runtimeDependencies().ResolvePublicURL(req, cfg, h.hostedMode)
}

func writeConfigAgentInstallBaseURLUnavailable(w http.ResponseWriter) {
	http.Error(w, "A valid external Pulse URL is required", http.StatusServiceUnavailable)
}

func defaultPublicURL(req *http.Request, cfg *config.Config, hostedMode bool) string {
	if cfg == nil {
		return ""
	}
	if agentURL := strings.TrimSpace(cfg.AgentConnectURL); agentURL != "" {
		return strings.TrimRight(agentURL, "/")
	}
	publicURL := strings.TrimSpace(cfg.PublicURL)
	if publicURL != "" && (!cfg.PublicURLAutoDetected || hostedMode) {
		return strings.TrimRight(publicURL, "/")
	}
	if hostedMode {
		return ""
	}
	if req != nil && strings.TrimSpace(req.Host) != "" {
		scheme := "http"
		if req.TLS != nil {
			scheme = "https"
		}
		return scheme + "://" + strings.TrimSpace(req.Host)
	}
	if publicURL != "" {
		return strings.TrimRight(publicURL, "/")
	}
	if cfg.FrontendPort > 0 {
		return "http://localhost:" + fmt.Sprint(cfg.FrontendPort)
	}
	return "http://localhost:7655"
}

func (h *ConfigHandlers) runtimeDependencies() RuntimeDependencies {
	h.stateMu.RLock()
	deps := h.runtime
	h.stateMu.RUnlock()
	return deps
}

func safePrefixForLog(value string, n int) string {
	if n <= 0 || len(value) <= n {
		return value
	}
	return value[:n]
}

func explicitAPITokenFromRequest(r *http.Request) (string, bool) {
	if r == nil {
		return "", false
	}
	if values := r.Header.Values("X-API-Token"); len(values) > 0 {
		return strings.TrimSpace(values[0]), true
	}
	if header := r.Header.Get("Authorization"); strings.HasPrefix(strings.ToLower(header), "bearer ") {
		return strings.TrimSpace(header[7:]), true
	}
	if strings.EqualFold(strings.TrimSpace(r.Header.Get("Upgrade")), "websocket") {
		if values, ok := r.URL.Query()["token"]; ok && len(values) > 0 {
			return strings.TrimSpace(values[0]), true
		}
	}
	return "", false
}

func normalizeProxmoxInstallType(raw string) (string, error) {
	installType := strings.ToLower(strings.TrimSpace(raw))
	if installType != "pve" && installType != "pbs" {
		return "", fmt.Errorf("Type must be 'pve' or 'pbs'")
	}
	return installType, nil
}
