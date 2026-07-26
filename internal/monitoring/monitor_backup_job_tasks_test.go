package monitoring

import (
	"context"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

const testJobUPID = "UPID:pve1:000E9F2C:0AC734B2:68A1B2C3:vzdump::root@pam:"

func testLogLines(lines ...string) []proxmox.TaskLogLine {
	out := make([]proxmox.TaskLogLine, 0, len(lines))
	for i, line := range lines {
		out = append(out, proxmox.TaskLogLine{LineNumber: int64(i + 1), Text: line})
	}
	return out
}

func TestParseVzdumpJobLogFinishedJob(t *testing.T) {
	start := time.Date(2026, 7, 25, 2, 0, 0, 0, time.UTC)
	jobTask := models.BackupTask{
		ID:        "pve-cluster-" + testJobUPID,
		Node:      "pve1",
		Instance:  "pve-cluster",
		Type:      "vzdump",
		Status:    "job errors",
		StartTime: start,
		EndTime:   start.Add(10 * time.Minute),
	}

	tasks := parseVzdumpJobLog(jobTask, testJobUPID, testLogLines(
		"INFO: starting new backup job: vzdump 101 102 103 --mode snapshot --storage pbs",
		"INFO: Starting Backup of VM 101 (qemu)",
		"INFO: creating Proxmox Backup Server archive 'vm/101/2026-07-25T02:00:01Z'",
		"INFO: Finished Backup of VM 101 (00:02:30)",
		"INFO: Starting Backup of VM 102 (lxc)",
		"ERROR: Backup of VM 102 failed - command 'rsync' failed: exit code 23",
		"INFO: Starting Backup of VM 103 (qemu)",
		"INFO: Finished Backup of VM 103 (00:01:00)",
		"INFO: Backup job finished with errors",
	))

	if len(tasks) != 3 {
		t.Fatalf("expected 3 synthetic guest tasks, got %d", len(tasks))
	}

	vm101 := tasks[0]
	if vm101.VMID != 101 || vm101.Status != "OK" || vm101.Error != "" {
		t.Errorf("vm101 = %+v, want VMID 101 status OK with no error", vm101)
	}
	if vm101.ID != "pve-cluster-"+testJobUPID+"-vm101" {
		t.Errorf("vm101 ID = %q, want parent UPID embedded", vm101.ID)
	}
	if !vm101.StartTime.Equal(start) || !vm101.EndTime.Equal(start.Add(2*time.Minute+30*time.Second)) {
		t.Errorf("vm101 times = %v..%v, want job start plus printed duration", vm101.StartTime, vm101.EndTime)
	}

	vm102 := tasks[1]
	if vm102.VMID != 102 || vm102.Status != "error" {
		t.Errorf("vm102 = %+v, want VMID 102 status error", vm102)
	}
	if vm102.Error != "command 'rsync' failed: exit code 23" {
		t.Errorf("vm102 error = %q, want the reason from the log line", vm102.Error)
	}
	if !vm102.EndTime.Equal(jobTask.EndTime) {
		t.Errorf("vm102 end = %v, want parent job end for failed guest", vm102.EndTime)
	}

	vm103 := tasks[2]
	if vm103.VMID != 103 || vm103.Status != "OK" {
		t.Errorf("vm103 = %+v, want VMID 103 status OK", vm103)
	}
	// vm102 failed without a duration line, so vm103 starts from vm101's end.
	wantStart := start.Add(2*time.Minute + 30*time.Second)
	if !vm103.StartTime.Equal(wantStart) || !vm103.EndTime.Equal(wantStart.Add(time.Minute)) {
		t.Errorf("vm103 times = %v..%v, want %v..%v", vm103.StartTime, vm103.EndTime, wantStart, wantStart.Add(time.Minute))
	}

	for _, task := range tasks {
		if task.Node != "pve1" || task.Instance != "pve-cluster" || task.Type != "vzdump" {
			t.Errorf("task %d inherits wrong parent metadata: %+v", task.VMID, task)
		}
	}
}

func TestParseVzdumpJobLogRunningJob(t *testing.T) {
	start := time.Date(2026, 7, 25, 2, 0, 0, 0, time.UTC)
	jobTask := models.BackupTask{
		ID:        "pve-cluster-" + testJobUPID,
		Node:      "pve1",
		Instance:  "pve-cluster",
		Type:      "vzdump",
		StartTime: start,
	}

	tasks := parseVzdumpJobLog(jobTask, testJobUPID, testLogLines(
		"INFO: Starting Backup of VM 101 (qemu)",
		"INFO: Finished Backup of VM 101 (00:00:45)",
		"INFO: Starting Backup of VM 102 (lxc)",
	))

	if len(tasks) != 2 {
		t.Fatalf("expected 2 synthetic guest tasks, got %d", len(tasks))
	}
	if tasks[0].Status != "OK" || tasks[0].EndTime.IsZero() {
		t.Errorf("finished guest in running job = %+v, want status OK with end time", tasks[0])
	}
	if tasks[1].Status != "running" || !tasks[1].EndTime.IsZero() {
		t.Errorf("in-progress guest = %+v, want status running with zero end time", tasks[1])
	}
}

func TestParseVzdumpJobLogJobDiedMidGuest(t *testing.T) {
	start := time.Date(2026, 7, 25, 2, 0, 0, 0, time.UTC)
	jobTask := models.BackupTask{
		ID:        "pve-cluster-" + testJobUPID,
		Node:      "pve1",
		Instance:  "pve-cluster",
		Type:      "vzdump",
		Status:    "unexpected status",
		StartTime: start,
		EndTime:   start.Add(time.Minute),
	}

	tasks := parseVzdumpJobLog(jobTask, testJobUPID, testLogLines(
		"INFO: Starting Backup of VM 101 (qemu)",
	))

	if len(tasks) != 1 {
		t.Fatalf("expected 1 synthetic guest task, got %d", len(tasks))
	}
	if tasks[0].Status != "error" || tasks[0].Error == "" {
		t.Errorf("guest in dead job = %+v, want status error with message", tasks[0])
	}
	if !tasks[0].EndTime.Equal(jobTask.EndTime) {
		t.Errorf("guest end = %v, want parent end time", tasks[0].EndTime)
	}
}

func TestParseVzdumpJobLogNoGuestMarkers(t *testing.T) {
	jobTask := models.BackupTask{
		ID:        "pve-cluster-" + testJobUPID,
		Instance:  "pve-cluster",
		Type:      "vzdump",
		StartTime: time.Now(),
	}
	if tasks := parseVzdumpJobLog(jobTask, testJobUPID, testLogLines("INFO: nothing to do")); tasks != nil {
		t.Errorf("expected no synthetic tasks, got %+v", tasks)
	}
}

// jobBackupTaskClient serves a multi-guest vzdump job task plus its log.
// Only the two methods pollBackupTasks uses are implemented; the embedded
// interface satisfies the rest.
type jobBackupTaskClient struct {
	PVEClientInterface
	task        proxmox.Task
	logLines    []proxmox.TaskLogLine
	logRequests int
}

func (c *jobBackupTaskClient) GetBackupTasks(ctx context.Context) ([]proxmox.Task, error) {
	return []proxmox.Task{c.task}, nil
}

func (c *jobBackupTaskClient) GetTaskLog(ctx context.Context, node, upid string) ([]proxmox.TaskLogLine, error) {
	c.logRequests++
	return c.logLines, nil
}

func TestPollBackupTasksSynthesizesJobGuestTasks(t *testing.T) {
	start := time.Now().Add(-time.Hour)
	client := &jobBackupTaskClient{
		task: proxmox.Task{
			UPID:      testJobUPID,
			Node:      "pve1",
			Type:      "vzdump",
			ID:        "", // job-run UPIDs carry no VMID
			Status:    "OK",
			StartTime: start.Unix(),
			EndTime:   start.Add(5 * time.Minute).Unix(),
		},
		logLines: testLogLines(
			"INFO: Starting Backup of VM 101 (qemu)",
			"INFO: Finished Backup of VM 101 (00:02:00)",
			"INFO: Starting Backup of VM 102 (lxc)",
			"INFO: Finished Backup of VM 102 (00:01:00)",
		),
	}

	m := &Monitor{state: models.NewState()}
	m.pollBackupTasks(context.Background(), "pve-test", client)

	state := m.state.GetSnapshot()
	if len(state.PVEBackups.BackupTasks) != 3 {
		t.Fatalf("expected job task plus 2 synthetic guest tasks, got %d: %+v", len(state.PVEBackups.BackupTasks), state.PVEBackups.BackupTasks)
	}

	byVMID := make(map[int]models.BackupTask)
	for _, task := range state.PVEBackups.BackupTasks {
		byVMID[task.VMID] = task
	}
	for _, vmid := range []int{101, 102} {
		task, ok := byVMID[vmid]
		if !ok {
			t.Fatalf("no synthetic task for VM %d", vmid)
		}
		if task.Status != "OK" {
			t.Errorf("VM %d status = %q, want OK", vmid, task.Status)
		}
		if task.ObservedAt.IsZero() {
			t.Errorf("VM %d has no observation time", vmid)
		}
	}
	if _, ok := byVMID[0]; !ok {
		t.Error("parent job task should remain in state")
	}

	if client.logRequests != 1 {
		t.Fatalf("expected 1 log fetch, got %d", client.logRequests)
	}

	// A finished job's log is immutable: the second poll must serve the
	// synthetic tasks from cache without refetching, refreshing ObservedAt.
	m.pollBackupTasks(context.Background(), "pve-test", client)
	if client.logRequests != 1 {
		t.Errorf("expected cached synthesis on second poll, got %d log fetches", client.logRequests)
	}
	state = m.state.GetSnapshot()
	if len(state.PVEBackups.BackupTasks) != 3 {
		t.Errorf("expected stable task count after second poll, got %d", len(state.PVEBackups.BackupTasks))
	}
}

func TestPollBackupTasksRunningJobRefetchesAndSuppressesAlerts(t *testing.T) {
	start := time.Now().Add(-2 * time.Minute)
	client := &jobBackupTaskClient{
		task: proxmox.Task{
			UPID:      testJobUPID,
			Node:      "pve1",
			Type:      "vzdump",
			ID:        "",
			StartTime: start.Unix(),
			// running: no EndTime, no Status
		},
		logLines: testLogLines(
			"INFO: Starting Backup of VM 101 (qemu)",
		),
	}

	m := &Monitor{state: models.NewState()}
	m.pollBackupTasks(context.Background(), "pve-test", client)
	m.pollBackupTasks(context.Background(), "pve-test", client)

	if client.logRequests != 2 {
		t.Errorf("running job should be re-parsed every poll, got %d log fetches", client.logRequests)
	}

	state := m.state.GetSnapshot()
	var running *models.BackupTask
	for i := range state.PVEBackups.BackupTasks {
		if state.PVEBackups.BackupTasks[i].VMID == 101 {
			running = &state.PVEBackups.BackupTasks[i]
		}
	}
	if running == nil {
		t.Fatal("no synthetic task for VM 101")
	}
	if running.Status != "running" {
		t.Errorf("status = %q, want running", running.Status)
	}

	intent, ok := m.resolveBackupIntentContext("", "pve-test", "pve1", 101, time.Now())
	if !ok || !intent.Active {
		t.Errorf("expected active backup intent for guest covered by running job, got ok=%v intent=%+v", ok, intent)
	}
	if _, ok := m.resolveBackupIntentContext("", "pve-test", "pve1", 999, time.Now()); ok {
		t.Error("guest not covered by the job should have no backup intent")
	}
}
