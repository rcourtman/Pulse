package configapi

import "context"

type nodeEditGate struct {
	busy chan struct{}
	refs int // Includes both the holder and any waiters.
}

// acquireNodeEdit orders explicit connection edits and cluster refresh saves
// for the same configuration directory. Keep the turn through reload: an
// older apply can otherwise publish the snapshot it read before a newer save.
// This does not lock persistence reads, monitoring, or upstream discovery.
func (h *ConfigHandlers) acquireNodeEdit(ctx context.Context, directory string) (release func(), err error) {
	h.nodeEditMu.Lock()
	if h.nodeEdits == nil {
		h.nodeEdits = make(map[string]*nodeEditGate)
	}
	gate := h.nodeEdits[directory]
	if gate == nil {
		gate = &nodeEditGate{busy: make(chan struct{}, 1)}
		h.nodeEdits[directory] = gate
	}
	gate.refs++
	h.nodeEditMu.Unlock()

	unref := func() {
		h.nodeEditMu.Lock()
		gate.refs--
		if gate.refs == 0 {
			delete(h.nodeEdits, directory)
		}
		h.nodeEditMu.Unlock()
	}
	select {
	case gate.busy <- struct{}{}:
		return func() {
			<-gate.busy
			unref()
		}, nil
	case <-ctx.Done():
		unref()
		return nil, ctx.Err()
	}
}
