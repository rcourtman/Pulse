package servicediscovery

import (
	"strings"
	"testing"
)

func TestInferSurfaceIdentity(t *testing.T) {
	cases := []struct {
		name     string
		hostname string
		wantType string
		wantOK   bool
	}{
		{"home assistant by hostname", "home-assistant", "home-assistant", true},
		{"homeassistant one word", "homeassistant", "home-assistant", true},
		{"esphome by hostname", "esphome", "esphome", true},
		{"frigate by hostname", "frigate", "frigate", true},
		{"mqtt maps to mosquitto", "mqtt", "mosquitto", true},
		{"plex within a longer name", "plex-media", "plex", true},
		{"postgres by hostname", "postgres-primary", "postgresql", true},
		{"generic name does not match", "ct-200", "", false},
		{"numeric id does not match", "101", "", false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			identity, _, ok := inferSurfaceIdentity(DiscoveryRequest{Hostname: tc.hostname}, nil)
			if ok != tc.wantOK {
				t.Fatalf("inferSurfaceIdentity(%q) ok=%v, want %v", tc.hostname, ok, tc.wantOK)
			}
			if ok && identity.ServiceType != tc.wantType {
				t.Fatalf("inferSurfaceIdentity(%q) type=%q, want %q", tc.hostname, identity.ServiceType, tc.wantType)
			}
		})
	}
}

func TestSurfaceIdentityResponseIsIdentityOnly(t *testing.T) {
	identity, evidence, ok := inferSurfaceIdentity(DiscoveryRequest{Hostname: "home-assistant"}, nil)
	if !ok {
		t.Fatal("expected home-assistant to match by name")
	}
	resp := surfaceIdentityResponse(identity, evidence)
	if resp.ServiceType != "home-assistant" || resp.ServiceName != "Home Assistant" {
		t.Fatalf("unexpected identity: type=%q name=%q", resp.ServiceType, resp.ServiceName)
	}
	if resp.Confidence < 0.85 {
		t.Fatalf("expected a confident identity, got %v", resp.Confidence)
	}
	// Surface = identity only. Paths/facts are intentionally empty — the
	// Assistant knows standard layouts and fetches specifics on demand.
	if len(resp.ConfigPaths) != 0 || len(resp.DataPaths) != 0 || len(resp.LogPaths) != 0 || len(resp.Facts) != 0 {
		t.Fatalf("surface response must carry no deep paths/facts, got %+v", resp)
	}
}

func TestNestedContainerForService(t *testing.T) {
	// Mirrors the real HA-in-LXC topology on delly: homeassistant + watchtower.
	haProbe := "homeassistant|ghcr.io/home-assistant/home-assistant:stable\nwatchtower|containrrr/watchtower"
	cases := []struct {
		name        string
		probe       string
		serviceType string
		serviceName string
		want        string
	}{
		{"HA matches by container name", haProbe, "home-assistant", "Home Assistant", "homeassistant"},
		{"postgres matches by image", "db|postgres:16-alpine", "postgresql", "PostgreSQL", "db"},
		{"unrelated service does not match", haProbe, "redis", "Redis", ""},
		{"no nested containers", "", "home-assistant", "Home Assistant", ""},
		{"watchtower alone is not the service", "watchtower|containrrr/watchtower", "home-assistant", "Home Assistant", ""},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := nestedContainerForService(tc.probe, tc.serviceType, tc.serviceName)
			if got != tc.want {
				t.Fatalf("nestedContainerForService = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestWithNestedDockerAccess(t *testing.T) {
	base := "Use pulse_control with target_host matching this container's hostname."
	out := withNestedDockerAccess(base, "homeassistant")
	if !strings.Contains(out, base) {
		t.Fatalf("expected base guidance preserved, got %q", out)
	}
	if !strings.Contains(out, "docker exec homeassistant") {
		t.Fatalf("expected layered docker exec path, got %q", out)
	}
}

func TestInferSurfaceIdentityFromPorts(t *testing.T) {
	haSS := "State Recv-Q Send-Q Local-Address:Port Peer:Port Process\n" +
		"LISTEN 0 4096 0.0.0.0:8123 0.0.0.0:* users:((\"python3\",pid=1234,fd=10))\n" +
		"LISTEN 0 128 127.0.0.1:22 0.0.0.0:*"
	cases := []struct {
		name     string
		output   string
		wantType string
		wantOK   bool
	}{
		{"HA by distinctive port 8123", haSS, "home-assistant", true},
		{"postgres by 5432", "LISTEN 0 244 0.0.0.0:5432 0.0.0.0:*", "postgresql", true},
		{"plex by 32400", "LISTEN 0 128 *:32400 *:*", "plex", true},
		{"ambiguous ports only do not match", "LISTEN 0 128 0.0.0.0:80\nLISTEN 0 128 0.0.0.0:443\nLISTEN 0 128 0.0.0.0:22", "", false},
		{"empty output", "", "", false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			id, _, ok := inferSurfaceIdentityFromPorts(tc.output)
			if ok != tc.wantOK {
				t.Fatalf("inferSurfaceIdentityFromPorts ok=%v, want %v", ok, tc.wantOK)
			}
			if ok && id.ServiceType != tc.wantType {
				t.Fatalf("inferSurfaceIdentityFromPorts type=%q, want %q", id.ServiceType, tc.wantType)
			}
		})
	}
}
