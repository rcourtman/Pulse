# Pulse Assistant: Technical Deep Dive

This document provides an in-depth look at the engineering behind Pulse Assistant — a safety-gated, tool-driven AI system for infrastructure management that goes far beyond simple chatbots.

---

## Executive Summary

Pulse Assistant isn't a "chat wrapper around an LLM." It's a **protocol-driven, safety-gated agentic system** that:

1. **Treats the LLM as untrusted** — the model proposes, Go code enforces
2. **Proactively gathers context** — understands resources before you ask
3. **Learns within sessions** — extracts and caches facts to avoid redundant queries
4. **Enforces workflow invariants** — FSM prevents dangerous state transitions
5. **Supports parallel tool execution** — efficient batch operations
6. **Detects and prevents hallucinations** — phantom execution detection
7. **Auto-recovers from errors** — structured error envelopes enable self-correction

All while providing streaming responses with real-time tool execution visibility.

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           USER REQUEST                                      │
│                    (with optional @mentions)                                │
└────────────────────────────────┬────────────────────────────────────────────┘
                                 │
                                 ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                       CONTEXT PREFETCHER                                    │
│  • Detects resource mentions (@homepage, "jellyfin")                       │
│  • Resolves structured mentions from frontend autocomplete                  │
│  • Gathers discovery info (ports, config paths, bind mounts)               │
│  • Builds authoritative context summary                                     │
└────────────────────────────────┬────────────────────────────────────────────┘
                                 │
                                 ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                         AGENTIC LOOP                                        │
│  ┌─────────────┐   ┌──────────┐   ┌─────────────────┐   ┌───────────────┐  │
│  │    LLM      │──▶│   FSM    │──▶│  Tool Executor  │──▶│  Knowledge    │  │
│  │ (Proposer)  │   │ (Gating) │   │  (Validation)   │   │ Accumulator   │  │
│  └─────────────┘   └──────────┘   └─────────────────┘   └───────────────┘  │
│        │                │                  │                    │           │
│        ▼                ▼                  ▼                    ▼           │
│  ┌─────────────┐   ┌──────────┐   ┌─────────────────┐   ┌───────────────┐  │
│  │  Phantom    │   │ Telemetry│   │  ResolvedContext│   │   Context     │  │
│  │ Detection   │   │ Counters │   │  (Session Truth)│   │  Compaction   │  │
│  └─────────────┘   └──────────┘   └─────────────────┘   └───────────────┘  │
└────────────────────────────────┬────────────────────────────────────────────┘
                                 │
                                 ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                      AGENT EXECUTION LAYER                                  │
│         (CommandPolicy → AgentServer → Connected Agents)                    │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 1. Context Prefetcher

**📁 Location:** `internal/ai/chat/context_prefetch.go`

### What It Does

Before the LLM even sees your message, the Context Prefetcher **proactively gathers relevant context** about any resources you mention.

### How It Works

```go
type ResourceMention struct {
    Name           string
    ResourceType   string     // "vm", "lxc", "docker", "host", "node", "k8s_pod"
    ResourceID     string
    HostID         string
    MatchedText    string
    BindMounts     []MountInfo  // Docker bind mount mappings
    DockerHostName string       // Full routing chain for nested Docker
    DockerHostType string       // "lxc", "vm", or "host"
    DockerHostVMID int
    ProxmoxNode    string
    TargetHost     string       // THE correct target for commands
}
```

### Two Resolution Modes

1. **Structured Mentions** (preferred): Frontend autocomplete passes fully-resolved resource identities
2. **Fuzzy Matching** (fallback): Text analysis matches resource names from your message

### Fuzzy Matching Intelligence

```go
// Matches: "homepage" → "homepage-docker"
// Matches: "check jellyfin logs" → finds "jellyfin" container
func matchesResource(messageLower string, messageWords []string, resourceName string) bool {
    // Direct containment
    if strings.Contains(messageLower, resourceName) { return true }
    
    // Prefix matching (homepage → homepage-docker)
    for _, word := range messageWords {
        if len(word) >= 4 && strings.HasPrefix(resourceName, word) {
            return true
        }
    }
    // Hyphenated part matching
    // ...
}
```

### Unresolved Mention Handling

If you @mention something that doesn't exist:

