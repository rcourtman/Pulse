package api

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

const (
	proxmoxInstallTypePVE = "pve"
	proxmoxInstallTypePBS = "pbs"
)

var (
	errAgentInstallTokenGeneration = errors.New("agent install token generation failed")
	errAgentInstallTokenRecord     = errors.New("agent install token record failed")
	errAgentInstallTokenPersist    = errors.New("agent install token persistence failed")
)

func normalizeProxmoxInstallType(raw string) (string, error) {
	installType := strings.ToLower(strings.TrimSpace(raw))
	if installType != proxmoxInstallTypePVE && installType != proxmoxInstallTypePBS {
		return "", fmt.Errorf("Type must be 'pve' or 'pbs'")
	}
	return installType, nil
}

func proxmoxAgentInstallScopes() []string {
	return []string{
		config.ScopeHostReport,
		config.ScopeHostConfigRead,
		config.ScopeHostManage,
		config.ScopeAgentExec,
	}
}

type issueAgentInstallTokenOptions struct {
	TokenName string
	OrgID     string
	Metadata  map[string]string
}

func issueAndPersistAgentInstallToken(cfg *config.Config, persistence *config.ConfigPersistence, opts issueAgentInstallTokenOptions) (string, *config.APITokenRecord, error) {
	if cfg == nil {
		return "", nil, fmt.Errorf("config is required")
	}

	rawToken, err := internalauth.GenerateAPIToken()
	if err != nil {
		return "", nil, fmt.Errorf("%w: %w", errAgentInstallTokenGeneration, err)
	}

	record, err := config.NewAPITokenRecord(rawToken, opts.TokenName, proxmoxAgentInstallScopes())
	if err != nil {
		return "", nil, fmt.Errorf("%w: %w", errAgentInstallTokenRecord, err)
	}

	record.OrgID = strings.TrimSpace(opts.OrgID)
	if len(opts.Metadata) > 0 {
		record.Metadata = make(map[string]string, len(opts.Metadata))
		for k, v := range opts.Metadata {
			record.Metadata[k] = v
		}
	}

	config.Mu.Lock()
	defer config.Mu.Unlock()

	cfg.APITokens = append(cfg.APITokens, *record)
	cfg.SortAPITokens()
	if persistence != nil {
		if err := persistence.SaveAPITokens(cfg.APITokens); err != nil {
			cfg.APITokens = cfg.APITokens[:len(cfg.APITokens)-1]
			return "", nil, fmt.Errorf("%w: %w", errAgentInstallTokenPersist, err)
		}
	}

	return rawToken, record, nil
}

type agentInstallCommandOptions struct {
	BaseURL            string
	Token              string
	InstallType        string
	IncludeInstallType bool
}

func posixShellQuote(value string) string {
	escaped := strings.ReplaceAll(value, "'", `'"'"'`)
	return "'" + escaped + "'"
}

func buildProxmoxAgentInstallCommand(opts agentInstallCommandOptions) string {
	baseURL := strings.TrimRight(strings.TrimSpace(opts.BaseURL), "/")
	installScriptURL := baseURL + "/install.sh"
	command := fmt.Sprintf(`curl -fsSL %s | bash -s -- \
  --url %s \
  --token %s \
  --enable-proxmox`,
		posixShellQuote(installScriptURL), posixShellQuote(baseURL), posixShellQuote(opts.Token))

	if opts.IncludeInstallType {
		command += fmt.Sprintf(` \
  --proxmox-type %s`, posixShellQuote(opts.InstallType))
	}

	return command
}

func resolveConfigAgentInstallBaseURL(req *http.Request, cfg *config.Config) string {
	if cfg != nil {
		if agentConnectURL := strings.TrimSpace(cfg.AgentConnectURL); agentConnectURL != "" {
			return strings.TrimRight(agentConnectURL, "/")
		}
	}

	host := ""
	if req != nil {
		host = strings.TrimSpace(req.Host)
	}
	if host == "" {
		if cfg != nil && cfg.FrontendPort > 0 {
			host = fmt.Sprintf("localhost:%d", cfg.FrontendPort)
		} else {
			host = "localhost:7655"
		}
	}

	if cfg != nil {
		if parsedHost, parsedPort, err := net.SplitHostPort(host); err == nil {
			if (parsedHost == "127.0.0.1" || parsedHost == "localhost") && parsedPort == strconv.Itoa(cfg.FrontendPort) {
				if publicURL := strings.TrimSpace(cfg.PublicURL); publicURL != "" {
					if parsedURL, err := url.Parse(publicURL); err == nil && parsedURL.Host != "" {
						host = parsedURL.Host
					}
				}
			}
		}
	}

	scheme := "http"
	if req != nil && (req.TLS != nil || strings.EqualFold(req.Header.Get("X-Forwarded-Proto"), "https")) {
		scheme = "https"
	}

	return fmt.Sprintf("%s://%s", scheme, host)
}
