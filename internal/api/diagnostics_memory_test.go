package api

import (
	"context"
	"reflect"
	"testing"
	"time"
	"unsafe"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	discoverysvc "github.com/rcourtman/pulse-go-rewrite/internal/discovery"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	pkgdiscovery "github.com/rcourtman/pulse-go-rewrite/pkg/discovery"
)

func TestClassifyMemorySourceTrust(t *testing.T) {
	tests := []struct {
		source string
		want   string
	}{
		{source: "available-field", want: "preferred"},
		{source: "avail-field", want: "preferred"},
		{source: "meminfo-available", want: "preferred"},
		{source: "node-status-available", want: "preferred"},
		{source: "derived-free-buffers-cached", want: "derived"},
		{source: "derived-total-minus-used", want: "derived"},
		{source: "meminfo-derived", want: "derived"},
		{source: "meminfo-total-minus-used", want: "derived"},
		{source: "calculated", want: "derived"},
		{source: "rrd-memavailable", want: "fallback"},
		{source: "rrd-available", want: "fallback"},
		{source: "rrd-data", want: "fallback"},
		{source: "listing", want: "fallback"},
		{source: "powered-off", want: "fallback"},
		{source: "previous-snapshot", want: "fallback"},
		{source: "", want: "fallback"},
	}

	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			if got := classifyMemorySourceTrust(tt.source); got != tt.want {
				t.Fatalf("classifyMemorySourceTrust(%q) = %q, want %q", tt.source, got, tt.want)
			}
		})
	}
}

func TestBuildMemorySourceDiagnostics(t *testing.T) {
	base := time.Date(2026, 3, 14, 16, 30, 0, 0, time.UTC)
	snapshots := monitoring.DiagnosticSnapshotSet{
		Nodes: []monitoring.NodeMemorySnapshot{
			{
				Instance:       "pve-a",
				Node:           "node-1",
				RetrievedAt:    base,
				MemorySource:   "derived-free-buffers-cached",
				FallbackReason: "",
			},
			{
				Instance:       "pve-a",
				Node:           "node-2",
				RetrievedAt:    base.Add(2 * time.Minute),
				MemorySource:   "previous-snapshot",
				FallbackReason: "",
			},
			{
				Instance:       "pve-a",
				Node:           "node-3",
				RetrievedAt:    base.Add(3 * time.Minute),
				MemorySource:   "previous-snapshot",
				FallbackReason: "node-status-unavailable",
			},
		},
		Guests: []monitoring.GuestMemorySnapshot{
			{
				Instance:       "pve-a",
				GuestType:      "qemu",
				Node:           "node-1",
				VMID:           101,
				RetrievedAt:    base.Add(time.Minute),
				MemorySource:   "rrd-available",
				FallbackReason: "",
			},
			{
				Instance:       "pve-a",
				GuestType:      "qemu",
				Node:           "node-2",
				VMID:           102,
				RetrievedAt:    base.Add(4 * time.Minute),
				MemorySource:   "derived-total-minus-used",
				FallbackReason: "derived-total-minus-used",
			},
		},
	}

	nodeStats, breakdown := buildMemorySourceDiagnostics(snapshots)

	if len(nodeStats) != 2 {
		t.Fatalf("len(nodeStats) = %d, want 2", len(nodeStats))
	}
	if nodeStats[0].Source != "derived-free-buffers-cached" || nodeStats[0].Trust != "derived" || nodeStats[0].Fallback {
		t.Fatalf("unexpected first node stat: %#v", nodeStats[0])
	}
	if nodeStats[1].Source != "previous-snapshot" || nodeStats[1].Trust != "fallback" || !nodeStats[1].Fallback || nodeStats[1].NodeCount != 2 {
		t.Fatalf("unexpected second node stat: %#v", nodeStats[1])
	}
	if nodeStats[1].LastUpdated != "2026-03-14T16:33:00Z" {
		t.Fatalf("nodeStats[1].LastUpdated = %q, want 2026-03-14T16:33:00Z", nodeStats[1].LastUpdated)
	}

	if len(breakdown) != 4 {
		t.Fatalf("len(breakdown) = %d, want 4", len(breakdown))
	}

	var previousSnapshot MemorySourceBreakdown
	var guestRRD MemorySourceBreakdown
	for _, entry := range breakdown {
		switch {
		case entry.Scope == "node" && entry.Source == "previous-snapshot":
			previousSnapshot = entry
		case entry.Scope == "guest" && entry.Source == "rrd-memavailable":
			guestRRD = entry
		}
	}

	if previousSnapshot.Count != 2 || previousSnapshot.Trust != "fallback" || !previousSnapshot.Fallback {
		t.Fatalf("unexpected previous snapshot breakdown: %#v", previousSnapshot)
	}
	if len(previousSnapshot.FallbackReasons) != 2 ||
		previousSnapshot.FallbackReasons[0] != "node-status-unavailable" ||
		previousSnapshot.FallbackReasons[1] != "preserved-previous-snapshot" {
		t.Fatalf("unexpected previous snapshot fallback reasons: %#v", previousSnapshot.FallbackReasons)
	}

	if guestRRD.Scope != "guest" || guestRRD.Count != 1 || guestRRD.Trust != "fallback" || !guestRRD.Fallback {
		t.Fatalf("unexpected guest RRD breakdown: %#v", guestRRD)
	}
	if len(guestRRD.FallbackReasons) != 1 || guestRRD.FallbackReasons[0] != "rrd-memavailable" {
		t.Fatalf("unexpected guest RRD fallback reasons: %#v", guestRRD.FallbackReasons)
	}
}

