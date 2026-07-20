package config

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

// This file exercises every branch of the alert-intent policy helpers in
// internal/alerts/config/intent.go. It deliberately uses the in-package test
// package (package config, not config_test like the sibling files) because the
// target list includes unexported functions (normalizeIntentScopedRules,
// normalizeIntentSignalRules, cloneAlertIntentRule, validateIntentScopedRules,
// validateIntentSignalRules, validAlertIntentSignal) that cannot be referenced
// from config_test. Go compiles in-package and external test files together.
//
// Test function names embed "Intent" so `-run Intent` selects them in
// isolation. Each branch gets a dedicated sub-test that asserts the exact
// error message substring produced by the code under test.

// intPtr/boolPtr allocate fresh pointers so clone-independence assertions can
// distinguish source values from cloned values.
func intPtr(v int) *int                                                { return &v }
func boolPtr(v bool) *bool                                             { return &v }
func backupPtr(b BackupOfflineIntentPolicy) *BackupOfflineIntentPolicy { return &b }

// TestIntentNewPolicyDocumentDefaults covers NewAlertIntentPolicyDocument.
func TestIntentNewPolicyDocumentDefaults(t *testing.T) {
	doc := NewAlertIntentPolicyDocument()
	if doc.SchemaVersion != CurrentAlertIntentPolicySchemaVersion {
		t.Fatalf("SchemaVersion = %d, want %d", doc.SchemaVersion, CurrentAlertIntentPolicySchemaVersion)
	}
	if doc.Revision != 0 {
		t.Fatalf("Revision = %d, want 0", doc.Revision)
	}
	if doc.UpdatedAt != nil {
		t.Fatalf("UpdatedAt = %v, want nil", doc.UpdatedAt)
	}
	if doc.Defaults == nil {
		t.Fatal("Defaults is nil, want initialized non-nil map")
	}
	if len(doc.Defaults) != 0 {
		t.Fatalf("len(Default) = %d, want 0", len(doc.Defaults))
	}
	if doc.ResourceTypes == nil {
		t.Fatal("ResourceTypes is nil, want initialized non-nil map")
	}
	if len(doc.ResourceTypes) != 0 {
		t.Fatalf("len(ResourceTypes) = %d, want 0", len(doc.ResourceTypes))
	}
	if doc.Resources == nil {
		t.Fatal("Resources is nil, want initialized non-nil map")
	}
	if len(doc.Resources) != 0 {
		t.Fatalf("len(Resources) = %d, want 0", len(doc.Resources))
	}
}

// TestIntentMetricSignal covers MetricAlertIntentSignal: empty/whitespace
// collapse to "" and the trim+lowercase+"metric." prefix path.
func TestIntentMetricSignal(t *testing.T) {
	tests := []struct {
		name   string
		metric string
		want   string
	}{
		{name: "empty returns empty", metric: "", want: ""},
		{name: "spaces only returns empty", metric: "   ", want: ""},
		{name: "tabs and newlines only returns empty", metric: "\t \n", want: ""},
		{name: "uppercase lowercased and prefixed", metric: "CPU", want: "metric.cpu"},
		{name: "surrounding whitespace trimmed", metric: "  CPU  ", want: "metric.cpu"},
		{name: "mixed case and dots preserved except case", metric: "Cpu.Usage", want: "metric.cpu.usage"},
		{name: "inner space preserved through lowercase", metric: "CPU LOAD", want: "metric.cpu load"},
		{name: "leading tab trimmed", metric: "\tmetric Already", want: "metric.metric already"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := MetricAlertIntentSignal(tc.metric)
			if got != tc.want {
				t.Fatalf("MetricAlertIntentSignal(%q) = %q, want %q", tc.metric, got, tc.want)
			}
		})
	}
}

