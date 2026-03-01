package hostagent

import (
	"testing"
)

func TestShellescape(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"http://10.0.0.1:7655", "'http://10.0.0.1:7655'"},
		{"simple", "'simple'"},
		{"it's a test", "'it'\"'\"'s a test'"},
		{"", "''"},
		{"$(whoami)", "'$(whoami)'"},
		{"; rm -rf /", "'; rm -rf /'"},
	}
	for _, tt := range tests {
		got := shellescape(tt.input)
		if got != tt.expected {
			t.Errorf("shellescape(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestValidateNodeIP(t *testing.T) {
	valid := []string{"10.0.0.1", "192.168.1.100", "::1", "fe80::1"}
	for _, ip := range valid {
		if err := validateNodeIP(ip); err != nil {
			t.Errorf("expected valid IP %q, got error: %v", ip, err)
		}
	}

	invalid := []string{"", "not-an-ip", "10.0.0.1; rm -rf /", "example.com", "10.0.0.1:22"}
	for _, ip := range invalid {
		if err := validateNodeIP(ip); err == nil {
			t.Errorf("expected error for invalid IP %q", ip)
		}
	}
}

func TestMakeSemaphore(t *testing.T) {
	// Zero/negative defaults to 1.
	sem := makeSemaphore(0)
	if cap(sem) != 1 {
		t.Errorf("expected capacity 1 for 0, got %d", cap(sem))
	}

	sem = makeSemaphore(-1)
	if cap(sem) != 1 {
		t.Errorf("expected capacity 1 for -1, got %d", cap(sem))
	}

	sem = makeSemaphore(3)
	if cap(sem) != 3 {
		t.Errorf("expected capacity 3, got %d", cap(sem))
	}
}

func TestMarshalPreflightResult(t *testing.T) {
	data := marshalPreflightResult(true, true, false, "amd64", "")
	if data == "" {
		t.Fatal("expected non-empty JSON")
	}
	// Should be valid JSON.
	if data[0] != '{' {
		t.Errorf("expected JSON object, got: %s", data)
	}
}

func TestMarshalInstallResult(t *testing.T) {
	data := marshalInstallResult(0, "success")
	if data == "" {
		t.Fatal("expected non-empty JSON")
	}

	// Test truncation.
	longOutput := make([]byte, 5000)
	for i := range longOutput {
		longOutput[i] = 'x'
	}
	data = marshalInstallResult(1, string(longOutput))
	if len(data) > 5000 {
		t.Error("expected truncated output in result")
	}
}

func TestDeployCancelTracker(t *testing.T) {
	called := false
	cancel := func() { called = true }

	registerDeployJob("j1", cancel)

	// Cancel should call the function.
	activeDeploysMu.Lock()
	fn, ok := activeDeploys["j1"]
	activeDeploysMu.Unlock()
	if !ok {
		t.Fatal("expected job to be registered")
	}
	fn()
	if !called {
		t.Error("expected cancel function to be called")
	}

	unregisterDeployJob("j1")
	activeDeploysMu.Lock()
	_, ok = activeDeploys["j1"]
	activeDeploysMu.Unlock()
	if ok {
		t.Error("expected job to be unregistered")
	}
}
