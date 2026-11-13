package notifications

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

// Keep the pre-batch policy here as a comparison, including its ToLower (not
// EqualFold) tag identity. It deliberately prepares policy for every member.
func referenceNotificationBatch(list []*alerts.Alert, filter []string, mode, minimum string, event notificationEvent) []*alerts.Alert {
	if event == eventResolved {
		return list
	}
	filter = normalizeNotificationTagFilter(filter)
	minimum = normalizeNotificationMinimumSeverity(minimum)
	if len(filter) == 0 && minimum == notificationMinimumSeverityAll {
		return list
	}
	result := make([]*alerts.Alert, 0, len(list))
	for _, alert := range list {
		if referenceNotificationTagMatch(alert, filter, mode) && notificationAlertMeetsMinimumSeverity(alert, minimum) {
			result = append(result, alert)
		}
	}
	return result
}

func referenceNotificationTagMatch(alert *alerts.Alert, filter []string, mode string) bool {
	filter = normalizeNotificationTagFilter(filter)
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
	if normalizeNotificationTagMode(mode) == notificationTagModeAny {
		for _, tag := range filter {
			if _, ok := set[strings.ToLower(tag)]; ok {
				return true
			}
		}
		return false
	}
	for _, tag := range filter {
		if _, ok := set[strings.ToLower(tag)]; !ok {
			return false
		}
	}
	return true
}

func notificationBatchFixture(size int) []*alerts.Alert {
	list := make([]*alerts.Alert, size)
	for i := range list {
		list[i] = &alerts.Alert{
			ID: fmt.Sprintf("batch-%d", i), Level: alerts.AlertLevelWarning,
			StartTime: time.Unix(1_700_000_000, 0),
			Metadata: map[string]interface{}{
				"tags": " Customer:Alpha ; production ", "resourceTags": []string{"Critical"},
				"hostTags": []interface{}{"eu-west", 7}, "serviceTags": "service:store",
			},
		}
	}
	return list
}

func TestNotificationBatchRoutingCompatibility(t *testing.T) {
	list := append(notificationBatchFixture(3),
		nil,
		&alerts.Alert{ID: "untagged", Level: alerts.AlertLevelCritical},
		&alerts.Alert{ID: "alias", Level: "error", Metadata: map[string]interface{}{"tags": []string{"CUSTOMER:BETA", "σ"}}},
		&alerts.Alert{ID: "unknown", Level: "unexpected", Metadata: map[string]interface{}{"serviceTags": []interface{}{"customer:alpha", false}}},
	)
	filters := [][]string{nil, {}, {"", "  "}, {"customer:alpha"}, {" CUSTOMER:ALPHA ", "customer:alpha", "CRITICAL"}, {"missing", "eu-west"}, {"customer:*"}, {"Σ"}, {"ς"}}
	modes := []string{"all", " ANY ", "invalid"}
	floors := []string{"all", " WARNING ", "critical", "invalid"}
	lists := [][]*alerts.Alert{nil, {}, list}
	cases := 0
	for _, members := range lists {
		for _, filter := range filters {
			for _, mode := range modes {
				for _, floor := range floors {
					for _, event := range []notificationEvent{eventAlert, eventResolved} {
						before, err := json.Marshal([]interface{}{members, filter})
						if err != nil {
							t.Fatal(err)
						}
						want := referenceNotificationBatch(members, filter, mode, floor, event)
						got := routeNotificationAlerts(members, filter, mode, floor, event)
						if !reflect.DeepEqual(got, want) {
							t.Fatalf("case %d filter=%q mode=%q floor=%q event=%s: got %+v, want %+v", cases, filter, mode, floor, event, got, want)
						}
						for i := range got {
							if got[i] != want[i] {
								t.Fatalf("case %d changed occurrence pointer or order at %d", cases, i)
							}
						}
						after, err := json.Marshal([]interface{}{members, filter})
						if err != nil || string(after) != string(before) {
							t.Fatalf("case %d changed the caller's filter or alerts: %v", cases, err)
						}
						cases++
					}
				}
			}
		}
	}
	t.Logf("%d recipient, shape and input-ownership comparisons", cases)
}

func TestNotificationBatchRoutingSharedInputs(t *testing.T) {
	list := notificationBatchFixture(64)
	filter := []string{" CUSTOMER:ALPHA ", "CRITICAL", "critical"}
	want := append([]string(nil), filter...)
	var workers sync.WaitGroup
	for range 4 {
		workers.Go(func() {
			for range 20 {
				got := routeNotificationAlerts(list, filter, " ALL ", "warning", eventAlert)
				if len(got) != len(list) {
					t.Errorf("shared filter lost recipients: %d, want %d", len(got), len(list))
					return
				}
				for i := range got {
					if got[i] != list[i] {
						t.Errorf("changed shared occurrence at %d", i)
						return
					}
				}
			}
		})
	}
	workers.Wait()
	if !reflect.DeepEqual(filter, want) {
		t.Fatalf("changed shared filter: %v, want %v", filter, want)
	}
}

var notificationBatchSink []*alerts.Alert

func TestNotificationBatchRoutingAllocations(t *testing.T) {
	list := notificationBatchFixture(256)
	filter := []string{"CUSTOMER:ALPHA", "production", "critical", "eu-west"}
	for _, floor := range []string{"all", "critical"} {
		t.Run(floor, func(t *testing.T) {
			before := testing.AllocsPerRun(3, func() {
				notificationBatchSink = referenceNotificationBatch(list, filter, "all", floor, eventAlert)
			})
			after := testing.AllocsPerRun(3, func() {
				notificationBatchSink = routeNotificationAlerts(list, filter, "all", floor, eventAlert)
			})
			t.Logf("per 256-alert route: reference %.0f allocs; current %.0f allocs", before, after)
			// Re-preparing a destination's filter must not add at least one
			// allocation per alert. This is structural, not a timing threshold.
			if after+float64(len(list)) > before {
				t.Fatalf("batch routing still prepares policy per alert: %.0f allocs, reference %.0f", after, before)
			}
		})
	}
}

func BenchmarkNotificationBatchRouting(b *testing.B) {
	for _, tc := range []struct {
		name   string
		size   int
		filter []string
		mode   string
		floor  string
		event  notificationEvent
	}{
		{"single", 1, []string{"CUSTOMER:ALPHA", "production", "critical", "eu-west"}, "all", "all", eventAlert},
		{"all-100", 100, []string{"CUSTOMER:ALPHA", "production", "critical", "eu-west"}, "all", "all", eventAlert},
		{"all-1000", 1000, []string{"CUSTOMER:ALPHA", "production", "critical", "eu-west"}, "all", "all", eventAlert},
		{"any-1000", 1000, []string{"missing", "CUSTOMER:ALPHA"}, " ANY ", "warning", eventAlert},
		{"severity-reject-1000", 1000, []string{"CUSTOMER:ALPHA", "production", "critical", "eu-west"}, "all", "critical", eventAlert},
		{"global-1000", 1000, nil, "all", "all", eventAlert},
		{"resolved-1000", 1000, []string{"missing"}, "all", "critical", eventResolved},
	} {
		b.Run(tc.name, func(b *testing.B) {
			list := notificationBatchFixture(tc.size)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				notificationBatchSink = routeNotificationAlerts(list, tc.filter, tc.mode, tc.floor, tc.event)
			}
		})
	}
}
