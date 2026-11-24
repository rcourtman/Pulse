package securityutil

import (
	"net"
	"testing"
)

func TestIsSpecialUseInternalIP(t *testing.T) {
	for _, tc := range []struct {
		ip   string
		want bool
	}{
		{"0.0.0.0", true},
		{"0.255.255.255", true},
		{"100.64.0.0", true},
		{"100.100.100.200", true},
		{"100.127.255.255", true},
		{"192.0.0.0", true},
		{"192.0.0.192", true},
		{"198.18.0.0", true},
		{"198.19.255.255", true},
		{"240.0.0.0", true},
		{"255.255.255.255", true},
		{"::ffff:100.64.0.1", true},
		// Neighbouring public space, and ranges other predicates own.
		{"1.0.0.0", false},
		{"100.63.255.255", false},
		{"100.128.0.0", false},
		{"192.0.1.0", false},
		{"192.0.2.1", false},
		{"198.17.255.255", false},
		{"198.20.0.0", false},
		{"239.255.255.255", false},
		{"10.0.0.1", false},
		{"127.0.0.1", false},
		{"8.8.8.8", false},
		{"2001:db8::1", false},
		{"::", false},
	} {
		if got := IsSpecialUseInternalIP(net.ParseIP(tc.ip)); got != tc.want {
			t.Errorf("IsSpecialUseInternalIP(%s) = %v, want %v", tc.ip, got, tc.want)
		}
	}
	if IsSpecialUseInternalIP(nil) {
		t.Error("IsSpecialUseInternalIP(nil) = true, want false")
	}
}
