//go:build integration

package monitoring

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pmg"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// FailureType describes scripted failure behaviour used by the harness.
type FailureType int

const (
	FailureNone FailureType = iota
	FailureTransient
	FailurePermanent
)

// HarnessScenario captures the configuration for an integration run.
type HarnessScenario struct {
	Instances      []InstanceConfig
	Duration       time.Duration
	WarmupDuration time.Duration
}

// InstanceConfig models a single synthetic instance executed during the run.
type InstanceConfig struct {
	Type        string
	Name        string
	SuccessRate float64
	FailureSeq  []FailureType
	BaseLatency time.Duration
}

// InstanceStats aggregates execution data per instance.
type InstanceStats struct {
	Total             int
	Successes         int
	Failures          int
	TransientFailures int
	PermanentFailures int
	AverageLatency    time.Duration
	LastError         string
	LastSuccessAt     time.Time
}

// QueueStats summarises task queue behaviour.
type QueueStats struct {
	MaxDepth     int
	AverageDepth float64
	Samples      int
	FinalDepth   int
}

// StalenessStats captures staleness score distribution.
type StalenessStats struct {
	Max     float64
	Average float64
	Samples int
}

// ResourceStats samples runtime resource usage.
type ResourceStats struct {
	GoroutinesStart int
	GoroutinesEnd   int
	HeapAllocStart  uint64
	HeapAllocEnd    uint64
	StackInuseStart uint64
	StackInuseEnd   uint64
	GCCountStart    uint32
	GCCountEnd      uint32
}

// HarnessReport is returned after a harness run completes.
type HarnessReport struct {
	Scenario         HarnessScenario
	PerInstanceStats map[string]InstanceStats
	QueueStats       QueueStats
	StalenessStats   StalenessStats
	ResourceStats    ResourceStats
	Health           SchedulerHealthResponse
	MaxStaleness     time.Duration
	RuntimeSamples   []runtimeSnapshot
}

// Harness orchestrates the integration run.
type Harness struct {
	Monitor           *Monitor
	Executor          *fakeExecutor
	cancel            context.CancelFunc
	scenario          HarnessScenario
	dataPath          string
	queueMax          int
	queueSum          int
	queueSamples      int
	maxStaleness      time.Duration
	sampleEvery       time.Duration
	runtimeSamples    []runtimeSnapshot
	lastRuntimeSample time.Time
}

// NewHarness constructs a harness configured for the provided scenario.
func NewHarness(scenario HarnessScenario) *Harness {
	if scenario.Duration <= 0 {
		scenario.Duration = 30 * time.Second
	}
	if scenario.WarmupDuration <= 0 {
		scenario.WarmupDuration = 5 * time.Second
	}

	tempDir, err := os.MkdirTemp("", "pulse-harness-*")
	if err != nil {
		panic(fmt.Errorf("create harness data dir: %w", err))
	}

	baseInterval := 3 * time.Second
	minInterval := 750 * time.Millisecond
	maxInterval := 8 * time.Second

	cfg := &config.Config{
		DataPath:                    tempDir,
		AdaptivePollingEnabled:      true,
		AdaptivePollingBaseInterval: baseInterval,
		AdaptivePollingMinInterval:  minInterval,
		AdaptivePollingMaxInterval:  maxInterval,
		FrontendPort:                7655,
		PublicURL:                   "http://127.0.0.1",
	}

	monitor, err := New(cfg)
	if err != nil {
		panic(fmt.Errorf("create monitor for harness: %w", err))
	}

	// Populate synthetic client entries so scheduler inventory is aware of instances.
	for _, inst := range scenario.Instances {
		switch strings.ToLower(inst.Type) {
		case "pve":
			monitor.pveClients[inst.Name] = noopPVEClient{}
		case "pbs":
			monitor.pbsClients[inst.Name] = &pbs.Client{}
		case "pmg":
			monitor.pmgClients[inst.Name] = &pmg.Client{}
		default:
			// Unsupported types are ignored for now.
		}
	}

	exec := newFakeExecutor(monitor, scenario)
	monitor.SetExecutor(exec)

	return &Harness{
		Monitor:      monitor,
		Executor:     exec,
		scenario:     scenario,
		dataPath:     tempDir,
		maxStaleness: cfg.AdaptivePollingMaxInterval,
		sampleEvery:  15 * time.Second,
	}
}

