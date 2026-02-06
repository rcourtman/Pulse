package ai

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// --- filterStateByScope tests ---

func TestFilterStateByScope_NoScope(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{
		Nodes: []models.Node{{ID: "n1", Name: "node-1"}},
		VMs:   []models.VM{{ID: "vm1", Name: "vm-1"}},
	}
	scope := PatrolScope{} // no resource IDs or types

	filtered := ps.filterStateByScope(state, scope)

	if len(filtered.Nodes) != 1 {
		t.Errorf("expected 1 node with no scope filter, got %d", len(filtered.Nodes))
	}
	if len(filtered.VMs) != 1 {
		t.Errorf("expected 1 VM with no scope filter, got %d", len(filtered.VMs))
	}
}

func TestFilterStateByScope_ByResourceID(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{
		Nodes: []models.Node{
			{ID: "n1", Name: "node-1"},
			{ID: "n2", Name: "node-2"},
		},
	}
	scope := PatrolScope{
		ResourceIDs:   []string{"n1"},
		ResourceTypes: []string{"node"},
	}

	filtered := ps.filterStateByScope(state, scope)

	if len(filtered.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(filtered.Nodes))
	}
	if filtered.Nodes[0].ID != "n1" {
		t.Errorf("expected node n1, got %s", filtered.Nodes[0].ID)
	}
}

func TestFilterStateByScope_ByResourceName(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{
		Nodes: []models.Node{
			{ID: "n1", Name: "node-1"},
			{ID: "n2", Name: "node-2"},
		},
	}
	scope := PatrolScope{
		ResourceIDs:   []string{"node-1"},
		ResourceTypes: []string{"node"},
	}

	filtered := ps.filterStateByScope(state, scope)

	if len(filtered.Nodes) != 1 {
		t.Fatalf("expected 1 node matched by name, got %d", len(filtered.Nodes))
	}
	if filtered.Nodes[0].Name != "node-1" {
		t.Errorf("expected node-1, got %s", filtered.Nodes[0].Name)
	}
}

func TestFilterStateByScope_ByType_VM(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{
		Nodes: []models.Node{{ID: "n1", Name: "node-1"}},
		VMs:   []models.VM{{ID: "vm1", Name: "vm-1"}},
	}
	scope := PatrolScope{
		ResourceTypes: []string{"vm"},
	}

	filtered := ps.filterStateByScope(state, scope)

	if len(filtered.Nodes) != 0 {
		t.Errorf("expected 0 nodes when scoped to VM type, got %d", len(filtered.Nodes))
	}
	if len(filtered.VMs) != 1 {
		t.Errorf("expected 1 VM, got %d", len(filtered.VMs))
	}
}

func TestFilterStateByScope_TypeAliases(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{
		VMs:        []models.VM{{ID: "vm1", Name: "vm-1"}},
		Containers: []models.Container{{ID: "ct1", Name: "ct-1"}},
	}

	// "qemu" should match VMs
	scope := PatrolScope{ResourceTypes: []string{"qemu"}}
	filtered := ps.filterStateByScope(state, scope)
	if len(filtered.VMs) != 1 {
		t.Errorf("expected 'qemu' alias to match VMs, got %d", len(filtered.VMs))
	}

	// "container" should match LXC containers
	scope = PatrolScope{ResourceTypes: []string{"container"}}
	filtered = ps.filterStateByScope(state, scope)
	if len(filtered.Containers) != 1 {
		t.Errorf("expected 'container' alias to match LXC, got %d", len(filtered.Containers))
	}
}

func TestFilterStateByScope_DockerHost(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{
		DockerHosts: []models.DockerHost{
			{
				ID:       "dh1",
				Hostname: "docker-host-1",
				Containers: []models.DockerContainer{
					{ID: "c1", Name: "web"},
					{ID: "c2", Name: "db"},
				},
			},
		},
	}

	// Match by host ID
	scope := PatrolScope{
		ResourceIDs:   []string{"dh1"},
		ResourceTypes: []string{"docker"},
	}
	filtered := ps.filterStateByScope(state, scope)
	if len(filtered.DockerHosts) != 1 {
		t.Fatalf("expected 1 docker host, got %d", len(filtered.DockerHosts))
	}
	// When the host itself matches, all containers should be included
	if len(filtered.DockerHosts[0].Containers) != 2 {
		t.Errorf("expected all 2 containers when host matches, got %d", len(filtered.DockerHosts[0].Containers))
	}
}

func TestFilterStateByScope_DockerContainerOnly(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{
		DockerHosts: []models.DockerHost{
			{
				ID:       "dh1",
				Hostname: "docker-host-1",
				Containers: []models.DockerContainer{
					{ID: "c1", Name: "web"},
					{ID: "c2", Name: "db"},
				},
			},
		},
	}

	// Match by container ID only
	scope := PatrolScope{
		ResourceIDs:   []string{"c1"},
		ResourceTypes: []string{"docker_container"},
	}
	filtered := ps.filterStateByScope(state, scope)
	if len(filtered.DockerHosts) != 1 {
		t.Fatalf("expected 1 docker host, got %d", len(filtered.DockerHosts))
	}
	if len(filtered.DockerHosts[0].Containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(filtered.DockerHosts[0].Containers))
	}
	if filtered.DockerHosts[0].Containers[0].ID != "c1" {
		t.Errorf("expected container c1, got %s", filtered.DockerHosts[0].Containers[0].ID)
	}
}

