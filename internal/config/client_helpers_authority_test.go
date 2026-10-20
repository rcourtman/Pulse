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

func TestConfigValidationPreservesEndpointAuthority(t *testing.T) {
	for _, tc := range []struct{ raw, want string }{
		{"HTTPS://pve.example.invalid:8443", "https://pve.example.invalid:8443"},
		{"HTTP://pve.example.invalid:8080", "http://pve.example.invalid:8080"},
		{"HTTPS://[fe80::1%25ZoneA]:8443", "https://[fe80::1%25ZoneA]:8443"},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			cfg := getValidConfig()
			cfg.PVEInstances = []PVEInstance{{Host: tc.raw, TokenName: "fixture@pam!pulse", TokenValue: "synthetic-token", VerifySSL: true, Fingerprint: "fixture-pin"}}
			cfg.PBSInstances = []PBSInstance{{Host: tc.raw, TokenName: "fixture@pam!pulse", TokenValue: "synthetic-token", VerifySSL: true, Fingerprint: "fixture-pin"}}
			if err := cfg.Validate(); err != nil {
				t.Fatal(err)
			}
			if cfg.PVEInstances[0].Host != tc.want || cfg.PBSInstances[0].Host != tc.want {
				t.Error("startup validation changed the configured authority")
			}
			if !cfg.PVEInstances[0].VerifySSL || !cfg.PBSInstances[0].VerifySSL || cfg.PVEInstances[0].TokenValue != "synthetic-token" || cfg.PBSInstances[0].TokenValue != "synthetic-token" || cfg.PVEInstances[0].Fingerprint != "fixture-pin" || cfg.PBSInstances[0].Fingerprint != "fixture-pin" {
				t.Error("startup normalization changed trust or credentials")
			}
		})
	}
	for _, raw := range []string{"ftp://pve.example.invalid:8006", "HTTPS://fixture:synthetic-secret@pve.example.invalid", "https://pve.example.invalid?token=synthetic-secret"} {
		t.Run("invalid-"+raw, func(t *testing.T) {
			cfg := getValidConfig()
			cfg.PVEInstances = []PVEInstance{{Host: raw, Password: "synthetic-password"}}
			if err := cfg.Validate(); err == nil {
				t.Fatal("invalid PVE startup URL admitted")
			}
			cfg.PVEInstances = nil
			cfg.PBSInstances = []PBSInstance{{Host: raw, Password: "synthetic-password"}}
			if err := cfg.Validate(); err != nil {
				t.Fatal(err)
			}
			if len(cfg.PBSInstances) != 1 || cfg.PBSInstances[0].Host != raw {
				t.Error("invalid PBS configuration was silently retargeted or removed")
			}
		})
	}
}
