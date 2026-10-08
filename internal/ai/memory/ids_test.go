package memory

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

// TestSequencedIDsStayUniqueUnderConcurrentMinting mints from every
// package-level ID sequence on many goroutines at once. Under -race it proves
// the sequences are synchronized. The uniqueness check proves the suffix does
// not wrap: more than a thousand IDs from one generator land in the same
// wall-clock second, which the old counter%1000 suffix turned into duplicates.
func TestSequencedIDsStayUniqueUnderConcurrentMinting(t *testing.T) {
	generators := []struct {
		prefix string
		mint   func() string
	}{
		{"inc-", generateIncidentID},
		{"inc-evt-", generateIncidentEventID},
		{"rem-", generateRecordID},
		{"mem-", generateMemoryID},
		{"inc-mem-", generateIncidentMemoryID},
		{"pat-mem-", generatePatternMemoryID},
	}
	const goroutines, perGoroutine = 16, 150

	for _, gen := range generators {
		t.Run(strings.TrimSuffix(gen.prefix, "-"), func(t *testing.T) {
			shape := regexp.MustCompile("^" + regexp.QuoteMeta(gen.prefix) + `\d{14}-[1-9]\d*$`)
			minted := make([][]string, goroutines)
			start := make(chan struct{})
			var wg sync.WaitGroup
			for g := range goroutines {
				wg.Add(1)
				go func() {
					defer wg.Done()
					<-start
					batch := make([]string, perGoroutine)
					for i := range batch {
						batch[i] = gen.mint()
					}
					minted[g] = batch
				}()
			}
			close(start)
			wg.Wait()

			seen := make(map[string]bool, goroutines*perGoroutine)
			for _, batch := range minted {
				for _, id := range batch {
					if !shape.MatchString(id) {
						t.Fatalf("ID %q does not match %s", id, shape)
					}
					if seen[id] {
						t.Fatalf("duplicate ID %q among %d concurrent mints", id, goroutines*perGoroutine)
					}
					seen[id] = true
				}
			}
		})
	}
}

// TestIncidentStoresMintIDsConcurrentlyAcrossTenants mirrors multi-tenant
// monitoring: each tenant monitor owns its own IncidentStore, and their alert
// checks call RecordAlertFired from separate goroutines. Each store's mutex
// covers only its own state, so the shared ID sequence must be synchronized on
// its own. Run under -race.
func TestIncidentStoresMintIDsConcurrentlyAcrossTenants(t *testing.T) {
	const tenants, checkersPerTenant, alertsPerChecker = 2, 4, 50
	stores := make([]*IncidentStore, tenants)
	for i := range stores {
		stores[i] = NewIncidentStore(IncidentStoreConfig{MaxIncidents: checkersPerTenant * alertsPerChecker})
	}

	startedAt := time.Now().Add(-time.Minute)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for tenant, store := range stores {
		for checker := range checkersPerTenant {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				for n := range alertsPerChecker {
					store.RecordAlertFired(&alerts.Alert{
						ID:           fmt.Sprintf("tenant-%d-checker-%d-alert-%d", tenant, checker, n),
						Type:         "cpu",
						Level:        alerts.AlertLevelWarning,
						ResourceID:   fmt.Sprintf("tenant-%d-vm-%d-%d", tenant, checker, n),
						ResourceName: "vm",
						StartTime:    startedAt,
					})
				}
			}()
		}
	}
	close(start)
	wg.Wait()

	seen := make(map[string]bool, tenants*checkersPerTenant*alertsPerChecker)
	for tenant, store := range stores {
		store.mu.RLock()
		incidents := len(store.incidents)
		for _, shell := range store.incidents {
			if seen[shell.ID] {
				t.Errorf("tenant %d incident ID %q was already minted", tenant, shell.ID)
			}
			seen[shell.ID] = true
		}
		store.mu.RUnlock()
		if incidents != checkersPerTenant*alertsPerChecker {
			t.Errorf("tenant %d recorded %d incidents, want %d", tenant, incidents, checkersPerTenant*alertsPerChecker)
		}
	}
}
