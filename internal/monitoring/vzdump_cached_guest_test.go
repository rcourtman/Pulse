package monitoring

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	recoverymanager "github.com/rcourtman/pulse-go-rewrite/internal/recovery/manager"
	proxmoxmapper "github.com/rcourtman/pulse-go-rewrite/internal/recovery/mapper/proxmox"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery/model"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/stretchr/testify/require"
)

type cachedVzdumpClient struct {
	jobBackupTaskClient
	logErr error
}

func (c *cachedVzdumpClient) GetTaskLog(context.Context, string, string) ([]proxmox.TaskLogLine, error) {
	c.logRequests++
	return c.logLines, c.logErr
}

func TestVzdumpCachedGuestReadFailureDoesNotRenewRunningIntent(t *testing.T) {
	for _, failure := range []string{"503 unavailable", "403 denied"} {
		for _, parentStatus := range []string{"OK", "job errors"} {
			t.Run(failure+"/"+parentStatus, func(t *testing.T) {
				ctx := context.Background()
				job, lines := vzdumpGuestResultFixture()
				client := &cachedVzdumpClient{jobBackupTaskClient: jobBackupTaskClient{
					task:     proxmox.Task{UPID: testJobUPID, Node: job.Node, Type: "vzdump", StartTime: job.StartTime.Unix()},
					logLines: lines,
				}}
				manager := recoverymanager.New(config.NewMultiTenantPersistence(t.TempDir()))
				m := &Monitor{state: models.NewState(), recoveryManager: manager}
				store, err := manager.StoreForOrg("default")
				require.NoError(t, err)
				defer store.Close()
				other := models.BackupTask{ID: "other-job", Instance: "other", Node: job.Node, VMID: 103, Status: "running", ObservedAt: time.Now()}
				m.state.UpdateBackupTasksForInstance("other", []models.BackupTask{other})

				guests := func() []models.BackupTask {
					var result []models.BackupTask
					for _, task := range m.state.GetBackupTasks() {
						if task.Instance == "site" && task.VMID > 0 {
							result = append(result, task)
						}
						if task.Instance == "other" {
							require.Equal(t, other, task, "another connection must remain untouched")
						}
					}
					return result
				}
				// History intentionally ignores collections at or before its
				// millisecond watermark. These are distinct modeled poll cycles,
				// not duplicate observations within one persisted clock tick.
				poll := func() {
					t.Helper()
					coverage, err := store.BackupHistoryCoverage(ctx, job.StartTime, time.Now().UTC())
					require.NoError(t, err)
					for _, source := range coverage.Sources {
						if source.Provider == recovery.ProviderProxmoxPVE && source.Instance == "site" {
							if wait := time.Until(source.LastObservedAt.Add(time.Millisecond)); wait > 0 {
								time.Sleep(wait)
							}
							require.Greater(t, time.Now().UnixMilli(), source.LastObservedAt.UnixMilli())
						}
					}
					m.pollBackupTasks(ctx, "site", client)
				}
				checkStored := func(last recovery.Outcome, complete bool) {
					points, total, err := store.ListPoints(ctx, recovery.ListPointsOptions{Kind: recovery.KindOther})
					require.NoError(t, err)
					require.Equal(t, 4, total, "parent and three guest identities survive every poll")
					for _, point := range points {
						switch point.ID {
						case "pve-task:" + job.ID:
							require.Equal(t, complete, point.Details["guestTaskDetailsComplete"])
						case "pve-task:" + job.ID + "-vm101":
							require.Equal(t, recovery.OutcomeSuccess, point.Outcome)
						case "pve-task:" + job.ID + "-vm102":
							require.Equal(t, recovery.OutcomeFailed, point.Outcome)
						case "pve-task:" + job.ID + "-vm103":
							require.Equal(t, last, point.Outcome)
						default:
							t.Fatalf("unexpected point: %s", point.ID)
						}
					}
				}
				poll()
				assertVzdumpGuestResults(t, guests(), "running")
				checkStored(recovery.OutcomeRunning, false)
				_, active := m.resolveBackupIntentContext("", "site", job.Node, 103, time.Now())
				require.True(t, active, "an accepted live log still supplies backup intent")

				// Simulate passage of the evidence lease without a timing-dependent sleep.
				key := "site|" + testJobUPID
				entry := m.vzdumpJobTaskCache[key]
				observed := time.Now().UTC().Add(-2 * backupIntentEvidenceMaxAge)
				for i := range entry.tasks {
					require.False(t, entry.tasks[i].ObservedAt.IsZero(), "cache must retain its accepted log observation")
					entry.tasks[i].ObservedAt = observed
				}
				m.vzdumpJobTaskCache[key] = entry
				client.logErr = fmt.Errorf("task log %s", failure)
				poll()
				assertVzdumpGuestResults(t, guests(), "running")
				for _, task := range guests() {
					require.Equal(t, observed, task.ObservedAt, "failed reads cannot freshen the last log observation")
				}
				_, active = m.resolveBackupIntentContext("", "site", job.Node, 103, time.Now())
				require.False(t, active, "unread live logs must not indefinitely renew alert deferral")

				client.task.EndTime = job.EndTime.Unix()
				client.task.Status = parentStatus
				poll()
				assertVzdumpGuestResults(t, guests(), "unknown")
				for _, task := range guests() {
					require.Equal(t, observed, task.ObservedAt)
					require.False(t, task.EndTime.IsZero(), "ended parent must not leave an apparently active guest")
					if task.VMID == 103 {
						require.True(t, task.EndTime.Equal(job.EndTime), "compare the provider instant, not time.Location identity")
						require.Equal(t, "backup job ended without an observed guest result", task.Error)
					}
					_, active = m.resolveBackupIntentContext("", "site", job.Node, task.VMID, observed)
					require.False(t, active, "parent termination withdraws intent even while the cached observation is fresh")
				}
				checkStored(recovery.OutcomeUnknown, false)
				require.Equal(t, entry, m.vzdumpJobTaskCache[key], "fallback must not mutate/finalise accepted log data")
				require.Equal(t, 3, client.logRequests)

				// Only an ordinary accepted log supplies the missing guest verdict.
				client.logErr = nil
				client.logLines = append(lines, proxmox.TaskLogLine{LineNumber: 6, Text: "INFO: Finished Backup of VM 103 (00:02:00)"}, proxmox.TaskLogLine{LineNumber: 7, Text: "TASK ERROR: job errors"})
				poll()
				assertVzdumpGuestResults(t, guests(), "OK")
				checkStored(recovery.OutcomeSuccess, true)
				require.True(t, m.vzdumpJobTaskCache[key].final)
				for _, task := range guests() {
					require.True(t, task.ObservedAt.After(observed))
				}
				poll()
				require.Equal(t, 4, client.logRequests, "complete immutable logs still use the final cache")
				checkStored(recovery.OutcomeSuccess, true)
			})
		}
	}
}

