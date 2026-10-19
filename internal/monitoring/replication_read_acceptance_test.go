package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// scripts/replication_read_acceptance.py supplies the ordinary client's
// actual emitted jobs/errors. This stage exercises the real poll conversion and
// state replacement, not new HTTP verdicts or a native replication experiment.
func TestReplicationReadConsumerFrames(t *testing.T) {
	input := os.Getenv("PULSE_REPLICATION_ACCEPTANCE_FRAMES")
	if input == "" {
		t.Skip("run scripts/replication_read_acceptance.py for connected producer/consumer proof")
	}
	data, err := os.ReadFile(input)
	if err != nil {
		t.Fatal(err)
	}
	var frames map[string]struct {
		Jobs        []proxmox.ReplicationJob `json:"jobs"`
		Unavailable bool                     `json:"unavailable"`
	}
	if err := json.Unmarshal(data, &frames); err != nil {
		t.Fatal(err)
	}
	m := &Monitor{state: models.NewState()}
	peer := models.ReplicationJob{ID: "site-b-100-0", Instance: "site-b", JobID: "100-0", GuestID: 100,
		GuestName: "independent-replica", Enabled: true, LastSyncStatus: "ok", SourceNode: "peer-a", TargetNode: "peer-b", LastPolled: time.Unix(1792407600, 0)}
	m.state.UpdateReplicationJobsForInstance("site-b", []models.ReplicationJob{peer})
	vms := []models.VM{{VMID: 100, Name: "protected-replica", Type: "qemu", Node: "node-a"}, {VMID: 101, Name: "successful-sibling", Type: "qemu", Node: "node-a"}}
	var previous []models.ReplicationJob
	output := make(map[string][]models.ReplicationJob)
	for _, phase := range []string{"initial", "short-inventory", "ambiguous-status", "recovered", "empty"} {
		frame, ok := frames[phase]
		if !ok {
			t.Fatalf("missing client frame: %s", phase)
		}
		client := &lifecycleReplicationClient{read: func(context.Context) ([]proxmox.ReplicationJob, error) {
			if frame.Unavailable {
				return nil, fmt.Errorf("synthetic incomplete inventory")
			}
			return frame.Jobs, nil
		}}
		m.pollReplicationStatus(context.Background(), "site-a", client, vms)
		rows := m.ReplicationJobsSnapshot()
		byID := make(map[string]models.ReplicationJob)
		for _, row := range rows {
			byID[row.ID] = row
		}
		if !reflect.DeepEqual(byID[peer.ID], peer) {
			t.Fatalf("%s changed independent connection: %+v", phase, byID[peer.ID])
		}
		if phase == "empty" {
			if len(rows) != 1 {
				t.Fatal("complete empty observation failed to remove only its own jobs")
			}
		} else {
			if len(rows) != 3 || byID["site-a-101-0"].LastSyncStatus != "ok" {
				t.Fatalf("%s lost readable sibling: %+v", phase, rows)
			}
			if phase == "short-inventory" {
				if !reflect.DeepEqual(rows, previous) {
					t.Fatal("incomplete inventory replaced jobs, failure detail or original LastPolled")
				}
			} else {
				want := map[string]string{"initial": "error", "ambiguous-status": "", "recovered": "ok"}[phase]
				row := byID["site-a-100-0"]
				if row.LastSyncStatus != want || row.SourceNode != "node-a" || row.TargetNode != "node-b" || row.GuestName != "protected-replica" {
					t.Fatalf("%s misattributed outcome: %+v", phase, row)
				}
				if phase == "ambiguous-status" && (row.LastSyncTime != nil || row.Error != "" || row.FailCount != 0) {
					t.Fatal("rejected status merged provider fields")
				}
				if phase == "initial" && row.Error != "Synthetic source snapshot unavailable" {
					t.Fatal("explicit failure detail lost")
				}
			}
		}
		previous = rows
		output[phase] = rows
	}
	wire, err := json.Marshal(output)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("REPLICATION_MONITOR_FRAMES=%s", wire)
}
