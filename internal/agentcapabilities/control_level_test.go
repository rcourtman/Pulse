package agentcapabilities

import "testing"

func TestNormalizeControlLevelFailsClosed(t *testing.T) {
	tests := []struct {
		name  string
		level string
		want  ControlLevel
	}{
		{name: "empty", level: "", want: ControlLevelReadOnly},
		{name: "read only", level: "read_only", want: ControlLevelReadOnly},
		{name: "controlled", level: "controlled", want: ControlLevelControlled},
		{name: "autonomous", level: "autonomous", want: ControlLevelAutonomous},
		{name: "legacy suggest", level: "suggest", want: ControlLevelReadOnly},
		{name: "unknown", level: "bad", want: ControlLevelReadOnly},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeControlLevel(tt.level); got != tt.want {
				t.Fatalf("NormalizeControlLevel(%q) = %q, want %q", tt.level, got, tt.want)
			}
		})
	}
}

func TestControlLevelAllowsControlTools(t *testing.T) {
	tests := []struct {
		level ControlLevel
		want  bool
	}{
		{ControlLevelReadOnly, false},
		{ControlLevelControlled, true},
		{ControlLevelAutonomous, true},
		{ControlLevel(""), false},
		{ControlLevel("bad"), false},
	}

	for _, tt := range tests {
		if got := ControlLevelAllowsControlTools(tt.level); got != tt.want {
			t.Fatalf("ControlLevelAllowsControlTools(%q) = %v, want %v", tt.level, got, tt.want)
		}
	}
}

func TestIsValidControlLevel(t *testing.T) {
	for _, level := range []string{"read_only", "controlled", "autonomous"} {
		if !IsValidControlLevel(level) {
			t.Fatalf("IsValidControlLevel(%q) = false, want true", level)
		}
	}
	for _, level := range []string{"", "suggest", "bad"} {
		if IsValidControlLevel(level) {
			t.Fatalf("IsValidControlLevel(%q) = true, want false", level)
		}
	}
}