// TestIntentNormalizeDocument covers NormalizeAlertIntentPolicyDocument.
func TestIntentNormalizeDocument(t *testing.T) {
	t.Run("zero value input yields defaults", func(t *testing.T) {
		out := NormalizeAlertIntentPolicyDocument(AlertIntentPolicyDocument{})
		if out.SchemaVersion != CurrentAlertIntentPolicySchemaVersion {
			t.Fatalf("SchemaVersion = %d, want default %d", out.SchemaVersion, CurrentAlertIntentPolicySchemaVersion)
		}
		if out.Revision != 0 {
			t.Fatalf("Revision = %d, want 0", out.Revision)
		}
		if out.UpdatedAt != nil {
			t.Fatalf("UpdatedAt = %v, want nil", out.UpdatedAt)
		}
		if out.Defaults == nil || len(out.Defaults) != 0 {
			t.Fatalf("Defaults = %#v, want non-nil empty map", out.Defaults)
		}
		if out.ResourceTypes == nil || len(out.ResourceTypes) != 0 {
			t.Fatalf("ResourceTypes = %#v, want non-nil empty map", out.ResourceTypes)
		}
		if out.Resources == nil || len(out.Resources) != 0 {
			t.Fatalf("Resources = %#v, want non-nil empty map", out.Resources)
		}
	})

	t.Run("positive schema version preserved", func(t *testing.T) {
		out := NormalizeAlertIntentPolicyDocument(AlertIntentPolicyDocument{SchemaVersion: 7})
		if out.SchemaVersion != 7 {
			t.Fatalf("SchemaVersion = %d, want 7", out.SchemaVersion)
		}
	})

	t.Run("zero schema version falls back to default", func(t *testing.T) {
		out := NormalizeAlertIntentPolicyDocument(AlertIntentPolicyDocument{SchemaVersion: 0})
		if out.SchemaVersion != CurrentAlertIntentPolicySchemaVersion {
			t.Fatalf("SchemaVersion = %d, want default %d", out.SchemaVersion, CurrentAlertIntentPolicySchemaVersion)
		}
	})

	t.Run("negative schema version falls back to default", func(t *testing.T) {
		out := NormalizeAlertIntentPolicyDocument(AlertIntentPolicyDocument{SchemaVersion: -3})
		if out.SchemaVersion != CurrentAlertIntentPolicySchemaVersion {
			t.Fatalf("SchemaVersion = %d, want default %d (negative not > 0)", out.SchemaVersion, CurrentAlertIntentPolicySchemaVersion)
		}
	})

	t.Run("revision preserved including negative value", func(t *testing.T) {
		out := NormalizeAlertIntentPolicyDocument(AlertIntentPolicyDocument{Revision: 42})
		if out.Revision != 42 {
			t.Fatalf("Revision = %d, want 42", out.Revision)
		}
		outNeg := NormalizeAlertIntentPolicyDocument(AlertIntentPolicyDocument{Revision: -5})
		if outNeg.Revision != -5 {
			t.Fatalf("Revision = %d, want -5 (normalizer does not clamp)", outNeg.Revision)
		}
	})

	t.Run("nil updatedAt stays nil", func(t *testing.T) {
		out := NormalizeAlertIntentPolicyDocument(AlertIntentPolicyDocument{})
		if out.UpdatedAt != nil {
			t.Fatalf("UpdatedAt = %v, want nil", out.UpdatedAt)
		}
	})

	t.Run("non-nil updatedAt is copied and converted to UTC", func(t *testing.T) {
		loc, err := time.LoadLocation("America/New_York")
		if err != nil {
			t.Fatalf("load location: %v", err)
		}
		src := time.Date(2024, 6, 15, 12, 0, 0, 0, loc) // EDT, UTC-4
		in := AlertIntentPolicyDocument{UpdatedAt: &src}
		out := NormalizeAlertIntentPolicyDocument(in)
		if out.UpdatedAt == nil {
			t.Fatal("UpdatedAt = nil, want non-nil")
		}
		wantUTC := src.UTC()
		if !out.UpdatedAt.Equal(wantUTC) {
			t.Fatalf("UpdatedAt = %v, want %v", out.UpdatedAt, wantUTC)
		}
		if out.UpdatedAt.Location() != time.UTC {
			t.Fatalf("UpdatedAt location = %v, want UTC", out.UpdatedAt.Location())
		}
		// Detachment: mutating out.UpdatedAt must not affect the source.
		original := src
		*out.UpdatedAt = original.Add(time.Hour)
		if !src.Equal(original) {
			t.Fatalf("mutating out.UpdatedAt affected source time: src=%v want=%v (not detached)", src, original)
		}
	})

	t.Run("defaults normalized end to end", func(t *testing.T) {
		in := AlertIntentPolicyDocument{
			Defaults: map[string]AlertIntentRule{
				"  DEFAULT  ":   {GraceSeconds: intPtr(30)},
				"":              {GraceSeconds: intPtr(40)}, // dropped (empty signal)
				"state.offline": {GraceSeconds: intPtr(50)},
			},
		}
		out := NormalizeAlertIntentPolicyDocument(in)
		if _, ok := out.Defaults["*"]; !ok {
			t.Fatalf("Defaults = %#v, want '*' key (from DEFAULT normalization)", out.Defaults)
		}
		if _, ok := out.Defaults["state.offline"]; !ok {
			t.Fatalf("Defaults = %#v, want state.offline key preserved", out.Defaults)
		}
		if len(out.Defaults) != 2 {
			t.Fatalf("len(Default) = %d, want 2 (empty signal dropped)", len(out.Defaults))
		}
	})

	t.Run("resource types canonicalized and empty scopes dropped", func(t *testing.T) {
		in := AlertIntentPolicyDocument{
			ResourceTypes: map[string]map[string]AlertIntentRule{
				"  VMWARE Host  ": {"state.offline": {GraceSeconds: intPtr(15)}},
				"   ":             {"state.offline": {GraceSeconds: intPtr(20)}}, // whitespace scope dropped
			},
		}
		out := NormalizeAlertIntentPolicyDocument(in)
		if _, ok := out.ResourceTypes["vmware-host"]; !ok {
			t.Fatalf("ResourceTypes = %#v, want 'vmware-host' canonical key", out.ResourceTypes)
		}
		if len(out.ResourceTypes) != 1 {
			t.Fatalf("len(ResourceTypes) = %d, want 1 (whitespace scope dropped)", len(out.ResourceTypes))
		}
	})

	t.Run("resources preserve raw trimmed scope without canonicalization", func(t *testing.T) {
		in := AlertIntentPolicyDocument{
			Resources: map[string]map[string]AlertIntentRule{
				"  node/qemu/100  ": {"state.offline": {GraceSeconds: intPtr(25)}},
			},
		}
		out := NormalizeAlertIntentPolicyDocument(in)
		if _, ok := out.Resources["node/qemu/100"]; !ok {
			t.Fatalf("Resources = %#v, want trimmed-but-not-canonicalized 'node/qemu/100' key", out.Resources)
		}
		if len(out.Resources) != 1 {
			t.Fatalf("len(Resources) = %d, want 1", len(out.Resources))
		}
	})

	t.Run("scope whose rules normalize to empty is dropped", func(t *testing.T) {
		in := AlertIntentPolicyDocument{
			ResourceTypes: map[string]map[string]AlertIntentRule{
				"guest": {"   ": {GraceSeconds: intPtr(1)}}, // signal empty after trim
			},
		}
		out := NormalizeAlertIntentPolicyDocument(in)
		if len(out.ResourceTypes) != 0 {
			t.Fatalf("ResourceTypes = %#v, want empty (normalized rules empty -> scope dropped)", out.ResourceTypes)
		}
	})
}

