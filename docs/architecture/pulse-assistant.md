# Pulse Assistant Architecture

**Status:** Authoritative documentation for Pulse Assistant safety architecture
**Last Updated:** 2026-01-28
**Maintainers:** Core team

---

> **CRITICAL INVARIANT (read this first)**
>
> Read-only operations must never route through write tools.
> - Read operations → `pulse_read` (ToolKindRead)
> - Write operations → `pulse_control` (ToolKindWrite)
> - FSM VERIFYING is only entered after `ToolKindWrite` success
>
> Violation causes: Read commands like `grep logs` trigger VERIFYING state, blocking investigation workflows.
> See [Invariant 6](#invariant-6-readwrite-tool-separation) for details.

---

## 1. High-Level Architecture

Pulse Assistant is a **protocol-driven, safety-gated AI system** for infrastructure management. The key insight is that the LLM is treated as an **untrusted proposer** - it can request tool calls but cannot execute them directly. All routing, permissions, and execution are enforced in Go code.

```
┌─────────────────────────────────────────────────────────────────────┐
│                         User Request                                │
└─────────────────────────────────────────────────────────────────────┘
                                 │
                                 ▼
┌─────────────────────────────────────────────────────────────────────┐
│                       Agentic Loop (chat/agentic.go)                │
│                                                                     │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────────────────┐ │
│  │   LLM API   │───▶│    FSM      │───▶│   Tool Executor         │ │
│  │  (proposer) │    │  (gating)   │    │ (validation/execution)  │ │
│  └─────────────┘    └─────────────┘    └─────────────────────────┘ │
│         │                  │                       │                │
│         ▼                  ▼                       ▼                │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────────────────┐ │
│  │  Phantom    │    │  Telemetry  │    │   ResolvedContext       │ │
│  │  Detection  │    │  Counters   │    │ (session-scoped truth)  │ │
│  └─────────────┘    └─────────────┘    └─────────────────────────┘ │
└─────────────────────────────────────────────────────────────────────┘
                                 │
                                 ▼
┌─────────────────────────────────────────────────────────────────────┐
│                    Agent Execution Layer                            │
│  (CommandPolicy → AgentServer → Connected Agents)                   │
└─────────────────────────────────────────────────────────────────────┘
```

### Core Philosophy

1. **LLM is a proposer, not an executor.** The model suggests tool calls; Go code decides what runs.
2. **Resources must be discovered before controlled.** Strict resolution prevents fabricated resource IDs.
3. **Writes must be verified.** FSM enforces read-after-write before final answer.
4. **Errors are recoverable.** Structured error responses enable self-correction without prompt engineering.

---

## 2. Core Design Principles (Invariants)

These are the **structural guarantees** that the system must maintain. They are enforced in code, not prompts.

### Invariant 1: Discovery Before Action
An action tool **cannot** operate on a resource that wasn't first discovered via `pulse_query` or `pulse_discovery`. This prevents the model from fabricating resource IDs.

**Enforcement:** `validateResolvedResource()` in `tools/tools_query.go:282`

### Invariant 2: Verification After Write
After any write operation, the model **must** perform a read/status check before providing a final answer. This catches hallucinated success.

**Enforcement:** FSM `StateVerifying` in `chat/fsm.go:24`, gate in `chat/agentic.go:295`

### Invariant 3: Consistent Error Envelopes
All tool responses use the `ToolResponse` envelope structure. Errors include structured codes (`STRICT_RESOLUTION`, `FSM_BLOCKED`) that enable auto-recovery.

**Enforcement:** `tools/protocol.go:209-283`

### Invariant 4: Phantom Detection
If the model claims to have executed an action but produced no tool calls, the response is replaced with a safe failure message.

**Enforcement:** `hasPhantomExecution()` in `chat/agentic.go:708`

### Invariant 5: Session-Scoped Truth
Resolved resources are **session-scoped** and **in-memory only**. They are never persisted because infrastructure state may change between sessions.

**Enforcement:** `ResolvedContext` not serialized, rebuilt each session in `chat/session.go:25`

### Invariant 6: Read/Write Tool Separation

> **This is the most commonly violated invariant.** Read it carefully.

Read operations and write operations **must go through different tools** to ensure correct FSM classification. The FSM uses **tool classification**, not command content, to determine state transitions.

**The Rule:**
```
Read operations  → pulse_read     → ToolKindRead  → stays in READING
Write operations → pulse_control  → ToolKindWrite → enters VERIFYING
```

**Tool Classification:**
| Tool | Classification | Purpose |
|------|---------------|---------|
| `pulse_read` | **ToolKindRead** | Read-only operations: exec, file, find, tail, logs |
| `pulse_control` | **ToolKindWrite** | Write operations: guest control, service management |
| `pulse_file_edit action=read` | ToolKindRead | File reading |
| `pulse_file_edit action=write/append` | ToolKindWrite | File modification |

**Enforcement Points:**
- Tool registration: `tools/tools_read.go` (read-only tool)
- FSM classification: `chat/fsm.go:343-346` (`pulse_read` → `ToolKindRead`)
- Read-only enforcement: `tools/tools_read.go:139-161` (uses `ClassifyExecutionIntent`)
- Intent classification: `tools/tools_query.go:255-298` (`ClassifyExecutionIntent` function)
- Regression test: `chat/fsm_test.go:TestFSM_RegressionJellyfinLogsScenario`

**ExecutionIntent Classification:**

`pulse_read` uses the `ExecutionIntent` abstraction to determine if a command is provably non-mutating.

| Intent | Meaning | Example |
|--------|---------|---------|
| `IntentReadOnlyCertain` | Non-mutating by construction | `cat`, `grep`, `ls`, `docker logs` |
| `IntentReadOnlyConditional` | Proven read-only by content inspection | `sqlite3 db.db "SELECT..."` |
| `IntentWriteOrUnknown` | Cannot prove non-mutating | `rm`, `curl -X POST`, unknown binaries |

**Classification Phases:**
1. **Mutation-capability guards**: Blocks `sudo`, redirects, pipes to dual-use tools, shell chaining
2. **Known write patterns**: Matches destructive commands (`rm`, `shutdown`, `systemctl restart`)
3. **Read-only by construction**: Matches commands that cannot mutate (`cat`, `grep`, `docker logs`)
4. **Content inspection**: Uses `ContentInspector` interface for dual-use tools (SQL CLIs)
5. **Conservative fallback**: Unknown commands → `IntentWriteOrUnknown`

> **CRITICAL: Phase Ordering Contract**
>
> Any match in Phase 1–2 **dominates** and forces `IntentWriteOrUnknown`.
> Never "optimize" by short-circuiting on allowlists (Phase 3) first.
> This ordering ensures guardrails cannot be bypassed by prepending a read-only command.

**pulse_read Contract:**
```
pulse_read accepts:  IntentReadOnlyCertain, IntentReadOnlyConditional
pulse_read rejects:  IntentWriteOrUnknown (with recovery hint)

Any Phase 1–2 guardrail  → forces IntentWriteOrUnknown
Unknown commands         → IntentWriteOrUnknown (conservative)
```

**NonInteractiveOnly Invariant (Exit-Boundedness):**
```
pulse_read must only execute commands that terminate deterministically.

Rule: Allowed = exits deterministically OR has explicit bound
      Blocked = can run indefinitely without explicit bound

Categories (for telemetry labels):
┌─────────────────────┬──────────────────────────────────────────────────┐
│ [tty_flag]          │ docker exec -it, kubectl exec -it                │
│ [pager]             │ less, more, vim, nano, emacs                     │
│ [unbounded_stream]  │ top, htop, watch, tail -f, journalctl -f         │
│ [interactive_repl]  │ ssh host, mysql, psql, python, node (no command) │
└─────────────────────┴──────────────────────────────────────────────────┘

Exit bounds that allow streaming:
- Line count: -n, --lines, --tail
- Time window: --since, --until
- Timeout wrapper: timeout 5s <cmd>

Examples:
  allow: journalctl --since "10 min ago" -f
  allow: kubectl logs --since=10m --tail=100
  allow: ssh host "ls -la"
  allow: mysql -e "SELECT 1"
  block: journalctl -f
  block: ssh host (no command)
  block: mysql (bare REPL)

Auto-recovery (bounded to 1 attempt):
  When blocked with auto_recoverable=true and suggested_rewrite:
  1. Agentic loop automatically applies the rewrite
  2. Retries once with modified command
  3. If still blocked, surfaces error to user

  Rewrite templates:
    journalctl -f      → journalctl -n 200 --since "10 min ago"
    docker logs -f x   → docker logs --tail=200 x
    kubectl logs -f x  → kubectl logs --tail=200 --since=10m x
    tail -f file       → tail -n 200 file
```

**Adding New Dual-Use Tools:**
Implement the `ContentInspector` interface in `tools/tools_query.go`:
```go
type ContentInspector interface {
    Applies(cmdLower string) bool    // Does this inspector handle this command?
    IsReadOnly(cmdLower string) (bool, string) // Is the content read-only?
}
```
Then add to `registeredInspectors` slice. Example: `sqlContentInspector` inspects SQL CLI commands.

**What happens if violated:**
1. User asks "check the logs on jellyfin"
2. Model runs `grep -i error /var/log/*.log` through `pulse_control`
3. FSM classifies `pulse_control` as `ToolKindWrite`
4. FSM enters VERIFYING state
5. Model tries to run another command → **BLOCKED**
6. Error: "Must verify the previous write operation"
7. User cannot investigate, stuck in verification loop

**Correct behavior:**
1. User asks "check the logs on jellyfin"
2. Model runs `grep -i error /var/log/*.log` through `pulse_read action=exec`
3. FSM classifies `pulse_read` as `ToolKindRead`
4. FSM stays in READING state
5. Model can run unlimited read operations
6. Investigation succeeds

### Invariant 7: Execution Context Binding

> **Tool execution must be context-bound to a resolved resource.**

When the user targets a non-host resource (LXC/VM/Docker container), file operations and commands **must execute within that resource's execution context**, never on the parent host.

**The Rule:**
```
If user mentions @homepage-docker (resolves to lxc:delly:141):
  - All file reads/writes MUST run inside that LXC context
  - NOT on delly just because a path happens to exist there
```

**What happens if violated:**
1. User asks "add InfluxDB to my @homepage-docker config"
2. Model runs `pulse_file_edit` with `target_host="delly"` (the Proxmox node)
3. File exists at `/opt/homepage/config/services.yaml` on delly (coincidentally)
4. Model edits the file on the host
5. Homepage (running in LXC 141) doesn't see the change
6. User's request appears to succeed but nothing actually changes

**Enforcement:** `validateRoutingContext()` in `tools/tools_query.go`
- Blocks with `ROUTING_MISMATCH` if targeting a Proxmox host when **recently referenced** child resources exist
- Error response includes `auto_recoverable: true` and suggests the correct target
- Applied to: `pulse_file_edit`, `pulse_read`, `pulse_control type=command`

**Critical Implementation Detail: LRU Access vs Explicit Access**

> **LRU access is a cache/eviction mechanism; explicit access is an intent mechanism. They must never be conflated.**

The routing validation uses **two separate tracking mechanisms** in `ResolvedContext`:

| Field | Purpose | Set By | Used For |
|-------|---------|--------|----------|
| `lastAccessed` | LRU eviction, TTL expiry | Every add/get | Cache management |
| `explicitlyAccessed` | Routing validation | `MarkExplicitAccess()` only | Detecting user intent |

**Why this matters:**
- Bulk discovery (e.g., `pulse_query action=search`) returns many resources → sets `lastAccessed` for all
- If routing validation used `lastAccessed`, bulk discovery would block subsequent host operations
- Instead, routing validation checks `explicitlyAccessed` which is ONLY set for single-resource operations

**Correct events to mark explicit access:**
- ✅ User @mention resolved to a resource
- ✅ `pulse_query action=get` returning a single resource
- ✅ Explicit selection of a specific resource

**Events that do NOT mark explicit access:**
- ❌ Bulk discovery returning many resources
- ❌ Background prefetch operations
- ❌ General `lastAccessed` updates for LRU

**Implementation:** `chat/types.go:ResolvedContext`
- `WasRecentlyAccessed()` checks `explicitlyAccessed`, not `lastAccessed`
- `MarkExplicitAccess()` only sets `explicitlyAccessed`
- Registration methods: `registerResolvedResource()` vs `registerResolvedResourceWithExplicitAccess()`

**Canonical Resource ID Format:**
```
{kind}:{host}:{provider_uid}   # Scoped resources (globally unique)
{kind}:{provider_uid}          # Global resources (nodes, clusters)

Examples:
  lxc:delly:141              # LXC container 141 on node delly
  vm:minipc:203              # VM 203 on node minipc
  docker_container:server1:abc123  # Docker container on host server1
  node:delly                 # Proxmox node (no parent scope)
```

**Regression tests:**
- `tools/strict_resolution_test.go:TestRoutingMismatch_RegressionHomepageScenario`
- `tools/strict_resolution_test.go:TestRoutingValidation_BulkDiscoveryShouldNotPoisonRouting`
- `tools/strict_resolution_test.go:TestRoutingValidation_ExplicitGetShouldMarkAccess`

**Error Response:**
```json
{
  "ok": false,
  "error": {
    "code": "ROUTING_MISMATCH",
    "message": "target_host 'delly' is a Proxmox node, but you recently referenced more specific resources on it: [homepage-docker].",
    "blocked": true,
    "details": {
      "target_host": "delly",
      "more_specific_resources": ["homepage-docker"],
      "more_specific_resource_ids": ["lxc:delly:141"],
      "target_resource_id": "lxc:delly:141",
      "recovery_hint": "Retry with target_resource_id='lxc:delly:141' (preferred) or target_host='homepage-docker' (legacy)",
      "auto_recoverable": true
    }
  }
}
```

**Telemetry:** `pulse_ai_routing_mismatch_block_total{tool, target_kind, child_kind}`

---

## 3. Tool Protocol

All tools return a consistent `ToolResponse` envelope defined in `tools/protocol.go:209`.

### Response Structure

```go
type ToolResponse struct {
    OK    bool                   `json:"ok"`             // true if tool succeeded
    Data  interface{}            `json:"data,omitempty"` // result data if ok=true
    Error *ToolError             `json:"error,omitempty"`// error details if ok=false
    Meta  map[string]interface{} `json:"meta,omitempty"` // optional metadata
}

type ToolError struct {
    Code      string                 `json:"code"`            // e.g., "STRICT_RESOLUTION"
    Message   string                 `json:"message"`         // Human-readable
    Blocked   bool                   `json:"blocked,omitempty"`   // Policy/validation block
    Failed    bool                   `json:"failed,omitempty"`    // Runtime failure
    Retryable bool                   `json:"retryable,omitempty"` // Auto-retry might succeed
    Details   map[string]interface{} `json:"details,omitempty"`   // Additional context
}
```

### Error Codes (tools/protocol.go:229)

| Code | Meaning | Auto-Recoverable |
|------|---------|------------------|
| `STRICT_RESOLUTION` | Resource not discovered | Yes (discover then retry) |
| `FSM_BLOCKED` | FSM state prevents operation | Yes (perform required action) |
| `NOT_FOUND` | Resource doesn't exist | No |
| `ACTION_NOT_ALLOWED` | Action not permitted | No |
| `POLICY_BLOCKED` | Security policy blocked | No |
| `APPROVAL_REQUIRED` | User approval needed | Yes (wait for approval) |
| `INVALID_INPUT` | Bad parameters | No |
| `EXECUTION_FAILED` | Runtime error | Depends on cause |

### Helper Functions

```go
NewToolSuccess(data)                    // Success response
NewToolBlockedError(code, message, details)  // Policy/validation block
NewToolFailedError(code, message, retryable, details)  // Runtime failure
```

---

## 4. Strict Resolution

Strict resolution prevents the model from operating on fabricated or hallucinated resource IDs.

### Enabling

```bash
export PULSE_STRICT_RESOLUTION=true
```

### Behavior (tools/tools_query.go:59-63)

When enabled (`PULSE_STRICT_RESOLUTION=true`):
- **Write actions** (`start`, `stop`, `restart`, `delete`, `exec`, `write`, `append`) are blocked if the resource wasn't discovered first
- **Read actions** are allowed if the session has any resolved context (scoped bypass)
- **Error response** includes `auto_recoverable: true` to signal the model can self-correct

### Error Type (tools/tools_query.go:16-43)

```go
type ErrStrictResolution struct {
    ResourceID string // The resource that wasn't found
    Action     string // The action that was attempted
    Message    string // Human-readable message
}

// ToToolResponse returns consistent error envelope
func (e *ErrStrictResolution) ToToolResponse() ToolResponse {
    return NewToolBlockedError(
        ErrCodeStrictResolution,
        e.Message,
        map[string]interface{}{
            "resource_id":      e.ResourceID,
            "action":           e.Action,
            "recovery_hint":    "Use pulse_query action=search to discover the resource first",
            "auto_recoverable": true,
        },
    )
}
```

### Validation Flow (tools/tools_query.go:282-383)

```go
func (e *PulseToolExecutor) validateResolvedResource(resourceName, action string, skipIfNoContext bool) ValidationResult {
    strictMode := isStrictResolutionEnabled()
    isWrite := isWriteAction(action)
    requireHardValidation := strictMode && isWrite

    // 1. Check if context exists
    if e.resolvedContext == nil {
        if requireHardValidation {
            return ValidationResult{StrictError: &ErrStrictResolution{...}}
        }
        // Soft validation: allow but log warning
    }

    // 2. Try to find resource by alias or ID
    res, found := e.resolvedContext.GetResolvedResourceByAlias(resourceName)
    if found {
        // 3. Check if action is allowed
        // ...
        return ValidationResult{Resource: res}
    }

    // 4. Not found - block if strict mode
    if requireHardValidation {
        return ValidationResult{StrictError: &ErrStrictResolution{...}}
    }
}
```

---

## 5. ResolvedContext

`ResolvedContext` is the **session-scoped source of truth** for discovered resources. It's defined in `chat/types.go:296-332`.

### Design Principles

1. **Authoritative:** Only query/discovery tools can add resources
2. **Session-scoped:** Not persisted across sessions
3. **In-memory only:** Infrastructure state may change
4. **Multi-indexed:** By name, ID, and aliases

### Structure (chat/types.go:307-332)

```go
type ResolvedContext struct {
    SessionID        string
    Resources        map[string]*ResolvedResource      // By name
    ResourcesByID    map[string]*ResolvedResource      // By canonical ID
    ResourcesByAlias map[string]*ResolvedResource      // By any alias
    lastAccessed     map[string]time.Time              // LRU tracking
    pinned           map[string]bool                   // Eviction protection
    ttl              time.Duration                     // Default: 45 minutes
    maxEntries       int                               // Default: 500
}
```

### Features

| Feature | Default | Purpose |
|---------|---------|---------|
| TTL | 45 minutes | Sliding window expiration |
| Max Entries | 500 | LRU eviction when exceeded |
| Pinning | - | Protect primary targets from eviction |

### Resource Registration Interface (tools/executor.go:136-159)

```go
type ResourceRegistration struct {
    Kind        string   // "node", "vm", "lxc", "docker_container"
    ProviderUID string   // Stable provider ID (container ID, VMID)
    Name        string   // Primary display name
    Aliases     []string // Additional names
    HostUID     string   // Host identifier
    HostName    string   // Host display name
    VMID        int      // For Proxmox guests
    Node        string   // For Proxmox guests
    Executors   []ExecutorRegistration // How to reach this resource
}
```

### Coherent Reset (chat/session.go:415-455)

When clearing session state, both FSM and ResolvedContext must be reset together:

```go
func (s *SessionStore) ClearSessionState(sessionID string, keepPinned bool) {
    // Clear resolved context
    ctx.Clear(keepPinned)

    // Reset FSM coherently
    if !keepPinned {
        fsm.Reset()  // Back to RESOLVING
    } else if ctx.HasAnyResources() {
        fsm.ResetKeepProgress()  // Stay in READING
    } else {
        fsm.Reset()  // No resources left, must rediscover
    }
}
```

---

## 6. Command Risk Classification

Command risk classification (tools/tools_query.go:81-214) determines how shell commands are treated by strict resolution.

### Risk Levels

```go
const (
    CommandRiskReadOnly    CommandRisk = 0 // Safe read-only commands
    CommandRiskLowWrite    CommandRisk = 1 // Low-risk writes
    CommandRiskMediumWrite CommandRisk = 2 // Medium-risk writes
    CommandRiskHighWrite   CommandRisk = 3 // High-risk writes
)
```

### Classification Algorithm

The classifier evaluates commands in 4 phases:

**Phase 1: Shell Metacharacters** (lines 99-125)
- `sudo` → HighWrite
- `>`, `>>`, `tee`, `2>` → HighWrite (output redirection)
- `;`, `&&`, `||` → MediumWrite (command chaining)
- `$(...)`, backticks → MediumWrite (command substitution)

**Phase 2: High-Risk Patterns** (lines 129-153)
- `rm`, `shutdown`, `reboot`, `poweroff`
- `systemctl restart/stop/start`
- Package managers: `apt`, `yum`, `dnf`, `pacman`
- Dangerous docker commands: `docker rm`, `docker kill`
- System modification: `chmod`, `chown`, `iptables`

**Phase 3: Medium-Risk Patterns** (lines 156-182)
- File operations: `mv`, `cp`, `touch`, `mkdir`
- In-place editing: `sed -i`
- Archive extraction: `tar -x`, `unzip`
- Curl with mutation: `-X POST`, `-X DELETE`, `--data`

**Phase 4: Read-Only Patterns** (lines 185-213)
- File inspection: `cat`, `head`, `tail`, `less`, `grep`
- System status: `ps`, `top`, `free`, `df`, `du`
- Docker read: `docker ps`, `docker logs`, `docker inspect`
- Network diagnostics: `ping`, `netstat`, `ss`, `ip addr`
- Systemd status: `systemctl status`, `systemctl is-active`

### Strict Mode Behavior (tools/tools_query.go:395-443)

```go
func (e *PulseToolExecutor) validateResolvedResourceForExec(resourceName, command string, ...) {
    risk := classifyCommandRisk(command)

    if risk == CommandRiskReadOnly && isStrictResolutionEnabled() {
        // Read-only commands allowed if session has ANY resolved context
        if e.resolvedContext != nil && e.hasAnyResolvedHost() {
            return ValidationResult{} // Allow with warning
        }
        // No context at all - require discovery
        return ValidationResult{StrictError: ...}
    }

    // Write commands require specific resource discovery
    return e.validateResolvedResource(resourceName, "exec", ...)
}
```

---

## 7. FSM (Finite State Machine)

The session workflow FSM (chat/fsm.go) enforces structural guarantees about discovery and verification.

### States (chat/fsm.go:14-26)

```
┌─────────────┐     resolve/read     ┌─────────────┐
│  RESOLVING  │─────────────────────▶│   READING   │
│ (initial)   │                      │             │
└─────────────┘                      └──────┬──────┘
      ▲                                     │
      │ Reset()                        write│
      │                                     ▼
      │              ┌─────────────┐  ┌─────────────┐
      └──────────────│   (done)    │◀─│  VERIFYING  │
                     └─────────────┘  └─────────────┘
                           ▲                │
                           │   read         │
                           └────────────────┘
```

| State | Description | Allowed Operations |
|-------|-------------|-------------------|
| `RESOLVING` | Initial state, no validated target | Resolve, Read |
| `READING` | Resources discovered, ready for actions | Resolve, Read, Write |
| `VERIFYING` | Write performed, must verify | Resolve, Read |

### Tool Classification (chat/fsm.go:28-53)

```go
const (
    ToolKindResolve ToolKind = iota  // Discovery/query tools
    ToolKindRead                      // Read-only tools
    ToolKindWrite                     // Mutating tools
)
```

### Classification Function (chat/fsm.go:297-415)

`ClassifyToolCall(toolName string, args map[string]interface{}) ToolKind` is the **single source of truth** for tool classification:

```go
func classifyToolByName(toolName string, args map[string]interface{}) ToolKind {
    switch toolName {
    // Resolve tools
    case "pulse_query", "pulse_discovery":
        return ToolKindResolve

    // Read tools
    case "pulse_metrics", "pulse_storage", "pulse_kubernetes", "pulse_pmg":
        return ToolKindRead

    // Write tools
    case "pulse_control", "pulse_run_command":
        return ToolKindWrite

    // Tools with action-dependent classification
    case "pulse_alerts":
        switch args["action"] {
        case "resolve", "dismiss":
            return ToolKindWrite
        default:
            return ToolKindRead
        }
    case "pulse_docker":
        switch args["action"] {
        case "control", "update":
            return ToolKindWrite
        default:
            return ToolKindRead
        }
    // ...
    }

    // Default to WRITE for unknown tools (security-safe)
    return ToolKindWrite
}
```

### Key Methods (chat/fsm.go)

```go
// Check if tool is allowed in current state
func (fsm *SessionFSM) CanExecuteTool(kind ToolKind, toolName string) error

// Check if final answer is allowed
func (fsm *SessionFSM) CanFinalAnswer() error

// Update state after successful tool execution
func (fsm *SessionFSM) OnToolSuccess(kind ToolKind, toolName string)

// Complete verification and return to READING
func (fsm *SessionFSM) CompleteVerification()
```

### Recovery Tracking (chat/fsm.go:103-133)

The FSM tracks pending recoveries for metrics correlation:

```go
type PendingRecovery struct {
    RecoveryID string    // Unique ID for correlation
    ErrorCode  string    // FSM_BLOCKED or STRICT_RESOLUTION
    Tool       string    // Tool that was blocked
    CreatedAt  time.Time
    Attempts   int
}

// Track a blocked operation
func (fsm *SessionFSM) TrackPendingRecovery(errorCode, tool string) string

// Check if a success resolves a pending recovery
func (fsm *SessionFSM) CheckRecoverySuccess(tool string) *PendingRecovery
```

---

## 8. Agentic Loop Enforcement

The agentic loop (chat/agentic.go) orchestrates LLM calls and enforces the FSM gates.

### Enforcement Gate 1: Tool Execution (chat/agentic.go:396-460)

Before executing any tool:

```go
// Check if tool is allowed in current state
toolKind := ClassifyToolCall(tc.Name, tc.Input)
if fsm != nil {
    if fsmErr := fsm.CanExecuteTool(toolKind, tc.Name); fsmErr != nil {
        // Record telemetry
        metrics.RecordFSMToolBlock(fsm.State, tc.Name, toolKind)

        // Track recovery opportunity
        if fsmBlockedErr.Recoverable {
            fsm.TrackPendingRecovery("FSM_BLOCKED", tc.Name)
            metrics.RecordAutoRecoveryAttempt("FSM_BLOCKED", tc.Name)
        }

        // Return error with recovery hint
        return ToolResult{
            Content: fsmErr.Error() + " Use a discovery or read tool first, then retry.",
            IsError: true,
        }
    }
}
```

### Enforcement Gate 2: Final Answer (chat/agentic.go:295-343)

Before allowing the model to respond without tool calls:

```go
if len(toolCalls) == 0 {
    if fsm != nil {
        if fsmErr := fsm.CanFinalAnswer(); fsmErr != nil {
            // Record telemetry
            metrics.RecordFSMFinalBlock(fsm.State)

            // Inject verification constraint (factual, not narrative)
            verifyPrompt := fmt.Sprintf(
                "Verification required: perform a read or status check on %s before responding.",
                fsm.LastWriteTool,
            )

            // Continue loop to force verification read
            continue
        }
    }
}
```

### Phantom Detection (chat/agentic.go:696-777)

Detects when the model claims execution without tool calls:

```go
func hasPhantomExecution(content string) bool {
    lower := strings.ToLower(content)

    // Category 1: Concrete metrics/values that MUST come from tools
    metricsPatterns := []string{
        "cpu usage is ", "memory usage is ", "disk usage is ",
    }

    // Category 2: Claims of infrastructure state
    statePatterns := []string{
        "is currently running", "is now restarted",
        "the logs show", "according to the output",
    }

    // Category 3: Fake tool call formatting
    fakeToolPatterns := []string{
        "```tool", "pulse_query(", "<tool_call>",
    }

    // Category 4: Past tense claims of specific actions
    actionResultPatterns := []string{
        "i restarted the", "successfully stopped",
    }
}
```

### State Transition After Success (chat/agentic.go:610-631)

```go
if fsm != nil && !isError {
    fsm.OnToolSuccess(toolKind, tc.Name)

    // Check if this success resolves a pending recovery
    if pr := fsm.CheckRecoverySuccess(tc.Name); pr != nil {
        metrics.RecordAutoRecoverySuccess(pr.ErrorCode, pr.Tool)
    }
}
```

---

## 9. Telemetry & Regression Detection

Prometheus counters (chat/metrics.go) provide operational visibility and regression detection.

### Metrics

| Counter | Labels | Purpose |
|---------|--------|---------|
| `pulse_ai_fsm_tool_block_total` | state, tool, kind | FSM blocks of tool execution |
| `pulse_ai_fsm_final_block_total` | state | FSM blocks of final answer |
| `pulse_ai_strict_resolution_block_total` | tool, action | Strict resolution blocks |
| `pulse_ai_phantom_detected_total` | provider, model | Phantom execution detected |
| `pulse_ai_auto_recovery_attempt_total` | error_code, tool | Auto-recovery attempts |
| `pulse_ai_auto_recovery_success_total` | error_code, tool | Successful auto-recoveries |
| `pulse_ai_agentic_iterations_total` | provider, model | Agentic loop turns |

### Recording Points

**FSM Tool Block** (agentic.go:408):
```go
metrics.RecordFSMToolBlock(fsm.State, tc.Name, toolKind)
```

**FSM Final Block** (agentic.go:311):
```go
metrics.RecordFSMFinalBlock(fsm.State)
```

**Strict Resolution Block** (tools_query.go:291, 362):
```go
e.telemetryCallback.RecordStrictResolutionBlock("validateResolvedResource", action)
```

**Phantom Detection** (agentic.go:356):
```go
metrics.RecordPhantomDetected(providerName, modelName)
```

**Auto-Recovery Attempt** (agentic.go:421, metrics.go:202):
```go
metrics.RecordAutoRecoveryAttempt("FSM_BLOCKED", tc.Name)
```

**Auto-Recovery Success** (agentic.go:629):
```go
metrics.RecordAutoRecoverySuccess(pr.ErrorCode, pr.Tool)
```

### Label Sanitization (chat/metrics.go:17-26)

All labels are sanitized to prevent cardinality explosion:

```go
func sanitizeLabel(s string) string {
    if s == "" {
        return "unknown"
    }
    s = strings.ReplaceAll(s, " ", "_")
    if len(s) > maxLabelLen {
        s = s[:maxLabelLen]
    }
    return s
}
```

### Recovery Rate Calculation

```promql
# Auto-recovery success rate
sum(rate(pulse_ai_auto_recovery_success_total[1h]))
/
sum(rate(pulse_ai_auto_recovery_attempt_total[1h]))
```

---

## 10. What NOT to Change

This section documents **critical invariants** that must not be modified without understanding the full system impact.

### DO NOT: Allow Action Tools Without Discovery

```go
// WRONG: Bypass validation
if validation.IsBlocked() {
    // Just log and continue  ← NO!
}

