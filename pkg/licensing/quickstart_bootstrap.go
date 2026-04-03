package licensing

// QuickstartBootstrapRequest is sent to the license server to exchange a
// runtime identity for a server-issued quickstart token.
//
// Activated installs authenticate the request with their installation token.
// Entitlement-backed runtimes authenticate the request with their signed
// entitlement lease JWT. The request body supplies only binding metadata.
type QuickstartBootstrapRequest struct {
	InstanceFingerprint string `json:"instance_fingerprint,omitempty"`
	InstanceName        string `json:"instance_name,omitempty"`
	UseCase             string `json:"use_case,omitempty"`
}

// QuickstartBootstrapResponse contains the server-issued quickstart token and
// the current authoritative quickstart inventory snapshot.
type QuickstartBootstrapResponse struct {
	QuickstartToken          string `json:"quickstart_token"`
	QuickstartTokenExpiresAt string `json:"quickstart_token_expires_at,omitempty"`
	TokenExpiresAt           string `json:"token_expires_at,omitempty"`
	CreditsRemaining         int    `json:"credits_remaining"`
	CreditsTotal             int    `json:"credits_total"`
}
