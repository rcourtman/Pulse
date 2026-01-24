package safety

import "testing"

func TestIsReadOnlyCommand(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		expected bool
	}{
		// Basic read-only commands
		{"cat file", "cat /etc/hosts", true},
		{"ls directory", "ls /var/log", true},
		{"ls alone", "ls", true},
		{"df", "df -h", true},
		{"free", "free -m", true},
		{"ps aux", "ps aux", true},
		{"uptime", "uptime", true},
		{"journalctl", "journalctl -u nginx", true},
		{"systemctl status", "systemctl status nginx", true},
		{"docker ps", "docker ps -a", true},
		{"docker logs", "docker logs nginx", true},
		{"kubectl get", "kubectl get pods", true},
		{"kubectl describe", "kubectl describe pod nginx", true},

		// Proxmox read-only
		{"pct list", "pct list", true},
		{"pct status", "pct status 100", true},
		{"qm list", "qm list", true},
		{"pvesh get", "pvesh get /cluster/status", true},
		{"zpool status", "zpool status", true},

		// Piped commands - safe
		{"ps with grep", "ps aux | grep nginx", true},
		{"journalctl with head", "journalctl -u nginx | head -50", true},
		{"cat with wc", "cat /etc/passwd | wc -l", true},
		{"docker logs with grep", "docker logs nginx | grep error", true},

		// Not read-only - dangerous commands
		{"rm file", "rm /tmp/test", false},
		{"rm -rf", "rm -rf /var/log/*", false},
		{"kill process", "kill 1234", false},
		{"systemctl restart", "systemctl restart nginx", false},
		{"systemctl stop", "systemctl stop nginx", false},
		{"docker stop", "docker stop nginx", false},
		{"docker rm", "docker rm nginx", false},
		{"kubectl delete", "kubectl delete pod nginx", false},
		{"echo to file", "echo test > /tmp/file", false},
		{"chmod", "chmod 755 /tmp/file", false},
		{"chown", "chown root /tmp/file", false},

		// Piped commands with dangerous parts
		{"pipe to rm", "ls | xargs rm", false},
		{"dangerous sed -i", "cat /etc/passwd | sed -i 's/old/new/'", false},

		// Edge cases
		{"empty command", "", false},
		{"spaces only", "   ", false},
		{"case insensitive", "CAT /etc/hosts", true},
		{"proc filesystem", "cat /proc/meminfo", true},
		{"sys filesystem", "cat /sys/class/net/eth0/address", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsReadOnlyCommand(tt.command)
			if result != tt.expected {
				t.Errorf("IsReadOnlyCommand(%q) = %v, expected %v", tt.command, result, tt.expected)
			}
		})
	}
}
