package monitoring

import (
	"errors"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
)

// Regression tests for the provider half of #1582: the scheduler already pins
// InstanceDescriptor.FixedInterval exactly (TestBuildPlan_FixedInterval), but
// the availability provider must actually populate that descriptor from the
// target's configured interval, or the adaptive scheduler takes over again.

func availabilityFixedIntervalTestMonitor(t *testing.T, targets []config.AvailabilityTarget) *Monitor {
	t.Helper()
	persistence := config.NewConfigPersistence(t.TempDir())
	if err := persistence.SaveAvailabilityTargets(targets); err != nil {
		t.Fatalf("SaveAvailabilityTargets() error = %v", err)
	}
	return &Monitor{configPersist: persistence}
}

func TestAvailabilityPollProviderFixedInstanceInterval(t *testing.T) {
	monitor := availabilityFixedIntervalTestMonitor(t, []config.AvailabilityTarget{
		{ID: "plex", Name: "Plex", Address: "plex.local", Protocol: config.AvailabilityProbeICMP, Enabled: true, PollIntervalSecs: 120},
		{ID: "defaulted", Name: "Defaulted", Address: "defaulted.local", Protocol: config.AvailabilityProbeICMP, Enabled: true},
		{ID: "oversized", Name: "Oversized", Address: "oversized.local", Protocol: config.AvailabilityProbeICMP, Enabled: true, PollIntervalSecs: 7200},
		{ID: "paused", Name: "Paused", Address: "paused.local", Protocol: config.AvailabilityProbeICMP, Enabled: false, PollIntervalSecs: 120},
	})
	provider := availabilityPollProvider{}

	if got := provider.FixedInstanceInterval(monitor, "plex"); got != 120*time.Second {
		t.Fatalf("FixedInstanceInterval(plex) = %v, want 120s", got)
	}
	wantDefault := time.Duration(config.DefaultAvailabilityPollIntervalSecs) * time.Second
	if got := provider.FixedInstanceInterval(monitor, "defaulted"); got != wantDefault {
		t.Fatalf("FixedInstanceInterval(defaulted) = %v, want %v", got, wantDefault)
	}
	if got := provider.FixedInstanceInterval(monitor, "oversized"); got != time.Hour {
		t.Fatalf("FixedInstanceInterval(oversized) = %v, want clamp to 1h", got)
	}
	if got := provider.FixedInstanceInterval(monitor, "paused"); got != 0 {
		t.Fatalf("FixedInstanceInterval(paused) = %v, want 0 for disabled target", got)
	}
	if got := provider.FixedInstanceInterval(monitor, "missing"); got != 0 {
		t.Fatalf("FixedInstanceInterval(missing) = %v, want 0 for unknown target", got)
	}
}

