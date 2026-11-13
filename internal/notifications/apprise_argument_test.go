package notifications

import (
	"context"
	"slices"
	"strings"
	"testing"
)

// A target reaches the Apprise CLI as a bare argument, so one starting with a
// dash would be read as an option, such as --attach with a server file or
// --plugin-path with code to load.
func TestAppriseTargetsNeverReachTheCommandLineAsOptions(t *testing.T) {
	const destination = "discord://id/token"
	cfg := AppriseConfig{
		Enabled: true,
		Mode:    AppriseModeCLI,
		Targets: []string{"--attach=/etc/passwd", "  -a /etc/passwd", "--plugin-path=/tmp/x", destination},
	}

	if got := NormalizeAppriseConfig(cfg).Targets; !slices.Equal(got, []string{destination}) {
		t.Fatalf("normalized targets = %v, want only the destination", got)
	}

	manager := NewNotificationManager("")
	t.Cleanup(manager.Stop)
	var args []string
	manager.appriseExec = func(_ context.Context, got []string) (int64, error) {
		args = got
		return 0, nil
	}
	if err := manager.SendTestAppriseWithConfig(cfg); err != nil {
		t.Fatalf("send test: %v", err)
	}
	if !slices.Contains(args, destination) {
		t.Fatalf("command line %q lost the destination", args)
	}
	for _, arg := range args {
		if strings.Contains(arg, "attach") || strings.Contains(arg, "plugin-path") || strings.Contains(arg, "/etc/passwd") {
			t.Fatalf("command line %q carries an injected option", args)
		}
	}
}
