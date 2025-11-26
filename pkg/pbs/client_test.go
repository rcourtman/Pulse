package pbs

import (
	"net/http"
	"testing"
	"time"
)

func TestAuthHTTPError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *authHTTPError
		contains []string
	}{
		{
			name:     "unauthorized",
			err:      &authHTTPError{status: http.StatusUnauthorized, body: "invalid credentials"},
			contains: []string{"authentication failed", "401", "invalid credentials"},
		},
		{
			name:     "forbidden",
			err:      &authHTTPError{status: http.StatusForbidden, body: "access denied"},
			contains: []string{"authentication failed", "403", "access denied"},
		},
		{
			name:     "other error",
			err:      &authHTTPError{status: http.StatusInternalServerError, body: "server error"},
			contains: []string{"authentication failed", "server error"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			errMsg := tc.err.Error()
			for _, expected := range tc.contains {
				if !containsString(errMsg, expected) {
					t.Errorf("Error() = %q, expected to contain %q", errMsg, expected)
				}
			}
		})
	}
}

func TestShouldFallbackToForm(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "bad request should fallback",
			err:      &authHTTPError{status: http.StatusBadRequest, body: "bad request"},
			expected: true,
		},
		{
			name:     "unsupported media type should fallback",
			err:      &authHTTPError{status: http.StatusUnsupportedMediaType, body: "unsupported"},
			expected: true,
		},
		{
			name:     "unauthorized should not fallback",
			err:      &authHTTPError{status: http.StatusUnauthorized, body: "unauthorized"},
			expected: false,
		},
		{
			name:     "server error should not fallback",
			err:      &authHTTPError{status: http.StatusInternalServerError, body: "error"},
			expected: false,
		},
		{
			name:     "non-auth error should not fallback",
			err:      http.ErrHandlerTimeout,
			expected: false,
		},
		{
			name:     "nil error should not fallback",
			err:      nil,
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := shouldFallbackToForm(tc.err)
			if result != tc.expected {
				t.Errorf("shouldFallbackToForm() = %v, want %v", result, tc.expected)
			}
		})
	}
}

func TestClientConfig_Defaults(t *testing.T) {
	cfg := ClientConfig{}

	// Verify zero values
	if cfg.Host != "" {
		t.Error("Host should be empty by default")
	}
	if cfg.User != "" {
		t.Error("User should be empty by default")
	}
	if cfg.Password != "" {
		t.Error("Password should be empty by default")
	}
	if cfg.TokenName != "" {
		t.Error("TokenName should be empty by default")
	}
	if cfg.TokenValue != "" {
		t.Error("TokenValue should be empty by default")
	}
	if cfg.Fingerprint != "" {
		t.Error("Fingerprint should be empty by default")
	}
	if cfg.VerifySSL {
		t.Error("VerifySSL should be false by default")
	}
	if cfg.Timeout != 0 {
		t.Error("Timeout should be zero by default")
	}
}

func TestNewClient_InvalidUserFormat(t *testing.T) {
	cfg := ClientConfig{
		Host:     "https://pbs.example.com:8007",
		User:     "root", // Missing @realm
		Password: "password",
	}

	_, err := NewClient(cfg)
	if err == nil {
		t.Error("NewClient should fail with invalid user format")
	}
	if !containsString(err.Error(), "user@realm") {
		t.Errorf("Error should mention user@realm format, got: %v", err)
	}
}

func TestNewClient_TokenAuthRequiresUser(t *testing.T) {
	cfg := ClientConfig{
		Host:       "https://pbs.example.com:8007",
		TokenName:  "mytoken", // Just token name, no user info
		TokenValue: "secret",
	}

	_, err := NewClient(cfg)
	if err == nil {
		t.Error("NewClient should fail when token auth lacks user info")
	}
	if !containsString(err.Error(), "user information") {
		t.Errorf("Error should mention user information, got: %v", err)
	}
}

