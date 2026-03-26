package portal

import (
	"encoding/json"
	"html/template"
	"time"
)

type BootstrapWorkspace struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	State       string `json:"state"`
	Healthy     bool   `json:"healthy"`
	CreatedAt   string `json:"created_at"`
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
}

type BootstrapData struct {
	Authenticated         bool               `json:"authenticated"`
	Email                 string             `json:"email"`
	PublicSiteURL         string             `json:"public_site_url"`
	SupportEmail          string             `json:"support_email"`
	CommercialAPIBasePath string             `json:"commercial_api_base_path"`
	PortalPath            string             `json:"portal_path"`
	BootstrapPath         string             `json:"bootstrap_path"`
	MagicLinkRequestPath  string             `json:"magic_link_request_path"`
	SignupPath            string             `json:"signup_path"`
	LogoutPath            string             `json:"logout_path"`
	AccountAPIBasePath    string             `json:"account_api_base_path"`
	PortalAPIBasePath     string             `json:"portal_api_base_path"`
	Accounts              []BootstrapAccount `json:"accounts"`
}

func MarshalBootstrapJSON(data BootstrapData) (template.JS, error) {
	payload, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	return template.JS(payload), nil
}

func BuildBootstrapData(authenticated bool, email string, accounts []portalPageAccount) BootstrapData {
	bootstrapAccounts := make([]BootstrapAccount, 0, len(accounts))
	for _, account := range accounts {
		workspaces := make([]BootstrapWorkspace, 0, len(account.Workspaces))
		for _, workspace := range account.Workspaces {
			workspaces = append(workspaces, BootstrapWorkspace{
				ID:          workspace.ID,
				DisplayName: workspace.DisplayName,
				State:       workspace.State,
				Healthy:     workspace.Healthy,
				CreatedAt:   workspace.CreatedAt.UTC().Format(time.RFC3339),
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
		})
	}

	return BootstrapData{
		Authenticated:         authenticated,
		Email:                 email,
		PublicSiteURL:         defaultPublicSiteURL,
		SupportEmail:          defaultSupportEmail,
		CommercialAPIBasePath: PortalCommercialAPIBasePath,
		PortalPath:            defaultPortalPath,
		BootstrapPath:         PortalBootstrapPath,
		MagicLinkRequestPath:  PortalMagicLinkRequestPath,
		SignupPath:            PortalSignupPath,
		LogoutPath:            defaultLogoutPath,
		AccountAPIBasePath:    defaultAccountAPIBasePath,
		PortalAPIBasePath:     defaultPortalAPIBasePath,
		Accounts:              bootstrapAccounts,
	}
}

func BuildAnonymousBootstrapData() BootstrapData {
	return BuildBootstrapData(false, "", nil)
}