// End-to-end wiring: the scheduler descriptor for an availability target must
// carry the configured interval as FixedInterval.
func TestDescribeInstancesForSchedulerPinsAvailabilityFixedInterval(t *testing.T) {
	monitor := availabilityFixedIntervalTestMonitor(t, []config.AvailabilityTarget{
		{ID: "plex", Name: "Plex", Address: "plex.local", Protocol: config.AvailabilityProbeICMP, Enabled: true, PollIntervalSecs: 120},
	})

	descriptors := monitor.describeInstancesForScheduler()
	var found *InstanceDescriptor
	for i := range descriptors {
		if descriptors[i].Type == InstanceTypeAvailability && descriptors[i].Name == "plex" {
			found = &descriptors[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("no availability descriptor for plex in %+v", descriptors)
	}
	if found.FixedInterval != 120*time.Second {
		t.Fatalf("descriptor FixedInterval = %v, want 120s", found.FixedInterval)
	}
}

// Count real catalog reads without replacing the encrypted persistence or its
// decoder. Only the reader uses this filesystem; the writer uses the real one.
type availabilityPlanningReadCounter struct {
	config.FileSystem
	reads int
	err   error
}

func (fs *availabilityPlanningReadCounter) ReadFile(path string) ([]byte, error) {
	fs.reads++
	if fs.err != nil {
		return nil, fs.err
	}
	return os.ReadFile(path)
}

func newAvailabilityPlanningFixture(t testing.TB, targets []config.AvailabilityTarget) (*Monitor, *config.ConfigPersistence, *availabilityPlanningReadCounter) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("PULSE_LEGACY_KEY_PATH", filepath.Join(dir, "absent-test-legacy.key"))
	writer := config.NewConfigPersistence(dir)
	if err := writer.SaveAvailabilityTargets(targets); err != nil {
		t.Fatal(err)
	}
	reader := config.NewConfigPersistence(dir)
	counter := &availabilityPlanningReadCounter{}
	reader.SetFileSystem(counter)
	monitor := &Monitor{
		config:        &config.Config{PVEPollingInterval: 45 * time.Second, PBSPollingInterval: 90 * time.Second},
		configPersist: reader,
		pveClients:    map[string]PVEClientInterface{"pve-peer": nil},
		pbsClients:    map[string]*pbs.Client{"pbs-peer": nil},
		taskQueue:     NewTaskQueue(),
	}
	return monitor, writer, counter
}

func TestAvailabilityPollLookupsUseTheCurrentSavedTarget(t *testing.T) {
	target := probeAgentTarget("z", "")
	target.Name, target.PollIntervalSecs = "Z local", 120
	monitor, writer, reads := newAvailabilityPlanningFixture(t, []config.AvailabilityTarget{
		target, probeAgentTarget("a", ""),
	})
	monitor.SetLicenseChecker(licenseWithExternalProbe(true))
	provider := availabilityPollProvider{}
	check := func(id string, interval time.Duration, admitted bool) {
		t.Helper()
		before := reads.reads
		if got := provider.FixedInstanceInterval(monitor, id); got != interval {
			t.Errorf("FixedInstanceInterval(%q) = %v, want %v", id, got, interval)
		}
		task, err := provider.BuildPollTask(monitor, id)
		if (err == nil) != admitted || (admitted && (task.InstanceName != "z" || task.Run == nil)) {
			t.Errorf("BuildPollTask(%q) instance = %q, error = %v; want admitted = %v", id, task.InstanceName, err, admitted)
		}
		if got := reads.reads - before; got != 2 {
			t.Errorf("lookups read catalog %d times; want one read each", got)
		}
	}
	check(" z ", 120*time.Second, true)

	// A persisted duplicate must keep the established display-order
	// selection, not start polling the first entry on disk instead.
	duplicate := target
	duplicate.Name, duplicate.PollIntervalSecs, duplicate.Enabled = "A paused", 180, false
	if err := writer.SaveAvailabilityTargets([]config.AvailabilityTarget{target, duplicate}); err != nil {
		t.Fatal(err)
	}
	check("z", 0, false)

	// Assignment and cadence edits take effect on the next lookup.
	remote := probeAgentTarget("z", "edge")
	remote.PollIntervalSecs = 180
	if err := writer.SaveAvailabilityTargets([]config.AvailabilityTarget{remote}); err != nil {
		t.Fatal(err)
	}
	check("z", 180*time.Second, false)
	monitor.SetLicenseChecker(licenseWithExternalProbe(false))
	check("z", 180*time.Second, true)

	if err := writer.SaveAvailabilityTargets(nil); err != nil {
		t.Fatal(err)
	}
	check("z", 0, false)
	reads.err = errors.New("fixture: catalog read unavailable")
	check("z", 0, false)
}

func TestAvailabilityPlanningReadsSavedCatalogOnce(t *testing.T) {
	for _, mode := range []string{"fixed", "adaptive"} {
		t.Run(mode, func(t *testing.T) {
			local := probeAgentTarget("local", "")
			local.Name, local.PollIntervalSecs = "Z local", 120
			both := probeAgentTarget("both", "")
			both.ObservationLocationIDs = []string{config.AvailabilityObservationLocationLocal, config.AvailabilityAgentObservationLocationID("edge")}
			remote := probeAgentTarget("remote", "edge")
			paused := probeAgentTarget("paused", "")
			paused.Enabled = false
			push := config.AvailabilityTarget{ID: "push", Name: "Push", Protocol: config.AvailabilityProbePush, Enabled: true}
			targets := []config.AvailabilityTarget{local, both, remote, paused, push}
			monitor, writer, counter := newAvailabilityPlanningFixture(t, targets)
			monitor.SetLicenseChecker(licenseWithExternalProbe(true))
			pbsCadence := 90 * time.Second
			if mode == "adaptive" {
				monitor.scheduler = NewAdaptiveScheduler(SchedulerConfig{
					BaseInterval: 45 * time.Second, MinInterval: 45 * time.Second, MaxInterval: 45 * time.Second,
				}, nil, nil, nil)
				pbsCadence = 45 * time.Second
			}
			now := time.Now()
			checkPlan := func(phase string, want map[string]time.Duration) []string {
				t.Helper()
				counter.reads = 0
				tasks := monitor.buildScheduledTasks(now)
				now = now.Add(time.Second)
				got := make(map[string]time.Duration)
				var availabilityOrder []string
				for _, task := range tasks {
					got[task.InstanceName] = task.Interval
					if task.InstanceType == InstanceTypeAvailability {
						availabilityOrder = append(availabilityOrder, task.InstanceName)
					}
				}
				want["pve-peer"], want["pbs-peer"] = 45*time.Second, pbsCadence
				t.Logf("%s: catalog reads=%d; plan=%v", phase, counter.reads, got)
				if counter.reads != 1 {
					t.Errorf("%s: read saved availability catalog %d times, want one per planning pass", phase, counter.reads)
				}
				if !maps.Equal(got, want) || len(tasks) != len(want) {
					t.Errorf("%s: %d tasks with intervals %v, want %v", phase, len(tasks), got, want)
				}
				return availabilityOrder
			}

			order := checkPlan("initial", map[string]time.Duration{
				"local": 120 * time.Second, "both": 60 * time.Second, "push": availabilityPushEvalInterval,
			})
			if !slices.Equal(order, []string{"both", "local", "push"}) {
				t.Errorf("initial availability plan order = %v, want instance-ID order", order)
			}
			// The next plan uses the new saved roster, not an enduring cache.
			targets[0].Enabled = false
			targets[1].PollIntervalSecs = 180
			if err := writer.SaveAvailabilityTargets(targets); err != nil {
				t.Fatal(err)
			}
			checkPlan("edited", map[string]time.Duration{"both": 180 * time.Second, "push": availabilityPushEvalInterval})
			if _, ok := monitor.buildPollTask(ScheduledTask{InstanceName: local.ID, InstanceType: InstanceTypeAvailability}); ok {
				t.Error("worker admitted a previously planned target after it was disabled")
			}

			monitor.SetLicenseChecker(licenseWithExternalProbe(false))
			checkPlan("entitlement lapse", map[string]time.Duration{
				"both": 180 * time.Second, "push": availabilityPushEvalInterval, "remote": 60 * time.Second,
			})
			counter.err = errors.New("fixture: catalog read unavailable")
			checkPlan("unreadable", map[string]time.Duration{})
		})
	}
}
