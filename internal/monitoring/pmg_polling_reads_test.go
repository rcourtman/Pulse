package monitoring

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rs/zerolog"
)

// Exercise the real read-only PMG poll with fixed provider replies. Only
// unrelated inventory grows: no live gateways, guest commands or databases.
func pmgPollingReadFixture(unrelated int, scenario string) (*Monitor, *pmgBackupRetentionFixture) {
	m := pmgScopeMonitor(config.PMGInstance{MonitoringConfigured: true})
	at := time.Unix(1700000000, 0).UTC()
	m.state.PMGBackups = []models.PMGBackup{
		{ID: "one-old", Instance: "gateway", Node: "one", Filename: "one-old.tgz", BackupTime: at, Size: 10},
		{ID: "peer-old", Instance: "peer", Node: "one", Filename: "peer-old.tgz", BackupTime: at.Add(time.Minute), Size: 99},
		{ID: "two-old", Instance: "gateway", Node: "two", Filename: "two-old.tgz", BackupTime: at.Add(2 * time.Minute), Size: 20},
		{ID: "gone", Instance: "gateway", Node: "gone", Filename: "gone.tgz", BackupTime: at.Add(3 * time.Minute), Size: 30},
	}
	// The separately cached aggregate must never supply carry-forward evidence.
	m.state.Backups.PMG = []models.PMGBackup{{ID: "stale-cache", Instance: "gateway", Node: "one"}}
	cluster := models.KubernetesCluster{ID: "unrelated"}
	for i := 0; i < unrelated; i++ {
		id := fmt.Sprint(i)
		m.state.VMs = append(m.state.VMs, models.VM{ID: id, Instance: "other", Disks: []models.Disk{{Mountpoint: "/"}}})
		cluster.ConfigMaps = append(cluster.ConfigMaps, models.KubernetesConfigMap{UID: id, Labels: map[string]string{"fixture": "only"}, DataKeys: []string{"key"}})
	}
	if unrelated > 0 {
		m.state.KubernetesClusters = []models.KubernetesCluster{cluster}
	}
	f := newPMGBackupRetentionFixture()
	switch scenario {
	case "cluster-unread":
		f.clusterErr = errors.New("synthetic cluster read unavailable")
	case "all-nodes-unread":
		f.backupErr["one"] = errors.New("synthetic backup read unavailable")
		f.backupErr["two"] = errors.New("synthetic backup read unavailable")
	case "partial":
		f.backupErr["one"] = errors.New("synthetic backup read unavailable")
	case "complete":
	default:
		panic("unknown PMG read fixture")
	}
	return m, f
}

