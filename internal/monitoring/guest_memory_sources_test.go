package monitoring

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

func TestDeriveGuestMemInfoAvailable(t *testing.T) {
	kb := uint64(1024)
	miB := uint64(1024 * 1024)
	giB := uint64(1024 * 1024 * 1024)

	tests := []struct {
		name          string
		memInfo       *proxmox.VMMemInfo
		wantAvailable uint64
		wantSource    string
	}{
		{
			name: "uses available field when present",
			memInfo: &proxmox.VMMemInfo{
				Total:     8 * giB,
				Used:      6 * giB,
				Free:      2 * giB,
				Available: 4 * giB,
				Buffers:   500 * miB,
				Cached:    1 * giB,
			},
			wantAvailable: 4 * giB,
			wantSource:    "available-field",
		},
		{
			name: "derives from free+buffers+cached when available missing",
			memInfo: &proxmox.VMMemInfo{
				Total:   8 * giB,
				Used:    5 * giB,
				Free:    2 * giB,
				Buffers: 500 * miB,
				Cached:  1 * giB,
			},
			wantAvailable: 2*giB + 500*miB + 1*giB,
			wantSource:    "derived-free-buffers-cached",
		},
		{
			name: "returns zero when only free is available (no cache metrics)",
			memInfo: &proxmox.VMMemInfo{
				Total: 7796964 * kb,
				Used:  7352140 * kb,
				Free:  444824 * kb,
			},
			wantAvailable: 0,
			wantSource:    "",
		},
		{
			name: "returns availableFromUsed when used excludes cache despite missing cache metrics",
			memInfo: &proxmox.VMMemInfo{
				Total: 8 * giB,
				Used:  3 * giB,
				Free:  1 * giB,
			},
			wantAvailable: 5 * giB,
			wantSource:    "derived-total-minus-used",
		},
		{
			name: "returns zero when only total and used with used equal to total minus free",
			memInfo: &proxmox.VMMemInfo{
				Total: 8 * giB,
				Used:  7 * giB,
			},
			wantAvailable: 1 * giB,
			wantSource:    "derived-total-minus-used",
		},
		{
			name: "returns zero for empty meminfo",
			memInfo: &proxmox.VMMemInfo{
				Total: 8 * giB,
			},
			wantAvailable: 0,
			wantSource:    "",
		},
		{
			name:          "nil meminfo returns zero",
			memInfo:       nil,
			wantAvailable: 0,
			wantSource:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotAvailable, gotSource := deriveGuestMemInfoAvailable(tt.memInfo, nil)
			if gotAvailable != tt.wantAvailable {
				t.Fatalf("available = %d, want %d", gotAvailable, tt.wantAvailable)
			}
			if gotSource != tt.wantSource {
				t.Fatalf("source = %q, want %q", gotSource, tt.wantSource)
			}
		})
	}
}

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
			name:     "uses ballooninfo free memory when status mem is falsely saturated",
			memTotal: 8 * giB,
			status: &proxmox.VMStatus{
				Mem:    8 * giB,
				MaxMem: 8 * giB,
				BalloonInfo: &proxmox.VMBalloonInfo{
					FreeMem:  5 * giB,
					TotalMem: 8 * giB,
				},
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
