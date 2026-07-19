package truenas

import "testing"

// TestAvailableAppLogContainersBranchCov exercises every branch of
// availableAppLogContainers: the empty-input short circuit, the
// post-filter empty fallback, the ServiceName→ID fallback, the
// skip-when-both-empty filter, the case-insensitive ID-equality guard
// that suppresses the parenthesised ID, the ID-in-parens formatting,
// whitespace trimming, and the multi-container join.
func TestAvailableAppLogContainersBranchCov(t *testing.T) {
	tests := []struct {
		name       string
		containers []AppContainer
		want       string
	}{
		{
			name:       "empty slice returns empty string",
			containers: nil,
			want:       "",
		},
		{
			name: "all containers filtered out returns empty string",
			containers: []AppContainer{
				{ServiceName: "   ", ID: ""},
				{ServiceName: "", ID: "  "},
			},
			want: "",
		},
		{
			name: "service name only uses service name without parens",
			containers: []AppContainer{
				{ServiceName: "svc-a", ID: ""},
			},
			want: "svc-a",
		},
		{
			name: "id only falls back to id without parens",
			containers: []AppContainer{
				{ServiceName: "", ID: "id-1"},
			},
			want: "id-1",
		},
		{
			name: "service name equal to id case-insensitive suppresses parens",
			containers: []AppContainer{
				{ServiceName: "svc", ID: "SVC"},
			},
			want: "svc",
		},
		{
			name: "service name with differing id appends id in parens",
			containers: []AppContainer{
				{ServiceName: "svc-a", ID: "id-1"},
			},
			want: "svc-a (id-1)",
		},
		{
			name: "leading and trailing whitespace is trimmed before formatting",
			containers: []AppContainer{
				{ServiceName: "  padded-svc  ", ID: "  padded-id  "},
			},
			want: "padded-svc (padded-id)",
		},
		{
			name: "trimmed service name matching trimmed id suppresses parens",
			containers: []AppContainer{
				{ServiceName: "  Name  ", ID: "  name  "},
			},
			want: "Name",
		},
		{
			name: "multiple containers are comma joined in order",
			containers: []AppContainer{
				{ServiceName: "svc-a", ID: "id-a"},
				{ServiceName: "", ID: "id-only"},
				{ServiceName: "svc-c", ID: ""},
				{ServiceName: "  ", ID: "  "},
				{ServiceName: "Same", ID: "same"},
			},
			want: "svc-a (id-a), id-only, svc-c, Same",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := availableAppLogContainers(tt.containers)
			if got != tt.want {
				t.Fatalf("availableAppLogContainers(%+v) = %q, want %q", tt.containers, got, tt.want)
			}
		})
	}
}
