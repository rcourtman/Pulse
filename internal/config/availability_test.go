package config

import "testing"

func TestNormalizeAvailabilityTargetPreservesHTTPAddress(t *testing.T) {
	target := NormalizeAvailabilityTarget(AvailabilityTarget{
		ID:       " target-1 ",
		Name:     "  Status page ",
		Address:  " https://device.local/status?ready=1 ",
		Protocol: AvailabilityProbeHTTP,
		Path:     " health ",
		Enabled:  true,
	})

	if target.ID != "target-1" {
		t.Fatalf("ID = %q, want target-1", target.ID)
	}
	if target.Name != "Status page" {
		t.Fatalf("Name = %q, want Status page", target.Name)
	}
	if target.Address != "https://device.local/status?ready=1" {
		t.Fatalf("Address = %q, want preserved HTTP URL", target.Address)
	}
	if target.Path != "health" {
		t.Fatalf("Path = %q, want health", target.Path)
	}
}

func TestNormalizeAvailabilityTargetReducesICMPAddressToHost(t *testing.T) {
	target := NormalizeAvailabilityTarget(AvailabilityTarget{
		Address:  " https://device.local:8443/status ",
		Protocol: AvailabilityProbeICMP,
		Enabled:  true,
	})

	if target.Address != "device.local" {
		t.Fatalf("Address = %q, want device.local", target.Address)
	}
	if target.Port != 0 {
		t.Fatalf("Port = %d, want 0", target.Port)
	}
}

func TestAvailabilityTargetHTTPURLAppliesPortAndPath(t *testing.T) {
	target := NormalizeAvailabilityTarget(AvailabilityTarget{
		Address:  "device.local/status",
		Protocol: AvailabilityProbeHTTP,
		Port:     8080,
		Path:     "health",
		Enabled:  true,
	})

	u, err := target.HTTPURL()
	if err != nil {
		t.Fatalf("HTTPURL() error = %v", err)
	}
	if got := u.String(); got != "http://device.local:8080/health" {
		t.Fatalf("HTTPURL() = %q, want http://device.local:8080/health", got)
	}
}

func TestAvailabilityTargetValidateRejectsTCPWithoutPort(t *testing.T) {
	target := NormalizeAvailabilityTarget(AvailabilityTarget{
		Address:  "device.local",
		Protocol: AvailabilityProbeTCP,
		Enabled:  true,
	})

	if err := target.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want TCP port error")
	}
}

func TestAvailabilityTargetsRoundTripThroughPersistence(t *testing.T) {
	persistence := NewConfigPersistence(t.TempDir())
	targets := []AvailabilityTarget{
		{
			ID:       "endpoint-1",
			Name:     "Energy monitor",
			Address:  "device.local",
			Protocol: AvailabilityProbeICMP,
			Enabled:  true,
		},
	}

	if err := persistence.SaveAvailabilityTargets(targets); err != nil {
		t.Fatalf("SaveAvailabilityTargets() error = %v", err)
	}

	loaded, err := persistence.LoadAvailabilityTargets()
	if err != nil {
		t.Fatalf("LoadAvailabilityTargets() error = %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("LoadAvailabilityTargets() length = %d, want 1", len(loaded))
	}
	if loaded[0].Name != "Energy monitor" {
		t.Fatalf("loaded name = %q, want Energy monitor", loaded[0].Name)
	}
	if loaded[0].PollIntervalSecs != DefaultAvailabilityPollIntervalSecs {
		t.Fatalf("poll interval = %d, want default", loaded[0].PollIntervalSecs)
	}
}
