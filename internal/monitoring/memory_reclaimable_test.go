package monitoring

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestSplitReclaimableMemory(t *testing.T) {
	t.Run("splits cache out of available-based free", func(t *testing.T) {
		memory := models.Memory{Total: 16, Used: 6, Free: 10, Usage: 37.5}
		splitReclaimableMemory(&memory, 4)
		if memory.Cache != 6 {
			t.Fatalf("Cache = %d, want 6", memory.Cache)
		}
		if memory.Free != 4 {
			t.Fatalf("Free = %d, want 4", memory.Free)
		}
		if memory.Used != 6 || memory.Total != 16 {
			t.Fatalf("Used/Total mutated: %+v", memory)
		}
		if got := memory.Used + memory.Cache + memory.Free; got != memory.Total {
			t.Fatalf("used+cache+free = %d, want %d", got, memory.Total)
		}
	})

	t.Run("no-op when truly free is unknown", func(t *testing.T) {
		memory := models.Memory{Total: 16, Used: 6, Free: 10}
		splitReclaimableMemory(&memory, 0)
		if memory.Cache != 0 || memory.Free != 10 {
			t.Fatalf("unexpected split: %+v", memory)
		}
	})

	t.Run("no-op when truly free is not smaller than free", func(t *testing.T) {
		memory := models.Memory{Total: 16, Used: 6, Free: 10}
		splitReclaimableMemory(&memory, 10)
		if memory.Cache != 0 || memory.Free != 10 {
			t.Fatalf("unexpected split: %+v", memory)
		}
		splitReclaimableMemory(&memory, 12)
		if memory.Cache != 0 || memory.Free != 10 {
			t.Fatalf("unexpected split with oversized trulyFree: %+v", memory)
		}
	})

	t.Run("no-op on nil or non-positive free", func(t *testing.T) {
		splitReclaimableMemory(nil, 4)
		memory := models.Memory{Total: 16, Used: 16, Free: 0}
		splitReclaimableMemory(&memory, 4)
		if memory.Cache != 0 {
			t.Fatalf("unexpected split on zero free: %+v", memory)
		}
	})
}
