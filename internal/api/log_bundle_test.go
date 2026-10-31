package api

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/logging"
	"github.com/rcourtman/pulse-go-rewrite/internal/updates"
)

func readSupportBundle(t *testing.T, body []byte) map[string][]byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatal(err)
	}
	out := make(map[string][]byte)
	for _, f := range zr.File {
		r, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		out[f.Name], err = io.ReadAll(r)
		r.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
	return out
}

func TestSupportBundleEnvironmentAllowlist(t *testing.T) {
	for _, tc := range []struct {
		name     string
		in, want []string
	}{
		{"unset", nil, []string{}},
		{"valid runtime settings", []string{"GOMAXPROCS=2", "GOGC=100", "GOMEMLIMIT=512MiB"}, []string{"GOMAXPROCS=2", "GOGC=100", "GOMEMLIMIT=512MiB"}},
		{"off and zero", []string{"GOGC=off", "GOMEMLIMIT=off", "GOMAXPROCS=0"}, []string{"GOGC=off", "GOMEMLIMIT=off", "GOMAXPROCS=[REDACTED]"}},
		{"no secret-name heuristic", []string{"HTTPS_PROXY=http://user:synthetic-secret@proxy.invalid", "DATABASE_URL=postgres://user:synthetic-secret@db.invalid", "TEST_SECRET=synthetic-secret", "ORDINARY_OPTION=synthetic-secret", "PLAIN_NAME", "PATH=/private/path"}, []string{}},
		{"grammar not name trust", []string{"GOMAXPROCS=synthetic-secret", "GOGC=100\nsynthetic-secret", "GOMEMLIMIT=1GiB#synthetic-secret"}, []string{"GOMAXPROCS=[REDACTED]", "GOGC=[REDACTED]", "GOMEMLIMIT=[REDACTED]"}},
		{"numeric syntax", []string{"GOGC=0", "GOMAXPROCS=+2", "GOGC=-1", "GOMEMLIMIT=0B"}, []string{"GOGC=0", "GOMAXPROCS=[REDACTED]", "GOGC=[REDACTED]", "GOMEMLIMIT=0B"}},
		{"bounded syntax", []string{"GOMAXPROCS=9223372036854775808", "GOGC=" + strings.Repeat("0", 100), "GOMEMLIMIT="}, []string{"GOMAXPROCS=[REDACTED]", "GOGC=[REDACTED]", "GOMEMLIMIT=[REDACTED]"}},
		{"memory units", []string{"GOMEMLIMIT=1B", "GOMEMLIMIT=1KiB", "GOMEMLIMIT=1MiB", "GOMEMLIMIT=1GiB", "GOMEMLIMIT=1TiB", "GOMEMLIMIT=1GB"}, []string{"GOMEMLIMIT=1B", "GOMEMLIMIT=1KiB", "GOMEMLIMIT=1MiB", "GOMEMLIMIT=1GiB", "GOMEMLIMIT=1TiB", "GOMEMLIMIT=[REDACTED]"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := supportBundleEnvironment(tc.in); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("environment projection = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestSupportBundleConfigDoesNotExportFreeFormValues(t *testing.T) {
	cfg := &config.Config{
		AuthUser: "synthetic-private-user", AuthPass: "synthetic-secret-auth",
		APIToken: "synthetic-secret-api", ProxyAuthSecret: "synthetic-secret-proxy",
		PublicURL:       "https://synthetic-secret:synthetic-secret@host.invalid/?auth=synthetic-secret",
		AgentConnectURL: "synthetic-private-url", ConfigPath: "synthetic-private-path", DataPath: "synthetic-private-path",
		AllowedOrigins: "synthetic-private-origin", DiscoverySubnet: "synthetic-private-subnet",
		ProxyAuthLogoutURL: "synthetic-secret", TLSKeyFile: "synthetic-private-path",
		LogLevel: "info", LogFormat: "json", UpdateChannel: "rc", FrontendPort: 7655,
		PVEPollingInterval: 10 * time.Second, EnableBackupPolling: true, ConnectionTimeout: time.Minute,
		PVEInstances: []config.PVEInstance{{Name: "synthetic-private-name", Host: "synthetic-secret", Password: "synthetic-secret", TokenValue: "synthetic-secret", Fingerprint: "synthetic-private-pin", VerifySSL: true, MonitorVMs: true, MonitorBackups: true, ClusterEndpoints: []config.ClusterEndpoint{{PulseError: "synthetic-secret"}}}},
		PBSInstances: []config.PBSInstance{{Name: "synthetic-private-name", TokenValue: "synthetic-secret", Disabled: true, MonitorDatastores: true}},
		PMGInstances: []config.PMGInstance{{Name: "synthetic-private-name", Password: "synthetic-secret", MonitorQueues: true}},
	}
	before, _ := json.Marshal(cfg)
	summary := newSupportBundleConfig(cfg)
	body, err := json.Marshal(summary)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "synthetic-") {
		t.Fatal("configuration projection exported a synthetic private value")
	}
	if summary.LogLevel != "info" || summary.LogFormat != "json" || summary.UpdateChannel != "rc" || summary.FrontendPort != 7655 || summary.PVEPollingInterval != 10*time.Second || !summary.EnableBackupPolling || summary.ConnectionTimeout != time.Minute {
		t.Fatal("diagnostic scalars lost")
	}
	if len(summary.PVEInstances) != 1 || !summary.PVEInstances[0].VerifySSL || !summary.PVEInstances[0].PasswordConfigured || !summary.PVEInstances[0].TokenConfigured || !summary.PVEInstances[0].FingerprintConfigured || !reflect.DeepEqual(summary.PVEInstances[0].MonitoredSurfaces, []string{"vms", "backups"}) {
		t.Fatal("PVE diagnostic summary lost")
	}
	if len(summary.PBSInstances) != 1 || !summary.PBSInstances[0].Disabled || !summary.PBSInstances[0].TokenConfigured || !reflect.DeepEqual(summary.PBSInstances[0].MonitoredSurfaces, []string{"datastores"}) {
		t.Fatal("PBS diagnostic summary lost")
	}
	if len(summary.PMGInstances) != 1 || !summary.PMGInstances[0].PasswordConfigured || !reflect.DeepEqual(summary.PMGInstances[0].MonitoredSurfaces, []string{"queues"}) {
		t.Fatal("PMG diagnostic summary lost")
	}
	// A downloaded summary must be detached from live connection slices.
	summary.PVEInstances[0].MonitoredSurfaces[0] = "changed"
	after, _ := json.Marshal(cfg)
	if !bytes.Equal(before, after) {
		t.Fatal("summary mutated live configuration")
	}
	cfg.LogLevel, cfg.LogFormat, cfg.UpdateChannel = "synthetic-secret", "synthetic-secret", "synthetic-secret"
	body, _ = json.Marshal(newSupportBundleConfig(cfg))
	if strings.Contains(string(body), "synthetic-") || bytes.Count(body, []byte("[REDACTED]")) != 3 {
		t.Fatal("enum-shaped config fields trusted arbitrary contents")
	}
}

func TestSupportBundleKeepsLogSourcesAndRuntimeIdentity(t *testing.T) {
	oldVersion := updates.BuildVersion
	updates.BuildVersion = "6.7.0-test"
	t.Cleanup(func() { updates.BuildVersion = oldVersion })
	// Logs remain private, unmodified diagnostic evidence, not automatically
	// anonymised text. Check file, missing-file and unconfigured-file paths.
	marker := "synthetic-private-log-marker"
	path := filepathWithTemp(t, "pulse.log")
	if err := os.WriteFile(path, []byte(marker), 0600); err != nil {
		t.Fatal(err)
	}
	logging.GetBroadcaster().Write([]byte(marker))
	for _, tc := range []struct{ name, path, entry string }{
		{"file", path, "pulse.log"}, {"missing file", path + "-absent", "pulse-tail.log"}, {"buffer", "", "pulse-tail.log"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := NewLogHandlers(&config.Config{LogFile: tc.path}, nil)
			rr := httptest.NewRecorder()
			h.HandleDownloadBundle(rr, httptest.NewRequest(http.MethodGet, "/api/logs/download", nil))
			entries := readSupportBundle(t, rr.Body.Bytes())
			if len(entries) != 2 || !bytes.Contains(entries[tc.entry], []byte(marker)) {
				t.Fatal("original log evidence missing")
			}
			var info struct{ Version, GoVersion, OS string }
			if err := json.Unmarshal(entries["system-info.json"], &info); err != nil {
				t.Fatal(err)
			}
			if info.Version != "6.7.0-test" || info.GoVersion != runtime.Version() || info.OS != runtime.GOOS {
				t.Fatal("diagnostic runtime identity is invented")
			}
		})
	}
	updates.BuildVersion = ""
	rr := httptest.NewRecorder()
	NewLogHandlers(&config.Config{LogFile: path}, nil).HandleDownloadBundle(rr, httptest.NewRequest(http.MethodGet, "/api/logs/download", nil))
	if !bytes.Contains(readSupportBundle(t, rr.Body.Bytes())["system-info.json"], []byte(`"Version": "unknown"`)) {
		t.Fatal("missing build version must be explicit")
	}
}

func TestSupportBundleSharesConfigOwnershipWithLogLevelUpdates(t *testing.T) {
	original := logging.GetGlobalLevel()
	t.Cleanup(func() { logging.SetGlobalLevel(original) })
	cfg := &config.Config{LogLevel: "info"}
	h := NewLogHandlers(cfg, nil)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			h.HandleSetLevel(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/logs/level", strings.NewReader(`{"level":"info"}`)))
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			h.HandleDownloadBundle(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/logs/download", nil))
		}
	}()
	wg.Wait()
}
