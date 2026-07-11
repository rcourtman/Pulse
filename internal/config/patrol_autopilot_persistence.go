package config

import (
	"encoding/json"
	"fmt"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// validatePatrolAutopilotHistoryRewrite makes the encrypted AI config an
// append-only authority ledger for acknowledgement and revocation facts. The
// requested mode and current activation may change, but existing history may
// never be removed, reordered, or edited by a stale config writer.
func validatePatrolAutopilotHistoryRewrite(previous, next AIConfig) error {
	if err := unifiedresources.ValidatePatrolAutopilotStoredEvidence(
		next.PatrolAutopilotAcknowledgements,
		next.PatrolAutopilotRevocations,
		next.PatrolAutopilotActivation,
	); err != nil {
		return err
	}
	if len(next.PatrolAutopilotAcknowledgements) < len(previous.PatrolAutopilotAcknowledgements) || len(next.PatrolAutopilotRevocations) < len(previous.PatrolAutopilotRevocations) {
		return fmt.Errorf("patrol autopilot authority history cannot be removed")
	}
	for index := range previous.PatrolAutopilotAcknowledgements {
		if !canonicalPatrolAutopilotPersistenceEqual(previous.PatrolAutopilotAcknowledgements[index], next.PatrolAutopilotAcknowledgements[index]) {
			return fmt.Errorf("patrol autopilot acknowledgement history is immutable")
		}
	}
	for index := range previous.PatrolAutopilotRevocations {
		if !canonicalPatrolAutopilotPersistenceEqual(previous.PatrolAutopilotRevocations[index], next.PatrolAutopilotRevocations[index]) {
			return fmt.Errorf("patrol autopilot revocation history is immutable")
		}
	}
	if next.PatrolAutopilotActivation != nil {
		if next.GetPatrolAutonomyLevel() != PatrolAutonomyFull {
			return fmt.Errorf("patrol autopilot activation requires requested full mode")
		}
	}
	return nil
}

func canonicalPatrolAutopilotPersistenceEqual(left, right any) bool {
	leftJSON, leftErr := json.Marshal(left)
	rightJSON, rightErr := json.Marshal(right)
	return leftErr == nil && rightErr == nil && string(leftJSON) == string(rightJSON)
}
