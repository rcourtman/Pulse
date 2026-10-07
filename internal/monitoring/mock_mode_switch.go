package monitoring

import (
	"sync"
	"sync/atomic"

	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
)

// Mock mode is process-wide (mock.SetEnabled flips one package flag), while
// every Monitor keeps its own alert manager, fixture agents, state and
// mock-mode fence, and the server runs one Monitor per organization. A switch
// made through one monitor therefore has to reach every running monitor, or
// the others keep what they raised in the mode the process left: fixture
// alerts and fixture-agent node links in live mode, live alerts in mock mode,
// until the 24-hour stale sweep.
//
// mockModeSwitchMu serializes switches and guards the set of running monitors
// and each monitor's mockModeAligned and mockModeSwitchesSeen. A switch is
// several steps per monitor (end the epoch, clear, reset), so two interleaved
// switches could clear what the later one just admitted. Start joins the set
// under the same lock, so a monitor joins either before a switch, which then
// switches it with the others, or after it, and starts in the new mode.
//
// mockModeSwitchCount counts the flips SetMockMode made. A monitor that missed
// flips while it was not running can hold the alerts and state of a mode the
// process left even when a later flip brought the flag back to the mode it
// records (a read path evaluated fixture data before Start), so Start
// compares the count as well as the mode.
var (
	mockModeSwitchMu        sync.Mutex
	mockModeRunningMonitors = make(map[*Monitor]struct{})
	mockModeSwitchCount     atomic.Uint64
)

// mockModeSwitchTargetsLocked returns the monitors a switch made through m
// reaches: m itself, which need not be running, and every running monitor.
func mockModeSwitchTargetsLocked(m *Monitor) []*Monitor {
	targets := make([]*Monitor, 0, len(mockModeRunningMonitors)+1)
	targets = append(targets, m)
	for monitor := range mockModeRunningMonitors {
		if monitor != m {
			targets = append(targets, monitor)
		}
	}
	return targets
}

// joinMockModeSwitches adds m to the monitors every switch reaches, then runs
// start with the current mode while no switch can run, so the mode-dependent
// runtime start chooses (the mock metrics sampler or discovery) cannot be
// overtaken by a switch. It returns the function that removes m again.
//
// A switch can land between New and Start, while m is not yet running. New
// restores persisted alerts only outside mock mode, so m would then carry live
// alerts into mock mode, or fixture alerts that a read path raised before
// Start into live mode; m first leaves what it holds, as the switches it
// missed would have done.
func (m *Monitor) joinMockModeSwitches(start func(mockEnabled bool)) (leave func()) {
	mockModeSwitchMu.Lock()
	defer mockModeSwitchMu.Unlock()

	enabled := mock.IsMockEnabled()
	if m.mockModeAligned != enabled || m.mockModeSwitchesSeen != mockModeSwitchCount.Load() {
		m.endMockModeEpoch(enabled)
	}
	mockModeRunningMonitors[m] = struct{}{}
	start(enabled)

	return func() {
		mockModeSwitchMu.Lock()
		defer mockModeSwitchMu.Unlock()
		delete(mockModeRunningMonitors, m)
	}
}
