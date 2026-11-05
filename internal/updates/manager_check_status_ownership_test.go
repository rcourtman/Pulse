package updates

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// These tests pin update-status ownership: an update check that is running
// when an apply or rollback is admitted must never replace that operation's
// progress or terminal status, including when the check returns after the
// operation has ended. They drive the real CheckForUpdatesWithOptions,
// ApplyUpdate and RollbackToBackup concurrently against a controllable
// release server, and observe the status snapshot, the SSE cached status,
// the progress channel and a real SSE client.

const ownershipTimeout = 10 * time.Second

// checkOriginErrorMessages are the error messages only the update-check path
// publishes. Apply and rollback never publish idle, checking or available.
var checkOriginErrorMessages = map[string]bool{
	"Failed to get current version":         true,
	"Invalid current version":               true,
	"Failed to check for Pulse Pro updates": true,
	"Fresh update check unavailable":        true,
	"Failed to check for updates":           true,
	"Invalid latest version":                true,
}

func isCheckOriginStatus(s UpdateStatus) bool {
	switch s.Status {
	case "checking", "available", "idle":
		return true
	case "error":
		return checkOriginErrorMessages[s.Message]
	}
	return false
}

type releaseCall struct {
	reply chan bool // true: fail the release lookup
}

type statusFeed struct {
	mu       sync.Mutex
	statuses []UpdateStatus
}

func (f *statusFeed) add(s UpdateStatus) {
	f.mu.Lock()
	f.statuses = append(f.statuses, s)
	f.mu.Unlock()
}

func (f *statusFeed) snapshot() []UpdateStatus {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]UpdateStatus(nil), f.statuses...)
}

type ownershipHarness struct {
	t               *testing.T
	manager         *Manager
	server          *httptest.Server
	releaseCalls    chan releaseCall
	autoReply       atomic.Bool // answer release lookups immediately with success
	holdDownload    atomic.Bool // hold the archive download until released
	downloadStarted chan struct{}
	downloadRelease chan struct{}
	releaseOnce     sync.Once
	closing         chan struct{} // closed at teardown to free held handlers
	progress        *statusFeed
	sse             *statusFeed
	sentinels       int
}

func newOwnershipHarness(t *testing.T) *ownershipHarness {
	t.Helper()
	withBuildVersion(t, "6.6.0")
	t.Setenv("PULSE_ALLOW_DOCKER_UPDATES", "true") // never exit the test process
	dataDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", dataDir)
	t.Setenv("PULSE_INSTALL_DIR", t.TempDir())

	h := &ownershipHarness{
		t:               t,
		releaseCalls:    make(chan releaseCall),
		downloadStarted: make(chan struct{}, 1),
		downloadRelease: make(chan struct{}),
		closing:         make(chan struct{}),
		progress:        &statusFeed{},
		sse:             &statusFeed{},
	}
	h.holdDownload.Store(true)

	const archive = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	h.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == updateReleaseAPIPath():
			fail := false
			if !h.autoReply.Load() {
				call := releaseCall{reply: make(chan bool, 1)}
				select {
				case h.releaseCalls <- call:
				case <-h.closing:
					return
				case <-r.Context().Done():
					return
				}
				select {
				case fail = <-call.reply:
				case <-h.closing:
					return
				case <-r.Context().Done():
					return
				}
			}
			if fail {
				http.Error(w, "release lookup rejected", http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]ReleaseInfo{makeRelease("v6.7.0", false)})
		case strings.HasSuffix(r.URL.Path, ".tar.gz"):
			if !h.holdDownload.Load() {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Length", fmt.Sprint(len(archive)))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(archive[:8]))
			w.(http.Flusher).Flush()
			select {
			case h.downloadStarted <- struct{}{}:
			default:
			}
			select {
			case <-h.downloadRelease:
			case <-h.closing:
				return
			case <-r.Context().Done():
				return
			}
			_, _ = w.Write([]byte(archive[8:]))
		default:
			// Includes the .sshsig sidecar: a released download then fails
			// signature verification, a genuine apply error.
			http.NotFound(w, r)
		}
	}))
	t.Setenv("PULSE_UPDATE_SERVER", h.server.URL)

	h.manager = NewManager(&config.Config{UpdateChannel: "stable"})

	progressDone := make(chan struct{})
	go func() {
		defer close(progressDone)
		for status := range h.manager.GetProgressChannel() {
			h.progress.add(status)
		}
	}()

	sseServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.manager.AddSSEClient(w, "ownership-test") == nil {
			http.Error(w, "no stream", http.StatusInternalServerError)
			return
		}
		<-r.Context().Done()
		// RemoveSSEClient joins the client's writer before returning.
		h.manager.RemoveSSEClient("ownership-test")
	}))
	sseCtx, cancelSSE := context.WithCancel(context.Background())
	req, err := http.NewRequestWithContext(sseCtx, http.MethodGet, sseServer.URL, nil)
	if err != nil {
		t.Fatalf("build SSE request: %v", err)
	}
	resp, err := sseServer.Client().Do(req)
	if err != nil {
		t.Fatalf("open SSE stream: %v", err)
	}
	sseDone := make(chan struct{})
	go func() {
		defer close(sseDone)
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			var status UpdateStatus
			if json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &status) == nil {
				h.sse.add(status)
			}
		}
	}()

	t.Cleanup(func() {
		close(h.closing)
		cancelSSE()
		resp.Body.Close()
		<-sseDone
		sseServer.Close()
		h.manager.Close()
		<-progressDone
		h.server.Close()
	})
	return h
}

