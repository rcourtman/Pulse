package api

import (
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// Cover the other production writer of the same channel: normal settings
// saves must agree with the background updater's reload/read boundary too.
func TestSystemSettingsChannelWriterSharesConfigBoundary(t *testing.T) {
	root := t.TempDir()
	cfg := &config.Config{DataPath: root, ConfigPath: root, UpdateChannel: "stable"}
	token, err := config.NewAPITokenRecord("synthetic-channel-settings-token", "Settings", []string{config.ScopeSettingsWrite})
	if err != nil {
		t.Fatal(err)
	}
	cfg.APITokens = []config.APITokenRecord{*token}
	persistence := config.NewConfigPersistence(root)
	h := newTestSystemSettingsHandler(cfg, persistence, &mockMonitor{}, func() {}, func() error { return nil })
	stop, done := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-stop:
				return
			default:
			}
			config.Mu.RLock()
			channel := cfg.UpdateChannel
			config.Mu.RUnlock()
			if channel != "stable" && channel != "rc" {
				t.Errorf("incoherent channel %q", channel)
				return
			}
			runtime.Gosched()
		}
	}()
	defer func() { close(stop); <-done }()
	for i := range 32 {
		channel := "stable"
		if i%2 != 0 {
			channel = "rc"
		}
		req := httptest.NewRequest(http.MethodPost, "/api/system-settings", strings.NewReader(`{"updateChannel":"`+channel+`","autoUpdateEnabled":true}`))
		req.Header.Set("X-API-Token", "synthetic-channel-settings-token")
		rec := httptest.NewRecorder()
		h.HandleUpdateSystemSettings(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("channel save: %d %s", rec.Code, rec.Body.String())
		}
		loaded, err := persistence.LoadSystemSettings()
		if err != nil || loaded.UpdateChannel != channel || (channel == "rc" && loaded.AutoUpdateEnabled) {
			t.Fatal("channel precedence or RC auto-update policy changed")
		}
	}
}
