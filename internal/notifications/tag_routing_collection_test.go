package notifications

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

// Freeze the old collection algorithm independently of the production matcher.
// In particular, a string is delimited, but a string inside either array shape
// is one literal tag. Identity is ToLower, not Unicode EqualFold.
func referenceNotificationCollectedTags(alert *alerts.Alert) []string {
	if alert == nil || alert.Metadata == nil {
		return nil
	}
	var values []string
	for _, key := range []string{"tags", "resourceTags", "hostTags", "serviceTags"} {
		switch value := alert.Metadata[key].(type) {
		case string:
			values = append(values, strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == ';' })...)
		case []string:
			values = append(values, value...)
		case []interface{}:
			for _, item := range value {
				if text, ok := item.(string); ok {
					values = append(values, text)
				}
			}
		}
	}
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		key := strings.ToLower(value)
		if value == "" {
			continue
		}
		if _, ok := seen[key]; !ok {
			seen[key] = struct{}{}
			result = append(result, value)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func TestNotificationTagCollectionShapes(t *testing.T) {
	for _, tc := range []struct {
		name   string
		alert  *alerts.Alert
		filter []string
		mode   string
		want   bool
	}{
		{"nil-empty", nil, nil, "all", true},
		{"nil-filtered", nil, []string{"alpha"}, "any", false},
		{"nil-metadata", &alerts.Alert{}, []string{"alpha"}, "all", false},
		{"empty-normalized-filter", &alerts.Alert{}, []string{"", "\u2003"}, "any", true},
		{"string-delimiters", taggedRoutingAlert("one", " ;ALPHA,, \u2003Beta;"), []string{"alpha", " BETA "}, "all", true},
		{"array-literal", taggedRoutingAlert("one", []string{"alpha,beta", " gamma;delta "}), []string{"alpha,beta", "gamma;delta"}, "all", true},
		{"array-not-split", taggedRoutingAlert("one", []string{"alpha,beta"}), []string{"alpha"}, "any", false},
		{"mixed-array-literal", taggedRoutingAlert("one", []interface{}{nil, 7, " alpha;beta ", []string{"gamma"}}), []string{"alpha;beta"}, "all", true},
		{"mixed-array-not-recursive", taggedRoutingAlert("one", []interface{}{[]string{"alpha"}, map[string]string{"tag": "alpha"}}), []string{"alpha"}, "any", false},
		{"unsupported", taggedRoutingAlert("one", map[string]string{"alpha": "beta"}), []string{"alpha"}, "all", false},
		{"no-wildcards", taggedRoutingAlert("one", "alpha"), []string{"a*"}, "any", false},
		{"literal-wildcard", taggedRoutingAlert("one", "a*"), []string{"a*"}, "all", true},
		{"duplicate-filter", taggedRoutingAlert("one", "alpha;ALPHA"), []string{"alpha", " ALPHA "}, "all", true},
		{"default-all", taggedRoutingAlert("one", "alpha"), []string{"alpha", "beta"}, "unknown", false},
		{"normalized-any", taggedRoutingAlert("one", "alpha"), []string{"missing", " ALPHA "}, " ANY ", true},
		{"sigma-lower", taggedRoutingAlert("one", "Σ"), []string{"σ"}, "all", true},
		{"sigma-not-final", taggedRoutingAlert("one", "Σ"), []string{"ς"}, "any", false},
		{"long-s-not-ascii", taggedRoutingAlert("one", "ſ"), []string{"S"}, "any", false},
		{"kelvin-lower", taggedRoutingAlert("one", "K"), []string{"K"}, "all", true},
		{"invalid-utf8", taggedRoutingAlert("one", "\xffALPHA;β"), []string{"\xffalpha", "Β"}, "all", true},
		{"nul-is-not-space", taggedRoutingAlert("one", "\x00alpha"), []string{"alpha"}, "any", false},
		{"metadata-key-is-exact", &alerts.Alert{Metadata: map[string]interface{}{"Tags": "alpha", "tag": "alpha"}}, []string{"alpha"}, "any", false},
		{"cross-source-all", &alerts.Alert{Metadata: map[string]interface{}{
			"tags": " ALPHA,alpha ", "resourceTags": []string{"beta"},
			"hostTags": []interface{}{" gamma ", false}, "serviceTags": "delta;epsilon",
		}}, []string{"alpha", "BETA", "gamma", "delta", "epsilon"}, "all", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if want := referenceNotificationTagMatch(tc.alert, tc.filter, tc.mode); want != tc.want {
				t.Fatalf("frozen reference = %t, explicit expectation = %t", want, tc.want)
			}
			if got := notificationAlertMatchesTags(tc.alert, tc.filter, tc.mode); got != tc.want {
				t.Fatalf("match = %t, want %t", got, tc.want)
			}
		})
	}
}

