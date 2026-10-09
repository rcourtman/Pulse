# Pulse agent integrations

Pulse exposes a set of HTTP endpoints so an external agent can read your
infrastructure and act on it with the same context Pulse Patrol and the Pulse
Assistant have. Claude Desktop, Claude Code, OpenCode, other MCP clients, and
plain HTTP consumers can all drive it.

This page explains what those endpoints offer and what ships to help you
connect to them. Open **Settings → API Access → Agent integrations** to inspect
your running instance's manifest and generate a client configuration.

**Start read-only.** Connecting an external client is a separate trust decision:
it can read infrastructure context and, with broader credentials, change
monitoring or act on workloads. A tool listing or readiness badge is not an
execution grant or proof of recovery. Use only a client and model provider
permitted to receive the data in those responses.

## Connect without granting control

1. Inspect the manifest on the running server, not just this page. Its
   `requiredScopes` is the union for the whole advertised surface, **not the
   minimum for a read-only client**. Read each capability's `scope`, `actionMode`
   and `approvalPolicy`; `scope_only` writes do not use the action approval loop.
2. Create a dedicated, suitably expiring **`monitoring:read`** token for fleet
   and resource context. Keep its organisation/resource restrictions. Do not
   grant `monitoring:write`, `settings:write`, `ai:execute` or action scopes to
   remove a denied-tool error or make an all-scope readiness indicator pass.
   A client can list tools its token cannot call.
3. Keep the credential in your client's private, user-local secret configuration
   or supported secret store. `pulse-mcp` reads `PULSE_API_TOKEN` from its
   environment, not a token flag. Do not enter a real token in a project-shared
   `.mcp.json` or `opencode.json`, command argument, URL, repository or transcript.
   Generated snippets contain placeholders; they do not secure the saved file.
   Local administrators and the client process can still read its environment.
4. Use loopback only when the client runs on the Pulse host. For another machine,
   configure your existing verified **HTTPS** origin; keep certificate and
   authentication checks enabled. A URL copied from the browser is not proof
   that a desktop client can reach or trust it. Do not expose a backend port or
   weaken a proxy boundary merely to connect a client.
5. Read existing context for one intended resource and check the organisation,
   canonical ID and observation time privately. Names and VMIDs can repeat
   across installations. A successful response does not establish complete or
   current provider collection. Keep findings, notes, command output and
   infrastructure identities out of public diagnostic logs and reports.

