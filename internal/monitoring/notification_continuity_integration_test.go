package monitoring

// Real monitor lifecycle, persistent queue and HTTP restart regression.
import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"

	_ "modernc.org/sqlite"
)

const (
	continuityContinuityProofCooldownMinutes = 30
	continuityProofRestoredWait              = 15 * time.Second // restored criticals re-dispatch after 10s
)

type continuityProofReceipt struct {
	Event      string         `json:"event"`
	Identifier string         `json:"alertIdentifier"`
	Alerts     []alerts.Alert `json:"alerts"`
}

type continuityProofReceiver struct {
	server *httptest.Server
	mu     sync.Mutex
	got    []continuityProofReceipt
}

func continuityNewProofReceiver(t *testing.T) *continuityProofReceiver {
	r := &continuityProofReceiver{}
	r.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		defer req.Body.Close()
		var got continuityProofReceipt
		if err := json.NewDecoder(req.Body).Decode(&got); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		r.mu.Lock()
		r.got = append(r.got, got)
		r.mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(r.server.Close)
	return r
}

func (r *continuityProofReceiver) snapshot() []continuityProofReceipt {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]continuityProofReceipt(nil), r.got...)
}

func (r *continuityProofReceiver) count(event, alertID string, start time.Time) int {
	n := 0
	for _, got := range r.snapshot() {
		if got.Event != event {
			continue
		}
		for _, a := range got.Alerts {
			if a.ID == alertID && a.StartTime.Equal(start) {
				n++
			}
		}
	}
	return n
}