// TestIntentNormalizeSignalRules covers normalizeIntentSignalRules directly:
// nil/empty input, alias collapsing, whitespace/empty drop, lowercasing, and
// clone independence.
func TestIntentNormalizeSignalRules(t *testing.T) {
	t.Run("nil input returns empty non-nil map", func(t *testing.T) {
		out := normalizeIntentSignalRules(nil)
		if out == nil {
			t.Fatal("got nil map, want non-nil empty map")
		}
		if len(out) != 0 {
			t.Fatalf("len = %d, want 0", len(out))
		}
	})

	t.Run("empty input returns empty non-nil map", func(t *testing.T) {
		out := normalizeIntentSignalRules(map[string]AlertIntentRule{})
		if out == nil {
			t.Fatal("got nil map, want non-nil empty map")
		}
		if len(out) != 0 {
			t.Fatalf("len = %d, want 0", len(out))
		}
	})

	t.Run("already normal keys preserved", func(t *testing.T) {
		in := map[string]AlertIntentRule{
			"*":                     {GraceSeconds: intPtr(1)},
			"state.offline":         {GraceSeconds: intPtr(2)},
			"incident.availability": {GraceSeconds: intPtr(3)},
			"metric.cpu":            {GraceSeconds: intPtr(4)},
		}
		out := normalizeIntentSignalRules(in)
		if len(out) != 4 {
			t.Fatalf("len = %d, want 4", len(out))
		}
		want := map[string]int{"*": 1, "state.offline": 2, "incident.availability": 3, "metric.cpu": 4}
		for k, w := range want {
			r, ok := out[k]
			if !ok {
				t.Fatalf("missing key %q in %#v", k, out)
			}
			if r.GraceSeconds == nil || *r.GraceSeconds != w {
				t.Fatalf("out[%q].GraceSeconds = %v, want %d", k, r.GraceSeconds, w)
			}
		}
	})

	t.Run("default and _default aliases collapse to star", func(t *testing.T) {
		in := map[string]AlertIntentRule{
			"default":    {GraceSeconds: intPtr(10)},
			"_default":   {GraceSeconds: intPtr(20)},
			"DEFAULT":    {GraceSeconds: intPtr(30)},
			" _Default ": {GraceSeconds: intPtr(40)},
		}
		out := normalizeIntentSignalRules(in)
		if len(out) != 1 {
			t.Fatalf("len = %d, want 1 (all collapse to '*')", len(out))
		}
		if _, ok := out["*"]; !ok {
			t.Fatalf("out = %#v, want single '*' key", out)
		}
	})

	t.Run("whitespace and empty signals dropped", func(t *testing.T) {
		in := map[string]AlertIntentRule{
			"":         {GraceSeconds: intPtr(1)},
			"   ":      {GraceSeconds: intPtr(2)},
			"\t\n":     {GraceSeconds: intPtr(3)},
			"metric.x": {GraceSeconds: intPtr(4)},
		}
		out := normalizeIntentSignalRules(in)
		if len(out) != 1 {
			t.Fatalf("len = %d, want 1 (only metric.x survives)", len(out))
		}
		if _, ok := out["metric.x"]; !ok {
			t.Fatalf("out = %#v, want metric.x", out)
		}
	})

	t.Run("uppercase signal keys lowercased", func(t *testing.T) {
		in := map[string]AlertIntentRule{
			"State.Offline": {GraceSeconds: intPtr(7)},
		}
		out := normalizeIntentSignalRules(in)
		if _, ok := out["state.offline"]; !ok {
			t.Fatalf("out = %#v, want 'state.offline' (lowercased)", out)
		}
	})

	t.Run("cloned rule pointer fields are independent", func(t *testing.T) {
		in := map[string]AlertIntentRule{
			"state.offline": {GraceSeconds: intPtr(99)},
		}
		out := normalizeIntentSignalRules(in)
		r := out["state.offline"]
		if r.GraceSeconds == nil {
			t.Fatal("cloned GraceSeconds is nil")
		}
		*r.GraceSeconds = 0
		if *in["state.offline"].GraceSeconds != 99 {
			t.Fatalf("mutating clone affected source: source = %d, want 99", *in["state.offline"].GraceSeconds)
		}
	})
}

