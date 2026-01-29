// Command eval runs Pulse Assistant evaluation scenarios against a live Pulse instance.
//
// Usage:
//
//	go run ./cmd/eval                    # Run quick smoke test
//	go run ./cmd/eval -scenario all      # Run all scenarios
//	go run ./cmd/eval -scenario readonly # Run read-only infrastructure scenario
//	go run ./cmd/eval -list              # List available scenarios
//
// Options:
//
//	-scenario string  Scenario to run: smoke, readonly, enforce, routing, routing-recovery, logs, readonly-recovery, search-id, disambiguate, context-target, discovery, writeverify, strict, strict-block, strict-recovery, readonly-guardrails, noninteractive, approval, approval-approve, approval-deny, patrol, patrol-basic, patrol-investigation, patrol-finding-quality, all (default "smoke")
//	-url string       Pulse API base URL (default "http://127.0.0.1:7655")
//	-user string      Username for auth (default "admin")
//	-pass string      Password for auth (default "admin")
//	-list             List available scenarios and exit
//	-quiet            Only show summary, not step-by-step output
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/eval"
)

func main() {
	scenario := flag.String("scenario", "smoke", "Scenario to run: smoke, readonly, enforce, routing, routing-recovery, logs, readonly-recovery, search-id, disambiguate, context-target, discovery, writeverify, strict, strict-block, strict-recovery, readonly-guardrails, noninteractive, approval, approval-approve, approval-deny, patrol, patrol-basic, patrol-investigation, patrol-finding-quality, all")
	url := flag.String("url", "http://127.0.0.1:7655", "Pulse API base URL")
	user := flag.String("user", "admin", "Username for auth")
	pass := flag.String("pass", "admin", "Password for auth")
	list := flag.Bool("list", false, "List available scenarios and exit")
	quiet := flag.Bool("quiet", false, "Only show summary, not step-by-step output")

	flag.Parse()

	if *list {
		listScenarios()
		return
	}

	config := eval.Config{
		BaseURL:  *url,
		Username: *user,
		Password: *pass,
		Verbose:  !*quiet,
	}

	runner := eval.NewRunner(config)

	// Check for patrol scenarios first
	patrolScenarios := getPatrolScenarios(*scenario)
	if len(patrolScenarios) > 0 {
		allPassed := true
		for _, ps := range patrolScenarios {
			fmt.Printf("\n>>> Running patrol scenario: %s\n", ps.Name)
			fmt.Printf(">>> %s\n", ps.Description)

			result := runner.RunPatrolScenario(ps)
			runner.PrintPatrolSummary(result)

			if !result.Success {
				allPassed = false
			}
		}

		if allPassed {
			fmt.Printf("\n>>> ALL PATROL SCENARIOS PASSED\n")
			os.Exit(0)
		} else {
			fmt.Printf("\n>>> SOME PATROL SCENARIOS FAILED\n")
			os.Exit(1)
		}
		return
	}

	// Standard chat scenarios
	scenarios := getScenarios(*scenario)
	if len(scenarios) == 0 {
		fmt.Fprintf(os.Stderr, "Unknown scenario: %s\n", *scenario)
		fmt.Fprintf(os.Stderr, "Use -list to see available scenarios\n")
		os.Exit(1)
	}

	allPassed := true
	for _, s := range scenarios {
		fmt.Printf("\n>>> Running scenario: %s\n", s.Name)
		fmt.Printf(">>> %s\n", s.Description)

		result := runner.RunScenario(s)
		runner.PrintSummary(result)

		if !result.Passed {
			allPassed = false
		}
	}

	if allPassed {
		fmt.Printf("\n>>> ALL SCENARIOS PASSED\n")
		os.Exit(0)
	} else {
		fmt.Printf("\n>>> SOME SCENARIOS FAILED\n")
		os.Exit(1)
	}
}

