package eval

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
	return Scenario{
		Name:        "Read-Only Infrastructure",
		Description: "Tests basic read-only operations against live infrastructure",
		Steps: []Step{
			{
				Name:   "List containers",
				Prompt: "What containers are running on delly?",
				Assertions: []Assertion{
					AssertNoError(),
					AssertAnyToolUsed(),
					AssertNoToolErrors(),
					AssertNoPhantomDetection(),
					AssertToolNotBlocked(),
					// Should mention at least one known container
					AssertContentContains("homepage"),
				},
			},
			{
				Name:   "Read logs",
				Prompt: "Show me the recent logs from homepage-docker",
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
				Prompt: "What is the current status of the jellyfin container?",
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

// RoutingValidationScenario tests that the assistant correctly routes commands
// to containers vs their parent hosts.
func RoutingValidationScenario() Scenario {
	return Scenario{
		Name:        "Routing Validation",
		Description: "Tests that commands are routed to the correct targets",
		Steps: []Step{
			{
				Name:   "Target container by name",
				Prompt: "Check the disk usage inside the homepage-docker container",
				Assertions: []Assertion{
					AssertNoError(),
					AssertAnyToolUsed(),
					AssertNoToolErrors(),
					AssertNoPhantomDetection(),
					// Should route to the container, not the host
					AssertToolNotBlocked(),
				},
			},
			{
				Name:   "Explicit container context",
				Prompt: "Run 'hostname' inside the jellyfin container",
				Assertions: []Assertion{
					AssertNoError(),
					AssertAnyToolUsed(),
					AssertNoToolErrors(),
					AssertNoPhantomDetection(),
					AssertToolNotBlocked(),
					// Response should include jellyfin's hostname
					AssertContentContains("jellyfin"),
				},
			},
		},
	}
}

// LogTailingScenario tests that log-related commands use bounded forms
// and don't hang indefinitely.
func LogTailingScenario() Scenario {
	return Scenario{
		Name:        "Log Tailing (Bounded)",
		Description: "Tests that log commands use bounded forms and complete",
		Steps: []Step{
			{
				Name:   "Tail logs request",
				Prompt: "Tail the jellyfin logs",
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
				Prompt: "Show me the last few docker logs from homepage",
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

// DiscoveryScenario tests infrastructure discovery capabilities
func DiscoveryScenario() Scenario {
	return Scenario{
		Name:        "Infrastructure Discovery",
		Description: "Tests ability to discover and describe infrastructure",
		Steps: []Step{
			{
				Name:   "List all infrastructure",
				Prompt: "What Proxmox nodes do I have and what's running on them?",
				Assertions: []Assertion{
					AssertNoError(),
					AssertAnyToolUsed(),
					AssertNoToolErrors(),
					AssertNoPhantomDetection(),
					AssertToolNotBlocked(),
					// Should find the known node
					AssertContentContains("delly"),
				},
			},
			{
				Name:   "Describe specific resource",
				Prompt: "Tell me about the homepage-docker container",
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
	return Scenario{
		Name:        "Troubleshooting Investigation",
		Description: "Tests multi-step troubleshooting: status check -> logs -> analysis",
		Steps: []Step{
			{
				Name:   "Initial complaint",
				Prompt: "My home automation seems slow. Can you check the status of my homeassistant container?",
				Assertions: []Assertion{
					AssertNoError(),
					AssertAnyToolUsed(),
					AssertEventualSuccess(), // Allow intermediate failures if model recovers
					AssertHasContent(),
					AssertContentContains("homeassistant"),
				},
			},
			{
				Name:   "Dig into logs",
				Prompt: "Can you check the Home Assistant logs for any errors or warnings?",
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
				Prompt: "What about mqtt and zigbee2mqtt? Are they running okay?",
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
	return Scenario{
		Name:        "Deep Dive Investigation",
		Description: "Thorough investigation of a single service: status, config, logs, processes",
		Steps: []Step{
			{
				Name:   "Get overview",
				Prompt: "Check the status and resource usage of my grafana container",
				Assertions: []Assertion{
					AssertNoError(),
					AssertAnyToolUsed(),
					AssertEventualSuccess(),
					AssertHasContent(),
					AssertContentContains("grafana"),
				},
			},
			{
				Name:   "Check running processes",
				Prompt: "What processes are running inside the grafana container?",
				Assertions: []Assertion{
					AssertNoError(),
					AssertAnyToolUsed(),
					AssertEventualSuccess(),
					AssertHasContent(),
				},
			},
			{
				Name:   "Check listening ports",
				Prompt: "What ports is grafana listening on inside the container?",
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
				Prompt: "Show me the most recent grafana logs, I want to see if there are any errors",
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
	return Scenario{
		Name:        "Configuration Inspection",
		Description: "Tests reading and analyzing configuration files from containers",
		Steps: []Step{
			{
				Name:   "Find config location",
				Prompt: "Where is the configuration file for zigbee2mqtt?",
				Assertions: []Assertion{
					AssertNoError(),
					AssertHasContent(),
					// May or may not need tools depending on model knowledge
				},
			},
			{
				Name:   "Read config file",
				Prompt: "Can you read the zigbee2mqtt configuration and tell me what MQTT broker it's connecting to?",
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
				Prompt: "Is the mqtt container actually running and accessible?",
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
	return Scenario{
		Name:        "Resource Analysis",
		Description: "Tests gathering and comparing resource usage across containers",
		Steps: []Step{
			{
				Name:   "Find heavy hitters",
				Prompt: "Which of my containers are using the most resources? Show me the top 5 by CPU and memory.",
				Assertions: []Assertion{
					AssertNoError(),
					AssertAnyToolUsed(),
					AssertEventualSuccess(),
					AssertHasContent(),
				},
			},
			{
				Name:   "Investigate top consumer",
				Prompt: "Tell me more about the one using the most memory. What's it doing?",
				Assertions: []Assertion{
					AssertNoError(),
					AssertAnyToolUsed(),
					AssertEventualSuccess(),
					AssertHasContent(),
				},
			},
			{
				Name:   "Check for issues",
				Prompt: "Check the logs for that container - are there any memory-related warnings or errors?",
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
				Prompt: "What Proxmox nodes do I have and are they all healthy?",
				Assertions: []Assertion{
					AssertNoError(),
					AssertAnyToolUsed(),
					AssertEventualSuccess(),
					AssertHasContent(),
				},
			},
			{
				Name:   "Compare nodes",
				Prompt: "Compare the resource usage between my nodes. Which one has the most headroom?",
				Assertions: []Assertion{
					AssertNoError(),
					AssertAnyToolUsed(),
					AssertEventualSuccess(),
					AssertHasContent(),
				},
			},
			{
				Name:   "Cross-node query",
				Prompt: "Show me all running containers across all nodes, sorted by memory usage",
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
	return Scenario{
		Name:        "Docker-in-LXC Operations",
		Description: "Tests operations on Docker containers running inside LXC containers",
		Steps: []Step{
			{
				Name:   "List Docker containers",
				Prompt: "What Docker containers are running inside homepage-docker?",
				Assertions: []Assertion{
					AssertNoError(),
					AssertAnyToolUsed(),
					AssertEventualSuccess(),
					AssertHasContent(),
				},
			},
			{
				Name:   "Docker container logs",
				Prompt: "Show me the logs from the homepage Docker container",
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
				Prompt: "How much CPU and memory is the homepage Docker container using?",
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
	return Scenario{
		Name:        "Context Chain",
		Description: "Tests context retention across a chain of related questions",
		Steps: []Step{
			{
				Name:   "Initial query",
				Prompt: "Check the status of frigate",
				Assertions: []Assertion{
					AssertNoError(),
					AssertAnyToolUsed(),
					AssertEventualSuccess(),
					AssertHasContent(),
					AssertContentContains("frigate"),
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
				Prompt: "Show me the frigate logs",
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
