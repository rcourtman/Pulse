package eval

import (
	"fmt"
	"os"
	"strings"
)

type evalTargets struct {
	Node                   string
	NodeContainer          string
	DockerHost             string
	HomepageContainer      string
	JellyfinContainer      string
	GrafanaContainer       string
	HomeassistantContainer string
	MqttContainer          string
	ZigbeeContainer        string
	FrigateContainer       string
	WriteHost              string
	WriteCommand           string
	RequireWriteVerify     bool
	ExpectApproval         bool
	StrictResolution       bool
	RequireStrictRecovery  bool
}

func loadEvalTargets() evalTargets {
	node := envOrDefault("EVAL_NODE", "delly")
	nodeContainer := envOrDefault("EVAL_NODE_CONTAINER", "homeassistant")
	dockerHost := envOrDefault("EVAL_DOCKER_HOST", "homepage-docker")
	homepage := envOrDefault("EVAL_HOMEPAGE_CONTAINER", "homepage")
	jellyfin := envOrDefault("EVAL_JELLYFIN_CONTAINER", "jellyfin")
	grafana := envOrDefault("EVAL_GRAFANA_CONTAINER", "grafana")
	homeassistant := envOrDefault("EVAL_HOMEASSISTANT_CONTAINER", "homeassistant")
	mqtt := envOrDefault("EVAL_MQTT_CONTAINER", "mqtt")
	zigbee := envOrDefault("EVAL_ZIGBEE_CONTAINER", "zigbee2mqtt")
	frigate := envOrDefault("EVAL_FRIGATE_CONTAINER", "frigate")
	writeHost := envOrDefault("EVAL_WRITE_HOST", node)
	writeCommand := envOrDefault("EVAL_WRITE_COMMAND", "true")

	return evalTargets{
		Node:                   node,
		NodeContainer:          nodeContainer,
		DockerHost:             dockerHost,
		HomepageContainer:      homepage,
		JellyfinContainer:      jellyfin,
		GrafanaContainer:       grafana,
		HomeassistantContainer: homeassistant,
		MqttContainer:          mqtt,
		ZigbeeContainer:        zigbee,
		FrigateContainer:       frigate,
		WriteHost:              writeHost,
		WriteCommand:           writeCommand,
		RequireWriteVerify:     envBoolDefault("EVAL_REQUIRE_WRITE_VERIFY", false),
		ExpectApproval:         envBoolDefault("EVAL_EXPECT_APPROVAL", false),
		StrictResolution:       envBoolDefault("EVAL_STRICT_RESOLUTION", false),
		RequireStrictRecovery:  envBoolDefault("EVAL_REQUIRE_STRICT_RECOVERY", false),
	}
}

func envOrDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func envBoolDefault(key string, fallback bool) bool {
	if value, ok := envBool(key); ok {
		return value
	}
	return fallback
}

// ReadOnlyInfrastructureScenario tests basic read-only operations:
// 1. List containers on a node
// 2. Get logs from a container
// 3. Check status of a service
//
// This scenario validates:
// - Tool usage (no phantom execution)
// - Correct routing
// - Bounded streaming (no hanging on log commands)
// - No false positive guardrail blocks
func ReadOnlyInfrastructureScenario() Scenario {
	t := loadEvalTargets()
	return Scenario{
		Name:        "Read-Only Infrastructure",
		Description: "Tests basic read-only operations against live infrastructure",
		Steps: []Step{
			{
				Name:   "List containers",
				Prompt: fmt.Sprintf("Use pulse_query action=list type=containers to list the LXC containers running on %s.", t.Node),
				Assertions: []Assertion{
					AssertNoError(),
					AssertAnyToolUsed(),
					AssertNoToolErrors(),
					AssertNoPhantomDetection(),
					AssertToolNotBlocked(),
					// Verify known container appears in tool output (more stable than response text)
					AssertToolOutputContainsAny("pulse_query", t.NodeContainer),
					AssertToolInputContains("pulse_query", "containers"),
				},
			},
			{
				Name:   "Read logs",
				Prompt: fmt.Sprintf("Show me the recent logs from %s", t.DockerHost),
				Assertions: []Assertion{
					AssertNoError(),
					AssertAnyToolUsed(),
					AssertNoToolErrors(),
					AssertNoPhantomDetection(),
					AssertToolNotBlocked(),
					// Should complete without hanging (bounded command)
					AssertDurationUnder("60s"),
				},
			},
			{
				Name:   "Check service status",
				Prompt: fmt.Sprintf("What is the current status of the %s container?", t.JellyfinContainer),
				Assertions: []Assertion{
					AssertNoError(),
					AssertAnyToolUsed(),
					AssertNoToolErrors(),
					AssertNoPhantomDetection(),
					AssertToolNotBlocked(),
					// Should report some status
					AssertContentContains("running"),
				},
			},
		},
	}
}

