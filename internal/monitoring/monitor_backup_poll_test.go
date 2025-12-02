package monitoring

import (
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

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
			name: "cycle-based: pollCounter=1 returns true",
			monitor: &Monitor{
				config: &config.Config{
					EnableBackupPolling:   true,
					BackupPollingCycles:   10,
					BackupPollingInterval: 0,
				},
				pollCounter: 1,
			},
			last:           last,
			now:            now,
			wantRun:        true,
			wantReasonSub:  "",
			wantReturnLast: false,
		},
		{
			name: "cycle-based: pollCounter divisible by cycles returns true",
			monitor: &Monitor{
				config: &config.Config{
					EnableBackupPolling:   true,
					BackupPollingCycles:   5,
					BackupPollingInterval: 0,
				},
				pollCounter: 15, // 15 % 5 == 0
			},
			last:           last,
			now:            now,
			wantRun:        true,
			wantReasonSub:  "",
			wantReturnLast: false,
		},
		{
			name: "cycle-based: pollCounter not divisible returns false",
			monitor: &Monitor{
				config: &config.Config{
					EnableBackupPolling:   true,
					BackupPollingCycles:   5,
					BackupPollingInterval: 0,
				},
				pollCounter: 7, // 7 % 5 == 2, remaining = 3
			},
			last:           last,
			now:            now,
			wantRun:        false,
			wantReasonSub:  "next run in 3 polling cycles",
			wantReturnLast: true,
		},
		{
			name: "default cycles (10) when BackupPollingCycles is 0",
			monitor: &Monitor{
				config: &config.Config{
					EnableBackupPolling:   true,
					BackupPollingCycles:   0, // should default to 10
					BackupPollingInterval: 0,
				},
				pollCounter: 10, // 10 % 10 == 0
			},
			last:           last,
			now:            now,
			wantRun:        true,
			wantReasonSub:  "",
			wantReturnLast: false,
		},
		{
			name: "default cycles (10) when BackupPollingCycles is negative",
			monitor: &Monitor{
				config: &config.Config{
					EnableBackupPolling:   true,
					BackupPollingCycles:   -5, // should default to 10
					BackupPollingInterval: 0,
				},
				pollCounter: 20, // 20 % 10 == 0
			},
			last:           last,
			now:            now,
			wantRun:        true,
			wantReasonSub:  "",
			wantReturnLast: false,
		},
		{
			name: "default cycles (10) not divisible returns false with correct remaining",
			monitor: &Monitor{
				config: &config.Config{
					EnableBackupPolling:   true,
					BackupPollingCycles:   0, // defaults to 10
					BackupPollingInterval: 0,
				},
				pollCounter: 3, // 3 % 10 == 3, remaining = 7
			},
			last:           last,
			now:            now,
			wantRun:        false,
			wantReasonSub:  "next run in 7 polling cycles",
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