func listScenarios() {
	fmt.Println("Available scenarios:")
	fmt.Println()
	fmt.Println("  Basic:")
	fmt.Println("    smoke        - Quick smoke test (1 step)")
	fmt.Println("    readonly     - Read-only infrastructure test (3 steps)")
	fmt.Println("    enforce      - Explicit tool enforcement (1 step)")
	fmt.Println("    routing      - Routing validation test (2 steps)")
	fmt.Println("    routing-recovery - Routing mismatch recovery (2 steps)")
	fmt.Println("    logs         - Log tailing/bounded command test (2 steps)")
	fmt.Println("    readonly-recovery - Read-only violation recovery (1 step)")
	fmt.Println("    search-id    - Search then get by resource ID (1 step)")
	fmt.Println("    disambiguate - Ambiguous resource disambiguation (1 step)")
	fmt.Println("    context-target - Context target carryover (2 steps)")
	fmt.Println("    discovery    - Infrastructure discovery test (2 steps)")
	fmt.Println()
	fmt.Println("  Advanced:")
	fmt.Println("    troubleshoot - Multi-step troubleshooting workflow (4 steps)")
	fmt.Println("    deepdive     - Deep investigation of a service (4 steps)")
	fmt.Println("    config       - Configuration file inspection (3 steps)")
	fmt.Println("    resources    - Resource analysis and comparison (3 steps)")
	fmt.Println("    multinode    - Multi-node operations (3 steps)")
	fmt.Println("    docker       - Docker-in-LXC operations (3 steps)")
	fmt.Println("    context      - Context chain / follow-up questions (4 steps)")
	fmt.Println("    writeverify  - Write + verify FSM flow (1 step)")
	fmt.Println("    strict       - Strict resolution block + recovery (2 steps)")
	fmt.Println("    strict-block - Strict resolution block only (1 step)")
	fmt.Println("    strict-recovery - Strict resolution recovery (1 step)")
	fmt.Println("    readonly-guardrails - Read-only enforcement (1 step)")
	fmt.Println("    noninteractive    - Non-interactive guardrails (1 step)")
	fmt.Println("    approval    - Approval flow (1 step, opt-in)")
	fmt.Println("    approval-approve - Approval approve flow (1 step, opt-in)")
	fmt.Println("    approval-deny - Approval deny flow (1 step, opt-in)")
	fmt.Println()
	fmt.Println("  Patrol:")
	fmt.Println("    patrol              - Run all patrol scenarios")
	fmt.Println("    patrol-basic        - Basic patrol run (completion, tools, findings)")
	fmt.Println("    patrol-investigation - Investigation quality (investigate before report)")
	fmt.Println("    patrol-finding-quality - Finding validation (well-formed findings)")
	fmt.Println()
	fmt.Println("  Collections:")
	fmt.Println("    all          - Run all basic scenarios")
	fmt.Println("    advanced     - Run all advanced scenarios")
	fmt.Println("    full         - Run everything")
	fmt.Println()
	fmt.Println("Example:")
	fmt.Println("  go run ./cmd/eval -scenario troubleshoot")
	fmt.Println("  go run ./cmd/eval -scenario patrol-basic")
}

func getPatrolScenarios(name string) []eval.PatrolScenario {
	switch name {
	case "patrol":
		return eval.AllPatrolScenarios()
	case "patrol-basic":
		return []eval.PatrolScenario{eval.PatrolBasicScenario()}
	case "patrol-investigation":
		return []eval.PatrolScenario{eval.PatrolInvestigationScenario()}
	case "patrol-finding-quality":
		return []eval.PatrolScenario{eval.PatrolFindingQualityScenario()}
	default:
		return nil
	}
}