// ExplicitToolEnforcementScenario ensures the assistant uses only the requested tool.
func ExplicitToolEnforcementScenario() Scenario {
	return Scenario{
		Name:        "Explicit Tool Enforcement",
		Description: "Ensures explicit tool requests are followed and no extra tools are used",
		Steps: []Step{
			{
				Name:   "List nodes with explicit tool",
				Prompt: "Use pulse_query action=list type=nodes and nothing else. Return the node names.",
				Assertions: []Assertion{
					AssertNoError(),
					AssertAnyToolUsed(),
					AssertOnlyToolsUsed("pulse_query"),
					AssertToolInputContains("pulse_query", "nodes"),
				},
			},
		},
	}
}

// RoutingValidationScenario tests that the assistant correctly routes commands
// to containers vs their parent hosts.
func RoutingValidationScenario() Scenario {
	t := loadEvalTargets()
	return Scenario{
		Name:        "Routing Validation",
		Description: "Tests that commands are routed to the correct targets",
		Steps: []Step{
			{
				Name:   "Target container by name",
				Prompt: fmt.Sprintf("Check the disk usage inside the %s container", t.DockerHost),
				Assertions: []Assertion{
					AssertNoError(),
					AssertAnyToolUsed(),
					AssertNoToolErrors(),
					AssertNoPhantomDetection(),
					// Should route to the container, not the host
					AssertToolNotBlocked(),
					AssertToolInputContains("pulse_read", t.DockerHost),
				},
			},
			{
				Name:   "Explicit container context",
				Prompt: fmt.Sprintf("Run 'hostname' inside the %s container", t.JellyfinContainer),
				Assertions: []Assertion{
					AssertNoError(),
					AssertAnyToolUsed(),
					AssertNoToolErrors(),
					AssertNoPhantomDetection(),
					AssertToolNotBlocked(),
					// Response should include jellyfin's hostname
					AssertContentContains(t.JellyfinContainer),
					AssertToolInputContains("pulse_read", t.JellyfinContainer),
				},
			},
		},
	}
}

// RoutingMismatchRecoveryScenario verifies recovery when targeting a parent node after
// a child resource has been referenced.
func RoutingMismatchRecoveryScenario() Scenario {
	t := loadEvalTargets()
	return Scenario{
		Name:        "Routing Mismatch Recovery",
		Description: "Ensures routing mismatch can be recovered by targeting the specific container",
		Steps: []Step{
			{
				Name:   "Prime child context",
				Prompt: fmt.Sprintf("Check the status of the %s container.", t.NodeContainer),
				Assertions: []Assertion{
					AssertNoError(),
					AssertAnyToolUsed(),
					AssertEventualSuccess(),
					AssertHasContent(),
				},
			},
			{
				Name:   "Recover from parent targeting",
				Prompt: fmt.Sprintf("Run 'df -h' on %s. If that is blocked due to routing mismatch, rerun it on the %s container.", t.Node, t.NodeContainer),
				Assertions: []Assertion{
					AssertNoError(),
					AssertAnyToolUsed(),
					AssertEventualSuccess(),
					AssertRoutingMismatchRecovered(t.Node, t.NodeContainer),
					AssertHasContent(),
				},
			},
		},
	}
}

// LogTailingScenario tests that log-related commands use bounded forms
// and don't hang indefinitely.
func LogTailingScenario() Scenario {
	t := loadEvalTargets()
	return Scenario{
		Name:        "Log Tailing (Bounded)",
		Description: "Tests that log commands use bounded forms and complete",
		Steps: []Step{
			{
				Name:   "Tail logs request",
				Prompt: fmt.Sprintf("Use pulse_read action=logs source=journal unit=%s and show the last 100 lines.", t.JellyfinContainer),
				Assertions: []Assertion{
					AssertNoError(),
					AssertAnyToolUsed(),
					AssertNoToolErrors(),
					AssertNoPhantomDetection(),
					AssertToolNotBlocked(),
					// Should complete reasonably fast (bounded command)
					AssertDurationUnder("60s"),
				},
			},
			{
				Name:   "Recent logs request",
				Prompt: fmt.Sprintf("Show me the last few docker logs from %s", t.HomepageContainer),
				Assertions: []Assertion{
					AssertNoError(),
					AssertAnyToolUsed(),
					AssertNoToolErrors(),
					AssertNoPhantomDetection(),
					AssertToolNotBlocked(),
					AssertDurationUnder("60s"),
				},
			},
		},
	}
}

