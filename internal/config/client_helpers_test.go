package config

import "testing"

func TestNormalizeHostPort(t *testing.T) {
	tests := []struct {
		name        string
		host        string
		defaultPort string
		want        string
	}{
		{
			name:        "adds default port to https host",
			host:        "https://example.com",
			defaultPort: defaultPVEPort,
			want:        "https://example.com:8006",
		},
		{
			name:        "adds default port when scheme missing",
			host:        "pbs.lan",
			defaultPort: defaultPBSPort,
			want:        "https://pbs.lan:8007",
		},
		{
			name:        "preserves existing port and scheme",
			host:        "http://example.com:8443",
			defaultPort: defaultPVEPort,
			want:        "http://example.com:8443",
		},
		{
			name:        "drops path segments before applying port",
			host:        "https://example.com/api",
			defaultPort: defaultPVEPort,
			want:        "https://example.com:8006",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeHostPort(tt.host, tt.defaultPort); got != tt.want {
				t.Fatalf("normalizeHostPort(%q, %q) = %q, want %q", tt.host, tt.defaultPort, got, tt.want)
			}
		})
	}
}

func TestCreateConfigHelpersNormalizeHosts(t *testing.T) {
	pveCfg := PVEInstance{
		Host: "https://pve.local",
		User: "root@pam",
	}
	pveClientCfg := CreateProxmoxConfig(&pveCfg)
	if pveClientCfg.Host != "https://pve.local:8006" {
		t.Fatalf("expected default port on PVE host, got %q", pveClientCfg.Host)
	}

	pbsCfg := PBSInstance{
		Host: "pbs.local",
		User: "root@pam",
	}
	pbsClientCfg := CreatePBSConfig(&pbsCfg)
	if pbsClientCfg.Host != "https://pbs.local:8007" {
		t.Fatalf("expected default PBS port on host, got %q", pbsClientCfg.Host)
	}

	pmgCfg := PMGInstance{
		Host: "https://pmg.local/api",
		User: "root@pam",
	}
	pmgClientCfg := CreatePMGConfig(&pmgCfg)
	if pmgClientCfg.Host != "https://pmg.local:8006" {
		t.Fatalf("expected default PMG port on host, got %q", pmgClientCfg.Host)
	}

	fromFields := CreateProxmoxConfigFromFields("https://cluster.local", "root@pam", "", "token!name", "value", "", true)
	if fromFields.Host != "https://cluster.local:8006" {
		t.Fatalf("expected default port on host built from fields, got %q", fromFields.Host)
	}
}

func TestCreateProxmoxConfigFromFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		host         string
		user         string
		password     string
		tokenName    string
		tokenValue   string
		fingerprint  string
		verifySSL    bool
		expectedUser string
		expectedHost string
	}{
		{
			name:         "user without realm gets @pam appended (password auth)",
			host:         "https://pve.local",
			user:         "root",
			password:     "secret",
			tokenName:    "",
			tokenValue:   "",
			expectedUser: "root@pam",
			expectedHost: "https://pve.local:8006",
		},
		{
			name:         "user with realm unchanged",
			host:         "https://pve.local",
			user:         "admin@pve",
			password:     "secret",
			tokenName:    "",
			tokenValue:   "",
			expectedUser: "admin@pve",
			expectedHost: "https://pve.local:8006",
		},
		{
			name:         "token auth does not modify user",
			host:         "https://pve.local",
			user:         "root",
			password:     "",
			tokenName:    "mytoken",
			tokenValue:   "token-value",
			expectedUser: "root",
			expectedHost: "https://pve.local:8006",
		},
		{
			name:         "empty user unchanged",
			host:         "https://pve.local",
			user:         "",
			password:     "secret",
			tokenName:    "",
			tokenValue:   "",
			expectedUser: "",
			expectedHost: "https://pve.local:8006",
		},
		{
			name:         "fingerprint and verifySSL preserved",
			host:         "pve.local",
			user:         "root",
			password:     "secret",
			tokenName:    "",
			tokenValue:   "",
			fingerprint:  "AA:BB:CC",
			verifySSL:    false,
			expectedUser: "root@pam",
			expectedHost: "https://pve.local:8006",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := CreateProxmoxConfigFromFields(tt.host, tt.user, tt.password, tt.tokenName, tt.tokenValue, tt.fingerprint, tt.verifySSL)

			if cfg.User != tt.expectedUser {
				t.Errorf("User = %q, want %q", cfg.User, tt.expectedUser)
			}
			if cfg.Host != tt.expectedHost {
				t.Errorf("Host = %q, want %q", cfg.Host, tt.expectedHost)
			}
			if cfg.Password != tt.password {
				t.Errorf("Password = %q, want %q", cfg.Password, tt.password)
			}
			if cfg.TokenName != tt.tokenName {
				t.Errorf("TokenName = %q, want %q", cfg.TokenName, tt.tokenName)
			}
			if cfg.TokenValue != tt.tokenValue {
				t.Errorf("TokenValue = %q, want %q", cfg.TokenValue, tt.tokenValue)
			}
			if cfg.Fingerprint != tt.fingerprint {
				t.Errorf("Fingerprint = %q, want %q", cfg.Fingerprint, tt.fingerprint)
			}
			if cfg.VerifySSL != tt.verifySSL {
				t.Errorf("VerifySSL = %v, want %v", cfg.VerifySSL, tt.verifySSL)
			}
		})
	}
}

func TestStripDefaultPort(t *testing.T) {
	tests := []struct {
		name        string
		host        string
		defaultPort string
		want        string
	}{
		{
			name:        "removes default PVE port",
			host:        "https://pve.local:8006",
			defaultPort: defaultPVEPort,
			want:        "https://pve.local",
		},
		{
			name:        "keeps non-default port",
			host:        "https://pve.local:8443",
			defaultPort: defaultPVEPort,
			want:        "https://pve.local:8443",
		},
		{
			name:        "preserves http scheme when removing port",
			host:        "http://pve.local:8006",
			defaultPort: defaultPVEPort,
			want:        "http://pve.local",
		},
		{
			name:        "returns trimmed host unchanged when parse fails",
			host:        "://bad",
			defaultPort: defaultPVEPort,
			want:        "://bad",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StripDefaultPort(tt.host, tt.defaultPort); got != tt.want {
				t.Fatalf("StripDefaultPort(%q, %q) = %q, want %q", tt.host, tt.defaultPort, got, tt.want)
			}
		})
	}
}
