package monitoring

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

func TestSelectGuestLowTrustUsedMemory(t *testing.T) {
	giB := uint64(1024 * 1024 * 1024)

	tests := []struct {
		name       string
		memTotal   uint64
		status     *proxmox.VMStatus
		wantUsed   uint64
		wantSource string
	}{
		{
			name:     "prefers status mem when it is plausible",
			memTotal: 8 * giB,
			status: &proxmox.VMStatus{
				Mem:     3 * giB,
				FreeMem: 5 * giB,
			},
			wantUsed:   3 * giB,
			wantSource: "status-mem",
		},
		{
			name:     "falls back to status freemem when status mem is falsely saturated",
			memTotal: 8 * giB,
			status: &proxmox.VMStatus{
				Mem:     8 * giB,
				FreeMem: 5 * giB,
			},
			wantUsed:   3 * giB,
			wantSource: "status-freemem",
		},
		{
			name:     "falls back to status freemem when status mem and freemem are materially inconsistent",
			memTotal: 8 * giB,
			status: &proxmox.VMStatus{
				Mem:     7920 * 1024 * 1024,
				FreeMem: 5 * giB,
			},
			wantUsed:   3 * giB,
			wantSource: "status-freemem",
		},
		{
			name:     "uses status freemem when status mem is absent",
			memTotal: 8 * giB,
			status: &proxmox.VMStatus{
				FreeMem: 6 * giB,
			},
			wantUsed:   2 * giB,
			wantSource: "status-freemem",
		},
		{
			name:     "uses balloon total when deriving used from status freemem",
			memTotal: 8 * giB,
			status: &proxmox.VMStatus{
				Balloon: 4 * giB,
				FreeMem: 1 * giB,
			},
			wantUsed:   3 * giB,
			wantSource: "status-freemem",
		},
		{
			name:     "uses balloon total when status mem is falsely saturated against balloon",
			memTotal: 8 * giB,
			status: &proxmox.VMStatus{
				Mem:     4 * giB,
				Balloon: 4 * giB,
				FreeMem: 1 * giB,
			},
			wantUsed:   3 * giB,
			wantSource: "status-freemem",
		},
		{
			name:       "returns empty selection without usable fields",
			memTotal:   8 * giB,
			status:     &proxmox.VMStatus{},
			wantUsed:   0,
			wantSource: "",
		},
		{
			name:     "keeps status mem when freemem only shows tiny headroom",
			memTotal: 8 * giB,
			status: &proxmox.VMStatus{
				Mem:     8*giB - (64 * 1024 * 1024),
				FreeMem: 64 * 1024 * 1024,
			},
			wantUsed:   8*giB - (64 * 1024 * 1024),
			wantSource: "status-mem",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotUsed, gotSource := selectGuestLowTrustUsedMemory(tt.memTotal, tt.status)
			if gotUsed != tt.wantUsed {
				t.Fatalf("used = %d, want %d", gotUsed, tt.wantUsed)
			}
			if gotSource != tt.wantSource {
				t.Fatalf("source = %q, want %q", gotSource, tt.wantSource)
			}
		})
	}
}
