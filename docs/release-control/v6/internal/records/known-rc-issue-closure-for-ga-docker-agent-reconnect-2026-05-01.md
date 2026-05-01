# Known RC Issue Closure For GA Docker Agent Reconnect Record

- Date: `2026-05-01`
- Gate: `known-rc-issue-closure-for-ga`
- Result: `passed`

## Context

Late RC3 issue triage rechecked `#1447`, where a Docker agent running inside an
LXC could reconnect initially, then be rejected after the agent container was
recreated because the server treated the new container-derived identity as a
different host association.

An earlier pass correctly noted that v6 had agent ID file persistence and
hostname-plus-token Docker host re-identification, but this pass found one
remaining gap: after the host was matched, the Docker token-binding guard still
compared the token against the new raw agent ID.

## Disposition

The fix is in the monitoring-owned Docker report ingest path:

- `resolveDockerHostIdentifier` still owns the canonical Docker host match.
- `ApplyDockerReport` now resolves token-binding identity from the matched
  canonical Docker host when one exists, while treating the previous host source
  ID, previous agent ID, current agent ID, machine ID, and hostname as aliases
  for the same already-matched host.
- Token-binding rebuild after API token reload now uses the same canonical
  Docker host identity rather than reintroducing raw-agent-ID-first bindings.
- The one-token-per-Docker-agent rule remains intact for genuinely different
  hosts. A different host using a token that is already bound to another Docker
  host is still rejected.

## Proof

- `go test ./internal/monitoring -run 'TestApplyDockerReport_(ReconnectWithoutMachineID|RecreatedContainerAgentIDKeepsTokenBinding|TokenBoundToDifferentAgent|SameHostnameDifferentTokens)$' -count=1`
- `go test ./internal/monitoring -run 'TestApplyDockerReport' -count=1`
- `go test ./internal/monitoring -count=1`
- `go test ./cmd/pulse-agent -run 'TestAgentIDFilePersistence|TestRun_PassesStateDirToUpdaterAndHostAgent' -count=1`

## Outcome

The v6 candidate no longer regresses the reported Docker-in-LXC recreation path:
a recreated container can keep reporting under the same API token after the
canonical host match succeeds, without weakening token uniqueness for separate
Docker agents.
