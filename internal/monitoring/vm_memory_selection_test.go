package monitoring

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

func TestSelectVMAvailableFromMemInfo(t *testing.T) {
	giB := uint64(1024 * 1024 * 1024)
	miB := uint64(1024 * 1024)

	tests := []struct {
		name               string
		memInfo            *proxmox.VMMemInfo
		wantAvailable      uint64
		wantSource         string
		wantTotalMinusUsed uint64
	}{
		{
			name: "prefers explicit available",
			memInfo: &proxmox.VMMemInfo{
				Total:     16 * giB,
				Used:      10 * giB,
				Free:      2 * giB,
				Available: 6 * giB,
				Buffers:   512 * miB,
				Cached:    2 * giB,
			},
			wantAvailable:      6 * giB,
			wantSource:         "meminfo-available",
			wantTotalMinusUsed: 6 * giB,
		},
		{
			name: "uses derived free buffers cached when consistent",
			memInfo: &proxmox.VMMemInfo{
				Total:   8 * giB,
				Used:    4 * giB,
				Free:    1536 * miB,
				Buffers: 512 * miB,
				Cached:  2 * giB,
			},
			wantAvailable:      4 * giB,
			wantSource:         "meminfo-derived",
			wantTotalMinusUsed: 4 * giB,
		},
		{
			name: "defers derived when total-used gap is materially larger",
			memInfo: &proxmox.VMMemInfo{
				Total:   64 * giB,
				Used:    16 * giB,
				Free:    6 * giB,
				Buffers: 64 * miB,
				Cached:  128 * miB,
			},
			wantAvailable:      0,
			wantSource:         "",
			wantTotalMinusUsed: 48 * giB,
		},
		{
			name: "skips free-only meminfo and keeps total-used fallback",
			memInfo: &proxmox.VMMemInfo{
				Total: 32 * giB,
				Used:  22 * giB,
				Free:  10 * giB,
			},
			wantAvailable:      0,
			wantSource:         "",
			wantTotalMinusUsed: 10 * giB,
		},
		{
			name:    "nil meminfo",
			memInfo: nil,
		},
		{
			name: "keeps derived when gap is below tolerance",
			memInfo: &proxmox.VMMemInfo{
				Total:   1024 * miB,
				Used:    256 * miB,
				Free:    760 * miB,
				Buffers: 4 * miB,
			},
			wantAvailable:      764 * miB,
			wantSource:         "meminfo-derived",
			wantTotalMinusUsed: 768 * miB,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := selectVMAvailableFromMemInfo(tt.memInfo)
			if got.Available != tt.wantAvailable {
				t.Fatalf("available = %d, want %d", got.Available, tt.wantAvailable)
			}
			if got.Source != tt.wantSource {
				t.Fatalf("source = %q, want %q", got.Source, tt.wantSource)
			}
			if got.TotalMinusUsed != tt.wantTotalMinusUsed {
				t.Fatalf("totalMinusUsed = %d, want %d", got.TotalMinusUsed, tt.wantTotalMinusUsed)
			}
		})
	}
}
