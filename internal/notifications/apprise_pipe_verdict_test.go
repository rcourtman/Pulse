package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// Emit only synthetic public diagnostics for source-bound browser replay through
// the existing test-send consumer, not provider output or private arguments.
func TestAppriseInheritedPipeTestSendVerdict(t *testing.T) {
	for _, mode := range []string{"success", "drained"} {
		t.Run(mode, func(t *testing.T) {
			prepareApprisePipeFixture(t, mode, "both")
			manager := &NotificationManager{appriseExec: func(ctx context.Context, args []string) (int64, error) {
				return defaultAppriseExec(ctx, append([]string{"-test.run=^TestApprisePipeFixture$", "--"}, args...))
			}}
			started := time.Now()
			err := manager.SendTestAppriseWithConfig(AppriseConfig{
				Enabled: true, Mode: AppriseModeCLI, Targets: appriseFixtureArgs[4:], TimeoutSeconds: 5,
			})
			if time.Since(started) > 5*time.Second {
				t.Error("test send exceeded bounded output wait")
			}
			message := ""
			if mode == "success" {
				if !errors.Is(err, exec.ErrWaitDelay) || ClassifyNotificationFailureError(err) != NotificationFailureUnknown {
					t.Fatalf("unclosed test output = %v; want retained unclassified wait failure", err)
				}
				message = err.Error()
				if strings.Contains(message, "synthetic-secret") || strings.Contains(message, "private-alert-body") {
					t.Fatal("test-send failure exposed synthetic private content")
				}
			} else if err != nil {
				t.Fatal(err)
			}
			verdict, err := json.Marshal(map[string]any{"mode": mode, "error": message, "elapsed_ms": time.Since(started).Milliseconds()})
			if err != nil {
				t.Fatal(err)
			}
			fmt.Printf("APPRISE_PIPE_PRODUCER %s\n", verdict)
		})
	}
}
