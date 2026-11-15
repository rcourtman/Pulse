package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/availabilityprobe"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/securityutil"
)

// These namesakes intentionally have identical Docker host, container and
// port keys. Only the request's admitted organization chooses their inventory,
// saved targets and resource link. A body or a previous request cannot do so.
func TestDockerPortSuggestionsKeepNamesakesInTheRequestOrganization(t *testing.T) {
	stores := map[string]*config.ConfigPersistence{}
	for _, org := range []string{"customer-a", "customer-b"} {
		store := config.NewConfigPersistence(t.TempDir())
		keep := config.NormalizeAvailabilityTarget(config.AvailabilityTarget{ID: "keep-" + org, Name: "router", Protocol: config.AvailabilityProbeICMP, Address: "192.0.2.1"})
		if err := store.SaveAvailabilityTargets([]config.AvailabilityTarget{keep}); err != nil {
			t.Fatal(err)
		}
		stores[org] = store
	}
	addresses := map[string]string{"customer-a": "192.0.2.20", "customer-b": "192.0.2.21"}
	handler := NewAvailabilityHandlers(func(ctx context.Context) *config.ConfigPersistence { return stores[GetOrgID(ctx)] }, nil, nil)
	handler.dockerHosts = func(ctx context.Context) []models.DockerHost {
		address, ok := addresses[GetOrgID(ctx)]
		if !ok {
			t.Errorf("inventory received unbound context")
			return nil
		}
		return []models.DockerHost{{
			ID: "shared-host-id", Hostname: "docker-vm",
			NetworkInterfaces: []models.HostNetworkInterface{{Name: "eth0", Addresses: []string{address + "/24"}}},
			Containers:        []models.DockerContainer{{ID: "shared-container-id", Name: "web", State: "running", Ports: []models.DockerContainerPort{{PrivatePort: 80, PublicPort: 8081, Protocol: "tcp", IP: "0.0.0.0"}}}},
		}}
	}
	handler.dockerContainerResources = func(ctx context.Context) map[string]string {
		return map[string]string{dockerContainerResourceKey("shared-host-id", "shared-container-id"): "resource-" + GetOrgID(ctx)}
	}
	var mu sync.Mutex
	var probes []string
	handler.dockerPortProbe = func(ctx context.Context, target config.AvailabilityTarget) (availabilityprobe.ProbeResult, error) {
		org := GetOrgID(ctx)
		if target.Address != addresses[org] {
			t.Errorf("%s probed another organization's address: %q", org, target.Address)
		}
		mu.Lock()
		probes = append(probes, org)
		mu.Unlock()
		return availabilityprobe.ProbeResult{Outcome: availabilityprobe.OutcomeReachable, TransportOutcome: availabilityprobe.OutcomeReachable}, nil
	}
	request := func(org, method, body string) *http.Request {
		req := httptest.NewRequest(method, "/api/availability-targets/docker-suggestions", strings.NewReader(body))
		return req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, org))
	}
	list := func(org string) []monitoring.DockerPortCheckCandidate {
		t.Helper()
		rec := httptest.NewRecorder()
		handler.HandleDockerPortSuggestions(rec, request(org, http.MethodGet, ""))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s list = %d %s", org, rec.Code, rec.Body.String())
		}
		var payload dockerPortSuggestionsResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		return payload.Suggestions
	}

	for _, org := range []string{"customer-a", "customer-b"} {
		suggestions := list(org)
		if len(suggestions) != 1 || suggestions[0].Address != addresses[org] {
			t.Fatalf("%s saw another organization's suggestions: %+v", org, suggestions)
		}
		// An organization identity in the untrusted body cannot switch the
		// persistence, inventory, probe context or canonical resource lookup.
		body, err := json.Marshal(map[string]any{"orgId": "customer-b", "ports": suggestions})
		if err != nil {
			t.Fatal(err)
		}
		rec := httptest.NewRecorder()
		handler.HandleApplyDockerPortSuggestions(rec, request(org, http.MethodPost, string(body)))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s apply = %d %s", org, rec.Code, rec.Body.String())
		}
		saved, err := stores[org].LoadAvailabilityTargets()
		if err != nil || len(saved) != 2 || saved[0].ID != "keep-"+org || saved[1].Address != addresses[org] || saved[1].LinkedResourceID != "resource-"+org {
			t.Fatalf("%s saved another organization's data: %+v (%v)", org, saved, err)
		}
		if remaining := list(org); len(remaining) != 0 {
			t.Fatalf("%s did not read its own newly saved checks: %+v", org, remaining)
		}
		if org == "customer-a" {
			other, err := stores["customer-b"].LoadAvailabilityTargets()
			if err != nil || len(other) != 1 || other[0].ID != "keep-customer-b" {
				t.Fatalf("customer-a changed customer-b's saved state: %+v (%v)", other, err)
			}
		}
	}
	if len(probes) != 4 || probes[0] != "customer-a" || probes[1] != "customer-a" || probes[2] != "customer-b" || probes[3] != "customer-b" {
		t.Fatalf("probes lost request organization: %v", probes)
	}
}

