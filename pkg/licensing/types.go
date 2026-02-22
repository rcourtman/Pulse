package licensing

// LicenseFeaturesResponse provides a minimal, non-admin license view for feature gating.
type LicenseFeaturesResponse struct {
	LicenseStatus string          `json:"license_status"`
	Features      map[string]bool `json:"features"`
	UpgradeURL    string          `json:"upgrade_url"`
}

// ActivateLicenseRequest is the request body for activating a license.
type ActivateLicenseRequest struct {
	LicenseKey string `json:"license_key"`
}

// ActivateLicenseResponse is the response for license activation.
type ActivateLicenseResponse struct {
	Success bool           `json:"success"`
	Message string         `json:"message,omitempty"`
	Status  *LicenseStatus `json:"status,omitempty"`
}
