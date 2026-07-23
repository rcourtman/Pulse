package servicediscovery

import (
	"testing"
)

// TestBranchcov0723pmNeedsDeepScan_Branches exercises every return arm of
// (*Service).needsDeepScan (service.go:525). needsDeepScan is a pure
// predicate: it never reads receiver state, so a zero-value &Service{} is
// sufficient and no network/SSH/daemon/database rig is required.
//
// Arms covered:
//   - nil discovery                                  -> return true
//   - non-empty RawCommandOutput (short-circuit)     -> return false
//   - Confidence < 0.7                               -> return true
//   - Confidence == 0.7 boundary (skips conf arm)    -> falls through
//   - ServiceType == ""                              -> return true
//   - ServiceType == "unknown"                       -> return true
//   - all of Facts/ConfigPaths/LogPaths empty        -> return true
//   - final fall-through (at least one path present) -> return false
func TestBranchcov0723pmNeedsDeepScan_Branches(t *testing.T) {
	s := &Service{} // zero-value receiver; needsDeepScan never dereferences s

	cases := []struct {
		name      string
		discovery *ResourceDiscovery
		want      bool
	}{
		{
			name:      "nil-discovery-returns-true",
			discovery: nil,
			want:      true,
		},
		{
			// RawCommandOutput short-circuits before the confidence and
			// service-type checks, so even a zero-confidence, empty-type
			// discovery must return false here.
			name: "raw-command-output-present-short-circuits-to-false",
			discovery: &ResourceDiscovery{
				RawCommandOutput: map[string]string{"uname -a": "Linux node 6.1.0"},
				Confidence:       0.0,
				ServiceType:      "",
			},
			want: false,
		},
		{
			name: "confidence-well-below-threshold-returns-true",
			discovery: &ResourceDiscovery{
				Confidence:  0.5,
				ServiceType: "postgres",
			},
			want: true,
		},
		{
			name: "confidence-just-below-threshold-returns-true",
			// 0.69 as a float64 is strictly less than the 0.7 literal used
			// in the source, so the confidence arm fires.
			discovery: &ResourceDiscovery{
				Confidence:  0.69,
				ServiceType: "postgres",
			},
			want: true,
		},
		{
			// Boundary: Confidence == 0.7 is NOT < 0.7, so the confidence
			// arm is skipped. With a known service type and a non-empty
			// path, every other guard is also cleared and the function
			// reaches the final `return false`.
			name: "confidence-exactly-at-threshold-skips-confidence-arm",
			discovery: &ResourceDiscovery{
				Confidence:  0.7,
				ServiceType: "postgres",
				ConfigPaths: []string{"/etc/postgresql/15/main/postgresql.conf"},
			},
			want: false,
		},
		{
			// Boundary corroboration: Confidence == 0.7 skips the confidence
			// arm, then the empty ServiceType arm fires. If 0.7 were treated
			// as < 0.7 this would be indistinguishable; pairing it with the
			// previous case pins the boundary precisely.
			name: "confidence-exactly-at-threshold-then-empty-service-type-returns-true",
			discovery: &ResourceDiscovery{
				Confidence:  0.7,
				ServiceType: "",
			},
			want: true,
		},
		{
			name: "empty-service-type-returns-true",
			discovery: &ResourceDiscovery{
				Confidence:  0.9,
				ServiceType: "",
			},
			want: true,
		},
		{
			name: "literal-unknown-service-type-returns-true",
			discovery: &ResourceDiscovery{
				Confidence:  0.9,
				ServiceType: "unknown",
			},
			want: true,
		},
		{
			// Case-sensitivity check: "Unknown" != "unknown", so the
			// service-type guard is cleared and with a path present the
			// function falls through to false. This is an observable
			// behaviour, not a restatement of the comparison.
			name: "capitalized-unknown-is-not-unknown-and-falls-through",
			discovery: &ResourceDiscovery{
				Confidence:  0.9,
				ServiceType: "Unknown",
				ConfigPaths: []string{"/etc/foo.conf"},
			},
			want: false,
		},
		{
			name: "all-paths-empty-returns-true",
			discovery: &ResourceDiscovery{
				Confidence:  0.9,
				ServiceType: "postgres",
				// Facts, ConfigPaths, LogPaths all zero-length
			},
			want: true,
		},
		{
			name: "only-facts-non-empty-returns-false",
			discovery: &ResourceDiscovery{
				Confidence:  0.9,
				ServiceType: "postgres",
				Facts:       []DiscoveryFact{{Key: "arch", Value: "amd64"}},
			},
			want: false,
		},
		{
			name: "only-configpaths-non-empty-returns-false",
			discovery: &ResourceDiscovery{
				Confidence:  0.9,
				ServiceType: "postgres",
				ConfigPaths: []string{"/etc/postgresql/postgresql.conf"},
			},
			want: false,
		},
		{
			name: "only-logpaths-non-empty-returns-false",
			discovery: &ResourceDiscovery{
				Confidence:  0.9,
				ServiceType: "postgres",
				LogPaths:    []string{"/var/log/postgresql/postgresql.log"},
			},
			want: false,
		},
		{
			name: "all-three-paths-non-empty-returns-false",
			discovery: &ResourceDiscovery{
				Confidence:  0.9,
				ServiceType: "postgres",
				Facts:       []DiscoveryFact{{Key: "arch", Value: "amd64"}},
				ConfigPaths: []string{"/etc/postgresql/postgresql.conf"},
				LogPaths:    []string{"/var/log/postgresql/postgresql.log"},
			},
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := s.needsDeepScan(tc.discovery)
			if got != tc.want {
				t.Fatalf("needsDeepScan(%+v) = %v, want %v", tc.discovery, got, tc.want)
			}
		})
	}
}

// TestBranchcov0723pmNeedsDeepScan_NilReceiverPurity asserts the purity
// guarantee that needsDeepScan never dereferences its receiver: a nil *Service
// must not panic and must behave identically to a zero-value receiver. This is
// an observable property of the implementation, not a restatement of it.
func TestBranchcov0723pmNeedsDeepScan_NilReceiverPurity(t *testing.T) {
	var s *Service // nil receiver

	t.Run("nil-receiver-nil-discovery-returns-true", func(t *testing.T) {
		if !s.needsDeepScan(nil) {
			t.Fatal("nil receiver + nil discovery must return true")
		}
	})

	t.Run("nil-receiver-healthy-discovery-returns-false", func(t *testing.T) {
		d := &ResourceDiscovery{
			Confidence:  0.95,
			ServiceType: "redis",
			ConfigPaths: []string{"/etc/redis/redis.conf"},
		}
		if s.needsDeepScan(d) {
			t.Fatal("nil receiver + healthy discovery must return false")
		}
	})
}
