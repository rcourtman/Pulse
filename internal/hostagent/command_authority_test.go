package hostagent

import "testing"

func TestNormalizeCommandAuthorityProfile(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  CommandAuthorityProfile
	}{
		{input: "", want: CommandAuthorityLegacy},
		{input: "legacy", want: CommandAuthorityLegacy},
		{input: " MONITORING-ONLY ", want: CommandAuthorityMonitoringOnly},
		{input: "command-capable", want: CommandAuthorityCommandCapable},
	} {
		got, err := NormalizeCommandAuthorityProfile(tc.input)
		if err != nil || got != tc.want {
			t.Fatalf("NormalizeCommandAuthorityProfile(%q) = (%q, %v), want %q", tc.input, got, err, tc.want)
		}
	}
	if _, err := NormalizeCommandAuthorityProfile("root"); err == nil {
		t.Fatal("invalid command authority profile was accepted")
	}
}

func TestResolveCommandAuthority(t *testing.T) {
	for _, tc := range []struct {
		name      string
		profile   CommandAuthorityProfile
		desired   bool
		effective bool
		accepted  bool
	}{
		{name: "legacy enable", profile: CommandAuthorityLegacy, desired: true, effective: true, accepted: true},
		{name: "command capable enable", profile: CommandAuthorityCommandCapable, desired: true, effective: true, accepted: true},
		{name: "monitoring disable", profile: CommandAuthorityMonitoringOnly, desired: false, effective: false, accepted: true},
		{name: "monitoring rejects enable", profile: CommandAuthorityMonitoringOnly, desired: true, effective: false, accepted: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			effective, accepted := ResolveCommandAuthority(tc.profile, tc.desired)
			if effective != tc.effective || accepted != tc.accepted {
				t.Fatalf("ResolveCommandAuthority() = (%v, %v), want (%v, %v)", effective, accepted, tc.effective, tc.accepted)
			}
		})
	}
}
