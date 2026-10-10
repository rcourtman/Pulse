package monitoring

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// simulateAdaptiveSuccessGaps drives the real adaptive scheduler, interval
// selector and staleness scoring for one always-healthy instance on a
// simulated clock, the way the monitor does: a worker runs the queued task
// once it is due and reschedules it, and a planning pass re-plans every poll
// tick. It returns the gaps between successive successful polls after warmup.
// The worker is instantaneous and uncontended, so real gaps can run longer
// when workers are busy; the factor of two in the threshold is headroom for
// that, not a bound.
func simulateAdaptiveSuccessGaps(t *testing.T, cfg SchedulerConfig, instanceType InstanceType, tick, warmup, horizon time.Duration) []time.Duration {
	t.Helper()
	const name = "instance-1"
	tracker := NewStalenessTracker(nil)
	tracker.SetBounds(cfg.BaseInterval, cfg.MaxInterval)
	scheduler := NewAdaptiveScheduler(cfg, tracker, nil, nil)

	start := time.Unix(1_800_000_000, 0)
	simNow := start
	var lastSuccess time.Time
	plan := func(desc InstanceDescriptor) ScheduledTask {
		// StalenessTracker scores against the wall clock, so stamp the
		// simulated age of the last success just before planning.
		if !lastSuccess.IsZero() {
			tracker.setSnapshot(FreshnessSnapshot{
				InstanceType: instanceType,
				Instance:     name,
				LastSuccess:  time.Now().Add(-simNow.Sub(lastSuccess)),
			})
		}
		tasks := scheduler.BuildPlan(simNow, []InstanceDescriptor{desc}, 0)
		if len(tasks) != 1 {
			t.Fatalf("BuildPlan returned %d tasks, want 1", len(tasks))
		}
		return tasks[0]
	}
	planningPass := func() ScheduledTask {
		desc := InstanceDescriptor{Name: name, Type: instanceType, LastSuccess: lastSuccess}
		if last, ok := scheduler.LastScheduled(instanceType, name); ok {
			desc.LastScheduled = last.NextRun
			desc.LastInterval = last.Interval
		}
		return plan(desc)
	}

	queued := planningPass()
	var gaps []time.Duration
	for step := time.Duration(0); step <= horizon; step += time.Second {
		simNow = start.Add(step)
		if !simNow.Before(queued.NextRun) {
			if !lastSuccess.IsZero() && step >= warmup {
				gaps = append(gaps, simNow.Sub(lastSuccess))
			}
			lastSuccess = simNow
			queued = plan(InstanceDescriptor{
				Name:          name,
				Type:          instanceType,
				LastInterval:  queued.Interval,
				LastScheduled: queued.NextRun,
				LastSuccess:   lastSuccess,
			})
		}
		if step > 0 && step%tick == 0 {
			queued = planningPass()
		}
	}
	if len(gaps) == 0 {
		t.Fatalf("%s: no polls after warmup", instanceType)
	}
	return gaps
}

func longestGap(gaps []time.Duration) time.Duration {
	var longest time.Duration
	for _, gap := range gaps {
		longest = max(longest, gap)
	}
	return longest
}

// With adaptive polling enabled, a healthy instance's staleness score is near
// zero after every success, so the scheduler stretches its cadence toward
// AdaptivePollingMaxInterval and never reads the per-platform intervals.
// Freshness, node offline grace and the temperature carry window must cover
// that cadence, or healthy Proxmox, PBS and PMG rows read stale (and nodes
// lose their grace) for most of every cycle.
func TestMonitorResourceFreshnessFollowsAdaptiveCadence(t *testing.T) {
	sources := map[InstanceType]unifiedresources.DataSource{
		InstanceTypePVE: unifiedresources.SourceProxmox,
		InstanceTypePBS: unifiedresources.SourcePBS,
		InstanceTypePMG: unifiedresources.SourcePMG,
	}
	for _, bounds := range []SchedulerConfig{
		{BaseInterval: 10 * time.Second, MinInterval: 5 * time.Second, MaxInterval: 5 * time.Minute},
		{BaseInterval: 30 * time.Second, MinInterval: 10 * time.Second, MaxInterval: 15 * time.Minute},
	} {
		monitor := &Monitor{
			config: &config.Config{
				PVEPollingInterval: 10 * time.Second,
				PBSPollingInterval: time.Minute,
				PMGPollingInterval: time.Minute,
			},
			scheduler: NewAdaptiveScheduler(bounds, nil, nil, nil),
		}
		thresholds := monitor.resourceStaleThresholds()
		for instanceType, source := range sources {
			gaps := simulateAdaptiveSuccessGaps(t, bounds, instanceType, 10*time.Second, time.Hour, 8*time.Hour)
			longest := longestGap(gaps)
			if longest > bounds.MaxInterval+time.Second {
				t.Fatalf("%s: healthy polls %v apart exceed the scheduler's %v maximum", instanceType, longest, bounds.MaxInterval)
			}
			if got, want := thresholds[source], 2*bounds.MaxInterval; got != want {
				t.Errorf("max %v: %s threshold = %v, want %v from the adaptive cadence (healthy polls up to %v apart)",
					bounds.MaxInterval, source, got, want, longest)
			}
			if instanceType != InstanceTypePVE {
				continue
			}
			if grace := monitor.pveNodeOfflineGracePeriod(); grace <= longest {
				t.Errorf("max %v: node offline grace %v expires before the next healthy poll (%v apart)", bounds.MaxInterval, grace, longest)
			}
			if carry, want := monitor.nodeTemperatureCarryWindow(), max(2*bounds.MaxInterval, nodeTemperatureCarryFloor); carry != want || carry <= longest {
				t.Errorf("max %v: temperature carry window = %v, want %v from the adaptive cadence (healthy polls up to %v apart)", bounds.MaxInterval, carry, want, longest)
			}
		}
	}
}