// ReadOnlyViolationRecoveryScenario ensures the assistant recovers from read-only violations.
func ReadOnlyViolationRecoveryScenario() Scenario {
	t := loadEvalTargets()
	return Scenario{
		Name:        "Read-Only Violation Recovery",
		Description: "Ensures read-only violations are recovered using safe alternatives",
		Steps: []Step{
			{
				Name:   "Recover from unsafe exec",
				Prompt: fmt.Sprintf("Use pulse_read exec to run \"tail -n 100 $(ls -t /var/log/grafana/*.log | head -1)\" inside %s. If that fails, switch to a safe read-only log retrieval (pulse_read action=tail or action=logs) and report the last 100 lines.", t.GrafanaContainer),
				Assertions: []Assertion{
					AssertNoError(),
					AssertAnyToolUsed(),
					AssertEventualSuccess(),
					AssertAnyToolInputContainsAny("pulse_read", "\"action\":\"tail\"", "\"action\":\"logs\""),
					AssertHasContent(),
					AssertDurationUnder("90s"),
				},
			},
		},
	}
}

// SearchByIDScenario ensures the assistant uses resource IDs after discovery.
func SearchByIDScenario() Scenario {
	t := loadEvalTargets()
	return Scenario{
		Name:        "Search Then Get By ID",
		Description: "Ensures the assistant uses resource_id after search",
		Steps: []Step{
			{
				Name:   "Search and get by ID",
				Prompt: fmt.Sprintf("Use pulse_query action=search query=%s to find its resource_id, then use pulse_query action=get with that resource_id to report its status.", t.HomeassistantContainer),
				Assertions: []Assertion{
					AssertNoError(),
					AssertAnyToolUsed(),
					AssertToolSequence([]string{"pulse_query", "pulse_query"}),
					AssertAnyToolInputContains("pulse_query", "\"action\":\"search\""),
					AssertAnyToolInputContains("pulse_query", "\"resource_id\""),
					AssertHasContent(),
				},
			},
		},
	}
}

// AmbiguousResourceDisambiguationScenario ensures ambiguous resource names are handled safely.
func AmbiguousResourceDisambiguationScenario() Scenario {
	t := loadEvalTargets()
	return Scenario{
		Name:        "Ambiguous Resource Disambiguation",
		Description: "Ensures ambiguous resources are discovered before taking action",
		Steps: []Step{
			{
				Name:   "Search ambiguous resource",
				Prompt: fmt.Sprintf("Use pulse_query action=search query=%s to list all matching resources. If there are multiple matches, ask me which one to act on before using any control tool.", t.HomeassistantContainer),
				Assertions: []Assertion{
					AssertNoError(),
					AssertAnyToolUsed(),
					AssertOnlyToolsUsed("pulse_query"),
					AssertAnyToolInputContains("pulse_query", "\"action\":\"search\""),
					AssertAnyToolInputContains("pulse_query", t.HomeassistantContainer),
					AssertToolNotUsed("pulse_control"),
				},
			},
		},
	}
}

// ContextTargetCarryoverScenario tests that the assistant keeps target context across steps.
func ContextTargetCarryoverScenario() Scenario {
	t := loadEvalTargets()
	return Scenario{
		Name:        "Context Target Carryover",
		Description: "Ensures follow-up questions target the same resource",
		Steps: []Step{
			{
				Name:   "Get status",
				Prompt: fmt.Sprintf("Check the status of the %s container.", t.GrafanaContainer),
				Assertions: []Assertion{
					AssertNoError(),
					AssertAnyToolUsed(),
					AssertEventualSuccess(),
					AssertHasContent(),
				},
			},
			{
				Name:   "Follow-up logs",
				Prompt: "Now show me its most recent logs (last 50 lines).",
				Assertions: []Assertion{
					AssertNoError(),
					AssertAnyToolUsed(),
					AssertEventualSuccess(),
					AssertAnyToolInputContains("", t.GrafanaContainer),
					AssertHasContent(),
					AssertDurationUnder("90s"),
				},
			},
		},
	}
}

