package monitoring

import "testing"

func TestDescribeMemorySourceCanonicalizesAliases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                string
		source              string
		wantCanonical       string
		wantTrust           string
		wantFallback        bool
		wantDefaultFallback string
	}{
		{
			name:          "available alias",
			source:        " meminfo-available ",
			wantCanonical: "available-field",
			wantTrust:     "preferred",
		},
		{
			name:          "legacy node available alias",
			source:        "node-status-available",
			wantCanonical: "available-field",
			wantTrust:     "preferred",
		},
		{
			name:                "derived total minus used alias",
			source:              "meminfo-total-minus-used",
			wantCanonical:       "derived-total-minus-used",
			wantTrust:           "derived",
			wantFallback:        true,
			wantDefaultFallback: "derived-total-minus-used",
		},
		{
			name:          "legacy calculated alias",
			source:        "calculated",
			wantCanonical: "derived-free-buffers-cached",
			wantTrust:     "derived",
		},
		{
			name:                "legacy rrd available alias",
			source:              "rrd-available",
			wantCanonical:       "rrd-memavailable",
			wantTrust:           "fallback",
			wantFallback:        true,
			wantDefaultFallback: "rrd-memavailable",
		},
		{
			name:                "legacy rrd data alias",
			source:              "rrd-data",
			wantCanonical:       "rrd-memused",
			wantTrust:           "fallback",
			wantFallback:        true,
			wantDefaultFallback: "rrd-memused",
		},
		{
			name:                "listing alias",
			source:              "listing-mem",
			wantCanonical:       "cluster-resources",
			wantTrust:           "fallback",
			wantFallback:        true,
			wantDefaultFallback: "cluster-resources",
		},
		{
			name:                "legacy listing alias",
			source:              "listing",
			wantCanonical:       "cluster-resources",
			wantTrust:           "fallback",
			wantFallback:        true,
			wantDefaultFallback: "cluster-resources",
		},
		{
			name:                "agent alias",
			source:              "agent",
			wantCanonical:       "agent",
			wantTrust:           "fallback",
			wantFallback:        true,
			wantDefaultFallback: "host-agent-memory",
		},
		{
			name:          "powered off is non-fallback",
			source:        "powered-off",
			wantCanonical: "powered-off",
			wantTrust:     "fallback",
		},
		{
			name:          "unknown source falls back canonically",
			source:        "custom-source",
			wantCanonical: "custom-source",
			wantTrust:     "fallback",
			wantFallback:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := DescribeMemorySource(tt.source)
			if got.Canonical != tt.wantCanonical {
				t.Fatalf("Canonical = %q, want %q", got.Canonical, tt.wantCanonical)
			}
			if got.Trust != tt.wantTrust {
				t.Fatalf("Trust = %q, want %q", got.Trust, tt.wantTrust)
			}
			if got.Fallback != tt.wantFallback {
				t.Fatalf("Fallback = %v, want %v", got.Fallback, tt.wantFallback)
			}
			if got.DefaultFallbackReason != tt.wantDefaultFallback {
				t.Fatalf("DefaultFallbackReason = %q, want %q", got.DefaultFallbackReason, tt.wantDefaultFallback)
			}
		})
	}
}
