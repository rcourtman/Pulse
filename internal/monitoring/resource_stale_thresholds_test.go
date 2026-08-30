package monitoring

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestResourceStaleThresholdsDeriveMockSupplementalCadence(t *testing.T) {
	supplementalSources := []unifiedresources.DataSource{
		unifiedresources.SourceTrueNAS,
		unifiedresources.SourceVMware,
		unifiedresources.SourceAvailability,
	}

	t.Run("mock disabled leaves supplemental sources on registry defaults", func(t *testing.T) {
		thresholds := resourceStaleThresholdsForConfig(nil, false, func() time.Duration {
			t.Fatal("supplemental cadence must not be consulted while mock mode is off")
			return 0
		})
		for _, source := range supplementalSources {
			if _, ok := thresholds[source]; ok {
				t.Fatalf("unexpected %s threshold override with mock disabled", source)
			}
		}
	})

	t.Run("fast mock cadence keeps the default platform threshold", func(t *testing.T) {
		thresholds := resourceStaleThresholdsForConfig(nil, true, func() time.Duration {
			return 20 * time.Second
		})
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
		thresholds := resourceStaleThresholdsForConfig(nil, true, func() time.Duration {
			return 300 * time.Second
		})
		want := 600 * time.Second
		for _, source := range supplementalSources {
			if got := thresholds[source]; got != want {
				t.Fatalf("%s threshold = %s, want %s", source, got, want)
			}
		}
	})

	t.Run("supplemental derivation leaves polled sources untouched", func(t *testing.T) {
		base := resourceStaleThresholdsForConfig(nil, false, nil)
		derived := resourceStaleThresholdsForConfig(nil, true, func() time.Duration {
			return 300 * time.Second
		})
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
