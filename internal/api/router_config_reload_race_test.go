package api

import (
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// The supported reload refreshes the canonical config before starting the new
// monitor, then calls SetConfig with that same pointer. Its live reader must
// not race a redundant self-copy even though it does not take config.Mu.
func TestRouterSetConfigCanonicalPointerDoesNotRewriteLiveConfig(t *testing.T) {
	cfg := &config.Config{DataPath: "saved", PVEPollingInterval: 10 * time.Second, PublicURL: "https://pulse.invalid"}
	settings := &SystemSettingsHandler{config: cfg}
	router := &Router{config: cfg, systemSettingsHandler: settings}
	var group sync.WaitGroup
	start := make(chan struct{})
	group.Go(func() {
		<-start
		for range 10000 {
			if cfg.DataPath != "saved" || cfg.PVEPollingInterval != 10*time.Second || cfg.PublicURL != "https://pulse.invalid" {
				t.Error("canonical config changed during same-pointer refresh")
				return
			}
		}
	})
	close(start)
	for range 10000 {
		router.SetConfig(cfg)
	}
	group.Wait()
	if router.config != cfg || settings.config != cfg {
		t.Fatal("refresh lost canonical handler ownership")
	}
	// A genuinely different loaded object still refreshes the existing owner,
	// preserving the pointer held by authentication and dependent handlers.
	replacement := &config.Config{DataPath: "saved", PublicURL: "https://updated.invalid"}
	router.SetConfig(replacement)
	if router.config != cfg || settings.config != cfg || cfg.PublicURL != replacement.PublicURL {
		t.Fatal("distinct config failed to refresh the canonical owner")
	}
	router.SetConfig(nil)
	if cfg.PublicURL != replacement.PublicURL {
		t.Fatal("nil config changed the existing owner")
	}
}
