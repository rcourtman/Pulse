package websocket

import "fmt"

// NewBroadcastProjectionProbeForTest exercises the production getter, snapshot,
// per-client delta and queue path without network/compression or timer sleeps.
// It is exported only in the test binary so the fixture can use a real Monitor
// without introducing a monitoring -> websocket import cycle.
func NewBroadcastProjectionProbeForTest(getState func(string) interface{}, recipients int) (func() (int, error), error) {
	hub := NewHub(getState)
	clients := make([]*Client, recipients)
	for i := range clients {
		client := &Client{hub: hub, orgID: "default", send: make(chan []byte, 1)}
		if _, sent, err := client.queueFullState("initialState", getState("default")); err != nil || !sent {
			return nil, fmt.Errorf("initial state: sent=%v error=%v", sent, err)
		}
		<-client.send
		clients[i] = client
		hub.clients[client] = true
	}
	return func() (int, error) {
		hub.dispatchStateBroadcast(&Message{Type: "rawData", Data: stateBroadcastRequest{}}, "")
		bytes := 0
		for _, client := range clients {
			select {
			case data := <-client.send:
				bytes += len(data)
			default:
				return 0, fmt.Errorf("broadcast did not queue fresh state")
			}
		}
		return bytes, nil
	}, nil
}

// NewBroadcastProjectionCaptureForTest retains the actual per-client frames,
// including a nil frame for a quiet or unrelated tenant. It runs the production
// current-state getter, projection, delta baseline and queue path without timers
// or a network socket, and is available only in the test binary.
func NewBroadcastProjectionCaptureForTest(getState func(string) interface{}, orgIDs []string) (func(string) ([][]byte, error), error) {
	hub := NewHub(getState)
	clients := make([]*Client, len(orgIDs))
	for i, orgID := range orgIDs {
		client := &Client{hub: hub, orgID: orgID, send: make(chan []byte, 1)}
		if _, sent, err := client.queueFullState("initialState", getState(orgID)); err != nil || !sent {
			return nil, fmt.Errorf("initial state: sent=%v error=%v", sent, err)
		}
		<-client.send
		clients[i] = client
		hub.clients[client] = true
		if hub.clientsByTenant[orgID] == nil {
			hub.clientsByTenant[orgID] = make(map[*Client]bool)
		}
		hub.clientsByTenant[orgID][client] = true
	}
	return func(orgID string) ([][]byte, error) {
		hub.dispatchStateBroadcast(&Message{Type: "rawData", Data: stateBroadcastRequest{}}, orgID)
		frames := make([][]byte, len(clients))
		for i, client := range clients {
			if !hub.clients[client] {
				return nil, fmt.Errorf("broadcast removed client %d", i)
			}
			select {
			case frames[i] = <-client.send:
			default:
			}
		}
		return frames, nil
	}, nil
}