```go
// Returns explicit feedback rather than wasting tool calls
sb.WriteString("'myservice' was NOT found in Pulse monitoring.\n")
sb.WriteString("Do NOT use pulse_discovery to search — they are not in the system.\n")
sb.WriteString("Instead: use pulse_control directly if you know the host.\n")
```

### Full Routing Chain for Docker

Docker containers get special treatment — the prefetcher resolves the complete routing chain:

```
## jellyfin (Docker container)
Location: Docker on "media-server" (LXC 141) on Proxmox node "delly"
>>> target_host: "media-server" <<<
Bind mounts (paths on media-server filesystem, NOT inside container):
  /opt/jellyfin/config → /config
  /mnt/media → /media
```

This prevents the common mistake of running commands on the Proxmox node instead of the LXC.

---

## 2. Knowledge Accumulator

**📁 Location:** `internal/ai/chat/knowledge_accumulator.go`

### What It Does

The Knowledge Accumulator **extracts and caches facts** from every tool result during a session. This prevents redundant queries and helps the model remember what it's already learned.

### Fact Categories

```go
const (
    FactCategoryResource  = "resource"   // VM/LXC/container status
    FactCategoryStorage   = "storage"    // Pool usage, backup info
    FactCategoryAlert     = "alert"      // Active alerts, findings
    FactCategoryDiscovery = "discovery"  // Ports, paths, services
    FactCategoryExec      = "exec"       // Command outputs
    FactCategoryMetrics   = "metrics"    // Performance data
    FactCategoryFinding   = "finding"    // Patrol findings
)
```

### Fact Structure

```go
type Fact struct {
    Category   FactCategory
    Key        string      // Dedup key: "lxc:delly:106:status"
    Value      string      // Compact value: "running, Postfix, hostname=patrol-test"
    ObservedAt time.Time
    Turn       int         // Which agentic turn observed this
}
```

### Bounded, Session-Scoped

```go
const (
    defaultMaxEntries = 60     // Max facts stored
    defaultMaxChars   = 2000   // Max total characters
    maxValueLen       = 200    // Max per-fact value length
)
```

### LRU Eviction with Turn Pinning

Facts from the current or previous turn are "soft-pinned" and won't be evicted:

```go
func (ka *KnowledgeAccumulator) evict() {
    // ...
    // Soft-pin: don't evict facts from current or previous turn
    if fact.Turn >= ka.currentTurn-1 {
        continue
    }
    // Evict oldest facts first
}
```

### Knowledge Gate in Agentic Loop

Before executing a tool, the loop checks if we already have the answer:

```go
// KNOWLEDGE GATE: Return cached facts for redundant tool calls
if keys := PredictFactKeys(tc.Name, tc.Input); len(keys) > 0 {
    if cachedValue, found := ka.Lookup(key); found {
        return "Already known (from earlier investigation): " + cachedValue
    }
}
```

---

## 3. Knowledge Extractor

**📁 Location:** `internal/ai/chat/knowledge_extractor.go`

### What It Does

Deterministically parses tool results and extracts structured facts — **no LLM calls** required.

### Coverage

The extractor handles all major tools:

| Tool | Facts Extracted |
|------|-----------------|
| `pulse_query` | Resource status, health, topology, configs |
| `pulse_storage` | Pool usage, backup tasks, disk health |
| `pulse_discovery` | Ports, config paths, log paths, services |
| `pulse_read` | Command outputs (keyed by command) |
| `pulse_metrics` | CPU, memory, disk averages, baselines |
| `pulse_alerts` | Active alerts, findings with severity |
| `pulse_docker` | Container states, update availability |
| `pulse_kubernetes` | Pod counts, deployment status |

### Example Extraction

For `pulse_query action=get`:

```go
func extractQueryGetFacts(input, resultText string) []FactEntry {
    // Parse JSON response
    // Extract: status, cpu_avg, mem_avg, disk_avg, hostname
    // Key format: "lxc:delly:106:status"
    // Value format: "running, cpu:12%, mem:45%, disk:67%"
}
```

### Negative Markers

If a query returns an error or empty result, a "negative marker" is stored:

```go
// Key: "lxc:delly:106:status:queried"
// Value: "not_found" or "error"
```

This prevents the model from re-querying resources that don't exist.

---

## 4. Finite State Machine (FSM)

**📁 Location:** `internal/ai/chat/fsm.go`

### What It Does

The FSM enforces **workflow invariants** that prevent dangerous state transitions. The model cannot bypass these — they're enforced in Go code.

