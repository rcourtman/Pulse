package eval

import "time"

// PatrolBasicScenario tests basic patrol run: completion, tool usage, and findings interaction.
func PatrolBasicScenario() PatrolScenario {
	return PatrolScenario{
		Name:        "Patrol Basic Run",
		Description: "Does patrol run to completion, use tools, and interact with the findings system?",
		Assertions: []PatrolAssertion{
			PatrolAssertNoError(),
			PatrolAssertCompleted(),
			PatrolAssertMinToolCalls(3),
			PatrolAssertToolUsed("pulse_query"),
			PatrolAssertToolUsedAny("patrol_report_finding", "patrol_get_findings"),
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
			PatrolAssertToolUsedAny("pulse_query", "pulse_metrics", "pulse_storage"),
			PatrolAssertToolUsed("patrol_get_findings"),
			PatrolAssertNoToolErrors(),
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

// AllPatrolScenarios returns all patrol scenarios.
func AllPatrolScenarios() []PatrolScenario {
	return []PatrolScenario{
		PatrolBasicScenario(),
		PatrolInvestigationScenario(),
		PatrolFindingQualityScenario(),
	}
}
