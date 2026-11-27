package monitoring

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/stretchr/testify/require"
)

func TestPVEClusterCheckAdmissionIsAtomicAndPerInstance(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		m := &Monitor{lastClusterCheck: make(map[string]time.Time)}
		// Concurrent attempts for one connection share one check, while a
		// different connection's first check is immediately eligible.
		const callers = 8
		start, admitted := make(chan struct{}), make(chan bool, callers)
		for range callers {
			go func() {
				<-start
				admitted <- m.claimPVEClusterCheck("first")
			}()
		}
		close(start)
		claims := 0
		for range callers {
			if <-admitted {
				claims++
			}
		}
		require.Equal(t, 1, claims)
		time.Sleep(time.Minute)
		require.True(t, m.claimPVEClusterCheck("second"))
		time.Sleep(4 * time.Minute)
		require.False(t, m.claimPVEClusterCheck("first"), "exactly five minutes is still inside the check window")
		time.Sleep(time.Millisecond)
		require.True(t, m.claimPVEClusterCheck("first"))
		require.False(t, m.claimPVEClusterCheck("second"), "another connection keeps its own window")
	})
}

type pveDiscoveryStatusFunc func(context.Context) ([]proxmox.ClusterStatus, error)

func (f pveDiscoveryStatusFunc) GetClusterStatus(ctx context.Context) ([]proxmox.ClusterStatus, error) {
	return f(ctx)
}

func TestPVEClusterDiscoveryRetryStopsAtPollDeadline(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()
		started := time.Now()
		calls := 0
		_, err := readMonitorPVEClusterStatus(ctx, pveDiscoveryStatusFunc(func(readCtx context.Context) ([]proxmox.ClusterStatus, error) {
			calls++
			deadline, ok := readCtx.Deadline()
			require.True(t, ok)
			require.Equal(t, started.Add(200*time.Millisecond), deadline)
			return nil, errors.New("temporary cluster status failure")
		}))
		require.ErrorIs(t, err, context.DeadlineExceeded)
		require.Equal(t, 1, calls, "do not issue another read after the poll deadline")
		require.Equal(t, 200*time.Millisecond, time.Since(started), "do not wait out the full backoff")
	})
}

func TestPVEClusterDiscoveryKeepsRetryAndAttemptBudgets(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		started := time.Now()
		wantStatus := []proxmox.ClusterStatus{{Type: "cluster", Name: "lab"}}
		var readTimes []time.Duration
		status, err := readMonitorPVEClusterStatus(context.Background(), pveDiscoveryStatusFunc(func(ctx context.Context) ([]proxmox.ClusterStatus, error) {
			readTimes = append(readTimes, time.Since(started))
			deadline, ok := ctx.Deadline()
			require.True(t, ok)
			require.Equal(t, 5*time.Second, time.Until(deadline))
			switch len(readTimes) {
			case 1:
				// A child timeout alone does not end discovery.
				<-ctx.Done()
				return nil, ctx.Err()
			case 2:
				return nil, errors.New("temporary cluster status failure")
			default:
				return wantStatus, nil
			}
		}))
		require.NoError(t, err)
		require.Equal(t, wantStatus, status)
		require.Equal(t, []time.Duration{0, 6 * time.Second, 8 * time.Second}, readTimes)
	})
}

func TestPVEClusterDiscoveryReleasesCancelledProviderRequest(t *testing.T) {
	for _, tc := range []struct {
		name string
		path string
		auth config.PVEInstance
	}{
		{"password login", "/api2/json/access/ticket", config.PVEInstance{User: "fixture@pam", Password: "fixture"}},
		{"cluster status", "/api2/json/cluster/status", config.PVEInstance{TokenName: "fixture@pam!monitor", TokenValue: "fixture"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			entered, cancelled, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
			var first sync.Once
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != tc.path {
					http.NotFound(w, r)
					t.Errorf("unexpected discovery request: %s", r.URL.Path)
					return
				}
				// Consume the login body so the server can observe the
				// client's closed connection while the response is held.
				if _, err := io.Copy(io.Discard, r.Body); err != nil {
					t.Errorf("read discovery fixture request: %v", err)
					return
				}
				first.Do(func() {
					close(entered)
					select {
					case <-r.Context().Done():
						close(cancelled)
					case <-release:
					}
				})
				w.WriteHeader(http.StatusServiceUnavailable)
			}))
			defer srv.Close()
			defer close(release)

			cfg := tc.auth
			cfg.Name, cfg.Host = "lab", srv.URL
			m := &Monitor{
				config:           &config.Config{PVEInstances: []config.PVEInstance{cfg}},
				lastClusterCheck: make(map[string]time.Time),
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan struct{})
			go func() {
				defer close(done)
				m.detectClusterMembership(ctx, cfg.Name, &cfg, nil)
			}()

			select {
			case <-entered:
			case <-time.After(4 * time.Second):
				t.Fatal("discovery did not reach the fixture")
			}
			cancel()
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Fatal("cancelled discovery held the poll open")
			}
			select {
			case <-cancelled:
			case <-time.After(2 * time.Second):
				t.Fatal("provider request outlived the poll")
			}
			require.False(t, m.config.PVEInstances[0].IsCluster)
			require.Empty(t, m.config.PVEInstances[0].ClusterEndpoints)
		})
	}
}