// TestIntentNormalizeScopedRules covers normalizeIntentScopedRules directly: nil
// and empty inputs, whitespace scope drop (both lowerScope arms), canonical
// vs trim-only scope handling, and drop of scopes whose normalized rules are
// empty.
func TestIntentNormalizeScopedRules(t *testing.T) {
	t.Run("nil input returns empty non-nil map", func(t *testing.T) {
		out := normalizeIntentScopedRules(nil, true)
		if out == nil {
			t.Fatal("got nil, want non-nil empty map")
		}
		if len(out) != 0 {
			t.Fatalf("len = %d, want 0", len(out))
		}
	})

	t.Run("empty input returns empty non-nil map", func(t *testing.T) {
		out := normalizeIntentScopedRules(map[string]map[string]AlertIntentRule{}, false)
		if out == nil {
			t.Fatal("got nil, want non-nil empty map")
		}
		if len(out) != 0 {
			t.Fatalf("len = %d, want 0", len(out))
		}
	})

	t.Run("whitespace scope dropped when lowerScope true", func(t *testing.T) {
		in := map[string]map[string]AlertIntentRule{
			"   ": {"state.offline": {GraceSeconds: intPtr(1)}},
		}
		out := normalizeIntentScopedRules(in, true)
		if len(out) != 0 {
			t.Fatalf("len = %d, want 0 (whitespace scope dropped)", len(out))
		}
	})

	t.Run("whitespace scope dropped when lowerScope false", func(t *testing.T) {
		in := map[string]map[string]AlertIntentRule{
			"   ": {"state.offline": {GraceSeconds: intPtr(1)}},
		}
		out := normalizeIntentScopedRules(in, false)
		if len(out) != 0 {
			t.Fatalf("len = %d, want 0 (whitespace scope dropped)", len(out))
		}
	})

	t.Run("lowerScope true applies resource type canonicalization", func(t *testing.T) {
		in := map[string]map[string]AlertIntentRule{
			"  VMWARE Host  ": {"state.offline": {GraceSeconds: intPtr(1)}},
			"Guest":           {"state.offline": {GraceSeconds: intPtr(2)}},
		}
		out := normalizeIntentScopedRules(in, true)
		if _, ok := out["vmware-host"]; !ok {
			t.Fatalf("out = %#v, want 'vmware-host' canonical key", out)
		}
		if _, ok := out["guest"]; !ok {
			t.Fatalf("out = %#v, want 'guest' (lowercased default arm)", out)
		}
	})

	t.Run("lowerScope false only trims without lowercasing", func(t *testing.T) {
		in := map[string]map[string]AlertIntentRule{
			"  Node/QEMU/100  ": {"state.offline": {GraceSeconds: intPtr(1)}},
		}
		out := normalizeIntentScopedRules(in, false)
		if _, ok := out["Node/QEMU/100"]; !ok {
			t.Fatalf("out = %#v, want trimmed but case-preserved key", out)
		}
		if _, ok := out["node/qemu/100"]; ok {
			t.Fatalf("out = %#v, did not expect lowercased key", out)
		}
	})

	t.Run("scope with empty normalized rules is dropped", func(t *testing.T) {
		in := map[string]map[string]AlertIntentRule{
			"guest": {"   ": {GraceSeconds: intPtr(1)}},
		}
		out := normalizeIntentScopedRules(in, true)
		if len(out) != 0 {
			t.Fatalf("out = %#v, want empty (normalized rules empty -> scope dropped)", out)
		}
	})

	t.Run("multiple scopes with mixed survival", func(t *testing.T) {
		in := map[string]map[string]AlertIntentRule{
			"Guest": {"state.offline": {GraceSeconds: intPtr(1)}},
			"   ":   {"state.offline": {GraceSeconds: intPtr(2)}},
			"Empty": {"   ": {GraceSeconds: intPtr(3)}},
		}
		out := normalizeIntentScopedRules(in, true)
		if len(out) != 1 {
			t.Fatalf("out = %#v, want only 'guest' to survive", out)
		}
		if _, ok := out["guest"]; !ok {
			t.Fatalf("out = %#v, want 'guest' key", out)
		}
	})
}

