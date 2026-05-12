package alerts

import (
	"sync"

	"github.com/rs/zerolog/log"
)

type callbackBus struct {
	mu sync.RWMutex

	onAlert        func(alert *Alert)
	alertSubs      map[int]func(alert *Alert)
	onAlertForAI   func(alert *Alert)
	alertForAISubs map[int]func(alert *Alert)
	onResolved     func(alertID string)
	resolvedSubs   map[int]func(alertID string)

	onAcknowledged   func(alert *Alert, user string)
	onUnacknowledged func(alert *Alert, user string)
	onEscalate       func(alert *Alert, level int)

	onFlappingDetected func(alert *Alert, trackingKey string)

	nextCallbackID int
}

func newCallbackBus() callbackBus {
	return callbackBus{
		alertSubs:      make(map[int]func(*Alert)),
		alertForAISubs: make(map[int]func(*Alert)),
		resolvedSubs:   make(map[int]func(string)),
	}
}

func (b *callbackBus) nextIDLocked() int {
	b.nextCallbackID++
	return b.nextCallbackID
}

func (b *callbackBus) ensureAlertSubsLocked() {
	if b.alertSubs == nil {
		b.alertSubs = make(map[int]func(*Alert))
	}
}

func (b *callbackBus) ensureAlertForAISubsLocked() {
	if b.alertForAISubs == nil {
		b.alertForAISubs = make(map[int]func(*Alert))
	}
}

func (b *callbackBus) ensureResolvedSubsLocked() {
	if b.resolvedSubs == nil {
		b.resolvedSubs = make(map[int]func(string))
	}
}

func (b *callbackBus) setAlertCallback(cb func(alert *Alert)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.onAlert = cb
}

func (b *callbackBus) subscribeAlertCallback(cb func(alert *Alert)) func() {
	if cb == nil {
		return func() {}
	}

	b.mu.Lock()
	b.ensureAlertSubsLocked()
	id := b.nextIDLocked()
	b.alertSubs[id] = cb
	b.mu.Unlock()

	return func() {
		b.mu.Lock()
		delete(b.alertSubs, id)
		b.mu.Unlock()
	}
}

func (b *callbackBus) setAlertForAICallback(cb func(alert *Alert)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.onAlertForAI = cb
}

func (b *callbackBus) subscribeAlertForAICallback(cb func(alert *Alert)) func() {
	if cb == nil {
		return func() {}
	}

	b.mu.Lock()
	b.ensureAlertForAISubsLocked()
	id := b.nextIDLocked()
	b.alertForAISubs[id] = cb
	b.mu.Unlock()

	return func() {
		b.mu.Lock()
		delete(b.alertForAISubs, id)
		b.mu.Unlock()
	}
}

func (b *callbackBus) setResolvedCallback(cb func(alertID string)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.onResolved = cb
}

func (b *callbackBus) subscribeResolvedCallback(cb func(alertID string)) func() {
	if cb == nil {
		return func() {}
	}

	b.mu.Lock()
	b.ensureResolvedSubsLocked()
	id := b.nextIDLocked()
	b.resolvedSubs[id] = cb
	b.mu.Unlock()

	return func() {
		b.mu.Lock()
		delete(b.resolvedSubs, id)
		b.mu.Unlock()
	}
}

func (b *callbackBus) setAcknowledgedCallback(cb func(alert *Alert, user string)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.onAcknowledged = cb
}

func (b *callbackBus) setUnacknowledgedCallback(cb func(alert *Alert, user string)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.onUnacknowledged = cb
}

func (b *callbackBus) setEscalateCallback(cb func(alert *Alert, level int)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.onEscalate = cb
}

func (b *callbackBus) setFlappingDetectedCallback(cb func(alert *Alert, trackingKey string)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.onFlappingDetected = cb
}

func (b *callbackBus) alertCallback() func(alert *Alert) {
	b.mu.RLock()
	cb := b.onAlert
	b.mu.RUnlock()
	return cb
}

func (b *callbackBus) alertCallbacks() []func(alert *Alert) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	callbacks := make([]func(alert *Alert), 0, len(b.alertSubs)+1)
	if b.onAlert != nil {
		callbacks = append(callbacks, b.onAlert)
	}
	for _, cb := range b.alertSubs {
		if cb != nil {
			callbacks = append(callbacks, cb)
		}
	}
	return callbacks
}

func (b *callbackBus) alertForAICallback() func(alert *Alert) {
	b.mu.RLock()
	cb := b.onAlertForAI
	b.mu.RUnlock()
	return cb
}

