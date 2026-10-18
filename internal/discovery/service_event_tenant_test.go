package discovery

import (
	"reflect"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/websocket"
	pkgdiscovery "github.com/rcourtman/pulse-go-rewrite/pkg/discovery"
)

type recordedDiscoveryEvents struct {
	orgIDs []string
	types  []string
}

func (r *recordedDiscoveryEvents) BroadcastToTenant(orgID string, data interface{}) {
	msg := data.(websocket.Message)
	r.orgIDs = append(r.orgIDs, orgID)
	r.types = append(r.types, msg.Type)
}

func TestDiscoveryCallbacksKeepOwningOrganization(t *testing.T) {
	for _, orgID := range []string{"org-a", "default", ""} {
		t.Run("org="+orgID, func(t *testing.T) {
			events := &recordedDiscoveryEvents{}
			service := NewService(nil, orgID, time.Minute, "192.0.2.0/24", nil)
			service.wsHub = events
			service.scannerFactory = func(config.DiscoveryConfig) (discoveryScanner, error) {
				return &fakeScanner{result: &pkgdiscovery.DiscoveryResult{
					Servers: []pkgdiscovery.DiscoveredServer{{IP: "192.0.2.10", Port: 8006, Type: "pve"}},
				}}, nil
			}
			service.performScan()
			want := []string{"discovery_started", "discovery_server_found", "discovery_progress", "discovery_update", "discovery_complete"}
			if !reflect.DeepEqual(events.types, want) {
				t.Fatalf("callback events = %v, want %v", events.types, want)
			}
			for i, got := range events.orgIDs {
				if got != orgID {
					t.Errorf("%s targets org %q, want %q", events.types[i], got, orgID)
				}
			}
		})
	}
}
