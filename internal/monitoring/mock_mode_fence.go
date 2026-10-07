package monitoring

import (
	"sync"
	"sync/atomic"
)

// mockModeFence keeps alert evaluations of mode-dependent data inside the
// mock-mode epoch they read it in. GetState, the fixture graph, recovery
// rollups and the connection ledger all switch source when mock mode flips,
// and a pass already holding one side's data can reach the alert manager
// after SetMockMode cleared it. Its fixture alerts then outlive mock mode,
// with nothing left to evaluate or resolve them.
//
// A pass takes a scope before it reads anything mode-dependent and sends each
// alert-manager call through scope.run. SetMockMode flips the mode, advances
// the fence and only then clears: advance refuses every later call made under
// an older epoch and returns once the calls already admitted have finished,
// so nothing a pass read before the flip lands after the clear. advance waits
// for the alert-manager calls already admitted, not for whole passes; reads,
// metric queries and registry rebuilds stay outside it.
//
// A registry rebuild publishes shared state that a later refresh reads back
// and evaluates, so a rebuild still running when the epoch ends can replace a
// newer refresh's registry with the mode the monitor left. Rebuilds go
// through scope.publish, which advance does not wait for (a rebuild can spend
// seconds in SQLite), and a refresh evaluates what it read back only while
// undisturbedSince confirms no ended-epoch publish overlapped it. Readers
// that use the registry outside a refresh ask registryCurrent: after a switch
// the registry still holds the estate of the mode the monitor left, until an
// undisturbed publish in the new epoch replaces it.
//
// Admission never blocks, so a call that re-enters the fence (an alert
// callback that refreshes the resource store) cannot deadlock against a
// waiting advance; an inner call made under an ended epoch is refused like
// any other. advance must not be reached from inside an admitted call. The
// zero value is ready to use.
type mockModeFence struct {
	mu       sync.Mutex
	drained  *sync.Cond
	epoch    uint64
	inFlight map[uint64]int
	// publishing counts registry publishes by epoch; lateFinishes counts
	// publishes that finished after their epoch ended.
	publishing   map[uint64]int
	lateFinishes uint64
	// cleanEpoch and cleanLate record the last undisturbed replacement;
	// registryStale caches whether the registry is current for lock-free
	// readers. Their zero values make a never-switched monitor's registry
	// current.
	cleanEpoch    uint64
	cleanLate     uint64
	registryStale atomic.Bool
}

// mockModeScope is the epoch one pass read its data in.
type mockModeScope struct {
	fence *mockModeFence
	epoch uint64
}

// begin opens a scope in the current epoch. Callers must take it before they
// read the mock mode or any data whose source depends on it.
func (f *mockModeFence) begin() mockModeScope {
	f.mu.Lock()
	defer f.mu.Unlock()
	return mockModeScope{fence: f, epoch: f.epoch}
}

// advance ends the current epoch and waits until no call admitted under an
// ended epoch is still running.
func (f *mockModeFence) advance() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.epoch++
	f.registryStale.Store(true)
	for f.staleInFlightLocked() {
		if f.drained == nil {
			f.drained = sync.NewCond(&f.mu)
		}
		f.drained.Wait()
	}
}

func (f *mockModeFence) staleInFlightLocked() bool {
	for epoch := range f.inFlight {
		if epoch < f.epoch {
			return true
		}
	}
	return false
}

// current reports whether the scope's epoch is still the fence's epoch.
func (s mockModeScope) current() bool {
	s.fence.mu.Lock()
	defer s.fence.mu.Unlock()
	return s.fence.epoch == s.epoch
}

// run calls fn if the scope's epoch is still current and reports whether it
// did. A mode change that starts while fn runs waits for it to return.
func (s mockModeScope) run(fn func()) bool {
	f := s.fence
	f.mu.Lock()
	if f.epoch != s.epoch {
		f.mu.Unlock()
		return false
	}
	if f.inFlight == nil {
		f.inFlight = make(map[uint64]int)
	}
	f.inFlight[s.epoch]++
	f.mu.Unlock()

	defer func() {
		f.mu.Lock()
		defer f.mu.Unlock()
		if f.inFlight[s.epoch]--; f.inFlight[s.epoch] == 0 {
			delete(f.inFlight, s.epoch)
		}
		if f.drained != nil {
			f.drained.Broadcast()
		}
	}()
	fn()
	return true
}

// publish runs fn, which replaces state that later refreshes read back (the
// unified resource registry) and reports whether it did, unless the scope's
// epoch has ended. It returns the mark undisturbedSince compares against.
func (s mockModeScope) publish(fn func() bool) (mark uint64, ok bool) {
	f := s.fence
	f.mu.Lock()
	if f.epoch != s.epoch {
		f.mu.Unlock()
		return 0, false
	}
	if f.publishing == nil {
		f.publishing = make(map[uint64]int)
	}
	f.publishing[s.epoch]++
	mark = f.lateFinishes
	f.mu.Unlock()

	replaced := false
	defer func() {
		f.mu.Lock()
		defer f.mu.Unlock()
		if f.publishing[s.epoch]--; f.publishing[s.epoch] == 0 {
			delete(f.publishing, s.epoch)
		}
		switch {
		case f.epoch != s.epoch:
			f.lateFinishes++
		case replaced && f.lateFinishes == mark && !f.olderPublishingLocked():
			f.cleanEpoch = s.epoch
			f.cleanLate = f.lateFinishes
		}
		f.registryStale.Store(!f.registryCurrentLocked())
	}()
	replaced = fn()
	return mark, true
}

// undisturbedSince reports whether no publish from an ended epoch has finished
// since mark or is still running. A refresh that read the registry back after
// its own publish evaluates it only then; otherwise the registry may hold the
// data of the mode the monitor left.
func (s mockModeScope) undisturbedSince(mark uint64) bool {
	f := s.fence
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.lateFinishes == mark && !f.olderPublishingLocked()
}

// registryCurrent reports whether the registry's last replacement was an
// undisturbed publish in the current epoch and no ended-epoch publish is still
// running. Until then it may hold the estate of the mode the monitor left.
func (f *mockModeFence) registryCurrent() bool {
	return !f.registryStale.Load()
}

func (f *mockModeFence) registryCurrentLocked() bool {
	return f.cleanEpoch == f.epoch && f.cleanLate == f.lateFinishes && !f.olderPublishingLocked()
}

func (f *mockModeFence) olderPublishingLocked() bool {
	for epoch := range f.publishing {
		if epoch < f.epoch {
			return true
		}
	}
	return false
}