### States

```go
const (
    StateResolving  = "RESOLVING"  // No target yet, must discover first
    StateReading    = "READING"    // Read tools allowed, can explore
    StateWriting    = "WRITING"    // Write in progress (transitional)
    StateVerifying  = "VERIFYING"  // Must read/verify before final answer
)
```

### Tool Classification

```go
const (
    ToolKindResolve  // Discovery/query (pulse_query, pulse_discovery)
    ToolKindRead     // Read-only (pulse_read, pulse_metrics, pulse_storage)
    ToolKindWrite    // Mutating (pulse_control, pulse_file_edit write)
)
```

### State Transitions

```
       ┌───────────────────────────────────────────────┐
       │                                               │
       ▼                                               │
  ┌──────────┐  resolve/read   ┌─────────┐   read    │
  │RESOLVING │ ───────────────▶│ READING │◀──────────┤
  └──────────┘                 └────┬────┘           │
                                    │                │
                               write│                │
                                    ▼                │
                              ┌───────────┐  verify  │
                              │ VERIFYING │──────────┘
                              └───────────┘
                                    │
                           write blocked!
```

### Key Invariants Enforced

1. **Discovery Before Action**: Can't write to undiscovered resources
2. **Verification After Write**: Must read/check after any mutation
3. **Read/Write Tool Separation**: `pulse_read` never triggers VERIFYING

### Error Response for FSM Blocks

```json
{
    "error": {
        "code": "FSM_BLOCKED",
        "message": "Must verify the previous write operation",
        "blocked": true,
        "recoverable": true,
        "recovery_hint": "Use a read tool to check the result first"
    }
}
```

---

## 5. Resolved Context

**📁 Location:** `internal/ai/chat/types.go`

### What It Does

ResolvedContext is the **session-scoped source of truth** for discovered resources. The model cannot fabricate resource IDs — they must come from successful discovery.

### Resource Structure

```go
type ResolvedResource struct {
    // Structured Identity
    Kind        string        // "lxc", "vm", "docker_container", "node"
    ProviderUID string        // Stable ID from provider
    Scope       ResourceScope // Host, parent, cluster, namespace
    Aliases     []string      // All names this resource is known by
    
    // Routing
    ReachableVia   []ExecutorPath  // All ways to reach this resource
    TargetHost     string          // Primary command target
    AllowedActions []string        // What can be done
    
    // Proxmox-specific
    VMID int
    Node string
}
```

### Canonical Resource ID Format

```
{kind}:{host}:{provider_uid}   # Scoped resources
lxc:delly:141                  # LXC 141 on node delly
docker_container:media-server:abc123  # Docker on host media-server

{kind}:{provider_uid}          # Global resources
node:delly                     # Proxmox node
```

### TTL and LRU Eviction

```go
const (
    DefaultResolvedContextTTL        = 45 * time.Minute
    DefaultResolvedContextMaxEntries = 500
)
```

### Explicit vs General Access Tracking

**Critical distinction** that prevents false positives:

| Tracking | Set By | Purpose |
|----------|--------|---------|
| `lastAccessed` | Every add/get | LRU eviction, TTL expiry |
| `explicitlyAccessed` | User intent only | Routing validation |

Why this matters:
- Bulk discovery adds many resources → sets `lastAccessed` for all
- If routing validation used `lastAccessed`, bulk discovery would block host operations
- Instead, routing checks `explicitlyAccessed` which is only set for single-resource operations

---

## 6. Parallel Tool Execution

**📁 Location:** `internal/ai/chat/agentic.go`

### What It Does

When the LLM requests multiple tool calls, compatible operations execute **in parallel** for efficiency.

### Three-Phase Pipeline

```
Phase 1: Pre-check (sequential)
├── FSM validation
├── Loop detection
└── Knowledge gate

Phase 2: Execute (parallel, max 4 concurrent)
├── Tool 1 ────────────┐
├── Tool 2 ─────────┐  │
├── Tool 3 ───────┐ │  │
└── Tool 4 ─────┐ │ │  │
                ▼ ▼ ▼  ▼
              Results

Phase 3: Post-process (sequential)
├── Stream events to UI
├── FSM state transitions
├── Knowledge extraction
└── Approval flow (if needed)
```

### Concurrency Control