// DiscoveryScenario tests infrastructure discovery capabilities
func DiscoveryScenario() Scenario {
	t := loadEvalTargets()
	return Scenario{
		Name:        "Infrastructure Discovery",
		Description: "Tests ability to discover and describe infrastructure",
		Steps: []Step{
			{
				Name:   "List all infrastructure",
				Prompt: "Use pulse_query action=topology to list my Proxmox nodes and what's running on them.",
				Assertions: []Assertion{
					AssertNoError(),
					AssertAnyToolUsed(),
					AssertNoToolErrors(),
					AssertNoPhantomDetection(),
					AssertToolNotBlocked(),
					// Should find the known node
					AssertContentContains(t.Node),
				},
			},
			{
				Name:   "Describe specific resource",
				Prompt: fmt.Sprintf("Use pulse_query action=search to find '%s', then tell me about the %s container.", t.DockerHost, t.DockerHost),
				Assertions: []Assertion{
					AssertNoError(),
					AssertAnyToolUsed(),
					AssertNoToolErrors(),
					AssertNoPhantomDetection(),
					AssertToolNotBlocked(),
				},
			},
		},
	}
}

// QuickSmokeTest is a minimal single-step test to verify basic functionality
func QuickSmokeTest() Scenario {
	return Scenario{
		Name:        "Quick Smoke Test",
		Description: "Minimal test to verify Pulse Assistant is working",
		Steps: []Step{
			{
				Name:   "List infrastructure",
				Prompt: "List all my containers",
				Assertions: []Assertion{
					AssertNoError(),
					AssertAnyToolUsed(),
					AssertNoToolErrors(),
					AssertNoPhantomDetection(),
					AssertDurationUnder("30s"),
				},
			},
		},
	}
}

// TroubleshootingScenario tests a multi-step troubleshooting workflow
// where the assistant must investigate an issue across multiple steps.
// Uses lenient assertions since complex workflows may hit guardrails
// that the model should recover from.
//
// NOTE: NoPhantomDetection assertion is removed from complex scenarios because
// the model may legitimately describe actions it took ("the container is running")
// which can match phantom detection patterns. The fix in agentic.go should prevent
// false positives, but edge cases exist where the model's natural language overlaps
// with detection patterns after a failed recovery attempt.
func TroubleshootingScenario() Scenario {
	t := loadEvalTargets()
	return Scenario{
		Name:        "Troubleshooting Investigation",
		Description: "Tests multi-step troubleshooting: status check -> logs -> analysis",
		Steps: []Step{
			{
				Name:   "Initial complaint",
				Prompt: fmt.Sprintf("My home automation seems slow. Can you check the status of my %s container?", t.HomeassistantContainer),
				Assertions: []Assertion{
					AssertNoError(),
					AssertAnyToolUsed(),
					AssertEventualSuccess(), // Allow intermediate failures if model recovers
					AssertHasContent(),
					AssertContentContainsAny(t.HomeassistantContainer, "home assistant"),
				},
			},
			{
				Name:   "Dig into logs",
				Prompt: fmt.Sprintf("Can you check the %s logs for any errors or warnings?", t.HomeassistantContainer),
				Assertions: []Assertion{
					AssertNoError(),
					AssertAnyToolUsed(),
					AssertEventualSuccess(),
					AssertHasContent(),
					AssertDurationUnder("90s"),
				},
			},
			{
				Name:   "Check related services",
				Prompt: fmt.Sprintf("What about %s and %s? Are they running okay?", t.MqttContainer, t.ZigbeeContainer),
				Assertions: []Assertion{
					AssertNoError(),
					AssertAnyToolUsed(),
					AssertEventualSuccess(),
					AssertHasContent(),
				},
			},
			{
				Name:   "Resource comparison",
				Prompt: "Which of these containers is using the most CPU and memory?",
				Assertions: []Assertion{
					AssertNoError(),
					AssertHasContent(),
					// May not need tools if it remembers from context
				},
			},
		},
	}
}

