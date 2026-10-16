package notifications

import (
	"net"
	"testing"
)

// An IPv6 transition address carries an IPv4 destination inside it, so a
// NAT64 gateway delivers 64:ff9b::a9fe:a9fe to the cloud metadata service at
// 169.254.169.254. The webhook guard must judge the embedded destination, not
// just the outer address.
func TestIsPrivateIPCatchesTransitionAddresses(t *testing.T) {
	for _, tc := range []struct {
		name string
		ip   string
		want bool
	}{
		{"NAT64 metadata service", "64:ff9b::a9fe:a9fe", true},
		{"NAT64 loopback", "64:ff9b::7f00:1", true},
		{"NAT64 local-use prefix RFC1918", "64:ff9b:1::a00:1", true},
		{"6to4 loopback", "2002:7f00:1::", true},
		{"6to4 RFC1918", "2002:c0a8:101::1", true},
		{"Teredo server RFC1918", "2001:0:a00:1::1", true},
		{"NAT64 public destination", "64:ff9b::808:808", false},
		{"public IPv6", "2607:f8b0::1", false},
		{"public IPv4", "8.8.8.8", false},
		{"plain loopback", "127.0.0.1", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ip := net.ParseIP(tc.ip)
			if ip == nil {
				t.Fatalf("parse %q", tc.ip)
			}
			if got := isPrivateIP(ip); got != tc.want {
				t.Fatalf("isPrivateIP(%s) = %v, want %v", tc.ip, got, tc.want)
			}
		})
	}
}

func TestValidateWebhookURLRejectsNAT64MetadataAddress(t *testing.T) {
	nm := NewNotificationManager("")
	if err := nm.ValidateWebhookURL("http://[64:ff9b::a9fe:a9fe]/latest/meta-data/"); err == nil {
		t.Fatal("a NAT64-encoded metadata address passed webhook validation")
	}
}