```go
var wg sync.WaitGroup
sem := make(chan struct{}, 4)  // Cap at 4 concurrent

for j, pe := range pendingExec {
    wg.Add(1)
    go func(idx int, tc providers.ToolCall) {
        defer wg.Done()
        sem <- struct{}{}         // Acquire
        defer func() { <-sem }()  // Release
        
        r, e := a.executor.ExecuteTool(ctx, tc.Name, tc.Input)
        execResults[idx] = parallelToolResult{Result: r, Err: e}
    }(j, pe.tc)
}
wg.Wait()
```

---

## 7. Loop Detection

**📁 Location:** `internal/ai/chat/agentic.go`

### What It Does

Prevents the model from getting stuck calling the same tool repeatedly.

### Implementation

```go
const maxIdenticalCalls = 3
recentCallCounts := make(map[string]int)

func toolCallKey(name string, input map[string]interface{}) string {
    inputJSON, _ := json.Marshal(input)
    return name + ":" + string(inputJSON)
}

// In the loop:
callKey := toolCallKey(tc.Name, tc.Input)
recentCallCounts[callKey]++
if recentCallCounts[callKey] > maxIdenticalCalls {
    return "LOOP_DETECTED: You have called " + tc.Name + 
           " with the same arguments " + count + " times. Blocked."
}
```

---

## 8. Phantom Execution Detection

**📁 Location:** `internal/ai/chat/agentic.go`

### What It Does

Detects when the model **claims to have done something** but never actually called tools. This catches hallucinations.

### Pattern Detection

```go
func hasPhantomExecution(content string) bool {
    // Only flag if tools haven't succeeded this episode
    // Look for phrases like:
    // - "I have restarted..."
    // - "Successfully stopped..."
    // - "The service has been..."
    // - "Done! I've..."
}
```

### Safe Response on Detection

```go
safeResponse := "I apologize, but I wasn't able to access the infrastructure " +
    "tools needed to complete that request. This can happen when:\n\n" +
    "1. The tools aren't available right now\n" +
    "2. There was a connection issue\n" +
    "3. The model I'm running on doesn't support function calling\n\n" +
    "Please try again, or let me know if you have a question I can " +
    "answer without checking live infrastructure."
```

---

## 9. Context Compaction

**📁 Location:** `internal/ai/chat/agentic.go`

### What It Does

Compacts old tool results to prevent context window exhaustion in long conversations.

### Strategy

```go
const compactionKeepTurns = 2   // Keep last 2 turns in full
const compactionMinChars = 300  // Only compact results > 300 chars

func compactOldToolResults(messages, turnStartIdx, keepTurns, minChars int, ka *KnowledgeAccumulator) {
    // For results older than keepTurns:
    // 1. Replace with fact summary from KnowledgeAccumulator
    // 2. Or truncate to first 200 chars + "[truncated]"
}
```

### Wrap-Up Nudges

After many tool calls, the loop hints the model to wrap up:

```go
const wrapUpNudgeAfterCalls = 12
const wrapUpEscalateAfterCalls = 18

// After 12 calls: "Consider summarizing your findings"
// After 18 calls: "Please provide your final answer now"
```

---

## 10. Execution Intent Classification

**📁 Location:** `internal/ai/tools/tools_query.go`

### What It Does

Classifies commands as read-only or potentially mutating — **deterministically, without LLM judgment**.

### Intent Levels

```go
const (
    IntentReadOnlyCertain      // Non-mutating by construction (cat, grep, docker logs)
    IntentReadOnlyConditional  // Proven read-only by content (SELECT queries)
    IntentWriteOrUnknown       // Cannot prove safe (unknown, or has mutation patterns)
)
```

### Classification Phases (Order Matters!)

```
Phase 1: Mutation-capability guards
├── Block: sudo, redirects, pipes to dual-use tools, shell chaining

Phase 2: Known write patterns
├── Block: rm, shutdown, systemctl restart, DROP DATABASE

Phase 3: Read-only by construction
├── Allow: cat, grep, ls, docker logs, ffprobe, kubectl get

Phase 4: Content inspection
├── SQL: SELECT → allow, INSERT/UPDATE/DELETE → block
├── Redis: GET → allow, SET/DEL → block

Phase 5: Conservative fallback
└── Unknown → IntentWriteOrUnknown
```

**Critical**: Phase 1-2 **dominate** — a command matching known write patterns is blocked even if it also matches read-only patterns.

