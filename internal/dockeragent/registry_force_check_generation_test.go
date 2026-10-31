package dockeragent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	containertypes "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/image"
	"github.com/rcourtman/pulse-go-rewrite/internal/api"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
	"github.com/rs/zerolog"
)

// These tests use only identifiers that exist before the generation check, so
// they also run as behavioural controls against the unchanged checker.

const forceCheckRef = "registry.example.test/team/app:stable"

// forceCheckRegistry answers manifest HEADs from a digest or failure status the
// test sets, counting requests. A parked request waits, after reading that
// state, until the test resumes it.
type forceCheckRegistry struct {
	t        *testing.T
	mu       sync.Mutex
	digest   string
	status   int
	requests int
	park     *forceCheckPark
}

type forceCheckPark struct {
	reached chan struct{}
	resume  chan struct{}
}

func (f *forceCheckRegistry) set(digest string, status int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.digest, f.status = digest, status
}

func (f *forceCheckRegistry) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.requests
}

func (f *forceCheckRegistry) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Method != http.MethodHead || req.URL.Host != "registry.example.test" {
		f.t.Errorf("unexpected registry request %s %s", req.Method, req.URL)
		return newStringResponse(http.StatusNotFound, nil, ""), nil
	}
	f.mu.Lock()
	f.requests++
	digest, status, park := f.digest, f.status, f.park
	f.park = nil
	f.mu.Unlock()
	if park != nil {
		close(park.reached)
		<-park.resume
	}
	if status != http.StatusOK {
		return newStringResponse(status, nil, ""), nil
	}
	return newStringResponse(http.StatusOK, map[string]string{
		"Content-Type":          "application/vnd.docker.distribution.manifest.v2+json",
		"Docker-Content-Digest": digest,
	}, ""), nil
}

func newForceCheckChecker(t *testing.T, digest string, status int) (*RegistryChecker, *forceCheckRegistry) {
	t.Helper()
	reg := &forceCheckRegistry{t: t, digest: digest, status: status}
	checker := NewRegistryChecker(zerolog.Nop())
	checker.httpClient = &http.Client{Transport: reg}
	return checker, reg
}

func forceCheckLookup(checker *RegistryChecker, local string) *ImageUpdateResult {
	return checker.CheckImageUpdate(context.Background(), forceCheckRef, local, "amd64", "linux", "")
}

// parkLookup starts a check whose registry request parks once it has read the
// registry state, and returns a function that resumes it and waits for the
// check's result.
func parkLookup(t *testing.T, reg *forceCheckRegistry, check func() *ImageUpdateResult) func() *ImageUpdateResult {
	t.Helper()
	park := &forceCheckPark{reached: make(chan struct{}), resume: make(chan struct{})}
	reg.mu.Lock()
	reg.park = park
	reg.mu.Unlock()
	var once sync.Once
	resume := func() { once.Do(func() { close(park.resume) }) }
	t.Cleanup(resume)

	done := make(chan *ImageUpdateResult, 1)
	go func() { done <- check() }()
	select {
	case <-park.reached:
	case <-time.After(5 * time.Second):
		t.Fatal("parked lookup never reached the registry")
	}
	return func() *ImageUpdateResult {
		t.Helper()
		resume()
		select {
		case got := <-done:
			return got
		case <-time.After(5 * time.Second):
			t.Fatal("parked lookup never finished")
			return nil
		}
	}
}

// A lookup begun before ForceCheck must not overwrite the answer or backoff of
// a lookup made after it, and reports no status of its own.
func TestForceCheckKeepsPostClearOutcome(t *testing.T) {
	for _, tt := range []struct {
		name       string
		preDigest  string
		preStatus  int
		postDigest string
		postStatus int
		wantErr    string
		wantLatest string
	}{
		{"pre-clear answer, post-clear rate limit", "sha256:aaaa", http.StatusOK, "", http.StatusTooManyRequests, "rate limited", ""},
		{"pre-clear error, post-clear answer", "", http.StatusBadGateway, "sha256:bbbb", http.StatusOK, "", "sha256:bbbb"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			checker, reg := newForceCheckChecker(t, tt.preDigest, tt.preStatus)
			resume := parkLookup(t, reg, func() *ImageUpdateResult { return forceCheckLookup(checker, "sha256:aaaa") })

			checker.ForceCheck()
			reg.set(tt.postDigest, tt.postStatus)
			if got := forceCheckLookup(checker, "sha256:aaaa"); got == nil || got.Error != tt.wantErr || got.LatestDigest != tt.wantLatest {
				t.Fatalf("post-clear lookup = %+v, want error %q latest %q", got, tt.wantErr, tt.wantLatest)
			}

			if got := resume(); got != nil {
				t.Fatalf("lookup begun before ForceCheck = %+v, want no status", got)
			}
			if got := forceCheckLookup(checker, "sha256:aaaa"); got == nil || got.Error != tt.wantErr || got.LatestDigest != tt.wantLatest || reg.count() != 2 {
				t.Fatalf("after the stale lookup finished = %+v after %d requests, want the post-clear outcome to stand", got, reg.count())
			}
		})
	}
}

