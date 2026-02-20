package api

import (
	"context"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

func TestNormalizePVEUser(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		// Empty and whitespace cases
		{
			name:  "empty string returns empty",
			input: "",
			want:  "",
		},
		{
			name:  "whitespace-only returns empty",
			input: "   \t  ",
			want:  "",
		},
		{
			name:  "single space returns empty",
			input: " ",
			want:  "",
		},
		{
			name:  "tabs and newlines return empty",
			input: "\t\n\r",
			want:  "",
		},

		// Already has realm - no change
		{
			name:  "already has @pam realm",
			input: "root@pam",
			want:  "root@pam",
		},
		{
			name:  "already has @pve realm",
			input: "admin@pve",
			want:  "admin@pve",
		},
		{
			name:  "already has @custom-realm",
			input: "user@custom-realm",
			want:  "user@custom-realm",
		},
		{
			name:  "already has @ldap realm",
			input: "user@ldap",
			want:  "user@ldap",
		},
		{
			name:  "multiple @ symbols (keeps as-is)",
			input: "user@realm@extra",
			want:  "user@realm@extra",
		},
		{
			name:  "@ at the end",
			input: "user@",
			want:  "user@",
		},
		{
			name:  "@ at the beginning",
			input: "@realm",
			want:  "@realm",
		},

		// No realm - adds @pam suffix
		{
			name:  "simple username adds @pam",
			input: "root",
			want:  "root@pam",
		},
		{
			name:  "username with numbers adds @pam",
			input: "admin123",
			want:  "admin123@pam",
		},
		{
			name:  "username with dash adds @pam",
			input: "backup-user",
			want:  "backup-user@pam",
		},
		{
			name:  "username with underscore adds @pam",
			input: "backup_user",
			want:  "backup_user@pam",
		},
		{
			name:  "username with dot adds @pam",
			input: "first.last",
			want:  "first.last@pam",
		},

		// Whitespace trimming
		{
			name:  "leading whitespace trimmed before adding @pam",
			input: "  root",
			want:  "root@pam",
		},
		{
			name:  "trailing whitespace trimmed before adding @pam",
			input: "root  ",
			want:  "root@pam",
		},
		{
			name:  "leading and trailing whitespace trimmed before adding @pam",
			input: "  root  ",
			want:  "root@pam",
		},
		{
			name:  "whitespace trimmed when realm present",
			input: "  root@pam  ",
			want:  "root@pam",
		},
		{
			name:  "tabs trimmed before adding @pam",
			input: "\troot\t",
			want:  "root@pam",
		},
		{
			name:  "mixed whitespace trimmed",
			input: " \t root \t ",
			want:  "root@pam",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizePVEUser(tt.input)
			if got != tt.want {
				t.Errorf("normalizePVEUser(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestShouldSkipClusterAutoDetection(t *testing.T) {
	tests := []struct {
		name   string
		host   string
		vmName string
		want   bool
	}{
		// Empty host cases
		{
			name:   "empty host returns false",
			host:   "",
			vmName: "any-name",
			want:   false,
		},
		{
			name:   "empty host and empty name returns false",
			host:   "",
			vmName: "",
			want:   false,
		},

		// Test subnet 192.168.77.x
		{
			name:   "test subnet 192.168.77.1 returns true",
			host:   "192.168.77.1",
			vmName: "normal-vm",
			want:   true,
		},
		{
			name:   "test subnet 192.168.77.100 returns true",
			host:   "192.168.77.100",
			vmName: "normal-vm",
			want:   true,
		},
		{
			name:   "test subnet 192.168.77.254 returns true",
			host:   "192.168.77.254",
			vmName: "normal-vm",
			want:   true,
		},
		{
			name:   "test subnet with port 192.168.77.1:8006 returns true",
			host:   "192.168.77.1:8006",
			vmName: "normal-vm",
			want:   true,
		},

		// Test subnet 192.168.88.x
		{
			name:   "test subnet 192.168.88.1 returns true",
			host:   "192.168.88.1",
			vmName: "normal-vm",
			want:   true,
		},
		{
			name:   "test subnet 192.168.88.100 returns true",
			host:   "192.168.88.100",
			vmName: "normal-vm",
			want:   true,
		},
		{
			name:   "test subnet 192.168.88.254 returns true",
			host:   "192.168.88.254",
			vmName: "normal-vm",
			want:   true,
		},

		// Normal subnet - returns false
		{
			name:   "normal subnet 192.168.1.1 returns false",
			host:   "192.168.1.1",
			vmName: "normal-vm",
			want:   false,
		},
		{
			name:   "normal subnet 192.168.0.100 returns false",
			host:   "192.168.0.100",
			vmName: "normal-vm",
			want:   false,
		},
		{
			name:   "normal subnet 192.168.100.50 returns false",
			host:   "192.168.100.50",
			vmName: "normal-vm",
			want:   false,
		},
		{
			name:   "different subnet 10.0.0.1 returns false",
			host:   "10.0.0.1",
			vmName: "normal-vm",
			want:   false,
		},
		{
			name:   "hostname returns false",
			host:   "pve.example.com",
			vmName: "normal-vm",
			want:   false,
		},

		// Host with test- prefix
		{
			name:   "host with test- prefix returns true",
			host:   "test-pve-node",
			vmName: "normal-vm",
			want:   true,
		},
		{
			name:   "host with test- in middle returns true",
			host:   "pve-test-node",
			vmName: "normal-vm",
			want:   true,
		},
		{
			name:   "host ending with test- returns true",
			host:   "pve-node-test-",
			vmName: "normal-vm",
			want:   true,
		},

		// Name with test- prefix
		{
			name:   "name with test- prefix returns true",
			host:   "192.168.1.1",
			vmName: "test-vm",
			want:   true,
		},
		{
			name:   "name with test- in middle returns true",
			host:   "192.168.1.1",
			vmName: "my-test-vm",
			want:   true,
		},
		{
			name:   "name ending with test- returns true",
			host:   "192.168.1.1",
			vmName: "vm-test-",
			want:   true,
		},

		// Name with persist- prefix
		{
			name:   "name with persist- prefix returns true",
			host:   "192.168.1.1",
			vmName: "persist-vm",
			want:   true,
		},
		{
			name:   "name with persist- in middle returns true",
			host:   "192.168.1.1",
			vmName: "my-persist-vm",
			want:   true,
		},
		{
			name:   "name ending with persist- returns true",
			host:   "192.168.1.1",
			vmName: "vm-persist-",
			want:   true,
		},

		// Name with concurrent- prefix
		{
			name:   "name with concurrent- prefix returns true",
			host:   "192.168.1.1",
			vmName: "concurrent-vm",
			want:   true,
		},
		{
			name:   "name with concurrent- in middle returns true",
			host:   "192.168.1.1",
			vmName: "my-concurrent-vm",
			want:   true,
		},
		{
			name:   "name ending with concurrent- returns true",
			host:   "192.168.1.1",
			vmName: "vm-concurrent-",
			want:   true,
		},

		// Case insensitivity tests
		{
			name:   "TEST- uppercase in host returns true",
			host:   "TEST-node",
			vmName: "normal-vm",
			want:   true,
		},
		{
			name:   "Test- mixed case in host returns true",
			host:   "Test-node",
			vmName: "normal-vm",
			want:   true,
		},
		{
			name:   "TeSt- mixed case in host returns true",
			host:   "pve-TeSt-node",
			vmName: "normal-vm",
			want:   true,
		},
		{
			name:   "TEST- uppercase in name returns true",
			host:   "192.168.1.1",
			vmName: "TEST-vm",
			want:   true,
		},
		{
			name:   "Test- mixed case in name returns true",
			host:   "192.168.1.1",
			vmName: "Test-vm",
			want:   true,
		},
		{
			name:   "PERSIST- uppercase in name returns true",
			host:   "192.168.1.1",
			vmName: "PERSIST-vm",
			want:   true,
		},
		{
			name:   "Persist- mixed case in name returns true",
			host:   "192.168.1.1",
			vmName: "Persist-vm",
			want:   true,
		},
		{
			name:   "CONCURRENT- uppercase in name returns true",
			host:   "192.168.1.1",
			vmName: "CONCURRENT-vm",
			want:   true,
		},
		{
			name:   "Concurrent- mixed case in name returns true",
			host:   "192.168.1.1",
			vmName: "Concurrent-vm",
			want:   true,
		},

		// Multiple conditions could trigger true
		{
			name:   "both host and name have test- returns true",
			host:   "test-node",
			vmName: "test-vm",
			want:   true,
		},
		{
			name:   "test subnet and test name returns true",
			host:   "192.168.77.1",
			vmName: "test-vm",
			want:   true,
		},
		{
			name:   "test subnet and persist name returns true",
			host:   "192.168.88.1",
			vmName: "persist-vm",
			want:   true,
		},

		// Edge cases
		{
			name:   "just test without dash returns false",
			host:   "testnode",
			vmName: "testvm",
			want:   false,
		},
		{
			name:   "just persist without dash returns false",
			host:   "192.168.1.1",
			vmName: "persistvm",
			want:   false,
		},
		{
			name:   "just concurrent without dash returns false",
			host:   "192.168.1.1",
			vmName: "concurrentvm",
			want:   false,
		},
		{
			name:   "partial IP match 192.168.7.1 (not 77) returns false",
			host:   "192.168.7.1",
			vmName: "normal-vm",
			want:   false,
		},
		{
			name:   "partial IP match 192.168.8.1 (not 88) returns false",
			host:   "192.168.8.1",
			vmName: "normal-vm",
			want:   false,
		},
		{
			name:   "IP containing 77 but not in right position returns false",
			host:   "10.77.168.1",
			vmName: "normal-vm",
			want:   false,
		},
		{
			name:   "IP containing 88 but not in right position returns false",
			host:   "10.88.168.1",
			vmName: "normal-vm",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldSkipClusterAutoDetection(tt.host, tt.vmName)
			if got != tt.want {
				t.Errorf("shouldSkipClusterAutoDetection(host=%q, name=%q) = %v, want %v", tt.host, tt.vmName, got, tt.want)
			}
		})
	}
}

func TestFindInstanceNameByHost(t *testing.T) {
	t.Run("pve instances", func(t *testing.T) {
		cfg := &config.Config{
			PVEInstances: []config.PVEInstance{
				{Name: "pve-node1", Host: "https://192.168.1.10:8006"},
				{Name: "pve-node2", Host: "https://192.168.1.11:8006"},
				{Name: "pve-node3", Host: "https://pve3.example.com:8006"},
			},
		}
		h := &ConfigHandlers{legacyConfig: cfg}

		tests := []struct {
			name     string
			nodeType string
			host     string
			want     string
		}{
			{
				name:     "finds first PVE node",
				nodeType: "pve",
				host:     "https://192.168.1.10:8006",
				want:     "pve-node1",
			},
			{
				name:     "finds second PVE node",
				nodeType: "pve",
				host:     "https://192.168.1.11:8006",
				want:     "pve-node2",
			},
			{
				name:     "finds PVE node by hostname",
				nodeType: "pve",
				host:     "https://pve3.example.com:8006",
				want:     "pve-node3",
			},
			{
				name:     "returns empty for non-existent PVE host",
				nodeType: "pve",
				host:     "https://192.168.1.99:8006",
				want:     "",
			},
			{
				name:     "returns empty for empty host",
				nodeType: "pve",
				host:     "",
				want:     "",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got := h.findInstanceNameByHost(context.Background(), tt.nodeType, tt.host)
				if got != tt.want {
					t.Errorf("findInstanceNameByHost(context.Background(), %q, %q) = %q, want %q", tt.nodeType, tt.host, got, tt.want)
				}
			})
		}
	})

	t.Run("pbs instances", func(t *testing.T) {
		cfg := &config.Config{
			PBSInstances: []config.PBSInstance{
				{Name: "pbs-backup1", Host: "https://192.168.1.20:8007"},
				{Name: "pbs-backup2", Host: "https://backup.example.com:8007"},
			},
		}
		h := &ConfigHandlers{legacyConfig: cfg}

		tests := []struct {
			name     string
			nodeType string
			host     string
			want     string
		}{
			{
				name:     "finds PBS node by IP",
				nodeType: "pbs",
				host:     "https://192.168.1.20:8007",
				want:     "pbs-backup1",
			},
			{
				name:     "finds PBS node by hostname",
				nodeType: "pbs",
				host:     "https://backup.example.com:8007",
				want:     "pbs-backup2",
			},
			{
				name:     "returns empty for non-existent PBS host",
				nodeType: "pbs",
				host:     "https://192.168.1.99:8007",
				want:     "",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got := h.findInstanceNameByHost(context.Background(), tt.nodeType, tt.host)
				if got != tt.want {
					t.Errorf("findInstanceNameByHost(context.Background(), %q, %q) = %q, want %q", tt.nodeType, tt.host, got, tt.want)
				}
			})
		}
	})

	t.Run("unknown node type", func(t *testing.T) {
		cfg := &config.Config{
			PVEInstances: []config.PVEInstance{
				{Name: "pve-node1", Host: "https://192.168.1.10:8006"},
			},
			PBSInstances: []config.PBSInstance{
				{Name: "pbs-backup1", Host: "https://192.168.1.20:8007"},
			},
		}
		h := &ConfigHandlers{legacyConfig: cfg}

		tests := []struct {
			name     string
			nodeType string
			host     string
			want     string
		}{
			{
				name:     "returns empty for unknown type",
				nodeType: "unknown",
				host:     "https://192.168.1.10:8006",
				want:     "",
			},
			{
				name:     "returns empty for pmg type (not implemented)",
				nodeType: "pmg",
				host:     "https://192.168.1.30:8006",
				want:     "",
			},
			{
				name:     "returns empty for empty type",
				nodeType: "",
				host:     "https://192.168.1.10:8006",
				want:     "",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got := h.findInstanceNameByHost(context.Background(), tt.nodeType, tt.host)
				if got != tt.want {
					t.Errorf("findInstanceNameByHost(context.Background(), %q, %q) = %q, want %q", tt.nodeType, tt.host, got, tt.want)
				}
			})
		}
	})

	t.Run("empty config", func(t *testing.T) {
		cfg := &config.Config{}
		h := &ConfigHandlers{legacyConfig: cfg}

		got := h.findInstanceNameByHost(context.Background(), "pve", "https://192.168.1.10:8006")
		if got != "" {
			t.Errorf("expected empty string for empty config, got %q", got)
		}
	})
}

