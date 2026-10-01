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