func TestBuildMemorySourceDiagnostics_BackfillsCanonicalFallbackReason(t *testing.T) {
	base := time.Date(2026, 3, 14, 17, 0, 0, 0, time.UTC)
	snapshots := monitoring.DiagnosticSnapshotSet{
		Nodes: []monitoring.NodeMemorySnapshot{
			{
				Instance:       "pve-a",
				Node:           "node-1",
				RetrievedAt:    base,
				MemorySource:   "rrd-available",
				FallbackReason: "",
			},
		},
		Guests: []monitoring.GuestMemorySnapshot{
			{
				Instance:       "pve-a",
				GuestType:      "qemu",
				Node:           "node-1",
				VMID:           101,
				RetrievedAt:    base.Add(time.Minute),
				MemorySource:   "listing",
				FallbackReason: "",
			},
		},
	}

	_, breakdown := buildMemorySourceDiagnostics(snapshots)
	if len(breakdown) != 2 {
		t.Fatalf("len(breakdown) = %d, want 2", len(breakdown))
	}

	var nodeRRD MemorySourceBreakdown
	var guestListing MemorySourceBreakdown
	for _, entry := range breakdown {
		switch {
		case entry.Scope == "node" && entry.Source == "rrd-memavailable":
			nodeRRD = entry
		case entry.Scope == "guest" && entry.Source == "cluster-resources":
			guestListing = entry
		}
	}

	if len(nodeRRD.FallbackReasons) != 1 || nodeRRD.FallbackReasons[0] != "rrd-memavailable" {
		t.Fatalf("unexpected node fallback reasons: %#v", nodeRRD.FallbackReasons)
	}
	if len(guestListing.FallbackReasons) != 1 || guestListing.FallbackReasons[0] != "cluster-resources" {
		t.Fatalf("unexpected guest fallback reasons: %#v", guestListing.FallbackReasons)
	}
}

func TestBuildMemorySourceDiagnostics_PoweredOffIsLowTrustNotFallback(t *testing.T) {
	base := time.Date(2026, 3, 14, 17, 5, 0, 0, time.UTC)
	snapshots := monitoring.DiagnosticSnapshotSet{
		Guests: []monitoring.GuestMemorySnapshot{
			{
				Instance:       "pve-a",
				GuestType:      "qemu",
				Node:           "node-1",
				VMID:           101,
				RetrievedAt:    base,
				MemorySource:   "powered-off",
				FallbackReason: "",
			},
		},
	}

	_, breakdown := buildMemorySourceDiagnostics(snapshots)
	if len(breakdown) != 1 {
		t.Fatalf("len(breakdown) = %d, want 1", len(breakdown))
	}

	entry := breakdown[0]
	if entry.Scope != "guest" || entry.Source != "powered-off" {
		t.Fatalf("unexpected breakdown entry: %#v", entry)
	}
	if entry.Trust != "fallback" {
		t.Fatalf("entry.Trust = %q, want fallback", entry.Trust)
	}
	if entry.Fallback {
		t.Fatalf("entry.Fallback = true, want false")
	}
	if len(entry.FallbackReasons) != 0 {
		t.Fatalf("entry.FallbackReasons = %#v, want empty", entry.FallbackReasons)
	}
}

