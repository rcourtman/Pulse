package servicediscovery

import (
	"strings"
	"testing"
)

func TestSuggestWebURL(t *testing.T) {
	tests := []struct {
		name      string
		discovery *ResourceDiscovery
		hostIP    string
		wantURL   string
		wantEmpty bool
	}{
		{
			name: "jellyfin with known service type",
			discovery: &ResourceDiscovery{
				ServiceType: "jellyfin",
				Category:    CategoryMedia,
			},
			hostIP:  "192.168.1.50",
			wantURL: "http://192.168.1.50:8096",
		},
		{
			name: "plex with path",
			discovery: &ResourceDiscovery{
				ServiceType: "plex",
				Category:    CategoryMedia,
			},
			hostIP:  "192.168.1.50",
			wantURL: "http://192.168.1.50:32400/web",
		},
		{
			name: "proxmox with https",
			discovery: &ResourceDiscovery{
				ServiceType: "proxmox",
				Category:    CategoryVirtualizer,
			},
			hostIP:  "192.168.1.10",
			wantURL: "https://192.168.1.10:8006",
		},
		{
			name: "home-assistant",
			discovery: &ResourceDiscovery{
				ServiceType: "home-assistant",
				Category:    CategoryHomeAuto,
			},
			hostIP:  "192.168.1.100",
			wantURL: "http://192.168.1.100:8123",
		},
		{
			name: "service type with underscores (normalized)",
			discovery: &ResourceDiscovery{
				ServiceType: "home_assistant",
				Category:    CategoryHomeAuto,
			},
			hostIP:  "192.168.1.100",
			wantURL: "http://192.168.1.100:8123",
		},
		{
			name: "traefik with path",
			discovery: &ResourceDiscovery{
				ServiceType: "traefik",
				Category:    CategoryWebServer,
			},
			hostIP:  "10.0.0.5",
			wantURL: "http://10.0.0.5:8080/dashboard/",
		},
		{
			name: "pihole with path",
			discovery: &ResourceDiscovery{
				ServiceType: "pihole",
				Category:    CategoryNetwork,
			},
			hostIP:  "192.168.1.1",
			wantURL: "http://192.168.1.1/admin",
		},
		{
			name: "unknown service with web port discovered",
			discovery: &ResourceDiscovery{
				ServiceType: "myapp",
				Category:    CategoryUnknown,
				Ports: []PortInfo{
					{Port: 8080, Protocol: "tcp"},
				},
			},
			hostIP:  "192.168.1.50",
			wantURL: "http://192.168.1.50:8080",
		},
		{
			name: "unknown service with https port",
			discovery: &ResourceDiscovery{
				ServiceType: "myapp",
				Category:    CategoryUnknown,
				Ports: []PortInfo{
					{Port: 443, Protocol: "tcp"},
				},
			},
			hostIP:  "192.168.1.50",
			wantURL: "https://192.168.1.50",
		},
		{
			name: "database - no web UI expected",
			discovery: &ResourceDiscovery{
				ServiceType: "postgres",
				Category:    CategoryDatabase,
				Ports: []PortInfo{
					{Port: 5432, Protocol: "tcp"},
				},
			},
			hostIP:    "192.168.1.50",
			wantEmpty: true,
		},
		{
			name: "cache service - no web UI expected",
			discovery: &ResourceDiscovery{
				ServiceType: "redis",
				Category:    CategoryCache,
			},
			hostIP:    "192.168.1.50",
			wantEmpty: true,
		},
		{
			name:      "nil discovery",
			discovery: nil,
			hostIP:    "192.168.1.50",
			wantEmpty: true,
		},
		{
			name: "empty host IP",
			discovery: &ResourceDiscovery{
				ServiceType: "jellyfin",
				Category:    CategoryMedia,
			},
			hostIP:    "",
			wantEmpty: true,
		},
		{
			name: "frigate NVR",
			discovery: &ResourceDiscovery{
				ServiceType: "frigate",
				Category:    CategoryNVR,
			},
			hostIP:  "192.168.1.200",
			wantURL: "http://192.168.1.200:5000",
		},
		{
			name: "grafana monitoring",
			discovery: &ResourceDiscovery{
				ServiceType: "grafana",
				Category:    CategoryMonitoring,
			},
			hostIP:  "10.0.0.10",
			wantURL: "http://10.0.0.10:3000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SuggestWebURL(tt.discovery, tt.hostIP)
			if tt.wantEmpty {
				if got != "" {
					t.Errorf("SuggestWebURL() = %q, want empty string", got)
				}
			} else if got != tt.wantURL {
				t.Errorf("SuggestWebURL() = %q, want %q", got, tt.wantURL)
			}
		})
	}
}

