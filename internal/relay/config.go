package relay

// Config holds relay client configuration.
type Config struct {
	Enabled        bool   `json:"enabled"`
	ServerURL      string `json:"server_url"`
	InstanceSecret string `json:"instance_secret"`

	// Instance identity keypair for MITM prevention.
	IdentityPrivateKey  string `json:"identity_private_key,omitempty"`
	IdentityPublicKey   string `json:"identity_public_key,omitempty"`
	IdentityFingerprint string `json:"identity_fingerprint,omitempty"`
}

// DefaultServerURL is the production relay server endpoint.
const DefaultServerURL = "wss://relay.pulserelay.pro/ws/instance"

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Enabled:   false,
		ServerURL: DefaultServerURL,
	}
}
