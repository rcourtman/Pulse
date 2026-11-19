package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	recoverymanager "github.com/rcourtman/pulse-go-rewrite/internal/recovery/manager"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery/model"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type shorterVzdumpLogFixture struct {
	job     models.BackupTask
	monitor *Monitor
	client  *proxmox.Client
	phase   atomic.Int32
	reads   atomic.Int32
}

func newShorterVzdumpLogFixture(t *testing.T) *shorterVzdumpLogFixture {
	t.Helper()
	job, accepted := vzdumpGuestResultFixture()
	runningShort := accepted[:2]
	endedShort := append(append([]proxmox.TaskLogLine{}, runningShort...),
		proxmox.TaskLogLine{LineNumber: 3, Text: "TASK ERROR: job errors"})
	full := append(accepted,
		proxmox.TaskLogLine{LineNumber: 6, Text: "INFO: Finished Backup of VM 103 (00:02:00)"},
		proxmox.TaskLogLine{LineNumber: 7, Text: "TASK ERROR: job errors"})

	f := &shorterVzdumpLogFixture{job: job, monitor: &Monitor{state: models.NewState()}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		phase := f.phase.Load()
		var data any
		switch r.URL.Path {
		case "/api2/json/nodes":
			data = []proxmox.Node{{Node: job.Node, Status: "online"}}
		case "/api2/json/access/permissions":
			data = map[string]any{"/nodes/" + job.Node: map[string]int{"Sys.Audit": 1}}
		case "/api2/json/nodes/" + job.Node + "/tasks":
			task := proxmox.Task{UPID: testJobUPID, Node: job.Node, Type: "vzdump", StartTime: job.StartTime.Unix()}
			if phase >= 2 {
				task.EndTime, task.Status = job.EndTime.Unix(), "job errors"
			}
			data = []proxmox.Task{task}
		case "/api2/json/nodes/" + job.Node + "/tasks/" + testJobUPID + "/log":
			f.reads.Add(1)
			data = [][]proxmox.TaskLogLine{accepted, runningShort, endedShort, full, full}[phase]
		default:
			t.Errorf("unexpected Proxmox read %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet {
			t.Errorf("unexpected method %s", r.Method)
		}
		if err := json.NewEncoder(w).Encode(map[string]any{"data": data}); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(server.Close)
	client, err := proxmox.NewClient(proxmox.ClientConfig{
		Host: server.URL, TokenName: "fixture@pam!readonly", TokenValue: "fixture",
	})
	require.NoError(t, err)
	f.client = client
	return f
}

func (f *shorterVzdumpLogFixture) poll(ctx context.Context, phase int32) models.PVEBackups {
	f.phase.Store(phase)
	f.monitor.pollBackupTasks(ctx, f.job.Instance, f.client)
	return f.monitor.PVEBackupsSnapshot()
}

// A valid HTTP envelope and contiguous line numbers don't prove that a reread
// is as complete as the already accepted prefix of this same job.
func TestVzdumpTaskLogShorterReadRetainsAcceptedEvidence(t *testing.T) {
	ctx := context.Background()
	f := newShorterVzdumpLogFixture(t)
	m := f.monitor
	dataDir := t.TempDir()
	// Keep the fixture self-contained even when running on a Pulse host.
	t.Setenv("PULSE_LEGACY_KEY_PATH", filepath.Join(dataDir, "absent-test-legacy.key"))
	m.recoveryManager = recoverymanager.New(config.NewMultiTenantPersistence(dataDir))
	store, err := m.recoveryManager.StoreForOrg("default")
	require.NoError(t, err)
	defer store.Close()

	guestTasks := func(snapshot models.PVEBackups) map[int]models.BackupTask {
		guests := map[int]models.BackupTask{}
		for _, task := range snapshot.BackupTasks {
			if task.VMID > 0 {
				guests[task.VMID] = task
			}
		}
		return guests
	}
	poll := func(phase int32) map[int]models.BackupTask {
		// Persist collection observations in distinct milliseconds, as ordinary
		// polls do. Do not overwrite synthetic guest evidence between polls.
		coverage, err := store.BackupHistoryCoverage(ctx, f.job.StartTime, time.Now().UTC())
		require.NoError(t, err)
		for _, source := range coverage.Sources {
			if wait := time.Until(source.LastObservedAt.Add(time.Millisecond)); wait > 0 {
				time.Sleep(wait)
			}
		}
		return guestTasks(f.poll(ctx, phase))
	}
	checkHistory := func(complete bool, last recovery.Outcome) {
		points, _, err := store.ListPoints(ctx, recovery.ListPointsOptions{Kind: recovery.KindOther})
		require.NoError(t, err)
		byID := map[string]recovery.RecoveryPoint{}
		for _, point := range points {
			byID[point.ID] = point
		}
		id := "pve-task:" + f.job.ID
		assert.Equal(t, complete, byID[id].Details["guestTaskDetailsComplete"], "parent detail coverage")
		assert.Equal(t, recovery.OutcomeFailed, byID[id+"-vm102"].Outcome, "recorded failure")
		lastGuest := byID[id+"-vm103"]
		assert.Equal(t, last, lastGuest.Outcome, "recorded last guest")
		if last == recovery.OutcomeUnknown || last == recovery.OutcomeSuccess {
			failed := byID[id+"-vm102"]
			require.NotNil(t, failed.CompletedAt, "durable reports retain the known failure under the parent bound")
			assert.True(t, f.job.EndTime.Equal(*failed.CompletedAt))
			require.NotNil(t, lastGuest.CompletedAt)
			if last == recovery.OutcomeUnknown {
				assert.True(t, f.job.EndTime.Equal(*lastGuest.CompletedAt), "report timing still belongs to the job")
			} else {
				assert.True(t, f.job.StartTime.Add(3*time.Minute).Equal(*lastGuest.CompletedAt))
			}
		}
	}

	accepted := poll(0)
	require.Len(t, accepted, 3)
	require.Equal(t, "OK", accepted[101].Status)
	require.Equal(t, "error", accepted[102].Status)
	require.Equal(t, "running", accepted[103].Status)
	checkHistory(false, recovery.OutcomeRunning)

	observed := accepted[103].ObservedAt
	cutoff := observed.Add(5 * time.Minute)
	workload := []model.BackupHealthWorkload{{ID: "fixture-writer", Instance: f.job.Instance, VMID: 103}}
	for i := 0; i < 2; i++ {
		running := poll(1)
		assert.Equal(t, accepted, running, "shorter prefix must not erase rows or renew their observations")
		checkHistory(false, recovery.OutcomeRunning)

		points, _, err := store.ListPoints(ctx, recovery.ListPointsOptions{Kind: recovery.KindOther})
		require.NoError(t, err)
		at := recovery.BuildBackupHealth(workload, points, model.BackupHistoryCoverage{}, f.job.StartTime, cutoff, time.UTC)
		assert.Equal(t, 1, at.Counts.Running, "the original observation still counts at the cutoff")
		expired := recovery.BuildBackupHealth(workload, points, model.BackupHistoryCoverage{}, f.job.StartTime, cutoff.Add(time.Nanosecond), time.UTC)
		assert.Equal(t, model.BackupOutcomeCounts{Unknown: 1}, expired.Counts, "re-ingesting a carried guest must not renew Running")
		assert.Equal(t, expired.Counts, expired.Days[0].Counts)

		intent, active := m.resolveBackupIntentContext("", f.job.Instance, f.job.Node, 103, cutoff)
		assert.True(t, active, "the original observed writer remains visible until the ordinary age limit")
		assert.Equal(t, observed, intent.ObservedAt)
		_, active = m.resolveBackupIntentContext("", f.job.Instance, f.job.Node, 103, cutoff.Add(time.Nanosecond))
		assert.False(t, active, "a shorter read must not extend intent")
	}

	ended := poll(2)
	assert.Len(t, ended, 3)
	assert.Equal(t, "error", ended[102].Status)
	assert.Equal(t, accepted[102].Error, ended[102].Error)
	assert.Equal(t, "unknown", ended[103].Status, "an ended parent doesn't supply a guest verdict")
	assert.Equal(t, accepted[101].EndTime, ended[101].EndTime, "keep the observed guest finish")
	assert.True(t, ended[102].EndTime.IsZero(), "a failure line without timing cannot borrow the parent end")
	assert.True(t, ended[103].EndTime.IsZero(), "an unknown guest result can't borrow the parent end")
	assert.Equal(t, observed, ended[103].ObservedAt)
	_, active := m.resolveBackupIntentContext("", f.job.Instance, f.job.Node, 103, observed.Add(time.Second))
	assert.False(t, active, "ended parent withdraws running intent even without a new guest result")
	checkHistory(false, recovery.OutcomeUnknown)

	recovered := poll(3)
	assert.Len(t, recovered, 3)
	assert.Equal(t, "OK", recovered[101].Status)
	assert.Equal(t, "storage unavailable", recovered[102].Error)
	assert.True(t, recovered[102].EndTime.IsZero(), "complete logs still cannot measure a failure without a finish line")
	assert.Equal(t, "OK", recovered[103].Status)
	assert.True(t, f.job.StartTime.Add(3*time.Minute).Equal(recovered[103].EndTime), "keep the observed finish duration")
	assert.Equal(t, accepted[103].ID, recovered[103].ID)
	checkHistory(true, recovery.OutcomeSuccess)
	cached := poll(4)
	assert.Equal(t, recovered[103].ID, cached[103].ID)
	assert.Equal(t, "OK", cached[103].Status)
	assert.EqualValues(t, 5, f.reads.Load(), "only the full terminal log finalizes; the next poll uses its cache")
}

func TestVzdumpTaskLogShortPrefixIsUsableWithoutPreviousObservation(t *testing.T) {
	f := newShorterVzdumpLogFixture(t)
	tasks := f.poll(context.Background(), 1).BackupTasks
	require.Len(t, tasks, 2, "a short cold read may contain useful evidence")
	for _, task := range tasks {
		if task.VMID != 0 {
			require.Equal(t, 101, task.VMID)
			require.Equal(t, "OK", task.Status)
			require.Equal(t, fmt.Sprintf("%s-vm101", f.job.ID), task.ID)
		}
	}
}