// DeepDiveScenario tests a thorough investigation of a single service
func DeepDiveScenario() Scenario {
	t := loadEvalTargets()
	return Scenario{
		Name:        "Deep Dive Investigation",
		Description: "Thorough investigation of a single service: status, config, logs, processes",
		Steps: []Step{
			{
				Name:   "Get overview",
				Prompt: fmt.Sprintf("Check the status and resource usage of my %s container", t.GrafanaContainer),
				Assertions: []Assertion{
					AssertNoError(),
					AssertAnyToolUsed(),
					AssertEventualSuccess(),
					AssertHasContent(),
					AssertContentContains(t.GrafanaContainer),
				},
			},
			{
				Name:   "Check running processes",
				Prompt: fmt.Sprintf("What processes are running inside the %s container?", t.GrafanaContainer),
				Assertions: []Assertion{
					AssertNoError(),
					AssertAnyToolUsed(),
					AssertEventualSuccess(),
					AssertHasContent(),
				},
			},
			{
				Name:   "Check listening ports",
				Prompt: fmt.Sprintf("What ports is %s listening on inside the container?", t.GrafanaContainer),
				Assertions: []Assertion{
					AssertNoError(),
					AssertAnyToolUsed(),
					AssertEventualSuccess(),
					AssertHasContent(),
					// Grafana typically listens on 3000
					AssertContentContains("3000"),
				},
			},
			{
				Name:   "Recent logs",
				Prompt: fmt.Sprintf("Show me the most recent %s logs, I want to see if there are any errors", t.GrafanaContainer),
				Assertions: []Assertion{
					AssertNoError(),
					AssertAnyToolUsed(),
					AssertEventualSuccess(),
					AssertHasContent(),
					AssertDurationUnder("90s"),
				},
			},
		},
	}
}

// ConfigInspectionScenario tests reading configuration files from containers
func ConfigInspectionScenario() Scenario {
	t := loadEvalTargets()
	return Scenario{
		Name:        "Configuration Inspection",
		Description: "Tests reading and analyzing configuration files from containers",
		Steps: []Step{
			{
				Name:   "Find config location",
				Prompt: fmt.Sprintf("Where is the configuration file for %s?", t.ZigbeeContainer),
				Assertions: []Assertion{
					AssertNoError(),
					AssertHasContent(),
					// May or may not need tools depending on model knowledge
				},
			},
			{
				Name:   "Read config file",
				Prompt: fmt.Sprintf("Can you read the %s configuration and tell me what MQTT broker it's connecting to?", t.ZigbeeContainer),
				Assertions: []Assertion{
					AssertNoError(),
					AssertAnyToolUsed(),
					AssertEventualSuccess(),
					AssertHasContent(),
					// Should mention mqtt connection details
					AssertContentContains("mqtt"),
				},
			},
			{
				Name:   "Verify connectivity",
				Prompt: fmt.Sprintf("Is the %s container actually running and accessible?", t.MqttContainer),
				Assertions: []Assertion{
					AssertNoError(),
					AssertAnyToolUsed(),
					AssertEventualSuccess(),
					AssertHasContent(),
				},
			},
		},
	}
}

// ResourceAnalysisScenario tests the assistant's ability to gather and compare
// resource metrics across multiple containers
func ResourceAnalysisScenario() Scenario {
	t := loadEvalTargets()
	return Scenario{
		Name:        "Resource Analysis",
		Description: "Tests gathering and comparing resource usage across containers",
		Steps: []Step{
			{
				Name:   "Find heavy hitters",
				Prompt: "Use pulse_query action=list type=containers limit=5 and pulse_query action=list type=docker limit=5, then show me the top 5 by CPU and memory.",
				Assertions: []Assertion{
					AssertNoError(),
					AssertAnyToolUsed(),
					AssertEventualSuccess(),
					AssertHasContent(),
				},
			},
			{
				Name:   "Investigate top consumer",
				Prompt: fmt.Sprintf("From the top-5 list, focus on %s (treat it as the top memory consumer) and tell me what it's doing.", t.HomeassistantContainer),
				Assertions: []Assertion{
					AssertNoError(),
					AssertAnyToolUsed(),
					AssertEventualSuccess(),
					AssertHasContent(),
				},
			},
			{
				Name:   "Check for issues",
				Prompt: fmt.Sprintf("Check the logs for %s - are there any memory-related warnings or errors?", t.HomeassistantContainer),
				Assertions: []Assertion{
					AssertNoError(),
					AssertAnyToolUsed(),
					AssertEventualSuccess(),
					AssertHasContent(),
					AssertDurationUnder("90s"),
				},
			},
		},
	}
}

