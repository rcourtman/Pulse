package dockeragent

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/hostagent"
	"github.com/rcourtman/pulse-go-rewrite/internal/hostmetrics"
)

// Exercise the real module entry points: independent Collector tests cannot
// catch a caller reverting to the package-level shared baseline (#1894).
func TestIssue1894HostDockerCPURouting(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("uses Linux procfs counter fixtures")
	}
	proc := t.TempDir()
	write := func(name, data string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(proc, name), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write("meminfo", "MemTotal: 8192 kB\nMemFree: 4096 kB\nMemAvailable: 4096 kB\n")
	write("mountinfo", "")
	t.Setenv("HOST_PROC", proc)
	t.Setenv("HOST_PROC_MOUNTINFO", filepath.Join(proc, "mountinfo"))
	dockerHostMetrics = hostmetrics.Collector{}
	t.Cleanup(func() { dockerHostMetrics = hostmetrics.Collector{} })
	host := hostagent.NewDefaultCollector()
	ctx := context.Background()
	for cycle := 0; cycle < 4; cycle++ {
		for caller := 0; caller < 3; caller++ {
			// Each loop sees 300 busy / 6000 total jiffies per interval.
			// Adjacent loops see a short 80%-busy burst instead.
			busy := 10000 + cycle*300 + caller*80
			idle := 100000 + cycle*5700 + caller*20
			write("stat", fmt.Sprintf("cpu %d 0 0 %d 0 0 0 0 0 0\ncpu0 %d 0 0 %d 0 0 0 0 0 0\n", busy, idle, busy, idle))
			var snapshot hostmetrics.Snapshot
			var err error
			switch caller {
			case 0:
				snapshot, err = host.Metrics(ctx, nil)
			case 1:
				// Both Docker routes must reuse the same retained baseline.
				var include []string
				if cycle%2 == 1 {
					include = []string{"/"}
				}
				snapshot, err = hostmetricsCollectWithDiskFilters(ctx, nil, include)
			case 2:
				snapshot, err = hostmetrics.Collect(ctx, nil)
			}
			if err != nil {
				t.Fatal(err)
			}
			if cycle > 0 && math.Abs(snapshot.CPUUsagePercent-5) > 1e-8 {
				t.Fatalf("cycle %d caller %d: CPU = %v, want full-interval 5%%", cycle, caller, snapshot.CPUUsagePercent)
			}
		}
	}
}
