package monitoring

import (
	"context"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
	"time"
)

// Exercise Go's real benchmark reporter, not testing.Benchmark (which does not
// print the name before fixture setup). This checks sample visibility, not speed.
func TestMetricWindowNodeAlertBenchmarkOutput(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, executable,
		"-test.run=^$", "-test.bench=^BenchmarkMetricWindowNodeAlertPass$",
		"-test.benchtime=1x", "-test.count=2").CombinedOutput()
	if err != nil {
		t.Fatalf("benchmark reporter failed: %v\n%s", err, output)
	}
	text := string(output)
	row := regexp.MustCompile(`^BenchmarkMetricWindowNodeAlertPass/(nodes-[0-9]+)(-[0-9]+)?\s+1\s+[0-9.]+ ns/op\s+[0-9.]+ ns/subject\s+[0-9]+ B/op\s+[0-9]+ allocs/op$`)
	counts := map[string]int{}
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, "BenchmarkMetricWindowNodeAlertPass/") {
			match := row.FindStringSubmatch(line)
			if match == nil {
				t.Fatalf("sample name and measurements are not one parseable row: %q", line)
			}
			counts[match[1]]++
		}
	}
	if len(counts) != 2 || counts["nodes-100"] != 2 || counts["nodes-1000"] != 2 {
		t.Fatalf("missing or duplicated benchmark samples: %v\n%s", counts, output)
	}
}