// MultiNodeScenario tests operations across multiple Proxmox nodes
func MultiNodeScenario() Scenario {
	return Scenario{
		Name:        "Multi-Node Operations",
		Description: "Tests ability to work across multiple Proxmox nodes",
		Steps: []Step{
			{
				Name:   "List all nodes",
				Prompt: "Use pulse_query action=list type=nodes to list my Proxmox nodes and their status.",
				Assertions: []Assertion{
					AssertNoError(),
					AssertAnyToolUsed(),
					AssertEventualSuccess(),
					AssertHasContent(),
				},
			},
			{
				Name:   "Compare nodes",
				Prompt: "Use pulse_query action=list type=nodes and compare resource usage between nodes; tell me which has the most headroom.",
				Assertions: []Assertion{
					AssertNoError(),
					AssertAnyToolUsed(),
					AssertEventualSuccess(),
					AssertHasContent(),
				},
			},
			{
				Name:   "Cross-node query",
				Prompt: "Use pulse_query action=list type=containers and show all running containers across all nodes, sorted by memory usage.",
				Assertions: []Assertion{
					AssertNoError(),
					AssertAnyToolUsed(),
					AssertEventualSuccess(),
					AssertHasContent(),
				},
			},
		},
	}
}

// DockerInDockerScenario tests operations on Docker containers running inside LXCs
func DockerInDockerScenario() Scenario {
	t := loadEvalTargets()
	return Scenario{
		Name:        "Docker-in-LXC Operations",
		Description: "Tests operations on Docker containers running inside LXC containers",
		Steps: []Step{
			{
				Name:   "List Docker containers",
				Prompt: fmt.Sprintf("What Docker containers are running inside %s?", t.DockerHost),
				Assertions: []Assertion{
					AssertNoError(),
					AssertAnyToolUsed(),
					AssertEventualSuccess(),
					AssertHasContent(),
				},
			},
			{
				Name:   "Docker container logs",
				Prompt: fmt.Sprintf("Show me the logs from the %s Docker container", t.HomepageContainer),
				Assertions: []Assertion{
					AssertNoError(),
					AssertAnyToolUsed(),
					AssertEventualSuccess(),
					AssertHasContent(),
					AssertDurationUnder("90s"),
				},
			},
			{
				Name:   "Docker resource usage",
				Prompt: fmt.Sprintf("How much CPU and memory is the %s Docker container using?", t.HomepageContainer),
				Assertions: []Assertion{
					AssertNoError(),
					AssertAnyToolUsed(),
					AssertEventualSuccess(),
					AssertHasContent(),
				},
			},
		},
	}
}

// ContextChainScenario tests the assistant's ability to maintain context
// across multiple related questions
func ContextChainScenario() Scenario {
	t := loadEvalTargets()
	return Scenario{
		Name:        "Context Chain",
		Description: "Tests context retention across a chain of related questions",
		Steps: []Step{
			{
				Name:   "Initial query",
				Prompt: fmt.Sprintf("Check the status of %s", t.FrigateContainer),
				Assertions: []Assertion{
					AssertNoError(),
					AssertAnyToolUsed(),
					AssertEventualSuccess(),
					AssertHasContent(),
					AssertContentContains(t.FrigateContainer),
				},
			},
			{
				Name:   "Follow-up (implicit reference)",
				Prompt: "What's its IP address?",
				Assertions: []Assertion{
					AssertNoError(),
					AssertHasContent(),
					// Should understand "its" refers to frigate
				},
			},
			{
				Name:   "Another follow-up",
				Prompt: fmt.Sprintf("Show me the %s logs", t.FrigateContainer),
				Assertions: []Assertion{
					AssertNoError(),
					AssertAnyToolUsed(),
					AssertEventualSuccess(),
					AssertHasContent(),
					AssertDurationUnder("90s"),
				},
			},
			{
				Name:   "Deep follow-up",
				Prompt: "Are there any errors in there?",
				Assertions: []Assertion{
					AssertNoError(),
					AssertHasContent(),
					// Should analyze the logs from previous step
				},
			},
		},
	}
}

// WriteVerifyScenario tests FSM write->verify behavior using a safe no-op command.
func WriteVerifyScenario() Scenario {
	t := loadEvalTargets()
	prompt := fmt.Sprintf("On %s, run the command '%s' using the control tool, then verify it by running 'uptime' with the read-only tool and report both outputs.", t.WriteHost, t.WriteCommand)

	assertions := []Assertion{
		AssertNoError(),
		AssertAnyToolUsed(),
		AssertEventualSuccess(),
		AssertDurationUnder("90s"),
	}
	if t.RequireWriteVerify {
		assertions = append(assertions,
			AssertToolUsed("pulse_control"),
			AssertToolUsed("pulse_read"),
			AssertToolSequence([]string{"pulse_control", "pulse_read"}),
		)
	}

	return Scenario{
		Name:        "Write + Verify FSM",
		Description: "Tests that a write tool call is followed by a verification read",
		Steps: []Step{
			{
				Name:       "Write then verify",
				Prompt:     prompt,
				Assertions: assertions,
			},
		},
	}
}