func TestDockerPortSuggestionsDoNotFallBackWhenRequestPersistenceIsMissing(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		t.Run(method, func(t *testing.T) {
			handler := NewAvailabilityHandlers(func(context.Context) *config.ConfigPersistence { return nil }, nil, nil)
			handler.dockerHosts = func(context.Context) []models.DockerHost {
				t.Error("unavailable request persistence must stop before inventory access")
				return nil
			}
			handler.dockerPortProbe = func(context.Context, config.AvailabilityTarget) (availabilityprobe.ProbeResult, error) {
				t.Error("unavailable request persistence must stop before any probe")
				return availabilityprobe.ProbeResult{}, nil
			}
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(method, "/api/availability-targets/docker-suggestions", strings.NewReader(`{"ports":[{"hostId":"h","containerId":"c","containerName":"web","privatePort":80,"publicPort":8081,"address":"192.0.2.20"}]}`))
			if method == http.MethodGet {
				handler.HandleDockerPortSuggestions(rec, req)
			} else {
				handler.HandleApplyDockerPortSuggestions(rec, req)
			}
			if rec.Code != http.StatusInternalServerError || !strings.Contains(rec.Body.String(), `"code":"availability_unavailable"`) {
				t.Fatalf("missing persistence = %d %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestDockerPortDetectionAppliesTenantPolicyBeforeAnyProbe(t *testing.T) {
	for _, tc := range []struct {
		name    string
		address string
		private bool
		blocked bool
	}{
		{"cloud-private-v4", "192.168.1.20", false, true},
		{"cloud-private-v6", "fd12::5", false, true},
		{"self-hosted-private-v4", "192.168.1.20", true, false},
		{"self-hosted-private-v6", "fd12::5", true, false},
		{"self-hosted-loopback-v4", "127.0.0.1", true, true},
		{"self-hosted-loopback-v6", "::1", true, true},
		{"self-hosted-metadata", "169.254.169.254", true, true},
		{"self-hosted-link-local-v6", "fe80::1", true, true},
		{"cloud-public", "93.184.216.34", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := securityutil.WithTenantOutboundPolicy(context.Background(), securityutil.TenantOutboundPolicy{AllowPrivateNetworks: tc.private})
			var calls int
			probe := func(gotCtx context.Context, target config.AvailabilityTarget) (availabilityprobe.ProbeResult, error) {
				calls++
				policy, ok := securityutil.TenantOutboundPolicyFromContext(gotCtx)
				if !ok || policy.AllowPrivateNetworks != tc.private {
					t.Error("probe lost tenant policy")
				}
				if target.Protocol != config.AvailabilityProbeTCP && target.Protocol != config.AvailabilityProbeHTTPS {
					t.Errorf("unexpected plaintext fallback: %s", target.Protocol)
				}
				return availabilityprobe.ProbeResult{Outcome: availabilityprobe.OutcomeReachable, TransportOutcome: availabilityprobe.OutcomeReachable}, nil
			}
			candidate := monitoring.DockerPortCheckCandidate{HostID: "h", ContainerID: "c", ContainerName: "web", PrivatePort: 443, PublicPort: 8443, Address: tc.address}
			target, err := detectDockerPortCheck(ctx, probe, candidate)
			if tc.blocked {
				if err == nil || calls != 0 {
					t.Fatalf("tenant destination %q: err=%v, probes=%d", tc.address, err, calls)
				}
			} else if err != nil || calls != 2 || target.Protocol != config.AvailabilityProbeHTTPS || target.DockerPort == nil {
				t.Fatalf("permitted destination %q: target=%+v, err=%v, probes=%d", tc.address, target, err, calls)
			}
		})
	}
}

func TestDockerPortSuggestionsRetainSavedStateWhenTenantRefusesDestination(t *testing.T) {
	script := &dockerPortProbeScript{}
	handler, persistence := dockerPortSuggestionHandlers(t, script)
	existing := config.NormalizeAvailabilityTarget(config.AvailabilityTarget{ID: "keep", Name: "router", Protocol: config.AvailabilityProbeICMP, Address: "192.0.2.1"})
	if err := persistence.SaveAvailabilityTargets([]config.AvailabilityTarget{existing}); err != nil {
		t.Fatal(err)
	}
	ctx := securityutil.WithTenantOutboundPolicy(context.Background(), securityutil.TenantOutboundPolicy{})
	body := `{"ports":[{"hostId":"host-1","containerId":"c-web","containerName":"web","privatePort":80,"publicPort":8081,"address":"192.168.1.20"}]}`
	rec := httptest.NewRecorder()
	handler.HandleApplyDockerPortSuggestions(rec, httptest.NewRequest(http.MethodPost, "/api/availability-targets/docker-suggestions", strings.NewReader(body)).WithContext(ctx))
	var response dockerPortApplyResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK || len(response.Created) != 0 || len(response.Unanswered) != 1 || !strings.Contains(response.Unanswered[0].Error, "Organization administrators cannot connect Pulse") || len(script.calls) != 0 {
		t.Fatalf("tenant refusal = %d %+v, probes=%v", rec.Code, response, script.calls)
	}
	saved, err := persistence.LoadAvailabilityTargets()
	if err != nil || len(saved) != 1 || saved[0].ID != existing.ID {
		t.Fatalf("tenant refusal changed saved state: %+v (%v)", saved, err)
	}
}
