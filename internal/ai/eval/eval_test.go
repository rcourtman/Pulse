package eval

import (
	"flag"
	"os"
	"testing"
)

var runLiveEval = flag.Bool("live", false, "Run live eval against Pulse API (requires running Pulse)")

func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(m.Run())
}

// TestQuickSmokeTest runs a minimal smoke test against the live API
// Run with: go test -v ./internal/ai/eval -run TestQuickSmokeTest -live
func TestQuickSmokeTest(t *testing.T) {
	if !*runLiveEval {
		t.Skip("Skipping live eval test. Use -live flag to run against live Pulse API")
	}

	runner := NewRunner(DefaultConfig())
	scenario := QuickSmokeTest()

	result := runner.RunScenario(scenario)
	runner.PrintSummary(result)

	if !result.Passed {
		t.Fatalf("Scenario '%s' failed", scenario.Name)
	}
}

// TestReadOnlyInfrastructure runs the full read-only infrastructure scenario
// Run with: go test -v ./internal/ai/eval -run TestReadOnlyInfrastructure -live
func TestReadOnlyInfrastructure(t *testing.T) {
	if !*runLiveEval {
		t.Skip("Skipping live eval test. Use -live flag to run against live Pulse API")
	}

	runner := NewRunner(DefaultConfig())
	scenario := ReadOnlyInfrastructureScenario()

	result := runner.RunScenario(scenario)
	runner.PrintSummary(result)

	if !result.Passed {
		t.Fatalf("Scenario '%s' failed", scenario.Name)
	}
}

// TestRoutingValidation runs the routing validation scenario
// Run with: go test -v ./internal/ai/eval -run TestRoutingValidation -live
func TestRoutingValidation(t *testing.T) {
	if !*runLiveEval {
		t.Skip("Skipping live eval test. Use -live flag to run against live Pulse API")
	}

	runner := NewRunner(DefaultConfig())
	scenario := RoutingValidationScenario()

	result := runner.RunScenario(scenario)
	runner.PrintSummary(result)

	if !result.Passed {
		t.Fatalf("Scenario '%s' failed", scenario.Name)
	}
}

// TestLogTailing runs the log tailing scenario
// Run with: go test -v ./internal/ai/eval -run TestLogTailing -live
func TestLogTailing(t *testing.T) {
	if !*runLiveEval {
		t.Skip("Skipping live eval test. Use -live flag to run against live Pulse API")
	}

	runner := NewRunner(DefaultConfig())
	scenario := LogTailingScenario()

	result := runner.RunScenario(scenario)
	runner.PrintSummary(result)

	if !result.Passed {
		t.Fatalf("Scenario '%s' failed", scenario.Name)
	}
}

// TestDiscovery runs the infrastructure discovery scenario
// Run with: go test -v ./internal/ai/eval -run TestDiscovery -live
func TestDiscovery(t *testing.T) {
	if !*runLiveEval {
		t.Skip("Skipping live eval test. Use -live flag to run against live Pulse API")
	}

	runner := NewRunner(DefaultConfig())
	scenario := DiscoveryScenario()

	result := runner.RunScenario(scenario)
	runner.PrintSummary(result)

	if !result.Passed {
		t.Fatalf("Scenario '%s' failed", scenario.Name)
	}
}

// TestAllScenarios runs all defined scenarios
// Run with: go test -v ./internal/ai/eval -run TestAllScenarios -live
func TestAllScenarios(t *testing.T) {
	if !*runLiveEval {
		t.Skip("Skipping live eval test. Use -live flag to run against live Pulse API")
	}

	runner := NewRunner(DefaultConfig())

	scenarios := []Scenario{
		QuickSmokeTest(),
		ReadOnlyInfrastructureScenario(),
		RoutingValidationScenario(),
		LogTailingScenario(),
		DiscoveryScenario(),
	}

	allPassed := true
	for _, scenario := range scenarios {
		t.Run(scenario.Name, func(t *testing.T) {
			result := runner.RunScenario(scenario)
			runner.PrintSummary(result)

			if !result.Passed {
				allPassed = false
				t.Errorf("Scenario '%s' failed", scenario.Name)
			}
		})
	}

	if !allPassed {
		t.Fatal("One or more scenarios failed")
	}
}

// TestPatrolBasic runs the basic patrol eval scenario
// Run with: go test -v ./internal/ai/eval -run TestPatrolBasic -live
func TestPatrolBasic(t *testing.T) {
	if !*runLiveEval {
		t.Skip("Skipping live eval test. Use -live flag to run against live Pulse API")
	}

	runner := NewRunner(DefaultConfig())
	result := runner.RunPatrolScenario(PatrolBasicScenario())
	runner.PrintPatrolSummary(result)

	if !result.Success {
		t.Fatalf("Patrol scenario '%s' failed", "Patrol Basic Run")
	}
}

// TestPatrolInvestigation runs the patrol investigation quality scenario
// Run with: go test -v ./internal/ai/eval -run TestPatrolInvestigation -live
func TestPatrolInvestigation(t *testing.T) {
	if !*runLiveEval {
		t.Skip("Skipping live eval test. Use -live flag to run against live Pulse API")
	}

	runner := NewRunner(DefaultConfig())
	result := runner.RunPatrolScenario(PatrolInvestigationScenario())
	runner.PrintPatrolSummary(result)

	if !result.Success {
		t.Fatalf("Patrol scenario '%s' failed", "Patrol Investigation Quality")
	}
}

// TestPatrolFindingQuality runs the patrol finding quality scenario
// Run with: go test -v ./internal/ai/eval -run TestPatrolFindingQuality -live
func TestPatrolFindingQuality(t *testing.T) {
	if !*runLiveEval {
		t.Skip("Skipping live eval test. Use -live flag to run against live Pulse API")
	}

	runner := NewRunner(DefaultConfig())
	result := runner.RunPatrolScenario(PatrolFindingQualityScenario())
	runner.PrintPatrolSummary(result)

	if !result.Success {
		t.Fatalf("Patrol scenario '%s' failed", "Patrol Finding Quality")
	}
}

// TestAllPatrolScenarios runs all patrol eval scenarios
// Run with: go test -v ./internal/ai/eval -run TestAllPatrolScenarios -live
func TestAllPatrolScenarios(t *testing.T) {
	if !*runLiveEval {
		t.Skip("Skipping live eval test. Use -live flag to run against live Pulse API")
	}

	runner := NewRunner(DefaultConfig())

	allPassed := true
	for _, scenario := range AllPatrolScenarios() {
		t.Run(scenario.Name, func(t *testing.T) {
			result := runner.RunPatrolScenario(scenario)
			runner.PrintPatrolSummary(result)

			if !result.Success {
				allPassed = false
				t.Errorf("Patrol scenario '%s' failed", scenario.Name)
			}
		})
	}

	if !allPassed {
		t.Fatal("One or more patrol scenarios failed")
	}
}