// CORRECT: Return error for auto-recovery
if validation.IsBlocked() {
    return NewToolResponseResult(validation.StrictError.ToToolResponse())
}
```

**Location:** `tools/tools_control.go:267-271`, `tools/tools_file.go:188-191`, `tools/tools_file.go:276-279`

### DO NOT: Remove FSM Gates from Agentic Loop

```go
// WRONG: Skip FSM check
// if fsm != nil { ... }  ← Removing this breaks verification

// CORRECT: Always check FSM
if fsm != nil {
    if fsmErr := fsm.CanExecuteTool(toolKind, tc.Name); fsmErr != nil {
        // Handle block
    }
}
```

**Location:** `chat/agentic.go:396-461` (Gate 1), `chat/agentic.go:295-343` (Gate 2)

### DO NOT: Change Unknown Tool Default to Read

```go
// WRONG: Default to read (bypasses gates for new tools)
return ToolKindRead  ← NO!

// CORRECT: Default to write (security-safe)
return ToolKindWrite
```

**Location:** `chat/fsm.go:412-414`

### DO NOT: Persist ResolvedContext

```go
// WRONG: Save to disk
json.Marshal(resolvedContext)  ← NO!

// CORRECT: Keep in-memory only
resolvedContexts map[string]*ResolvedContext  // Not persisted
```

**Location:** `chat/session.go:23-30`, `chat/types.go:315`

### DO NOT: Reset FSM Without Context

```go
// WRONG: Reset FSM only
fsm.Reset()