func (r *continuityProofReceiver) waitFor(t *testing.T, event, alertID string, start time.Time, want int, within time.Duration) bool {
	t.Helper()
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		if r.count(event, alertID, start) >= want {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return r.count(event, alertID, start) >= want
}

func continuityStartProofMonitor(t *testing.T, alertDir, notificationDir, receiverURL string) *Monitor {
	t.Helper()
	a := alerts.NewManagerWithDataDir(alertDir, alerts.WithDurableAlertStore())
	n := notifications.NewNotificationManagerWithDeferredQueue("http://pulse.example", notificationDir)
	if err := n.UpdateAllowedPrivateCIDRs("127.0.0.1/32,::1/128"); err != nil {
		t.Fatal(err)
	}
	n.AddWebhook(notifications.WebhookConfig{ID: "restart-proof", URL: receiverURL, Enabled: true, Service: "generic"})
	n.SetNotifyOnResolve(true)
	n.SetGroupingWindow(0)
	n.SetCooldown(continuityContinuityProofCooldownMinutes)
	a.UpdateConfig(alerts.AlertConfig{Enabled: true, ActivationState: alerts.ActivationActive,
		TimeThresholds: map[string]int{"storage": 0},
		StorageDefault: alerts.HysteresisThreshold{Trigger: 80, Clear: 70}})
	m := &Monitor{alertManager: a, notificationMgr: n}
	m.wireExternalAlertCallbacks(nil)
	n.SetOccurrenceLifecycleProvider(a.NotificationOccurrenceLifecycle)
	n.SetQuietHoursPolicyProvider(a.QuietHoursNotificationPolicy)
	if err := n.StartQueueProcessingWithConfiguration(true); err != nil {
		t.Fatal(err)
	}
	return m
}

func continuityStopProofMonitor(m *Monitor) { m.alertManager.Stop(); m.notificationMgr.Stop() }

func continuityObserveProofStorage(m *Monitor, usage float64) {
	s := models.Storage{ID: "pbs-primary-backups", Name: "backups", Instance: "pbs-primary", Type: "pbs", Status: "online",
		Total: 1000, Used: int64(usage * 10), Free: 1000 - int64(usage*10), Usage: usage}
	for range 5 {
		m.alertManager.CheckStorage(s)
	}
}

func continuityDrainProofQueue(t *testing.T, m *Monitor) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		stats, err := m.notificationMgr.GetQueueStats()
		if err != nil {
			t.Fatal(err)
		}
		if stats["pending"]+stats["sending"] == 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("queue did not drain: %v", stats)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// continuityDurableFiringReceipts reads the persisted receipt rows for one occurrence
// through an independent read-only SQLite handle.
func continuityDurableFiringReceipts(t *testing.T, notificationDir, alertID string, start time.Time) int {
	t.Helper()
	var dbPath string
	_ = filepath.WalkDir(notificationDir, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && d.Name() == "notification_queue.db" {
			dbPath = path
		}
		return nil
	})
	if dbPath == "" {
		t.Fatal("notification queue database not found")
	}
	db, err := sql.Open("sqlite", "file:"+dbPath+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM notification_delivery_receipts WHERE alert_id = ? AND alert_start_nanos = ?`,
		alertID, start.UnixNano()).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func continuityFirstCriticalFiring(t *testing.T, r *continuityProofReceiver) alerts.Alert {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		for _, got := range r.snapshot() {
			if got.Event == "" && len(got.Alerts) == 1 && got.Alerts[0].Level == alerts.AlertLevelCritical {
				return got.Alerts[0]
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("no critical firing received: %+v", r.snapshot())
	return alerts.Alert{}
}

func TestNotificationExtendedMonitorRestartKeepsDeliveredCriticalCooldown(t *testing.T) {
	receiver := continuityNewProofReceiver(t)
	alertDir, notificationDir := t.TempDir(), t.TempDir()

	m := continuityStartProofMonitor(t, alertDir, notificationDir, receiver.server.URL)
	continuityObserveProofStorage(m, 95) // trigger 80 -> critical at 90
	original := continuityFirstCriticalFiring(t, receiver)
	continuityDrainProofQueue(t, m)
	receipts := continuityDurableFiringReceipts(t, notificationDir, original.ID, original.StartTime)
	t.Logf("before restart: firing deliveries for occurrence = %d, durable receipts = %d",
		receiver.count("", original.ID, original.StartTime), receipts)
	if receipts != 1 {
		t.Fatalf("expected one durable firing receipt before restart, got %d", receipts)
	}
	continuityStopProofMonitor(m)
	stoppedAt := time.Now()

	m = continuityStartProofMonitor(t, alertDir, notificationDir, receiver.server.URL)
	t.Cleanup(func() { continuityStopProofMonitor(m) })
	active := m.alertManager.GetActiveAlerts()
	if len(active) != 1 || active[0].ID != original.ID || !active[0].StartTime.Equal(original.StartTime) || active[0].Acknowledged {
		t.Fatalf("restart did not restore the same unacknowledged occurrence: %+v", active)
	}
	time.Sleep(continuityProofRestoredWait)
	continuityDrainProofQueue(t, m)
	firings := receiver.count("", original.ID, original.StartTime)
	t.Logf("after restart (%.1fs later, cooldown %dm): firing deliveries for the same occurrence = %d",
		time.Since(stoppedAt).Seconds(), continuityContinuityProofCooldownMinutes, firings)
	if firings != 1 {
		t.Errorf("restart inside cooldown re-delivered the already-delivered firing (%d deliveries of one occurrence)", firings)
	}

	// Controls a fix must preserve: the recovery still reaches the destination
	// that received the firing, and a genuinely new occurrence still delivers.
	continuityObserveProofStorage(m, 10)
	if !receiver.waitFor(t, "resolved", original.ID, original.StartTime, 1, 10*time.Second) {
		t.Fatalf("recovery for the delivered occurrence was not delivered: %+v", receiver.snapshot())
	}
	continuityDrainProofQueue(t, m)
	time.Sleep(10 * time.Millisecond) // occurrence start times must differ
	continuityObserveProofStorage(m, 95)
	next := m.alertManager.GetActiveAlerts()
	if len(next) != 1 || next[0].StartTime.Equal(original.StartTime) {
		t.Fatalf("expected a new occurrence: %+v", next)
	}
	if !receiver.waitFor(t, "", next[0].ID, next[0].StartTime, 1, 10*time.Second) {
		t.Fatalf("new occurrence was not delivered: %+v", receiver.snapshot())
	}
}

// Native subprocess form. Phases run in child processes of this test binary.
func TestNotificationExtendedMonitorNativeRestartKeepsDeliveredCriticalCooldown(t *testing.T) {
	if phase := os.Getenv("PULSE_RESTART_PROOF_PHASE"); phase != "" {
		continuityRunRestartProofPhase(t, phase)
		return
	}
	receiver := continuityNewProofReceiver(t)
	alertDir, notificationDir := t.TempDir(), t.TempDir()
	run := func(phase string) {
		cmd := exec.Command(os.Args[0], "-test.run=^TestNotificationExtendedMonitorNativeRestartKeepsDeliveredCriticalCooldown$", "-test.v", "-test.count=1")
		cmd.Env = append(os.Environ(),
			"PULSE_RESTART_PROOF_PHASE="+phase,
			"PULSE_RESTART_PROOF_ALERT_DIR="+alertDir,
			"PULSE_RESTART_PROOF_NOTIFICATION_DIR="+notificationDir,
			"PULSE_RESTART_PROOF_RECEIVER="+receiver.server.URL,
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("phase %s process failed: %v\n%s", phase, err, out)
		}
		t.Logf("phase %s process pid exited cleanly", phase)
	}
	run("first-process")
	original := continuityFirstCriticalFiring(t, receiver)
	t.Logf("first process: firing deliveries = %d, durable receipts = %d",
		receiver.count("", original.ID, original.StartTime),
		continuityDurableFiringReceipts(t, notificationDir, original.ID, original.StartTime))
	run("second-process")
	firings := receiver.count("", original.ID, original.StartTime)
	t.Logf("after second process (cooldown %dm): firing deliveries for the same occurrence = %d", continuityContinuityProofCooldownMinutes, firings)
	if firings != 1 {
		t.Errorf("a new process inside cooldown re-delivered the already-delivered firing (%d deliveries of one occurrence)", firings)
	}
}

func continuityRunRestartProofPhase(t *testing.T, phase string) {
	alertDir := os.Getenv("PULSE_RESTART_PROOF_ALERT_DIR")
	notificationDir := os.Getenv("PULSE_RESTART_PROOF_NOTIFICATION_DIR")
	receiverURL := os.Getenv("PULSE_RESTART_PROOF_RECEIVER")
	m := continuityStartProofMonitor(t, alertDir, notificationDir, receiverURL)
	switch phase {
	case "first-process":
		continuityObserveProofStorage(m, 95)
		deadline := time.Now().Add(10 * time.Second)
		for {
			active := m.alertManager.GetActiveAlerts()
			if len(active) == 1 && continuityDurableFiringReceipts(t, notificationDir, active[0].ID, active[0].StartTime) == 1 {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("first process did not record the firing receipt")
			}
			time.Sleep(20 * time.Millisecond)
		}
	case "second-process":
		time.Sleep(continuityProofRestoredWait)
	default:
		t.Fatalf("unknown phase %q", phase)
	}
	continuityDrainProofQueue(t, m)
	continuityStopProofMonitor(m)
	fmt.Printf("phase %s pid %d finished\n", phase, os.Getpid())
}

func TestNotificationExtendedMonitorUnreadableDestinationsRetainRecoveryAuthority(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", dir)
	var mu sync.Mutex
	var events []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Event string `json:"event"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(400)
			return
		}
		mu.Lock()
		events = append(events, body.Event)
		mu.Unlock()
		w.WriteHeader(200)
	}))
	defer server.Close()
	persistence := config.NewConfigPersistence(dir)
	if err := persistence.SaveAlertConfig(alerts.AlertConfig{Enabled: true, ActivationState: alerts.ActivationActive, Schedule: alerts.ScheduleConfig{NotifyOnResolve: true}}); err != nil {
		t.Fatal(err)
	}
	hook := notifications.WebhookConfig{ID: "ops", URL: server.URL, Enabled: true, Service: "generic"}
	if err := persistence.SaveWebhooks([]notifications.WebhookConfig{hook}); err != nil {
		t.Fatal(err)
	}
	open := func() *Monitor {
		m, err := New(&config.Config{DataPath: dir})
		if err != nil {
			t.Fatal(err)
		}
		if err = m.notificationMgr.UpdateAllowedPrivateCIDRs("127.0.0.1/32"); err != nil {
			m.Stop()
			t.Fatal(err)
		}
		return m
	}
	m := open()
	m.notificationMgr.SetGroupingConfig(false, 0, false, false)
	m.alertManager.UpdateConfig(alerts.AlertConfig{Enabled: true, ActivationState: alerts.ActivationActive, TimeThresholds: map[string]int{"storage": 0}, StorageDefault: alerts.HysteresisThreshold{Trigger: 80, Clear: 70}, Schedule: alerts.ScheduleConfig{NotifyOnResolve: true}})
	continuityObserveProofStorage(m, 95)
	deadline := time.Now().Add(10 * time.Second)
	for {
		mu.Lock()
		count := len(events)
		mu.Unlock()
		if count == 1 {
			break
		}
		if time.Now().After(deadline) {
			m.Stop()
			t.Fatal("detected firing did not reach provider")
		}
		time.Sleep(5 * time.Millisecond)
	}
	continuityDrainProofQueue(t, m)
	m.notificationMgr.SetGroupingConfig(true, 3600, false, false)
	continuityObserveProofStorage(m, 0)
	deadline = time.Now().Add(10 * time.Second)
	for {
		stats, err := m.notificationMgr.GetQueueStats()
		if err != nil {
			m.Stop()
			t.Fatal(err)
		}
		if stats["pending"] == 1 {
			break
		}
		if time.Now().After(deadline) {
			m.Stop()
			t.Fatal("detected recovery was not admitted")
		}
		time.Sleep(5 * time.Millisecond)
	}
	m.Stop()
	db, err := sql.Open("sqlite", filepath.Join(dir, "notification_queue.db"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`UPDATE notification_queue SET next_retry_at=? WHERE type='webhook_resolved'`, time.Now().Add(-time.Minute).Unix()); err != nil {
		t.Fatal(err)
	}
	db.Close()
	configPath := filepath.Join(dir, "webhooks.enc")
	saved, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(configPath, []byte("unreadable synthetic destination file"), 0600); err != nil {
		t.Fatal(err)
	}
	held, loadErr := New(&config.Config{DataPath: dir})
	if loadErr != nil {
		t.Fatalf("notification damage stopped monitoring: %v", loadErr)
	}
	if held.notificationMgr.ConfigurationError() == nil || held.notificationMgr.DeliveryHealth().Healthy {
		t.Fatal("unreadable inventory did not hold notification delivery")
	}
	if err := held.notificationMgr.SetEmailConfig(notifications.EmailConfig{}); err == nil {
		t.Fatal("held inventory accepted partial destination publication")
	}
	if err := held.notificationMgr.StartQueueProcessingWithConfiguration(true); err == nil {
		t.Fatal("held inventory activated through a low-level start")
	}
	held.alertManager.UpdateConfig(alerts.AlertConfig{Enabled: true, ActivationState: alerts.ActivationActive,
		TimeThresholds: map[string]int{"storage": 0}, StorageDefault: alerts.HysteresisThreshold{Trigger: 80, Clear: 70}})
	other := models.Storage{ID: "held-other", Name: "held detection", Instance: "held-instance", Type: "pbs", Status: "online", Total: 1000, Used: 950, Free: 50, Usage: 95}
	for range 5 {
		held.alertManager.CheckStorage(other)
	}
	detected := false
	for _, active := range held.alertManager.GetActiveAlerts() {
		if active.ResourceID == other.ID {
			detected = true
		}
	}
	if !detected {
		held.Stop()
		t.Fatal("notification hold disabled alert detection")
	}
	other.Used, other.Free, other.Usage = 0, 1000, 0
	for range 5 {
		held.alertManager.CheckStorage(other)
	}
	for _, active := range held.alertManager.GetActiveAlerts() {
		if active.ResourceID == other.ID {
			held.Stop()
			t.Fatal("held detector could not resolve the synthetic incident")
		}
	}
	held.Stop()
	if unchanged, err := os.ReadFile(configPath); err != nil || string(unchanged) != "unreadable synthetic destination file" {
		t.Fatal("held notification owner overwrote the unreadable inventory")
	}
	db, err = sql.Open("sqlite", filepath.Join(dir, "notification_queue.db"))
	if err != nil {
		t.Fatal(err)
	}
	var pending, cancelled int
	err = db.QueryRow(`SELECT COUNT(CASE WHEN status='pending' THEN 1 END),COUNT(CASE WHEN status='cancelled' THEN 1 END) FROM notification_queue WHERE type='webhook_resolved'`).Scan(&pending, &cancelled)
	db.Close()
	if err != nil || pending != 1 || cancelled != 0 {
		t.Fatalf("failed bootstrap changed recovery: pending=%d cancelled=%d error=%v", pending, cancelled, err)
	}
	mu.Lock()
	count := len(events)
	mu.Unlock()
	if count != 1 {
		t.Fatalf("unexpected delivery during failed bootstrap: %d", count)
	}
	if err = os.WriteFile(configPath, saved, 0600); err != nil {
		t.Fatal(err)
	}
	m = open()
	defer m.Stop()
	continuityDrainProofQueue(t, m)
	mu.Lock()
	defer mu.Unlock()
	if len(events) != 2 || events[1] != "resolved" {
		t.Fatalf("restored configuration lost owed recovery: %v", events)
	}
}