func (b *callbackBus) alertForAICallbacks() []func(alert *Alert) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	callbacks := make([]func(alert *Alert), 0, len(b.alertForAISubs)+1)
	if b.onAlertForAI != nil {
		callbacks = append(callbacks, b.onAlertForAI)
	}
	for _, cb := range b.alertForAISubs {
		if cb != nil {
			callbacks = append(callbacks, cb)
		}
	}
	return callbacks
}

func (b *callbackBus) resolvedCallback() func(alertID string) {
	b.mu.RLock()
	cb := b.onResolved
	b.mu.RUnlock()
	return cb
}

func (b *callbackBus) resolvedCallbacks() []func(alertID string) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	callbacks := make([]func(alertID string), 0, len(b.resolvedSubs)+1)
	if b.onResolved != nil {
		callbacks = append(callbacks, b.onResolved)
	}
	for _, cb := range b.resolvedSubs {
		if cb != nil {
			callbacks = append(callbacks, cb)
		}
	}
	return callbacks
}

func (b *callbackBus) acknowledgedCallback() func(alert *Alert, user string) {
	b.mu.RLock()
	cb := b.onAcknowledged
	b.mu.RUnlock()
	return cb
}

func (b *callbackBus) unacknowledgedCallback() func(alert *Alert, user string) {
	b.mu.RLock()
	cb := b.onUnacknowledged
	b.mu.RUnlock()
	return cb
}

func (b *callbackBus) escalateCallback() func(alert *Alert, level int) {
	b.mu.RLock()
	cb := b.onEscalate
	b.mu.RUnlock()
	return cb
}

func (b *callbackBus) flappingDetectedCallback() func(alert *Alert, trackingKey string) {
	b.mu.RLock()
	cb := b.onFlappingDetected
	b.mu.RUnlock()
	return cb
}

// SetAlertCallback sets the callback for new alerts.
func (m *Manager) SetAlertCallback(cb func(alert *Alert)) {
	m.callbacks.setAlertCallback(cb)
}

// SubscribeAlertCallback registers an additional alert callback without
// replacing the legacy single callback slot. The returned function removes the
// subscription when called.
func (m *Manager) SubscribeAlertCallback(cb func(alert *Alert)) func() {
	return m.callbacks.subscribeAlertCallback(cb)
}

// SetAlertForAICallback sets a callback for AI analysis when alerts are created.
// Unlike SetAlertCallback, this callback is invoked unconditionally - it bypasses
// activation state, quiet hours, and other notification suppression checks.
// This allows AI to analyze alerts even when the user hasn't finished setup.
func (m *Manager) SetAlertForAICallback(cb func(alert *Alert)) {
	m.callbacks.setAlertForAICallback(cb)
	log.Info().Msg("alert-for-AI callback registered (bypasses notification suppression)")
}

// SubscribeAlertForAICallback registers an additional AI alert callback without
// replacing the legacy single callback slot. The returned function removes the
// subscription when called.
func (m *Manager) SubscribeAlertForAICallback(cb func(alert *Alert)) func() {
	return m.callbacks.subscribeAlertForAICallback(cb)
}

// SetResolvedCallback sets the callback for resolved alerts.
func (m *Manager) SetResolvedCallback(cb func(alertID string)) {
	m.callbacks.setResolvedCallback(cb)
}

// SubscribeResolvedCallback registers an additional resolved-alert callback
// without replacing the legacy single callback slot. The returned function
// removes the subscription when called.
func (m *Manager) SubscribeResolvedCallback(cb func(alertID string)) func() {
	return m.callbacks.subscribeResolvedCallback(cb)
}

// SetAcknowledgedCallback sets the callback for acknowledged alerts.
func (m *Manager) SetAcknowledgedCallback(cb func(alert *Alert, user string)) {
	m.callbacks.setAcknowledgedCallback(cb)
}

// SetUnacknowledgedCallback sets the callback for unacknowledged alerts.
func (m *Manager) SetUnacknowledgedCallback(cb func(alert *Alert, user string)) {
	m.callbacks.setUnacknowledgedCallback(cb)
}

// SetEscalateCallback sets the callback for escalated alerts.
func (m *Manager) SetEscalateCallback(cb func(alert *Alert, level int)) {
	m.callbacks.setEscalateCallback(cb)
}

