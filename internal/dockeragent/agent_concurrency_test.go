package dockeragent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	containertypes "github.com/docker/docker/api/types/container"
	systemtypes "github.com/docker/docker/api/types/system"
	"github.com/rcourtman/pulse-go-rewrite/internal/buffer"
	"github.com/rcourtman/pulse-go-rewrite/internal/hostmetrics"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
	"github.com/rs/zerolog"
)

func TestCollectOnceSerializesConcurrentRuns(t *testing.T) {
	swap(t, &hostmetricsCollect, func(context.Context, []string) (hostmetrics.Snapshot, error) {
		return hostmetrics.Snapshot{}, nil
	})

	releaseInfo := make(chan struct{})
	infoEntered := make(chan struct{}, 2)
	var concurrentInfoCalls int32
	var maxConcurrentInfoCalls int32

	agent := &Agent{
		cfg: Config{Interval: 30 * time.Second},
		docker: &fakeDockerClient{
			daemonHost: "unix:///var/run/docker.sock",
			infoFunc: func(context.Context) (systemtypes.Info, error) {
				current := atomic.AddInt32(&concurrentInfoCalls, 1)
				for {
					prevMax := atomic.LoadInt32(&maxConcurrentInfoCalls)
					if current <= prevMax {
						break
					}
					if atomic.CompareAndSwapInt32(&maxConcurrentInfoCalls, prevMax, current) {
						break
					}
				}

				infoEntered <- struct{}{}
				<-releaseInfo
				atomic.AddInt32(&concurrentInfoCalls, -1)

				return systemtypes.Info{ID: "daemon", ServerVersion: "24.0.0"}, nil
			},
		},
		logger:       zerolog.Nop(),
		reportBuffer: buffer.New[agentsdocker.Report](10),
	}

	errCh := make(chan error, 2)
	go func() { errCh <- agent.collectOnce(context.Background()) }()

	select {
	case <-infoEntered:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for first collect to start")
	}

	go func() { errCh <- agent.collectOnce(context.Background()) }()

	select {
	case <-infoEntered:
		t.Fatal("collectOnce overlapped while first collection was running")
	case <-time.After(50 * time.Millisecond):
	}

	close(releaseInfo)

	for i := 0; i < 2; i++ {
		if err := <-errCh; err != nil {
			t.Fatalf("collectOnce returned error: %v", err)
		}
	}

	if got := atomic.LoadInt32(&maxConcurrentInfoCalls); got != 1 {
		t.Fatalf("expected collectOnce to serialize calls, max concurrent info calls = %d", got)
	}
}

func TestCheckForUpdatesSkipsOverlappingRuns(t *testing.T) {
	swap(t, &Version, "v1.2.3")

	requestStarted := make(chan struct{}, 1)
	release := make(chan struct{})
	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		requestStarted <- struct{}{}
		<-release
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"version":"1.2.3"}`))
	}))
	defer server.Close()

	agent := &Agent{
		targets: []TargetConfig{{URL: server.URL}},
		httpClients: map[bool]*http.Client{
			false: server.Client(),
		},
		logger: zerolog.Nop(),
	}

	done := make(chan struct{})
	go func() {
		agent.checkForUpdates(context.Background())
		close(done)
	}()

	select {
	case <-requestStarted:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for update check request to start")
	}

	agent.checkForUpdates(context.Background())

	if got := atomic.LoadInt32(&requestCount); got != 1 {
		t.Fatalf("expected overlapping update check to be skipped, request count = %d", got)
	}

	close(release)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for first update check to finish")
	}
}

func TestCleanupOrphanedBackupsSkipsOverlappingRuns(t *testing.T) {
	listStarted := make(chan struct{}, 1)
	release := make(chan struct{})
	var listCalls int32

	agent := &Agent{
		docker: &fakeDockerClient{
			containerListFunc: func(context.Context, containertypes.ListOptions) ([]containertypes.Summary, error) {
				atomic.AddInt32(&listCalls, 1)
				listStarted <- struct{}{}
				<-release
				return nil, nil
			},
		},
		logger: zerolog.Nop(),
	}

	done := make(chan struct{})
	go func() {
		agent.cleanupOrphanedBackups(context.Background())
		close(done)
	}()

	select {
	case <-listStarted:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for cleanup task to start")
	}

	agent.cleanupOrphanedBackups(context.Background())

	if got := atomic.LoadInt32(&listCalls); got != 1 {
		t.Fatalf("expected overlapping cleanup task to be skipped, list calls = %d", got)
	}

	close(release)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for first cleanup task to finish")
	}
}