For a bounded HTTP check, prepare the private header file and define `pulse_api`
**in the same Bash session** using [API authentication](API.md#-authentication).
That helper saves responses to new owner-only files, ignores curl defaults and
uses deadlines without redirects or retries. It is example code, not an installed
Pulse command. Adapt its origin for remote HTTPS as that guide describes.

```bash
pulse_api GET /api/agent/capabilities
pulse_api GET /api/agent/fleet-context
```

These are separate read-only checks, not an action walkthrough. Discovery is
public; only the protected fleet read checks token access. Inspect each saved
response privately. A 401/403, redirect or incomplete response is not an empty
fleet. Do not run a workload action, change operator state, induce a finding or
enable Debug just to test connectivity. Revoke an unused or exposed token in
**API Access**; deleting a client configuration alone does not revoke it.

## What the endpoints offer

**Discovery.** A manifest at `/api/agent/capabilities` lists every
agent-consumable capability with its name, description, HTTP method and path,
required auth scope, response shape, stable error codes, and a deduplicated
`requiredScopes` summary for the whole surface. It also carries the Pulse
Intelligence Core, Patrol, Assistant, and MCP surface contract, including
which affordances each supported operator surface exposes. The manifest needs
no token, so an agent can introspect Pulse before you issue it credentials.

**Depth.** `/api/agent/resource-context/{id}` returns the situated picture of
one resource in a single read. That covers identity, operator-set state,
active findings, pending approvals, and recent actions including refused
dispatches and verification probe outcomes. Stable token prefixes such as
`plan_drift:` and `resource_remediation_locked:` reach the wire verbatim, so
an agent can branch on codes rather than on human-readable text.

**Breadth.** `/api/agent/fleet-context` returns a thin per-resource rollup
across the whole organisation, covering identity, operator flags, per-severity
finding counts, and pending-approval count. It answers "where do I focus" in
one read, with the per-resource endpoint available for follow-up depth.

**Write.** There are two different write surfaces. The operator-state intent loop
(`/api/resources/{id}/operator-state`) records per-resource commitments such
as intentionally offline, never auto-remediate, maintenance window, and
criticality. The action governance loop (`/api/actions/plan`,
`/api/actions/{id}/decision`, `/api/actions/{id}/execute`) plans, approves,
and executes capability invocations against a resource through the canonical
audit store. Operator-state writes require `monitoring:write` and use
`scope_only` policy, not action-plan approval. PUT replaces the whole saved
record: read it first, preserve unrelated fields and coordinate other writers.
Deleting it can remove maintenance and remediation protections as well as notes.
See [operator-state semantics](API.md#resource-maintenance-and-operator-state).

Action planning, approval and execution have separate scopes and permission
checks; a scope does not bypass actor binding, separation of duties, step-up or
live readiness. The legacy `ai:execute` scope also permits these action stages,
so it is not a read-only substitute. Follow the
[separate action steps](API.md#unified-action-planning), inspect the target,
blast radius and `planHash`, and retain any refusal. Do not grant an external
client all stages unless that is the authority you intend to delegate.
Pulse derives attribution from the authenticated identity, not a caller-supplied
actor. Validation failures emit the `operator_state_invalid` and
`invalid_action_request` codes. Lifecycle conflicts on the action loop emit
`action_not_pending`, `action_not_approved`, `action_already_executing`,
`action_execution_final`, and `action_dry_run_only`, so an agent can branch on
the specific conflict instead of retrying blindly.

**Push.** `/api/agent/events` is an SSE stream that fires `finding.created`,
`approval.pending`, and `action.completed` as state changes. `action.completed`
means a terminal action record, **including failed or refused actions**, not
necessarily a successful dispatch. Inspect its result and verification evidence;
the event name alone does not confirm the intended outcome. An idle stream is
normal and is not a reason to create a finding or execute something.

## Read action outcomes without repeating them

Keep HTTP success, planning, approval, dispatch and post-action verification
separate. An allowed plan has not executed; an approval is not an execution
receipt; a successful dispatch or command exit does not prove workload recovery.

| Verification evidence | Meaning |
| --- | --- |
| `unknown`, absent evidence or legacy record | No established post-action check |
| `unverified` or `ran: false` | The intended state was not established; for example, no usable check or an unreachable agent |
| `failed` | A postcondition was checked and did not match |
| `verified`, with a successful check that ran | That particular postcondition matched at its recorded time, not every application or filesystem health requirement |

Where the response exposes `ran`, `success`, `ranAt` and `note`, read them
together, not `success` alone. Review the existing resource context and, with
the required licensed audit access, its action history. Retain the same action
identity after a timeout or lost response: **a change may already have happened**.
Do not resend execution, invent another request ID or repeat an action to obtain
better evidence. Use the workload's established safe checks independently of
Pulse; verification can be unavailable without proving that nothing ran.

## What ships to help you connect

- **Settings, then API Access, then Agent integrations** is the in-app
  surface. It reads `/api/agent/capabilities` from your running instance,
  lists the declared capabilities by category, shows the surface contract and
  affordance badges, and shows each capability's method, path, scope, and
  stable error codes. It also generates client-ready `pulse-mcp` configuration
  snippets with your instance's URL already filled in, covering OpenCode's
  native `opencode.json` shape and the `mcpServers` shape used by
  Claude-style clients. API tokens are minted on the same tab, so one place
  covers both what agents can do and which token unlocks it.

- **`cmd/pulse-mcp`** is the MCP server adapter. Wire it into any MCP client
  that can launch a local server. It projects each manifest capability into
  one MCP tool with an auto-derived input schema, so capabilities added to
  Pulse extend the MCP surface without an adapter change. Run it with
  `--emit-notifications` to translate Pulse's SSE events into JSON-RPC
  notifications on the stdio channel, which lets an autonomous MCP-bound agent
  react to push events without holding a separate HTTP connection. The
  one-line installers `install-mcp.sh` and `install-mcp.ps1` fetch the
  matching binary from the latest Pulse release, verify the checksum manifest
  against Pulse's pinned release key, and then verify the binary's checksum.
  They refuse installation when any integrity evidence is unavailable or
  invalid. Building from source stays available.

- **`cmd/agent-probe`** is a worked source example for discovery, triage,
  depth and push, not a production diagnostic tool. Its current credential input
  is `--token`, which exposes the token in process arguments, and it prints
  resource context and event bodies. **Do not use it with a real token or
  production data.** Use an isolated synthetic fixture for development, or the
  private-response HTTP checks above for operator reads. Its final summary and
  an idle event window do not prove that an event was received or a workload
  recovered.

## Contracts and limits

- **The manifest matches the implementation.** Every error code an
  agent-surface handler can emit is declared in the manifest, and every code
  the manifest declares is one a handler can emit. Drift in either direction
  is covered by source contract tests. Use the manifest served by the running
  version; a test pass or a newer repository page does not update an installed
  server or establish an external client's compatibility.

- **Discovery needs no token.** `/api/agent/capabilities` serves without
  credentials by design. The capabilities it describes keep their own auth
  scopes, so introspection does not grant access to anything.

- **Error codes come in two layers.** Capability-specific codes such as
  `resource_not_found`, `operator_state_not_set`, and
  `operator_state_invalid` are declared per capability in the manifest.
  Cross-cutting codes such as `invalid_org`, `org_suspended`, and
  `access_denied` come from the auth and multi-tenant middleware and apply to
  every authenticated endpoint.

- **The surfaces compose.** Discovery, triage, depth, and the operator-state
  write loop are exercised together through the real HTTP boundary on every
  build, rather than only in isolation. These fixture checks are not evidence
  of ordinary installed use, complete provider collection or recovered workloads.

## Known rough edges

- **Unsigned macOS binary.** The installer verifies release checksums, but the
  first launch of the macOS `pulse-mcp` binary can still show a Gatekeeper
  warning because the binary is not notarised. Homebrew and other
  package-manager distribution may follow.

- **Field acceptance is separate.** The in-app panel, adapters and contract
  fixtures do not establish every external client's compatibility, scale or
  privacy behaviour. Report a consequential integration gap with the running
  version, client type, capability name and relevant redacted error. Keep tokens,
  client configuration, full responses and model transcripts private; do not
  recreate an action or send production data just to make a report.

## Where to read more

- [Configuration](CONFIGURATION.md) for API tokens and their scopes.
- [API reference](API.md) for the wider Pulse HTTP surface.
- [AI features](AI.md) for how Patrol and the Assistant use the same context.
- `cmd/pulse-mcp/README.md` in the repository for adapter setup examples
  covering OpenCode, Claude Desktop, and Claude Code.
