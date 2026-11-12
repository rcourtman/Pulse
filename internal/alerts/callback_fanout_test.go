package alerts

import (
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func fanoutTestManager(t *testing.T) *Manager {
	t.Helper()
	m := NewManagerWithDataDir(t.TempDir())
	t.Cleanup(m.Stop)
	cfg := m.GetConfig()
	cfg.ActivationState = ActivationActive
	cfg.FlappingEnabled = false
	m.UpdateConfig(cfg)
	return m
}

func fanoutTestAlert() *Alert {
	return &Alert{
		ID: "fanout-cpu", Type: "cpu", Level: AlertLevelWarning,
		ResourceID: "agent:fanout", ResourceName: "original node",
		StartTime: time.Now().Add(-time.Minute), LastSeen: time.Now(),
		Metadata: map[string]interface{}{
			"labels": []string{"original"},
			"nested": map[string]interface{}{"value": "original"},
		},
		EscalationTimes: []time.Time{time.Unix(100, 0)},
	}
}

func mutateFanoutSnapshot(alert *Alert) {
	alert.ID = "subscriber-owned"
	alert.ResourceName = "subscriber-owned"
	alert.Metadata["labels"].([]string)[0] = "subscriber-owned"
	alert.Metadata["nested"].(map[string]interface{})["value"] = "subscriber-owned"
	alert.EscalationTimes[0] = time.Unix(200, 0)
	if alert.LastNotified != nil {
		*alert.LastNotified = time.Unix(300, 0)
	}
}

func TestAlertDeliverySubscriberPanicDoesNotDropPeers(t *testing.T) {
	for _, async := range []bool{false, true} {
		t.Run(map[bool]string{false: "sync", true: "async"}[async], func(t *testing.T) {
			m := fanoutTestManager(t)
			var legacy, first, second atomic.Int32
			// The legacy slot always runs before the subscriptions, independent
			// of the map iteration order of the additional consumers.
			m.SetAlertCallback(func(*Alert) { legacy.Add(1); panic("synthetic subscriber panic") })
			m.SubscribeAlertCallback(func(*Alert) { first.Add(1) })
			m.SubscribeAlertCallback(func(*Alert) { second.Add(1) })
			alert := fanoutTestAlert()
			m.mu.Lock()
			accepted := m.dispatchAlert(alert, async)
			m.mu.Unlock()
			m.Stop() // Drains the whole accepted fan-out, not just its first callback.
			if !accepted || alert.LastNotified == nil {
				t.Fatal("admitted delivery was not recorded")
			}
			if legacy.Load() != 1 || first.Load() != 1 || second.Load() != 1 {
				t.Fatalf("callbacks legacy/first/second = %d/%d/%d; one subscriber must not drop its peers", legacy.Load(), first.Load(), second.Load())
			}
		})
	}
}

func TestAlertSubscribersOwnPrivateSnapshots(t *testing.T) {
	for _, route := range []string{"delivery", "firing-only", "firing-delivery-inactive", "firing-delivery-active"} {
		for _, async := range []bool{false, true} {
			t.Run(route+map[bool]string{false: "/sync", true: "/async"}[async], func(t *testing.T) {
				m := fanoutTestManager(t)
				if route == "firing-delivery-inactive" {
					cfg := m.GetConfig()
					cfg.ActivationState = ActivationPending
					m.UpdateConfig(cfg)
				}
				observed := make(chan *Alert, 3)
				retained := make(chan *Alert, 3)
				callback := func(alert *Alert) {
					observed <- alert.Clone()
					retained <- alert
					mutateFanoutSnapshot(alert)
				}
				if route == "delivery" {
					m.SetAlertCallback(callback)
					m.SubscribeAlertCallback(callback)
					m.SubscribeAlertCallback(callback)
				} else {
					for range 3 {
						m.SubscribeAlertFiringCallback(callback)
					}
					if route != "firing-only" {
						m.SetAlertCallback(func(*Alert) {})
					}
				}
				alert := fanoutTestAlert()
				want := cloneAlertForOutput(alert)
				m.mu.Lock()
				dispatched := m.dispatchAlert(alert, async)
				m.mu.Unlock()
				m.Stop()
				wantDelivery := route == "delivery" || route == "firing-delivery-active"
				if dispatched != wantDelivery {
					t.Fatalf("delivery dispatch = %t, want %t", dispatched, wantDelivery)
				}
				if route == "delivery" {
					want.LastNotified = alert.LastNotified
				}
				if len(observed) != 3 || len(retained) != 3 {
					t.Fatalf("observed/retained callbacks = %d/%d, want 3/3", len(observed), len(retained))
				}
				seen := make(map[*Alert]bool)
				for range 3 {
					got, owned := <-observed, <-retained
					if !reflect.DeepEqual(got, want) {
						t.Errorf("subscriber saw a peer's edits: got %+v, want %+v", got, want)
					}
					if seen[owned] || owned == alert {
						t.Error("subscribers or live state share an alert pointer")
					}
					seen[owned] = true
				}
				live := cloneAlertForOutput(alert)
				if route == "firing-delivery-active" {
					want.LastNotified = alert.LastNotified
				}
				if !reflect.DeepEqual(live, want) {
					t.Error("subscriber edits reached live alert state")
				}
			})
		}
	}
}

// A subscriber may retain its snapshot after it returns. Another subscriber
// encoding or inspecting its own snapshot must not race with those edits.
// Run with -race.
func TestAlertSubscriberRetainedSnapshotsAreConcurrent(t *testing.T) {
	for _, route := range []string{"delivery", "firing"} {
		t.Run(route, func(t *testing.T) {
			m := fanoutTestManager(t)
			retained := make(chan *Alert, 2)
			callback := func(alert *Alert) { retained <- alert }
			if route == "delivery" {
				m.SetAlertCallback(callback)
				m.SubscribeAlertCallback(callback)
			} else {
				m.SubscribeAlertFiringCallback(callback)
				m.SubscribeAlertFiringCallback(callback)
			}
			m.mu.Lock()
			m.dispatchAlert(fanoutTestAlert(), true)
			m.mu.Unlock()
			m.Stop()
			if len(retained) != 2 {
				t.Fatalf("retained snapshots = %d, want 2", len(retained))
			}
			left, right := <-retained, <-retained
			var wg sync.WaitGroup
			wg.Add(2)
			for _, alert := range []*Alert{left, right} {
				go func() {
					defer wg.Done()
					for range 100 {
						mutateFanoutSnapshot(alert)
						_ = alert.Clone()
					}
				}()
			}
			wg.Wait()
		})
	}
}

func TestAlertSubscriberStopDrainsPastFault(t *testing.T) {
	m := fanoutTestManager(t)
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	t.Cleanup(unblock)
	m.SetAlertCallback(func(*Alert) { panic("synthetic first-consumer panic") })
	m.SubscribeAlertCallback(func(*Alert) {
		close(entered)
		<-release
		_ = m.GetActiveAlerts() // The async subscriber can still re-enter.
	})
	alert := fanoutTestAlert()
	m.mu.Lock()
	m.dispatchAlert(alert, true)
	m.mu.Unlock()
	awaitProducerSignal(t, "healthy subscriber after panic", entered)
	stopped := make(chan struct{})
	go func() { m.Stop(); close(stopped) }()
	awaitCallbackClosure(t, m)
	select {
	case <-stopped:
		t.Fatal("Stop returned while the healthy peer still owned accepted work")
	default:
	}
	unblock()
	awaitProducerSignal(t, "drained peer and Stop", stopped)
	m.mu.Lock()
	late := fanoutTestAlert()
	accepted := m.dispatchAlert(late, true)
	m.mu.Unlock()
	if accepted || late.LastNotified != nil {
		t.Fatal("closed producer admitted a late fan-out")
	}
}
