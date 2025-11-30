package api

import "testing"

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
		name string
		host string
		vmName string
		want bool
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