// After ForceCheck the next ordinary check asks the registry, whatever the
// lookup begun before the clear found, including an early refresh holding the
// reservation for a replaced local image.
func TestForceCheckNextCheckFetchesFreshEvidence(t *testing.T) {
	for _, tt := range []struct {
		name      string
		seed      bool
		preDigest string
		preStatus int
		local     string
	}{
		{"pre-clear answer", false, "sha256:aaaa", http.StatusOK, "sha256:aaaa"},
		{"pre-clear rate limit", false, "", http.StatusTooManyRequests, "sha256:aaaa"},
		{"pre-clear early refresh", true, "sha256:bbbb", http.StatusOK, "sha256:bbbb"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			checker, reg := newForceCheckChecker(t, "sha256:aaaa", http.StatusOK)
			seeded := 0
			if tt.seed {
				// A container on aaaa is checked, then replaced by one on bbbb,
				// whose check takes the one early-refresh reservation.
				if got := forceCheckLookup(checker, "sha256:aaaa"); got == nil || got.UpdateAvailable {
					t.Fatalf("seed check = %+v", got)
				}
				seeded = 1
			}
			reg.set(tt.preDigest, tt.preStatus)
			resume := parkLookup(t, reg, func() *ImageUpdateResult { return forceCheckLookup(checker, tt.local) })

			checker.ForceCheck()
			if got := resume(); got != nil {
				t.Fatalf("lookup begun before ForceCheck = %+v, want no status", got)
			}

			// The same local image again: a stale entry would answer it from
			// the cache without asking the registry.
			reg.set("sha256:cccc", http.StatusOK)
			got := forceCheckLookup(checker, tt.local)
			if got == nil || got.Error != "" || !got.UpdateAvailable || got.LatestDigest != "sha256:cccc" {
				t.Fatalf("first check after ForceCheck = %+v, want fresh evidence sha256:cccc", got)
			}
			if want := seeded + 2; reg.count() != want {
				t.Fatalf("requests = %d, want %d: the check after ForceCheck must ask the registry", reg.count(), want)
			}
		})
	}
}

