package proxmox

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTaskSameExecutionIdentity(t *testing.T) {
	task := Task{UPID: "opaque", Node: "node", Type: "vzdump", ID: "100", PID: 17, PStart: 5, StartTime: 1700000000, User: "fixture@pve", Status: "running"}
	for _, tc := range []struct {
		name   string
		change func(*Task)
		same   bool
	}{
		{"identical", func(*Task) {}, true},
		{"completion", func(t *Task) { t.Status = "OK"; t.EndTime = 1700000060 }, true},
		{"upid", func(t *Task) { t.UPID = "another" }, false},
		{"node", func(t *Task) { t.Node = "peer" }, false},
		{"unbound-node", func(t *Task) { t.Node = "" }, false},
		{"type", func(t *Task) { t.Type = "qmstart" }, false},
		{"guest", func(t *Task) { t.ID = "101" }, false},
		{"parent", func(t *Task) { t.ID = "" }, false},
		{"pid", func(t *Task) { t.PID++ }, false},
		{"pstart", func(t *Task) { t.PStart++ }, false},
		{"start", func(t *Task) { t.StartTime++ }, false},
		{"owner", func(t *Task) { t.User = "other@pve" }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			other := task
			tc.change(&other)
			original := other
			require.Equal(t, tc.same, task.SameExecutionIdentity(other))
			require.Equal(t, tc.same, other.SameExecutionIdentity(task), "identity is symmetric")
			require.Equal(t, original, other, "comparison cannot mutate outcome or identity")
			require.Equal(t, "running", task.Status)
		})
	}
}