// CORRECT: Reset both coherently
s.ClearSessionState(sessionID, keepPinned)  // Handles both
```

**Location:** `chat/session.go:415-455`

### DO NOT: Remove Phantom Detection

```go
// WRONG: Skip phantom check
// if hasPhantomExecution(content) { ... }  ← Removing this allows hallucinated actions

// CORRECT: Always check and replace
if hasPhantomExecution(assistantMsg.Content) {
    metrics.RecordPhantomDetected(...)
    resultMessages[...].Content = safeResponse
}
```

**Location:** `chat/agentic.go:348-374`

### DO NOT: Route Read Operations Through Write Tools

Read-only operations must use `pulse_read`, NOT `pulse_control`:

```go
// WRONG: Read command through write tool
pulse_control type="command" command="grep logs"  ← Triggers VERIFYING!

// CORRECT: Read command through read tool
pulse_read action="exec" command="grep logs"  ← Stays in READING
```

**Why:** `pulse_control` is classified as `ToolKindWrite` regardless of what command it runs. Running `grep` through `pulse_control` triggers VERIFYING state, blocking subsequent commands. Using `pulse_read` keeps the FSM in READING state where unlimited reads are allowed.

**Location:** `tools/tools_read.go` (read-only tool), `chat/fsm.go:343-346` (classification)

### DO NOT: Target Parent Host When Child Resources Exist

File and exec operations must target the specific resource, not the parent Proxmox host:

```go
// WRONG: Target Proxmox host when LXC was mentioned
pulse_file_edit target_host="delly" path="/opt/homepage/config/services.yaml"
// ^ Edits file on host, not inside homepage-docker LXC!

