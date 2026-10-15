package pmg

import (
	"encoding/json"
	"testing"
)

func TestClusterStatusIntegerCID(t *testing.T) {
	for _, tc := range []struct {
		name, cid string
		want      int
		invalid   bool
	}{
		{"number", `1`, 1, false}, {"string", `"1"`, 1, false},
		{"zero-number", `0`, 0, false}, {"zero-string", `"0"`, 0, false},
		{"null", `null`, 0, false}, {"omitted", ``, 0, false},
		{"fraction", `1.5`, 0, true}, {"fraction-string", `"1.5"`, 0, true},
		{"exponent", `1e2`, 0, true}, {"exponent-string", `"1e2"`, 0, true},
		{"text", `"primary"`, 0, true}, {"empty-string", `""`, 0, true},
		{"overflow", `"999999999999999999999999999999999"`, 0, true},
		{"bool", `true`, 0, true}, {"object", `{}`, 0, true}, {"array", `[]`, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := `{"name":"mail-a","type":"master","ip":"192.0.2.10","fingerprint":"fixture"`
			if tc.cid != "" {
				data += `,"cid":` + tc.cid
			}
			data += `}`
			got := ClusterStatusEntry{CID: 9}
			err := json.Unmarshal([]byte(data), &got)
			if tc.invalid {
				if err == nil {
					t.Fatal("invalid cluster ID accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.CID != tc.want || got.Name != "mail-a" || got.Type != "master" || got.IP != "192.0.2.10" || got.Fingerprint != "fixture" {
				t.Fatalf("entry = %+v", got)
			}
			// The Go/wire representation remains an integer, even after string input.
			b, err := json.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			var again map[string]any
			if err := json.Unmarshal(b, &again); err != nil {
				t.Fatal(err)
			}
			if again["cid"] != float64(tc.want) {
				t.Fatalf("CID marshalled as %s", b)
			}
		})
	}
}