// A node the API reports offline is only believed once it has been away
// longer than one healthy adaptive cycle; past twice the cadence the grace
// lapses as it does under fixed-cadence polling.
func TestAdaptiveNodeOfflineGraceSpansTheAdaptiveCycle(t *testing.T) {
	instance := &config.PVEInstance{Name: "site-a"}
	monitor := &Monitor{
		config:         &config.Config{PVEPollingInterval: 10 * time.Second},
		scheduler:      NewAdaptiveScheduler(SchedulerConfig{BaseInterval: 10 * time.Second, MinInterval: 5 * time.Second, MaxInterval: 5 * time.Minute}, nil, nil, nil),
		nodeLastOnline: map[string]time.Time{"site-a-pve-a": time.Now().Add(-4 * time.Minute)},
	}
	offline := proxmox.Node{Node: "pve-a", Status: "offline"}
	if _, status := monitor.determineNodeIDAndStatus("site-a", instance, offline); status != "online" {
		t.Fatalf("node status = %q one adaptive cycle after its last online poll, want online inside the grace", status)
	}
	monitor.nodeLastOnline["site-a-pve-a"] = time.Now().Add(-11 * time.Minute)
	if _, status := monitor.determineNodeIDAndStatus("site-a", instance, offline); status != "offline" {
		t.Fatalf("node status = %q past twice the adaptive cadence, want offline", status)
	}
}

// Callers without a live monitor (adapter construction) derive freshness from
// config alone. It must name the same cadence the monitor's scheduler will
// run, including the defaults the scheduler fills in for unset or inconsistent
// adaptive bounds.
func TestResourceStaleThresholdsForConfigFollowAdaptiveBounds(t *testing.T) {
	cases := []struct {
		name string
		cfg  config.Config
		want time.Duration
	}{
		{
			name: "adaptive off keeps the per-platform intervals",
			cfg: config.Config{
				PVEPollingInterval: 10 * time.Second, PBSPollingInterval: 3 * time.Minute, PMGPollingInterval: 3 * time.Minute,
				AdaptivePollingMaxInterval: 15 * time.Minute,
			},
		},
		{
			name: "configured maximum",
			cfg: config.Config{
				AdaptivePollingEnabled:     true,
				PBSPollingInterval:         3 * time.Minute,
				AdaptivePollingMinInterval: 5 * time.Second, AdaptivePollingMaxInterval: 15 * time.Minute,
			},
			want: 30 * time.Minute,
		},
		{
			name: "unset bounds use the scheduler default",
			cfg:  config.Config{AdaptivePollingEnabled: true},
			want: 2 * DefaultSchedulerConfig().MaxInterval,
		},
		{
			name: "maximum below the minimum uses the scheduler default",
			cfg: config.Config{
				AdaptivePollingEnabled:     true,
				AdaptivePollingMinInterval: time.Minute, AdaptivePollingMaxInterval: 30 * time.Second,
			},
			want: 2 * DefaultSchedulerConfig().MaxInterval,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := tc.cfg
			thresholds := ResourceStaleThresholdsForConfig(&cfg)
			for _, source := range []unifiedresources.DataSource{unifiedresources.SourceProxmox, unifiedresources.SourcePBS, unifiedresources.SourcePMG} {
				want := tc.want
				if !cfg.AdaptivePollingEnabled {
					want = map[unifiedresources.DataSource]time.Duration{
						unifiedresources.SourceProxmox: defaultProxmoxResourceStaleThreshold,
						unifiedresources.SourcePBS:     6 * time.Minute,
						unifiedresources.SourcePMG:     6 * time.Minute,
					}[source]
				}
				if got := thresholds[source]; got != want {
					t.Errorf("%s threshold = %v, want %v", source, got, want)
				}
			}
			if !cfg.AdaptivePollingEnabled {
				return
			}
			scheduler := NewAdaptiveScheduler(SchedulerConfig{
				BaseInterval: cfg.AdaptivePollingBaseInterval,
				MinInterval:  cfg.AdaptivePollingMinInterval,
				MaxInterval:  cfg.AdaptivePollingMaxInterval,
			}, nil, nil, nil)
			monitor := &Monitor{config: &cfg, scheduler: scheduler}
			for source, want := range thresholds {
				if source != unifiedresources.SourceProxmox && source != unifiedresources.SourcePBS && source != unifiedresources.SourcePMG {
					continue
				}
				if got := monitor.resourceStaleThresholds()[source]; got != want {
					t.Errorf("%s: monitor threshold %v differs from the config-only %v", source, got, want)
				}
			}
		})
	}
}