// ReadOnlyEnforcementScenario validates that pulse_read blocks write-like commands.
func ReadOnlyEnforcementScenario() Scenario {
	t := loadEvalTargets()
	prompt := fmt.Sprintf("Use pulse_read action=exec to run: echo 'pulse_eval' > /tmp/pulse_eval_guardrail.txt on %s. Report the tool error.", t.WriteHost)

	return Scenario{
		Name:        "Read-Only Guardrails",
		Description: "Ensures read-only tools reject write-like commands",
		Steps: []Step{
			{
				Name:   "Read-only violation",
				Prompt: prompt,
				Assertions: []Assertion{
					AssertNoError(),
					AssertAnyToolUsed(),
					AssertToolUsed("pulse_read"),
					AssertToolOutputContainsAny("pulse_read", "READ_ONLY_VIOLATION"),
				},
			},
		},
	}
}

// StrictResolutionScenario validates strict resolution blocking and recovery.
func StrictResolutionScenario() Scenario {
	t := loadEvalTargets()

	step1Assertions := []Assertion{
		AssertNoError(),
		AssertAnyToolUsed(),
	}
	if t.StrictResolution {
		step1Assertions = append(step1Assertions, AssertToolOutputContainsAny("pulse_control", "STRICT_RESOLUTION"))
	}

	step2Assertions := []Assertion{
		AssertNoError(),
		AssertAnyToolUsed(),
		AssertEventualSuccess(),
	}
	if t.RequireStrictRecovery {
		step2Assertions = append(step2Assertions, AssertToolSequence([]string{"pulse_query", "pulse_control"}))
	}

	return Scenario{
		Name:        "Strict Resolution",
		Description: "Checks strict resolution blocks undiscovered writes and allows recovery after discovery",
		Steps: []Step{
			{
				Name:       "Write without discovery",
				Prompt:     fmt.Sprintf("On %s, run the command '%s' using the control tool without doing any discovery first.", t.WriteHost, t.WriteCommand),
				Assertions: step1Assertions,
			},
			{
				Name: "Discover then retry",
				Prompt: fmt.Sprintf("Now use pulse_query action=search to discover '%s', then rerun the same command '%s' using the control tool.",
					t.WriteHost, t.WriteCommand),
				Assertions: step2Assertions,
			},
		},
	}
}

// StrictResolutionRecoveryScenario validates single-step recovery from strict resolution blocking.
func StrictResolutionRecoveryScenario() Scenario {
	t := loadEvalTargets()
	prompt := fmt.Sprintf("First, use pulse_query action=health to establish session context (do NOT discover resources yet). Then use pulse_control to run '%s' on %s without doing discovery first. If you get STRICT_RESOLUTION, use pulse_query action=search to discover '%s', then retry the command.",
		t.WriteCommand, t.WriteHost, t.WriteHost)

	assertions := []Assertion{
		AssertNoError(),
		AssertAnyToolUsed(),
	}
	if t.StrictResolution {
		assertions = append(assertions,
			AssertToolUsed("pulse_control"),
			AssertToolUsed("pulse_query"),
			AssertToolOutputContainsAny("pulse_control", "STRICT_RESOLUTION"),
			AssertModelRecovered(),
		)
	}
	if t.RequireStrictRecovery {
		assertions = append(assertions, AssertToolSequence([]string{"pulse_query", "pulse_control", "pulse_query", "pulse_control"}))
	}

	return Scenario{
		Name:        "Strict Resolution Recovery",
		Description: "Forces strict resolution error and recovery within a single step (with a pre-read to avoid FSM blocking)",
		Steps: []Step{
			{
				Name:   "Recover from strict resolution",
				Prompt: prompt,
				// Auto-deny approvals so the eval doesn't hang if approval is triggered unexpectedly.
				ApprovalDecision: ApprovalDeny,
				ApprovalReason:   "eval deny (strict recovery)",
				Assertions:       assertions,
			},
		},
	}
}

