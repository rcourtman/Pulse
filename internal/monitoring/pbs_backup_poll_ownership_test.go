package monitoring

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
	"github.com/stretchr/testify/require"
)

func pbsOwnershipMonitor() (*Monitor, *pbs.Client) {
	client := &pbs.Client{}
	m := &Monitor{config: &config.Config{EnableBackupPolling: true, BackupPollingCycles: 1}}
	m.replacePBSClient("site", client)
	return m, client
}

func TestPBSBackupLifecycleAdmissionHoldsUntilReturn(t *testing.T) {
	m, client := pbsOwnershipMonitor()
	owner, _ := m.beginPBSBackupPoll("site", time.Now(), client)
	require.NotNil(t, owner)
	m.SetBackupPollingInterval(time.Minute)
	m.setRuntimeContext(context.Background(), nil)
	fresh := &pbs.Client{}
	m.replacePBSClient("site", fresh)
	require.ErrorIs(t, owner.ctx.Err(), context.Canceled)
	for i := 0; i < 8; i++ {
		other, _ := m.beginPBSBackupPoll("site", time.Now(), fresh)
		require.Nil(t, other, "cancellation/cadence reset cannot release a live reader's slot")
	}
	m.finishPBSBackupPoll("site", owner)
	require.Zero(t, m.lastPBSBackupPoll["site"], "withdrawn return must not delay normal resumption")
	next, _ := m.beginPBSBackupPoll("site", time.Now(), fresh)
	require.NotNil(t, next)
	m.SetBackupPollingInterval(30 * time.Second)
	m.finishPBSBackupPoll("site", next)
	require.Zero(t, m.lastPBSBackupPoll["site"], "completion must preserve an explicit cadence reset")
	next, _ = m.beginPBSBackupPoll("site", time.Now(), fresh)
	require.NotNil(t, next)
	m.finishPBSBackupPoll("site", next)
	require.False(t, m.lastPBSBackupPoll["site"].IsZero(), "normal completion must preserve cadence")
}

func TestPBSBackupLifecycleAdmissionIsAtomicAndConnectionScoped(t *testing.T) {
	m, client := pbsOwnershipMonitor()
	var wg sync.WaitGroup
	owners := make(chan *pbsBackupPoll, 32)
	for i := 0; i < cap(owners); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if owner, _ := m.beginPBSBackupPoll("site", time.Now(), client); owner != nil {
				owners <- owner
			}
		}()
	}
	wg.Wait()
	require.Len(t, owners, 1)
	owner := <-owners
	peer := &pbs.Client{}
	m.replacePBSClient("peer", peer)
	peerOwner, _ := m.beginPBSBackupPoll("peer", time.Now(), peer)
	require.NotNil(t, peerOwner)
	m.replacePBSClient("site", &pbs.Client{})
	require.Error(t, owner.ctx.Err())
	require.NoError(t, peerOwner.ctx.Err(), "connection replacement must not withdraw a sibling")
	m.finishPBSBackupPoll("site", owner)
	m.finishPBSBackupPoll("peer", peerOwner)
}

func TestPBSBackupLifecycleEndedOrUnregisteredCannotStart(t *testing.T) {
	m, client := pbsOwnershipMonitor()
	owner, _ := m.beginPBSBackupPoll("site", time.Now(), &pbs.Client{})
	require.Nil(t, owner)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	m.setRuntimeContext(ctx, nil)
	owner, _ = m.beginPBSBackupPoll("site", time.Now(), client)
	require.Nil(t, owner)
}

func TestPBSBackupLifecyclePublicationCannotCrossWithdrawal(t *testing.T) {
	m, client := pbsOwnershipMonitor()
	owner, _ := m.beginPBSBackupPoll("site", time.Now(), client)
	require.NotNil(t, owner)
	ctx := context.WithValue(owner.ctx, pbsBackupPollContextKey{}, owner)
	entered, release, returned := make(chan struct{}), make(chan struct{}), make(chan struct{})
	go func() {
		m.applyPBSBackupEffect(ctx, "site", func() { close(entered); <-release })
		close(returned)
	}()
	<-entered
	withdrawn := make(chan struct{})
	go func() { m.replacePBSClient("site", &pbs.Client{}); close(withdrawn) }()
	select {
	case <-withdrawn:
		t.Error("replacement crossed an already admitted publication")
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	<-returned
	<-withdrawn
	called := false
	m.applyPBSBackupEffect(ctx, "site", func() { called = true })
	require.False(t, called, "late successful reader cannot republish state/cache/recovery/alerts")
	m.finishPBSBackupPoll("site", owner)
}

func TestPBSBackupLifecycleReplacementExpiresOnlyItsOwnCache(t *testing.T) {
	m, client := pbsOwnershipMonitor()
	key := pbsBackupGroupKey{datastore: "main", backupType: "vm", backupID: "100"}
	at := time.Now()
	m.setPBSBackupCacheTime("site", key, at)
	m.setPBSBackupCacheTime("peer", key, at)
	m.replacePBSClient("site", client)
	require.Equal(t, at, m.pbsBackupCacheTimeFor("site", key), "same client keeps ordinary cache reuse")
	m.replacePBSClient("site", &pbs.Client{})
	require.Zero(t, m.pbsBackupCacheTimeFor("site", key))
	require.Equal(t, at, m.pbsBackupCacheTimeFor("peer", key))
}