func TestFilterStateByScope_Storage(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{
		Storage: []models.Storage{
			{ID: "s1", Name: "local"},
			{ID: "s2", Name: "ceph"},
		},
	}

	scope := PatrolScope{
		ResourceIDs:   []string{"s1"},
		ResourceTypes: []string{"storage"},
	}
	filtered := ps.filterStateByScope(state, scope)
	if len(filtered.Storage) != 1 {
		t.Fatalf("expected 1 storage, got %d", len(filtered.Storage))
	}
	if filtered.Storage[0].ID != "s1" {
		t.Errorf("expected storage s1, got %s", filtered.Storage[0].ID)
	}
}

func TestFilterStateByScope_Hosts(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{
		Hosts: []models.Host{
			{ID: "h1", Hostname: "host-1"},
			{ID: "h2", Hostname: "host-2"},
		},
	}

	scope := PatrolScope{
		ResourceIDs:   []string{"h1"},
		ResourceTypes: []string{"host"},
	}
	filtered := ps.filterStateByScope(state, scope)
	if len(filtered.Hosts) != 1 {
		t.Fatalf("expected 1 host, got %d", len(filtered.Hosts))
	}
	if filtered.Hosts[0].ID != "h1" {
		t.Errorf("expected host h1, got %s", filtered.Hosts[0].ID)
	}
}

func TestFilterStateByScope_PBS(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{
		PBSInstances: []models.PBSInstance{
			{
				ID:   "pbs1",
				Name: "pbs-main",
				Datastores: []models.PBSDatastore{
					{Name: "ds1"},
				},
			},
			{
				ID:   "pbs2",
				Name: "pbs-secondary",
			},
		},
	}

	scope := PatrolScope{
		ResourceIDs:   []string{"pbs1"},
		ResourceTypes: []string{"pbs"},
	}
	filtered := ps.filterStateByScope(state, scope)
	if len(filtered.PBSInstances) != 1 {
		t.Fatalf("expected 1 PBS instance, got %d", len(filtered.PBSInstances))
	}
	if filtered.PBSInstances[0].ID != "pbs1" {
		t.Errorf("expected PBS pbs1, got %s", filtered.PBSInstances[0].ID)
	}
}

func TestFilterStateByScope_Kubernetes(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{
		KubernetesClusters: []models.KubernetesCluster{
			{ID: "k1", Name: "cluster-1"},
			{ID: "k2", Name: "cluster-2"},
		},
	}

	scope := PatrolScope{
		ResourceIDs:   []string{"k1"},
		ResourceTypes: []string{"kubernetes"},
	}
	filtered := ps.filterStateByScope(state, scope)
	if len(filtered.KubernetesClusters) != 1 {
		t.Fatalf("expected 1 kubernetes cluster, got %d", len(filtered.KubernetesClusters))
	}
	if filtered.KubernetesClusters[0].ID != "k1" {
		t.Errorf("expected cluster k1, got %s", filtered.KubernetesClusters[0].ID)
	}
}

func TestFilterStateByScope_PreservesMetadata(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	now := time.Now()
	state := models.StateSnapshot{
		LastUpdate: now,
		ConnectionHealth: map[string]bool{
			"node-1": true,
		},
	}
	scope := PatrolScope{ResourceTypes: []string{"node"}}

	filtered := ps.filterStateByScope(state, scope)

	if filtered.LastUpdate != now {
		t.Error("expected LastUpdate to be preserved")
	}
	if len(filtered.ConnectionHealth) != 1 {
		t.Error("expected ConnectionHealth to be preserved")
	}
}

func TestFilterStateByScope_WhitespaceInIDs(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{
		Nodes: []models.Node{{ID: "n1", Name: "node-1"}},
	}

	// IDs with whitespace should be trimmed
	scope := PatrolScope{
		ResourceIDs:   []string{"  n1  "},
		ResourceTypes: []string{"node"},
	}
	filtered := ps.filterStateByScope(state, scope)
	if len(filtered.Nodes) != 1 {
		t.Errorf("expected whitespace-trimmed ID to match, got %d nodes", len(filtered.Nodes))
	}
}

func TestFilterStateByScope_EmptyIDsIgnored(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{
		Nodes: []models.Node{{ID: "n1", Name: "node-1"}},
	}

	scope := PatrolScope{
		ResourceIDs:   []string{"", "  ", "n1"},
		ResourceTypes: []string{"node"},
	}
	filtered := ps.filterStateByScope(state, scope)
	if len(filtered.Nodes) != 1 {
		t.Errorf("expected empty IDs to be ignored, got %d nodes", len(filtered.Nodes))
	}
}

func TestFilterStateByScope_CaseInsensitiveTypes(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{
		Nodes: []models.Node{{ID: "n1", Name: "node-1"}},
	}

	scope := PatrolScope{
		ResourceTypes: []string{"NODE"},
	}
	filtered := ps.filterStateByScope(state, scope)
	if len(filtered.Nodes) != 1 {
		t.Errorf("expected case-insensitive type matching, got %d nodes", len(filtered.Nodes))
	}
}

// --- tryStartRun / endRun tests ---

func TestTryStartRun_Basic(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	if !ps.tryStartRun("full") {
		t.Error("expected first tryStartRun to succeed")
	}
	if ps.tryStartRun("full") {
		t.Error("expected second tryStartRun to fail (run in progress)")
	}

	ps.endRun()

	if !ps.tryStartRun("full") {
		t.Error("expected tryStartRun to succeed after endRun")
	}
	ps.endRun()
}

func TestTryStartRun_Concurrent(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	var wg sync.WaitGroup
	var successes int32
	var mu sync.Mutex

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if ps.tryStartRun("scoped") {
				mu.Lock()
				successes++
				mu.Unlock()
				// Simulate some work
				time.Sleep(1 * time.Millisecond)
				ps.endRun()
			}
		}()
	}
	wg.Wait()

	if successes == 0 {
		t.Error("expected at least one goroutine to acquire the run")
	}
	// After all goroutines complete, run should not be in progress
	ps.mu.RLock()
	inProgress := ps.runInProgress
	ps.mu.RUnlock()
	if inProgress {
		t.Error("expected runInProgress to be false after all goroutines complete")
	}
}