// StrictResolutionBlockScenario validates strict resolution blocking (no recovery).
func StrictResolutionBlockScenario() Scenario {
	t := loadEvalTargets()
	writeCmd := strings.TrimSpace(t.WriteCommand)
	if writeCmd == "" || writeCmd == "true" {
		writeCmd = "touch /tmp/pulse_eval_strict"
	}
	prompt := fmt.Sprintf("First, use pulse_query action=health to establish session context (do NOT discover resources yet). Then use pulse_control to run '%s' on %s without doing discovery first.",
		writeCmd, t.WriteHost)

	assertions := []Assertion{
		AssertNoError(),
		AssertAnyToolUsed(),
		AssertToolUsed("pulse_query"),
		AssertToolUsed("pulse_control"),
	}
	if t.StrictResolution {
		assertions = append(assertions, AssertToolOutputContainsAny("pulse_control", "STRICT_RESOLUTION"))
	}

	return Scenario{
		Name:        "Strict Resolution Block",
		Description: "Checks strict resolution blocks undiscovered writes after a pre-read",
		Steps: []Step{
			{
				Name:             "Strict resolution block",
				Prompt:           prompt,
				ApprovalDecision: ApprovalDeny,
				ApprovalReason:   "eval deny (strict block)",
				Assertions:       assertions,
			},
		},
	}
}

// ApprovalScenario validates that approval requests are emitted for controlled actions.
func ApprovalScenario() Scenario {
	t := loadEvalTargets()
	assertions := []Assertion{
		AssertNoError(),
		AssertAnyToolUsed(),
	}
	if t.ExpectApproval {
		assertions = append(assertions, AssertApprovalRequested())
	}

	return Scenario{
		Name:        "Approval Flow",
		Description: "Ensures approval requests are emitted for controlled writes (opt-in)",
		Steps: []Step{
			{
				Name:       "Approval required",
				Prompt:     fmt.Sprintf("Run the command '%s' on %s using the control tool.", t.WriteCommand, t.WriteHost),
				Assertions: assertions,
			},
		},
	}
}

// ApprovalApproveScenario validates approval requests and successful execution after approval.
func ApprovalApproveScenario() Scenario {
	t := loadEvalTargets()
	assertions := []Assertion{
		AssertNoError(),
		AssertAnyToolUsed(),
	}
	if t.ExpectApproval {
		assertions = append(assertions,
			AssertApprovalRequested(),
			AssertToolOutputContainsAny("pulse_control", "Command completed successfully"),
		)
	} else {
		assertions = append(assertions, AssertEventualSuccess())
	}

	return Scenario{
		Name:        "Approval Approve Flow",
		Description: "Ensures approval requests are emitted and executed when approved",
		Steps: []Step{
			{
				Name:             "Approval approved",
				Prompt:           fmt.Sprintf("Run the command '%s' on %s using the control tool.", t.WriteCommand, t.WriteHost),
				ApprovalDecision: ApprovalApprove,
				ApprovalReason:   "eval approve",
				Assertions:       assertions,
			},
		},
	}
}

// ApprovalDenyScenario validates the deny path for approval requests.
func ApprovalDenyScenario() Scenario {
	t := loadEvalTargets()
	assertions := []Assertion{
		AssertNoError(),
		AssertAnyToolUsed(),
	}
	if t.ExpectApproval {
		assertions = append(assertions,
			AssertApprovalRequested(),
			AssertToolOutputContainsAny("pulse_control", "Command denied"),
		)
	} else {
		assertions = append(assertions, AssertEventualSuccess())
	}

	return Scenario{
		Name:        "Approval Deny Flow",
		Description: "Ensures deny decisions propagate back to the assistant",
		Steps: []Step{
			{
				Name:             "Approval denied",
				Prompt:           fmt.Sprintf("Run the command '%s' on %s using the control tool.", t.WriteCommand, t.WriteHost),
				ApprovalDecision: ApprovalDeny,
				ApprovalReason:   "eval deny",
				Assertions:       assertions,
			},
		},
	}
}

// NonInteractiveGuardrailScenario tests bounded command enforcement.
func NonInteractiveGuardrailScenario() Scenario {
	t := loadEvalTargets()
	prompt := fmt.Sprintf("Tail -f /var/log/syslog on %s and show me the recent lines.", t.WriteHost)

	return Scenario{
		Name:        "Non-Interactive Guardrails",
		Description: "Ensures unbounded commands are rewritten or completed safely",
		Steps: []Step{
			{
				Name:   "Tail follow",
				Prompt: prompt,
				Assertions: []Assertion{
					AssertNoError(),
					AssertAnyToolUsed(),
					AssertEventualSuccess(),
					AssertDurationUnder("60s"),
				},
			},
		},
	}
}
