package updates

import (
	"sync"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// The background update checker outlives a monitor reload. The config writer's
// mutex, not the update manager's status mutex, owns this shared field.
func TestResolveChannelUsesReloadConfigBoundary(t *testing.T) {
	cfg := &config.Config{UpdateChannel: "stable"}
	manager := NewManager(cfg)
	start := make(chan struct{})
	var group sync.WaitGroup
	group.Go(func() {
		<-start
		for i := range 10000 {
			channel := "stable"
			if i%2 != 0 {
				channel = "rc"
			}
			config.Mu.Lock()
			*cfg = config.Config{UpdateChannel: channel}
			config.Mu.Unlock()
		}
	})
	group.Go(func() {
		<-start
		for range 10000 {
			if got := manager.resolveChannel("", nil); got != "stable" && got != "rc" {
				t.Errorf("incoherent saved channel %q", got)
				return
			}
			if got := manager.resolveChannel("rc", nil); got != "rc" {
				t.Errorf("explicit channel lost precedence: %q", got)
				return
			}
		}
	})
	close(start)
	group.Wait()
}
