package monitoring

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestResourceStaleThresholdsDeriveMockSupplementalCadence(t *testing.T) {
	supplementalSources := []unifiedresources.DataSource{
		unifiedresources.SourceTrueNAS,
		unifiedresources.SourceVMware,
		unifiedresources.SourceAvailability,
	}

	t.Run("mock disabled leaves supplemental sources on registry defaults", func(t *testing.T) {
		thresholds := resourceStaleThresholdsForConfig(nil, false, mockResourceCadences{supplemental: func() time.Duration {
			t.Fatal("supplemental cadence must not be consulted while mock mode is off")
			return 0
		}})
		for _, source := range supplementalSources {
			if _, ok := thresholds[source]; ok {
				t.Fatalf("unexpected %s threshold override with mock disabled", source)
			}
		}
	})

	t.Run("fast mock cadence keeps the default platform threshold", func(t *testing.T) {
		thresholds := resourceStaleThresholdsForConfig(nil, true, mockResourceCadences{supplemental: func() time.Duration {
			return 20 * time.Second
		}})
		for _, source := range supplementalSources {
			if got := thresholds[source]; got != defaultPlatformResourceStaleThreshold {
				t.Fatalf("%s threshold = %s, want default %s", source, got, defaultPlatformResourceStaleThreshold)
			}
		}
	})

	// The public demo runs PULSE_MOCK_UPDATE_INTERVAL well above the default;
	// a fixed 120s threshold there flagged every TrueNAS/VMware/availability
	// row as a stale source for most of each refresh cycle.
	t.Run("slow mock cadence widens supplemental thresholds", func(t *testing.T) {
		thresholds := resourceStaleThresholdsForConfig(nil, true, mockResourceCadences{supplemental: func() time.Duration {
			return 300 * time.Second
		}})
		want := 600 * time.Second
		for _, source := range supplementalSources {
			if got := thresholds[source]; got != want {
				t.Fatalf("%s threshold = %s, want %s", source, got, want)
			}
		}
	})

	t.Run("supplemental derivation leaves polled sources untouched", func(t *testing.T) {
		base := resourceStaleThresholdsForConfig(nil, false, mockResourceCadences{})
		derived := resourceStaleThresholdsForConfig(nil, true, mockResourceCadences{supplemental: func() time.Duration {
			return 300 * time.Second
		}})
		for _, source := range []unifiedresources.DataSource{
			unifiedresources.SourceProxmox,
			unifiedresources.SourcePBS,
			unifiedresources.SourcePMG,
		} {
			if base[source] != derived[source] {
				t.Fatalf("%s threshold changed with mock enabled: %s -> %s", source, base[source], derived[source])
			}
		}
	})
}

func TestResourceStaleThresholdsDeriveMockProxmoxCadence(t *testing.T) {
	proxmoxCadence := func(cadence time.Duration) mockResourceCadences {
		return mockResourceCadences{proxmox: func() time.Duration { return cadence }}
	}
	configured := resourceStaleThresholdsForConfig(nil, false, mockResourceCadences{})[unifiedresources.SourceProxmox]

	t.Run("mock disabled ignores the mock Proxmox cadence", func(t *testing.T) {
		thresholds := resourceStaleThresholdsForConfig(nil, false, mockResourceCadences{proxmox: func() time.Duration {
			t.Fatal("Proxmox cadence must not be consulted while mock mode is off")
			return 0
		}})
		if got := thresholds[unifiedresources.SourceProxmox]; got != configured {
			t.Fatalf("Proxmox threshold = %s with mock disabled, want configured %s", got, configured)
		}
	})

	// The public demo's 15s tick sights PVE guests every 30s, inside the
	// configured threshold, so the threshold must not widen.
	t.Run("cadence inside the budget keeps the configured threshold", func(t *testing.T) {
		if got := resourceStaleThresholdsForConfig(nil, true, proxmoxCadence(30*time.Second))[unifiedresources.SourceProxmox]; got != configured {
			t.Fatalf("Proxmox threshold = %s, want configured %s", got, configured)
		}
	})

	// A tick slower than the budget sights every guest once per tick; a 60s
	// tick judged by the 60s threshold flagged every guest stale on every
	// tick boundary.
	t.Run("slow tick derives the threshold from the sighting cadence", func(t *testing.T) {
		if got, want := resourceStaleThresholdsForConfig(nil, true, proxmoxCadence(60*time.Second))[unifiedresources.SourceProxmox], 120*time.Second; got != want {
			t.Fatalf("Proxmox threshold = %s, want %s", got, want)
		}
	})

	t.Run("mock cadence never narrows a slower configured PVE interval", func(t *testing.T) {
		cfg := &config.Config{PVEPollingInterval: 5 * time.Minute}
		if got, want := resourceStaleThresholdsForConfig(cfg, true, proxmoxCadence(20*time.Second))[unifiedresources.SourceProxmox], 10*time.Minute; got != want {
			t.Fatalf("Proxmox threshold = %s, want %s", got, want)
		}
	})
}
