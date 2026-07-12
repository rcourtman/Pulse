package chat

import (
	"errors"
	"fmt"
	"strings"
)

// maxPendingSteersPerSession bounds the steering inbox so a chatty client
// cannot grow the running turn's prompt (and the service's memory) without
// limit; excess steers stay on the client's ordinary follow-up queue.
const maxPendingSteersPerSession = 8

// errSteerBacklogFull reports a full steering inbox; the service maps it to
// a normal accepted=false outcome rather than an error status.
var errSteerBacklogFull = errors.New("steer backlog full")

// pendingSteer pairs the steering message with the client-side transcript
// row id so the steer_applied event can reconcile the originating drawer.
type pendingSteer struct {
	message         Message
	clientMessageID string
}

// Steer queues a user message for injection into the running loop at the
// next turn boundary (the same checkpoint that observes aborts). Delivery is
// not guaranteed: if the run finishes before a boundary arrives, unconsumed
// steers are discarded and the client re-sends through the normal queue
// drain, so the message is never persisted twice.
func (a *AgenticLoop) Steer(sessionID string, msg Message, clientMessageID string) error {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return fmt.Errorf("steer requires a session id")
	}
	if strings.TrimSpace(msg.Content) == "" {
		return fmt.Errorf("steer requires a non-empty prompt")
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	if a.pendingSteers == nil {
		a.pendingSteers = make(map[string][]pendingSteer)
	}
	if len(a.pendingSteers[sessionID]) >= maxPendingSteersPerSession {
		return errSteerBacklogFull
	}
	a.pendingSteers[sessionID] = append(a.pendingSteers[sessionID], pendingSteer{
		message:         msg,
		clientMessageID: strings.TrimSpace(clientMessageID),
	})
	return nil
}

// takePendingSteers drains and returns the steering messages queued for a
// session, in arrival order.
func (a *AgenticLoop) takePendingSteers(sessionID string) []pendingSteer {
	a.mu.Lock()
	defer a.mu.Unlock()
	steers := a.pendingSteers[sessionID]
	if len(steers) == 0 {
		return nil
	}
	delete(a.pendingSteers, sessionID)
	return steers
}

// discardPendingSteers drops any unconsumed steering messages when a run
// ends. The frontend keeps the row queued until steer_applied confirms
// delivery, so an undelivered steer drains as a normal follow-up turn.
func (a *AgenticLoop) discardPendingSteers(sessionID string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.pendingSteers, sessionID)
}