// Run executes the scenario and returns a report of collected statistics.
func (h *Harness) Run(ctx context.Context) HarnessReport {
	h.runtimeSamples = nil
	runtimeStart := sampleRuntime()
	h.runtimeSamples = append(h.runtimeSamples, runtimeStart)
	h.lastRuntimeSample = runtimeStart.Timestamp

	runCtx, cancel := context.WithCancel(ctx)
	h.cancel = cancel

	workerCount := len(h.scenario.Instances)
	if workerCount < 1 {
		workerCount = 1
	}

	h.Monitor.startTaskWorkers(runCtx, workerCount)
	h.schedule(time.Now())

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	runEnd := time.Now().Add(h.scenario.WarmupDuration + h.scenario.Duration)

loop:
	for {
		select {
		case <-runCtx.Done():
			break loop
		case <-ticker.C:
			now := time.Now()
			h.schedule(now)
			if h.sampleEvery > 0 && time.Since(h.lastRuntimeSample) >= h.sampleEvery {
				h.recordRuntimeSample()
			}
			if now.After(runEnd) {
				cancel()
			}
		}
	}

	// Allow in-flight work to finish.
	time.Sleep(500 * time.Millisecond)

	instanceStats := h.Executor.InstanceReport()
	queueAverage := 0.0
	if h.queueSamples > 0 {
		queueAverage = float64(h.queueSum) / float64(h.queueSamples)
	}

	finalQueueDepth := h.Monitor.taskQueue.Size()
	health := h.Monitor.SchedulerHealth()
	runtimeEnd := sampleRuntime()
	h.runtimeSamples = append(h.runtimeSamples, runtimeEnd)
	staleness := computeStalenessStats(h.Monitor)

	h.Monitor.Stop()
	runtimeSamplesCopy := append([]runtimeSnapshot(nil), h.runtimeSamples...)
	h.cleanup()
	if len(runtimeSamplesCopy) == 0 {
		runtimeSamplesCopy = append(runtimeSamplesCopy, runtimeStart, runtimeEnd)
	}
	startSample := runtimeSamplesCopy[0]
	endSample := runtimeSamplesCopy[len(runtimeSamplesCopy)-1]

	report := HarnessReport{
		Scenario:         h.scenario,
		PerInstanceStats: instanceStats,
		QueueStats: QueueStats{
			MaxDepth:     h.queueMax,
			AverageDepth: queueAverage,
			Samples:      h.queueSamples,
			FinalDepth:   finalQueueDepth,
		},
		StalenessStats: staleness,
		ResourceStats: ResourceStats{
			GoroutinesStart: startSample.Goroutines,
			GoroutinesEnd:   endSample.Goroutines,
			HeapAllocStart:  startSample.HeapAlloc,
			HeapAllocEnd:    endSample.HeapAlloc,
			StackInuseStart: startSample.StackInuse,
			StackInuseEnd:   endSample.StackInuse,
			GCCountStart:    startSample.NumGC,
			GCCountEnd:      endSample.NumGC,
		},
		Health:         health,
		MaxStaleness:   h.maxStaleness,
		RuntimeSamples: runtimeSamplesCopy,
	}

	return report
}

func (h *Harness) schedule(now time.Time) {
	if h.Monitor == nil {
		return
	}

	currentDepth := h.Monitor.taskQueue.Size()
	if currentDepth > 0 {
		h.recordQueueDepth(currentDepth)
		return
	}

	tasks := h.Monitor.buildScheduledTasks(now)
	if len(tasks) == 0 {
		h.recordQueueDepth(currentDepth)
		return
	}

	for _, task := range tasks {
		h.Monitor.taskQueue.Upsert(task)
	}

	h.recordQueueDepth(h.Monitor.taskQueue.Size())
}

func (h *Harness) recordQueueDepth(depth int) {
	h.queueSamples++
	h.queueSum += depth
	if depth > h.queueMax {
		h.queueMax = depth
	}
	if h.Monitor != nil && h.Monitor.pollMetrics != nil {
		h.Monitor.pollMetrics.SetQueueDepth(depth)
	}
}

func (h *Harness) recordRuntimeSample() {
	snap := sampleRuntime()
	h.runtimeSamples = append(h.runtimeSamples, snap)
	h.lastRuntimeSample = snap.Timestamp
}

func (h *Harness) cleanup() {
	if h.cancel != nil {
		h.cancel()
		h.cancel = nil
	}
	h.runtimeSamples = nil
	h.lastRuntimeSample = time.Time{}
	if h.dataPath != "" {
		_ = os.RemoveAll(h.dataPath)
		h.dataPath = ""
	}
}

type runtimeSnapshot struct {
	Timestamp  time.Time
	Goroutines int
	HeapAlloc  uint64
	StackInuse uint64
	NumGC      uint32
}

func sampleRuntime() runtimeSnapshot {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	return runtimeSnapshot{
		Timestamp:  time.Now(),
		Goroutines: runtime.NumGoroutine(),
		HeapAlloc:  ms.HeapAlloc,
		StackInuse: ms.StackInuse,
		NumGC:      ms.NumGC,
	}
}

func computeStalenessStats(m *Monitor) StalenessStats {
	if m == nil || m.stalenessTracker == nil {
		return StalenessStats{}
	}

	snapshots := m.stalenessTracker.Snapshot()
	if len(snapshots) == 0 {
		return StalenessStats{}
	}

	var sum float64
	maxScore := 0.0
	for _, snap := range snapshots {
		sum += snap.Score
		if snap.Score > maxScore {
			maxScore = snap.Score
		}
	}

	avg := sum / float64(len(snapshots))
	return StalenessStats{
		Max:     maxScore,
		Average: avg,
		Samples: len(snapshots),
	}
}

