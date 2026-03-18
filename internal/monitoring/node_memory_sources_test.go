package monitoring

import (
	"context"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

func TestResolveNodeMemoryCharacterization(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())

	const gib = uint64(1024 * 1024 * 1024)

	tests := []struct {
		name          string
		memory        *proxmox.MemoryStatus
		rrdPoints     []proxmox.NodeRRDPoint
		wantSource    string
		wantFallback  string
		wantUsed      uint64
		wantRawSource string
	}{
		{
			name: "missing MemAvailable derives from free buffers cached",
			memory: &proxmox.MemoryStatus{
				Total:   32 * gib,
				Used:    26 * gib,
				Free:    2 * gib,
				Buffers: 3 * gib,
				Cached:  7 * gib,
			},
			wantSource:    "derived-free-buffers-cached",
			wantFallback:  "",
			wantUsed:      20 * gib,
			wantRawSource: "node-status",
		},
		{
			name: "proxmox 8.4 field drift derives from total minus used gap",
			memory: &proxmox.MemoryStatus{
				Total: 134794743808,
				Used:  107351023616,
				Free:  6471057408,
			},
			wantSource:    "derived-total-minus-used",
			wantFallback:  "node-status-total-minus-used",
			wantUsed:      107351023616,
			wantRawSource: "node-status-total-minus-used",
		},
		{
			name: "missing cache fields falls back to rrd memused",
			memory: &proxmox.MemoryStatus{
				Total: 16 * gib,
				Used:  16 * gib,
				Free:  0,
			},
			rrdPoints: []proxmox.NodeRRDPoint{{
				MemTotal: floatPtr(float64(16 * gib)),
				MemUsed:  floatPtr(float64(6 * gib)),
			}},
			wantSource:    "rrd-memused",
			wantFallback:  "rrd-memused",
			wantUsed:      6 * gib,
			wantRawSource: "rrd-memused",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mon := newTestPVEMonitor("test")
			defer mon.alertManager.Stop()
			defer mon.notificationMgr.Stop()

			client := &stubPVEClient{rrdPoints: tt.rrdPoints}

			memory, source, fallback, raw, ok := mon.resolveNodeMemory(
				context.Background(),
				client,
				"test",
				"node1",
				tt.memory,
				NodeMemoryRaw{},
			)
			if !ok {
				t.Fatal("resolveNodeMemory() = ok=false")
			}
			if got := uint64(memory.Used); got != tt.wantUsed {
				t.Fatalf("memory.Used = %d, want %d", got, tt.wantUsed)
			}
			if source != tt.wantSource {
				t.Fatalf("source = %q, want %q", source, tt.wantSource)
			}
			if fallback != tt.wantFallback {
				t.Fatalf("fallback = %q, want %q", fallback, tt.wantFallback)
			}
			if raw.ProxmoxMemorySource != tt.wantRawSource {
				t.Fatalf("raw.ProxmoxMemorySource = %q, want %q", raw.ProxmoxMemorySource, tt.wantRawSource)
			}
		})
	}
}