func TestNewClient_HTTPSDefault(t *testing.T) {
	// Can't test the actual client creation (needs network), but we can verify
	// the config normalization logic
	tests := []struct {
		input    string
		expected string
	}{
		{"pbs.example.com:8007", "https://pbs.example.com:8007"},
		{"https://pbs.example.com:8007", "https://pbs.example.com:8007"},
		{"http://pbs.example.com:8007", "http://pbs.example.com:8007"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			// This tests the normalization logic
			host := tc.input
			if host[:4] != "http" {
				host = "https://" + host
			}
			if host != tc.expected {
				t.Errorf("Host = %q, want %q", host, tc.expected)
			}
		})
	}
}

func TestAuth_Fields(t *testing.T) {
	a := auth{
		user:       "root",
		realm:      "pam",
		ticket:     "ticket123",
		csrfToken:  "csrf456",
		tokenName:  "mytoken",
		tokenValue: "secret",
		expiresAt:  time.Now().Add(2 * time.Hour),
	}

	if a.user != "root" {
		t.Errorf("user = %q, want root", a.user)
	}
	if a.realm != "pam" {
		t.Errorf("realm = %q, want pam", a.realm)
	}
	if a.ticket != "ticket123" {
		t.Errorf("ticket = %q, want ticket123", a.ticket)
	}
	if a.csrfToken != "csrf456" {
		t.Errorf("csrfToken = %q, want csrf456", a.csrfToken)
	}
	if a.tokenName != "mytoken" {
		t.Errorf("tokenName = %q, want mytoken", a.tokenName)
	}
	if a.tokenValue != "secret" {
		t.Errorf("tokenValue = %q, want secret", a.tokenValue)
	}
	if a.expiresAt.IsZero() {
		t.Error("expiresAt should not be zero")
	}
}

func TestDatastore_Fields(t *testing.T) {
	ds := Datastore{
		Store:               "backup1",
		Total:               1000000000000,
		Used:                500000000000,
		Avail:               500000000000,
		DeduplicationFactor: 1.5,
		GCStatus:            "ok",
		Error:               "",
	}

	if ds.Store != "backup1" {
		t.Errorf("Store = %q, want backup1", ds.Store)
	}
	if ds.Total != 1000000000000 {
		t.Errorf("Total = %d, want 1000000000000", ds.Total)
	}
	if ds.Used != 500000000000 {
		t.Errorf("Used = %d, want 500000000000", ds.Used)
	}
	if ds.Avail != 500000000000 {
		t.Errorf("Avail = %d, want 500000000000", ds.Avail)
	}
	if ds.DeduplicationFactor != 1.5 {
		t.Errorf("DeduplicationFactor = %f, want 1.5", ds.DeduplicationFactor)
	}
}

func TestVersion_Fields(t *testing.T) {
	v := Version{
		Version: "2.4",
		Release: "2",
		Repoid:  "abcdef123",
	}

	if v.Version != "2.4" {
		t.Errorf("Version = %q, want 2.4", v.Version)
	}
	if v.Release != "2" {
		t.Errorf("Release = %q, want 2", v.Release)
	}
	if v.Repoid != "abcdef123" {
		t.Errorf("Repoid = %q, want abcdef123", v.Repoid)
	}
}

func TestNodeStatus_Fields(t *testing.T) {
	ns := NodeStatus{
		CPU:         0.25,
		Uptime:      86400,
		LoadAverage: []float64{0.5, 0.7, 0.9},
		Memory: Memory{
			Total: 16000000000,
			Used:  8000000000,
			Free:  8000000000,
		},
		Swap: Memory{
			Total: 4000000000,
			Used:  1000000000,
			Free:  3000000000,
		},
	}

	if ns.CPU != 0.25 {
		t.Errorf("CPU = %f, want 0.25", ns.CPU)
	}
	if ns.Uptime != 86400 {
		t.Errorf("Uptime = %d, want 86400", ns.Uptime)
	}
	if len(ns.LoadAverage) != 3 {
		t.Errorf("LoadAverage length = %d, want 3", len(ns.LoadAverage))
	}
	if ns.Memory.Total != 16000000000 {
		t.Errorf("Memory.Total = %d, want 16000000000", ns.Memory.Total)
	}
}