type checkResult struct {
	info *UpdateInfo
	err  error
}

// startCheck runs a forced check on the install's own channel, so it
// populates the check cache and the effective-channel observation.
func (h *ownershipHarness) startCheck() <-chan checkResult {
	done := make(chan checkResult, 1)
	go func() {
		info, err := h.manager.CheckForUpdatesWithOptions(context.Background(), UpdateCheckOptions{Force: true})
		done <- checkResult{info: info, err: err}
	}()
	return done
}

func (h *ownershipHarness) nextReleaseCall() releaseCall {
	h.t.Helper()
	select {
	case call := <-h.releaseCalls:
		return call
	case <-time.After(ownershipTimeout):
		h.t.Fatal("update check never reached the release server")
	}
	return releaseCall{}
}

func (h *ownershipHarness) startApply() <-chan error {
	done := make(chan error, 1)
	go func() {
		done <- h.manager.ApplyUpdate(context.Background(), ApplyUpdateRequest{
			DownloadURL: h.server.URL + "/rcourtman/Pulse/releases/download/v6.7.0/pulse-v6.7.0-linux-amd64.tar.gz",
			Channel:     "stable",
		})
	}()
	return done
}

func (h *ownershipHarness) releaseDownload() {
	h.releaseOnce.Do(func() { close(h.downloadRelease) })
}

func (h *ownershipHarness) waitDownloadStarted() {
	h.t.Helper()
	select {
	case <-h.downloadStarted:
	case <-time.After(ownershipTimeout):
		h.t.Fatalf("apply never started downloading; status %+v", h.manager.GetStatus())
	}
}

func awaitCheck(t *testing.T, done <-chan checkResult) checkResult {
	t.Helper()
	select {
	case res := <-done:
		return res
	case <-time.After(ownershipTimeout):
		t.Fatal("update check did not return")
	}
	return checkResult{}
}

func awaitApply(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(ownershipTimeout):
		t.Fatal("apply did not return")
	}
	return nil
}

