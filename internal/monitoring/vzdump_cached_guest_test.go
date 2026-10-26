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
				m.pollBackupTasks(ctx, "site", client)
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
				m.pollBackupTasks(ctx, "site", client)
				assertVzdumpGuestResults(t, guests(), "running")
				for _, task := range guests() {
					require.Equal(t, observed, task.ObservedAt, "failed reads cannot freshen the last log observation")
				}
				_, active = m.resolveBackupIntentContext("", "site", job.Node, 103, time.Now())
				require.False(t, active, "unread live logs must not indefinitely renew alert deferral")

				client.task.EndTime = job.EndTime.Unix()
				client.task.Status = parentStatus
				m.pollBackupTasks(ctx, "site", client)
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
				m.pollBackupTasks(ctx, "site", client)
				assertVzdumpGuestResults(t, guests(), "OK")
				checkStored(recovery.OutcomeSuccess, true)
				require.True(t, m.vzdumpJobTaskCache[key].final)
				for _, task := range guests() {
					require.True(t, task.ObservedAt.After(observed))
				}
				m.pollBackupTasks(ctx, "site", client)
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
