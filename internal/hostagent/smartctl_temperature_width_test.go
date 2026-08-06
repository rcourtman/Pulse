package hostagent

import "testing"

// SMART raw attribute values are 64-bit, but DiskSMART.Temperature is an int,
// which is 32 bits wide on the 386 and arm release builds. Range-checking after
// the narrowing conversion let a raw value whose low 32 bits landed in the
// plausible band pass as a real temperature. validSMARTTemperature64 is the
// gate that must run first, so it is pinned here rather than through the parse
// path: on a 64-bit test host the conversion is lossless and the end-to-end
// case cannot distinguish the fixed code from the broken code.
func TestValidSMARTTemperature64RejectsValuesThatTruncateIntoRange(t *testing.T) {
	cases := []struct {
		name  string
		value int64
		want  bool
	}{
		{name: "plausible reading", value: 38, want: true},
		{name: "lower bound excluded", value: 0, want: false},
		{name: "upper bound excluded", value: 150, want: false},
		{name: "negative", value: -5, want: false},

		// Each of these has low 32 bits equal to a plausible temperature.
		{name: "2^32 plus 20", value: 1<<32 + 20, want: false},
		{name: "2^32 plus 45", value: 1<<32 + 45, want: false},
		{name: "2^40 plus 38", value: 1<<40 + 38, want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := validSMARTTemperature64(tc.value); got != tc.want {
				t.Fatalf("validSMARTTemperature64(%d) = %v, want %v", tc.value, got, tc.want)
			}
		})
	}

	// The int wrapper must stay consistent with the 64-bit gate.
	for _, value := range []int{0, 1, 38, 149, 150, -5} {
		if validSMARTTemperature(value) != validSMARTTemperature64(int64(value)) {
			t.Fatalf("validSMARTTemperature(%d) disagrees with the 64-bit gate", value)
		}
	}
}
