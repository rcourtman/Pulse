package config

import "testing"

func TestNormalizeHostPort(t *testing.T) {
	tests := []struct {
		name        string
		host        string
		defaultPort string
		want        string
	}{
		{
			name:        "adds default port to https host",
			host:        "https://example.com",
			defaultPort: defaultPVEPort,
			want:        "https://example.com:8006",
		},
		{
			name:        "adds default port when scheme missing",
			host:        "pbs.lan",
			defaultPort: defaultPBSPort,
			want:        "https://pbs.lan:8007",
		},
		{
			name:        "preserves existing port and scheme",
			host:        "http://example.com:8443",
			defaultPort: defaultPVEPort,
			want:        "http://example.com:8443",
		},
		{
			name:        "drops path segments before applying port",
			host:        "https://example.com/api",
			defaultPort: defaultPVEPort,
			want:        "https://example.com:8006",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeHostPort(tt.host, tt.defaultPort); got != tt.want {
				t.Fatalf("normalizeHostPort(%q, %q) = %q, want %q", tt.host, tt.defaultPort, got, tt.want)
			}
		})
	}
}

func TestCreateConfigHelpersNormalizeHosts(t *testing.T) {
	pveCfg := PVEInstance{
		Host: "https://pve.local",
		User: "root@pam",
	}
	pveClientCfg := CreateProxmoxConfig(&pveCfg)
	if pveClientCfg.Host != "https://pve.local:8006" {
		t.Fatalf("expected default port on PVE host, got %q", pveClientCfg.Host)
	}

	pbsCfg := PBSInstance{
		Host: "pbs.local",
		User: "root@pam",
	}
	pbsClientCfg := CreatePBSConfig(&pbsCfg)
	if pbsClientCfg.Host != "https://pbs.local:8007" {
		t.Fatalf("expected default PBS port on host, got %q", pbsClientCfg.Host)
	}

	pmgCfg := PMGInstance{
		Host: "https://pmg.local/api",
		User: "root@pam",
	}
	pmgClientCfg := CreatePMGConfig(&pmgCfg)
	if pmgClientCfg.Host != "https://pmg.local:8006" {
		t.Fatalf("expected default PMG port on host, got %q", pmgClientCfg.Host)
	}

	fromFields := CreateProxmoxConfigFromFields("https://cluster.local", "root@pam", "", "token!name", "value", "", true)
	if fromFields.Host != "https://cluster.local:8006" {
		t.Fatalf("expected default port on host built from fields, got %q", fromFields.Host)
	}
}
