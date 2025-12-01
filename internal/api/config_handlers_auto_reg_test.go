package api

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestIsRecentlyAutoRegistered(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		nodeType  string
		nodeName  string
		setup     func(*ConfigHandlers)
		want      bool
		checkGone bool // verify entry was cleaned up
	}{
		{
			name:     "empty nodeType returns false",
			nodeType: "",
			nodeName: "node1",
			setup:    nil,
			want:     false,
		},
		{
			name:     "empty nodeName returns false",
			nodeType: "pve",
			nodeName: "",
			setup:    nil,
			want:     false,
		},
		{
			name:     "key not in map returns false",
			nodeType: "pve",
			nodeName: "nonexistent",
			setup:    nil,
			want:     false,
		},
		{
			name:     "recent registration returns true",
			nodeType: "pve",
			nodeName: "node1",
			setup: func(h *ConfigHandlers) {
				h.recentAutoRegistered["pve:node1"] = time.Now()
			},
			want: true,
		},
		{
			name:     "expired registration returns false and cleans up",
			nodeType: "pve",
			nodeName: "node1",
			setup: func(h *ConfigHandlers) {
				h.recentAutoRegistered["pve:node1"] = time.Now().Add(-3 * time.Minute)
			},
			want:      false,
			checkGone: true,
		},
		{
			name:     "different keys are independent",
			nodeType: "pbs",
			nodeName: "node2",
			setup: func(h *ConfigHandlers) {
				h.recentAutoRegistered["pve:node1"] = time.Now()
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{
				DataPath:   t.TempDir(),
				ConfigPath: t.TempDir(),
			}
			h := newTestConfigHandlers(t, cfg)

			if tt.setup != nil {
				tt.setup(h)
			}

			got := h.isRecentlyAutoRegistered(tt.nodeType, tt.nodeName)
			if got != tt.want {
				t.Errorf("isRecentlyAutoRegistered(%q, %q) = %v, want %v", tt.nodeType, tt.nodeName, got, tt.want)
			}

			if tt.checkGone {
				key := tt.nodeType + ":" + tt.nodeName
				h.recentAutoRegMutex.Lock()
				_, exists := h.recentAutoRegistered[key]
				h.recentAutoRegMutex.Unlock()
				if exists {
					t.Errorf("expected key %q to be cleaned up, but it still exists", key)
				}
			}
		})
	}
}

func TestClearAutoRegistered(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		nodeType string
		nodeName string
		setup    func(*ConfigHandlers)
		check    func(*testing.T, *ConfigHandlers)
	}{
		{
			name:     "empty nodeType is safe",
			nodeType: "",
			nodeName: "node1",
			setup: func(h *ConfigHandlers) {
				h.recentAutoRegistered["pve:node1"] = time.Now()
			},
			check: func(t *testing.T, h *ConfigHandlers) {
				h.recentAutoRegMutex.Lock()
				defer h.recentAutoRegMutex.Unlock()
				if _, exists := h.recentAutoRegistered["pve:node1"]; !exists {
					t.Error("expected pve:node1 to still exist")
				}
			},
		},
		{
			name:     "empty nodeName is safe",
			nodeType: "pve",
			nodeName: "",
			setup: func(h *ConfigHandlers) {
				h.recentAutoRegistered["pve:node1"] = time.Now()
			},
			check: func(t *testing.T, h *ConfigHandlers) {
				h.recentAutoRegMutex.Lock()
				defer h.recentAutoRegMutex.Unlock()
				if _, exists := h.recentAutoRegistered["pve:node1"]; !exists {
					t.Error("expected pve:node1 to still exist")
				}
			},
		},
		{
			name:     "successfully clears existing entry",
			nodeType: "pve",
			nodeName: "node1",
			setup: func(h *ConfigHandlers) {
				h.recentAutoRegistered["pve:node1"] = time.Now()
			},
			check: func(t *testing.T, h *ConfigHandlers) {
				h.recentAutoRegMutex.Lock()
				defer h.recentAutoRegMutex.Unlock()
				if _, exists := h.recentAutoRegistered["pve:node1"]; exists {
					t.Error("expected pve:node1 to be cleared")
				}
			},
		},
		{
			name:     "non-existent key does not error",
			nodeType: "pve",
			nodeName: "nonexistent",
			setup:    nil,
			check: func(t *testing.T, h *ConfigHandlers) {
				// Just verify no panic occurred and map is still valid
				h.recentAutoRegMutex.Lock()
				defer h.recentAutoRegMutex.Unlock()
				if h.recentAutoRegistered == nil {
					t.Error("map should not be nil")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{
				DataPath:   t.TempDir(),
				ConfigPath: t.TempDir(),
			}
			h := newTestConfigHandlers(t, cfg)

			if tt.setup != nil {
				tt.setup(h)
			}

			h.clearAutoRegistered(tt.nodeType, tt.nodeName)

			if tt.check != nil {
				tt.check(t, h)
			}
		})
	}
}
