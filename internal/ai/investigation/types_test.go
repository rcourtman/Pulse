package investigation

import "testing"

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.MaxTurns == 0 || cfg.Timeout == 0 || cfg.MaxConcurrent == 0 {
		t.Fatalf("expected non-zero defaults")
	}
	if !cfg.CriticalRequireApproval {
		t.Fatalf("expected critical approval default")
	}
}

func TestIsDestructiveAndHelpers(t *testing.T) {
	if !IsDestructive("rm -rf /tmp") {
		t.Fatalf("expected destructive command")
	}
	if IsDestructive("echo safe") {
		t.Fatalf("expected non-destructive command")
	}
	if !containsPattern("Systemctl Stop", "systemctl stop") {
		t.Fatalf("expected case-insensitive pattern match")
	}
	if containsPattern("short", "longpattern") {
		t.Fatalf("expected pattern mismatch for longer pattern")
	}
	if indexString("abc", "d") != -1 {
		t.Fatalf("expected -1 for missing substring")
	}
	if indexString("abc", "") != 0 {
		t.Fatalf("expected 0 for empty substring")
	}
	if toLower("AbC") != "abc" {
		t.Fatalf("expected lowercase conversion")
	}
	if !contains("Hello", "he") {
		t.Fatalf("expected case-insensitive contains")
	}
}
