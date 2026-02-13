package dockeragent

import (
	"context"
	"errors"
	"testing"
	"time"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/rs/zerolog"
)

func TestCleanupOrphanedBackups(t *testing.T) {
	t.Run("container list error", func(t *testing.T) {
		agent := &Agent{
			logger: zerolog.Nop(),
			docker: &fakeDockerClient{
				containerListFunc: func(context.Context, containertypes.ListOptions) ([]containertypes.Summary, error) {
					return nil, errors.New("list failed")
				},
			},
		}

		agent.cleanupOrphanedBackups(context.Background())
	})

	t.Run("removes only stale backups", func(t *testing.T) {
		oldTimestamp := time.Now().Add(-20 * time.Minute).Format("20060102_150405")
		recentTimestamp := time.Now().Add(-5 * time.Minute).Format("20060102_150405")
		oldCreated := time.Now().Add(-20 * time.Minute).Unix()
		recentCreated := time.Now().Add(-5 * time.Minute).Unix()

		removed := make(map[string]containertypes.RemoveOptions)
		agent := &Agent{
			logger: zerolog.Nop(),
			docker: &fakeDockerClient{
				containerListFunc: func(_ context.Context, opts containertypes.ListOptions) ([]containertypes.Summary, error) {
					if !opts.All {
						t.Fatal("expected cleanup list query to set All=true")
					}
					return []containertypes.Summary{
						{ID: "non-backup", Names: []string{"/service"}, Created: oldCreated},
						{ID: "old-parseable", Names: []string{"/service_pulse_backup_" + oldTimestamp}, Created: recentCreated},
						{ID: "recent-parseable", Names: []string{"/service_pulse_backup_" + recentTimestamp}, Created: oldCreated},
						{ID: "old-fallback", Names: []string{"/service_pulse_backup_invalid"}, Created: oldCreated},
						{ID: "recent-fallback", Names: []string{"/service_pulse_backup_invalid"}, Created: recentCreated},
						{ID: "secondary-name-only", Names: []string{"/service", "/service_pulse_backup_" + oldTimestamp}, Created: oldCreated},
					}, nil
				},
				containerRemoveFn: func(_ context.Context, id string, opts containertypes.RemoveOptions) error {
					removed[id] = opts
					if id == "old-fallback" {
						return errors.New("remove failed")
					}
					return nil
				},
			},
		}

		agent.cleanupOrphanedBackups(context.Background())

		if len(removed) != 2 {
			t.Fatalf("expected exactly 2 stale backup removals, got %d (%v)", len(removed), removed)
		}
		if _, ok := removed["old-parseable"]; !ok {
			t.Fatal("expected parseable stale backup to be removed")
		}
		if _, ok := removed["old-fallback"]; !ok {
			t.Fatal("expected fallback stale backup to be removed")
		}
		if !removed["old-parseable"].Force || !removed["old-fallback"].Force {
			t.Fatal("expected stale backup removals to use Force=true")
		}
	})
}
