package ai

import (
	"reflect"
	"testing"
)

// TestFindingJSONMirrorStaysInSync enforces the deliberate marshal-mirror
// relationship between Finding and findingJSON: identical field names and
// types, identical json tags, with AlertIdentifier as the only sanctioned
// divergence (hidden via `json:"-"` on the public struct, round-tripped as
// "alert_identifier" by the mirror).
func TestFindingJSONMirrorStaysInSync(t *testing.T) {
	assertMarshalMirrorInSync(t,
		reflect.TypeOf(Finding{}),
		reflect.TypeOf(findingJSON{}),
		map[string]string{"AlertIdentifier": "alert_identifier,omitempty"},
	)
}

// assertMarshalMirrorInSync fails the test unless mirror has exactly the same
// fields (names, types, json tags) as public, modulo the sanctioned tag
// overrides keyed by field name.
func assertMarshalMirrorInSync(t *testing.T, public, mirror reflect.Type, tagOverrides map[string]string) {
	t.Helper()

	mirrorFields := make(map[string]reflect.StructField, mirror.NumField())
	for i := 0; i < mirror.NumField(); i++ {
		f := mirror.Field(i)
		mirrorFields[f.Name] = f
	}

	if public.NumField() != mirror.NumField() {
		t.Errorf("%s has %d fields but %s has %d — keep the marshal mirror in sync",
			public.Name(), public.NumField(), mirror.Name(), mirror.NumField())
	}

	for i := 0; i < public.NumField(); i++ {
		pf := public.Field(i)
		mf, ok := mirrorFields[pf.Name]
		if !ok {
			t.Errorf("%s.%s is missing from mirror %s", public.Name(), pf.Name, mirror.Name())
			continue
		}
		if pf.Type != mf.Type {
			t.Errorf("%s.%s type %s diverged from mirror type %s", public.Name(), pf.Name, pf.Type, mf.Type)
		}
		wantTag := pf.Tag.Get("json")
		if override, isOverride := tagOverrides[pf.Name]; isOverride {
			wantTag = override
		}
		if got := mf.Tag.Get("json"); got != wantTag {
			t.Errorf("%s.%s mirror json tag = %q, want %q", public.Name(), pf.Name, got, wantTag)
		}
	}
}
