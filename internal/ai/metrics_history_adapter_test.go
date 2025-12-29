package ai

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
)

func TestNewMetricsHistoryAdapter_Nil(t *testing.T) {
	if NewMetricsHistoryAdapter(nil) != nil {
		t.Fatal("expected nil adapter when history is nil")
	}
}

func TestMetricsHistoryAdapter_NilHistory(t *testing.T) {
	adapter := &MetricsHistoryAdapter{}

	if got := adapter.GetNodeMetrics("node-1", "cpu", time.Hour); got != nil {
		t.Fatalf("expected nil node metrics, got %v", got)
	}
	if got := adapter.GetGuestMetrics("guest-1", "cpu", time.Hour); got != nil {
		t.Fatalf("expected nil guest metrics, got %v", got)
	}
	if got := adapter.GetAllGuestMetrics("guest-1", time.Hour); got != nil {
		t.Fatalf("expected nil guest metrics map, got %v", got)
	}
	if got := adapter.GetAllStorageMetrics("storage-1", time.Hour); got != nil {
		t.Fatalf("expected nil storage metrics map, got %v", got)
	}
}

func TestMetricsHistoryAdapter_Conversions(t *testing.T) {
	history := monitoring.NewMetricsHistory(10, 24*time.Hour)
	adapter := NewMetricsHistoryAdapter(history)
	if adapter == nil {
		t.Fatal("expected adapter to be created")
	}

	now := time.Now()
	history.AddNodeMetric("node-1", "cpu", 42.5, now)
	history.AddGuestMetric("guest-1", "memory", 73.2, now)
	history.AddStorageMetric("storage-1", "usage", 88.1, now)

	nodePoints := adapter.GetNodeMetrics("node-1", "cpu", time.Hour)
	if len(nodePoints) != 1 {
		t.Fatalf("expected 1 node point, got %d", len(nodePoints))
	}
	if nodePoints[0].Value != 42.5 {
		t.Fatalf("expected node value 42.5, got %v", nodePoints[0].Value)
	}

	guestPoints := adapter.GetGuestMetrics("guest-1", "memory", time.Hour)
	if len(guestPoints) != 1 {
		t.Fatalf("expected 1 guest point, got %d", len(guestPoints))
	}
	if guestPoints[0].Value != 73.2 {
		t.Fatalf("expected guest value 73.2, got %v", guestPoints[0].Value)
	}

	allGuest := adapter.GetAllGuestMetrics("guest-1", time.Hour)
	if len(allGuest["memory"]) != 1 {
		t.Fatalf("expected 1 guest memory point, got %d", len(allGuest["memory"]))
	}

	allStorage := adapter.GetAllStorageMetrics("storage-1", time.Hour)
	if len(allStorage["usage"]) != 1 {
		t.Fatalf("expected 1 storage usage point, got %d", len(allStorage["usage"]))
	}
}

func TestMetricsHistoryAdapter_ConvertHelpers(t *testing.T) {
	if convertMetricPoints(nil) != nil {
		t.Fatal("expected nil when converting nil points")
	}
	if convertMetricsMap(nil) != nil {
		t.Fatal("expected nil when converting nil map")
	}
}
