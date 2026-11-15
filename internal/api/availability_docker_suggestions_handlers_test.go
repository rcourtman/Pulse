package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/availabilityprobe"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
)

// dockerPortProbeScript answers probes by "protocol port", so a test states
// what each published port does without opening sockets.
type dockerPortProbeScript struct {
	mu      sync.Mutex
	answers map[string]error
	calls   []string
}

func (s *dockerPortProbeScript) probe(_ context.Context, target config.AvailabilityTarget) (availabilityprobe.ProbeResult, error) {
	key := fmt.Sprintf("%s %s:%d", target.Protocol, target.Address, target.Port)
	s.mu.Lock()
	s.calls = append(s.calls, key)
	err, known := s.answers[key]
	s.mu.Unlock()
	if !known {
		err = errors.New("connection refused")
	}
	if err != nil {
		result := availabilityprobe.ProbeResult{Outcome: availabilityprobe.OutcomeUnreachable, TransportOutcome: availabilityprobe.OutcomeUnreachable}
		if strings.HasPrefix(err.Error(), "http probe returned status") {
			result.TransportOutcome = availabilityprobe.OutcomeReachable
		}
		return result, err
	}
	return availabilityprobe.ProbeResult{Outcome: availabilityprobe.OutcomeReachable, TransportOutcome: availabilityprobe.OutcomeReachable}, nil
}

func dockerPortSuggestionHandlers(t *testing.T, script *dockerPortProbeScript) (*AvailabilityHandlers, *config.ConfigPersistence) {
	t.Helper()
	persistence := config.NewConfigPersistence(t.TempDir())
	host := models.DockerHost{
		ID:       "host-1",
		Hostname: "docker-vm",
		NetworkInterfaces: []models.HostNetworkInterface{
			{Name: "eth0", Addresses: []string{"192.168.1.20/24"}},
		},
		Containers: []models.DockerContainer{
			{ID: "c-web", Name: "web", State: "running", Ports: []models.DockerContainerPort{{PrivatePort: 80, PublicPort: 8081, Protocol: "tcp", IP: "0.0.0.0"}}},
			{ID: "c-tls", Name: "secure", State: "running", Ports: []models.DockerContainerPort{{PrivatePort: 443, PublicPort: 8443, Protocol: "tcp", IP: "0.0.0.0"}}},
			{ID: "c-cache", Name: "cache", State: "running", Ports: []models.DockerContainerPort{{PrivatePort: 6379, PublicPort: 6379, Protocol: "tcp", IP: "0.0.0.0"}}},
			{ID: "c-raw", Name: "raw", State: "running", Ports: []models.DockerContainerPort{{PrivatePort: 9999, PublicPort: 9999, Protocol: "tcp", IP: "0.0.0.0"}}},
			{ID: "c-sick", Name: "sick", State: "running", Ports: []models.DockerContainerPort{{PrivatePort: 3000, PublicPort: 3000, Protocol: "tcp", IP: "0.0.0.0"}}},
			{ID: "c-dead", Name: "dead", State: "running", Ports: []models.DockerContainerPort{{PrivatePort: 80, PublicPort: 8082, Protocol: "tcp", IP: "0.0.0.0"}}},
			{ID: "c-local", Name: "local", State: "running", Ports: []models.DockerContainerPort{{PrivatePort: 80, PublicPort: 8083, Protocol: "tcp", IP: "127.0.0.1"}}},
		},
	}
	handler := NewAvailabilityHandlers(
		func(context.Context) *config.ConfigPersistence { return persistence },
		func(context.Context) *monitoring.Monitor { return nil },
		nil,
	)
	handler.dockerHosts = func(context.Context) []models.DockerHost { return []models.DockerHost{host} }
	handler.dockerContainerResources = func(context.Context) map[string]string {
		return map[string]string{dockerContainerResourceKey("host-1", "c-web"): "res-web"}
	}
	handler.dockerPortProbe = script.probe
	return handler, persistence
}

