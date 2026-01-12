package dryrun

import (
	"strings"
	"testing"
)

func TestSimulator_Systemctl(t *testing.T) {
	tests := []struct {
		name         string
		command      string
		wantWouldDo  string
		wantReverse  bool
		wantRollback string
	}{
		{
			name:         "restart service",
			command:      "systemctl restart nginx",
			wantWouldDo:  "Restart service nginx",
			wantReverse:  true,
			wantRollback: "systemctl restart nginx",
		},
		{
			name:         "stop service",
			command:      "systemctl stop apache2",
			wantWouldDo:  "Stop service apache2",
			wantReverse:  true,
			wantRollback: "systemctl start apache2",
		},
		{
			name:         "start service",
			command:      "systemctl start mysql",
			wantWouldDo:  "Start service mysql",
			wantReverse:  true,
			wantRollback: "systemctl stop mysql",
		},
	}

	sim := NewSimulator()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sim.Simulate(tt.command)

			if !result.Simulated {
				t.Error("Simulate() Simulated = false, want true")
			}

			if !strings.Contains(result.WouldDo, tt.wantWouldDo) {
				t.Errorf("Simulate() WouldDo = %q, want to contain %q", result.WouldDo, tt.wantWouldDo)
			}

			if result.Reversible != tt.wantReverse {
				t.Errorf("Simulate() Reversible = %v, want %v", result.Reversible, tt.wantReverse)
			}

			if tt.wantRollback != "" && result.RollbackCmd != tt.wantRollback {
				t.Errorf("Simulate() RollbackCmd = %q, want %q", result.RollbackCmd, tt.wantRollback)
			}

			if result.ExitCode != 0 {
				t.Errorf("Simulate() ExitCode = %v, want 0", result.ExitCode)
			}

			if !strings.Contains(result.Output, "[SIMULATED]") {
				t.Errorf("Simulate() Output should contain [SIMULATED]")
			}
		})
	}
}

func TestSimulator_Apt(t *testing.T) {
	tests := []struct {
		name        string
		command     string
		wantWouldDo string
		wantReverse bool
	}{
		{
			name:        "apt update",
			command:     "apt update",
			wantWouldDo: "Update package lists",
			wantReverse: false,
		},
		{
			name:        "apt upgrade",
			command:     "apt upgrade",
			wantWouldDo: "Upgrade",
			wantReverse: false,
		},
		{
			name:        "apt install",
			command:     "apt install htop vim",
			wantWouldDo: "Install package",
			wantReverse: true,
		},
		{
			name:        "apt remove",
			command:     "apt remove nginx",
			wantWouldDo: "Remove package",
			wantReverse: true,
		},
	}

	sim := NewSimulator()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sim.Simulate(tt.command)

			if !result.Simulated {
				t.Error("Simulate() Simulated = false, want true")
			}

			if !strings.Contains(result.WouldDo, tt.wantWouldDo) {
				t.Errorf("Simulate() WouldDo = %q, want to contain %q", result.WouldDo, tt.wantWouldDo)
			}

			if result.Reversible != tt.wantReverse {
				t.Errorf("Simulate() Reversible = %v, want %v", result.Reversible, tt.wantReverse)
			}
		})
	}
}

func TestSimulator_Docker(t *testing.T) {
	tests := []struct {
		name        string
		command     string
		wantWouldDo string
		wantReverse bool
	}{
		{
			name:        "docker restart",
			command:     "docker restart myapp",
			wantWouldDo: "Restart container myapp",
			wantReverse: true,
		},
		{
			name:        "docker stop",
			command:     "docker stop webserver",
			wantWouldDo: "Stop container webserver",
			wantReverse: true,
		},
		{
			name:        "docker exec",
			command:     "docker exec -it nginx bash",
			wantWouldDo: "Execute command in container",
			wantReverse: false,
		},
	}

	sim := NewSimulator()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sim.Simulate(tt.command)

			if !result.Simulated {
				t.Error("Simulate() Simulated = false, want true")
			}

			if !strings.Contains(result.WouldDo, tt.wantWouldDo) {
				t.Errorf("Simulate() WouldDo = %q, want to contain %q", result.WouldDo, tt.wantWouldDo)
			}

			if result.Reversible != tt.wantReverse {
				t.Errorf("Simulate() Reversible = %v, want %v", result.Reversible, tt.wantReverse)
			}
		})
	}
}

