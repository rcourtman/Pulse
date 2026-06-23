package aicontracts

import "testing"

func TestClassifyAutomationRisk(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    RiskLevel
	}{
		{
			name:    "read only command is low risk",
			command: "systemctl status nginx",
			want:    RiskLow,
		},
		{
			name:    "stderr redirection on read only command stays low risk",
			command: "find /var/log -name '*.log' 2>/dev/null",
			want:    RiskLow,
		},
		{
			name:    "file mutation is medium risk",
			command: "touch /tmp/pulse-marker",
			want:    RiskMedium,
		},
		{
			name:    "service restart is high risk",
			command: "systemctl restart nginx",
			want:    RiskHigh,
		},
		{
			name:    "stdout redirection is high risk",
			command: "cat /etc/hosts > /tmp/hosts",
			want:    RiskHigh,
		},
		{
			name:    "blocked command is critical risk",
			command: "rm -rf /tmp/pulse-data",
			want:    RiskCritical,
		},
		{
			name:    "unknown write capability defaults high risk",
			command: "custom-repair-tool --apply",
			want:    RiskHigh,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ClassifyAutomationRisk(tt.command); got != tt.want {
				t.Fatalf("ClassifyAutomationRisk(%q) = %q, want %q", tt.command, got, tt.want)
			}
		})
	}
}
