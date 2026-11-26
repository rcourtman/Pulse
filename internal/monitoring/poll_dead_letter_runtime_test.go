package monitoring

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func assertDeadLetterScrape(t *testing.T, handler http.Handler, phase string, want map[metricKey]int) {
	t.Helper()
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("%s: Prometheus scrape returned %d", phase, rec.Code)
	}
	body := rec.Body.String()
	got := 0
	for line := range strings.SplitSeq(body, "\n") {
		if strings.HasPrefix(line, "pulse_scheduler_dead_letter_depth{") {
			t.Logf("DEAD_LETTER_SCRAPE %s %s", phase, line)
			got++
		}
	}
	if got != len(want) {
		t.Errorf("%s: got %d dead-letter series, want %d", phase, got, len(want))
	}
	for key, count := range want {
		sample := fmt.Sprintf("pulse_scheduler_dead_letter_depth{instance=%q,instance_type=%q} %d", key.instance, key.instanceType, count)
		if !strings.Contains(body, "\n"+sample+"\n") {
			t.Errorf("%s: missing Prometheus sample %q", phase, sample)
		}
	}
}

func TestSchedulerDeadLettersFollowTenantRuntime(t *testing.T) {
	mtp, p := newTestTenantPersistence(t)
	for _, id := range []string{"replace", "peer"} {
		if err := mtp.SaveOrganization(&models.Organization{ID: id, DisplayName: id}); err != nil {
			t.Fatal(err)
		}
	}
	reg := prometheus.NewRegistry()
	pm := newPollMetrics(reg)
	mtm := NewMultiTenantMonitor(&config.Config{DataPath: p.GetConfigDir()}, mtp, nil)
	t.Cleanup(mtm.Stop)
	mtm.SetMonitorInitializer(func(m *Monitor) { m.pollMetrics = pm })

	get := func(id string) *Monitor {
		t.Helper()
		m, err := mtm.GetMonitor(id)
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	old, peer := get("replace"), get("peer")
	park := func(m *Monitor, name string) {
		m.sendToDeadLetter(ScheduledTask{InstanceType: InstanceTypePVE, InstanceName: name}, errors.New("fixture terminal poll"))
	}
	// The shared label is intentionally present in both live queues; the
	// process-wide series must add the two sources, not let one erase the other.
	park(old, "shared")
	park(old, "old-only")
	park(peer, "shared")
	park(peer, "peer-only")
	handler := promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
	scrape := func(phase string, shared, oldOnly, peerOnly int) {
		t.Helper()
		assertDeadLetterScrape(t, handler, phase, map[metricKey]int{
			{"pve", "shared"}:    shared,
			{"pve", "old-only"}:  oldOnly,
			{"pve", "peer-only"}: peerOnly,
		})
	}
	scrape("both-runtimes-active", 2, 1, 1)

	// Capture exactly what an old updateDeadLetterMetrics call would capture.
	// Delay publication until after the production ReloadTenant path has
	// cancelled, stopped and replaced the monitor.
	captured := make(chan int, 1)
	publish, published := make(chan struct{}), make(chan struct{})
	go func() {
		tasks := old.deadLetterQueue.PeekAll(old.deadLetterQueue.Size())
		captured <- len(tasks)
		<-publish
		pm.UpdateDeadLetterCounts(tasks)
		close(published)
	}()
	finishRefresh := sync.OnceFunc(func() {
		close(publish)
		<-published
	})
	defer finishRefresh()
	if n := <-captured; n != 2 {
		t.Fatalf("old refresh captured %d terminal entries, want 2", n)
	}

	replacement, err := mtm.ReloadTenant("replace")
	if err != nil {
		t.Fatal(err)
	}
	if replacement == old || replacement.deadLetterQueue == old.deadLetterQueue ||
		old.DeadLetterCount() != 2 || replacement.DeadLetterCount() != 0 {
		t.Fatal("fixture did not retain the old failed queue alongside an empty replacement")
	}
	// A new scrape must be right without waiting for GC, a worker result or
	// a Scheduler Health call on either monitor.
	scrape("after-reload-before-health", 1, 0, 1)
	finishRefresh()
	scrape("after-delayed-old-refresh", 1, 0, 1)
	if count := replacement.SchedulerHealth().DeadLetter.Count; count != 0 {
		t.Errorf("replacement Scheduler Health has %d dead letters, want zero", count)
	}

	park(replacement, "shared")
	scrape("replacement-fails-with-same-label", 2, 0, 1)
	if err := mtm.removeTenant("peer", false); err != nil {
		t.Fatal(err)
	}
	scrape("peer-removed", 1, 0, 0)
	if err := mtm.stop(); err != nil {
		t.Fatal(err)
	}
	scrape("last-runtime-stopped", 0, 0, 0)
	// Keep the old queue live across every scrape. Dropping the last strong
	// reference or clearing the queue would not prove runtime ownership.
	if count := old.DeadLetterCount(); count != 2 {
		t.Errorf("retired queue retained %d terminal entries, want 2", count)
	}
}