func TestDockerPortSuggestionsListWhatHasNoCheck(t *testing.T) {
	handler, _ := dockerPortSuggestionHandlers(t, &dockerPortProbeScript{})

	rec := httptest.NewRecorder()
	handler.HandleDockerPortSuggestions(rec, httptest.NewRequest(http.MethodGet, "/api/availability-targets/docker-suggestions", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var payload dockerPortSuggestionsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(payload.Suggestions) != 7 {
		t.Fatalf("suggestions = %d, want all 7 published ports: %s", len(payload.Suggestions), rec.Body.String())
	}
	for _, suggestion := range payload.Suggestions {
		if suggestion.ContainerName == "local" {
			if suggestion.SkipReason != monitoring.DockerPortSkipLoopbackOnly || suggestion.Address != "" {
				t.Fatalf("loopback-only port = %+v", suggestion)
			}
		} else if suggestion.ContainerName == "cache" {
			if suggestion.SkipReason != monitoring.DockerPortSkipNotHTTP {
				t.Fatalf("Redis port = %+v, want it marked as not a web port", suggestion)
			}
		} else if suggestion.Address != "192.168.1.20" || suggestion.SkipReason != "" {
			t.Fatalf("suggestion = %+v, want the host's LAN address", suggestion)
		}
	}
}

func TestApplyDockerPortSuggestionsAddsOnlyWhatAnswers(t *testing.T) {
	script := &dockerPortProbeScript{answers: map[string]error{
		"tcp 192.168.1.20:8081":   nil,
		"http 192.168.1.20:8081":  nil,
		"tcp 192.168.1.20:8443":   nil,
		"https 192.168.1.20:8443": nil,
		"tcp 192.168.1.20:6379":   nil,
		"tcp 192.168.1.20:9999":   nil,
		"tcp 192.168.1.20:3000":   nil,
		"http 192.168.1.20:3000":  errors.New("http probe returned status 503"),
	}}
	handler, persistence := dockerPortSuggestionHandlers(t, script)

	rec := httptest.NewRecorder()
	handler.HandleApplyDockerPortSuggestions(rec, httptest.NewRequest(http.MethodPost, "/api/availability-targets/docker-suggestions", strings.NewReader(`{"hostId":"host-1"}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var payload dockerPortApplyResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}

	saved, err := persistence.LoadAvailabilityTargets()
	if err != nil {
		t.Fatalf("LoadAvailabilityTargets() error = %v", err)
	}
	protocols := map[string]config.AvailabilityProbeProtocol{}
	for _, target := range saved {
		protocols[target.Name] = target.Protocol
		if target.DockerPort == nil || target.DockerPort.HostID != "host-1" {
			t.Errorf("%s saved without its source: %+v", target.Name, target.DockerPort)
		}
		if target.Address != "192.168.1.20" || !target.Enabled || !target.IncludesLocalObservation() {
			t.Errorf("%s = %+v, want an enabled check from this server on the host address", target.Name, target)
		}
	}
	want := map[string]config.AvailabilityProbeProtocol{
		"web:8081":    config.AvailabilityProbeHTTP,
		"secure:8443": config.AvailabilityProbeHTTPS,
	}
	if len(protocols) != len(want) {
		t.Fatalf("saved checks = %v, want %v", protocols, want)
	}
	for name, protocol := range want {
		if protocols[name] != protocol {
			t.Errorf("%s saved as %q, want %q", name, protocols[name], protocol)
		}
	}
	if len(payload.Created) != len(want) {
		t.Fatalf("created = %d, want %d", len(payload.Created), len(want))
	}
	for _, target := range saved {
		if target.Name == "web:8081" && target.LinkedResourceID != "res-web" {
			t.Errorf("web check linked to %q, want its container resource", target.LinkedResourceID)
		}
	}

	// A port that refuses, and one that answers HTTP 503, get no check: it
	// would alert from its first run. A port that only accepts a connection
	// gets none either: Docker's proxy accepts for a dead service when Pulse
	// shares the host, so a connection check there would pass for good.
	unanswered := map[string]string{}
	for _, item := range payload.Unanswered {
		unanswered[item.ContainerName] = item.Error
	}
	if len(unanswered) != 3 || unanswered["dead"] == "" || !strings.Contains(unanswered["sick"], "503") ||
		!strings.Contains(unanswered["raw"], "did not answer over HTTP or HTTPS") {
		t.Fatalf("unanswered = %v, want dead (refused), sick (503) and raw (not HTTP)", unanswered)
	}
	skipped := map[string]string{}
	for _, item := range payload.Skipped {
		skipped[item.ContainerName] = item.SkipReason
	}
	if len(skipped) != 2 || skipped["local"] != monitoring.DockerPortSkipLoopbackOnly || skipped["cache"] != monitoring.DockerPortSkipNotHTTP {
		t.Fatalf("skipped = %v, want the loopback-only port and the Redis port", skipped)
	}
	for _, call := range script.calls {
		if strings.HasSuffix(call, ":6379") {
			t.Fatalf("Redis port was probed: %q", call)
		}
		if strings.HasSuffix(call, ":8083") {
			t.Fatalf("loopback-only port was probed: %q", call)
		}
	}

	// A second request offers only what is still unchecked and adds nothing
	// twice.
	rec = httptest.NewRecorder()
	handler.HandleApplyDockerPortSuggestions(rec, httptest.NewRequest(http.MethodPost, "/api/availability-targets/docker-suggestions", nil))
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode second apply: %v", err)
	}
	if len(payload.Created) != 0 || len(payload.Unanswered) != 3 {
		t.Fatalf("second apply = %+v, want nothing new", payload)
	}
	again, _ := persistence.LoadAvailabilityTargets()
	if len(again) != len(saved) {
		t.Fatalf("second apply changed the saved checks: %d -> %d", len(saved), len(again))
	}
}

func TestApplyDockerPortSuggestionsLimitsToTheRequestedHost(t *testing.T) {
	script := &dockerPortProbeScript{answers: map[string]error{"tcp 192.168.1.20:8081": nil, "http 192.168.1.20:8081": nil}}
	handler, persistence := dockerPortSuggestionHandlers(t, script)

	rec := httptest.NewRecorder()
	handler.HandleApplyDockerPortSuggestions(rec, httptest.NewRequest(http.MethodPost, "/api/availability-targets/docker-suggestions", strings.NewReader(`{"hostId":"another-host"}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if saved, _ := persistence.LoadAvailabilityTargets(); len(saved) != 0 {
		t.Fatalf("checks added for a host that was not asked for: %+v", saved)
	}
	if len(script.calls) != 0 {
		t.Fatalf("probed %v for a host that was not asked for", script.calls)
	}
}
