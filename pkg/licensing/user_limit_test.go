package licensing

import "testing"

func TestMaxUsersLimitFromLimits(t *testing.T) {
	tests := []struct {
		name   string
		limits map[string]int64
		want   int
	}{
		{name: "nil map", limits: nil, want: 0},
		{name: "missing key", limits: map[string]int64{"max_agents": 10}, want: 0},
		{name: "non-positive value", limits: map[string]int64{MaxUsersLicenseGateKey: 0}, want: 0},
		{name: "positive value", limits: map[string]int64{MaxUsersLicenseGateKey: 25}, want: 25},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MaxUsersLimitFromLimits(tt.limits); got != tt.want {
				t.Fatalf("expected %d, got %d", tt.want, got)
			}
		})
	}
}

func TestMaxUsersLimitFromLicense(t *testing.T) {
	tests := []struct {
		name string
		lic  *License
		want int
	}{
		{name: "nil license", lic: nil, want: 0},
		{
			name: "missing limit",
			lic: &License{
				Claims: Claims{
					Limits: map[string]int64{"max_agents": 10},
				},
			},
			want: 0,
		},
		{
			name: "with max users",
			lic: &License{
				Claims: Claims{
					Limits: map[string]int64{MaxUsersLicenseGateKey: 7},
				},
			},
			want: 7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MaxUsersLimitFromLicense(tt.lic); got != tt.want {
				t.Fatalf("expected %d, got %d", tt.want, got)
			}
		})
	}
}

func TestExceedsUserLimit(t *testing.T) {
	tests := []struct {
		name      string
		current   int
		additions int
		limit     int
		want      bool
	}{
		{name: "unlimited", current: 100, additions: 1, limit: 0, want: false},
		{name: "no additions", current: 5, additions: 0, limit: 5, want: false},
		{name: "within limit", current: 4, additions: 1, limit: 5, want: false},
		{name: "at limit", current: 5, additions: 1, limit: 5, want: true},
		{name: "over limit", current: 6, additions: 1, limit: 5, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExceedsUserLimit(tt.current, tt.additions, tt.limit); got != tt.want {
				t.Fatalf("expected %t, got %t", tt.want, got)
			}
		})
	}
}

func TestUserLimitExceededMessage(t *testing.T) {
	got := UserLimitExceededMessage(6, 5)
	want := "User limit reached (6/5). Remove a member or upgrade your license."
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}