func getScenarios(name string) []eval.Scenario {
	switch name {
	// Basic scenarios
	case "smoke":
		return []eval.Scenario{eval.QuickSmokeTest()}
	case "readonly":
		return []eval.Scenario{eval.ReadOnlyInfrastructureScenario()}
	case "enforce":
		return []eval.Scenario{eval.ExplicitToolEnforcementScenario()}
	case "routing":
		return []eval.Scenario{eval.RoutingValidationScenario()}
	case "routing-recovery":
		return []eval.Scenario{eval.RoutingMismatchRecoveryScenario()}
	case "logs":
		return []eval.Scenario{eval.LogTailingScenario()}
	case "readonly-recovery":
		return []eval.Scenario{eval.ReadOnlyViolationRecoveryScenario()}
	case "search-id":
		return []eval.Scenario{eval.SearchByIDScenario()}
	case "disambiguate":
		return []eval.Scenario{eval.AmbiguousResourceDisambiguationScenario()}
	case "context-target":
		return []eval.Scenario{eval.ContextTargetCarryoverScenario()}
	case "discovery":
		return []eval.Scenario{eval.DiscoveryScenario()}

	// Advanced scenarios
	case "troubleshoot":
		return []eval.Scenario{eval.TroubleshootingScenario()}
	case "deepdive":
		return []eval.Scenario{eval.DeepDiveScenario()}
	case "config":
		return []eval.Scenario{eval.ConfigInspectionScenario()}
	case "resources":
		return []eval.Scenario{eval.ResourceAnalysisScenario()}
	case "multinode":
		return []eval.Scenario{eval.MultiNodeScenario()}
	case "docker":
		return []eval.Scenario{eval.DockerInDockerScenario()}
	case "context":
		return []eval.Scenario{eval.ContextChainScenario()}
	case "writeverify":
		return []eval.Scenario{eval.WriteVerifyScenario()}
	case "strict":
		return []eval.Scenario{eval.StrictResolutionScenario()}
	case "strict-block":
		return []eval.Scenario{eval.StrictResolutionBlockScenario()}
	case "strict-recovery":
		return []eval.Scenario{eval.StrictResolutionRecoveryScenario()}
	case "readonly-guardrails":
		return []eval.Scenario{eval.ReadOnlyEnforcementScenario()}
	case "noninteractive":
		return []eval.Scenario{eval.NonInteractiveGuardrailScenario()}
	case "approval":
		return []eval.Scenario{eval.ApprovalScenario()}
	case "approval-approve":
		return []eval.Scenario{eval.ApprovalApproveScenario()}
	case "approval-deny":
		return []eval.Scenario{eval.ApprovalDenyScenario()}

	// Collections
	case "all":
		return []eval.Scenario{
			eval.QuickSmokeTest(),
			eval.ReadOnlyInfrastructureScenario(),
			eval.ExplicitToolEnforcementScenario(),
			eval.RoutingValidationScenario(),
			eval.RoutingMismatchRecoveryScenario(),
			eval.LogTailingScenario(),
			eval.ReadOnlyViolationRecoveryScenario(),
			eval.SearchByIDScenario(),
			eval.AmbiguousResourceDisambiguationScenario(),
			eval.ContextTargetCarryoverScenario(),
			eval.DiscoveryScenario(),
		}
	case "advanced":
		return []eval.Scenario{
			eval.TroubleshootingScenario(),
			eval.DeepDiveScenario(),
			eval.ConfigInspectionScenario(),
			eval.ResourceAnalysisScenario(),
			eval.MultiNodeScenario(),
			eval.DockerInDockerScenario(),
			eval.ContextChainScenario(),
			eval.WriteVerifyScenario(),
			eval.StrictResolutionScenario(),
			eval.StrictResolutionBlockScenario(),
			eval.StrictResolutionRecoveryScenario(),
			eval.ReadOnlyEnforcementScenario(),
			eval.NonInteractiveGuardrailScenario(),
			eval.ApprovalApproveScenario(),
			eval.ApprovalDenyScenario(),
		}
	case "full":
		return []eval.Scenario{
			eval.QuickSmokeTest(),
			eval.ReadOnlyInfrastructureScenario(),
			eval.ExplicitToolEnforcementScenario(),
			eval.RoutingValidationScenario(),
			eval.RoutingMismatchRecoveryScenario(),
			eval.LogTailingScenario(),
			eval.ReadOnlyViolationRecoveryScenario(),
			eval.SearchByIDScenario(),
			eval.AmbiguousResourceDisambiguationScenario(),
			eval.ContextTargetCarryoverScenario(),
			eval.DiscoveryScenario(),
			eval.TroubleshootingScenario(),
			eval.DeepDiveScenario(),
			eval.ConfigInspectionScenario(),
			eval.ResourceAnalysisScenario(),
			eval.MultiNodeScenario(),
			eval.DockerInDockerScenario(),
			eval.ContextChainScenario(),
			eval.WriteVerifyScenario(),
			eval.StrictResolutionScenario(),
			eval.StrictResolutionBlockScenario(),
			eval.StrictResolutionRecoveryScenario(),
			eval.ReadOnlyEnforcementScenario(),
			eval.NonInteractiveGuardrailScenario(),
			eval.ApprovalApproveScenario(),
			eval.ApprovalDenyScenario(),
		}
	default:
		return nil
	}
}
