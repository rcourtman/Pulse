package servicediscovery

import (
	"testing"
)

func TestSuggestAvailabilityProbe_WebService(t *testing.T) {
	tests := []struct {
		name             string
		serviceType      string
		serviceName      string
		hostIP           string
		wantProtocol     string
		wantPort         int
		wantPath         string
		wantReasonSubstr string
	}{
		{"grafana", "grafana", "Grafana", "10.0.0.1", "http", 3000, "", "service default"},
		{"homeassistant", "homeassistant", "Home Assistant", "10.0.0.2", "http", 8123, "", "service default"},
		{"esphome", "esphome", "ESPHome", "10.0.0.3", "http", 6052, "", "service default"},
		{"plex with path", "plex", "Plex", "10.0.0.4", "http", 32400, "/web", "service default"},
		{"pbs https", "pbs", "Proxmox Backup Server", "10.0.0.5", "https", 8007, "", "service default"},
		{"proxmox https", "proxmox", "Proxmox VE", "10.0.0.6", "https", 8006, "", "service default"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &ResourceDiscovery{
				ServiceType: tt.serviceType,
				ServiceName: tt.serviceName,
			}
			got := SuggestAvailabilityProbe(d, tt.hostIP)
			if got == nil {
				t.Fatalf("expected suggestion for %s, got nil", tt.serviceType)
			}
			if got.Protocol != tt.wantProtocol {
				t.Errorf("protocol = %q, want %q", got.Protocol, tt.wantProtocol)
			}
			if got.Port != tt.wantPort {
				t.Errorf("port = %d, want %d", got.Port, tt.wantPort)
			}
			if got.Path != tt.wantPath {
				t.Errorf("path = %q, want %q", got.Path, tt.wantPath)
			}
			if got.Address != tt.hostIP {
				t.Errorf("address = %q, want %q", got.Address, tt.hostIP)
			}
			if got.ServiceName != tt.serviceName {
				t.Errorf("service_name = %q, want %q", got.ServiceName, tt.serviceName)
			}
		})
	}
}

func TestSuggestAvailabilityProbe_TCPService(t *testing.T) {
	tests := []struct {
		name        string
		serviceType string
		hostIP      string
		wantPort    int
	}{
		{"mqtt", "mqtt", "10.0.0.10", 1883},
		{"mosquitto", "mosquitto", "10.0.0.10", 1883},
		{"postgres", "postgres", "10.0.0.11", 5432},
		{"redis", "redis", "10.0.0.12", 6379},
		{"rabbitmq", "rabbitmq", "10.0.0.13", 5672},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &ResourceDiscovery{
				ServiceType: tt.serviceType,
				ServiceName: tt.serviceType,
			}
			got := SuggestAvailabilityProbe(d, tt.hostIP)
			if got == nil {
				t.Fatalf("expected suggestion for %s, got nil", tt.serviceType)
			}
			if got.Protocol != "tcp" {
				t.Errorf("protocol = %q, want tcp", got.Protocol)
			}
			if got.Port != tt.wantPort {
				t.Errorf("port = %d, want %d", got.Port, tt.wantPort)
			}
		})
	}
}

func TestSuggestAvailabilityProbe_NoSuggestion(t *testing.T) {
	tests := []struct {
		name      string
		discovery *ResourceDiscovery
		hostIP    string
	}{
		{"nil discovery", nil, "10.0.0.1"},
		{"empty hostIP", &ResourceDiscovery{ServiceType: "grafana"}, ""},
		{"unknown service", &ResourceDiscovery{ServiceType: "custom-app"}, "10.0.0.1"},
		{"empty service type", &ResourceDiscovery{ServiceType: ""}, "10.0.0.1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SuggestAvailabilityProbe(tt.discovery, tt.hostIP)
			if got != nil {
				t.Errorf("expected nil suggestion, got %+v", got)
			}
		})
	}
}

func TestSuggestAvailabilityProbe_VariationMatch(t *testing.T) {
	d := &ResourceDiscovery{
		ServiceType: "node-red",
		ServiceName: "Node-RED",
	}
	got := SuggestAvailabilityProbe(d, "10.0.0.20")
	if got == nil {
		t.Fatal("expected suggestion for node-red variation, got nil")
	}
	if got.Protocol != "http" {
		t.Errorf("protocol = %q, want http", got.Protocol)
	}
	if got.Port != 1880 {
		t.Errorf("port = %d, want 1880", got.Port)
	}
}

func TestSuggestAvailabilityProbe_ServiceNameFallback(t *testing.T) {
	d := &ResourceDiscovery{
		ServiceType: "grafana",
		ServiceName: "",
	}
	got := SuggestAvailabilityProbe(d, "10.0.0.1")
	if got == nil {
		t.Fatal("expected suggestion, got nil")
	}
	if got.ServiceName != "grafana" {
		t.Errorf("service_name = %q, want grafana (fallback to matched key)", got.ServiceName)
	}
}
