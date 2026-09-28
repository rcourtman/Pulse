package updates

import "testing"

func TestUpdateReleaseArchitectureRequiresMatchingARMVariant(t *testing.T) {
	for _, tc := range []struct {
		goarch, goarm, want string
		ok                  bool
	}{
		{goarch: "amd64", want: "amd64", ok: true},
		{goarch: "arm64", want: "arm64", ok: true},
		{goarch: "386", want: "386", ok: true},
		{goarch: "arm", goarm: "6", want: "armv6", ok: true},
		{goarch: "arm", goarm: "7", want: "armv7", ok: true},
		{goarch: "arm", goarm: "5"},
		{goarch: "arm"},
		{goarch: "arm", goarm: "unknown"},
		{goarch: "riscv64"},
	} {
		got, ok := updateReleaseArchitecture(tc.goarch, tc.goarm)
		if got != tc.want || ok != tc.ok {
			t.Errorf("updateReleaseArchitecture(%q, %q) = (%q, %t), want (%q, %t)", tc.goarch, tc.goarm, got, ok, tc.want, tc.ok)
		}
	}
}
