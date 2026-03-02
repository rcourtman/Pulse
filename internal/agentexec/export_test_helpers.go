package agentexec

import "github.com/gorilla/websocket"

// TestRegisterAgent registers a fake agent for testing purposes.
// This bypasses the WebSocket handshake and directly inserts into the agents map.
// It also installs a no-op writeTextMessage so that send operations don't panic
// on the nil websocket connection. The original writer is restored via the
// returned cleanup function (typically deferred by the caller).
//
// NOTE: This mutates the package-global writeTextMessage without synchronization.
// Callers must NOT use t.Parallel() when using this helper.
func (s *Server) TestRegisterAgent(agentID, hostname string) (cleanup func()) {
	// Save and override the global writer.
	origWriter := writeTextMessage
	writeTextMessage = func(_ *websocket.Conn, _ []byte) error {
		return nil
	}

	s.mu.Lock()
	s.agents[agentID] = &agentConn{
		agent: ConnectedAgent{
			AgentID:  agentID,
			Hostname: hostname,
		},
		done: make(chan struct{}),
	}
	s.mu.Unlock()

	return func() {
		writeTextMessage = origWriter
		s.mu.Lock()
		delete(s.agents, agentID)
		s.mu.Unlock()
	}
}