func TestComputeDiagnostics_EmitsMemorySourceBreakdown(t *testing.T) {
	cfg := &config.Config{DataPath: t.TempDir()}
	monitor := newMonitorForDiagnostics(t, cfg)

	base := time.Date(2026, 3, 14, 16, 45, 0, 0, time.UTC)
	setDiagnosticsUnexportedField(t, monitor, "nodeSnapshots", map[string]monitoring.NodeMemorySnapshot{
		"pve-a|node-1": {
			Instance:       "pve-a",
			Node:           "node-1",
			RetrievedAt:    base,
			MemorySource:   "available-field",
			FallbackReason: "",
		},
		"pve-a|node-2": {
			Instance:       "pve-a",
			Node:           "node-2",
			RetrievedAt:    base.Add(time.Minute),
			MemorySource:   "previous-snapshot",
			FallbackReason: "preserved-previous-snapshot",
		},
	})
	setDiagnosticsUnexportedField(t, monitor, "guestSnapshots", map[string]monitoring.GuestMemorySnapshot{
		"pve-a|qemu|node-1|101": {
			Instance:       "pve-a",
			GuestType:      "qemu",
			Node:           "node-1",
			VMID:           101,
			RetrievedAt:    base.Add(2 * time.Minute),
			MemorySource:   "rrd-memavailable",
			FallbackReason: "rrd-memavailable",
		},
	})

	router := &Router{config: cfg, monitor: monitor}
	diag := router.computeDiagnostics(context.Background())

	if len(diag.MemorySources) != 2 {
		t.Fatalf("len(diag.MemorySources) = %d, want 2", len(diag.MemorySources))
	}
	if diag.MemorySources[0].Source != "available-field" || diag.MemorySources[0].Trust != "preferred" {
		t.Fatalf("unexpected first memory source stat: %#v", diag.MemorySources[0])
	}
	if diag.MemorySources[1].Source != "previous-snapshot" || !diag.MemorySources[1].Fallback || diag.MemorySources[1].Trust != "fallback" {
		t.Fatalf("unexpected second memory source stat: %#v", diag.MemorySources[1])
	}

	if len(diag.MemorySourceBreakdown) != 3 {
		t.Fatalf("len(diag.MemorySourceBreakdown) = %d, want 3", len(diag.MemorySourceBreakdown))
	}
	if len(diag.GuestSnapshots) != 1 {
		t.Fatalf("len(diag.GuestSnapshots) = %d, want 1", len(diag.GuestSnapshots))
	}
	if diag.GuestSnapshots[0].Notes == nil {
		t.Fatalf("expected guest snapshot notes to normalize to empty slice")
	}

	var nodeFallback MemorySourceBreakdown
	var guestFallback MemorySourceBreakdown
	for _, entry := range diag.MemorySourceBreakdown {
		switch {
		case entry.Scope == "node" && entry.Source == "previous-snapshot":
			nodeFallback = entry
		case entry.Scope == "guest" && entry.Source == "rrd-memavailable":
			guestFallback = entry
		}
	}

	if nodeFallback.Count != 1 || len(nodeFallback.FallbackReasons) != 1 || nodeFallback.FallbackReasons[0] != "preserved-previous-snapshot" {
		t.Fatalf("unexpected node fallback breakdown: %#v", nodeFallback)
	}
	if guestFallback.Count != 1 || guestFallback.Trust != "fallback" || !guestFallback.Fallback {
		t.Fatalf("unexpected guest fallback breakdown: %#v", guestFallback)
	}
	if len(guestFallback.FallbackReasons) != 1 || guestFallback.FallbackReasons[0] != "rrd-memavailable" {
		t.Fatalf("unexpected guest fallback reasons: %#v", guestFallback.FallbackReasons)
	}
}

func TestComputeDiagnostics_DiscoveryUsesStructuredErrorOwnership(t *testing.T) {
	cfg := &config.Config{DataPath: t.TempDir()}
	monitor := newMonitorForDiagnostics(t, cfg)

	service := discoverysvc.NewService(nil, time.Minute, "auto", func() config.DiscoveryConfig {
		return config.DefaultDiscoveryConfig()
	})
	cache := &discoverysvc.DiscoveryCache{}
	setDiagnosticsUnexportedField(t, cache, "result", &pkgdiscovery.DiscoveryResult{
		Servers: []pkgdiscovery.DiscoveredServer{
			{IP: "10.0.0.1", Port: 8006, Type: "pve"},
		},
		StructuredErrors: []pkgdiscovery.DiscoveryError{
			{
				Phase:     "docker_bridge_network",
				ErrorType: "timeout",
				Message:   "request timed out",
				IP:        "10.0.0.2",
				Port:      8007,
				Timestamp: time.Unix(1700000000, 0).UTC(),
			},
		},
	})
	setDiagnosticsUnexportedField(t, cache, "updated", time.Unix(1700000010, 0).UTC())
	setDiagnosticsUnexportedField(t, service, "cache", cache)
	setDiagnosticsUnexportedField(t, monitor, "discoveryService", service)

	router := &Router{config: cfg, monitor: monitor}
	diag := router.computeDiagnostics(context.Background())

	if diag.Discovery == nil {
		t.Fatalf("expected discovery diagnostics")
	}
	if diag.Discovery.LastResultServers != 1 {
		t.Fatalf("LastResultServers = %d, want 1", diag.Discovery.LastResultServers)
	}
	if diag.Discovery.LastResultErrors != 1 {
		t.Fatalf("LastResultErrors = %d, want 1", diag.Discovery.LastResultErrors)
	}
}

func setDiagnosticsUnexportedField(t *testing.T, target interface{}, fieldName string, value interface{}) {
	t.Helper()

	v := reflect.ValueOf(target)
	if v.Kind() != reflect.Ptr || v.IsNil() {
		t.Fatalf("target must be a non-nil pointer")
	}

	field := v.Elem().FieldByName(fieldName)
	if !field.IsValid() {
		t.Fatalf("field %q not found", fieldName)
	}

	ptr := unsafe.Pointer(field.UnsafeAddr())
	reflect.NewAt(field.Type(), ptr).Elem().Set(reflect.ValueOf(value))
}