// TestIntentCloneRule covers cloneAlertIntentRule: zero-value, fully-populated
// deep equality, mutation-independence for every pointer field, and nil/empty
// handling.
func TestIntentCloneRule(t *testing.T) {
	t.Run("zero value rule clones cleanly with nil pointers", func(t *testing.T) {
		r := AlertIntentRule{}
		c := cloneAlertIntentRule(r)
		if !reflect.DeepEqual(c, r) {
			t.Fatalf("clone = %#v, want %#v", c, r)
		}
		if c.GraceSeconds != nil || c.HonorOperatorState != nil || c.BackupOffline != nil {
			t.Fatalf("clone pointers non-nil: %+v", c)
		}
	})

	t.Run("fully populated rule clones with deep equality", func(t *testing.T) {
		r := AlertIntentRule{
			GraceSeconds:       intPtr(42),
			HonorOperatorState: boolPtr(true),
			BackupOffline: backupPtr(BackupOfflineIntentPolicy{
				Enabled:            true,
				PostGraceSeconds:   10,
				MaxDeferralSeconds: 20,
			}),
		}
		c := cloneAlertIntentRule(r)
		if !reflect.DeepEqual(c, r) {
			t.Fatalf("clone = %#v, want %#v", c, r)
		}
	})

	t.Run("mutating cloned GraceSeconds does not affect source", func(t *testing.T) {
		r := AlertIntentRule{GraceSeconds: intPtr(5)}
		c := cloneAlertIntentRule(r)
		*c.GraceSeconds = 999
		if *r.GraceSeconds != 5 {
			t.Fatalf("source mutated: %d, want 5", *r.GraceSeconds)
		}
	})

	t.Run("mutating cloned HonorOperatorState does not affect source", func(t *testing.T) {
		r := AlertIntentRule{HonorOperatorState: boolPtr(false)}
		c := cloneAlertIntentRule(r)
		*c.HonorOperatorState = true
		if *r.HonorOperatorState != false {
			t.Fatalf("source mutated: %v, want false", *r.HonorOperatorState)
		}
	})

	t.Run("mutating cloned BackupOffline fields does not affect source", func(t *testing.T) {
		r := AlertIntentRule{BackupOffline: &BackupOfflineIntentPolicy{
			Enabled: true, PostGraceSeconds: 1, MaxDeferralSeconds: 2,
		}}
		c := cloneAlertIntentRule(r)
		c.BackupOffline.Enabled = false
		c.BackupOffline.PostGraceSeconds = 100
		c.BackupOffline.MaxDeferralSeconds = 200
		if !r.BackupOffline.Enabled || r.BackupOffline.PostGraceSeconds != 1 || r.BackupOffline.MaxDeferralSeconds != 2 {
			t.Fatalf("source mutated: %+v, want unchanged", r.BackupOffline)
		}
	})

	t.Run("partial population only clones set fields", func(t *testing.T) {
		r := AlertIntentRule{GraceSeconds: intPtr(7)}
		c := cloneAlertIntentRule(r)
		if c.GraceSeconds == nil || *c.GraceSeconds != 7 {
			t.Fatalf("GraceSeconds = %v, want 7", c.GraceSeconds)
		}
		if c.HonorOperatorState != nil {
			t.Fatalf("HonorOperatorState = %v, want nil", c.HonorOperatorState)
		}
		if c.BackupOffline != nil {
			t.Fatalf("BackupOffline = %v, want nil", c.BackupOffline)
		}
	})

	t.Run("only HonorOperatorState set clones correctly", func(t *testing.T) {
		r := AlertIntentRule{HonorOperatorState: boolPtr(true)}
		c := cloneAlertIntentRule(r)
		if c.HonorOperatorState == nil || !*c.HonorOperatorState {
			t.Fatalf("HonorOperatorState = %v, want true", c.HonorOperatorState)
		}
		if c.GraceSeconds != nil || c.BackupOffline != nil {
			t.Fatalf("unexpected non-nil pointers: %+v", c)
		}
	})

	t.Run("only BackupOffline set clones correctly", func(t *testing.T) {
		r := AlertIntentRule{BackupOffline: &BackupOfflineIntentPolicy{Enabled: true, MaxDeferralSeconds: 30}}
		c := cloneAlertIntentRule(r)
		if c.BackupOffline == nil || !c.BackupOffline.Enabled || c.BackupOffline.MaxDeferralSeconds != 30 {
			t.Fatalf("BackupOffline = %+v, want preserved", c.BackupOffline)
		}
		if c.GraceSeconds != nil || c.HonorOperatorState != nil {
			t.Fatalf("unexpected non-nil pointers: %+v", c)
		}
	})
}

// TestIntentValidSignal covers every branch of validAlertIntentSignal: the
// three constant valid signals, the metric.* prefix path (with and without a
// non-empty suffix), and the unsupported default-arm path.
func TestIntentValidSignal(t *testing.T) {
	tests := []struct {
		name   string
		signal string
		want   bool
	}{
		{name: "default star is valid", signal: "*", want: true},
		{name: "offline is valid", signal: "state.offline", want: true},
		{name: "availability is valid", signal: "incident.availability", want: true},
		{name: "metric with name is valid", signal: "metric.cpu", want: true},
		{name: "metric with dotted name is valid", signal: "metric.cpu.load", want: true},
		{name: "metric with no name is invalid", signal: "metric.", want: false},
		{name: "metric prefix without dot is invalid", signal: "metric", want: false},
		{name: "empty is invalid", signal: "", want: false},
		{name: "unrelated state is invalid", signal: "state.online", want: false},
		{name: "case sensitive prefix Metric not metric", signal: "Metric.cpu", want: false},
		{name: "arbitrary string is invalid", signal: "foo", want: false},
		{name: "metric- no dot is invalid", signal: "metric-cpu", want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := validAlertIntentSignal(tc.signal); got != tc.want {
				t.Fatalf("validAlertIntentSignal(%q) = %v, want %v", tc.signal, got, tc.want)
			}
		})
	}
}