// syncFeeds publishes an ordered marker through the operation path and waits
// until both feeds deliver it. Both feeds are FIFO, so anything published
// before the marker has been delivered by the time it is seen.
func (h *ownershipHarness) syncFeeds() (progress, sse []UpdateStatus) {
	h.t.Helper()
	h.sentinels++
	marker := fmt.Sprintf("feed-sync-%d", h.sentinels)
	h.manager.updateStatus("sync", 0, marker)
	deadline := time.Now().Add(ownershipTimeout)
	for {
		progress = upToMarker(h.progress.snapshot(), marker)
		sse = upToMarker(h.sse.snapshot(), marker)
		if progress != nil && sse != nil {
			return progress, sse
		}
		if time.Now().After(deadline) {
			h.t.Fatalf("feeds never delivered %s", marker)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func upToMarker(feed []UpdateStatus, marker string) []UpdateStatus {
	for i, s := range feed {
		if s.Message == marker {
			return feed[:i]
		}
	}
	return nil
}

func requireStatus(t *testing.T, label string, got UpdateStatus, status, message string) {
	t.Helper()
	if got.Status != status || got.Message != message {
		t.Fatalf("%s = {%s %q}, want {%s %q}", label, got.Status, got.Message, status, message)
	}
}

// requireCurrentStatus checks both the snapshot behind /api/updates/status
// and the SSE cached status a reconnecting client receives first.
func (h *ownershipHarness) requireCurrentStatus(status, message string) {
	h.t.Helper()
	requireStatus(h.t, "GetStatus", h.manager.GetStatus(), status, message)
	cached, _ := h.manager.GetSSECachedStatus()
	requireStatus(h.t, "SSE cached status", cached, status, message)
}

// requireNoCheckStatusAfter fails when a check-origin status follows the
// first status whose message is operationStart, in either feed.
func requireNoCheckStatusAfter(t *testing.T, operationStart string, feeds map[string][]UpdateStatus) {
	t.Helper()
	for name, feed := range feeds {
		start := -1
		for i, s := range feed {
			if s.Message == operationStart {
				start = i
				break
			}
		}
		if start < 0 {
			t.Fatalf("%s feed never delivered %q: %+v", name, operationStart, feed)
		}
		for _, s := range feed[start:] {
			if isCheckOriginStatus(s) {
				t.Fatalf("%s feed delivered check status {%s %q} after %q: %+v", name, s.Status, s.Message, operationStart, feed)
			}
		}
	}
}

func requireFeedsContain(t *testing.T, status, message string, feeds map[string][]UpdateStatus) {
	t.Helper()
	for name, feed := range feeds {
		found := false
		for _, s := range feed {
			if s.Status == status && s.Message == message {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("%s feed never delivered {%s %q}: %+v", name, status, message, feed)
		}
	}
}

// requireCheckOutcome verifies the check's own result, cache and observation
// are unchanged by status ownership.
func (h *ownershipHarness) requireCheckOutcome(res checkResult, failed bool) {
	h.t.Helper()
	observation := h.manager.LastUpdateCheck()
	if failed {
		if res.err == nil || !strings.Contains(res.err.Error(), "status 400") {
			h.t.Fatalf("failed check returned info=%+v err=%v, want the release lookup error", res.info, res.err)
		}
		if observation.Outcome != UpdateCheckOutcomeNetworkError {
			h.t.Fatalf("observation outcome = %q, want %q", observation.Outcome, UpdateCheckOutcomeNetworkError)
		}
		return
	}
	if res.err != nil || res.info == nil || !res.info.Available || res.info.LatestVersion != "6.7.0" {
		h.t.Fatalf("successful check returned info=%+v err=%v, want 6.7.0 available", res.info, res.err)
	}
	if cached := h.manager.GetCachedUpdateInfo(); cached != res.info {
		h.t.Fatalf("check cache = %+v, want the returned result", cached)
	}
	if observation.Outcome != UpdateCheckOutcomeAvailable || !observation.Available {
		h.t.Fatalf("observation = %+v, want available", observation)
	}
}

const (
	applyStartMessage      = "Downloading update..."
	applySignatureFailure  = "Failed to verify update signature"
	applyDownloadFailure   = "Failed to download update"
	rollbackStartFragment  = "from backup..."
	rollbackCompletedTitle = "Rollback completed, restarting..."
)

func TestUpdateCheckStartedDuringApplyNeverPublishes(t *testing.T) {
	for _, failed := range []bool{true, false} {
		t.Run(fmt.Sprintf("check failed=%v", failed), func(t *testing.T) {
			h := newOwnershipHarness(t)

			applyDone := h.startApply()
			h.waitDownloadStarted()
			h.requireCurrentStatus("downloading", applyStartMessage)

			checkDone := h.startCheck()
			h.nextReleaseCall().reply <- failed
			res := awaitCheck(t, checkDone)
			h.requireCheckOutcome(res, failed)
			h.requireCurrentStatus("downloading", applyStartMessage)

			h.releaseDownload()
			if err := awaitApply(t, applyDone); err == nil || !strings.Contains(err.Error(), "signature") {
				t.Fatalf("apply error = %v, want signature verification failure", err)
			}
			h.requireCurrentStatus("error", applySignatureFailure)

			progress, sse := h.syncFeeds()
			feeds := map[string][]UpdateStatus{"progress": progress, "sse": sse}
			requireNoCheckStatusAfter(t, applyStartMessage, feeds)
			requireFeedsContain(t, "error", applySignatureFailure, feeds)
		})
	}
}

func TestUpdateCheckStartedBeforeApplyStopsAtAdmission(t *testing.T) {
	for _, failed := range []bool{true, false} {
		t.Run(fmt.Sprintf("check failed=%v", failed), func(t *testing.T) {
			h := newOwnershipHarness(t)

			// The check is admitted while idle and publishes "checking"...
			checkDone := h.startCheck()
			call := h.nextReleaseCall()
			h.requireCurrentStatus("checking", "Checking for updates...")

			// ...then an apply is admitted while the check is still waiting.
			applyDone := h.startApply()
			h.waitDownloadStarted()
			h.requireCurrentStatus("downloading", applyStartMessage)

			call.reply <- failed
			res := awaitCheck(t, checkDone)
			h.requireCheckOutcome(res, failed)
			h.requireCurrentStatus("downloading", applyStartMessage)

			h.releaseDownload()
			awaitApply(t, applyDone)
			h.requireCurrentStatus("error", applySignatureFailure)

			progress, sse := h.syncFeeds()
			feeds := map[string][]UpdateStatus{"progress": progress, "sse": sse}
			requireFeedsContain(t, "checking", "Checking for updates...", feeds)
			requireNoCheckStatusAfter(t, applyStartMessage, feeds)
		})
	}
}

func TestUpdateCheckReturningAfterApplyEndsKeepsTerminalStatus(t *testing.T) {
	t.Run("started before apply, succeeds after the apply failed", func(t *testing.T) {
		h := newOwnershipHarness(t)
		h.holdDownload.Store(false) // the archive is missing: the apply fails fast

		checkDone := h.startCheck()
		call := h.nextReleaseCall()

		if err := awaitApply(t, h.startApply()); err == nil || !strings.Contains(err.Error(), "download") {
			t.Fatalf("apply error = %v, want download failure", err)
		}
		h.requireCurrentStatus("error", applyDownloadFailure)

		call.reply <- false
		h.requireCheckOutcome(awaitCheck(t, checkDone), false)
		h.requireCurrentStatus("error", applyDownloadFailure)

		progress, sse := h.syncFeeds()
		requireNoCheckStatusAfter(t, applyStartMessage, map[string][]UpdateStatus{"progress": progress, "sse": sse})

		// A check admitted after the operation ended publishes normally.
		h.autoReply.Store(true)
		h.requireCheckOutcome(awaitCheck(t, h.startCheck()), false)
		h.requireCurrentStatus("available", "Update available: 6.7.0")
	})

	t.Run("started during apply, fails after the apply failed", func(t *testing.T) {
		h := newOwnershipHarness(t)

		applyDone := h.startApply()
		h.waitDownloadStarted()
		checkDone := h.startCheck()
		call := h.nextReleaseCall()

		h.releaseDownload()
		awaitApply(t, applyDone)
		h.requireCurrentStatus("error", applySignatureFailure)

		call.reply <- true
		h.requireCheckOutcome(awaitCheck(t, checkDone), true)
		h.requireCurrentStatus("error", applySignatureFailure)

		progress, sse := h.syncFeeds()
		requireNoCheckStatusAfter(t, applyStartMessage, map[string][]UpdateStatus{"progress": progress, "sse": sse})
	})
}

func TestUpdateCheckStartedBeforeRollbackKeepsRollbackStatus(t *testing.T) {
	for _, failed := range []bool{true, false} {
		t.Run(fmt.Sprintf("check failed=%v", failed), func(t *testing.T) {
			h := newOwnershipHarness(t)

			// An empty retained backup: the restore succeeds without touching
			// the running test binary.
			backupDir := filepath.Join(os.Getenv("PULSE_DATA_DIR"), "backup-20261104-235900")
			if err := os.MkdirAll(backupDir, 0o755); err != nil {
				t.Fatalf("mkdir backup: %v", err)
			}
			history, err := NewUpdateHistory(t.TempDir())
			if err != nil {
				t.Fatalf("NewUpdateHistory: %v", err)
			}
			eventID, err := history.CreateEntry(context.Background(), UpdateHistoryEntry{
				Action:      "update",
				Status:      StatusSuccess,
				VersionFrom: "6.5.0",
				VersionTo:   "6.6.0",
				BackupPath:  backupDir,
			})
			if err != nil {
				t.Fatalf("CreateEntry: %v", err)
			}
			h.manager.SetHistory(history)

			checkDone := h.startCheck()
			call := h.nextReleaseCall()

			if err := h.manager.RollbackToBackup(context.Background(), RollbackRequest{EventID: eventID}); err != nil {
				t.Fatalf("RollbackToBackup: %v", err)
			}
			h.requireCurrentStatus("completed", rollbackCompletedTitle)

			call.reply <- failed
			h.requireCheckOutcome(awaitCheck(t, checkDone), failed)
			h.requireCurrentStatus("completed", rollbackCompletedTitle)

			progress, sse := h.syncFeeds()
			feeds := map[string][]UpdateStatus{"progress": progress, "sse": sse}
			restoring := ""
			for _, s := range progress {
				if s.Status == "restoring" && strings.HasSuffix(s.Message, rollbackStartFragment) {
					restoring = s.Message
				}
			}
			if restoring == "" {
				t.Fatalf("progress feed never delivered the restoring stage: %+v", progress)
			}
			requireNoCheckStatusAfter(t, restoring, feeds)
			requireFeedsContain(t, "completed", rollbackCompletedTitle, feeds)
		})
	}
}

func TestIdleUpdateCheckStillPublishesStatus(t *testing.T) {
	h := newOwnershipHarness(t)

	checkDone := h.startCheck()
	h.nextReleaseCall().reply <- false
	h.requireCheckOutcome(awaitCheck(t, checkDone), false)
	h.requireCurrentStatus("available", "Update available: 6.7.0")

	checkDone = h.startCheck()
	h.nextReleaseCall().reply <- true
	h.requireCheckOutcome(awaitCheck(t, checkDone), true)
	h.requireCurrentStatus("error", "Failed to check for updates")

	progress, sse := h.syncFeeds()
	feeds := map[string][]UpdateStatus{"progress": progress, "sse": sse}
	requireFeedsContain(t, "checking", "Checking for updates...", feeds)
	requireFeedsContain(t, "available", "Update available: 6.7.0", feeds)
	requireFeedsContain(t, "error", "Failed to check for updates", feeds)
}

// Many checks race one apply admission. Whatever the interleaving, no check
// status may follow the apply's first status in any feed, and the snapshot
// and SSE cache must agree on the apply's terminal status.
func TestConcurrentUpdateChecksRacingApplyAdmission(t *testing.T) {
	const iterations = 25
	const checks = 8
	for i := 0; i < iterations; i++ {
		t.Run(fmt.Sprintf("iteration %d", i), func(t *testing.T) {
			raceChecksAgainstApply(t, checks)
		})
	}
}

func raceChecksAgainstApply(t *testing.T, checks int) {
	h := newOwnershipHarness(t)
	h.autoReply.Store(true)

	start := make(chan struct{})
	var wg sync.WaitGroup
	for c := 0; c < checks; c++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, _ = h.manager.CheckForUpdatesWithOptions(context.Background(), UpdateCheckOptions{Force: true})
		}()
	}
	close(start)
	applyDone := h.startApply()
	h.waitDownloadStarted()
	wg.Wait() // every check returns while the apply still holds its slot
	h.requireCurrentStatus("downloading", applyStartMessage)

	h.releaseDownload()
	awaitApply(t, applyDone)
	h.requireCurrentStatus("error", applySignatureFailure)

	progress, sse := h.syncFeeds()
	requireNoCheckStatusAfter(t, applyStartMessage, map[string][]UpdateStatus{"progress": progress, "sse": sse})
}
