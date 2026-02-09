package api

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func BenchmarkTenantRBACProvider_ManyOrgs(b *testing.B) {
	baseDir := b.TempDir()
	orgIDs := makeOrgIDs("bench-org", 100)
	mustCreateOrgDirs(b, baseDir, orgIDs)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		provider := NewTenantRBACProvider(baseDir)

		var wg sync.WaitGroup
		errCh := make(chan error, len(orgIDs))
		for _, orgID := range orgIDs {
			orgID := orgID
			wg.Add(1)
			go func() {
				defer wg.Done()
				if _, err := provider.GetManager(orgID); err != nil {
					errCh <- fmt.Errorf("GetManager(%s) failed: %w", orgID, err)
				}
			}()
		}
		wg.Wait()
		close(errCh)

		for err := range errCh {
			b.Fatal(err)
		}

		if err := provider.Close(); err != nil {
			b.Fatalf("Close() failed: %v", err)
		}
	}
}

func TestTenantRBACProvider_ConcurrentAccessStress(t *testing.T) {
	baseDir := t.TempDir()
	orgIDs := makeOrgIDs("stress-org", 20)
	mustCreateOrgDirs(t, baseDir, orgIDs)

	provider := NewTenantRBACProvider(baseDir)

	const (
		workers         = 50
		callsPerWorker  = 20
		randomSeedBase  = int64(11_000)
		testWaitTimeout = 10 * time.Second
	)

	var wg sync.WaitGroup
	errCh := make(chan error, workers*callsPerWorker)

	for worker := 0; worker < workers; worker++ {
		worker := worker
		wg.Add(1)
		go func() {
			defer wg.Done()
			rng := rand.New(rand.NewSource(randomSeedBase + int64(worker)))
			for i := 0; i < callsPerWorker; i++ {
				orgID := orgIDs[rng.Intn(len(orgIDs))]
				if _, err := provider.GetManager(orgID); err != nil {
					errCh <- fmt.Errorf("worker %d GetManager(%s) failed: %w", worker, orgID, err)
				}
			}
		}()
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(testWaitTimeout):
		t.Fatal("concurrent access stress test timed out (possible deadlock)")
	}

	close(errCh)
	for err := range errCh {
		t.Fatal(err)
	}

	if got, want := provider.ManagerCount(), len(orgIDs); got != want {
		t.Fatalf("ManagerCount()=%d, want %d", got, want)
	}

	if err := provider.Close(); err != nil {
		t.Fatalf("Close() failed: %v", err)
	}

	if got := provider.ManagerCount(); got != 0 {
		t.Fatalf("ManagerCount() after Close()=%d, want 0", got)
	}
}

func TestTenantRBACProvider_ConcurrentGetAndRemove(t *testing.T) {
	baseDir := t.TempDir()
	orgIDs := makeOrgIDs("mix-org", 10)
	mustCreateOrgDirs(t, baseDir, orgIDs)

	provider := NewTenantRBACProvider(baseDir)

	const (
		workers         = 20
		randomSeedBase  = int64(22_000)
		runDuration     = 500 * time.Millisecond
		shutdownTimeout = 10 * time.Second
	)

	stop := make(chan struct{})
	errCh := make(chan error, 1024)
	var wg sync.WaitGroup

	for worker := 0; worker < workers; worker++ {
		worker := worker
		wg.Add(1)
		go func() {
			defer wg.Done()
			rng := rand.New(rand.NewSource(randomSeedBase + int64(worker)))
			for {
				select {
				case <-stop:
					return
				default:
				}

				orgID := orgIDs[rng.Intn(len(orgIDs))]
				var err error
				if worker%2 == 0 {
					_, err = provider.GetManager(orgID)
				} else {
					err = provider.RemoveTenant(orgID)
				}
				if err != nil {
					errCh <- fmt.Errorf("worker %d org %s failed: %w", worker, orgID, err)
				}
			}
		}()
	}

	time.Sleep(runDuration)
	close(stop)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(shutdownTimeout):
		t.Fatal("concurrent get/remove test timed out (possible deadlock)")
	}

	close(errCh)
	for err := range errCh {
		t.Fatal(err)
	}

	if err := provider.Close(); err != nil {
		t.Fatalf("Close() failed: %v", err)
	}
}

func TestTenantRBACProvider_NoConnectionLeak(t *testing.T) {
	baseDir := t.TempDir()
	orgIDs := makeOrgIDs("leak-org", 50)
	mustCreateOrgDirs(t, baseDir, orgIDs)

	provider := NewTenantRBACProvider(baseDir)
	for _, orgID := range orgIDs {
		if _, err := provider.GetManager(orgID); err != nil {
			t.Fatalf("first provider GetManager(%s) failed: %v", orgID, err)
		}
	}

	if got, want := provider.ManagerCount(), len(orgIDs); got != want {
		t.Fatalf("first provider ManagerCount()=%d, want %d", got, want)
	}

	if err := provider.Close(); err != nil {
		t.Fatalf("first provider Close() failed: %v", err)
	}

	if got := provider.ManagerCount(); got != 0 {
		t.Fatalf("first provider ManagerCount() after Close()=%d, want 0", got)
	}

	freshProvider := NewTenantRBACProvider(baseDir)
	for _, orgID := range orgIDs {
		if _, err := freshProvider.GetManager(orgID); err != nil {
			t.Fatalf("fresh provider GetManager(%s) failed: %v", orgID, err)
		}
	}

	if err := freshProvider.Close(); err != nil {
		t.Fatalf("fresh provider Close() failed: %v", err)
	}
}

func makeOrgIDs(prefix string, count int) []string {
	orgIDs := make([]string, 0, count)
	for i := 0; i < count; i++ {
		orgIDs = append(orgIDs, fmt.Sprintf("%s-%03d", prefix, i))
	}
	return orgIDs
}

func mustCreateOrgDirs(tb testing.TB, baseDir string, orgIDs []string) {
	tb.Helper()
	for _, orgID := range orgIDs {
		orgDir := filepath.Join(baseDir, "orgs", orgID)
		if err := os.MkdirAll(orgDir, 0700); err != nil {
			tb.Fatalf("failed to create org dir %s: %v", orgDir, err)
		}
	}
}
