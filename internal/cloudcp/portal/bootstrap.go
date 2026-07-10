package portal

import (
	"encoding/json"
	"html/template"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
)

type BootstrapWorkspace struct {
	ID                          string `json:"id"`
	DisplayName                 string `json:"display_name"`
	State                       string `json:"state"`
	Healthy                     bool   `json:"healthy"`
	HealthStatus                string `json:"health_status"`
	SetupStatus                 string `json:"setup_status,omitempty"`
	AgentCount                  *int   `json:"agent_count,omitempty"`
	AgentTokenCount             *int   `json:"agent_token_count,omitempty"`
	UnusedAgentTokenCount       *int   `json:"unused_agent_token_count,omitempty"`
	LastAgentSeenAt             string `json:"last_agent_seen_at,omitempty"`
	AlertRouteCount             *int   `json:"alert_route_count,omitempty"`
	DisabledAlertRouteCount     *int   `json:"disabled_alert_route_count,omitempty"`
	ActiveCriticalAlertCount    *int   `json:"active_critical_alert_count,omitempty"`
	ActiveWarningAlertCount     *int   `json:"active_warning_alert_count,omitempty"`
	ActiveAlertsUpdatedAt       string `json:"active_alerts_updated_at,omitempty"`
	ReportScheduleCount         *int   `json:"report_schedule_count,omitempty"`
	DisabledReportScheduleCount *int   `json:"disabled_report_schedule_count,omitempty"`
	LastHealthCheck             string `json:"last_health_check,omitempty"`
	CreatedAt                   string `json:"created_at"`
}