// CORRECT: Target the specific resource
pulse_file_edit target_host="homepage-docker" path="/opt/homepage/config/services.yaml"
// ^ Edits file inside the LXC where Homepage actually runs
```

**Why:** Files at the same path may exist on both the host and inside containers, but they are completely different filesystems. Editing the host file has no effect on applications running inside containers.

**Enforcement:** `validateRoutingContext()` in `tools/tools_query.go` blocks with `ROUTING_MISMATCH` when targeting a Proxmox host that has discovered child resources.

**Location:** `tools/tools_query.go:validateRoutingContext()`, applied in `tools_file.go` and `tools_read.go`

### DO NOT: Add Prompts to Error Recovery

The system uses **structured error responses**, not prompts, for recovery:

```go
// WRONG: Add narrative prompts
return "I notice you haven't discovered resources yet. Let me suggest..."  ← NO!

// CORRECT: Return structured error
return NewToolBlockedError(
    ErrCodeStrictResolution,
    "Resource not discovered",
    map[string]interface{}{
        "recovery_hint":    "Use pulse_query action=search...",
        "auto_recoverable": true,
    },
)
```

### DO NOT: Skip Telemetry Recording

Every block must be recorded for regression detection:

```go
// WRONG: Silent block
return ValidationResult{StrictError: err}

