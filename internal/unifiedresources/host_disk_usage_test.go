package unifiedresources

import (
	"math"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestDiskUsagePercentSharesCanonicalAdmission(t *testing.T) {
	for _, tc := range []struct {
		name     string
		disk     models.Disk
		observed bool
	}{
		{"observed zero", models.Disk{Total: 1000}, true},
		{"observed fraction", models.Disk{Total: 1000, Used: 1, Usage: 0.1}, true},
		{"observed full", models.Disk{Total: 1000, Used: 1000, Usage: 100}, true},
		{"unavailable", models.Disk{}, false},
		{"capacity absent", models.Disk{Usage: 95}, false},
		{"used negative", models.Disk{Total: 1000, Used: -1}, false},
		{"used over capacity", models.Disk{Total: 1000, Used: 1001}, false},
		{"percentage negative", models.Disk{Total: 1000, Usage: -1}, false},
		{"percentage over 100", models.Disk{Total: 1000, Usage: 101}, false},
		{"NaN", models.Disk{Total: 1000, Usage: math.NaN()}, false},
		{"infinite", models.Disk{Total: 1000, Usage: math.Inf(1)}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			value, observed := DiskUsagePercent(tc.disk)
			if observed != tc.observed || (observed && value != tc.disk.Usage) || (!observed && value != 0) {
				t.Fatalf("DiskUsagePercent = %v, %v; want observed=%v", value, observed, tc.observed)
			}
			summary, summaryObserved := HostDiskUsagePercent([]models.Disk{tc.disk})
			if value != summary || observed != summaryObserved {
				t.Fatal("summary and filesystem admission diverged")
			}
		})
	}
}
