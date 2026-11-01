package main

import (
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agenthelper"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentupdate"
)

func TestHelperUpdatePreviousVersionRequiresActivatedBinary(t *testing.T) {
	activeSHA := strings.Repeat("a", 64)
	rollbackSHA := strings.Repeat("b", 64)
	pending := &agentupdate.PendingPrivilegedUpdate{
		Activation: agenthelper.UpdateResult{
			Action:         "pending",
			ActivationID:   "pulse-agent-0123456789abcdef0123456789abcdef:0123456789abcdef",
			ActiveSHA256:   activeSHA,
			RollbackSHA256: rollbackSHA,
		},
		PreviousVersion: "1.0.0",
	}
	cases := []struct {
		name    string
		helper  bool
		pending *agentupdate.PendingPrivilegedUpdate
		running string
		want    string
	}{
		{"LocalUpdateUsesLegacyEvidence", false, nil, "", ""},
		{"HelperWithoutHandoffOverridesLegacyEvidence", true, nil, activeSHA, ""},
		{"ActivatedBinaryRunning", true, pending, activeSHA, "1.0.0"},
		{"ActivatedBinaryRunningCaseAndSpace", true, pending, " " + strings.ToUpper(activeSHA) + "\n", "1.0.0"},
		{"RollbackBinaryRunning", true, pending, rollbackSHA, ""},
		{"UnknownBinaryRunning", true, pending, strings.Repeat("c", 64), ""},
		{"RunningIdentityUnavailable", true, pending, "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := helperUpdatePreviousVersion(tc.helper, tc.pending, tc.running)
			if !tc.helper {
				if got != nil {
					t.Fatal("local update lost its legacy evidence source")
				}
				return
			}
			if got == nil || *got != tc.want {
				t.Fatalf("helper evidence = %v, want explicit %q", got, tc.want)
			}
		})
	}
}