func TestNotificationTagCollectionGenerated(t *testing.T) {
	rng := rand.New(rand.NewSource(914_523))
	atoms := []string{"", " ", "ALPHA", "alpha", " Beta ", "a,b", "c;d", "\u2003prod\u00a0", "Σ", "σ", "ς", "ſ", "S", "K", "K", "\xffA", "\x00tag", "a*"}
	filters := [][]string{nil, {""}, {"alpha"}, {"beta", "prod"}, {"ALPHA", " alpha "}, {"a,b"}, {"a", "b"}, {"c;d"}, {"Σ"}, {"ς"}, {"S"}, {"K"}, {"a*"}, {"missing"}}
	comparisons := 0
	for i := range 320 {
		metadata := make(map[string]interface{})
		for _, key := range []string{"tags", "resourceTags", "hostTags", "serviceTags", "ignored"} {
			items := make([]string, rng.Intn(12))
			for j := range items {
				items[j] = atoms[rng.Intn(len(atoms))]
			}
			switch rng.Intn(6) {
			case 0:
				metadata[key] = strings.Join(items, ";,")
			case 1:
				metadata[key] = items
			case 2:
				mixed := []interface{}{7, nil, false, []string{"missing"}}
				for _, item := range items {
					mixed = append(mixed, item)
				}
				metadata[key] = mixed
			case 3:
				metadata[key] = 7
			case 4:
				metadata[key] = nil
			}
		}
		alert := &alerts.Alert{ID: fmt.Sprintf("generated-%d", i), Metadata: metadata}
		before, err := json.Marshal(metadata)
		if err != nil {
			t.Fatal(err)
		}
		for _, filter := range filters {
			original := append([]string(nil), filter...)
			for _, mode := range []string{"all", " ANY ", "invalid"} {
				want := referenceNotificationTagMatch(alert, filter, mode)
				if got := notificationAlertMatchesTags(alert, filter, mode); got != want {
					t.Fatalf("case %d filter=%q mode=%s match=%t want=%t", i, filter, mode, got, want)
				}
				comparisons++
			}
			if !reflect.DeepEqual(original, filter) {
				t.Fatalf("case %d mutated filter", i)
			}
		}
		after, err := json.Marshal(metadata)
		if err != nil || string(before) != string(after) {
			t.Fatalf("case %d mutated alert metadata: %v", i, err)
		}
	}
	t.Logf("%d deterministic metadata/filter/mode comparisons", comparisons)
}

func TestNotificationTagCollectionReobservesInputs(t *testing.T) {
	metadata := map[string]interface{}{"tags": []string{"alpha"}}
	alert := &alerts.Alert{ID: "same-occurrence", Metadata: metadata, Level: alerts.AlertLevelWarning}
	filter := []string{"ALPHA"}
	for _, tags := range []interface{}{[]string{"alpha"}, "beta", []interface{}{" alpha ", false}, nil, []string{"alpha;beta"}} {
		metadata["tags"] = tags
		want := referenceNotificationBatch([]*alerts.Alert{alert}, filter, "all", "warning", eventAlert)
		got := routeNotificationAlerts([]*alerts.Alert{alert}, filter, "all", "warning", eventAlert)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("reobserved %v: got %d alerts, want %d", tags, len(got), len(want))
		}
	}
	metadata["tags"] = "alpha"
	filter[0] = "BETA"
	if got := routeNotificationAlerts([]*alerts.Alert{alert}, filter, "all", "warning", eventAlert); len(got) != 0 {
		t.Fatal("retained an earlier destination filter")
	}
	// Recovery admission is still receipt-owned, regardless of current tags.
	if got := routeNotificationAlerts([]*alerts.Alert{alert}, filter, "all", "critical", eventResolved); len(got) != 1 || got[0] != alert {
		t.Fatal("current tags or severity suppressed recovery before its receipt check")
	}
}

var notificationTagCollectionSink bool

func notificationTagCollectionLargeFixture(asString bool) *alerts.Alert {
	tags := make([]string, 128)
	for i := range tags {
		tags[i] = fmt.Sprintf(" Rack:%03d ", i)
	}
	var raw interface{} = tags
	if asString {
		raw = strings.Join(tags, ";,")
	}
	return taggedRoutingAlert("large", raw)
}

func BenchmarkNotificationTagCollection(b *testing.B) {
	for _, tc := range []struct {
		name   string
		alert  *alerts.Alert
		filter []string
		mode   string
	}{
		{"mixed-all", notificationBatchFixture(1)[0], []string{"customer:alpha", "production", "critical", "eu-west"}, "all"},
		{"mixed-any", notificationBatchFixture(1)[0], []string{"missing", "customer:alpha"}, "any"},
		{"array-128", notificationTagCollectionLargeFixture(false), []string{"rack:127"}, "all"},
		{"string-128", notificationTagCollectionLargeFixture(true), []string{"rack:127"}, "all"},
		{"untagged", &alerts.Alert{}, []string{"alpha"}, "all"},
		{"unsupported", taggedRoutingAlert("bad-shape", 7), []string{"alpha"}, "all"},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				notificationTagCollectionSink = notificationAlertMatchesPreparedTags(tc.alert, tc.filter, tc.mode)
			}
		})
	}
}

var notificationTagCollectionJobsSink []notificationDeliveryJob

func BenchmarkNotificationTagCollectionJobs(b *testing.B) {
	for _, size := range []int{1, 100, 1000} {
		b.Run(fmt.Sprintf("alerts-%d", size), func(b *testing.B) {
			list := notificationBatchFixture(size)
			email := EmailConfig{Enabled: true, TagFilter: []string{"customer:alpha", "production"}, MinimumSeverity: "warning"}
			hooks := []WebhookConfig{
				{ID: "matching-all", Enabled: true, TagFilter: []string{"customer:alpha", "critical"}},
				{ID: "matching-any", Enabled: true, TagFilter: []string{"missing", "eu-west"}, TagMode: "any"},
				{ID: "missing", Enabled: true, TagFilter: []string{"absent"}},
				{ID: "severity-reject", Enabled: true, TagFilter: []string{"customer:alpha"}, MinimumSeverity: "critical"},
				{ID: "global", Enabled: true},
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				notificationTagCollectionJobsSink = buildNotificationDeliveryJobs(email, hooks, AppriseConfig{}, list, eventAlert, time.Time{})
			}
		})
	}
}