func TestValidateSetupToken(t *testing.T) {
	t.Run("empty token returns false", func(t *testing.T) {
		h := &ConfigHandlers{
			setupCodes:        make(map[string]*SetupCode),
			recentSetupTokens: make(map[string]time.Time),
		}

		if h.ValidateSetupToken("") {
			t.Error("expected false for empty token")
		}
	})

	t.Run("valid setup code token", func(t *testing.T) {
		token := "test-setup-token-12345"
		tokenHash := auth.HashAPIToken(token)

		h := &ConfigHandlers{
			setupCodes: map[string]*SetupCode{
				tokenHash: {
					ExpiresAt: time.Now().Add(1 * time.Hour),
					Used:      false,
				},
			},
			recentSetupTokens: make(map[string]time.Time),
		}

		if !h.ValidateSetupToken(token) {
			t.Error("expected true for valid setup code token")
		}
	})

	t.Run("expired setup code returns false", func(t *testing.T) {
		token := "expired-token-12345"
		tokenHash := auth.HashAPIToken(token)

		h := &ConfigHandlers{
			setupCodes: map[string]*SetupCode{
				tokenHash: {
					ExpiresAt: time.Now().Add(-1 * time.Hour), // Expired
					Used:      false,
				},
			},
			recentSetupTokens: make(map[string]time.Time),
		}

		if h.ValidateSetupToken(token) {
			t.Error("expected false for expired setup code")
		}
	})

	t.Run("used setup code returns false", func(t *testing.T) {
		token := "used-token-12345"
		tokenHash := auth.HashAPIToken(token)

		h := &ConfigHandlers{
			setupCodes: map[string]*SetupCode{
				tokenHash: {
					ExpiresAt: time.Now().Add(1 * time.Hour),
					Used:      true, // Already used
				},
			},
			recentSetupTokens: make(map[string]time.Time),
		}

		if h.ValidateSetupToken(token) {
			t.Error("expected false for used setup code")
		}
	})

	t.Run("valid recent setup token", func(t *testing.T) {
		token := "recent-setup-token-12345"
		tokenHash := auth.HashAPIToken(token)

		h := &ConfigHandlers{
			setupCodes: make(map[string]*SetupCode),
			recentSetupTokens: map[string]time.Time{
				tokenHash: time.Now().Add(1 * time.Hour), // Valid for another hour
			},
		}

		if !h.ValidateSetupToken(token) {
			t.Error("expected true for valid recent setup token")
		}
	})

	t.Run("expired recent setup token returns false", func(t *testing.T) {
		token := "expired-recent-token-12345"
		tokenHash := auth.HashAPIToken(token)

		h := &ConfigHandlers{
			setupCodes: make(map[string]*SetupCode),
			recentSetupTokens: map[string]time.Time{
				tokenHash: time.Now().Add(-1 * time.Hour), // Expired
			},
		}

		if h.ValidateSetupToken(token) {
			t.Error("expected false for expired recent setup token")
		}
	})

	t.Run("non-existent token returns false", func(t *testing.T) {
		h := &ConfigHandlers{
			setupCodes:        make(map[string]*SetupCode),
			recentSetupTokens: make(map[string]time.Time),
		}

		if h.ValidateSetupToken("non-existent-token") {
			t.Error("expected false for non-existent token")
		}
	})

	t.Run("setup code takes precedence over recent token", func(t *testing.T) {
		token := "dual-token-12345"
		tokenHash := auth.HashAPIToken(token)

		h := &ConfigHandlers{
			setupCodes: map[string]*SetupCode{
				tokenHash: {
					ExpiresAt: time.Now().Add(1 * time.Hour),
					Used:      false,
				},
			},
			recentSetupTokens: map[string]time.Time{
				tokenHash: time.Now().Add(1 * time.Hour),
			},
		}

		// Should return true (setup code is valid)
		if !h.ValidateSetupToken(token) {
			t.Error("expected true when both setup code and recent token exist")
		}
	})
}

func TestValidateSetupTokenForOrg(t *testing.T) {
	token := "org-bound-token-12345"
	tokenHash := auth.HashAPIToken(token)

	h := &ConfigHandlers{
		setupCodes: map[string]*SetupCode{
			tokenHash: {
				ExpiresAt: time.Now().Add(1 * time.Hour),
				Used:      false,
				OrgID:     "org-a",
			},
		},
		recentSetupTokens: make(map[string]time.Time),
	}

	if !h.ValidateSetupTokenForOrg(token, "org-a") {
		t.Fatal("expected org-bound token to validate for matching org")
	}
	if h.ValidateSetupTokenForOrg(token, "org-b") {
		t.Fatal("expected org-bound token to fail for different org")
	}
}
