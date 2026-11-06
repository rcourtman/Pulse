package alerts

// beginCallback takes ownership before a callback can be scheduled. Stop closes
// this admission after the manager's workers have ended, then drains every
// accepted callback before saving state and closing history. Callbacks may
// re-enter the manager, but must not call Stop on their own manager.
func (m *Manager) beginCallback() bool {
	m.stopMu.Lock()
	defer m.stopMu.Unlock()
	if m.callbacksClosed {
		return false
	}
	m.callbackWG.Add(1)
	return true
}

// runAdmittedCallback preserves the caller's synchronous/asynchronous contract.
// The caller must have acquired exactly one callback admission first.
func (m *Manager) runAdmittedCallback(async bool, callback func()) {
	run := func() {
		defer m.callbackWG.Done()
		callback()
	}
	if async {
		go run()
	} else {
		run()
	}
}

func (m *Manager) dispatchCallback(async bool, callback func()) bool {
	if !m.beginCallback() {
		return false
	}
	m.runAdmittedCallback(async, callback)
	return true
}
