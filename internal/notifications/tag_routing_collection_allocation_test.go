package notifications

import (
	"strings"
	"sync"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

// The prepared-policy matcher from before direct collection. Keep filter
// preparation out of both sides of the allocation comparison.
func referenceNotificationPreparedCollectionMatch(alert *alerts.Alert, filter []string, mode string) bool {
	if len(filter) == 0 {
		return true
	}
	tags := referenceNotificationCollectedTags(alert)
	if len(tags) == 0 {
		return false
	}
	set := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		set[strings.ToLower(tag)] = struct{}{}
	}
	for _, tag := range filter {
		_, found := set[tag]
		if mode == notificationTagModeAny && found {
			return true
		}
		if mode != notificationTagModeAny && !found {
			return false
		}
	}
	return mode != notificationTagModeAny
}

func TestNotificationTagCollectionAllocations(t *testing.T) {
	for _, tc := range []struct {
		name   string
		alert  *alerts.Alert
		filter []string
	}{
		{"mixed", notificationBatchFixture(1)[0], []string{"customer:alpha", "critical"}},
		{"array-128", notificationTagCollectionLargeFixture(false), []string{"rack:127"}},
		{"string-128", notificationTagCollectionLargeFixture(true), []string{"rack:127"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := testing.AllocsPerRun(5, func() {
				notificationTagCollectionSink = referenceNotificationPreparedCollectionMatch(tc.alert, tc.filter, "all")
			})
			after := testing.AllocsPerRun(5, func() {
				notificationTagCollectionSink = notificationAlertMatchesPreparedTags(tc.alert, tc.filter, "all")
			})
			t.Logf("per alert: frozen collector %.0f allocs; direct collector %.0f allocs", before, after)
			if !notificationTagCollectionSink || after+1 > before {
				t.Fatalf("temporary collection work returned: %.0f allocs, old %.0f", after, before)
			}
		})
	}
}

func TestNotificationTagCollectionConcurrentDestinations(t *testing.T) {
	alert := taggedRoutingAlert("shared", []interface{}{" alpha ", "Σ", "a,b", 7})
	filters := [][]string{{"alpha"}, {"σ"}, {"ς"}, {"a,b"}, {"a"}, {"absent"}}
	var workers sync.WaitGroup
	for _, filter := range filters {
		workers.Go(func() {
			want := referenceNotificationTagMatch(alert, filter, "all")
			for range 100 {
				if got := notificationAlertMatchesPreparedTags(alert, filter, "all"); got != want {
					t.Errorf("concurrent filter %q: match=%t, want %t", filter, got, want)
					return
				}
			}
		})
	}
	workers.Wait()
}
