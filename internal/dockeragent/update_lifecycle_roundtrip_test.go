package dockeragent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	containertypes "github.com/moby/moby/api/types/container"
	imagetypes "github.com/moby/moby/api/types/image"
	systemtypes "github.com/moby/moby/api/types/system"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/eventlog"
	"github.com/rcourtman/pulse-go-rewrite/internal/api"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/hostmetrics"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
	"github.com/rs/zerolog"
)

// Join the real collector, compressed authenticated HTTPS report, report-order
// admission and alert lifecycle. The daemon and registry cache are synthetic;
// no workload or registry is contacted. Only the saved occurrence's timestamps
// are aged, not the clock, production thresholds or cached registry answer.
// This closes the boundary gap between the collector and manager unit tests for
// #2353; it does not identify the reporter's installed cause or simulate a day.
func TestDockerUpdateCollectorAdmissionPreservesOccurrence(t *testing.T) {
	t.Setenv("PULSE_DEV", "true")
	swap(t, &hostmetricsCollect, func(context.Context, []string) (hostmetrics.Snapshot, error) {
		return hostmetrics.Snapshot{}, nil
	})
	for _, delay := range []int{24, 48} {
		for _, acknowledged := range []bool{false, true} {
			t.Run(fmt.Sprintf("delay=%d/ack=%t", delay, acknowledged), func(t *testing.T) {
				const token = "docker-update-report-fixture.12345678"
				const hostID, containerID = "update-host", "update-container"
				const oldImage, newImage = "sha256:old-image", "sha256:new-image"
				registry, repository, tag := parseImageReference("mongo:7")
				cacheKey := registry + "/" + repository + ":" + tag + "|amd64/linux/"
				var registryCalls atomic.Int32
				newChecker := func() *RegistryChecker {
					checker := NewRegistryChecker(zerolog.Nop())
					checker.httpClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
						registryCalls.Add(1)
						return nil, errors.New("unexpected registry request in offline fixture")
					})}
					checker.cacheDigestResult(cacheKey, "sha256:new", "sha256:new")
					return checker
				}
				imageID, localDigest, inspectFails := oldImage, "sha256:old", false
				agent := &Agent{
					cfg:     Config{AgentID: hostID, IncludeContainers: true, Interval: 30 * time.Second},
					runtime: RuntimeDocker, logger: zerolog.Nop(), agentVersion: "test", reportStreamID: "update-stream",
					registryChecker: newChecker(),
					docker: &fakeDockerClient{
						infoFunc: func(context.Context) (systemtypes.Info, error) {
							return systemtypes.Info{Name: hostID, NCPU: 4, ServerVersion: "28.0.0"}, nil
						},
						containerListFunc: func(context.Context, dockerContainerListOptions) ([]containertypes.Summary, error) {
							return []containertypes.Summary{{ID: containerID, Names: []string{"/database"}, Image: "mongo:7", ImageID: imageID, State: "running", Status: "Up"}}, nil
						},
						containerInspectWithRawFn: func(context.Context, string, bool) (containertypes.InspectResponse, []byte, error) {
							inspect := baseInspect()
							inspect.Config.Image = "mongo:7"
							inspect.Image = imageID
							return inspect, nil, nil
						},
						containerStatsOneShotFn: func(context.Context, string) (dockerStatsResponseReader, error) {
							return dockerStatsResponseReader{Body: io.NopCloser(strings.NewReader(`{}`))}, nil
						},
						imageInspectWithRawFn: func(context.Context, string) (imagetypes.InspectResponse, []byte, error) {
							if inspectFails {
								return imagetypes.InspectResponse{}, nil, context.DeadlineExceeded
							}
							return imagetypes.InspectResponse{RepoDigests: []string{"docker.io/library/mongo@" + localDigest}, Architecture: "amd64", Os: "linux"}, nil, nil
						},
					},
				}
				collect := func() agentsdocker.Report {
					t.Helper()
					report, err := agent.buildReport(context.Background())
					if err != nil || len(report.Containers) != 1 {
						t.Fatalf("collect report: %v, containers=%d", err, len(report.Containers))
					}
					return report
				}
				connect := func(server *httptest.Server) {
					agent.targets = []TargetConfig{{URL: server.URL, Token: token}}
					agent.httpClients = map[bool]*http.Client{false: server.Client()}
				}
				send := func(report agentsdocker.Report) {
					t.Helper()
					if err := agent.sendReport(context.Background(), report); err != nil {
						t.Fatalf("send compressed HTTPS report: %v", err)
					}
				}

				// The existing manager tests prove initial firing/delivery. Begin
				// this boundary proof at a synthetic aged recovery checkpoint,
				// derived from the collector's resource rather than changing the
				// supported 24/48-hour delay or waiting for wall-clock ageing.
				firstReport := collect()
				resourceID := alerts.DockerContainerResourceID(firstReport.Agent.ID, firstReport.Containers[0].ID, "")
				lastObservation := time.Now().Add(-25 * time.Hour)
				lastNotified := lastObservation
				seed := alerts.Alert{
					ID: "docker-container-update-" + resourceID, Type: "docker-container-update", Level: alerts.AlertLevelWarning,
					ResourceID: resourceID, ResourceName: "database", Node: hostID, Instance: hostID,
					StartTime: time.Now().Add(-time.Duration(delay+2) * time.Hour), LastSeen: lastObservation,
					LastNotified: &lastNotified, Acknowledged: acknowledged,
					Metadata: map[string]interface{}{"hostId": hostID, "hostAgentId": hostID, "containerId": containerID, "containerName": "database", "image": "mongo:7"},
				}
				if acknowledged {
					seed.AckTime, seed.AckUser = &lastObservation, "fixture-operator"
				}
				monitor, server, _ := newDockerUpdateRoundTripServer(t, token, &seed, delay)
				connect(server)
				manager := monitor.GetAlertManager()
				loaded := requireDockerUpdateOccurrence(t, manager)
				if loaded.ResourceID != resourceID || !loaded.StartTime.Equal(seed.StartTime) || !loaded.LastSeen.Equal(seed.LastSeen) || loaded.LastNotified == nil || !loaded.LastNotified.Equal(lastNotified) {
					t.Fatalf("saved occurrence was not restored: %+v", loaded)
				}
				seed = loaded // Restoration may canonicalise the legacy checkpoint ID.
				var firings, resolutions, deliveries atomic.Int32
				manager.SetAlertCallback(func(*alerts.Alert) { deliveries.Add(1) })
				manager.SubscribeLifecycleCallback(func(event alerts.LifecycleEvent) {
					if event.Alert == nil || event.Alert.Type != "docker-container-update" {
						return
					}
					switch event.Type {
					case eventlog.TypeFired, eventlog.TypeRefired:
						firings.Add(1)
					case eventlog.TypeResolved:
						resolutions.Add(1)
					}
				})
				admitAndKeep := func(name string, report agentsdocker.Report) {
					t.Helper()
					before := time.Now()
					send(report)
					host, ok := monitor.GetDockerHost(hostID)
					if !ok || len(host.Containers) != 1 || host.Containers[0].ID != containerID || host.Containers[0].ImageDigest != imageID {
						t.Fatalf("%s: admitted identity changed: %+v", name, host)
					}
					wire, admitted := report.Containers[0].UpdateStatus, host.Containers[0].UpdateStatus
					if (wire == nil) != (admitted == nil) || (wire != nil && (wire.UpdateAvailable != admitted.UpdateAvailable || wire.CurrentDigest != admitted.CurrentDigest || wire.LatestDigest != admitted.LatestDigest || wire.Error != admitted.Error || !wire.LastChecked.Equal(admitted.LastChecked))) {
						t.Fatalf("%s: ingestion changed comparison evidence: wire=%+v admitted=%+v", name, wire, admitted)
					}
					manager.Cleanup(24 * time.Hour)
					got := requireDockerUpdateOccurrence(t, manager)
					if got.ID != seed.ID || got.ResourceID != seed.ResourceID || !got.StartTime.Equal(seed.StartTime) || got.LastSeen.Before(before) || got.Acknowledged != acknowledged || got.AckUser != seed.AckUser || got.LastNotified == nil || !got.LastNotified.Equal(*seed.LastNotified) {
						t.Fatalf("%s: report/retention changed the occurrence: %+v", name, got)
					}
					if firings.Load() != 0 || resolutions.Load() != 0 || deliveries.Load() != 0 {
						t.Fatalf("%s: invented lifecycle/delivery: %d/%d/%d", name, firings.Load(), resolutions.Load(), deliveries.Load())
					}
					t.Logf("%s: collector/wire/admission evidence, original age/identity/ack/notification and retention agree", name)
				}
				admitAndKeep("cached positive after server restart", collect())
				inspectFails = true
				fallback := collect()
				if status := fallback.Containers[0].UpdateStatus; status == nil || !status.UpdateAvailable || status.CurrentDigest != "sha256:old" || status.Error != "" {
					t.Fatalf("failed image inspect lost immutable-image fallback: %+v", status)
				}
				admitAndKeep("failed inspect with remembered image", fallback)
				agent.registryChecker = newChecker() // A cold module has no local-image memo.
				unknown := collect()
				if unknown.Containers[0].UpdateStatus != nil {
					t.Fatalf("cold failed inspect invented comparison: %+v", unknown.Containers[0].UpdateStatus)
				}
				admitAndKeep("failed inspect without remembered image", unknown)
				inspectFails = false
				agent.registryChecker.cacheError(cacheKey, "fixture registry unavailable")
				failed := collect()
				if status := failed.Containers[0].UpdateStatus; status == nil || status.Error == "" || status.CurrentDigest != "sha256:old" {
					t.Fatalf("failed registry observation was not reported: %+v", status)
				}
				admitAndKeep("registry error", failed)

				// Older agents can still send this invalid comparison. Admit it
				// through the same wire route, but do not call it collector output.
				legacy := collect()
				legacy.Containers[0].UpdateStatus = &agentsdocker.UpdateStatus{LatestDigest: "sha256:new", LastChecked: time.Now()}
				admitAndKeep("legacy no-current-digest wire control", legacy)
				agent.registryChecker.cacheDigestResult(cacheKey, "sha256:new", "sha256:new")
				admitAndKeep("unchanged cached positive resumes", collect())

				// An actual matching local/registry comparison is affirmative
				// recovery. A stale earlier positive must not undo it at admission.
				imageID, localDigest = newImage, "sha256:new"
				recovered := collect()
				if status := recovered.Containers[0].UpdateStatus; status == nil || status.UpdateAvailable || status.Error != "" || status.CurrentDigest != "sha256:new" || status.LatestDigest != "sha256:new" {
					t.Fatalf("affirmative comparison missing: %+v", status)
				}
				send(recovered)
				manager.Cleanup(24 * time.Hour)
				if got := manager.GetActiveAlerts(); len(got) != 0 || resolutions.Load() != 1 {
					t.Fatalf("affirmative recovery did not resolve exactly once: active=%+v resolutions=%d", got, resolutions.Load())
				}
				send(fallback) // Lower report sequence: no alert evaluation or state rollback.
				host, ok := monitor.GetDockerHost(hostID)
				if !ok || host.Containers[0].ImageDigest != newImage || host.Containers[0].UpdateStatus.UpdateAvailable || len(manager.GetActiveAlerts()) != 0 || resolutions.Load() != 1 {
					t.Fatal("stale positive report undid admitted recovery")
				}
				imageID, localDigest = oldImage, "sha256:old"
				send(collect())
				if len(manager.GetActiveAlerts()) != 0 || firings.Load() != 0 || deliveries.Load() != 0 {
					t.Fatal("a new pending update inherited the retired age or delivered again")
				}
				if registryCalls.Load() != 0 {
					t.Fatalf("fixture attempted %d registry calls", registryCalls.Load())
				}
				t.Log("affirmative producer recovery resolves once; stale admission cannot roll it back; a new update waits the configured delay; zero registry requests")
			})
		}
	}
}

