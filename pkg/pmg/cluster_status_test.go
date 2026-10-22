package pmg

import (
	"encoding/json"
	"testing"
)

// The upstream PMG repair discards this unused field entirely. Its wire shape
// must not hide the node identity that actually addresses queue/backup reads.
func TestClusterStatusIgnoresUnusedCID(t *testing.T) {
	for _, cid := range []string{`1`, `"1"`, `0`, `"0"`, `null`, ``, `1.5`, `"1.5"`, `1e2`, `"1e2"`, `"primary"`, `""`, `"999999999999999999999999999999999"`, `true`, `{}`, `[]`} {
		t.Run(cid, func(t *testing.T) {
			data := `{"name":"mail-a","type":"master","ip":"192.0.2.10","fingerprint":"fixture"`
			if cid != "" {
				data += `,"cid":` + cid
			}
			data += `}`
			var got ClusterStatusEntry
			if err := json.Unmarshal([]byte(data), &got); err != nil {
				t.Fatal(err)
			}
			if got.Name != "mail-a" || got.Type != "master" || got.IP != "192.0.2.10" || got.Fingerprint != "fixture" {
				t.Fatalf("entry = %+v", got)
			}
			b, err := json.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			var again map[string]any
			if err := json.Unmarshal(b, &again); err != nil {
				t.Fatal(err)
			}
			if _, present := again["cid"]; present {
				t.Fatalf("unused CID marshalled as %s", b)
			}
		})
	}
}