### Content Inspectors

```go
type ContentInspector interface {
    Applies(cmdLower string) bool
    IsReadOnly(cmdLower string) (bool, string)
}

// Example: SQL inspector
type sqlContentInspector struct{}

func (s *sqlContentInspector) IsReadOnly(cmd string) (bool, string) {
    // Parse SQL, check for SELECT only
    // Block: INSERT, UPDATE, DELETE, DROP, TRUNCATE, ALTER
}
```

---

## 11. Strict Resolution

**📁 Location:** `internal/ai/tools/tools_query.go`

### What It Does

Prevents the model from operating on **fabricated or hallucinated resource IDs**.

### Error Response

```go
type ErrStrictResolution struct {
    Action      string
    Name        string
    ResourceID  string
    Message     string
    Suggestions []string  // Discovered resources with similar names
}

func (e *ErrStrictResolution) ToToolResponse() ToolResponse {
    return ToolResponse{
        OK: false,
        Error: &ToolError{
            Code:    "STRICT_RESOLUTION",
            Message: e.Message,
            Blocked: true,
            Details: map[string]interface{}{
                "recovery_hint": "Use pulse_query to discover resources first",
                "auto_recoverable": true,
            },
        },
    }
}
```

---

## 12. Routing Mismatch Detection

**📁 Location:** `internal/ai/tools/tools_query.go`

### What It Does

Prevents accidentally operating on a parent host when you meant to target a child resource.

### Scenario

1. User asks "edit config on @jellyfin" (Docker container on LXC)
2. Model calls `pulse_file_edit` with `target_host="delly"` (the Proxmox node)
3. File happens to exist on delly too (shared path)
4. **Wrong file gets edited!**

### Detection

```go
type ErrRoutingMismatch struct {
    TargetHost              string
    MoreSpecificResources   []string  // ["jellyfin"]
    MoreSpecificResourceIDs []string  // ["docker_container:media-server:abc123"]
    TargetResourceID        string
    Message                 string
}
```

### Error Response

```json
{
    "error": {
        "code": "ROUTING_MISMATCH",
        "message": "target_host 'delly' is a Proxmox node, but you recently referenced more specific resources: [jellyfin]",
        "details": {
            "target_resource_id": "docker_container:media-server:abc123",
            "recovery_hint": "Retry with target_host='media-server'",
            "auto_recoverable": true
        }
    }
}
```

---

## 13. Approval Flow

**📁 Location:** `internal/ai/chat/agentic.go`, `internal/ai/approval/`

### What It Does

In "Controlled" mode, write operations require explicit user approval before execution.

### Flow

```
1. Tool returns APPROVAL_REQUIRED
   ├── approval_id
   ├── command
   ├── risk_level
   └── description

2. Agentic loop emits approval_needed SSE event

3. UI shows approval card to user

4. User approves/denies via API
   POST /api/ai/approvals/{id}/approve
   POST /api/ai/approvals/{id}/deny

5. On approve: Tool re-executes with _approval_id
   On deny: Assistant responds "Command denied: <reason>"
```

### Autonomous Mode

For investigations, approvals can be bypassed:

```go
func (a *AgenticLoop) SetAutonomousMode(enabled bool) {
    a.mu.Lock()
    a.autonomousMode = enabled
    a.mu.Unlock()
}
```

---

## 14. Token Budget Management

**📁 Location:** `internal/ai/chat/agentic.go`

### Token Tracking

```go
// Per-turn tracking
a.totalInputTokens += data.InputTokens
a.totalOutputTokens += data.OutputTokens

// After each turn:
if a.budgetChecker != nil {
    if err := a.budgetChecker(); err != nil {
        return resultMessages, fmt.Errorf("budget exceeded: %w", err)
    }
}
```

### Dynamic Turn Limits

```go
// Force text-only on last turn to get a summary
if turn >= maxTurns-1 {
    req.ToolChoice = &providers.ToolChoice{Type: providers.ToolChoiceNone}
}

// After write completion, force summary (prevents stale-data loops)
if writeCompletedLastTurn {
    req.ToolChoice = &providers.ToolChoice{Type: providers.ToolChoiceNone}
}
```

---

## 15. DeepSeek Artifact Cleanup

**📁 Location:** `internal/ai/chat/agentic.go`

### What It Does

