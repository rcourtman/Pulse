package ai

// NOTE: These helpers are intended for tests and local debugging tools.
// They allow exercising the SSE resume/replay logic without running a full patrol.

// DebugResetStreamForRun resets streaming state and buffer for a synthetic run.
func (p *PatrolService) DebugResetStreamForRun(runID string) {
	p.resetStreamForRun(runID)
}

// DebugSetStreamPhase sets the current stream phase (broadcasting a phase event).
func (p *PatrolService) DebugSetStreamPhase(phase string) {
	p.setStreamPhase(phase)
}

// DebugAppendStreamContent appends content to the current output and broadcasts it as a content event.
func (p *PatrolService) DebugAppendStreamContent(content string) {
	p.appendStreamContent(content)
}

// DebugBroadcastStreamEvent broadcasts an event to all subscribers. The event will be decorated
// and truncated using the same rules as real patrol events.
func (p *PatrolService) DebugBroadcastStreamEvent(event PatrolStreamEvent) {
	p.broadcast(event)
}
