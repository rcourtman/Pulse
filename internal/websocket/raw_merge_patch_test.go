package websocket

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"reflect"
	"testing"
)

func assertRawMergeMatchesReference(t testing.TB, previous, current []byte) {
	t.Helper()
	want, wantErr := referenceJSONMergePatch(previous, current)
	got, gotErr := createJSONMergePatch(previous, current)
	if (wantErr == nil) != (gotErr == nil) {
		t.Fatalf("error mismatch: before=%q after=%q reference=%v raw=%v", previous, current, wantErr, gotErr)
	}
	if wantErr == nil && !reflect.DeepEqual(decodeExactDeltaJSON(t, want), decodeExactDeltaJSON(t, got)) {
		t.Fatalf("patch mismatch: before=%s after=%s reference=%s raw=%s", previous, current, want, got)
	}
}

func TestRawMergePatchSemanticEquivalence(t *testing.T) {
	cases := []string{`null`, `true`, `false`, `0`, `1.0`, `1e0`, `9007199254740993`, `18446744073709551615`, `"<&>"`, `"\u003c\u0026\u003e"`, `[]`, `[null,1.0,{"z":2,"a":1}]`, `[null,1.0,{"a":1,"z":2}]`, `{}`, `{"id":"r","nested":{"id":"child","n":9007199254740993}}`, `{"nested":{"n":9007199254740993,"id":"child"},"id":"r"}`, `{"id":"r","nested":{"id":"changed","n":9007199254740994}}`, `{"id":"old","id":"r","nested":{"a":1,"a":2}}`, `{"id":"r","nested":{"a":2}}`, `{"id":null,"optional":null}`, `{"id":"r","optional":[]}`, `{"id":"r","optional":{}}`}
	for i, a := range cases {
		for j, b := range cases {
			t.Run(fmt.Sprintf("%d-%d", i, j), func(t *testing.T) { assertRawMergeMatchesReference(t, []byte(a), []byte(b)) })
		}
	}
	// Identical invalid bytes may not bypass validation.
	for _, invalid := range []string{``, `{"a":`, `{"a":1} {}`, `[1,]`, `NaN`} {
		assertRawMergeMatchesReference(t, []byte(invalid), []byte(invalid))
	}
}

func randomMergeValue(r *rand.Rand, depth int) any {
	if depth == 0 {
		return []any{nil, true, false, "same<&>", "changed", json.Number("9007199254740993"), json.Number("1.0"), json.Number("1e0")}[r.Intn(8)]
	}
	switch r.Intn(3) {
	case 0:
		return randomMergeValue(r, 0)
	case 1:
		return []any{randomMergeValue(r, depth-1), randomMergeValue(r, depth-1)}
	default:
		return map[string]any{"id": "r", "a": randomMergeValue(r, depth-1), "b": randomMergeValue(r, depth-1)}
	}
}
func TestRawMergePatchRandomEquivalence(t *testing.T) {
	r := rand.New(rand.NewSource(2199))
	for i := 0; i < 1000; i++ {
		a, _ := json.Marshal(randomMergeValue(r, 3))
		b, _ := json.Marshal(randomMergeValue(r, 3))
		assertRawMergeMatchesReference(t, a, b)
	}
}
func FuzzRawMergePatchEquivalence(f *testing.F) {
	f.Add([]byte(`{"id":"r","a":{"b":1},"c":null}`), []byte(`{"id":"r","a":{"b":2},"c":[]}`))
	f.Add([]byte(`{"id":"r","x":18446744073709551615}`), []byte(`{"id":"r","x":9007199254740993}`))
	f.Add([]byte(`{"id":"a","id":"r"}`), []byte(`{"id":"r"}`))
	f.Fuzz(func(t *testing.T, a, b []byte) { assertRawMergeMatchesReference(t, a, b) })
}
