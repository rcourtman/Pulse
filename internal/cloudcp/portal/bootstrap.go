package portal

import (
	"encoding/json"
	"html/template"
	"time"
)

type BootstrapWorkspace struct {
	ID              string `json:"id"`
	DisplayName     string `json:"display_name"`
	State           string `json:"state"`
	Healthy         bool   `json:"healthy"`
	HealthStatus    string `json:"health_status"`
	LastHealthCheck string `json:"last_health_check,omitempty"`
	CreatedAt       string `json:"created_at"`
}

type BootstrapMember struct {
	UserID    string `json:"user_id"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	CreatedAt string `json:"created_at,omitempty"`
}

type BootstrapAccount struct {
	ID         string               `json:"id"`
	Kind       string               `json:"kind"`
	KindLabel  string               `json:"kind_label"`
	Name       string               `json:"name"`
	Role       string               `json:"role"`
	CanManage  bool                 `json:"can_manage"`
	HasBilling bool                 `json:"has_billing"`
	Workspaces []BootstrapWorkspace `json:"workspaces"`
	Members    []BootstrapMember    `json:"members"`
}

type BootstrapData struct {
	Authenticated           bool               `json:"authenticated"`
	Email                   string             `json:"email"`
	HasSelfHostedCommercial bool               `json:"has_self_hosted_commercial"`
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
	bootstrapAccounts := make([]BootstrapAccount, 0, len(accounts))
	for _, account := range accounts {
		workspaces := make([]BootstrapWorkspace, 0, len(account.Workspaces))
		for _, workspace := range account.Workspaces {
			lastHealthCheck := ""
			if workspace.LastHealthCheck != nil {
				lastHealthCheck = workspace.LastHealthCheck.UTC().Format(time.RFC3339)
			}
			workspaces = append(workspaces, BootstrapWorkspace{
				ID:              workspace.ID,
				DisplayName:     workspace.DisplayName,
				State:           workspace.State,
				Healthy:         workspace.Healthy,
				HealthStatus:    workspace.HealthStatus,
				LastHealthCheck: lastHealthCheck,
				CreatedAt:       workspace.CreatedAt.UTC().Format(time.RFC3339),
			})
		}
		members := make([]BootstrapMember, 0, len(account.Members))
		for _, member := range account.Members {
			createdAt := ""
			if !member.CreatedAt.IsZero() {
				createdAt = member.CreatedAt.UTC().Format(time.RFC3339)
			}
			members = append(members, BootstrapMember{
				UserID:    member.UserID,
				Email:     member.Email,
				Role:      string(member.Role),
				CreatedAt: createdAt,
			})
		}
		bootstrapAccounts = append(bootstrapAccounts, BootstrapAccount{
			ID:         account.ID,
			Kind:       account.Kind,
			KindLabel:  account.KindLabel,
			Name:       account.Name,
			Role:       account.Role,
			CanManage:  account.CanManage,
			HasBilling: account.HasBilling,
			Workspaces: workspaces,
			Members:    members,
		})
	}

	return BootstrapData{
		Authenticated:           authenticated,
		Email:                   email,
		HasSelfHostedCommercial: hasSelfHostedCommercial,
		PublicSiteURL:           defaultPublicSiteURL,
		SupportEmail:            defaultSupportEmail,
		CommercialAPIBaseURL:    defaultCommercialAPIBaseURL,
		CommercialAPIBasePath:   PortalCommercialAPIBasePath,
		PortalPath:              defaultPortalPath,
		BootstrapPath:           PortalBootstrapPath,
		MagicLinkRequestPath:    PortalMagicLinkRequestPath,
		SignupPath:              signupPath,
		LogoutPath:              defaultLogoutPath,
		AccountAPIBasePath:      defaultAccountAPIBasePath,
		PortalAPIBasePath:       defaultPortalAPIBasePath,
		Accounts:                bootstrapAccounts,
	}
}

func BuildBootstrapData(authenticated bool, email string, accounts []portalPageAccount, hasSelfHostedCommercial bool) BootstrapData {
	return BuildBootstrapDataWithSignupPath(authenticated, email, accounts, hasSelfHostedCommercial, PortalSignupPath)
}

func BuildAnonymousBootstrapDataWithSignupPath(signupPath string) BootstrapData {
	return BuildBootstrapDataWithSignupPath(false, "", nil, false, signupPath)
}

func BuildAnonymousBootstrapData() BootstrapData {
	return BuildAnonymousBootstrapDataWithSignupPath(PortalSignupPath)
}
