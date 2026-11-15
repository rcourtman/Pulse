package config

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAvailabilityDockerPortSourceSurvivesNormalizeAndJSON(t *testing.T) {
	target := NewAvailabilityTarget()
	target.Protocol = AvailabilityProbeTCP
	target.Address = "192.168.1.20"
	target.Port = 8081
	target.DockerPort = &AvailabilityDockerPortSource{HostID: " host-1 ", ContainerName: "/web", PrivatePort: 80}

	normalized := NormalizeAvailabilityTarget(target)
	if err := normalized.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if got := *normalized.DockerPort; got.HostID != "host-1" || got.ContainerName != "web" || got.PrivatePort != 80 {
		t.Fatalf("normalized source = %+v", got)
	}
	if target.DockerPort.ContainerName != "/web" {
		t.Fatal("NormalizeAvailabilityTarget rewrote the caller's source")
	}

	raw, err := json.Marshal(normalized)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded AvailabilityTarget
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.DockerPort == nil || decoded.DockerPort.Key() != normalized.DockerPort.Key() {
		t.Fatalf("source lost in JSON: %s", raw)
	}

	// A check typed in by hand carries no source and no key in its JSON.
	plain := NormalizeAvailabilityTarget(AvailabilityTarget{Protocol: AvailabilityProbeTCP, Address: "192.168.1.20", Port: 22})
	raw, _ = json.Marshal(plain)
	if strings.Contains(string(raw), "dockerPort") {
		t.Fatalf("plain check serialised a docker source: %s", raw)
	}
}

func TestAvailabilityDockerPortSourceKeyIgnoresTheLeadingSlash(t *testing.T) {
	a := AvailabilityDockerPortSource{HostID: "h", ContainerName: "/web", PrivatePort: 80}
	b := AvailabilityDockerPortSource{HostID: "h", ContainerName: "web", PrivatePort: 80}
	if a.Key() != b.Key() {
		t.Fatalf("keys differ: %q vs %q", a.Key(), b.Key())
	}
	for _, other := range []AvailabilityDockerPortSource{
		{HostID: "h2", ContainerName: "web", PrivatePort: 80},
		{HostID: "h", ContainerName: "web2", PrivatePort: 80},
		{HostID: "h", ContainerName: "web", PrivatePort: 81},
	} {
		if other.Key() == a.Key() {
			t.Fatalf("%+v shares a key with %+v", other, a)
		}
	}
}

func TestAvailabilityDockerPortSourceValidation(t *testing.T) {
	base := AvailabilityTarget{TargetKind: AvailabilityTargetService, Protocol: AvailabilityProbeTCP, Address: "192.168.1.20", Port: 8081}
	for name, source := range map[string]AvailabilityDockerPortSource{
		"no host":      {ContainerName: "web", PrivatePort: 80},
		"no container": {HostID: "h", ContainerName: "/", PrivatePort: 80},
		"no port":      {HostID: "h", ContainerName: "web"},
		"port too big": {HostID: "h", ContainerName: "web", PrivatePort: 70000},
	} {
		target := base
		source := source
		target.DockerPort = &source
		if err := target.Validate(); err == nil {
			t.Errorf("%s: Validate() accepted %+v", name, source)
		}
	}
}
