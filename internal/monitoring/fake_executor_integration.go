//go:build integration

package monitoring

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	internalerrors "github.com/rcourtman/pulse-go-rewrite/internal/monitoring/errors"
)

type fakeExecutor struct {
	monitor *Monitor
	configs map[string]InstanceConfig
	mu      sync.Mutex
	state   map[string]*instanceState
	rng     *rand.Rand
}

type instanceState struct {
	config       InstanceConfig
	seqIndex     int
	successes    int
	failures     int
	transient    int
	permanent    int
	totalLatency time.Duration
	executions   int
	lastError    string
	lastSuccess  time.Time
}

func newFakeExecutor(m *Monitor, scenario HarnessScenario) *fakeExecutor {
	cfgs := make(map[string]InstanceConfig, len(scenario.Instances))
	for _, inst := range scenario.Instances {
		key := instanceKey(inst.Type, inst.Name)
		cfgs[key] = inst
	}

	return &fakeExecutor{
		monitor: m,
		configs: cfgs,
		state:   make(map[string]*instanceState, len(cfgs)),
		rng:     rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (f *fakeExecutor) Execute(ctx context.Context, task PollTask) {
	start := time.Now()
	key := instanceKey(task.InstanceType, task.InstanceName)
	cfg, found := f.configs[key]
	if !found {
		cfg = InstanceConfig{
			Type:        task.InstanceType,
			Name:        task.InstanceName,
			SuccessRate: 1.0,
		}
	}

	state := f.getState(key, cfg)
	latency := f.latencyFor(cfg)

	select {
	case <-ctx.Done():
		return
	case <-time.After(latency):
	}

	failType := f.nextFailure(state, cfg)
	success := failType == FailureNone

	var pollErr error
	if !success {
		err := fmt.Errorf("synthetic failure on %s", task.InstanceName)
		switch failType {
		case FailureTransient:
			pollErr = internalerrors.NewMonitorError(internalerrors.ErrorTypeConnection, "fake_poll", task.InstanceName, err)
		case FailurePermanent:
			pollErr = internalerrors.NewMonitorError(internalerrors.ErrorTypeValidation, "fake_poll", task.InstanceName, err)
		default:
			pollErr = internalerrors.NewMonitorError(internalerrors.ErrorTypeInternal, "fake_poll", task.InstanceName, err)
		}
	}

	result := PollResult{
		InstanceName: task.InstanceName,
		InstanceType: task.InstanceType,
		Success:      success,
		Error:        pollErr,
		StartTime:    start,
		EndTime:      time.Now(),
	}

	if f.monitor.pollMetrics != nil {
		f.monitor.pollMetrics.RecordResult(result)
	}

	instanceType := toInstanceType(task.InstanceType)
	if f.monitor.stalenessTracker != nil {
		if success {
			f.monitor.stalenessTracker.UpdateSuccess(instanceType, task.InstanceName, nil)
		} else {
			f.monitor.stalenessTracker.UpdateError(instanceType, task.InstanceName)
		}
	}

	f.monitor.recordTaskResult(instanceType, task.InstanceName, pollErr)

	f.recordStats(state, latency, success, failType, pollErr)
}

func (f *fakeExecutor) InstanceReport() map[string]InstanceStats {
	f.mu.Lock()
	defer f.mu.Unlock()

	report := make(map[string]InstanceStats, len(f.state))
	for key, st := range f.state {
		avgLatency := time.Duration(0)
		if st.executions > 0 {
			avgLatency = st.totalLatency / time.Duration(st.executions)
		}
		report[key] = InstanceStats{
			Total:             st.executions,
			Successes:         st.successes,
			Failures:          st.failures,
			TransientFailures: st.transient,
			PermanentFailures: st.permanent,
			AverageLatency:    avgLatency,
			LastError:         st.lastError,
			LastSuccessAt:     st.lastSuccess,
		}
	}

	return report
}

func (f *fakeExecutor) getState(key string, cfg InstanceConfig) *instanceState {
	f.mu.Lock()
	defer f.mu.Unlock()

	if st, ok := f.state[key]; ok {
		return st
	}

	st := &instanceState{config: cfg}
	f.state[key] = st
	return st
}

func (f *fakeExecutor) latencyFor(cfg InstanceConfig) time.Duration {
	base := cfg.BaseLatency
	if base <= 0 {
		base = 200 * time.Millisecond
	}

	jitter := base / 5
	if jitter <= 0 {
		return base
	}

	offset := time.Duration(f.rng.Int63n(int64(jitter))) - jitter/2
	return base + offset
}

func (f *fakeExecutor) nextFailure(state *instanceState, cfg InstanceConfig) FailureType {
	if state.seqIndex < len(cfg.FailureSeq) {
		ft := cfg.FailureSeq[state.seqIndex]
		state.seqIndex++
		return ft
	}

	successRate := cfg.SuccessRate
	if successRate <= 0 {
		return FailureTransient
	}
	if successRate >= 1 {
		return FailureNone
	}

	if f.rng.Float64() <= successRate {
		return FailureNone
	}
	return FailureTransient
}

func (f *fakeExecutor) recordStats(state *instanceState, latency time.Duration, success bool, failure FailureType, pollErr error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	state.executions++
	state.totalLatency += latency

	if success {
		state.successes++
		state.lastError = ""
		state.lastSuccess = time.Now()
		return
	}

	state.failures++
	if failure == FailureTransient {
		state.transient++
	} else if failure == FailurePermanent {
		state.permanent++
	}
	if pollErr != nil {
		state.lastError = pollErr.Error()
	}
}

func instanceKey(typ, name string) string {
	return fmt.Sprintf("%s::%s", strings.ToLower(typ), name)
}
