package config

import (
	"testing"
)

func TestConfigHelpersPreserveEndpointAuthority(t *testing.T) {
	for _, tc := range []struct{ name, host, pve, pbs string }{
		{"upper-https", "HTTPS://pve.example.invalid", "https://pve.example.invalid:8006", "https://pve.example.invalid:8007"},
		{"upper-http", "HTTP://pve.example.invalid:8080", "http://pve.example.invalid:8080", "http://pve.example.invalid:8080"},
		{"mixed-https", "hTtPs://pve.example.invalid:8443/api/", "https://pve.example.invalid:8443", "https://pve.example.invalid:8443"},
		{"ipv6", "HTTPS://[2001:db8::1]", "https://[2001:db8::1]:8006", "https://[2001:db8::1]:8007"},
		{"zoned-ipv6", "HTTPS://[fe80::1%25ZoneA]:8443", "https://[fe80::1%25ZoneA]:8443", "https://[fe80::1%25ZoneA]:8443"},
		{"ordinary-bare", "pve.example.invalid", "https://pve.example.invalid:8006", "https://pve.example.invalid:8007"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pve := CreateProxmoxConfig(&PVEInstance{Host: tc.host, VerifySSL: true, Fingerprint: "fixture-pin", TokenName: "fixture", TokenValue: "synthetic-token"})
			pbs := CreatePBSConfig(&PBSInstance{Host: tc.host, VerifySSL: true, Fingerprint: "fixture-pin", TokenName: "fixture", TokenValue: "synthetic-token"})
			pmg := CreatePMGConfig(&PMGInstance{Host: tc.host, VerifySSL: true, Fingerprint: "fixture-pin", TokenName: "fixture", TokenValue: "synthetic-token"})
			if pve.Host != tc.pve || pbs.Host != tc.pbs || pmg.Host != tc.pve {
				t.Errorf("configured authority changed: pve=%q pbs=%q pmg=%q", pve.Host, pbs.Host, pmg.Host)
			}
			if !pve.VerifySSL || !pbs.VerifySSL || !pmg.VerifySSL || pve.Fingerprint != "fixture-pin" || pbs.Fingerprint != "fixture-pin" || pmg.Fingerprint != "fixture-pin" || pve.TokenValue != "synthetic-token" || pbs.TokenValue != "synthetic-token" || pmg.TokenValue != "synthetic-token" {
				t.Error("URL normalization changed trust or credentials")
			}
		})
	}
	for _, host := range []string{"ftp://pve.example.invalid:8006", "ssh://pve.example.invalid:8006", "https://fixture:synthetic-secret@pve.example.invalid:8006", "https://pve.example.invalid:8006?token=synthetic-secret", "https://pve.example.invalid:8006#synthetic-secret"} {
		t.Run("invalid-"+host, func(t *testing.T) {
			if got := normalizeHostPort(host, defaultPVEPort); got != host {
				t.Errorf("invalid endpoint was rewritten to a usable authority: %q", got)
			}
			if got := StripDefaultPort(host, defaultPVEPort); got != host {
				t.Errorf("invalid endpoint acquired a fallback authority: %q", got)
			}
		})
	}
}

func TestStripDefaultPortPreservesIPv6Authority(t *testing.T) {
	for _, tc := range []struct{ host, want string }{
		{"https://[2001:db8::1]:8006", "https://[2001:db8::1]"},
		{"HTTPS://[2001:db8::1]:8006/tenant/", "https://[2001:db8::1]/tenant"},
		{"https://[fe80::1%25ZoneA]:8006/tenant/", "https://[fe80::1%25ZoneA]/tenant"},
		{"http://[::1]:8006", "http://[::1]"},
		{"https://pve.example.invalid:8006/tenant/", "https://pve.example.invalid/tenant"},
		{"https://[::1]:8443", "https://[::1]:8443"},
	} {
		t.Run(tc.host, func(t *testing.T) {
			if got := StripDefaultPort(tc.host, defaultPVEPort); got != tc.want {
				t.Errorf("fallback authority = %q, want %q", got, tc.want)
			}
		})
	}
}
