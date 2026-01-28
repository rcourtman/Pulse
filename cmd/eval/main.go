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
//	-scenario string  Scenario to run: smoke, readonly, routing, logs, discovery, all (default "smoke")
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
	scenario := flag.String("scenario", "smoke", "Scenario to run: smoke, readonly, routing, logs, discovery, all")
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
	fmt.Println("    routing      - Routing validation test (2 steps)")
	fmt.Println("    logs         - Log tailing/bounded command test (2 steps)")
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
	fmt.Println()
	fmt.Println("  Collections:")
	fmt.Println("    all          - Run all basic scenarios")
	fmt.Println("    advanced     - Run all advanced scenarios")
	fmt.Println("    full         - Run everything")
	fmt.Println()
	fmt.Println("Example:")
	fmt.Println("  go run ./cmd/eval -scenario troubleshoot")
}

func getScenarios(name string) []eval.Scenario {
	switch name {
	// Basic scenarios
	case "smoke":
		return []eval.Scenario{eval.QuickSmokeTest()}
	case "readonly":
		return []eval.Scenario{eval.ReadOnlyInfrastructureScenario()}
	case "routing":
		return []eval.Scenario{eval.RoutingValidationScenario()}
	case "logs":
		return []eval.Scenario{eval.LogTailingScenario()}
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

	// Collections
	case "all":
		return []eval.Scenario{
			eval.QuickSmokeTest(),
			eval.ReadOnlyInfrastructureScenario(),
			eval.RoutingValidationScenario(),
			eval.LogTailingScenario(),
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
		}
	case "full":
		return []eval.Scenario{
			eval.QuickSmokeTest(),
			eval.ReadOnlyInfrastructureScenario(),
			eval.RoutingValidationScenario(),
			eval.LogTailingScenario(),
			eval.DiscoveryScenario(),
			eval.TroubleshootingScenario(),
			eval.DeepDiveScenario(),
			eval.ConfigInspectionScenario(),
			eval.ResourceAnalysisScenario(),
			eval.MultiNodeScenario(),
			eval.DockerInDockerScenario(),
			eval.ContextChainScenario(),
		}
	default:
		return nil
	}
}
