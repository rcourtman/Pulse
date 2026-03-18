package monitoring

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var bannedSnapshotResourceAccessPatterns = []struct {
	re      *regexp.Regexp
	message string
}{
	{
		re:      regexp.MustCompile(`GetState\(\)\.(VMs|Containers|Nodes|Hosts|DockerHosts|PBSInstances|Storage|PhysicalDisks)\b`),
		message: "derive canonical monitoring resource views through ReadState-backed helpers instead of GetState() resource arrays",
	},
}

func readMonitoringRuntimeFiles(t *testing.T) map[string]string {
	t.Helper()

	files := make(map[string]string)
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("failed to read monitoring directory: %v", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || filepath.Ext(name) != ".go" || strings.HasSuffix(name, "_test.go") {
			continue
		}
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("failed to read %s: %v", name, err)
		}
		files[name] = string(data)
	}

	return files
}

func TestNoGetStateResourceArrayRegression(t *testing.T) {
	for name, content := range readMonitoringRuntimeFiles(t) {
		for _, pattern := range bannedSnapshotResourceAccessPatterns {
			if matches := pattern.re.FindAllStringIndex(content, -1); len(matches) > 0 {
				for _, match := range matches {
					line := 1 + strings.Count(content[:match[0]], "\n")
					t.Errorf("%s:%d: %s", name, line, pattern.message)
				}
			}
		}
	}
}

func TestLegacyMemorySourceAliasesRemainCanonicalized(t *testing.T) {
	t.Parallel()

	tests := []struct {
		source    string
		canonical string
	}{
		{source: "avail-field", canonical: "available-field"},
		{source: "meminfo-available", canonical: "available-field"},
		{source: "node-status-available", canonical: "available-field"},
		{source: "meminfo-derived", canonical: "derived-free-buffers-cached"},
		{source: "calculated", canonical: "derived-free-buffers-cached"},
		{source: "meminfo-total-minus-used", canonical: "derived-total-minus-used"},
		{source: "rrd-available", canonical: "rrd-memavailable"},
		{source: "rrd-data", canonical: "rrd-memused"},
		{source: "listing-mem", canonical: "cluster-resources"},
		{source: "listing", canonical: "cluster-resources"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.source, func(t *testing.T) {
			t.Parallel()
			if got := CanonicalMemorySource(tt.source); got != tt.canonical {
				t.Fatalf("CanonicalMemorySource(%q) = %q, want %q", tt.source, got, tt.canonical)
			}
		})
	}
}