func requireDockerUpdateOccurrence(t *testing.T, manager *alerts.Manager) alerts.Alert {
	t.Helper()
	active := manager.GetActiveAlerts()
	if len(active) != 1 || active[0].Type != "docker-container-update" {
		t.Fatalf("want exactly one Docker update occurrence, got %+v", active)
	}
	return active[0]
}

func newDockerUpdateRoundTripServer(t *testing.T, token string, seed *alerts.Alert, delay int) (*monitoring.Monitor, *httptest.Server, func()) {
	t.Helper()
	dir := t.TempDir()
	if seed != nil {
		payload, err := json.Marshal([]alerts.Alert{*seed})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(dir, "alerts"), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "alerts", "active-alerts.json"), payload, 0600); err != nil {
			t.Fatal(err)
		}
	}
	reporter, err := config.NewAPITokenRecord(token, "Docker update fixture", []string{config.ScopeDockerReport})
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{DataPath: dir, ConfigPath: dir, APITokens: []config.APITokenRecord{*reporter}}
	monitor, err := monitoring.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	manager := monitor.GetAlertManager()
	alertConfig := manager.GetConfig()
	alertConfig.Enabled = true
	alertConfig.ActivationState = alerts.ActivationActive
	alertConfig.Schedule.QuietHours.Enabled = false
	alertConfig.Schedule.Cooldown = 0 // First notification only, not a resend test.
	alertConfig.FlappingEnabled = false
	alertConfig.AutoAcknowledgeAfterHours = 0
	alertConfig.MaxAlertAgeDays, alertConfig.MaxAcknowledgedAgeDays = 1, 1
	alertConfig.DockerDefaults.UpdateAlertDelayHours = delay
	manager.UpdateConfig(alertConfig)
	router := api.NewRouter(cfg, monitor, nil, nil, nil, "test")
	server := httptest.NewTLSServer(router.Handler())
	var once sync.Once
	stop := func() {
		once.Do(func() {
			server.Close()
			router.ShutdownBackgroundWorkers()
			router.ShutdownResourceStores()
			router.ShutdownRBAC()
			monitor.Stop()
		})
	}
	t.Cleanup(stop)
	return monitor, server, stop
}
