package dockeragent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	containertypes "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/image"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
	"github.com/rs/zerolog"
)

// Optional output feeds the ingest and API qualification probes without
// substituting hand-written update statuses for collector output.
func TestCollectRegistryResponseIsolation(t *testing.T) {
	refs := []string{"redis:8-alpine", "postgres:16-alpine", "ghcr.io/searxng/searxng:latest", "ghcr.io/paperless-ngx/paperless-ngx:latest"}
	var reports []agentsdocker.Report
	for _, phase := range []string{"invalid", "valid"} {
		checker := NewRegistryChecker(zerolog.Nop())
		calls := map[string]int{}
		checker.httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Host == "auth.docker.io" || strings.HasSuffix(req.URL.Path, "/token") {
				return newStringResponse(200, nil, `{"token":"synthetic"}`), nil
			}
			key := req.URL.Host + req.URL.Path
			calls[key]++
			var i int
			for i = 0; i < len(refs); i++ {
				reg, repo, tag := parseImageReference(refs[i])
				if key == reg+"/v2/"+repo+"/manifests/"+tag {
					break
				}
			}
			if i == len(refs) {
				return nil, fmt.Errorf("unexpected reference %s", key)
			}
			if phase == "invalid" && i < 3 {
				if req.Method == http.MethodHead {
					return newStringResponse(200, nil, ""), nil
				}
				return newStringResponse(200, nil, `<html>registry unavailable</html>`), nil
			}
			if req.Method == http.MethodHead {
				return newStringResponse(200, map[string]string{"Content-Type": "application/vnd.oci.image.index.v1+json", "Docker-Content-Digest": fmt.Sprintf("sha256:index-%d", i)}, ""), nil
			}
			return newStringResponse(200, nil, fmt.Sprintf(`{"schemaVersion":2,"manifests":[{"digest":"sha256:platform-%d","platform":{"architecture":"amd64","os":"linux"}},{"digest":"sha256:arm-%d","platform":{"architecture":"arm64","os":"linux"}}]}`, i, i)), nil
		})}
		report := agentsdocker.Report{Timestamp: time.Now().UTC(), Agent: agentsdocker.AgentInfo{ID: "registry-proof", IntervalSeconds: 30}, Host: agentsdocker.HostInfo{Hostname: "registry-proof"}}
		for i, ref := range refs {
			inspect := baseInspect()
			inspect.Config.Image = ref
			reg, repo, _ := parseImageReference(ref)
			current := fmt.Sprintf("sha256:index-%d", i)
			if i == 2 {
				current = fmt.Sprintf("sha256:platform-%d", i)
			}
			a := &Agent{logger: zerolog.Nop(), runtime: RuntimeDocker, prevContainerCPU: make(map[string]cpuSample), registryChecker: checker, docker: &fakeDockerClient{
				containerInspectWithRawFn: func(context.Context, string, bool) (containertypes.InspectResponse, []byte, error) {
					return inspect, nil, nil
				},
				containerStatsOneShotFn: func(context.Context, string) (dockerStatsResponseReader, error) {
					return statsReader(t, containertypes.StatsResponse{}), nil
				},
				imageInspectWithRawFn: func(context.Context, string) (image.InspectResponse, []byte, error) {
					return image.InspectResponse{RepoDigests: []string{reg + "/" + repo + "@" + current}, Architecture: "amd64", Os: "linux"}, nil, nil
				},
			}}
			for attempt := 0; attempt < 2; attempt++ {
				got, err := a.collectContainer(context.Background(), containertypes.Summary{ID: fmt.Sprintf("%064x", i+1), Names: []string{fmt.Sprintf("/container-%d", i)}, Image: ref, ImageID: fmt.Sprintf("image-%d", i), State: "running"})
				if err != nil {
					t.Fatal(err)
				}
				s := got.UpdateStatus
				if s == nil {
					t.Fatal("missing status")
				}
				if s.CurrentDigest != current || s.UpdateAvailable {
					t.Errorf("%s %s attempt %d: %+v", phase, ref, attempt, s)
				}
				if phase == "invalid" && i < 3 {
					if s.Error == "" || s.LatestDigest != "" {
						t.Errorf("non-manifest became update: %+v", s)
					}
				} else if s.Error != "" || s.LatestDigest != fmt.Sprintf("sha256:index-%d", i) {
					t.Errorf("reference/platform mismatch: %+v", s)
				}
				if attempt == 1 {
					report.Containers = append(report.Containers, got)
				}
			}
		}
		for key, n := range calls {
			if n != 2 {
				t.Errorf("%s requests=%d, expected one HEAD/GET with cache reuse", key, n)
			}
		}
		reports = append(reports, report)
	}
	data, err := json.MarshalIndent(reports, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if path := os.Getenv("PULSE_REGISTRY_REPORT_PROOF"); path != "" {
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