func TestBuildURL(t *testing.T) {
	tests := []struct {
		protocol string
		host     string
		port     int
		path     string
		want     string
	}{
		{"http", "192.168.1.1", 8080, "", "http://192.168.1.1:8080"},
		{"https", "192.168.1.1", 443, "", "https://192.168.1.1"},
		{"http", "192.168.1.1", 80, "", "http://192.168.1.1"},
		{"http", "192.168.1.1", 80, "/admin", "http://192.168.1.1/admin"},
		{"https", "example.com", 8443, "/dashboard/", "https://example.com:8443/dashboard/"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := buildURL(tt.protocol, tt.host, tt.port, tt.path)
			if got != tt.want {
				t.Errorf("buildURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsCommonWebPort(t *testing.T) {
	webPorts := []int{80, 443, 8080, 8443, 3000, 5000, 8000, 8888, 9000}
	nonWebPorts := []int{22, 25, 3306, 5432, 6379, 27017}

	for _, port := range webPorts {
		if !isCommonWebPort(port) {
			t.Errorf("isCommonWebPort(%d) = false, want true", port)
		}
	}

	for _, port := range nonWebPorts {
		if isCommonWebPort(port) {
			t.Errorf("isCommonWebPort(%d) = true, want false", port)
		}
	}
}

func TestSuggestWebURLWithReason(t *testing.T) {
	tests := []struct {
		name      string
		discovery *ResourceDiscovery
		hostIP    string
		wantCode  string
	}{
		{
			name:      "nil discovery",
			discovery: nil,
			hostIP:    "192.168.1.10",
			wantCode:  urlReasonNoDiscovery,
		},
		{
			name: "missing host",
			discovery: &ResourceDiscovery{
				ServiceType: "proxmox",
				Category:    CategoryVirtualizer,
			},
			hostIP:   "",
			wantCode: urlReasonNoHost,
		},
		{
			name: "non-web category",
			discovery: &ResourceDiscovery{
				ServiceType: "postgres",
				Category:    CategoryDatabase,
			},
			hostIP:   "192.168.1.20",
			wantCode: urlReasonCategoryNotWeb,
		},
		{
			name: "unknown service with no ports",
			discovery: &ResourceDiscovery{
				ServiceType: "unknown",
				Category:    CategoryUnknown,
			},
			hostIP:   "192.168.1.30",
			wantCode: urlReasonNoPortsDetected,
		},
		{
			name: "ports present but no common web ports",
			discovery: &ResourceDiscovery{
				ServiceType: "custom",
				Category:    CategoryUnknown,
				Ports: []PortInfo{
					{Port: 22, Protocol: "tcp"},
					{Port: 3306, Protocol: "tcp"},
				},
			},
			hostIP:   "192.168.1.40",
			wantCode: urlReasonNoCommonWebPort,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url, code, detail := suggestWebURLWithReason(tt.discovery, tt.hostIP)
			if url != "" {
				t.Fatalf("expected empty URL, got %q", url)
			}
			if code != tt.wantCode {
				t.Fatalf("unexpected reason code: got %q want %q", code, tt.wantCode)
			}
			if detail == "" {
				t.Fatalf("expected non-empty reason detail")
			}
		})
	}
}

func TestSuggestWebURLWithReason_SuccessCases(t *testing.T) {
	tests := []struct {
		name         string
		discovery    *ResourceDiscovery
		hostIP       string
		wantURL      string
		wantCode     string
		detailSubstr string
	}{
		{
			name: "service default match",
			discovery: &ResourceDiscovery{
				ServiceType: "proxmox",
				Category:    CategoryVirtualizer,
			},
			hostIP:       "192.168.1.10",
			wantURL:      "https://192.168.1.10:8006",
			wantCode:     urlReasonServiceDefaultMatch,
			detailSubstr: "service default",
		},
		{
			name: "service variation match",
			discovery: &ResourceDiscovery{
				ServiceType: "homeassistant-server",
				Category:    CategoryHomeAuto,
			},
			hostIP:       "192.168.1.20",
			wantURL:      "http://192.168.1.20:8123",
			wantCode:     urlReasonServiceVariation,
			detailSubstr: "normalized match",
		},
		{
			name: "web port inference",
			discovery: &ResourceDiscovery{
				ServiceType: "custom",
				Category:    CategoryUnknown,
				Ports: []PortInfo{
					{Port: 8443, Protocol: "tcp"},
				},
			},
			hostIP:       "192.168.1.30",
			wantURL:      "https://192.168.1.30:8443",
			wantCode:     urlReasonWebPortInference,
			detailSubstr: "web port 8443",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotURL, gotCode, gotDetail := suggestWebURLWithReason(tt.discovery, tt.hostIP)
			if gotURL != tt.wantURL {
				t.Fatalf("unexpected URL. got %q want %q", gotURL, tt.wantURL)
			}
			if gotCode != tt.wantCode {
				t.Fatalf("unexpected reason code. got %q want %q", gotCode, tt.wantCode)
			}
			if !strings.Contains(gotDetail, tt.detailSubstr) {
				t.Fatalf("unexpected detail. got %q want substring %q", gotDetail, tt.detailSubstr)
			}
		})
	}
}
