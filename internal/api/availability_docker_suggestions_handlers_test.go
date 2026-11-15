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
	// duringProbes runs once, on the first probe, to stand in for an
	// operator changing checks while the request is still probing.
	duringProbes func()
}

func (s *dockerPortProbeScript) probe(_ context.Context, target config.AvailabilityTarget) (availabilityprobe.ProbeResult, error) {
	key := fmt.Sprintf("%s %s:%d", target.Protocol, target.Address, target.Port)
	s.mu.Lock()
	s.calls = append(s.calls, key)
	err, known := s.answers[key]
	hook := s.duringProbes
	s.duringProbes = nil
	s.mu.Unlock()
	if hook != nil {
		hook()
	}
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

// The operator is shown a list and adds exactly that: a port they dismissed
// earlier, or one published since, is not added behind their back.
func TestApplyDockerPortSuggestionsAddsOnlyTheSelectedPorts(t *testing.T) {
	script := &dockerPortProbeScript{answers: map[string]error{
		"tcp 192.168.1.20:8081": nil, "http 192.168.1.20:8081": nil,
		"tcp 192.168.1.20:8443": nil, "https 192.168.1.20:8443": nil,
	}}
	handler, persistence := dockerPortSuggestionHandlers(t, script)

	body := `{"ports":[{"hostId":"host-1","containerName":"web","privatePort":80}]}`
	rec := httptest.NewRecorder()
	handler.HandleApplyDockerPortSuggestions(rec, httptest.NewRequest(http.MethodPost, "/api/availability-targets/docker-suggestions", strings.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	saved, _ := persistence.LoadAvailabilityTargets()
	if len(saved) != 1 || saved[0].Name != "web:8081" {
		t.Fatalf("saved = %+v, want only the selected web port", saved)
	}
	for _, call := range script.calls {
		if !strings.HasSuffix(call, ":8081") {
			t.Fatalf("probed a port that was not selected: %q", call)
		}
	}
}

// Probing takes seconds. A check the operator adds or removes meanwhile must
// survive the save, and a port they covered by hand must not get a second check.
func TestApplyDockerPortSuggestionsKeepsChangesMadeWhileProbing(t *testing.T) {
	script := &dockerPortProbeScript{answers: map[string]error{
		"tcp 192.168.1.20:8081": nil, "http 192.168.1.20:8081": nil,
		"tcp 192.168.1.20:8443": nil, "https 192.168.1.20:8443": nil,
	}}
	handler, persistence := dockerPortSuggestionHandlers(t, script)
	existing := config.NormalizeAvailabilityTarget(config.AvailabilityTarget{ID: "doomed", Name: "old router", Protocol: config.AvailabilityProbeICMP, Address: "192.168.1.1", TargetKind: config.AvailabilityTargetDevice})
	if err := persistence.SaveAvailabilityTargets([]config.AvailabilityTarget{existing}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	script.duringProbes = func() {
		// Meanwhile: the operator deletes "doomed" and adds their own check
		// for the web port.
		byHand := config.NormalizeAvailabilityTarget(config.AvailabilityTarget{ID: "by-hand", Name: "my web check", Protocol: config.AvailabilityProbeTCP, Address: "192.168.1.20", Port: 8081, TargetKind: config.AvailabilityTargetService})
		if err := persistence.SaveAvailabilityTargets([]config.AvailabilityTarget{byHand}); err != nil {
			t.Errorf("concurrent save: %v", err)
		}
	}

	rec := httptest.NewRecorder()
	handler.HandleApplyDockerPortSuggestions(rec, httptest.NewRequest(http.MethodPost, "/api/availability-targets/docker-suggestions", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	saved, _ := persistence.LoadAvailabilityTargets()
	names := map[string]bool{}
	for _, target := range saved {
		names[target.Name] = true
	}
	if names["old router"] {
		t.Fatalf("a check deleted while probing came back: %+v", names)
	}
	if !names["my web check"] || names["web:8081"] {
		t.Fatalf("saved = %v, want the operator's web check kept and no second one", names)
	}
	if !names["secure:8443"] {
		t.Fatalf("saved = %v, want the untouched port still added", names)
	}
}

// A host reachable only over IPv6 gets a working HTTP check: a bare IPv6
// literal is not a valid URL host.
func TestApplyDockerPortSuggestionsBracketsAnIPv6Address(t *testing.T) {
	script := &dockerPortProbeScript{answers: map[string]error{
		"tcp 2001:db8::20:8081":    nil,
		"http [2001:db8::20]:8081": nil,
	}}
	handler, persistence := dockerPortSuggestionHandlers(t, script)
	handler.dockerHosts = func(context.Context) []models.DockerHost {
		return []models.DockerHost{{
			ID: "host-6", Hostname: "v6-only",
			NetworkInterfaces: []models.HostNetworkInterface{{Name: "eth0", Addresses: []string{"2001:db8::20/64"}}},
			Containers: []models.DockerContainer{
				{ID: "c-web", Name: "web", State: "running", Ports: []models.DockerContainerPort{{PrivatePort: 80, PublicPort: 8081, Protocol: "tcp", IP: "::"}}},
			},
		}}
	}

	rec := httptest.NewRecorder()
	handler.HandleApplyDockerPortSuggestions(rec, httptest.NewRequest(http.MethodPost, "/api/availability-targets/docker-suggestions", nil))
	saved, _ := persistence.LoadAvailabilityTargets()
	if len(saved) != 1 || saved[0].Protocol != config.AvailabilityProbeHTTP {
		t.Fatalf("saved = %+v, body=%s", saved, rec.Body.String())
	}
	u, err := saved[0].HTTPURL()
	if err != nil || u.String() != "http://[2001:db8::20]:8081" {
		t.Fatalf("saved URL = %v (%v), want http://[2001:db8::20]:8081", u, err)
	}
	// It still counts as covered, so it is not offered twice.
	rec = httptest.NewRecorder()
	handler.HandleDockerPortSuggestions(rec, httptest.NewRequest(http.MethodGet, "/api/availability-targets/docker-suggestions", nil))
	if !strings.Contains(rec.Body.String(), `"suggestions":[]`) {
		t.Fatalf("an IPv6 port with a check was offered again: %s", rec.Body.String())
	}
}
