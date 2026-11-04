package dockeragent

import (
	"fmt"
	"strings"
	"testing"
)

func TestDockerTargetRejectionDoesNotDiscloseURL(t *testing.T) {
	// Synthetic inputs cover the outer wrappers, including the missing-token
	// branch that runs before shared URL validation and both destination slots.
	for _, position := range []int{1, 2} {
		for _, token := range []string{"", "synthetic-runtime-token"} {
			for _, tc := range []struct{ name, raw string }{
				{"userinfo", "https://synthetic-user:synthetic-password@private-fixture.invalid"},
				{"query", "https://private-fixture.invalid/?token=synthetic-query"},
				{"fragment", "https://private-fixture.invalid/#synthetic-fragment"},
				{"port", "https://private-fixture.invalid:synthetic-port"},
				{"escape", "https://private-fixture.invalid/%synthetic-escape"},
				{"scheme", "synthetic-scheme://private-fixture.invalid"},
			} {
				t.Run(fmt.Sprintf("slot-%d/token-%t/%s", position, token != "", tc.name), func(t *testing.T) {
					targets := []TargetConfig{{URL: tc.raw, Token: token}}
					if position == 2 {
						targets = append([]TargetConfig{{URL: "https://primary.invalid", Token: "synthetic-primary-token", Authoritative: true}}, targets...)
					}
					normalized, err := normalizeTargets(targets)
					if err == nil || len(normalized) != 0 {
						t.Fatal("invalid target was admitted")
					}
					if strings.Contains(err.Error(), "synthetic-") || strings.Contains(err.Error(), "private-fixture.invalid") {
						t.Error("target refusal disclosed the rejected URL")
					}
					if !strings.Contains(err.Error(), fmt.Sprintf("target %d", position)) {
						t.Error("target refusal lost its destination index")
					}
					agent, newErr := New(Config{Targets: targets})
					if newErr == nil || agent != nil {
						t.Fatal("invalid target produced a Docker module")
					}
					if strings.Contains(newErr.Error(), "synthetic-") || strings.Contains(newErr.Error(), "private-fixture.invalid") {
						t.Error("constructor wrapper disclosed the rejected URL")
					}
				})
			}
		}
	}
}

func TestDockerLegacyTargetRejectionDoesNotDiscloseURL(t *testing.T) {
	agent, err := New(Config{PulseURL: "https://synthetic-user:synthetic-password@private-fixture.invalid", APIToken: "synthetic-runtime-token"})
	if err == nil || agent != nil {
		t.Fatal("invalid legacy target was admitted")
	}
	if strings.Contains(err.Error(), "synthetic-") || strings.Contains(err.Error(), "private-fixture.invalid") {
		t.Fatal("legacy target wrapper disclosed the rejected URL")
	}
}
