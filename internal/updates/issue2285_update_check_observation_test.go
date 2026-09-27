package updates

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// issue2285Server serves a fixed status and body for the release-list path.
func issue2285Server(t *testing.T, status int, body string) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	t.Setenv("PULSE_UPDATE_SERVER", server.URL)
}

func issue2285Releases(t *testing.T, releases ...ReleaseInfo) string {
	t.Helper()
	body, err := json.Marshal(releases)
	if err != nil {
		t.Fatalf("marshal releases: %v", err)
	}
	return string(body)
}

func TestIssue2285LastUpdateCheckRecordsEffectiveChannelOutcome(t *testing.T) {
	setRetrySettingsForTest(t, 1, time.Millisecond, time.Millisecond)
	stable := ReleaseInfo{TagName: "v6.4.5", PublishedAt: time.Date(2026, 9, 30, 8, 0, 0, 0, time.UTC)}
	preview := ReleaseInfo{TagName: "v6.4.6-rc.1", Prerelease: true, PublishedAt: time.Date(2026, 10, 2, 8, 0, 0, 0, time.UTC)}

	for _, tc := range []struct {
		name          string
		current       string
		channel       string
		status        int
		body          string
		wantOutcome   string
		wantAvailable bool
		wantErr       bool
	}{
		{name: "update offered", current: "6.4.1", channel: "stable", status: http.StatusOK, body: issue2285Releases(t, stable, preview), wantOutcome: UpdateCheckOutcomeAvailable, wantAvailable: true},
		{name: "already current", current: "6.4.5", channel: "stable", status: http.StatusOK, body: issue2285Releases(t, stable, preview), wantOutcome: UpdateCheckOutcomeUpToDate},
		{name: "preview channel offered newer prerelease", current: "6.4.5", channel: "rc", status: http.StatusOK, body: issue2285Releases(t, stable, preview), wantOutcome: UpdateCheckOutcomeAvailable, wantAvailable: true},
		{name: "no stable release for channel", current: "6.4.5-rc.3", channel: "stable", status: http.StatusOK, body: issue2285Releases(t, preview), wantOutcome: UpdateCheckOutcomeNoRelease},
		{name: "malformed metadata", current: "6.4.1", channel: "stable", status: http.StatusOK, body: `{"message":"not a list"}`, wantOutcome: UpdateCheckOutcomeMetadataError, wantErr: true},
		{name: "server error", current: "6.4.1", channel: "stable", status: http.StatusBadGateway, body: `bad gateway`, wantOutcome: UpdateCheckOutcomeNetworkError, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			withBuildVersion(t, tc.current)
			issue2285Server(t, tc.status, tc.body)
			manager := NewManager(&config.Config{UpdateChannel: tc.channel})

			if got := manager.LastUpdateCheck(); got.Outcome != UpdateCheckOutcomeNotChecked || got.Channel != tc.channel || got.Available {
				t.Fatalf("before any check LastUpdateCheck = %+v, want not_checked on %s", got, tc.channel)
			}

			_, err := manager.CheckForUpdates(context.Background())
			if (err != nil) != tc.wantErr {
				t.Fatalf("CheckForUpdates error = %v, wantErr %v", err, tc.wantErr)
			}
			got := manager.LastUpdateCheck()
			if got.Outcome != tc.wantOutcome || got.Available != tc.wantAvailable || got.Channel != tc.channel || got.CheckedAt.IsZero() {
				t.Fatalf("LastUpdateCheck = %+v, want outcome %s available %v on %s", got, tc.wantOutcome, tc.wantAvailable, tc.channel)
			}
		})
	}
}

func TestIssue2285PreviewOfOtherChannelDoesNotReplaceObservation(t *testing.T) {
	setRetrySettingsForTest(t, 1, time.Millisecond, time.Millisecond)
	withBuildVersion(t, "6.4.5")
	issue2285Server(t, http.StatusOK, issue2285Releases(t,
		ReleaseInfo{TagName: "v6.4.5"},
		ReleaseInfo{TagName: "v6.4.6-rc.1", Prerelease: true},
	))
	manager := NewManager(&config.Config{UpdateChannel: "stable"})

	if _, err := manager.CheckForUpdatesWithChannel(context.Background(), "stable"); err != nil {
		t.Fatalf("explicit stable check: %v", err)
	}
	if got := manager.LastUpdateCheck(); got.Outcome != UpdateCheckOutcomeUpToDate || got.Available {
		t.Fatalf("explicit check on the saved channel = %+v, want up_to_date", got)
	}

	// The settings UI can preview the preview channel before saving it. That
	// offer is not what this install is being offered, so it must not leak
	// into the observation telemetry reports.
	if _, err := manager.CheckForUpdatesWithChannel(context.Background(), "rc"); err != nil {
		t.Fatalf("preview-channel check: %v", err)
	}
	if got := manager.LastUpdateCheck(); got.Outcome != UpdateCheckOutcomeUpToDate || got.Available || got.Channel != "stable" {
		t.Fatalf("after previewing rc LastUpdateCheck = %+v, want stable up_to_date unchanged", got)
	}
}

func TestIssue2285SourceBuildReportsSkipped(t *testing.T) {
	withBuildVersion(t, "6.4.0")
	markerPath := "BUILD_FROM_SOURCE"
	if err := os.WriteFile(markerPath, []byte("1"), 0o644); err != nil {
		t.Fatalf("write %s: %v", markerPath, err)
	}
	t.Cleanup(func() { _ = os.Remove(markerPath) })

	manager := NewManager(&config.Config{UpdateChannel: "stable"})
	if _, err := manager.CheckForUpdates(context.Background()); err != nil {
		t.Fatalf("CheckForUpdates: %v", err)
	}
	if got := manager.LastUpdateCheck(); got.Outcome != UpdateCheckOutcomeSkipped || got.Available {
		t.Fatalf("source build LastUpdateCheck = %+v, want skipped", got)
	}
}

func TestIssue2285UpdateCheckErrorOutcomeClassification(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want string
	}{
		{withUpdateCheckOutcome(UpdateCheckOutcomeNetworkError, context.DeadlineExceeded), UpdateCheckOutcomeNetworkError},
		{withUpdateCheckOutcome(UpdateCheckOutcomeMetadataError, errGitHubRateLimited), UpdateCheckOutcomeRateLimited},
		{errGitHubRateLimited, UpdateCheckOutcomeRateLimited},
		{context.Canceled, UpdateCheckOutcomeError},
	} {
		if got := updateCheckErrorOutcome(tc.err); got != tc.want {
			t.Fatalf("updateCheckErrorOutcome(%v) = %q, want %q", tc.err, got, tc.want)
		}
	}
	if withUpdateCheckOutcome(UpdateCheckOutcomeNetworkError, nil) != nil {
		t.Fatal("withUpdateCheckOutcome(nil) must stay nil")
	}
	tagged := withUpdateCheckOutcome(UpdateCheckOutcomeNetworkError, context.DeadlineExceeded)
	if tagged.Error() != context.DeadlineExceeded.Error() {
		t.Fatalf("tagged error message = %q, want the wrapped message unchanged", tagged.Error())
	}
}
