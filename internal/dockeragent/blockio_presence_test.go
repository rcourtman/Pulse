package dockeragent

import (
	"encoding/json"
	"testing"

	containertypes "github.com/moby/moby/api/types/container"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
)

func TestSummarizeBlockIOPreservesDirectionPresenceAndZero(t *testing.T) {
	for _, tc := range []struct {
		name        string
		entries     []containertypes.BlkioStatEntry
		read, write bool
	}{
		{"absent", nil, false, false},
		{"unrelated", []containertypes.BlkioStatEntry{{Op: "Total", Value: 100}}, false, false},
		{"observed idle", []containertypes.BlkioStatEntry{{Op: "Read", Value: 0}, {Op: "Write", Value: 0}}, true, true},
		{"read only idle", []containertypes.BlkioStatEntry{{Op: "Read", Value: 0}}, true, false},
		{"write only", []containertypes.BlkioStatEntry{{Op: "Write", Value: 123}}, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := summarizeBlockIO(containertypes.StatsResponse{BlkioStats: containertypes.BlkioStats{IoServiceBytesRecursive: tc.entries}})
			if !tc.read && !tc.write {
				if got != nil {
					t.Fatalf("absent counters became observations: %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("explicit counters were dropped")
			}
			encoded, err := json.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			var decoded agentsdocker.ContainerBlockIO
			if err := json.Unmarshal(encoded, &decoded); err != nil {
				t.Fatal(err)
			}
			read, write := decoded.CounterPresence()
			if read != tc.read || write != tc.write {
				t.Fatalf("presence lost through report JSON %s: %v/%v", encoded, read, write)
			}
		})
	}
}