func TestSimulator_Proxmox(t *testing.T) {
	tests := []struct {
		name        string
		command     string
		wantWouldDo string
		wantReverse bool
	}{
		{
			name:        "pct start container",
			command:     "pct start 100",
			wantWouldDo: "Start LXC container 100",
			wantReverse: true,
		},
		{
			name:        "pct stop container",
			command:     "pct stop 101",
			wantWouldDo: "Stop LXC container 101",
			wantReverse: true,
		},
		{
			name:        "qm start VM",
			command:     "qm start 200",
			wantWouldDo: "Start VM 200",
			wantReverse: true,
		},
		{
			name:        "qm shutdown VM",
			command:     "qm shutdown 201",
			wantWouldDo: "Shutdown VM 201",
			wantReverse: true,
		},
		{
			name:        "pct resize",
			command:     "pct resize 100 rootfs +10G",
			wantWouldDo: "Resize container 100",
			wantReverse: true, // Growth is "reversible" in the sense we can shrink
		},
	}

	sim := NewSimulator()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sim.Simulate(tt.command)

			if !result.Simulated {
				t.Error("Simulate() Simulated = false, want true")
			}

			if !strings.Contains(result.WouldDo, tt.wantWouldDo) {
				t.Errorf("Simulate() WouldDo = %q, want to contain %q", result.WouldDo, tt.wantWouldDo)
			}

			if result.Reversible != tt.wantReverse {
				t.Errorf("Simulate() Reversible = %v, want %v", result.Reversible, tt.wantReverse)
			}
		})
	}
}

func TestSimulator_ProcessManagement(t *testing.T) {
	tests := []struct {
		name        string
		command     string
		wantWouldDo string
		wantReverse bool
	}{
		{
			name:        "kill process",
			command:     "kill 12345",
			wantWouldDo: "Send signal",
			wantReverse: false,
		},
		{
			name:        "kill with signal",
			command:     "kill -9 12345",
			wantWouldDo: "Send signal -9",
			wantReverse: false,
		},
		{
			name:        "pkill process",
			command:     "pkill nginx",
			wantWouldDo: "Kill processes matching 'nginx'",
			wantReverse: false,
		},
	}

	sim := NewSimulator()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sim.Simulate(tt.command)

			if !result.Simulated {
				t.Error("Simulate() Simulated = false, want true")
			}

			if !strings.Contains(result.WouldDo, tt.wantWouldDo) {
				t.Errorf("Simulate() WouldDo = %q, want to contain %q", result.WouldDo, tt.wantWouldDo)
			}

			if result.Reversible != tt.wantReverse {
				t.Errorf("Simulate() Reversible = %v, want %v", result.Reversible, tt.wantReverse)
			}
		})
	}
}

func TestSimulator_FileOperations(t *testing.T) {
	tests := []struct {
		name        string
		command     string
		wantWouldDo string
		wantReverse bool
	}{
		{
			name:        "rm file",
			command:     "rm /tmp/test.log",
			wantWouldDo: "Remove",
			wantReverse: false,
		},
		{
			name:        "rm recursive",
			command:     "rm -rf /tmp/data",
			wantWouldDo: "Remove",
			wantReverse: false,
		},
		{
			name:        "chmod",
			command:     "chmod 755 /usr/local/bin/script.sh",
			wantWouldDo: "Change permissions",
			wantReverse: false,
		},
		{
			name:        "chown",
			command:     "chown www-data:www-data /var/www",
			wantWouldDo: "Change owner",
			wantReverse: false,
		},
	}

	sim := NewSimulator()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sim.Simulate(tt.command)

			if !result.Simulated {
				t.Error("Simulate() Simulated = false, want true")
			}

			if !strings.Contains(result.WouldDo, tt.wantWouldDo) {
				t.Errorf("Simulate() WouldDo = %q, want to contain %q", result.WouldDo, tt.wantWouldDo)
			}

			if result.Reversible != tt.wantReverse {
				t.Errorf("Simulate() Reversible = %v, want %v", result.Reversible, tt.wantReverse)
			}
		})
	}
}

