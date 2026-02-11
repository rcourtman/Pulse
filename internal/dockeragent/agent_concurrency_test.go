package dockeragent

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

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
