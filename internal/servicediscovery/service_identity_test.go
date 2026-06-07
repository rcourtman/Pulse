package servicediscovery

import "testing"

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
