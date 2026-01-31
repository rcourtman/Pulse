package eval

import "time"

// PatrolBasicScenario tests basic patrol run: completion, tool usage, and findings interaction.
func PatrolBasicScenario() PatrolScenario {
	return PatrolScenario{
		Name:        "Patrol Basic Run",
		Description: "Does patrol run to completion and use infrastructure tools?",
		Assertions: []PatrolAssertion{
			PatrolAssertNoError(),
			PatrolAssertCompleted(),
			PatrolAssertToolUsedAny("pulse_query", "pulse_metrics", "pulse_storage", "pulse_read"),
			PatrolAssertDurationUnder(4 * time.Minute),
		},
	}
}

// PatrolInvestigationScenario tests investigation quality:
// does patrol investigate before reporting and check existing findings?
func PatrolInvestigationScenario() PatrolScenario {
	return PatrolScenario{
		Name:        "Patrol Investigation Quality",
		Description: "Does patrol investigate before reporting, and check existing findings?",
		Assertions: []PatrolAssertion{
			PatrolAssertNoError(),
			PatrolAssertCompleted(),
			PatrolAssertInvestigatedBeforeReporting("pulse_query", "pulse_metrics", "pulse_storage", "pulse_read"),
			PatrolAssertToolUsed("patrol_get_findings"),
			PatrolAssertToolSuccessRate(0.7),
		},
	}
}

// PatrolFindingQualityScenario tests that reported findings are well-formed.
func PatrolFindingQualityScenario() PatrolScenario {
	return PatrolScenario{
		Name:        "Patrol Finding Quality",
		Description: "Are reported findings well-formed with valid fields?",
		Assertions: []PatrolAssertion{
			PatrolAssertNoError(),
			PatrolAssertCompleted(),
			PatrolAssertAllFindingsValid(),
			PatrolAssertFindingSeveritiesValid(),
			PatrolAssertFindingCategoriesValid(),
			PatrolAssertReportFindingFieldsPresent(),
		},
	}
}

// PatrolSignalCoverageScenario evaluates coverage of deterministic signals.
func PatrolSignalCoverageScenario() PatrolScenario {
	minCoverage := patrolSignalCoverageMin()
	return PatrolScenario{
		Name:        "Patrol Signal Coverage",
		Description: "Do deterministic signals map to findings (coverage score)?",
		Assertions: []PatrolAssertion{
			PatrolAssertNoError(),
			PatrolAssertCompleted(),
			PatrolAssertSignalCoverage(minCoverage),
		},
	}
}

// AllPatrolScenarios returns all patrol scenarios.
func AllPatrolScenarios() []PatrolScenario {
	return []PatrolScenario{
		PatrolBasicScenario(),
		PatrolInvestigationScenario(),
		PatrolFindingQualityScenario(),
		PatrolSignalCoverageScenario(),
	}
}

func patrolSignalCoverageMin() float64 {
	if value, ok := envFloat("EVAL_PATROL_SIGNAL_COVERAGE_MIN"); ok {
		return value
	}
	// Default to a stricter bar once signal detection is tuned.
	return 0.75
}
