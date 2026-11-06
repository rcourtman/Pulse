package alerts

import (
	"github.com/rs/zerolog/log"
)

// Stop stops the alert manager and saves history
func (m *Manager) Stop() {
	m.stopOnce.Do(func() {
		m.stopMu.Lock()
		m.stopping = true
		closeSignalChannel(m.escalationStop)
		closeSignalChannel(m.cleanupStop)
		m.stopMu.Unlock()

		m.workerWG.Wait()

		// A goroutine being scheduled is not notification admission. Keep the
		// downstream notification manager and this manager's stores alive until
		// every callback accepted by the producer has actually returned.
		m.stopMu.Lock()
		m.callbacksClosed = true
		m.stopMu.Unlock()
		m.callbackWG.Wait()

		if m.historyManager != nil {
			m.historyManager.Stop()
		}

		// Save active alerts before stopping
		if err := m.SaveActiveAlerts(); err != nil {
			log.Error().Err(err).Msg("Failed to save active alerts on stop")
		}

		// Drain and close the event log last so shutdown-time transitions
		// are still recorded.
		m.SetEventLog(nil)
	})
}

func closeSignalChannel(ch chan struct{}) {
	if ch == nil {
		return
	}
	defer func() {
		if recover() != nil {
			// Channel was already closed by another shutdown path.
		}
	}()
	close(ch)
}
