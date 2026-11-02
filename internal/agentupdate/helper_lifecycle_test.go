package agentupdate

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

func TestRunLoopExplainsSeparateHelperLifecycleWithoutChangingHealth(t *testing.T) {
	for _, tc := range []struct {
		name     string
		helper   bool
		disabled bool
		version  string
		wantNote bool
	}{
		{name: "typed_helper", helper: true, version: "v6.7.0", wantNote: true},
		{name: "ordinary_collector", version: "v6.7.0"},
		{name: "disabled", helper: true, disabled: true, version: "v6.7.0"},
		{name: "development", helper: true, version: "dev"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var output bytes.Buffer
			logger := zerolog.New(&output)
			cfg := Config{CurrentVersion: tc.version, Disabled: tc.disabled, Logger: &logger,
				PulseURL: "https://example.invalid", APIToken: "must-not-be-logged"}
			helper := &fakePrivilegedUpdate{root: t.TempDir()}
			if tc.helper {
				cfg.PrivilegedUpdate = helper
			}
			u := New(cfg)
			before := u.Snapshot()
			ctx, cancel := context.WithCancel(context.Background())
			cancel() // Reach startup output without a network check or a mutation.
			u.RunLoop(ctx)
			if got := u.Snapshot(); got.State != before.State || got.LastError != before.LastError || got.LastAttemptAt != nil {
				t.Fatalf("lifecycle explanation changed updater health: before=%+v after=%+v", before, got)
			}
			if len(helper.events) != 0 || strings.Contains(output.String(), cfg.APIToken) {
				t.Fatalf("startup explanation invoked helper or disclosed credential: events=%v", helper.events)
			}
			notes := 0
			decoder := json.NewDecoder(&output)
			for decoder.More() {
				var event map[string]any
				if err := decoder.Decode(&event); err != nil {
					t.Fatal(err)
				}
				if event["update_scope"] != "collector-only" {
					continue
				}
				notes++
				message, _ := event["message"].(string)
				if event["level"] != "info" || !strings.Contains(message, "agent installer --update") ||
					event["helper_update_guide"] != "https://github.com/rcourtman/Pulse/blob/main/docs/UNIFIED_AGENT.md#updating-the-typed-helper" {
					t.Fatalf("missing actionable helper lifecycle: %+v", event)
				}
			}
			want := 0
			if tc.wantNote {
				want = 1
			}
			if notes != want {
				t.Fatalf("helper lifecycle notes = %d, want %d", notes, want)
			}
		})
	}
}