// CORRECT: Record then return
if e.telemetryCallback != nil {
    e.telemetryCallback.RecordStrictResolutionBlock(...)
}
return ValidationResult{StrictError: err}
```

**Locations:** All validation functions in `tools/tools_query.go`

---

## Appendix A: Known Failure Modes & Fixes

This appendix documents failure modes that have been discovered and fixed. Use this for debugging when similar symptoms appear.

### Failure Mode 1: VERIFYING Deadlock (Read Operations)

**Symptom:** `write → read → write` blocked repeatedly. User cannot investigate after any write.

**Cause:** Read operations routed through `pulse_control` (classified as `ToolKindWrite`) instead of `pulse_read` (classified as `ToolKindRead`).

**Fix/Invariant:** Read operations must use `pulse_read`. Tool classification determines FSM transitions, not command content.

**Regression Test:** `TestFSM_RegressionJellyfinLogsScenario`

**Telemetry Signal:** High `fsm_tool_block_total{state="VERIFYING"}` for read-like commands.

### Failure Mode 2: VERIFYING Never Clears

**Symptom:** After `write → read`, subsequent writes still blocked. FSM stays in VERIFYING.

**Cause:** `CompleteVerification()` was only called when model gave final answer (no tool calls), not during tool execution loop.

**Fix/Invariant:** Call `CompleteVerification()` immediately after `OnToolSuccess()` when in VERIFYING and `ReadAfterWrite=true`. Verification must complete on first successful read, not wait for final answer.

**Regression Test:** `TestFSM_RegressionWriteReadWriteSequence`, `TestFSM_RegressionMultipleReadsAfterWrite`

**Telemetry Signal:** Persistent `fsm_tool_block_total{state="VERIFYING"}` even after successful reads.

### Failure Mode 3: Stderr Redirects Blocked

**Symptom:** Commands like `find ... 2>/dev/null` rejected as "not read-only".

**Cause:** `classifyCommandRisk()` treated all `>` characters as dangerous output redirection.

**Fix/Invariant:** Strip safe stderr patterns (`2>/dev/null`, `2>&1`) before checking for dangerous redirects.

**Regression Test:** `TestClassifyCommandRisk` cases for stderr redirects.

### Failure Mode 4: Sticky ReadAfterWrite Flag

**Symptom:** Weird "already verified" states after multiple write cycles.

**Cause:** `CompleteVerification()` didn't reset `ReadAfterWrite` flag.

**Fix/Invariant:** `CompleteVerification()` must reset `ReadAfterWrite=false` for clean cycle.

### Failure Mode 5: File Operations on Wrong Host (Routing Mismatch)

**Symptom:** File edits "succeed" but have no effect. Config changes don't appear in the application.

**Cause:** Model targeted a Proxmox host (`target_host="delly"`) when user mentioned an LXC/VM (`@homepage-docker`). The file was edited on the host's filesystem, not inside the container where the application runs.

**Fix/Invariant:** `validateRoutingContext()` blocks operations that target a Proxmox host when the ResolvedContext contains LXC/VM children on that host. Returns `ROUTING_MISMATCH` with `auto_recoverable: true`.

**Regression Test:** `TestRoutingMismatch_RegressionHomepageScenario`

**Telemetry Signal:** Watch for `ROUTING_MISMATCH` error code in tool responses.

**Why This Happens:**
- Path `/opt/homepage/config/services.yaml` exists on both the host and inside the LXC
- Model picks the host because it's "simpler" or appears first
- Routing system correctly routes to the target it's given
- But the user intended the LXC context, not the host

---

## Appendix B: Contributor Checklist

Use this checklist when modifying FSM, tools, or the agentic loop.

### When Adding/Changing Tools

- [ ] Tool returns `ToolResponse` envelope (use `NewToolSuccess`, `NewToolBlockedError`, etc.)
- [ ] Tool is explicitly classified in `ClassifyToolCall()` (`chat/fsm.go:297-420`)
  - Unknown tools default to `ToolKindWrite` (security-safe)
- [ ] Read operations use `pulse_read` or are classified `ToolKindRead`
- [ ] Write operations are blocked in RESOLVING and require strict resolution
- [ ] Any new write requires verification read before final answer
- [ ] **Routing context validation** for file/exec tools (`validateRoutingContext()`)
  - Block if targeting Proxmox host when child resources (LXC/VM) exist in ResolvedContext
- [ ] Telemetry recorded for blocks (`RecordStrictResolutionBlock`, etc.)
- [ ] Add at least one FSM regression test for new behavior

### When Changing FSM Logic

- [ ] `write → read → write` sequence is supported (regression test must pass)
- [ ] VERIFYING clears on successful `ToolKindRead`
- [ ] `CompleteVerification()` resets appropriate flags
- [ ] Verification gating is structural (code), not prompt-dependent
- [ ] Run all FSM tests: `go test ./internal/ai/chat/... -run FSM`

### When Changing Agentic Loop

- [ ] Gate 1 (tool execution) checks `CanExecuteTool()` before every tool
- [ ] Gate 2 (final answer) checks `CanFinalAnswer()` before responding
- [ ] `OnToolSuccess()` called after every successful tool execution
- [ ] `CompleteVerification()` called when in VERIFYING and `ReadAfterWrite=true`
- [ ] Phantom detection remains active
- [ ] Recovery tracking records attempts and successes

### When Changing Strict Resolution

- [ ] Write actions require discovered resources
- [ ] Read-only commands allowed with scoped bypass (session has context)
- [ ] Error responses include `auto_recoverable: true` and `recovery_hint`
- [ ] Command risk classification handles edge cases (stderr, pipes)
- [ ] Telemetry recorded for all blocks

### Deployment Checklist

After deploying FSM changes, monitor for 24-48h:

```promql
# Should drop after fixing VERIFYING issues
rate(pulse_ai_fsm_tool_block_total{state="VERIFYING"}[5m])