func toInstanceType(value string) InstanceType {
	switch strings.ToLower(value) {
	case "pve":
		return InstanceTypePVE
	case "pbs":
		return InstanceTypePBS
	case "pmg":
		return InstanceTypePMG
	default:
		return InstanceType(strings.ToLower(value))
	}
}

type noopPVEClient struct{}

func (noopPVEClient) GetNodes(ctx context.Context) ([]proxmox.Node, error) { return nil, nil }
func (noopPVEClient) GetNodeStatus(ctx context.Context, node string) (*proxmox.NodeStatus, error) {
	return nil, nil
}
func (noopPVEClient) GetNodeRRDData(ctx context.Context, node string, timeframe string, cf string, ds []string) ([]proxmox.NodeRRDPoint, error) {
	return nil, nil
}
func (noopPVEClient) GetLXCRRDData(ctx context.Context, node string, vmid int, timeframe string, cf string, ds []string) ([]proxmox.GuestRRDPoint, error) {
	return nil, nil
}
func (noopPVEClient) GetVMRRDData(ctx context.Context, node string, vmid int, timeframe string, cf string, ds []string) ([]proxmox.GuestRRDPoint, error) {
	return nil, nil
}
func (noopPVEClient) GetVMs(ctx context.Context, node string) ([]proxmox.VM, error) { return nil, nil }
func (noopPVEClient) GetContainers(ctx context.Context, node string) ([]proxmox.Container, error) {
	return nil, nil
}
func (noopPVEClient) GetStorage(ctx context.Context, node string) ([]proxmox.Storage, error) {
	return nil, nil
}
func (noopPVEClient) GetAllStorage(ctx context.Context) ([]proxmox.Storage, error) { return nil, nil }
func (noopPVEClient) GetBackupTasks(ctx context.Context) ([]proxmox.Task, error)   { return nil, nil }
func (noopPVEClient) GetReplicationStatus(ctx context.Context) ([]proxmox.ReplicationJob, error) {
	return nil, nil
}
func (noopPVEClient) GetStorageContent(ctx context.Context, node, storage string) ([]proxmox.StorageContent, error) {
	return nil, nil
}
func (noopPVEClient) GetVMSnapshots(ctx context.Context, node string, vmid int) ([]proxmox.Snapshot, error) {
	return nil, nil
}
func (noopPVEClient) GetContainerSnapshots(ctx context.Context, node string, vmid int) ([]proxmox.Snapshot, error) {
	return nil, nil
}
func (noopPVEClient) GetVMStatus(ctx context.Context, node string, vmid int) (*proxmox.VMStatus, error) {
	return nil, nil
}
func (noopPVEClient) GetContainerStatus(ctx context.Context, node string, vmid int) (*proxmox.Container, error) {
	return nil, nil
}
func (noopPVEClient) GetContainerConfig(ctx context.Context, node string, vmid int) (map[string]interface{}, error) {
	return nil, nil
}
func (noopPVEClient) GetContainerInterfaces(ctx context.Context, node string, vmid int) ([]proxmox.ContainerInterface, error) {
	return nil, nil
}
func (noopPVEClient) GetClusterResources(ctx context.Context, resourceType string) ([]proxmox.ClusterResource, error) {
	return nil, nil
}
func (noopPVEClient) IsClusterMember(ctx context.Context) (bool, error) { return false, nil }
func (noopPVEClient) GetVMFSInfo(ctx context.Context, node string, vmid int) ([]proxmox.VMFileSystem, error) {
	return nil, nil
}
func (noopPVEClient) GetVMNetworkInterfaces(ctx context.Context, node string, vmid int) ([]proxmox.VMNetworkInterface, error) {
	return nil, nil
}
func (noopPVEClient) GetVMAgentInfo(ctx context.Context, node string, vmid int) (map[string]interface{}, error) {
	return map[string]interface{}{}, nil
}

func (noopPVEClient) GetVMAgentVersion(ctx context.Context, node string, vmid int) (string, error) {
	return "", nil
}
func (noopPVEClient) GetZFSPoolStatus(ctx context.Context, node string) ([]proxmox.ZFSPoolStatus, error) {
	return nil, nil
}
func (noopPVEClient) GetZFSPoolsWithDetails(ctx context.Context, node string) ([]proxmox.ZFSPoolInfo, error) {
	return nil, nil
}
func (noopPVEClient) GetDisks(ctx context.Context, node string) ([]proxmox.Disk, error) {
	return nil, nil
}
func (noopPVEClient) GetCephStatus(ctx context.Context) (*proxmox.CephStatus, error) { return nil, nil }
func (noopPVEClient) GetCephDF(ctx context.Context) (*proxmox.CephDF, error)         { return nil, nil }