func TestNotificationExtendedStorageWriteFailureHoldsDeliveryWithoutStoppingDetection(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", dir)
	persistence := config.NewConfigPersistence(dir)
	var wireMu sync.Mutex
	wireCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wireMu.Lock()
		wireCount++
		wireMu.Unlock()
		w.WriteHeader(200)
	}))
	defer server.Close()
	settings := config.DefaultSystemSettings()
	settings.WebhookAllowedPrivateCIDRs = "127.0.0.1/32"
	if err := persistence.SaveSystemSettings(*settings); err != nil {
		t.Fatal(err)
	}
	if err := persistence.SaveAlertConfig(alerts.AlertConfig{Enabled: true, ActivationState: alerts.ActivationActive}); err != nil {
		t.Fatal(err)
	}
	if err := persistence.SaveWebhooks([]notifications.WebhookConfig{{ID: "storage-held", URL: server.URL, Enabled: true, Service: "generic"}}); err != nil {
		t.Fatal(err)
	}
	q, err := notifications.NewNotificationQueue(dir)
	if err != nil {
		t.Fatal(err)
	}
	q.Stop()
	db, err := sql.Open("sqlite", filepath.Join(dir, "notification_queue.db"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TRIGGER reject_bootstrap_destination BEFORE INSERT ON notification_destination_state BEGIN SELECT RAISE(ABORT,'synthetic destination authority write failure'); END`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	held, err := New(&config.Config{DataPath: dir})
	if err != nil {
		t.Fatalf("notification write failure stopped monitoring: %v", err)
	}
	if notifications.ConfigurationHoldReason(held.notificationMgr.ConfigurationError()) != notifications.NotificationReasonStorageBootstrapUnavailable {
		held.Stop()
		t.Fatal("notification storage hold was presented as unreadable destination files")
	}
	held.alertManager.UpdateConfig(alerts.AlertConfig{Enabled: true, ActivationState: alerts.ActivationActive, TimeThresholds: map[string]int{"storage": 0}, StorageDefault: alerts.HysteresisThreshold{Trigger: 80, Clear: 70}})
	continuityObserveProofStorage(held, 95)
	if len(held.alertManager.GetActiveAlerts()) == 0 {
		held.Stop()
		t.Fatal("notification write failure disabled alert detection")
	}
	continuityObserveProofStorage(held, 0)
	held.Stop()
	wireMu.Lock()
	count := wireCount
	wireMu.Unlock()
	if count != 0 {
		t.Fatalf("held storage contacted provider %d times", count)
	}
	db, err = sql.Open("sqlite", filepath.Join(dir, "notification_queue.db"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DROP TRIGGER reject_bootstrap_destination`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	restored, err := New(&config.Config{DataPath: dir})
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Stop()
	if restored.notificationMgr.ConfigurationError() != nil {
		t.Fatal("repaired notification storage did not reconstruct delivery authority")
	}
}