// SetFlappingDetectedCallback registers a callback fired exactly once on the
// transition into flapping suppression for a given trackingKey. The callback
// is invoked from a goroutine -- the alerts manager lock is NOT held when it
// runs -- so the callback is free to take its own locks or schedule a patrol.
// It will NOT fire again for the same trackingKey while the flapping cooldown
// window is active; subsequent suppressed dispatches are silent.
func (m *Manager) SetFlappingDetectedCallback(cb func(alert *Alert, trackingKey string)) {
	m.callbacks.setFlappingDetectedCallback(cb)
}

func (m *Manager) getAlertCallback() func(alert *Alert) {
	return m.callbacks.alertCallback()
}

func (m *Manager) getAlertCallbacks() []func(alert *Alert) {
	return m.callbacks.alertCallbacks()
}

func (m *Manager) getAlertForAICallback() func(alert *Alert) {
	return m.callbacks.alertForAICallback()
}

func (m *Manager) getAlertForAICallbacks() []func(alert *Alert) {
	return m.callbacks.alertForAICallbacks()
}

func (m *Manager) getResolvedCallback() func(alertID string) {
	return m.callbacks.resolvedCallback()
}

func (m *Manager) getResolvedCallbacks() []func(alertID string) {
	return m.callbacks.resolvedCallbacks()
}

func (m *Manager) getAcknowledgedCallback() func(alert *Alert, user string) {
	return m.callbacks.acknowledgedCallback()
}

func (m *Manager) getUnacknowledgedCallback() func(alert *Alert, user string) {
	return m.callbacks.unacknowledgedCallback()
}

func (m *Manager) getEscalateCallback() func(alert *Alert, level int) {
	return m.callbacks.escalateCallback()
}

// safeCallResolvedAlertCallback invokes onResolved with panic recovery while
// preserving canonical state as the internal identity and emitting the public
// alert ID to external callbacks for compatibility.
func (m *Manager) safeCallResolvedAlertCallback(alert *Alert, fallbackID string, async bool) {
	callbacks := m.getResolvedCallbacks()
	if len(callbacks) == 0 {
		return
	}

	publicID := exportedAlertID(alert, fallbackID)
	trackingKey := canonicalTrackingKeyForAlert(alert)

	callbackFunc := func() {
		defer func() {
			if r := recover(); r != nil {
				log.Error().
					Interface("panic", r).
					Str("alertID", publicID).
					Str("trackingKey", trackingKey).
					Msg("Panic in onResolved callback")
			}
		}()
		for _, callback := range callbacks {
			callback(publicID)
		}
	}

	if async {
		go callbackFunc()
	} else {
		callbackFunc()
	}
}

// safeCallAcknowledgedCallback invokes onAcknowledged with panic recovery and alert cloning.
func (m *Manager) safeCallAcknowledgedCallback(alert *Alert, user string) {
	callback := m.getAcknowledgedCallback()
	if callback == nil || alert == nil {
		return
	}

	alertCopy := cloneAlertForOutput(alert)
	go func(a *Alert, u string) {
		defer func() {
			if r := recover(); r != nil {
				log.Error().
					Interface("panic", r).
					Str("alertID", a.ID).
					Msg("Panic in onAcknowledged callback")
			}
		}()
		callback(a, u)
	}(alertCopy, user)
}

// safeCallUnacknowledgedCallback invokes onUnacknowledged with panic recovery and alert cloning.
func (m *Manager) safeCallUnacknowledgedCallback(alert *Alert, user string) {
	callback := m.getUnacknowledgedCallback()
	if callback == nil || alert == nil {
		return
	}

	alertCopy := cloneAlertForOutput(alert)
	go func(a *Alert, u string) {
		defer func() {
			if r := recover(); r != nil {
				log.Error().
					Interface("panic", r).
					Str("alertID", a.ID).
					Msg("Panic in onUnacknowledged callback")
			}
		}()
		callback(a, u)
	}(alertCopy, user)
}

// safeCallEscalateCallback invokes onEscalate with panic recovery and alert cloning.
func (m *Manager) safeCallEscalateCallback(alert *Alert, level int) {
	callback := m.getEscalateCallback()
	if callback == nil {
		return
	}

	alertCopy := cloneAlertForOutput(alert)
	go func(a *Alert, lvl int) {
		defer func() {
			if r := recover(); r != nil {
				log.Error().
					Interface("panic", r).
					Str("alertID", a.ID).
					Int("level", lvl).
					Msg("Panic in onEscalate callback")
			}
		}()
		callback(a, lvl)
	}(alertCopy, level)
}