type BootstrapMember struct {
	SubjectID string `json:"subject_id"`
	UserID    string `json:"user_id,omitempty"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	State     string `json:"state"`
	CreatedAt string `json:"created_at,omitempty"`
}

type BootstrapSetupTemplate struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	AgentNaming  string `json:"agent_naming"`
	AlertRouting string `json:"alert_routing"`
	Reporting    string `json:"reporting"`
	Access       string `json:"access"`
}

type BootstrapAccount struct {
	ID             string                   `json:"id"`
	Kind           string                   `json:"kind"`
	KindLabel      string                   `json:"kind_label"`
	Name           string                   `json:"name"`
	Role           string                   `json:"role"`
	CanManage      bool                     `json:"can_manage"`
	HasBilling     bool                     `json:"has_billing"`
	Workspaces     []BootstrapWorkspace     `json:"workspaces"`
	Members        []BootstrapMember        `json:"members"`
	SetupTemplates []BootstrapSetupTemplate `json:"setup_templates,omitempty"`
}

// PortalEnvironment carries control-plane runtime facts the portal frontend
// needs to render honestly: whether emailed sign-in links can actually be
// delivered, and whether this is a provider-hosted (self-operated) control
// plane rather than Pulse-hosted cloud.
type PortalEnvironment struct {
	SignupPath           string
	EmailSignInAvailable bool
	ProviderHostedMode   bool
}

// DefaultPortalEnvironment matches the historical behavior: Pulse-hosted
// cloud with a working transactional email provider.
func DefaultPortalEnvironment(signupPath string) PortalEnvironment {
	return PortalEnvironment{SignupPath: signupPath, EmailSignInAvailable: true}
}

type BootstrapData struct {
	Authenticated           bool               `json:"authenticated"`
	Email                   string             `json:"email"`
	HasSelfHostedCommercial bool               `json:"has_self_hosted_commercial"`
	EmailSignInAvailable    bool               `json:"email_sign_in_available"`
	ProviderHostedMode      bool               `json:"provider_hosted_mode"`
	PublicSiteURL           string             `json:"public_site_url"`
	SupportEmail            string             `json:"support_email"`
	CommercialAPIBaseURL    string             `json:"commercial_api_base_url"`
	CommercialAPIBasePath   string             `json:"commercial_api_base_path"`
	PortalPath              string             `json:"portal_path"`
	BootstrapPath           string             `json:"bootstrap_path"`
	MagicLinkRequestPath    string             `json:"magic_link_request_path"`
	SignupPath              string             `json:"signup_path"`
	LogoutPath              string             `json:"logout_path"`
	AccountAPIBasePath      string             `json:"account_api_base_path"`
	PortalAPIBasePath       string             `json:"portal_api_base_path"`
	Accounts                []BootstrapAccount `json:"accounts"`
}

func MarshalBootstrapJSON(data BootstrapData) (template.JS, error) {
	payload, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	return template.JS(payload), nil
}

func BuildBootstrapDataWithSignupPath(authenticated bool, email string, accounts []portalPageAccount, hasSelfHostedCommercial bool, signupPath string) BootstrapData {
	return BuildBootstrapDataWithEnvironment(authenticated, email, accounts, hasSelfHostedCommercial, DefaultPortalEnvironment(signupPath))
}

func BuildBootstrapDataWithEnvironment(authenticated bool, email string, accounts []portalPageAccount, hasSelfHostedCommercial bool, env PortalEnvironment) BootstrapData {
	bootstrapAccounts := make([]BootstrapAccount, 0, len(accounts))
	for _, account := range accounts {
		workspaces := make([]BootstrapWorkspace, 0, len(account.Workspaces))
		for _, workspace := range account.Workspaces {
			lastHealthCheck := ""
			if workspace.LastHealthCheck != nil {
				lastHealthCheck = workspace.LastHealthCheck.UTC().Format(time.RFC3339)
			}
			lastAgentSeenAt := ""
			if workspace.LastAgentSeenAt != nil {
				lastAgentSeenAt = workspace.LastAgentSeenAt.UTC().Format(time.RFC3339)
			}
			activeAlertsUpdatedAt := ""
			if workspace.ActiveAlertsUpdatedAt != nil {
				activeAlertsUpdatedAt = workspace.ActiveAlertsUpdatedAt.UTC().Format(time.RFC3339)
			}
			workspaces = append(workspaces, BootstrapWorkspace{
				ID:                          workspace.ID,
				DisplayName:                 workspace.DisplayName,
				State:                       workspace.State,
				Healthy:                     workspace.Healthy,
				HealthStatus:                workspace.HealthStatus,
				SetupStatus:                 workspace.SetupStatus,
				AgentCount:                  workspace.AgentCount,
				AgentTokenCount:             workspace.AgentTokenCount,
				UnusedAgentTokenCount:       workspace.UnusedAgentTokenCount,
				LastAgentSeenAt:             lastAgentSeenAt,
				AlertRouteCount:             workspace.AlertRouteCount,
				DisabledAlertRouteCount:     workspace.DisabledAlertRouteCount,
				ActiveCriticalAlertCount:    workspace.ActiveCriticalAlertCount,
				ActiveWarningAlertCount:     workspace.ActiveWarningAlertCount,
				ActiveAlertsUpdatedAt:       activeAlertsUpdatedAt,
				ReportScheduleCount:         workspace.ReportScheduleCount,
				DisabledReportScheduleCount: workspace.DisabledReportScheduleCount,
				LastHealthCheck:             lastHealthCheck,
				CreatedAt:                   workspace.CreatedAt.UTC().Format(time.RFC3339),
			})
		}
		members := make([]BootstrapMember, 0, len(account.Members))
		for _, member := range account.Members {
			createdAt := ""
			if !member.CreatedAt.IsZero() {
				createdAt = member.CreatedAt.UTC().Format(time.RFC3339)
			}
			members = append(members, BootstrapMember{
				SubjectID: member.SubjectID,
				UserID:    member.UserID,
				Email:     member.Email,
				Role:      string(member.Role),
				State:     string(member.State),
				CreatedAt: createdAt,
			})
		}
		bootstrapAccounts = append(bootstrapAccounts, BootstrapAccount{
			ID:             account.ID,
			Kind:           account.Kind,
			KindLabel:      account.KindLabel,
			Name:           account.Name,
			Role:           account.Role,
			CanManage:      account.CanManage,
			HasBilling:     account.HasBilling,
			Workspaces:     workspaces,
			Members:        members,
			SetupTemplates: defaultSetupTemplatesForAccount(account),
		})
	}

	return BootstrapData{
		Authenticated:           authenticated,
		Email:                   email,
		HasSelfHostedCommercial: hasSelfHostedCommercial,
		EmailSignInAvailable:    env.EmailSignInAvailable,
		ProviderHostedMode:      env.ProviderHostedMode,
		PublicSiteURL:           defaultPublicSiteURL,
		SupportEmail:            defaultSupportEmail,
		CommercialAPIBaseURL:    defaultCommercialAPIBaseURL,
		CommercialAPIBasePath:   PortalCommercialAPIBasePath,
		PortalPath:              defaultPortalPath,
		BootstrapPath:           PortalBootstrapPath,
		MagicLinkRequestPath:    PortalMagicLinkRequestPath,
		SignupPath:              env.SignupPath,
		LogoutPath:              defaultLogoutPath,
		AccountAPIBasePath:      defaultAccountAPIBasePath,
		PortalAPIBasePath:       defaultPortalAPIBasePath,
		Accounts:                bootstrapAccounts,
	}
}

func defaultSetupTemplatesForAccount(account portalPageAccount) []BootstrapSetupTemplate {
	if account.Kind != string(registry.AccountKindMSP) {
		return nil
	}
	return []BootstrapSetupTemplate{
		{
			ID:           "standard-client-onboarding",
			Title:        "Standard client onboarding",
			AgentNaming:  "Each client workspace opens an isolated Pulse runtime; repeated hostnames are expected across clients.",
			AlertRouting: "Create at least one enabled alert route inside each client runtime.",
			Reporting:    "Schedule at least one client performance report, with client branding when your plan includes white-label, before the workspace is marked ready.",
			Access:       "Invite provider staff from Access and keep client users on the smallest useful role.",
		},
	}
}

func BuildBootstrapData(authenticated bool, email string, accounts []portalPageAccount, hasSelfHostedCommercial bool) BootstrapData {
	return BuildBootstrapDataWithSignupPath(authenticated, email, accounts, hasSelfHostedCommercial, PortalSignupPath)
}

func BuildAnonymousBootstrapDataWithSignupPath(signupPath string) BootstrapData {
	return BuildBootstrapDataWithSignupPath(false, "", nil, false, signupPath)
}

func BuildAnonymousBootstrapDataWithEnvironment(env PortalEnvironment) BootstrapData {
	return BuildBootstrapDataWithEnvironment(false, "", nil, false, env)
}

func BuildAnonymousBootstrapData() BootstrapData {
	return BuildAnonymousBootstrapDataWithSignupPath(PortalSignupPath)
}