func TestSimulator_Diagnostics(t *testing.T) {
	tests := []struct {
		name        string
		command     string
		wantWouldDo string
		wantReverse bool
	}{
		{
			name:        "df disk free",
			command:     "df -h",
			wantWouldDo: "Display disk space",
			wantReverse: true, // Read-only
		},
		{
			name:        "free memory",
			command:     "free -m",
			wantWouldDo: "Display memory usage",
			wantReverse: true, // Read-only
		},
		{
			name:        "journalctl",
			command:     "journalctl -u nginx -n 100",
			wantWouldDo: "Display system logs",
			wantReverse: true, // Read-only
		},
	}

	sim := NewSimulator()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sim.Simulate(tt.command)

			if !result.Simulated {
				t.Error("Simulate() Simulated = false, want true")
			}

			if !strings.Contains(result.WouldDo, tt.wantWouldDo) {
				t.Errorf("Simulate() WouldDo = %q, want to contain %q", result.WouldDo, tt.wantWouldDo)
			}

			if result.Reversible != tt.wantReverse {
				t.Errorf("Simulate() Reversible = %v, want %v", result.Reversible, tt.wantReverse)
			}
		})
	}
}

func TestSimulator_Nginx(t *testing.T) {
	tests := []struct {
		name        string
		command     string
		wantWouldDo string
		wantReverse bool
	}{
		{
			name:        "nginx reload",
			command:     "nginx -s reload",
			wantWouldDo: "Reload nginx",
			wantReverse: true,
		},
		{
			name:        "nginx restart",
			command:     "nginx -s restart",
			wantWouldDo: "Restart nginx",
			wantReverse: true,
		},
	}

	sim := NewSimulator()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sim.Simulate(tt.command)

			if !result.Simulated {
				t.Error("Simulate() Simulated = false, want true")
			}

			if !strings.Contains(result.WouldDo, tt.wantWouldDo) {
				t.Errorf("Simulate() WouldDo = %q, want to contain %q", result.WouldDo, tt.wantWouldDo)
			}

			if result.Reversible != tt.wantReverse {
				t.Errorf("Simulate() Reversible = %v, want %v", result.Reversible, tt.wantReverse)
			}
		})
	}
}

func TestSimulator_UnknownCommand(t *testing.T) {
	sim := NewSimulator()

	result := sim.Simulate("some-unknown-command --arg1 --arg2")

	if !result.Simulated {
		t.Error("Simulate() Simulated = false, want true")
	}

	if result.WouldDo != "Execute unknown command" {
		t.Errorf("Simulate() WouldDo = %q, want %q", result.WouldDo, "Execute unknown command")
	}

	if result.Reversible {
		t.Error("Simulate() unknown command should not be reversible")
	}

	if !strings.Contains(result.Output, "[SIMULATED]") {
		t.Error("Simulate() Output should contain [SIMULATED]")
	}

	if !strings.Contains(result.Output, "some-unknown-command") {
		t.Error("Simulate() Output should contain the original command")
	}
}

func TestGlobalSimulate(t *testing.T) {
	result := Simulate("systemctl restart nginx")

	if !result.Simulated {
		t.Error("Simulate() Simulated = false, want true")
	}

	if !strings.Contains(result.WouldDo, "Restart") {
		t.Errorf("Simulate() WouldDo = %q, want to contain 'Restart'", result.WouldDo)
	}
}

func TestReverseAction(t *testing.T) {
	tests := []struct {
		action string
		want   string
	}{
		{"start", "stop"},
		{"stop", "start"},
		{"restart", "restart"},
		{"reboot", "start"},
		{"reload", "reload"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.action, func(t *testing.T) {
			got := reverseAction(tt.action)
			if got != tt.want {
				t.Errorf("reverseAction(%q) = %q, want %q", tt.action, got, tt.want)
			}
		})
	}
}
