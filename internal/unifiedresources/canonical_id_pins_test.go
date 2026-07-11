package unifiedresources

import "testing"

func TestIdentityPinIndexFindsPreservedDottedHostname(t *testing.T) {
	index := newIdentityPinIndex([]ResourceIdentityPin{{
		CanonicalID:  "agent-cloud-rnd",
		ResourceType: ResourceTypeAgent,
		MachineID:    "machine-rnd",
		Hostname:     "Cloud.Rnd-Lax1.",
	}})

	for _, hostname := range []string{"cloud.rnd-lax1", "cloud"} {
		pin, ok := index.find(ResourceIdentity{Hostnames: []string{hostname}})
		if !ok {
			t.Fatalf("hostname %q did not resolve its identity pin", hostname)
		}
		if pin.MachineID != "machine-rnd" {
			t.Fatalf("hostname %q resolved machine ID %q, want %q", hostname, pin.MachineID, "machine-rnd")
		}
	}
}

func TestIdentityPinIndexKeepsSharedShortHostnameAliasAmbiguous(t *testing.T) {
	index := newIdentityPinIndex([]ResourceIdentityPin{
		{
			CanonicalID:  "agent-cloud-rnd",
			ResourceType: ResourceTypeAgent,
			MachineID:    "machine-rnd",
			ClusterName:  "homelab",
			Hostname:     "cloud.rnd-lax1",
		},
		{
			CanonicalID:  "agent-cloud-gce",
			ResourceType: ResourceTypeAgent,
			MachineID:    "machine-gce",
			ClusterName:  "homelab",
			Hostname:     "cloud.gce-or1",
		},
	})

	for hostname, wantMachineID := range map[string]string{
		"cloud.rnd-lax1": "machine-rnd",
		"cloud.gce-or1":  "machine-gce",
	} {
		pin, ok := index.find(ResourceIdentity{
			ClusterName: "homelab",
			Hostnames:   []string{hostname},
		})
		if !ok {
			t.Fatalf("exact hostname %q did not resolve its identity pin", hostname)
		}
		if pin.MachineID != wantMachineID {
			t.Fatalf("exact hostname %q resolved machine ID %q, want %q", hostname, pin.MachineID, wantMachineID)
		}
	}

	if pin, ok := index.find(ResourceIdentity{
		ClusterName: "homelab",
		Hostnames:   []string{"cloud"},
	}); ok {
		t.Fatalf("ambiguous short hostname resolved pin %+v", pin)
	}
}