// --- Subscribe / Unsubscribe ---

func TestSubscribeUnsubscribe(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	ch1 := ps.SubscribeToStream()
	ch2 := ps.SubscribeToStream()

	ps.streamMu.RLock()
	count := len(ps.streamSubscribers)
	ps.streamMu.RUnlock()

	if count != 2 {
		t.Errorf("expected 2 subscribers, got %d", count)
	}

	ps.UnsubscribeFromStream(ch1)

	ps.streamMu.RLock()
	count = len(ps.streamSubscribers)
	ps.streamMu.RUnlock()

	if count != 1 {
		t.Errorf("expected 1 subscriber after unsubscribe, got %d", count)
	}

	ps.UnsubscribeFromStream(ch2)

	ps.streamMu.RLock()
	count = len(ps.streamSubscribers)
	ps.streamMu.RUnlock()

	if count != 0 {
		t.Errorf("expected 0 subscribers after all unsubscribe, got %d", count)
	}
}

func TestSubscribeToStream_ReceivesCurrentState(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	// Set active streaming state
	ps.streamMu.Lock()
	ps.streamPhase = "analyzing"
	ps.currentOutput.WriteString("some output")
	ps.streamMu.Unlock()

	ch := ps.SubscribeToStream()
	defer ps.UnsubscribeFromStream(ch)

	// New subscriber should receive a snapshot of the current state.
	select {
	case event := <-ch:
		if event.Type != "snapshot" {
			t.Errorf("expected snapshot event first, got %s", event.Type)
		}
		if event.Phase != "analyzing" {
			t.Errorf("expected phase 'analyzing', got %q", event.Phase)
		}
		if event.Content != "some output" {
			t.Errorf("expected 'some output', got %q", event.Content)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("expected to receive snapshot event on subscribe")
	}
}

// --- broadcast ---

func TestBroadcast_MultipleSubscribers(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	ch1 := ps.SubscribeToStream()
	ch2 := ps.SubscribeToStream()
	ch3 := ps.SubscribeToStream()

	event := PatrolStreamEvent{Type: "test", Content: "hello"}
	ps.broadcast(event)

	for i, ch := range []chan PatrolStreamEvent{ch1, ch2, ch3} {
		select {
		case received := <-ch:
			if received.Content != "hello" {
				t.Errorf("subscriber %d: expected 'hello', got %q", i, received.Content)
			}
		case <-time.After(100 * time.Millisecond):
			t.Errorf("subscriber %d: timed out waiting for event", i)
		}
	}

	ps.UnsubscribeFromStream(ch1)
	ps.UnsubscribeFromStream(ch2)
	ps.UnsubscribeFromStream(ch3)
}

func TestBroadcast_StaleSubscriberRemoved(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	// Create a subscriber with a full buffer
	ch := ps.SubscribeToStream()

	// Fill the buffer (100 capacity)
	for i := 0; i < 100; i++ {
		ps.broadcast(PatrolStreamEvent{Type: "fill", Content: "x"})
	}

	// Keep broadcasting without reading; subscriber should eventually be dropped
	// after repeated full-buffer backpressure.
	for i := 0; i < 30; i++ {
		ps.broadcast(PatrolStreamEvent{Type: "overflow"})
	}

	// Give goroutine time to close the channel
	time.Sleep(50 * time.Millisecond)

	ps.streamMu.RLock()
	_, exists := ps.streamSubscribers[ch]
	ps.streamMu.RUnlock()

	if exists {
		t.Error("expected stale subscriber to be removed")
	}
}

func TestSubscribeToStreamFrom_ReplaysBufferedEvents(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	ps.resetStreamForRun("run-1")

	// No subscribers: broadcast should still buffer events for replay.
	ps.broadcast(PatrolStreamEvent{Type: "phase", Phase: "analyzing"})
	ps.broadcast(PatrolStreamEvent{Type: "content", Content: "hello"})
	ps.broadcast(PatrolStreamEvent{Type: "tool_start", ToolName: "uptime"})

	ps.streamMu.RLock()
	lastSeq := ps.streamSeq
	ps.streamMu.RUnlock()
	if lastSeq < 3 {
		t.Fatalf("expected seq to be >= 3, got %d", lastSeq)
	}

	// Subscribe from seq 1; should replay seq 2 and 3.
	ch := ps.SubscribeToStreamFrom(1)
	defer ps.UnsubscribeFromStream(ch)

	// First replayed event: content
	select {
	case ev := <-ch:
		if ev.Type != "content" {
			t.Fatalf("expected content replay, got %s", ev.Type)
		}
		if ev.Content != "hello" {
			t.Fatalf("expected hello content, got %q", ev.Content)
		}
		if ev.Seq <= 1 {
			t.Fatalf("expected seq > 1, got %d", ev.Seq)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for replayed content event")
	}

	// Second replayed event: tool_start
	select {
	case ev := <-ch:
		if ev.Type != "tool_start" {
			t.Fatalf("expected tool_start replay, got %s", ev.Type)
		}
		if ev.ToolName != "uptime" {
			t.Fatalf("expected tool uptime, got %q", ev.ToolName)
		}
		if ev.Seq <= 1 {
			t.Fatalf("expected seq > 1, got %d", ev.Seq)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for replayed tool_start event")
	}
}

func TestSubscribeToStreamFrom_BufferRotatedSnapshot(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	ps.resetStreamForRun("run-1")
	ps.setStreamPhase("analyzing")

	// Fill beyond replay buffer to force rotation.
	for i := 0; i < 260; i++ {
		ps.broadcast(PatrolStreamEvent{Type: "content", Content: "x"})
	}

	ps.streamMu.RLock()
	start, end := ps.streamBufferWindowLocked()
	ps.streamMu.RUnlock()
	if start == 0 || end == 0 || end <= start {
		t.Fatalf("expected non-empty buffer window, got start=%d end=%d", start, end)
	}

	// Ask to resume from a seq that is behind the buffered window.
	ch := ps.SubscribeToStreamFrom(start - 1)
	defer ps.UnsubscribeFromStream(ch)

	select {
	case ev := <-ch:
		if ev.Type != "snapshot" {
			t.Fatalf("expected snapshot, got %s", ev.Type)
		}
		if ev.ResyncReason != "buffer_rotated" {
			t.Fatalf("expected resync_reason buffer_rotated, got %q", ev.ResyncReason)
		}
		if ev.BufferStart == 0 || ev.BufferEnd == 0 || ev.BufferEnd < ev.BufferStart {
			t.Fatalf("expected buffer window in snapshot, got start=%d end=%d", ev.BufferStart, ev.BufferEnd)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for buffer_rotated snapshot")
	}
}

func TestSubscribeToStreamFrom_BufferRotatedSnapshotNotDuplicated(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	ps.resetStreamForRun("run-1")
	ps.setStreamPhase("analyzing")

	for i := 0; i < 260; i++ {
		ps.broadcast(PatrolStreamEvent{Type: "content", Content: "x"})
	}

	ps.streamMu.RLock()
	start, _ := ps.streamBufferWindowLocked()
	ps.streamMu.RUnlock()

	ch := ps.SubscribeToStreamFrom(start - 1)
	defer ps.UnsubscribeFromStream(ch)

	snapshotCount := 0
	read := 0
	for read < 25 {
		select {
		case ev := <-ch:
			read++
			if ev.Type == "snapshot" {
				snapshotCount++
			}
		case <-time.After(100 * time.Millisecond):
			read = 25
		}
	}

	if snapshotCount != 1 {
		t.Fatalf("expected exactly 1 snapshot for buffer_rotated resume, got %d", snapshotCount)
	}
}

func TestSubscribeToStreamFrom_ChurnDoesNotLeakSubscribers(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	const workers = 12
	const loopsPerWorker = 60

	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < loopsPerWorker; i++ {
				ch := ps.SubscribeToStreamFrom(0)
				ps.broadcast(PatrolStreamEvent{Type: "content", Content: "x"})
				ps.UnsubscribeFromStream(ch)
			}
		}()
	}
	wg.Wait()

	// Allow concurrent close/remove paths to settle.
	time.Sleep(25 * time.Millisecond)

	ps.streamMu.RLock()
	subs := len(ps.streamSubscribers)
	ps.streamMu.RUnlock()
	if subs != 0 {
		t.Fatalf("expected no leaked subscribers after churn, got %d", subs)
	}

	// Sanity: broadcasting after heavy churn should still work.
	ps.broadcast(PatrolStreamEvent{Type: "content", Content: "ok"})
}

func TestSubscribeToStreamFrom_RecordsReplayMetrics(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	ps.resetStreamForRun("run-metrics-replay")
	ps.setStreamPhase("analyzing")

	ps.broadcast(PatrolStreamEvent{Type: "phase", Phase: "analyzing"})
	ps.broadcast(PatrolStreamEvent{Type: "content", Content: "hello"})
	ps.broadcast(PatrolStreamEvent{Type: "tool_start", ToolName: "uptime"})

	m := GetPatrolMetrics()
	beforeOutcome := testutil.ToFloat64(m.streamResumeOutcome.WithLabelValues("replay"))
	beforeEvents := testutil.ToFloat64(m.streamReplayEvents)

	ch := ps.SubscribeToStreamFrom(1)
	defer ps.UnsubscribeFromStream(ch)

	// Drain replayed events to avoid backpressure affecting subsequent tests.
	<-ch
	<-ch

	afterOutcome := testutil.ToFloat64(m.streamResumeOutcome.WithLabelValues("replay"))
	afterEvents := testutil.ToFloat64(m.streamReplayEvents)
	if afterOutcome <= beforeOutcome {
		t.Fatalf("expected replay outcome counter to increase (before=%f after=%f)", beforeOutcome, afterOutcome)
	}
	if afterEvents < beforeEvents+2 {
		t.Fatalf("expected replay event counter to increase by at least 2 (before=%f after=%f)", beforeEvents, afterEvents)
	}
}

func TestSubscribeToStreamFrom_RecordsSnapshotAndMissMetrics(t *testing.T) {
	// Snapshot path (buffer rotated)
	psSnap := NewPatrolService(nil, nil)
	psSnap.resetStreamForRun("run-metrics-snapshot")
	psSnap.setStreamPhase("analyzing")
	for i := 0; i < 260; i++ {
		psSnap.broadcast(PatrolStreamEvent{Type: "content", Content: "x"})
	}
	psSnap.streamMu.RLock()
	start, _ := psSnap.streamBufferWindowLocked()
	psSnap.streamMu.RUnlock()

	m := GetPatrolMetrics()
	beforeSnapshotOutcome := testutil.ToFloat64(m.streamResumeOutcome.WithLabelValues("snapshot"))
	beforeReason := testutil.ToFloat64(m.streamResyncReason.WithLabelValues("buffer_rotated"))

	chSnap := psSnap.SubscribeToStreamFrom(start - 1)
	defer psSnap.UnsubscribeFromStream(chSnap)
	<-chSnap // consume snapshot

	afterSnapshotOutcome := testutil.ToFloat64(m.streamResumeOutcome.WithLabelValues("snapshot"))
	afterReason := testutil.ToFloat64(m.streamResyncReason.WithLabelValues("buffer_rotated"))
	if afterSnapshotOutcome <= beforeSnapshotOutcome {
		t.Fatalf("expected snapshot outcome counter to increase (before=%f after=%f)", beforeSnapshotOutcome, afterSnapshotOutcome)
	}
	if afterReason <= beforeReason {
		t.Fatalf("expected buffer_rotated reason counter to increase (before=%f after=%f)", beforeReason, afterReason)
	}

	// Miss path (resume requested while stream idle and no buffered events)
	psMiss := NewPatrolService(nil, nil)
	psMiss.resetStreamForRun("run-metrics-miss")

	beforeMiss := testutil.ToFloat64(m.streamResumeOutcome.WithLabelValues("miss"))
	chMiss := psMiss.SubscribeToStreamFrom(42)
	defer psMiss.UnsubscribeFromStream(chMiss)
	afterMiss := testutil.ToFloat64(m.streamResumeOutcome.WithLabelValues("miss"))
	if afterMiss <= beforeMiss {
		t.Fatalf("expected miss outcome counter to increase (before=%f after=%f)", beforeMiss, afterMiss)
	}
}

func TestBroadcast_RecordsBackpressureDropMetric(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	ch := ps.SubscribeToStream()
	defer ps.UnsubscribeFromStream(ch)

	m := GetPatrolMetrics()
	beforeDrop := testutil.ToFloat64(m.streamSubscriberDrop.WithLabelValues("backpressure"))

	for i := 0; i < 100; i++ {
		ps.broadcast(PatrolStreamEvent{Type: "fill", Content: "x"})
	}
	for i := 0; i < 30; i++ {
		ps.broadcast(PatrolStreamEvent{Type: "overflow"})
	}

	time.Sleep(25 * time.Millisecond)
	afterDrop := testutil.ToFloat64(m.streamSubscriberDrop.WithLabelValues("backpressure"))
	if afterDrop <= beforeDrop {
		t.Fatalf("expected backpressure drop metric to increase (before=%f after=%f)", beforeDrop, afterDrop)
	}
}

func TestStreamOutput_TruncatesTailAndSnapshotMarksIt(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	ps.resetStreamForRun("run-1")
	ps.setStreamPhase("analyzing")

	// Write more than the tail buffer can hold.
	payload := strings.Repeat("a", patrolStreamMaxOutputBytes+123)
	ps.appendStreamContent(payload)

	out, phase := ps.GetCurrentStreamOutput()
	if phase != "analyzing" {
		t.Fatalf("expected phase analyzing, got %q", phase)
	}
	if len(out) != patrolStreamMaxOutputBytes {
		t.Fatalf("expected output len %d, got %d", patrolStreamMaxOutputBytes, len(out))
	}
	expectedTail := payload[len(payload)-patrolStreamMaxOutputBytes:]
	if out != expectedTail {
		t.Fatalf("expected tail output to match last %d bytes", patrolStreamMaxOutputBytes)
	}

	ch := ps.SubscribeToStreamFrom(0)
	defer ps.UnsubscribeFromStream(ch)
	select {
	case ev := <-ch:
		if ev.Type != "snapshot" {
			t.Fatalf("expected snapshot, got %s", ev.Type)
		}
		if ev.ContentTruncated == nil || !*ev.ContentTruncated {
			t.Fatalf("expected snapshot content_truncated to be true")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for snapshot")
	}
}

// --- generateFindingID ---

func TestGenerateFindingID_Deterministic(t *testing.T) {
	id1 := generateFindingID("res-1", "performance", "high-cpu")
	id2 := generateFindingID("res-1", "performance", "high-cpu")

	if id1 != id2 {
		t.Errorf("expected deterministic ID, got %s and %s", id1, id2)
	}
}

func TestGenerateFindingID_Different(t *testing.T) {
	id1 := generateFindingID("res-1", "performance", "high-cpu")
	id2 := generateFindingID("res-2", "performance", "high-cpu")
	id3 := generateFindingID("res-1", "reliability", "high-cpu")
	id4 := generateFindingID("res-1", "performance", "high-mem")

	if id1 == id2 {
		t.Error("different resource should produce different ID")
	}
	if id1 == id3 {
		t.Error("different category should produce different ID")
	}
	if id1 == id4 {
		t.Error("different issue should produce different ID")
	}
}

func TestGenerateFindingID_Length(t *testing.T) {
	id := generateFindingID("res-1", "performance", "high-cpu")
	// sha256[:8] = 8 bytes = 16 hex chars
	if len(id) != 16 {
		t.Errorf("expected 16-char hex ID, got %d chars: %s", len(id), id)
	}
}

// --- joinParts (already tested in patrol_test.go, but verify here for completeness) ---

func TestJoinParts_Zero(t *testing.T) {
	if joinParts(nil) != "" {
		t.Error("expected empty string for nil")
	}
}

func TestJoinParts_Three(t *testing.T) {
	result := joinParts([]string{"a", "b", "c"})
	if result != "a, b, and c" {
		t.Errorf("expected 'a, b, and c', got %q", result)
	}
}

// --- GetStatus ---

func TestGetStatus_Running(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	// Simulate a run in progress
	ps.mu.Lock()
	ps.runInProgress = true
	ps.mu.Unlock()

	status := ps.GetStatus()
	if !status.Running {
		t.Error("expected Running to be true when runInProgress is true")
	}

	ps.mu.Lock()
	ps.runInProgress = false
	ps.mu.Unlock()

	status = ps.GetStatus()
	if status.Running {
		t.Error("expected Running to be false when runInProgress is false")
	}
}

func TestGetStatus_NextPatrolAt(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	// Set nextScheduledAt so nextPatrolAt is returned
	nextTime := time.Now().Add(30 * time.Minute)
	ps.mu.Lock()
	ps.nextScheduledAt = nextTime
	ps.mu.Unlock()

	status := ps.GetStatus()
	if status.NextPatrolAt == nil {
		t.Fatal("expected NextPatrolAt to be calculated")
	}

	if !status.NextPatrolAt.Equal(nextTime) {
		t.Errorf("expected NextPatrolAt %v, got %v", nextTime, *status.NextPatrolAt)
	}
}

func TestGetStatus_NextPatrolAt_IndependentOfLastPatrol(t *testing.T) {
	// nextScheduledAt should drive NextPatrolAt, not lastPatrol + interval.
	// This is the scenario that was previously broken: user changes interval
	// mid-cycle, lastPatrol is old, so lastPatrol+newInterval could be in the past.
	ps := NewPatrolService(nil, nil)

	// Simulate: patrol ran 45 min ago
	ps.mu.Lock()
	ps.lastPatrol = time.Now().Add(-45 * time.Minute)
	// But the ticker was just reset with a 15-min interval, so next fire is ~15 min from now
	expectedNext := time.Now().Add(15 * time.Minute)
	ps.nextScheduledAt = expectedNext
	ps.mu.Unlock()

	status := ps.GetStatus()
	if status.NextPatrolAt == nil {
		t.Fatal("expected NextPatrolAt to be set")
	}
	// NextPatrolAt must be the tracked nextScheduledAt (in the future),
	// NOT lastPatrol + interval (which would be 30 min in the past).
	if !status.NextPatrolAt.Equal(expectedNext) {
		t.Errorf("expected NextPatrolAt = %v, got %v", expectedNext, *status.NextPatrolAt)
	}
	if status.NextPatrolAt.Before(time.Now()) {
		t.Errorf("NextPatrolAt should be in the future, got %v", *status.NextPatrolAt)
	}
}

func TestGetStatus_NextPatrolAt_NilWhenNotScheduled(t *testing.T) {
	// Before the patrol loop starts, nextScheduledAt is zero — no NextPatrolAt should be returned.
	ps := NewPatrolService(nil, nil)

	status := ps.GetStatus()
	if status.NextPatrolAt != nil {
		t.Errorf("expected NextPatrolAt to be nil before patrol loop starts, got %v", *status.NextPatrolAt)
	}
}

func TestGetStatus_Disabled(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	ps.SetConfig(PatrolConfig{Enabled: false})

	// Even if nextScheduledAt is set, disabled patrol should not report NextPatrolAt
	ps.mu.Lock()
	ps.nextScheduledAt = time.Now().Add(10 * time.Minute)
	ps.mu.Unlock()

	status := ps.GetStatus()
	if status.Enabled {
		t.Error("expected Enabled to be false")
	}
	if status.NextPatrolAt != nil {
		t.Error("expected NextPatrolAt to be nil when disabled")
	}
}

func TestGetStatus_BlockedReason(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	ps.setBlockedReason("circuit breaker is open")

	status := ps.GetStatus()
	if status.BlockedReason != "circuit breaker is open" {
		t.Errorf("expected blocked reason, got %q", status.BlockedReason)
	}
	if status.BlockedAt == nil {
		t.Error("expected BlockedAt to be set")
	}

	ps.clearBlockedReason()
	status = ps.GetStatus()
	if status.BlockedReason != "" {
		t.Errorf("expected empty blocked reason after clear, got %q", status.BlockedReason)
	}
}

// --- appendStreamContent ---

func TestAppendStreamContent(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	ch := ps.SubscribeToStream()
	defer ps.UnsubscribeFromStream(ch)

	ps.appendStreamContent("hello ")
	ps.appendStreamContent("world")

	output, _ := ps.GetCurrentStreamOutput()
	if output != "hello world" {
		t.Errorf("expected 'hello world', got %q", output)
	}

	// Should have broadcast 2 events
	for i := 0; i < 2; i++ {
		select {
		case event := <-ch:
			if event.Type != "content" {
				t.Errorf("expected content event, got %s", event.Type)
			}
		case <-time.After(100 * time.Millisecond):
			t.Errorf("expected broadcast event %d", i)
		}
	}
}

// --- setStreamPhase ---

func TestSetStreamPhase_ResetClearsOutput(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	ps.setStreamPhase("analyzing")
	ps.appendStreamContent("some data")

	output, phase := ps.GetCurrentStreamOutput()
	if phase != "analyzing" {
		t.Errorf("expected phase 'analyzing', got %q", phase)
	}
	if output != "some data" {
		t.Errorf("expected 'some data', got %q", output)
	}

	ps.setStreamPhase("idle")
	output, phase = ps.GetCurrentStreamOutput()
	if phase != "idle" {
		t.Errorf("expected phase 'idle', got %q", phase)
	}
	// Output is no longer cleared on idle; it is cleared when a new run starts.
}

// --- isResourceOnline (heuristic alert resolution) ---

func TestIsResourceOnline_Node(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{
		Nodes: []models.Node{
			{ID: "n1", Name: "node-1", Status: "online"},
			{ID: "n2", Name: "node-2", Status: "offline"},
		},
	}

	alert := AlertInfo{ResourceID: "n1", ResourceType: "node"}
	if !ps.isResourceOnline(alert, state) {
		t.Error("expected node n1 to be online")
	}

	alert = AlertInfo{ResourceID: "n2", ResourceType: "node"}
	if ps.isResourceOnline(alert, state) {
		t.Error("expected node n2 to be offline")
	}
}

func TestIsResourceOnline_VM(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{
		VMs: []models.VM{
			{ID: "vm1", Name: "vm-1", Status: "running"},
			{ID: "vm2", Name: "vm-2", Status: "stopped"},
		},
	}

	alert := AlertInfo{ResourceID: "vm1", ResourceType: "vm"}
	if !ps.isResourceOnline(alert, state) {
		t.Error("expected VM vm1 to be online")
	}

	alert = AlertInfo{ResourceID: "vm2", ResourceType: "vm"}
	if ps.isResourceOnline(alert, state) {
		t.Error("expected VM vm2 to be offline")
	}
}

func TestIsResourceOnline_Container(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{
		Containers: []models.Container{
			{ID: "ct1", Name: "ct-1", Status: "running"},
		},
	}

	alert := AlertInfo{ResourceID: "ct1", ResourceType: "container"}
	if !ps.isResourceOnline(alert, state) {
		t.Error("expected container ct1 to be online")
	}
}

func TestIsResourceOnline_Docker(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{
		DockerHosts: []models.DockerHost{
			{
				Containers: []models.DockerContainer{
					{ID: "dc1", Name: "web", State: "running"},
					{ID: "dc2", Name: "db", State: "exited"},
				},
			},
		},
	}

	alert := AlertInfo{ResourceID: "dc1", ResourceType: "docker"}
	if !ps.isResourceOnline(alert, state) {
		t.Error("expected docker container dc1 to be online")
	}

	alert = AlertInfo{ResourceID: "dc2", ResourceType: "docker"}
	if ps.isResourceOnline(alert, state) {
		t.Error("expected docker container dc2 to be offline")
	}
}

func TestIsResourceOnline_UnknownType(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{}

	alert := AlertInfo{ResourceID: "x", ResourceType: "unknown"}
	if ps.isResourceOnline(alert, state) {
		t.Error("expected unknown type to return false")
	}
}

// --- getCurrentMetricValue ---

func TestGetCurrentMetricValue_Node(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{
		Nodes: []models.Node{
			{ID: "n1", Name: "node-1", CPU: 0.75, Memory: models.Memory{Usage: 60.0}},
		},
	}

	alert := AlertInfo{ResourceID: "n1", ResourceType: "node", Type: "cpu"}
	val := ps.getCurrentMetricValue(alert, state)
	if val != 75.0 {
		t.Errorf("expected CPU 75.0 (0.75 * 100), got %f", val)
	}

	alert.Type = "memory"
	val = ps.getCurrentMetricValue(alert, state)
	if val != 60.0 {
		t.Errorf("expected memory 60.0, got %f", val)
	}
}

func TestGetCurrentMetricValue_NotFound(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{}

	alert := AlertInfo{ResourceID: "missing", ResourceType: "node", Type: "cpu"}
	val := ps.getCurrentMetricValue(alert, state)
	if val != -1 {
		t.Errorf("expected -1 for not found, got %f", val)
	}
}

func TestGetCurrentMetricValue_Docker(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{
		DockerHosts: []models.DockerHost{
			{
				Containers: []models.DockerContainer{
					{ID: "dc1", Name: "web", CPUPercent: 45.0, MemoryPercent: 30.0},
				},
			},
		},
	}

	alert := AlertInfo{ResourceID: "dc1", ResourceType: "docker", Type: "cpu"}
	val := ps.getCurrentMetricValue(alert, state)
	if val != 45.0 {
		t.Errorf("expected docker CPU 45.0, got %f", val)
	}

	alert.Type = "memory"
	val = ps.getCurrentMetricValue(alert, state)
	if val != 30.0 {
		t.Errorf("expected docker memory 30.0, got %f", val)
	}
}

func TestGetCurrentMetricValue_Storage(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{
		Storage: []models.Storage{
			{ID: "s1", Name: "local", Usage: 72.5},
		},
	}

	alert := AlertInfo{ResourceID: "s1", ResourceType: "Storage", Type: "usage"}
	val := ps.getCurrentMetricValue(alert, state)
	if val != 72.5 {
		t.Errorf("expected storage usage 72.5, got %f", val)
	}
}

// --- shouldResolveAlert (heuristic-only, no AI) ---

func TestShouldResolveAlert_StorageUsageDropped(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{
		Storage: []models.Storage{
			{ID: "s1", Name: "local", Usage: 70.0, Status: "active"},
		},
	}

	alert := AlertInfo{
		ID:           "a1",
		ResourceID:   "s1",
		ResourceType: "usage",
		Type:         "usage",
		Threshold:    85.0,
		Value:        88.0,
		StartTime:    time.Now().Add(-1 * time.Hour),
	}

	shouldResolve, reason := ps.shouldResolveAlert(nil, alert, state, nil)
	if !shouldResolve {
		t.Error("expected alert to be resolved (usage dropped below threshold)")
	}
	if reason == "" {
		t.Error("expected a reason string")
	}
}

func TestShouldResolveAlert_CPUDropped(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{
		Nodes: []models.Node{
			{ID: "n1", Name: "node-1", CPU: 0.5, Memory: models.Memory{Usage: 40.0}},
		},
	}

	alert := AlertInfo{
		ID:           "a1",
		ResourceID:   "n1",
		ResourceType: "node",
		Type:         "cpu",
		Threshold:    90.0,
		Value:        95.0,
		StartTime:    time.Now().Add(-30 * time.Minute),
	}

	shouldResolve, _ := ps.shouldResolveAlert(nil, alert, state, nil)
	if !shouldResolve {
		t.Error("expected alert to be resolved (CPU dropped)")
	}
}

func TestShouldResolveAlert_OfflineNowOnline(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{
		Nodes: []models.Node{
			{ID: "n1", Name: "node-1", Status: "online"},
		},
	}

	alert := AlertInfo{
		ID:           "a1",
		ResourceID:   "n1",
		ResourceType: "node",
		Type:         "offline",
		StartTime:    time.Now().Add(-1 * time.Hour),
	}

	shouldResolve, reason := ps.shouldResolveAlert(nil, alert, state, nil)
	if !shouldResolve {
		t.Error("expected offline alert to be resolved (resource now online)")
	}
	if reason != "resource is now online/running" {
		t.Errorf("unexpected reason: %s", reason)
	}
}

func TestShouldResolveAlert_NoMatch(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{} // resource not in state

	alert := AlertInfo{
		ID:           "a1",
		ResourceID:   "n1",
		ResourceType: "node",
		Type:         "cpu",
		Threshold:    90.0,
		Value:        95.0,
		StartTime:    time.Now().Add(-30 * time.Minute),
	}

	shouldResolve, _ := ps.shouldResolveAlert(nil, alert, state, nil)
	if shouldResolve {
		t.Error("expected alert NOT to be resolved when resource not found")
	}
}

func TestShouldResolveAlert_StorageStillHigh(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{
		Storage: []models.Storage{
			{ID: "s1", Name: "local", Usage: 90.0},
		},
	}

	alert := AlertInfo{
		ID:           "a1",
		ResourceID:   "s1",
		ResourceType: "usage",
		Type:         "usage",
		Threshold:    85.0,
		Value:        88.0,
		StartTime:    time.Now().Add(-1 * time.Hour),
	}

	shouldResolve, _ := ps.shouldResolveAlert(nil, alert, state, nil)
	if shouldResolve {
		t.Error("expected alert NOT to be resolved (storage usage still above threshold)")
	}
}

// Regression test: CPU at 95% (0.95 raw) must NOT auto-resolve a 90% threshold alert.
// Before the fix, getCurrentMetricValue returned 0.95 (raw fraction) which was always
// below the 90.0 percentage threshold, causing every CPU alert to incorrectly auto-resolve.
func TestShouldResolveAlert_CPUScaleRegression(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{
		Nodes: []models.Node{
			{ID: "n1", Name: "node-1", CPU: 0.95, Memory: models.Memory{Usage: 40.0}},
		},
	}

	alert := AlertInfo{
		ID:           "a1",
		ResourceID:   "n1",
		ResourceType: "node",
		Type:         "cpu",
		Threshold:    90.0,
		Value:        95.0,
		StartTime:    time.Now().Add(-30 * time.Minute),
	}

	shouldResolve, _ := ps.shouldResolveAlert(nil, alert, state, nil)
	if shouldResolve {
		t.Error("expected CPU alert NOT to be resolved (95% is still above 90% threshold)")
	}
}

func TestGetCurrentMetricValue_CPUScalePercent(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	// Node CPU
	state := models.StateSnapshot{
		Nodes: []models.Node{{ID: "n1", CPU: 0.42}},
	}
	val := ps.getCurrentMetricValue(AlertInfo{ResourceID: "n1", ResourceType: "node", Type: "cpu"}, state)
	if val != 42.0 {
		t.Errorf("node CPU: expected 42.0, got %f", val)
	}

	// VM CPU
	state = models.StateSnapshot{
		VMs: []models.VM{{ID: "vm1", CPU: 0.88}},
	}
	val = ps.getCurrentMetricValue(AlertInfo{ResourceID: "vm1", ResourceType: "guest", Type: "cpu"}, state)
	if val != 88.0 {
		t.Errorf("VM CPU: expected 88.0, got %f", val)
	}

	// Container CPU
	state = models.StateSnapshot{
		Containers: []models.Container{{ID: "ct1", CPU: 0.15}},
	}
	val = ps.getCurrentMetricValue(AlertInfo{ResourceID: "ct1", ResourceType: "container", Type: "cpu"}, state)
	if val != 15.0 {
		t.Errorf("Container CPU: expected 15.0, got %f", val)
	}
}

// --- reviewAndResolveAlerts ---

func TestReviewAndResolveAlerts_NilResolver(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{}

	result := ps.reviewAndResolveAlerts(nil, state, true)
	if result != 0 {
		t.Errorf("expected 0 resolved with nil resolver, got %d", result)
	}
}

func TestReviewAndResolveAlerts_NoActiveAlerts(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	resolver := &mockAlertResolver{alerts: nil}
	ps.mu.Lock()
	ps.alertResolver = resolver
	ps.mu.Unlock()

	state := models.StateSnapshot{}
	result := ps.reviewAndResolveAlerts(nil, state, true)
	if result != 0 {
		t.Errorf("expected 0 resolved with no alerts, got %d", result)
	}
}

func TestReviewAndResolveAlerts_SkipsRecentAlerts(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	resolver := &mockAlertResolver{
		alerts: []AlertInfo{
			{
				ID:           "a1",
				ResourceID:   "n1",
				ResourceType: "node",
				Type:         "offline",
				StartTime:    time.Now().Add(-5 * time.Minute), // only 5 minutes old
			},
		},
	}
	ps.mu.Lock()
	ps.alertResolver = resolver
	ps.mu.Unlock()

	state := models.StateSnapshot{
		Nodes: []models.Node{{ID: "n1", Name: "node-1", Status: "online"}},
	}

	result := ps.reviewAndResolveAlerts(nil, state, true)
	if result != 0 {
		t.Errorf("expected 0 resolved (alert too recent), got %d", result)
	}
}

// --- mockAlertResolver ---

type mockAlertResolver struct {
	alerts   []AlertInfo
	resolved []string
}

func (m *mockAlertResolver) GetActiveAlerts() []AlertInfo {
	return m.alerts
}

func (m *mockAlertResolver) ResolveAlert(alertID string) bool {
	m.resolved = append(m.resolved, alertID)
	return true
}
