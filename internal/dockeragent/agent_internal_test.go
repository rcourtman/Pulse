package dockeragent

import (
	"reflect"
	"testing"
)

func TestNormalizeTargets(t *testing.T) {
	targets, err := normalizeTargets([]TargetConfig{
		{URL: " https://pulse.example.com/ ", Token: "tokenA", InsecureSkipVerify: false},
		{URL: "https://pulse.example.com", Token: "tokenA", InsecureSkipVerify: false}, // duplicate
		{URL: "https://pulse-dr.example.com", Token: "tokenB", InsecureSkipVerify: true},
	})
	if err != nil {
		t.Fatalf("normalizeTargets returned error: %v", err)
	}

	if len(targets) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(targets))
	}

	if targets[0].URL != "https://pulse.example.com" || targets[0].Token != "tokenA" || targets[0].InsecureSkipVerify {
		t.Fatalf("unexpected first target: %+v", targets[0])
	}

	if targets[1].URL != "https://pulse-dr.example.com" || targets[1].Token != "tokenB" || !targets[1].InsecureSkipVerify {
		t.Fatalf("unexpected second target: %+v", targets[1])
	}
}

func TestNormalizeTargetsInvalid(t *testing.T) {
	if _, err := normalizeTargets([]TargetConfig{{URL: "", Token: "token"}}); err == nil {
		t.Fatalf("expected error for missing URL")
	}
	if _, err := normalizeTargets([]TargetConfig{{URL: "https://pulse.example.com", Token: ""}}); err == nil {
		t.Fatalf("expected error for missing token")
	}
}

func TestNormalizeContainerStates(t *testing.T) {
	states, err := normalizeContainerStates([]string{"running", "Exited", "running", "stopped"})
	if err != nil {
		t.Fatalf("normalizeContainerStates returned error: %v", err)
	}

	expected := []string{"running", "exited"}
	if !reflect.DeepEqual(states, expected) {
		t.Fatalf("expected %v, got %v", expected, states)
	}
}

func TestNormalizeContainerStatesInvalid(t *testing.T) {
	if _, err := normalizeContainerStates([]string{"unknown"}); err == nil {
		t.Fatalf("expected error for invalid container state")
	}
}

func TestNormalizeSwarmScope(t *testing.T) {
	tests := map[string]string{
		"":        "node",
		"node":    "node",
		"NODE":    "node",
		"cluster": "cluster",
		"AUTO":    "auto",
	}

	for input, expected := range tests {
		scope, err := normalizeSwarmScope(input)
		if err != nil {
			t.Fatalf("normalizeSwarmScope(%q) returned error: %v", input, err)
		}
		if scope != expected {
			t.Fatalf("normalizeSwarmScope(%q)=%q, expected %q", input, scope, expected)
		}
	}
}

func TestNormalizeSwarmScopeInvalid(t *testing.T) {
	if _, err := normalizeSwarmScope("invalid"); err == nil {
		t.Fatalf("expected error for invalid swarm scope")
	}
}

func TestNormalizeRuntime(t *testing.T) {
	tests := map[string]RuntimeKind{
		"":       RuntimeAuto,
		"auto":   RuntimeAuto,
		"docker": RuntimeDocker,
		"podman": RuntimePodman,
		" Auto ": RuntimeAuto,
		"DOCKER": RuntimeDocker,
		"PODMAN": RuntimePodman,
	}

	for input, expected := range tests {
		runtime, err := normalizeRuntime(input)
		if err != nil {
			t.Fatalf("normalizeRuntime(%q) returned error: %v", input, err)
		}
		if runtime != expected {
			t.Fatalf("normalizeRuntime(%q) = %q, expected %q", input, runtime, expected)
		}
	}
}

func TestNormalizeRuntimeInvalid(t *testing.T) {
	if _, err := normalizeRuntime("containerd"); err == nil {
		t.Fatalf("expected error for unsupported runtime")
	}
}
