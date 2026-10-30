package dockeragent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	containertypes "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/image"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/api"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
	"github.com/rs/zerolog"
)

// Regression for #2048. The Docker reads and registry responses are synthetic;
// collection, report ingestion, alert evaluation and HTTP projection are real.
// Alert delay is not bypassed; a separate alert-consumer proof ages these exact
// produced hosts to exercise pending-incident opening, retention and clearing.
// An optional JSON log lets browser proof consume the produced HTTP response.
func TestCollectContainerImageReplacementRefreshesRegistry(t *testing.T) {
	t.Setenv("PULSE_DEV", "true")
	previousMock := mock.IsMockEnabled()
	if err := mock.SetEnabled(false); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := mock.SetEnabled(previousMock); err != nil {
			t.Error(err)
		}
	})
	for _, ref := range []string{"binwiederhier/ntfy:latest", "ghcr.io/mollyim/mollysocket:1"} {
		t.Run(ref, func(t *testing.T) {
			const readerToken = "image-replacement-read-fixture.12345678"
			reader, err := config.NewAPITokenRecord(readerToken, "Image replacement fixture", []string{config.ScopeMonitoringRead})
			if err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			cfg := &config.Config{DataPath: dir, ConfigPath: dir, APITokens: []config.APITokenRecord{*reader}}
			monitor, err := monitoring.New(cfg)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(monitor.Stop)
			router := api.NewRouter(cfg, monitor, nil, nil, nil, "test")
			t.Cleanup(router.ShutdownResourceStores)
			t.Cleanup(router.ShutdownRBAC)
			t.Cleanup(router.ShutdownBackgroundWorkers)
			alertCfg := monitor.GetAlertManager().GetConfig()
			alertCfg.Enabled = true
			alertCfg.ActivationState = alerts.ActivationActive
			alertCfg.Schedule.QuietHours.Enabled = false
			alertCfg.DockerDefaults.UpdateAlertDelayHours = 0
			monitor.GetAlertManager().UpdateConfig(alertCfg)

			old := "sha256:" + strings.Repeat("1", 64)
			updated := "sha256:" + strings.Repeat("2", 64)
			current, remote, imageID := old, old, "image-before"
			registry, repo, tag := parseImageReference(ref)
			key := fmt.Sprintf("%s/%s:%s|amd64/linux/", registry, repo, tag)
			reads := 0
			checker := NewRegistryChecker(zerolog.Nop())
			checker.httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.URL.Host == "auth.docker.io" || strings.HasSuffix(req.URL.Path, "/token") {
					return newStringResponse(200, nil, `{"token":"synthetic-only"}`), nil
				}
				if req.URL.Host != registry || req.URL.Path != "/v2/"+repo+"/manifests/"+tag || req.Method != http.MethodHead {
					return nil, fmt.Errorf("unexpected registry request %s %s", req.Method, req.URL)
				}
				reads++
				return newStringResponse(200, map[string]string{"Content-Type": "application/vnd.oci.image.manifest.v1+json", "Docker-Content-Digest": remote}, ""), nil
			})}
			a := &Agent{logger: zerolog.Nop(), runtime: RuntimeDocker, registryChecker: checker, prevContainerCPU: make(map[string]cpuSample), docker: &fakeDockerClient{
				containerInspectWithRawFn: func(_ context.Context, id string, _ bool) (containertypes.InspectResponse, []byte, error) {
					inspect := baseInspect()
					inspect.ID = id
					inspect.Config.Image = ref
					return inspect, nil, nil
				},
				containerStatsOneShotFn: func(context.Context, string) (dockerStatsResponseReader, error) {
					return statsReader(t, containertypes.StatsResponse{}), nil
				},
				imageInspectWithRawFn: func(_ context.Context, id string) (image.InspectResponse, []byte, error) {
					if id != imageID {
						return image.InspectResponse{}, nil, fmt.Errorf("unexpected image %s", id)
					}
					return image.InspectResponse{RepoDigests: []string{registry + "/" + repo + "@" + current}, Architecture: "amd64", Os: "linux"}, nil, nil
				},
			}}
			// Keep a wanted pending update on another container throughout.
			control := agentsdocker.Container{ID: strings.Repeat("c", 64), Name: "wanted-update", Image: "example.test/control:1", State: "running", UpdateStatus: &agentsdocker.UpdateStatus{UpdateAvailable: true, CurrentDigest: old, LatestDigest: updated, LastChecked: time.Now()}}
			collect := func(phase string, wantUpdate bool, wantReads int, errors ...string) {
				t.Helper()
				id := strings.Repeat("a", 64)
				if imageID != "image-before" {
					id = strings.Repeat("b", 64)
				}
				c, err := a.collectContainer(context.Background(), containertypes.Summary{ID: id, Names: []string{"/updated-service"}, Image: ref, ImageID: imageID, State: "running", Status: "Up"})
				if err != nil {
					t.Fatal(err)
				}
				s := c.UpdateStatus
				wantError, wantLatest := "", remote
				if len(errors) > 0 {
					wantError, wantLatest = errors[0], ""
				}
				if s == nil || s.Error != wantError || s.UpdateAvailable != wantUpdate || s.CurrentDigest != current || s.LatestDigest != wantLatest || reads != wantReads {
					t.Fatalf("%s: current image compared with stale registry evidence: status=%+v manifest_reads=%d want=%d remote=%s", phase, s, reads, wantReads, remote)
				}
				report := agentsdocker.Report{Timestamp: time.Now().UTC(), Agent: agentsdocker.AgentInfo{ID: "image-replacement-host", IntervalSeconds: 30}, Host: agentsdocker.HostInfo{Hostname: "image-replacement-host"}, Containers: []agentsdocker.Container{c, control}}
				host, err := monitor.ApplyDockerReport(report, nil)
				if err != nil {
					t.Fatal(err)
				}
				if len(host.Containers) != 2 || host.Containers[0].UpdateStatus == nil || host.Containers[0].UpdateStatus.LatestDigest != wantLatest || host.Containers[0].UpdateStatus.UpdateAvailable != wantUpdate {
					t.Fatalf("ingestion changed update evidence: %+v", host)
				}
				active := monitor.GetAlertManager().GetActiveAlerts()
				// Zero is normalised to the existing 24h default, not an instant
				// update alert. Do not weaken production policy for a fixture.
				if len(active) != 0 {
					t.Fatalf("%s: update alerted before its delay: %+v", phase, active)
				}
				req := httptest.NewRequest(http.MethodGet, "/api/resources?type=app-container&limit=100", nil)
				req.Header.Set("X-API-Token", readerToken)
				rec := httptest.NewRecorder()
				router.Handler().ServeHTTP(rec, req)
				if rec.Code != http.StatusOK {
					t.Fatalf("resource HTTP %d: %s", rec.Code, rec.Body.String())
				}
				var body struct {
					Data []struct {
						Name   string `json:"name"`
						Docker struct {
							UpdateStatus *agentsdocker.UpdateStatus `json:"updateStatus"`
						} `json:"docker"`
					} `json:"data"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				var found bool
				for _, resource := range body.Data {
					if resource.Name != "updated-service" {
						continue
					}
					got := resource.Docker.UpdateStatus
					if got == nil || got.UpdateAvailable != wantUpdate || got.CurrentDigest != current || got.LatestDigest != wantLatest || got.Error != wantError {
						t.Fatalf("HTTP projection changed status: %+v", got)
					}
					found = true
				}
				if !found {
					t.Fatalf("resource missing from HTTP: %s", rec.Body.String())
				}
				if os.Getenv("PULSE_IMAGE_REPLACEMENT_PROOF") != "" {
					proof, err := json.Marshal(map[string]any{"ref": ref, "phase": phase, "manifestReads": reads, "resources": json.RawMessage(rec.Body.Bytes()), "alerts": active, "host": host})
					if err != nil {
						t.Fatal(err)
					}
					fmt.Printf("IMAGE_REPLACEMENT_PROOF %s\n", proof)
				}
			}
			collect("before", false, 1)
			originalExpiry := checker.getCached(key).expiresAt
			current, remote, imageID = updated, updated, "image-after"
			collect("replaced", false, 2)
			expiry := checker.getCached(key).expiresAt
			if expiry.Before(originalExpiry) || time.Until(expiry) > defaultCacheTTL {
				t.Fatal("invalid refresh deadline")
			}
			for i := 0; i < 3; i++ {
				collect("stable", false, 2)
			}
			if !checker.getCached(key).expiresAt.Equal(expiry) {
				t.Fatal("ordinary cache hits extended success TTL")
			}
			// A new registry tag still makes a real update after normal expiry.
			remote = "sha256:" + strings.Repeat("3", 64)
			checker.cache.mu.Lock()
			entry := checker.cache.entries[key]
			entry.expiresAt = time.Now().Add(-time.Second)
			checker.cache.entries[key] = entry
			checker.cache.mu.Unlock()
			collect("new-update", true, 3)
			checker.ForceCheck()
			collect("forced-check", true, 4)
			current, remote, imageID = "sha256:"+strings.Repeat("4", 64), "sha256:"+strings.Repeat("5", 64), "image-after-2"
			collect("still-outdated", true, 5)
			current, remote, imageID = "sha256:"+strings.Repeat("6", 64), "sha256:"+strings.Repeat("6", 64), "image-after-3"
			collect("refresh-pending", false, 5, "local image changed; registry refresh pending")
			checker.cache.mu.Lock()
			entry = checker.cache.entries[key]
			entry.refreshAfter = time.Now().Add(-time.Second)
			checker.cache.entries[key] = entry
			checker.cache.mu.Unlock()
			collect("refresh-complete", false, 6)

		})
	}
}
