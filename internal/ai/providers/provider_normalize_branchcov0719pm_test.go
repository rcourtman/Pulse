package providers

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestToolProgressEventNormalizeCollections_BranchCov0719pm exercises every
// branch of (ToolProgressEvent).NormalizeCollections (provider.go:211):
//   - the `if e.Input == nil` true arm, which substitutes a non-nil empty map
//   - the implicit else, where a populated Input map is preserved unchanged
//
// Both cases also assert that the non-Input scalar/slice fields are passed
// through verbatim so the normalizer is confirmed to be a no-op outside of the
// nil-collection fix-up.
func TestToolProgressEventNormalizeCollections_BranchCov0719pm(t *testing.T) {
	tests := []struct {
		name  string
		event ToolProgressEvent
		// wantInputIsNil reports the expected nil-ness of the returned Input
		// map. It is always false after normalization; documented per-case for
		// clarity.
		wantInputIsNil bool
		// wantInput reports the expected contents of the returned Input map
		// (compared with reflect.DeepEqual so an empty non-nil map is
		// distinguishable from a nil one).
		wantInput map[string]interface{}
	}{
		{
			name: "nil input map is replaced with non-nil empty map",
			event: ToolProgressEvent{
				ID:       "tool_42",
				Name:     "get_time",
				Input:    nil,
				RawInput: `{"tz":"UTC"}`,
				Phase:    "streaming",
				Message:  "Receiving tool input.",
			},
			wantInputIsNil: false,
			wantInput:      map[string]interface{}{},
		},
		{
			name: "populated input map is preserved unchanged",
			event: ToolProgressEvent{
				ID:    "tool_43",
				Name:  "search",
				Input: map[string]interface{}{"q": "pulse", "limit": float64(10)},
				Phase: "ready",
			},
			wantInputIsNil: false,
			wantInput:      map[string]interface{}{"q": "pulse", "limit": float64(10)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Snapshot the input so we can prove the scalar/slice fields are
			// preserved on the populated case. Input is asserted separately.
			inID, inName := tt.event.ID, tt.event.Name
			inRaw, inPhase, inMsg := tt.event.RawInput, tt.event.Phase, tt.event.Message

			got := tt.event.NormalizeCollections()

			// Input map: must never be nil after normalization, and contents
			// must equal the expected map (empty for the nil case, the
			// original entries for the populated case).
			if got.Input == nil {
				t.Fatalf("NormalizeCollections returned nil Input map; want non-nil")
			}
			if tt.wantInputIsNil {
				t.Fatalf("test data error: wantInputIsNil must be false after normalization")
			}
			if !reflect.DeepEqual(got.Input, tt.wantInput) {
				t.Fatalf("NormalizeCollections Input = %#v, want %#v", got.Input, tt.wantInput)
			}

			// On the populated case, the original map object identity may be
			// preserved or a new map allocated with the same contents — the
			// contract is contents-equality, asserted above via DeepEqual.
			// Here we additionally assert the non-nil invariant explicitly so
			// a future refactor that drops the fix-up fails loudly.
			assert.NotNil(t, got.Input, "normalized Input map must be non-nil")

			// All other fields must round-trip unchanged.
			assert.Equal(t, inID, got.ID, "ID must be preserved")
			assert.Equal(t, inName, got.Name, "Name must be preserved")
			assert.Equal(t, inRaw, got.RawInput, "RawInput must be preserved")
			assert.Equal(t, inPhase, got.Phase, "Phase must be preserved")
			assert.Equal(t, inMsg, got.Message, "Message must be preserved")
		})
	}
}

// TestToolProgressEventNormalizeCollections_ZeroValue0719pm covers the literal
// zero value of ToolProgressEvent, which is the most common shape that reaches
// NormalizeCollections when a provider constructs an event before populating
// any field. Every field — including Input — starts nil and Input must end up a
// non-nil empty map.
func TestToolProgressEventNormalizeCollections_ZeroValue0719pm(t *testing.T) {
	var zero ToolProgressEvent
	got := zero.NormalizeCollections()

	if got.Input == nil {
		t.Fatalf("NormalizeCollections on zero-value event left Input nil; want non-nil empty map")
	}
	if len(got.Input) != 0 {
		t.Fatalf("NormalizeCollections on zero-value event Input len = %d, want 0", len(got.Input))
	}
	// Zero scalars are unchanged.
	assert.Equal(t, "", got.ID, "ID must remain zero value")
	assert.Equal(t, "", got.Name, "Name must remain zero value")
	assert.Equal(t, "", got.RawInput, "RawInput must remain zero value")
	assert.Equal(t, "", got.Phase, "Phase must remain zero value")
	assert.Equal(t, "", got.Message, "Message must remain zero value")
}