// The real collector must publish unknown after a superseded lookup, rather
// than resurrecting the prior update through ingestion or the resource API.
// Docker and registry transports are synthetic; the consuming paths are real.
func TestCollectForceCheckSupersededLookupRemainsUnknown(t *testing.T) {
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
	const readerToken = "force-check-read-fixture.12345678"
	reader, err := config.NewAPITokenRecord(readerToken, "ForceCheck fixture", []string{config.ScopeMonitoringRead})
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

	current := "sha256:" + strings.Repeat("a", 64)
	latest := "sha256:" + strings.Repeat("b", 64)
	id := strings.Repeat("a", 64)
	checker, reg := newForceCheckChecker(t, latest, http.StatusOK)
	a := &Agent{logger: zerolog.Nop(), runtime: RuntimeDocker, registryChecker: checker, prevContainerCPU: make(map[string]cpuSample), docker: &fakeDockerClient{
		containerInspectWithRawFn: func(context.Context, string, bool) (containertypes.InspectResponse, []byte, error) {
			inspect := baseInspect()
			inspect.ID, inspect.Config.Image = id, forceCheckRef
			return inspect, nil, nil
		},
		containerStatsOneShotFn: func(context.Context, string) (dockerStatsResponseReader, error) {
			return statsReader(t, containertypes.StatsResponse{}), nil
		},
		imageInspectWithRawFn: func(context.Context, string) (image.InspectResponse, []byte, error) {
			return image.InspectResponse{RepoDigests: []string{"registry.example.test/team/app@" + current}, Architecture: "amd64", Os: "linux"}, nil, nil
		},
	}}
	collect := func() (agentsdocker.Container, error) {
		return a.collectContainer(context.Background(), containertypes.Summary{ID: id, Names: []string{"/force-check-service"}, Image: forceCheckRef, ImageID: "force-check-image", State: "running", Status: "Up"})
	}
	publish := func(phase string, c agentsdocker.Container, known, available bool) {
		t.Helper()
		if (c.UpdateStatus != nil) != known || (known && c.UpdateStatus.UpdateAvailable != available) {
			t.Fatalf("%s: collector status = %+v", phase, c.UpdateStatus)
		}
		host, err := monitor.ApplyDockerReport(agentsdocker.Report{Timestamp: time.Now().UTC(), Agent: agentsdocker.AgentInfo{ID: "force-check-host", IntervalSeconds: 30}, Host: agentsdocker.HostInfo{Hostname: "force-check-host"}, Containers: []agentsdocker.Container{c}}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(host.Containers) != 1 || (host.Containers[0].UpdateStatus != nil) != known {
			t.Fatalf("%s: ingestion changed known status: %+v", phase, host)
		}
		// Respect the normal trailing publication window rather than forcing
		// the resource store to invalidate between these fast fixture reports.
		time.Sleep(2200 * time.Millisecond)
		req := httptest.NewRequest(http.MethodGet, "/api/resources?type=app-container&limit=100", nil)
		req.Header.Set("X-API-Token", readerToken)
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("resource HTTP %d: %s", rec.Code, rec.Body.String())
		}
		var body struct {
			Data []struct {
				Name   string                     `json:"name"`
				Docker map[string]json.RawMessage `json:"docker"`
			} `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		found := false
		for _, resource := range body.Data {
			if resource.Name != c.Name {
				continue
			}
			found = true
			raw, present := resource.Docker["updateStatus"]
			if present != known {
				t.Fatalf("%s: HTTP updateStatus presence = %t, want %t: %s", phase, present, known, rec.Body.String())
			}
			if known {
				var got agentsdocker.UpdateStatus
				if err := json.Unmarshal(raw, &got); err != nil {
					t.Fatal(err)
				}
				if got.UpdateAvailable != available || got.CurrentDigest != current || got.LatestDigest != c.UpdateStatus.LatestDigest || got.Error != "" {
					t.Fatalf("%s: HTTP changed fresh evidence: %+v", phase, got)
				}
			}
		}
		if !found {
			t.Fatalf("%s: container missing from HTTP: %s", phase, rec.Body.String())
		}
		if os.Getenv("PULSE_FORCE_CHECK_PROOF") != "" {
			proof, err := json.Marshal(map[string]any{"phase": phase, "resources": json.RawMessage(rec.Body.Bytes()), "host": host})
			if err != nil {
				t.Fatal(err)
			}
			fmt.Printf("FORCE_CHECK_PROOF %s\n", proof)
		}
	}
	first, err := collect()
	if err != nil {
		t.Fatal(err)
	}
	publish("available", first, true, true)

	checker.ForceCheck()
	park := &forceCheckPark{reached: make(chan struct{}), resume: make(chan struct{})}
	reg.mu.Lock()
	reg.park = park
	reg.mu.Unlock()
	var once sync.Once
	resume := func() { once.Do(func() { close(park.resume) }) }
	t.Cleanup(resume)
	type collected struct {
		container agentsdocker.Container
		err       error
	}
	done := make(chan collected, 1)
	go func() {
		c, err := collect()
		done <- collected{c, err}
	}()
	select {
	case <-park.reached:
	case <-time.After(5 * time.Second):
		t.Fatal("collection never reached the registry")
	}
	checker.ForceCheck()
	reg.set(current, http.StatusOK)
	resume()
	select {
	case got := <-done:
		if got.err != nil {
			t.Fatal(got.err)
		}
		publish("unknown", got.container, false, false)
	case <-time.After(5 * time.Second):
		t.Fatal("superseded collection never completed")
	}
	fresh, err := collect()
	if err != nil {
		t.Fatal(err)
	}
	publish("current", fresh, true, false)
	if got := reg.count(); got != 3 {
		t.Fatalf("manifest requests = %d, want 3 including fresh post-clear evidence", got)
	}
}
