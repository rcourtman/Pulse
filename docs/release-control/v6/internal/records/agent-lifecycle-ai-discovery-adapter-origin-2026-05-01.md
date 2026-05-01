# Agent Lifecycle AI Discovery Adapter Origin Record

- Date: `2026-05-01`
- Lane: `L16`
- Gate: `known-rc-issue-closure-for-ga`
- Result: `passed`

## Context

The v5 maintenance audit rechecked the oversized AI discovery response fix in
the v6 candidate. The discovery token budget and service-discovery prompt caps
were already present, but the broader proof command exposed stale assumptions in
the AI-to-agent execution boundary:

- the discovery command adapter and Patrol prober tests opened the secured agent
  WebSocket without the same-origin `Origin` header now required by the
  canonical agent execution server;
- the production Patrol reachability prober still composed a compound shell ping
  loop, which the current agent command policy correctly rejects because
  compound commands require approval.

## Disposition

The AI agent-exec tests now derive the WebSocket origin through the shared
security helper and send it on mock agent registration connections. The Patrol
reachability prober now validates each target as an IP address and sends one
single-target `ping -c 1 -W 1 <ip>` command per target; the agent command policy
explicitly auto-approves only that read-only IP probe shape while still rejecting
hostnames and compound ping commands.

This keeps the test harness aligned with the production handshake and fixes the
runtime reachability prober through the policy-owned command contract instead of
weakening origin enforcement or bypassing command approval.

## Proof

- `go test ./internal/ai -run 'TestDiscoveryCommandAdapter|AnalyzeForDiscovery|Discovery.*Token|Token.*Discovery|MaxTokens' -count=1`
- `go test ./internal/ai -run 'TestDiscoveryCommandAdapter|TestAgentExecProber|AnalyzeForDiscovery|Discovery.*Token|Token.*Discovery|MaxTokens' -count=1`
- `go test ./internal/agentexec -run 'TestDefaultPolicyEvaluate|TestPolicyHelpers' -count=1`
- `go test ./internal/servicediscovery -run 'Discovery|ServiceDiscovery|Prompt|Limit|Oversized|Large' -count=1`
- `go test ./internal/ai ./internal/servicediscovery -run 'Discovery|ServiceDiscovery|MaxTokens|Token|oversized|Large' -count=1`
- `go test ./internal/agentexec ./internal/ai ./internal/servicediscovery -count=1`

## Outcome

The v5 oversized-discovery-response fix remains present in v6, the RC3 proof
command now exercises the secured agent WebSocket path successfully, and Patrol's
agent-backed reachability probe is aligned with the current command policy.
