package monitoring

import (
	"testing"
	"testing/synctest"
	"time"

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
