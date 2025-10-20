package websocket

import (
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestHubConcurrentClients(t *testing.T) {
	hub := NewHub(nil)

	// Start hub processing loop in background
	go hub.Run()

	const iterations = 200

	var wg sync.WaitGroup
	wg.Add(3)

	// Simulate register/unregister from multiple goroutines
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			client := &Client{
				hub:  hub,
				send: make(chan []byte, 10),
				id:   "client-register-" + strconv.Itoa(i),
			}
			hub.register <- client
			time.Sleep(time.Microsecond)
			hub.unregister <- client
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			hub.BroadcastMessage(Message{Type: "test", Data: map[string]int{"iteration": i}})
			time.Sleep(time.Microsecond)
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			hub.SetAllowedOrigins([]string{"http://localhost", "http://example.com"})
			time.Sleep(time.Microsecond)
		}
	}()

	wg.Wait()

	// Allow hub to process remaining messages
	time.Sleep(10 * time.Millisecond)
}
