package config

import "testing"

func TestNormalizeAlertLevelPreservesCanonicalVocabulary(t *testing.T) {
	tests := []struct {
		name string
		in   AlertLevel
		want AlertLevel
	}{
		{name: "info", in: " INFO ", want: AlertLevelInfo},
		{name: "warning", in: AlertLevelWarning, want: AlertLevelWarning},
		{name: "critical", in: AlertLevelCritical, want: AlertLevelCritical},
		{name: "legacy error", in: "error", want: AlertLevelCritical},
		{name: "empty fails safe", in: "", want: AlertLevelWarning},
		{name: "unknown fails safe", in: "unexpected", want: AlertLevelWarning},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := NormalizeAlertLevel(test.in); got != test.want {
				t.Fatalf("NormalizeAlertLevel(%q) = %q, want %q", test.in, got, test.want)
			}
		})
	}
}