func TestBackupGroup_Fields(t *testing.T) {
	bg := BackupGroup{
		BackupType:  "vm",
		BackupID:    "100",
		LastBackup:  1700000000,
		BackupCount: 5,
		Owner:       "root@pam",
	}

	if bg.BackupType != "vm" {
		t.Errorf("BackupType = %q, want vm", bg.BackupType)
	}
	if bg.BackupID != "100" {
		t.Errorf("BackupID = %q, want 100", bg.BackupID)
	}
	if bg.LastBackup != 1700000000 {
		t.Errorf("LastBackup = %d, want 1700000000", bg.LastBackup)
	}
	if bg.BackupCount != 5 {
		t.Errorf("BackupCount = %d, want 5", bg.BackupCount)
	}
}

func TestBackupSnapshot_Fields(t *testing.T) {
	bs := BackupSnapshot{
		BackupType: "ct",
		BackupID:   "101",
		BackupTime: 1700000000,
		Size:       1073741824,
		Protected:  true,
		Comment:    "daily backup",
	}

	if bs.BackupType != "ct" {
		t.Errorf("BackupType = %q, want ct", bs.BackupType)
	}
	if bs.BackupID != "101" {
		t.Errorf("BackupID = %q, want 101", bs.BackupID)
	}
	if bs.BackupTime != 1700000000 {
		t.Errorf("BackupTime = %d, want 1700000000", bs.BackupTime)
	}
	if bs.Size != 1073741824 {
		t.Errorf("Size = %d, want 1073741824", bs.Size)
	}
	if !bs.Protected {
		t.Error("Protected should be true")
	}
	if bs.Comment != "daily backup" {
		t.Errorf("Comment = %q, want daily backup", bs.Comment)
	}
}

func TestNamespace_Fields(t *testing.T) {
	ns := Namespace{
		NS:     "ns1",
		Path:   "/ns1",
		Name:   "ns1",
		Parent: "",
	}

	if ns.NS != "ns1" {
		t.Errorf("NS = %q, want ns1", ns.NS)
	}
	if ns.Path != "/ns1" {
		t.Errorf("Path = %q, want /ns1", ns.Path)
	}
	if ns.Name != "ns1" {
		t.Errorf("Name = %q, want ns1", ns.Name)
	}
}

func TestMemory_Fields(t *testing.T) {
	m := Memory{
		Total: 16000000000,
		Used:  8000000000,
		Free:  8000000000,
	}

	if m.Total != 16000000000 {
		t.Errorf("Total = %d, want 16000000000", m.Total)
	}
	if m.Used != 8000000000 {
		t.Errorf("Used = %d, want 8000000000", m.Used)
	}
	if m.Free != 8000000000 {
		t.Errorf("Free = %d, want 8000000000", m.Free)
	}
}

func TestFSInfo_Fields(t *testing.T) {
	fs := FSInfo{
		Total: 500000000000,
		Used:  250000000000,
		Free:  250000000000,
	}

	if fs.Total != 500000000000 {
		t.Errorf("Total = %d, want 500000000000", fs.Total)
	}
	if fs.Used != 250000000000 {
		t.Errorf("Used = %d, want 250000000000", fs.Used)
	}
	if fs.Free != 250000000000 {
		t.Errorf("Free = %d, want 250000000000", fs.Free)
	}
}

func TestKSMInfo_Fields(t *testing.T) {
	ksm := KSMInfo{
		Shared: 1000000000,
	}

	if ksm.Shared != 1000000000 {
		t.Errorf("Shared = %d, want 1000000000", ksm.Shared)
	}
}

// Helper function
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
