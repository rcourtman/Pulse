package api

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestRuntimeInventorySourcesDropsDisabledSources(t *testing.T) {
	sources := runtimeInventorySources([]Connection{
		{ID: "pve:a", Type: ConnectionTypePVE, Name: "a", State: ConnectionStateActive, Enabled: true},
		{ID: "pve:b", Type: ConnectionTypePVE, Name: "b", State: ConnectionStateUnreachable, Enabled: false},
	})

	if len(sources) != 1 {
		t.Fatalf("expected only the enabled source, got %d", len(sources))
	}
	if sources[0].ID != "pve:a" {
		t.Fatalf("expected pve:a, got %q", sources[0].ID)
	}
}

func TestRuntimeInventorySourcesPrefersScopeOverSurfaces(t *testing.T) {
	sources := runtimeInventorySources([]Connection{{
		ID:       "pve:a",
		Type:     ConnectionTypePVE,
		Name:     "a",
		State:    ConnectionStateStale,
		Enabled:  true,
		Surfaces: []string{"vms", "containers"},
		Scope:    map[string]bool{"vms": true, "containers": false},
	}})

	if len(sources) != 1 {
		t.Fatalf("expected one source, got %d", len(sources))
	}
	if got := sources[0].Surfaces; !reflect.DeepEqual(got, []string{"vms"}) {
		t.Fatalf("expected scope to win over declared surfaces, got %v", got)
	}
}

func TestRuntimeInventorySourcesFallsBackToDeclaredSurfaces(t *testing.T) {
	sources := runtimeInventorySources([]Connection{{
		ID:       "docker:a",
		Type:     ConnectionTypeDocker,
		Name:     "a",
		State:    ConnectionStateActive,
		Enabled:  true,
		Surfaces: []string{"containers", "  ", "docker"},
		Scope:    map[string]bool{"containers": false},
	}})

	if got := sources[0].Surfaces; !reflect.DeepEqual(got, []string{"containers", "docker"}) {
		t.Fatalf("expected declared surfaces with blanks dropped, got %v", got)
	}
}

func TestRuntimeInventorySourcesCredentialSignals(t *testing.T) {
	expired := time.Now().Add(-time.Hour)
	cases := []struct {
		name string
		conn Connection
		want bool
	}{
		{
			name: "unauthorized state",
			conn: Connection{Enabled: true, State: ConnectionStateUnauthorized},
			want: true,
		},
		{
			name: "fleet credential status invalid",
			conn: Connection{
				Enabled: true,
				State:   ConnectionStateActive,
				Fleet:   ConnectionFleetGovernance{CredentialStatus: "invalid"},
			},
			want: true,
		},
		{
			name: "fleet credential health expired",
			conn: Connection{
				Enabled: true,
				State:   ConnectionStateActive,
				Fleet: ConnectionFleetGovernance{
					CredentialHealth: &ConnectionFleetCredentialHealth{Status: "expired", ExpiresAt: &expired},
				},
			},
			want: true,
		},
		{
			name: "healthy",
			conn: Connection{
				Enabled: true,
				State:   ConnectionStateActive,
				Fleet:   ConnectionFleetGovernance{CredentialStatus: "valid"},
			},
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sources := runtimeInventorySources([]Connection{tc.conn})
			if got := sources[0].CredentialsInvalid; got != tc.want {
				t.Fatalf("credentialsInvalid = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestRuntimeInventorySourceWireShapeIsWhitelisted is the disclosure guard.
// The projection must never grow a field that identifies where a source lives
// or how Pulse authenticates to it, because this route is served at
// monitoring:read to non-admin sessions.
func TestRuntimeInventorySourceWireShapeIsWhitelisted(t *testing.T) {
	encoded, err := json.Marshal(RuntimeInventorySource{})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	allowed := map[string]struct{}{
		"id":                 {},
		"type":               {},
		"name":               {},
		"state":              {},
		"surfaces":           {},
		"credentialsInvalid": {},
	}
	for field := range decoded {
		if _, ok := allowed[field]; !ok {
			t.Fatalf("RuntimeInventorySource grew unwhitelisted field %q; widening this "+
				"monitoring:read projection is a deliberate disclosure decision", field)
		}
	}
	for field := range allowed {
		if _, ok := decoded[field]; !ok {
			t.Fatalf("RuntimeInventorySource lost whitelisted field %q", field)
		}
	}
}

// TestRuntimeInventorySourcesOmitAdministrativeFacts pins that a fully
// populated administrative Connection loses every sensitive fact when
// projected, rather than relying on the field-name guard alone.
func TestRuntimeInventorySourcesOmitAdministrativeFacts(t *testing.T) {
	seen := time.Now()
	sources := runtimeInventorySources([]Connection{{
		ID:          "vmware:vc-1",
		Type:        ConnectionTypeVMware,
		Name:        "Primary vCenter",
		Address:     "https://vcenter.corp.local:443",
		HostAliases: []string{"10.0.1.5", "vcenter-old.corp.local"},
		State:       ConnectionStateUnreachable,
		StateReason: `Get "https://vcenter.corp.local/sdk": dial tcp 10.0.1.5:443: i/o timeout`,
		Enabled:     true,
		Surfaces:    []string{"vms"},
		LastSeen:    &seen,
		LastError: &ConnectionError{
			At:      seen,
			Message: `x509: certificate is valid for vcenter.corp.local, not 10.0.1.5`,
		},
		AgentIdentity: &ConnectionAgentIdentity{
			Hostname: "collector-01",
			ReportIP: "10.0.1.9",
			OSName:   "Debian",
		},
		AgentVersion: "6.1.0",
	}})

	encoded, err := json.Marshal(sources[0])
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	payload := string(encoded)

	for _, leaked := range []string{
		"vcenter.corp.local",
		"10.0.1.5",
		"10.0.1.9",
		"collector-01",
		"x509",
		"i/o timeout",
		"6.1.0",
		"Debian",
	} {
		if strings.Contains(payload, leaked) {
			t.Fatalf("projection leaked %q into the monitoring:read payload: %s", leaked, payload)
		}
	}

	if sources[0].Name != "Primary vCenter" || sources[0].State != ConnectionStateUnreachable {
		t.Fatalf("projection dropped the facts the banner needs: %+v", sources[0])
	}
}
