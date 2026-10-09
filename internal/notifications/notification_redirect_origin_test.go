package notifications

import (
	"net/url"
	"testing"
)

func TestNotificationOriginComparison(t *testing.T) {
	for _, tc := range []struct {
		name, initial, target string
		same                  bool
	}{
		{"path-query-fragment", "https://receiver.test/start", "https://receiver.test/next?key=fixture#section", true},
		{"dns-case", "https://RECEIVER.test/start", "https://receiver.TEST/next", true},
		{"http-default-port", "http://receiver.test", "http://receiver.test:80/next", true},
		{"https-default-port", "https://receiver.test:443/start", "https://receiver.test/next", true},
		{"decimal-port", "http://receiver.test:080/start", "http://receiver.test:80/next", true},
		{"ipv6-spelling", "https://[2001:DB8:0:0::1]/start", "https://[2001:db8::1]:443/next", true},
		{"ipv6-zone", "http://[fe80::1%25eth0]/start", "http://[fe80::1%25eth0]:80/next", true},
		{"different-zone", "http://[fe80::1%25eth0]", "http://[fe80::1%25ETH0]", false},
		{"different-host", "https://receiver.test", "https://other.test", false},
		{"subdomain", "https://receiver.test", "https://child.receiver.test", false},
		{"suffix-host", "https://receiver.test", "https://receiver.test.other.test", false},
		{"different-port", "https://receiver.test", "https://receiver.test:444", false},
		{"downgrade", "https://receiver.test:443", "http://receiver.test:443", false},
		{"upgrade", "http://receiver.test", "https://receiver.test", false},
		{"dns-root-dot", "https://receiver.test", "https://receiver.test.", false},
		{"userinfo", "https://receiver.test", "https://fixture@receiver.test", false},
		{"relative-target", "https://receiver.test", "/next", false},
		{"non-http", "https://receiver.test", "ftp://receiver.test", false},
		{"port-zero", "https://receiver.test", "https://receiver.test:0", false},
		{"port-overflow", "https://receiver.test", "https://receiver.test:65536", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			initial, err := url.Parse(tc.initial)
			if err != nil {
				t.Fatal(err)
			}
			target, err := url.Parse(tc.target)
			if err != nil {
				t.Fatal(err)
			}
			if sameNotificationOrigin(initial, target) != tc.same || sameNotificationOrigin(target, initial) != tc.same {
				t.Errorf("same-origin comparison = %t, want %t", !tc.same, tc.same)
			}
		})
	}
	if sameNotificationOrigin(nil, &url.URL{}) || sameNotificationOrigin(&url.URL{}, nil) || sameNotificationOrigin(&url.URL{}, &url.URL{}) {
		t.Fatal("missing origin was admitted")
	}
}
