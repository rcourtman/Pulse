package alerts

import (
	"time"

	"github.com/rs/zerolog/log"
)

// Stop stops the alert manager and saves history
func (m *Manager) Stop() {
	m.stopOnce.Do(func() {
		closeSignalChannel(m.escalationStop)
		closeSignalChannel(m.cleanupStop)
		if m.historyManager != nil {
			m.historyManager.Stop()
		}

		// Give background goroutines time to exit cleanly
		time.Sleep(100 * time.Millisecond)

		// Save active alerts before stopping
		if err := m.SaveActiveAlerts(); err != nil {
			log.Error().Err(err).Msg("Failed to save active alerts on stop")
		}
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
