package monitoring

import (
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestPVEBackupPermissionWarningUsesSavedIdentityWithoutACLMutation(t *testing.T) {
	for _, cfg := range []*config.PVEInstance{nil, {}, {TokenName: "custom-reader@pve!manual"}} {
		warning := pveBackupPermissionWarning(cfg)
		for _, snippet := range []string{"saved PVE connection", "both user and token scopes", "without disabling privilege separation", "rejected endpoint", "installed PVE version", "Do not delete nodes"} {
			if !strings.Contains(warning, snippet) {
				t.Fatalf("missing safe guidance %q: %q", snippet, warning)
			}
		}
		for _, unsafe := range []string{"pveum", "PVEDatastoreAdmin", "pulse-monitor@pve", "custom-reader@pve!manual"} {
			if strings.Contains(warning, unsafe) {
				t.Fatalf("warning prescribes or discloses %q: %q", unsafe, warning)
			}
		}
	}
}

func TestShouldRunBackupPoll(t *testing.T) {
	now := time.Now()
	last := now.Add(-5 * time.Minute)

	tests := []struct {
		name           string
		monitor        *Monitor
		last           time.Time
		now            time.Time
		wantRun        bool
		wantReasonSub  string // substring to check in reason
		wantReturnLast bool   // true if returned time should equal last, false if should equal now
	}{
		{
			name:           "nil monitor returns false",
			monitor:        nil,
			last:           last,
			now:            now,
			wantRun:        false,
			wantReasonSub:  "configuration unavailable",
			wantReturnLast: true,
		},
		{
			name:           "nil config returns false",
			monitor:        &Monitor{config: nil},
			last:           last,
			now:            now,
			wantRun:        false,
			wantReasonSub:  "configuration unavailable",
			wantReturnLast: true,
		},
		{
			name: "backup polling disabled returns false",
			monitor: &Monitor{
				config: &config.Config{EnableBackupPolling: false},
			},
			last:           last,
			now:            now,
			wantRun:        false,
			wantReasonSub:  "backup polling globally disabled",
			wantReturnLast: true,
		},
		{
			name: "interval-based: before interval elapsed returns false",
			monitor: &Monitor{
				config: &config.Config{
					EnableBackupPolling:   true,
					BackupPollingInterval: 10 * time.Minute,
				},
			},
			last:           last, // 5 min ago, interval is 10 min
			now:            now,
			wantRun:        false,
			wantReasonSub:  "next run scheduled for",
			wantReturnLast: true,
		},
		{
			name: "interval-based: after interval elapsed returns true",
			monitor: &Monitor{
				config: &config.Config{
					EnableBackupPolling:   true,
					BackupPollingInterval: 3 * time.Minute,
				},
			},
			last:           last, // 5 min ago, interval is 3 min
			now:            now,
			wantRun:        true,
			wantReasonSub:  "",
			wantReturnLast: false,
		},
		{
			name: "interval-based: last is zero (first run) returns true",
			monitor: &Monitor{
				config: &config.Config{
					EnableBackupPolling:   true,
					BackupPollingInterval: 10 * time.Minute,
				},
			},
			last:           time.Time{}, // zero time
			now:            now,
			wantRun:        true,
			wantReasonSub:  "",
			wantReturnLast: false,
		},
		{
			name: "cycle-based: first poll runs",
			monitor: &Monitor{
				config: &config.Config{
					EnableBackupPolling: true,
					BackupPollingCycles: 10,
					PVEPollingInterval:  10 * time.Second,
				},
			},
			last:           time.Time{},
			now:            now,
			wantRun:        true,
			wantReasonSub:  "",
			wantReturnLast: false,
		},
		{
			name: "cycle-based: runs once the cycles have elapsed",
			monitor: &Monitor{
				config: &config.Config{
					EnableBackupPolling: true,
					BackupPollingCycles: 5,
					PVEPollingInterval:  10 * time.Second,
				},
			},
			last:           now.Add(-50 * time.Second),
			now:            now,
			wantRun:        true,
			wantReasonSub:  "",
			wantReturnLast: false,
		},
		{
			name: "cycle-based: waits until the cycles have elapsed",
			monitor: &Monitor{
				config: &config.Config{
					EnableBackupPolling: true,
					BackupPollingCycles: 5,
					PVEPollingInterval:  10 * time.Second,
				},
			},
			last:           now.Add(-49 * time.Second),
			now:            now,
			wantRun:        false,
			wantReasonSub:  "next run scheduled for",
			wantReturnLast: true,
		},
		{
			name: "cycle-based: zero cycles defaults to 10",
			monitor: &Monitor{
				config: &config.Config{
					EnableBackupPolling: true,
					BackupPollingCycles: 0,
					PVEPollingInterval:  10 * time.Second,
				},
			},
			last:           now.Add(-99 * time.Second),
			now:            now,
			wantRun:        false,
			wantReasonSub:  "next run scheduled for",
			wantReturnLast: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRun, gotReason, gotTime := tt.monitor.shouldRunBackupPoll(tt.last, tt.now)

			if gotRun != tt.wantRun {
				t.Errorf("shouldRunBackupPoll() run = %v, want %v", gotRun, tt.wantRun)
			}

			if tt.wantReasonSub != "" && !strings.Contains(gotReason, tt.wantReasonSub) {
				t.Errorf("shouldRunBackupPoll() reason = %q, want substring %q", gotReason, tt.wantReasonSub)
			}

			if tt.wantReasonSub == "" && gotReason != "" {
				t.Errorf("shouldRunBackupPoll() reason = %q, want empty", gotReason)
			}

			if tt.wantReturnLast {
				if !gotTime.Equal(tt.last) {
					t.Errorf("shouldRunBackupPoll() time = %v, want last (%v)", gotTime, tt.last)
				}
			} else {
				if !gotTime.Equal(tt.now) {
					t.Errorf("shouldRunBackupPoll() time = %v, want now (%v)", gotTime, tt.now)
				}
			}
		})
	}
}

// A connection polls on its own schedule rather than on every cycle of the
// shared ticker. PBS on a real install polled every 60 seconds against a
// 10-second ticker, so a shared cycle count reached a multiple of 10 on one of
// its polls only once in over 30 minutes (#2681).
func TestBackupPollCadenceFollowsElapsedCyclesOnAnAdaptiveSchedule(t *testing.T) {
	monitor := &Monitor{config: &config.Config{
		EnableBackupPolling: true,
		BackupPollingCycles: 10,
		PVEPollingInterval:  10 * time.Second,
	}}
	start := time.Date(2026, 11, 5, 16, 0, 0, 0, time.UTC)
	var last time.Time
	var runs []time.Duration
	for elapsed := time.Duration(0); elapsed <= 30*time.Minute; elapsed += 61 * time.Second {
		now := start.Add(elapsed)
		run, _, newLast := monitor.shouldRunBackupPoll(last, now)
		if run {
			runs = append(runs, elapsed)
		}
		last = newLast
	}
	if len(runs) != 15 {
		t.Fatalf("backup polls in 30 minutes = %d (%v), want 15, one per elapsed 10 cycles", len(runs), runs)
	}
	for i := 1; i < len(runs); i++ {
		if gap := runs[i] - runs[i-1]; gap != 122*time.Second {
			t.Fatalf("gap between backup polls %d and %d = %s, want 122s", i-1, i, gap)
		}
	}
}
