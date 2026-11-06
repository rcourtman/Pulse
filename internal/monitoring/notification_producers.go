package monitoring

import "sync"

// notificationProducers owns the handoff from monitor callbacks to the
// notification manager. Taking the lease before launching the goroutine keeps a
// scheduled SendAlert/SendResolvedAlert from arriving after notification Stop.
// The zero value is ready for use, including monitors stopped before Start.
type notificationProducers struct {
	mu     sync.Mutex
	closed bool
	wg     sync.WaitGroup
}

func (p *notificationProducers) start(send func()) bool {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return false
	}
	p.wg.Add(1)
	p.mu.Unlock()
	go func() {
		defer p.wg.Done()
		send()
	}()
	return true
}

func (p *notificationProducers) stop() {
	p.mu.Lock()
	p.closed = true
	p.mu.Unlock()
	p.wg.Wait()
}