func TestVzdumpCachedGuestUnreadLogUsesCurrentParent(t *testing.T) {
	for _, blocked := range []string{"budget", "cancelled"} {
		t.Run(blocked, func(t *testing.T) {
			job, lines := vzdumpGuestResultFixture()
			running := job
			running.EndTime = time.Time{}
			observed := time.Now().UTC().Add(-time.Minute)
			entry := vzdumpJobTaskCacheEntry{tasks: refreshObservedAt(parseVzdumpJobLog(running, testJobUPID, lines), observed)}
			m := &Monitor{vzdumpJobTaskCache: map[string]vzdumpJobTaskCacheEntry{"site|" + testJobUPID: entry}}
			client := &cachedVzdumpClient{}
			ctx := context.Background()
			jobs := []models.BackupTask{}
			upids := map[string]string{job.ID: testJobUPID}
			if blocked == "budget" {
				for i := 0; i < maxVzdumpJobLogFetchesPerPoll; i++ {
					filler := models.BackupTask{ID: fmt.Sprintf("filler-%d", i)}
					jobs = append(jobs, filler)
					upids[filler.ID] = filler.ID
				}
			} else {
				cancelled, cancel := context.WithCancel(ctx)
				cancel()
				ctx = cancelled
			}
			jobs = append(jobs, job)
			tasks := m.synthesizeVzdumpJobGuestTasks(ctx, "site", client, jobs, upids)
			assertVzdumpGuestResults(t, tasks, "unknown")
			for _, task := range tasks {
				require.Equal(t, observed, task.ObservedAt, "an unread log is not a fresh guest observation")
				require.False(t, task.EndTime.IsZero())
			}
			require.Equal(t, entry, m.vzdumpJobTaskCache["site|"+testJobUPID])
			wantRequests := 0
			if blocked == "budget" {
				wantRequests = maxVzdumpJobLogFetchesPerPoll
			}
			require.Equal(t, wantRequests, client.logRequests, "repair must not bypass request or cancellation bounds")
		})
	}
}