DeepSeek models sometimes leak internal markup in responses. The Assistant cleans this:

```go
func containsDeepSeekMarker(text string) bool {
    return strings.Contains(text, "<｜DSML｜") ||
           strings.Contains(text, "<｜end▁of▁thinking｜>")
}

func cleanDeepSeekArtifacts(content string) string {
    // Remove:
    // - <｜DSML｜function_calls>...</｜DSML｜>
    // - <｜end▁of▁thinking｜>
    // - Internal reasoning markers
}
```

---

## 16. Tool Protocol

**📁 Location:** `internal/ai/tools/protocol.go`

### Consistent Error Envelopes

All tools return the same structure, enabling auto-recovery:

```go
type ToolResponse struct {
    OK    bool        `json:"ok"`
    Data  interface{} `json:"data,omitempty"`
    Error *ToolError  `json:"error,omitempty"`
    Meta  map[string]interface{} `json:"meta,omitempty"`
}

type ToolError struct {
    Code      string                 `json:"code"`
    Message   string                 `json:"message"`
    Blocked   bool                   `json:"blocked,omitempty"`
    Failed    bool                   `json:"failed,omitempty"`
    Retryable bool                   `json:"retryable,omitempty"`
    Details   map[string]interface{} `json:"details,omitempty"`
}
```

### Error Codes

| Code | Meaning | Auto-Recoverable |
|------|---------|------------------|
| `STRICT_RESOLUTION` | Resource not discovered | Yes (discover then retry) |
| `FSM_BLOCKED` | FSM state prevents operation | Yes (perform required action) |
| `ROUTING_MISMATCH` | Wrong target host | Yes (use correct target) |
| `APPROVAL_REQUIRED` | User approval needed | Yes (wait for approval) |
| `NOT_FOUND` | Resource doesn't exist | No |
| `POLICY_BLOCKED` | Security policy blocked | No |
| `EXECUTION_FAILED` | Runtime error | Depends |

---

## 17. Telemetry & Metrics

**📁 Location:** `internal/ai/chat/metrics.go`

### Key Metrics Collected

```go
// Agentic loop iterations
metrics.RecordAgenticIteration(provider, model)

// FSM blocks
metrics.RecordFSMToolBlock(state, toolName, toolKind)
metrics.RecordFSMFinalBlock(state)

// Phantom detection
metrics.RecordPhantomDetected(provider, model)

// Auto-recovery
metrics.RecordAutoRecoveryAttempt(errorCode, toolName)
metrics.RecordAutoRecoverySuccess(errorCode, toolName)

// Routing mismatches
pulse_ai_routing_mismatch_block_total{tool, target_kind, child_kind}
```

---

## Files Reference

| File | Purpose |
|------|---------|
| `internal/ai/chat/service.go` | Chat service orchestration |
| `internal/ai/chat/session.go` | Session lifecycle management |
| `internal/ai/chat/agentic.go` | Core agentic loop (2289 lines) |
| `internal/ai/chat/fsm.go` | Finite state machine |
| `internal/ai/chat/types.go` | ResolvedContext, ResolvedResource |
| `internal/ai/chat/context_prefetch.go` | Proactive context gathering |
| `internal/ai/chat/knowledge_accumulator.go` | Fact caching |
| `internal/ai/chat/knowledge_extractor.go` | Deterministic fact extraction |
| `internal/ai/tools/tools_query.go` | Query/discovery tools, strict resolution |
| `internal/ai/tools/tools_read.go` | Read-only execution, intent classification |
| `internal/ai/tools/tools_control.go` | Write operations |
| `internal/ai/tools/protocol.go` | ToolResponse envelope |
| `internal/ai/approval/` | Approval flow management |

---

## Summary

Pulse Assistant is engineered with **safety as a first-class concern**:

1. **LLM as proposer, Go as enforcer** — the model suggests, code validates
2. **Proactive context** — understands resources before you ask
3. **Session-scoped learning** — remembers what it's discovered
4. **Workflow enforcement** — FSM prevents dangerous transitions
5. **Parallel execution** — efficient batch operations
6. **Hallucination detection** — phantom execution caught and handled
7. **Auto-recovery** — structured errors enable self-correction
8. **Routing validation** — prevents wrong-target mistakes

This architecture ensures the Assistant is both **powerful** (can control infrastructure) and **safe** (can't cause unintended damage).