// TestIntentValidateDocument covers ValidateAlertIntentPolicyDocument end to
// end: the valid path and each distinct error branch.
func TestIntentValidateDocument(t *testing.T) {
	t.Run("valid minimal document returns nil", func(t *testing.T) {
		doc := NewAlertIntentPolicyDocument()
		if err := ValidateAlertIntentPolicyDocument(doc); err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
	})

	t.Run("valid populated document returns nil", func(t *testing.T) {
		doc := NewAlertIntentPolicyDocument()
		doc.Defaults["*"] = AlertIntentRule{GraceSeconds: intPtr(30)}
		doc.ResourceTypes["vmware-host"] = map[string]AlertIntentRule{
			"state.offline": {BackupOffline: &BackupOfflineIntentPolicy{
				Enabled: true, PostGraceSeconds: 5, MaxDeferralSeconds: 10,
			}},
		}
		doc.Resources["node/qemu/100"] = map[string]AlertIntentRule{
			"incident.availability": {GraceSeconds: intPtr(60)},
		}
		if err := ValidateAlertIntentPolicyDocument(doc); err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
	})

	t.Run("unsupported schema version returns specific error", func(t *testing.T) {
		doc := NewAlertIntentPolicyDocument()
		doc.SchemaVersion = 2
		err := ValidateAlertIntentPolicyDocument(doc)
		if err == nil {
			t.Fatal("err = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "unsupported alert intent policy schema version 2") {
			t.Fatalf("err = %q, want substring 'unsupported alert intent policy schema version 2'", err.Error())
		}
	})

	t.Run("zero schema version normalizes to current and is valid", func(t *testing.T) {
		doc := AlertIntentPolicyDocument{} // SchemaVersion 0
		if err := ValidateAlertIntentPolicyDocument(doc); err != nil {
			t.Fatalf("err = %v, want nil (zero schema normalizes to current)", err)
		}
	})

	t.Run("negative revision returns specific error", func(t *testing.T) {
		doc := NewAlertIntentPolicyDocument()
		doc.Revision = -1
		err := ValidateAlertIntentPolicyDocument(doc)
		if err == nil {
			t.Fatal("err = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "alert intent policy revision must be non-negative") {
			t.Fatalf("err = %q, want substring 'revision must be non-negative'", err.Error())
		}
	})

	t.Run("invalid defaults signal propagates error", func(t *testing.T) {
		doc := NewAlertIntentPolicyDocument()
		doc.Defaults["bogus-signal"] = AlertIntentRule{}
		err := ValidateAlertIntentPolicyDocument(doc)
		if err == nil {
			t.Fatal("err = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "unsupported alert intent signal") {
			t.Fatalf("err = %q, want substring 'unsupported alert intent signal'", err.Error())
		}
		if !strings.Contains(err.Error(), "defaults") {
			t.Fatalf("err = %q, want substring containing scope 'defaults'", err.Error())
		}
	})

	t.Run("invalid resource types scope propagates error", func(t *testing.T) {
		doc := NewAlertIntentPolicyDocument()
		doc.ResourceTypes["guest"] = map[string]AlertIntentRule{
			"": {GraceSeconds: intPtr(1)},
		}
		err := ValidateAlertIntentPolicyDocument(doc)
		if err == nil {
			t.Fatal("err = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "empty alert intent signal") {
			t.Fatalf("err = %q, want substring 'empty alert intent signal'", err.Error())
		}
	})

	t.Run("invalid resources scope propagates error", func(t *testing.T) {
		doc := NewAlertIntentPolicyDocument()
		doc.Resources["node/1"] = map[string]AlertIntentRule{
			"state.offline": {GraceSeconds: intPtr(-1)},
		}
		err := ValidateAlertIntentPolicyDocument(doc)
		if err == nil {
			t.Fatal("err = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "graceSeconds must be between") {
			t.Fatalf("err = %q, want substring about graceSeconds range", err.Error())
		}
	})
}

// TestIntentValidateScopedRules covers validateIntentScopedRules directly:
// nil/empty success, the empty-scope required error (both resourceType arms),
// the duplicate-canonical-collision error, valid distinct scopes, inner-rule
// error propagation, and trim-only behavior when resourceType is false.
func TestIntentValidateScopedRules(t *testing.T) {
	t.Run("nil scopes returns nil", func(t *testing.T) {
		if err := validateIntentScopedRules("resource type", nil, true); err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
	})

	t.Run("empty scopes returns nil", func(t *testing.T) {
		if err := validateIntentScopedRules("resource", map[string]map[string]AlertIntentRule{}, false); err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
	})

	t.Run("whitespace scope with resourceType true returns required error", func(t *testing.T) {
		scopes := map[string]map[string]AlertIntentRule{
			"   ": {"state.offline": {}},
		}
		err := validateIntentScopedRules("resource type", scopes, true)
		if err == nil {
			t.Fatal("err = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "alert intent resource type is required") {
			t.Fatalf("err = %q, want substring 'alert intent resource type is required'", err.Error())
		}
	})

	t.Run("whitespace scope with resourceType false returns required error", func(t *testing.T) {
		scopes := map[string]map[string]AlertIntentRule{
			"   ": {"state.offline": {}},
		}
		err := validateIntentScopedRules("resource", scopes, false)
		if err == nil {
			t.Fatal("err = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "alert intent resource is required") {
			t.Fatalf("err = %q, want substring 'alert intent resource is required'", err.Error())
		}
	})

	t.Run("duplicate canonicalized scopes returns collision error", func(t *testing.T) {
		scopes := map[string]map[string]AlertIntentRule{
			"vmware host": {"state.offline": {}},
			"VMWARE-HOST": {"state.offline": {}},
		}
		err := validateIntentScopedRules("resource type", scopes, true)
		if err == nil {
			t.Fatal("err = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "normalize to the same value") {
			t.Fatalf("err = %q, want substring 'normalize to the same value'", err.Error())
		}
	})

	t.Run("valid distinct canonical scopes returns nil", func(t *testing.T) {
		scopes := map[string]map[string]AlertIntentRule{
			"VMWARE Host": {"state.offline": {}},
			"guest":       {"*": {}},
		}
		if err := validateIntentScopedRules("resource type", scopes, true); err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
	})

	t.Run("inner rules error propagates with composed scope label", func(t *testing.T) {
		scopes := map[string]map[string]AlertIntentRule{
			"guest": {"bogus": {}},
		}
		err := validateIntentScopedRules("resource type", scopes, true)
		if err == nil {
			t.Fatal("err = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "resource type guest") {
			t.Fatalf("err = %q, want substring 'resource type guest'", err.Error())
		}
		if !strings.Contains(err.Error(), "unsupported alert intent signal") {
			t.Fatalf("err = %q, want substring 'unsupported alert intent signal'", err.Error())
		}
	})

	t.Run("resourceType false trims but does not lowercase so same-case duplicates collide", func(t *testing.T) {
		scopes := map[string]map[string]AlertIntentRule{
			" Node1 ": {"state.offline": {}},
			"Node1":   {"*": {}},
		}
		err := validateIntentScopedRules("resource", scopes, false)
		if err == nil {
			t.Fatal("err = nil, want non-nil (trim-only collision)")
		}
		if !strings.Contains(err.Error(), "normalize to the same value") {
			t.Fatalf("err = %q, want substring 'normalize to the same value'", err.Error())
		}
	})

	t.Run("resourceType false treats different casing as distinct scopes", func(t *testing.T) {
		scopes := map[string]map[string]AlertIntentRule{
			"Node1": {"state.offline": {}},
			"node1": {"*": {}},
		}
		if err := validateIntentScopedRules("resource", scopes, false); err != nil {
			t.Fatalf("err = %v, want nil (different casing = distinct without canonicalization)", err)
		}
	})
}

// TestIntentValidateSignalRules covers validateIntentSignalRules directly,
// asserting the valid path and each distinct invalid path with its specific
// error message substring.
func TestIntentValidateSignalRules(t *testing.T) {
	t.Run("nil rules returns nil", func(t *testing.T) {
		if err := validateIntentSignalRules("defaults", nil); err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
	})

	t.Run("empty rules returns nil", func(t *testing.T) {
		if err := validateIntentSignalRules("defaults", map[string]AlertIntentRule{}); err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
	})

	t.Run("valid rules across supported signals returns nil", func(t *testing.T) {
		rules := map[string]AlertIntentRule{
			"*":                     {GraceSeconds: intPtr(0)},
			"state.offline":         {GraceSeconds: intPtr(maxAlertIntentGraceSeconds)},
			"incident.availability": {},
			"metric.cpu":            {},
		}
		if err := validateIntentSignalRules("defaults", rules); err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
	})

	t.Run("empty signal returns empty-signal error", func(t *testing.T) {
		rules := map[string]AlertIntentRule{
			"   ": {},
		}
		err := validateIntentSignalRules("defaults", rules)
		if err == nil {
			t.Fatal("err = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "has an empty alert intent signal") {
			t.Fatalf("err = %q, want substring 'has an empty alert intent signal'", err.Error())
		}
	})

	t.Run("duplicate normalized signals returns collision error", func(t *testing.T) {
		rules := map[string]AlertIntentRule{
			"default":  {},
			"_DEFAULT": {},
		}
		err := validateIntentSignalRules("defaults", rules)
		if err == nil {
			t.Fatal("err = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "normalize to the same value") {
			t.Fatalf("err = %q, want substring 'normalize to the same value'", err.Error())
		}
	})

	t.Run("unsupported signal returns unsupported error", func(t *testing.T) {
		rules := map[string]AlertIntentRule{
			"state.online": {},
		}
		err := validateIntentSignalRules("defaults", rules)
		if err == nil {
			t.Fatal("err = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "unsupported alert intent signal") {
			t.Fatalf("err = %q, want substring 'unsupported alert intent signal'", err.Error())
		}
	})

	t.Run("metric prefix with no name is unsupported", func(t *testing.T) {
		rules := map[string]AlertIntentRule{
			"metric.": {},
		}
		err := validateIntentSignalRules("defaults", rules)
		if err == nil {
			t.Fatal("err = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), `"metric."`) {
			t.Fatalf("err = %q, want substring containing \"metric.\"", err.Error())
		}
	})

	t.Run("negative graceSeconds returns range error", func(t *testing.T) {
		rules := map[string]AlertIntentRule{
			"*": {GraceSeconds: intPtr(-1)},
		}
		err := validateIntentSignalRules("defaults", rules)
		if err == nil {
			t.Fatal("err = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "graceSeconds must be between 0 and") {
			t.Fatalf("err = %q, want substring 'graceSeconds must be between 0 and'", err.Error())
		}
	})

	t.Run("over-max graceSeconds returns range error", func(t *testing.T) {
		rules := map[string]AlertIntentRule{
			"*": {GraceSeconds: intPtr(maxAlertIntentGraceSeconds + 1)},
		}
		err := validateIntentSignalRules("defaults", rules)
		if err == nil {
			t.Fatal("err = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "graceSeconds must be between 0 and") {
			t.Fatalf("err = %q, want substring 'graceSeconds must be between 0 and'", err.Error())
		}
	})

	t.Run("boundary graceSeconds zero and max are valid", func(t *testing.T) {
		rules := map[string]AlertIntentRule{
			"state.offline":         {GraceSeconds: intPtr(0)},
			"incident.availability": {GraceSeconds: intPtr(maxAlertIntentGraceSeconds)},
		}
		if err := validateIntentSignalRules("defaults", rules); err != nil {
			t.Fatalf("err = %v, want nil (boundary values valid)", err)
		}
	})

	t.Run("backupOffline on availability signal returns may-not-configure error", func(t *testing.T) {
		rules := map[string]AlertIntentRule{
			"incident.availability": {BackupOffline: &BackupOfflineIntentPolicy{}},
		}
		err := validateIntentSignalRules("defaults", rules)
		if err == nil {
			t.Fatal("err = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "may not configure backupOffline") {
			t.Fatalf("err = %q, want substring 'may not configure backupOffline'", err.Error())
		}
	})

	t.Run("backupOffline on metric signal returns may-not-configure error", func(t *testing.T) {
		rules := map[string]AlertIntentRule{
			"metric.cpu": {BackupOffline: &BackupOfflineIntentPolicy{}},
		}
		err := validateIntentSignalRules("defaults", rules)
		if err == nil {
			t.Fatal("err = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "may not configure backupOffline") {
			t.Fatalf("err = %q, want substring 'may not configure backupOffline'", err.Error())
		}
	})

	t.Run("backupOffline negative postGraceSeconds returns range error", func(t *testing.T) {
		rules := map[string]AlertIntentRule{
			"state.offline": {BackupOffline: &BackupOfflineIntentPolicy{PostGraceSeconds: -1}},
		}
		err := validateIntentSignalRules("defaults", rules)
		if err == nil {
			t.Fatal("err = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "postGraceSeconds must be between 0 and") {
			t.Fatalf("err = %q, want substring 'postGraceSeconds must be between 0 and'", err.Error())
		}
	})

	t.Run("backupOffline over-max postGraceSeconds returns range error", func(t *testing.T) {
		rules := map[string]AlertIntentRule{
			"state.offline": {BackupOffline: &BackupOfflineIntentPolicy{PostGraceSeconds: maxAlertIntentGraceSeconds + 1}},
		}
		err := validateIntentSignalRules("defaults", rules)
		if err == nil {
			t.Fatal("err = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "postGraceSeconds must be between 0 and") {
			t.Fatalf("err = %q, want substring 'postGraceSeconds must be between 0 and'", err.Error())
		}
	})

	t.Run("backupOffline negative maxDeferralSeconds returns range error", func(t *testing.T) {
		rules := map[string]AlertIntentRule{
			"state.offline": {BackupOffline: &BackupOfflineIntentPolicy{MaxDeferralSeconds: -1}},
		}
		err := validateIntentSignalRules("defaults", rules)
		if err == nil {
			t.Fatal("err = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "maxDeferralSeconds must be between 0 and") {
			t.Fatalf("err = %q, want substring 'maxDeferralSeconds must be between 0 and'", err.Error())
		}
	})

	t.Run("backupOffline over-max maxDeferralSeconds returns range error", func(t *testing.T) {
		rules := map[string]AlertIntentRule{
			"state.offline": {BackupOffline: &BackupOfflineIntentPolicy{MaxDeferralSeconds: maxAlertIntentGraceSeconds + 1}},
		}
		err := validateIntentSignalRules("defaults", rules)
		if err == nil {
			t.Fatal("err = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "maxDeferralSeconds must be between 0 and") {
			t.Fatalf("err = %q, want substring 'maxDeferralSeconds must be between 0 and'", err.Error())
		}
	})

	t.Run("backupOffline enabled with zero maxDeferral returns must-be-positive error", func(t *testing.T) {
		rules := map[string]AlertIntentRule{
			"state.offline": {BackupOffline: &BackupOfflineIntentPolicy{Enabled: true, MaxDeferralSeconds: 0}},
		}
		err := validateIntentSignalRules("defaults", rules)
		if err == nil {
			t.Fatal("err = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "maxDeferralSeconds must be positive when enabled") {
			t.Fatalf("err = %q, want substring 'maxDeferralSeconds must be positive when enabled'", err.Error())
		}
	})

	t.Run("backupOffline disabled with zero maxDeferral is valid", func(t *testing.T) {
		rules := map[string]AlertIntentRule{
			"state.offline": {BackupOffline: &BackupOfflineIntentPolicy{Enabled: false, MaxDeferralSeconds: 0}},
		}
		if err := validateIntentSignalRules("defaults", rules); err != nil {
			t.Fatalf("err = %v, want nil (disabled backup with zero deferral is OK)", err)
		}
	})

	t.Run("backupOffline fully valid on offline signal returns nil", func(t *testing.T) {
		rules := map[string]AlertIntentRule{
			"state.offline": {BackupOffline: &BackupOfflineIntentPolicy{
				Enabled: true, PostGraceSeconds: 5, MaxDeferralSeconds: 10,
			}},
		}
		if err := validateIntentSignalRules("defaults", rules); err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
	})

	t.Run("backupOffline valid on default star signal returns nil", func(t *testing.T) {
		rules := map[string]AlertIntentRule{
			"*": {BackupOffline: &BackupOfflineIntentPolicy{
				Enabled: true, PostGraceSeconds: 0, MaxDeferralSeconds: 1,
			}},
		}
		if err := validateIntentSignalRules("defaults", rules); err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
	})

	t.Run("default alias signal also permits backupOffline", func(t *testing.T) {
		rules := map[string]AlertIntentRule{
			"default": {BackupOffline: &BackupOfflineIntentPolicy{
				Enabled: true, PostGraceSeconds: 0, MaxDeferralSeconds: 1,
			}},
		}
		if err := validateIntentSignalRules("defaults", rules); err != nil {
			t.Fatalf("err = %v, want nil ('default' normalizes to '*')", err)
		}
	})
}