// Pin the distinction behind the cached-guest watchdog failure without relying
// on machine speed, sleeps, repeated test executions or a changed store clock.
// Live state can accept a parent/log transition before the collection timestamp
// advances far enough to be a distinct durable observation.
func TestVzdumpCachedGuestHistoryCollectionPrecision(t *testing.T) {
	ctx := context.Background()
	job, lines := vzdumpGuestResultFixture()
	ended := job
	job.Status, job.EndTime = "running", time.Time{}
	client := &cachedVzdumpClient{jobBackupTaskClient: jobBackupTaskClient{logLines: lines}}
	manager := recoverymanager.New(config.NewMultiTenantPersistence(t.TempDir()))
	m := &Monitor{recoveryManager: manager}
	store, err := manager.StoreForOrg("default")
	require.NoError(t, err)
	defer store.Close()
	at := time.Now().UTC().Add(-time.Minute).Truncate(time.Millisecond)
	collection := model.BackupHistoryCollection{
		Provider: recovery.ProviderProxmoxPVE, Instance: "site",
		ObservedAt: at, Complete: true, MaxGap: time.Minute,
	}
	upids := map[string]string{job.ID: testJobUPID}
	capture := func(phase, liveStatus string, storedOutcome recovery.Outcome, complete bool) {
		t.Helper()
		guests := m.synthesizeVzdumpJobGuestTasks(ctx, "site", client, []models.BackupTask{job}, upids)
		assertVzdumpGuestResults(t, guests, liveStatus)
		points, err := proxmoxmapper.FromPVEBackupTasksWithEvidence(append([]models.BackupTask{job}, guests...), nil, collection.ObservedAt)
		require.NoError(t, err)
		m.qualifyBackupGuestTaskDetails("site", []models.BackupTask{job}, upids, points)
		m.persistBackupTaskHistory(ctx, points, collection)
		stored, total, err := store.ListPoints(ctx, recovery.ListPointsOptions{Kind: recovery.KindOther})
		require.NoError(t, err)
		require.Equal(t, 4, total)
		for _, point := range stored {
			switch point.ID {
			case "pve-task:" + job.ID:
				require.Equal(t, complete, point.Details["guestTaskDetailsComplete"], phase)
			case "pve-task:" + job.ID + "-vm101":
				require.Equal(t, recovery.OutcomeSuccess, point.Outcome, phase)
			case "pve-task:" + job.ID + "-vm102":
				require.Equal(t, recovery.OutcomeFailed, point.Outcome, phase)
			case "pve-task:" + job.ID + "-vm103":
				require.Equal(t, storedOutcome, point.Outcome, phase)
			default:
				t.Fatalf("%s: unexpected point %q", phase, point.ID)
			}
		}
		t.Logf("%s collection=%s live=%s durable=%s complete=%t", phase, collection.ObservedAt.Format(time.RFC3339Nano), liveStatus, storedOutcome, complete)
	}
	capture("running", "running", recovery.OutcomeRunning, false)
	job = ended
	client.logErr = fmt.Errorf("task log 503 unavailable")
	collection.ObservedAt = at.Add(time.Millisecond - time.Nanosecond)
	capture("ended-same-tick", "unknown", recovery.OutcomeRunning, false)
	collection.ObservedAt = at.Add(time.Millisecond)
	capture("ended-next-tick", "unknown", recovery.OutcomeUnknown, false)
	client.logErr = nil
	client.logLines = append(lines, proxmox.TaskLogLine{LineNumber: 6, Text: "INFO: Finished Backup of VM 103 (00:02:00)"}, proxmox.TaskLogLine{LineNumber: 7, Text: "TASK ERROR: job errors"})
	collection.ObservedAt = at.Add(2*time.Millisecond - time.Nanosecond)
	capture("complete-same-tick", "OK", recovery.OutcomeUnknown, false)
	collection.ObservedAt = at.Add(2 * time.Millisecond)
	capture("complete-next-tick", "OK", recovery.OutcomeSuccess, true)
	require.True(t, m.vzdumpJobTaskCache["site|"+testJobUPID].final)
	require.Equal(t, 4, client.logRequests, "the complete log remains immutable after recovery")
}
