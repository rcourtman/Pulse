package monitoring

import (
	"context"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/websocket"
)

func TestReloadableMonitorLifecycle(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())
	mock.SetEnabled(true)
	defer mock.SetEnabled(false)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	hub := websocket.NewHub(nil)
	rm, err := NewReloadableMonitor(cfg, hub)
	if err != nil {
		t.Fatalf("new reloadable monitor: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rm.Start(ctx)

	if err := rm.Reload(); err != nil {
		t.Fatalf("reload error: %v", err)
	}

	if rm.GetMonitor() == nil {
		t.Fatal("expected monitor instance")
	}
	if rm.GetConfig() == nil {
		t.Fatal("expected config instance")
	}

	rm.Stop()

	select {
	case <-time.After(10 * time.Millisecond):
		// Allow any goroutines to observe cancel without blocking test.
	}
}

func TestReloadableMonitorGetConfigNil(t *testing.T) {
	rm := &ReloadableMonitor{}
	if rm.GetConfig() != nil {
		t.Fatal("expected nil config")
	}
}