# Should become rarer with proper verification clearing
rate(pulse_ai_fsm_final_block_total{state="VERIFYING"}[5m])

# Should maintain high rate (model self-corrects)
sum(rate(pulse_ai_auto_recovery_success_total[1h]))
/ sum(rate(pulse_ai_auto_recovery_attempt_total[1h]))
```

If VERIFYING blocks don't drop:
1. Check if reads are being classified as `ToolKindRead`
2. Check if reads are failing (look at tool error rates)
3. Check if `CompleteVerification()` is being called

If file operations have no effect:
1. Check for `ROUTING_MISMATCH` errors in tool responses
2. Verify `target_host` matches the intended resource (LXC/VM name, not Proxmox host)
3. Check if child resources are in ResolvedContext but model is targeting parent host

---

## Appendix C: File Reference

| File | Purpose |
|------|---------|
| `chat/fsm.go` | Session workflow state machine |
| `chat/fsm_test.go` | FSM unit tests |
| `chat/agentic.go` | Agentic loop with enforcement gates |
| `chat/session.go` | Session store with FSM/context management |
| `chat/types.go` | ResolvedContext and ResolvedResource |
| `chat/metrics.go` | Prometheus telemetry |
| `tools/protocol.go` | ToolResponse envelope |
| `tools/executor.go` | PulseToolExecutor and interfaces |
| `tools/tools_query.go` | Query tools, strict resolution, and routing validation |
| `tools/tools_read.go` | Read-only operations (exec, file, find, tail, logs) |
| `tools/tools_control.go` | Control tool handlers |
| `tools/tools_control_consolidated.go` | Consolidated pulse_control (write-only) |
| `tools/tools_file.go` | File editing tools with routing validation |