func TestPMGBackupReadProjectionPreservesPollEvidence(t *testing.T) {
	for _, scenario := range []string{"cluster-unread", "all-nodes-unread", "partial", "complete"} {
		t.Run(scenario, func(t *testing.T) {
			m, f := pmgPollingReadFixture(10, scenario)
			before := m.state.GetSnapshot()
			current := []models.PMGBackup{}
			enumerated, read := map[string]struct{}{}, map[string]struct{}{}
			if f.clusterErr == nil {
				for _, node := range f.nodes {
					enumerated[node] = struct{}{}
					if f.backupErr[node] != nil {
						continue
					}
					read[node] = struct{}{}
					for _, b := range f.backups[node] {
						current = append(current, models.PMGBackup{ID: fmt.Sprintf("pmg-gateway-%s-%d", node, b.Timestamp.Int64()), Instance: "gateway", Node: node, Filename: b.Filename, BackupTime: time.Unix(b.Timestamp.Int64(), 0), Size: b.Size.Int64()})
					}
				}
			}
			// Legacy oracle reads the full primary snapshot at the same point. It is
			// independent of the new getter, and covers mixed-instance and lost-node data.
			expected, _ := preserveUnreadPMGNodeBackups("gateway", before.PMGBackups, current, f.clusterErr != nil, enumerated, read)
			expectedState := models.NewState()
			expectedState.PMGBackups = before.PMGBackups
			expectedState.UpdatePMGBackups("gateway", expected)
			want := expectedState.GetSnapshot().PMGBackups
			m.pollPMGInstance(context.Background(), "gateway", f)
			after := m.state.GetSnapshot()
			if !reflect.DeepEqual(after.PMGBackups, want) {
				t.Fatalf("backup evidence changed: got %+v; want %+v", after.PMGBackups, want)
			}
			gotJSON, _ := json.Marshal(after.PMGBackups)
			wantJSON, _ := json.Marshal(want)
			if string(gotJSON) != string(wantJSON) {
				t.Fatal("serialized backup values or order changed")
			}
			if !reflect.DeepEqual(after.VMs, before.VMs) || !reflect.DeepEqual(after.KubernetesClusters, before.KubernetesClusters) {
				t.Fatal("poll changed unrelated inventory")
			}
			if !after.ConnectionHealth["pmg-gateway"] || len(after.PMGInstances) != 1 || after.PMGInstances[0].Status != "online" {
				t.Fatal("read failure changed connection health")
			}
			if scenario != "complete" {
				assertPMGReadFailed(t, m)
			} else if m.pollStatusMap["pmg::gateway"].LastErrorMessage != "" {
				t.Fatal("complete read kept a failure")
			}
			if f.clusterErr != nil && (f.calls["backup/one"] != 0 || f.calls["backup/two"] != 0) {
				t.Fatal("failed cluster read guessed child requests")
			}
		})
	}
}

func TestPMGBackupReadsAvoidUnrelatedFleetCopy(t *testing.T) {
	// Only this poll owns the quiet output. Alert persistence and other
	// background workers may still be using the process logger.
	logger := zerolog.New(io.Discard)
	ctx := logger.WithContext(context.Background())
	for _, scenario := range []string{"cluster-unread", "all-nodes-unread", "partial", "complete"} {
		t.Run(scenario, func(t *testing.T) {
			small, sc := pmgPollingReadFixture(0, scenario)
			large, lc := pmgPollingReadFixture(1000, scenario)
			measure := func(m *Monitor, f *pmgBackupRetentionFixture) float64 {
				return testing.AllocsPerRun(5, func() { m.pollPMGInstance(ctx, "gateway", f) })
			}
			base, grown := measure(small, sc), measure(large, lc)
			t.Logf("allocations per real poll: small=%.0f unrelated-1000=%.0f", base, grown)
			if grown > base+4 {
				t.Fatalf("PMG read copies unrelated fleet inventory: %.0f -> %.0f", base, grown)
			}
		})
	}
	small, _ := pmgPollingReadFixture(0, "partial")
	large, _ := pmgPollingReadFixture(1000, "partial")
	measure := func(m *Monitor) float64 {
		return testing.AllocsPerRun(5, func() { pmgPollingSnapshotSink = m.state.GetSnapshot() })
	}
	base, grown := measure(small), measure(large)
	t.Logf("full snapshot positive control: small=%.0f unrelated-1000=%.0f", base, grown)
	if grown <= base+1000 {
		t.Fatal("fixture does not expose whole-fleet copying")
	}
}

var pmgPollingSnapshotSink models.StateSnapshot

func BenchmarkPMGBackupReadPoll(b *testing.B) {
	logger := zerolog.New(io.Discard)
	ctx := logger.WithContext(context.Background())
	for _, scenario := range []string{"cluster-unread", "all-nodes-unread", "partial", "complete"} {
		for _, size := range []int{0, 1000} {
			b.Run(fmt.Sprintf("%s/unrelated-%d", scenario, size), func(b *testing.B) {
				m, f := pmgPollingReadFixture(size, scenario)
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					m.pollPMGInstance(ctx, "gateway", f)
				}
			})
		}
	}
}
