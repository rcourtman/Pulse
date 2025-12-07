# Unified Resource Architecture

## Status: Draft
## Author: AI Assistant + rcourtman
## Created: 2025-12-07
## Last Updated: 2025-12-07

---

## Executive Summary

Pulse is evolving from a traditional "stare at dashboards" monitoring tool to an **AI-first infrastructure management platform** where AI is the primary operator and humans are supervisors. This document outlines the architectural changes needed to support this vision while preserving the excellent UI that 20,000+ users love.

The core change is introducing a **Unified Resource Model** - a common data abstraction that all platforms (Proxmox, Docker, Hosts, and future platforms like Kubernetes and TrueNAS) normalize to. This enables:

1. **AI intelligence across all platforms** - AI can reason about your entire infrastructure
2. **Elimination of duplicate monitoring** - One machine = one set of alerts
3. **Extensibility for new platforms** - Adding Kubernetes is adding a new resource type, not a new architecture
4. **Foundation for unified views** - Optional consolidated UI without breaking existing pages

---

## Problem Statement

### Current Architecture Issues

#### 1. Multiple Data Sources for Same Machine
When a Proxmox node also has a host agent running, Pulse monitors it twice:
- Via Proxmox API → "Node cpu at 97.9%"
- Via Host Agent → "Host cpu at 99.7%"

Result: Duplicate alerts, user confusion.

#### 2. Siloed Data Models
Each platform has its own types with no common abstraction:
```
Node, VM, Container           (Proxmox)
DockerHost, DockerContainer   (Docker)
Host                          (Host Agent)
PBSInstance, PBSDatastore     (PBS)
PMGInstance                   (PMG)
```

AI can't easily answer: "What's using the most CPU across my infrastructure?"

#### 3. Platform-Specific Pages
The frontend has separate components for each platform:
- Dashboard (Proxmox VMs/Containers)
- Docker page
- Hosts page
- Storage page

Adding a new platform (Kubernetes) means building an entirely new page.

#### 4. Agent Capabilities Underutilized
The unified pulse-agent can:
- Report richer metrics than Proxmox API
- Execute commands for AI
- Monitor temperature, RAID, disk I/O at granular level

But the architecture doesn't fully leverage this.

---

## Vision: AI-First Monitoring

```
┌────────────────────────────────────────────────────────────────────────┐
│                         TRADITIONAL MONITORING                         │
│                                                                        │
│   Human 👁️ ──watches──▶ Dashboard 📊 ──shows──▶ Metrics/Alerts        │
│      │                                                                 │
│      └──────────────────── investigates ──▶ fixes manually            │
└────────────────────────────────────────────────────────────────────────┘

                                 ⬇️

┌────────────────────────────────────────────────────────────────────────┐
│                         PULSE: AI-FIRST                                │
│                                                                        │
│   Infrastructure 🖥️ ──reports to──▶ Pulse AI 🤖                        │
│                                          │                             │
│                                          ├── Detects anomalies         │
│                                          ├── Correlates across systems │
│                                          ├── Diagnoses root causes     │
│                                          ├── Suggests fixes            │
│                                          └── Executes fixes (approved) │
│                                                    │                   │
│                                                    ▼                   │
│   Human 👤 ──reviews/approves──▶ Dashboard = CONTEXT, not PRIMARY     │
└────────────────────────────────────────────────────────────────────────┘
```

The dashboard remains beautiful and functional, but AI becomes the primary interface for investigation and remediation.

---

## Solution: Unified Resource Model

### Core Abstraction

Every monitored entity becomes a `Resource` with common fields:

```go
// Resource is the universal abstraction for any monitored entity
type Resource struct {
    // Identity
    ID           string       `json:"id"`           // Globally unique ID
    Type         ResourceType `json:"type"`         // vm, container, docker-container, pod, host, etc.
    Name         string       `json:"name"`         // Human-readable name
    DisplayName  string       `json:"displayName"`  // Custom display name (if set)
    
    // Platform/Source
    PlatformID   string       `json:"platformId"`   // Which platform instance
    PlatformType PlatformType `json:"platformType"` // proxmox-pve, docker, kubernetes, etc.
    SourceType   SourceType   `json:"sourceType"`   // api, agent, hybrid
    
    // Hierarchy
    ParentID     string       `json:"parentId,omitempty"` // VM → Node, Pod → K8s Node
    ClusterID    string       `json:"clusterId,omitempty"` // Cluster membership
    
    // Universal Metrics (nullable - not all resources have all metrics)
    Status       ResourceStatus `json:"status"`       // online, offline, running, stopped, degraded
    CPU          *MetricValue   `json:"cpu,omitempty"`
    Memory       *MetricValue   `json:"memory,omitempty"`
    Disk         *MetricValue   `json:"disk,omitempty"`
    Network      *NetworkMetric `json:"network,omitempty"`
    Temperature  *float64       `json:"temperature,omitempty"`
    Uptime       *int64         `json:"uptime,omitempty"`
    
    // Universal Metadata
    Tags         []string               `json:"tags,omitempty"`
    Labels       map[string]string      `json:"labels,omitempty"`
    LastSeen     time.Time              `json:"lastSeen"`
    Alerts       []ResourceAlert        `json:"alerts,omitempty"`
    
    // Platform-Specific Data (discriminated by Type)
    // This preserves all the rich data while allowing common handling
    PlatformData json.RawMessage `json:"platformData,omitempty"`
}

type ResourceType string

const (
    // Infrastructure
    ResourceTypeNode            ResourceType = "node"
    ResourceTypeHost            ResourceType = "host"
    ResourceTypeDockerHost      ResourceType = "docker-host"
    ResourceTypeK8sNode         ResourceType = "k8s-node"
    ResourceTypeTrueNASSystem   ResourceType = "truenas-system"
    
    // Compute Workloads
    ResourceTypeVM              ResourceType = "vm"
    ResourceTypeContainer       ResourceType = "container"       // LXC
    ResourceTypeDockerContainer ResourceType = "docker-container"
    ResourceTypePod             ResourceType = "pod"
    ResourceTypeJail            ResourceType = "jail"
    
    // Services
    ResourceTypeDockerService   ResourceType = "docker-service"
    ResourceTypeK8sDeployment   ResourceType = "k8s-deployment"
    ResourceTypeK8sService      ResourceType = "k8s-service"
    
    // Storage
    ResourceTypeStorage         ResourceType = "storage"
    ResourceTypeDatastore       ResourceType = "datastore"
    ResourceTypePool            ResourceType = "pool"
    ResourceTypeDataset         ResourceType = "dataset"
    
    // Backup Systems
    ResourceTypePBS             ResourceType = "pbs"
    ResourceTypePMG             ResourceType = "pmg"
)

type PlatformType string

const (
    PlatformProxmoxPVE  PlatformType = "proxmox-pve"
    PlatformProxmoxPBS  PlatformType = "proxmox-pbs"
    PlatformProxmoxPMG  PlatformType = "proxmox-pmg"
    PlatformDocker      PlatformType = "docker"
    PlatformKubernetes  PlatformType = "kubernetes"
    PlatformTrueNAS     PlatformType = "truenas"
    PlatformHostAgent   PlatformType = "host-agent"
)

type SourceType string

const (
    SourceAPI    SourceType = "api"    // Data from polling an API
    SourceAgent  SourceType = "agent"  // Data pushed from agent
    SourceHybrid SourceType = "hybrid" // Both sources, agent preferred
)
```

### Deduplication Strategy

When multiple sources report on the same physical machine:

```go
type ResourceIdentity struct {
    Hostname  string   // Primary identifier
    MachineID string   // /etc/machine-id or equivalent
    IPs       []string // Network addresses
}

// IdentityMatcher determines if two resources are the same machine
func (r *ResourceStore) IdentityMatcher(a, b Resource) bool {
    // 1. Machine ID match (most reliable)
    if a.MachineID != "" && a.MachineID == b.MachineID {
        return true
    }
    
    // 2. Hostname match (case-insensitive)
    if strings.EqualFold(a.Hostname, b.Hostname) {
        return true
    }
    
    // 3. IP overlap (if same IP, likely same machine)
    for _, ipA := range a.IPs {
        for _, ipB := range b.IPs {
            if ipA == ipB && !isLocalhost(ipA) {
                return true
            }
        }
    }
    
    return false
}
```

When duplicates are detected, prefer **Agent > API**:

| Scenario | Result |
|----------|--------|
| Node only (no agent) | Use Node data |
| Host agent only (no Proxmox) | Use Host data |
| Both Node + Host agent | Use Host agent data, suppress Node alerts |
| Docker agent on Proxmox node | Combine: Docker data + Host metrics from agent |

---

## Implementation Phases

### Phase 0: Document & Design (This Document)
- [x] Capture the vision
- [x] Define the data model
- [ ] Review with stakeholders
- [ ] Create GitHub issues/milestones

### Phase 1: Backend Unification (Invisible to Users)

**Goal:** Create the Resource abstraction without changing any frontend behavior.

**Status:** ✅ Core implementation complete

**Completed Tasks:**
1. ✅ Create `internal/resources/resource.go` with types - Core Resource type, enums, and helper methods
2. ✅ Create `internal/resources/platform_data.go` - Platform-specific data types for PlatformData field
3. ✅ Create `internal/resources/store.go` for unified storage - In-memory store with indexes
4. ✅ Create `internal/resources/converters.go` - Converters: FromNode(), FromHost(), FromDockerHost(), FromVM(), FromContainer(), FromDockerContainer(), FromPBSInstance(), FromStorage()
5. ✅ Add deduplication logic using identity matching (hostname, machineID, IP)
6. ✅ Create `internal/resources/converters_test.go` and `store_test.go` - 25 passing tests
7. ✅ Create `internal/api/resource_handlers.go` - HTTP handlers for unified resources API
8. ✅ Add `/api/resources` endpoint with filtering (type, platform, status, parent, infrastructure, workloads)
9. ✅ Add `/api/resources/stats` endpoint for store statistics
10. ✅ Add `/api/resources/{id}` endpoint for individual resource lookup
11. ✅ Add deduplication helper methods for alert manager: `IsSuppressed()`, `GetPreferredResourceFor()`, `IsSamePhysicalMachine()`, `HasPreferredSourceForHostname()`

**Integration Points for Alert Manager:**
The resource store provides these methods that the existing alert manager can optionally use:
- `store.IsSuppressed(resourceID)` - Check if a resource was deduplicated
- `store.GetPreferredResourceFor(resourceID)` - Get the preferred resource
- `store.HasPreferredSourceForHostname(hostname)` - Check if an agent is preferred for a hostname

**Remaining Tasks:**
- [ ] *(Optional)* Enhance alert manager to use resource store methods instead of hostname-only dedup
- [ ] *(Deferred to Phase 2)* Add optional `resources` array to WebSocket state

**Backward Compatibility:**
- WebSocket still sends `nodes`, `vms`, `containers`, `hosts`, `dockerHosts` as separate arrays
- Unified resources available via REST API at `/api/resources`
- All existing frontend code continues to work unchanged
- Existing hostname-based deduplication in alert manager still works

**Metrics:** 
- Zero frontend changes
- All existing tests pass
- New resource tests added (25 tests)
- New REST API endpoints available

### Phase 2: AI Context Enhancement

**Goal:** AI chat can query and act across all resource types.

**Status:** ✅ Fully complete

**Completed Tasks:**
1. ✅ Create `internal/ai/resource_context.go` - Unified context builder for AI
2. ✅ Add `ResourceProvider` interface to AI service
3. ✅ Modify `buildSystemPrompt` to use unified model when available
4. ✅ Wire up resource provider in router to AI handlers
5. ✅ AI now uses deduplicated view of infrastructure (falls back to legacy if not available)
6. ✅ Add cross-platform query methods: `GetTopByCPU()`, `GetTopByMemory()`, `GetTopByDisk()`
7. ✅ Add resource correlation: `GetRelated()` for parent/children/siblings/cluster members
8. ✅ Add infrastructure summary: `GetResourceSummary()` with status counts and averages
9. ✅ AI context includes "Top CPU Consumers", "Top Memory Consumers", "Top Disk Usage"
10. ✅ AI context includes infrastructure summary with health status

**How It Works:**
- When `ResourceProvider` is set, AI gets a cleaner "Unified Infrastructure View"
- Resources grouped by platform (Proxmox nodes, Standalone hosts, Docker hosts)
- Workloads grouped by parent infrastructure
- Agent status shown inline with infrastructure
- Resources with alerts highlighted
- **Top consumers** shown for CPU, Memory, and Disk
- **Infrastructure summary** with healthy/degraded/offline counts

**Cross-Platform Query Methods:**
```go
// Find top resource consumers across all platforms
store.GetTopByCPU(10, nil)    // Top 10 by CPU, any type
store.GetTopByMemory(5, []ResourceType{ResourceTypeVM}) // Top 5 VMs by memory

// Find related resources
store.GetRelated("vm-123")    // Returns parent, children, siblings, cluster_members

// Get infrastructure overview
store.GetResourceSummary()    // TotalResources, Healthy, Degraded, Offline, ByType, ByPlatform
```

**User Experience:**
- AI can now answer "What's using the most CPU?" across all platforms
- AI knows about resource relationships (parent nodes, sibling VMs, cluster members)
- AI has infrastructure summary context for better analysis

### Phase 3: Agent Preference & Hybrid Mode

**Goal:** When agents exist, prefer their data over API polling.

**Status:** ✅ Core implementation complete

**Completed Tasks:**
1. ✅ Polling optimization methods added to resource store:
   - `ShouldSkipAPIPolling(hostname)` - Check if API polling should be skipped
   - `GetAgentMonitoredHostnames()` - Get list of agent-monitored hosts
   - `GetPollingRecommendations()` - Get per-hostname polling multipliers
2. ✅ Hybrid source type defined (`SourceHybrid`)
3. ✅ Agent data automatically preferred over API data (store deduplication)
4. ✅ `ResourceStoreInterface` added to Monitor
5. ✅ `SetResourceStore()` method added to inject store into Monitor
6. ✅ `shouldSkipNodeMetrics()` helper method added to Monitor
7. ✅ Resource store wired into Monitor via `Router.SetMonitor()`

**How It Works:**
```go
// In Monitor struct
resourceStore ResourceStoreInterface

// Router injects the store when setting the monitor
func (r *Router) SetMonitor(m *monitoring.Monitor) {
    // ... other setup ...
    if r.resourceHandlers != nil {
        m.SetResourceStore(r.resourceHandlers.Store())
    }
}

// Monitor can now check if polling should be skipped
func (m *Monitor) shouldSkipNodeMetrics(nodeName string) bool {
    if store := m.resourceStore; store != nil {
        return store.ShouldSkipAPIPolling(nodeName)
    }
    return false
}
```

**How to Enable Polling Optimization:**

To enable polling optimization in the actual polling loops, add this check in `pollPVENode()`:

```go
// In monitor_polling.go, at the start of pollPVENode():
func (m *Monitor) pollPVENode(...) (models.Node, string, error) {
    // Skip detailed metric polling if host agent provides data
    if m.shouldSkipNodeMetrics(node.Node) {
        // Still return basic node info but skip expensive API calls
        // like GetNodeStatus, GetStorage, etc.
    }
    // ... rest of function
}
```

**Remaining Tasks (Future Enhancement):**
- [ ] Add config flag: `EnableAgentPollingOptimization bool`
- [ ] Actually integrate skip logic into `pollPVENode()` and related functions
- [ ] Add Prometheus metrics for skipped polls
- [ ] Add logging for poll optimization decisions

**Benefits:**
- Better data quality (agent metrics are more accurate)
- Reduced API load (skip redundant polling)
- More AI capabilities (command execution via agents)

### Phase 4: Optional Unified View (Future)

**Goal:** Add a consolidated "All Resources" view for power users.

**Tasks:**
1. Create new React/Solid component for unified resource table
2. Implement filtering by platform, type, status, tags
3. Support hierarchical grouping (cluster > node > workloads)
4. Add as new optional view, don't replace existing pages

**User Experience:**
- New "All Resources" option in navigation
- Existing pages unchanged
- Users choose their preferred view

### Phase 5: New Platform Support (Ongoing)

Each new platform follows the pattern:

1. **Collector:** Poll API or receive agent telemetry
2. **Converter:** `PlatformDataToResource()` function
3. **Platform Data:** Type-specific struct stored in `PlatformData` field
4. **UI Components:** (Optional) Platform-specific detail views

Example for Kubernetes:
```go
func K8sNodeToResource(node k8s.Node) Resource {
    return Resource{
        ID:           fmt.Sprintf("k8s/%s/%s", clusterID, node.Name),
        Type:         ResourceTypeK8sNode,
        Name:         node.Name,
        PlatformType: PlatformKubernetes,
        SourceType:   SourceAPI,
        Status:       mapK8sNodeStatus(node.Status),
        CPU:          extractK8sCPU(node),
        Memory:       extractK8sMemory(node),
        PlatformData: marshalK8sNodeData(node),
    }
}
```

---

## Migration Strategy

### Database Considerations

Current state is in-memory with JSON persistence for alerts. The unified model should:

1. **Short-term:** Continue in-memory, Resource is just an abstraction layer
2. **Medium-term:** SQLite metrics store (already implemented) extended for resources
3. **Long-term:** Consider time-series DB for metrics history

### API Compatibility

**Existing endpoints remain unchanged:**
- `/api/state` - Returns current format
- WebSocket `state` message - Returns current format

**New optional endpoints:**
- `/api/resources` - Returns unified resource list
- `/api/resources/{id}` - Returns single resource with full detail
- WebSocket `resources` message - Unified resource updates

### Frontend Migration

**No forced migration.** Existing components continue to work. New unified view is additive.

If we eventually want to migrate existing components:
1. Create `useResources()` hook that abstracts data source
2. Components can switch when ready
3. Old and new can coexist

---

## Success Metrics

### Phase 1 Complete When:
- [x] Resource type defined and tested
- [x] All current types have converters
- [x] Deduplication prevents duplicate alerts (store logic complete)
- [x] Zero frontend changes required
- [x] All existing tests pass
- [x] REST API endpoints available (`/api/resources`, `/api/resources/stats`, `/api/resources/{id}`)
- [x] Deduplication helper methods available for alert manager integration

### Phase 2 Complete When:
- [x] AI context includes unified resource view
- [x] AI can answer cross-platform questions (via GetTopByCPU/Memory/Disk)
- [x] Correlation across platforms works (via GetRelated)

### Phase 3 Complete When:
- [x] Agent data preferred when available (via store deduplication)
- [x] Polling optimization methods available (`ShouldSkipAPIPolling`, `GetPollingRecommendations`)
- [x] Resource store wired into Monitor (via `SetResourceStore`)
- [x] `shouldSkipNodeMetrics()` helper available for polling loops
- [x] AI can execute commands via agents (already implemented)
- [ ] Polling optimization actually used in live polling loops (optional enhancement)

### Phase 4 Complete When:
- [ ] Unified view accessible from UI
- [ ] Filtering and grouping works
- [ ] Existing pages still work

---

## Open Questions

1. **Should Resource replace existing types entirely, or wrap them?**
   - Recommendation: Wrap initially, replace gradually

2. **How to handle platform-specific features in unified view?**
   - Recommendation: Show common fields, expand for platform-specific

3. **Should we version the Resource schema?**
   - Recommendation: Yes, include version field for future evolution

4. **How to handle offline/stale data in unified model?**
   - Recommendation: Include `lastSeen` and `staleness` fields

---

## Appendix: Resource Type Mapping

| Current Type | Resource Type | Platform Type | Notes |
|--------------|---------------|---------------|-------|
| Node | node | proxmox-pve | Proxmox VE node |
| VM | vm | proxmox-pve | Proxmox VM |
| Container | container | proxmox-pve | LXC container |
| Host | host | host-agent | Standalone host |
| DockerHost | docker-host | docker | Docker/Podman host |
| DockerContainer | docker-container | docker | Docker container |
| PBSInstance | pbs | proxmox-pbs | Backup server |
| PBSDatastore | datastore | proxmox-pbs | PBS datastore |
| PMGInstance | pmg | proxmox-pmg | Mail gateway |
| Storage | storage | proxmox-pve | PVE storage |
| CephCluster | ceph-cluster | proxmox-pve | Ceph cluster |

---

## References

- [Conversation that sparked this design](internal discussion 2025-12-07)
- [Host Agent Deduplication Fix](commit implementing hostname-based dedup)
- [Pulse AI Features](current AI implementation)
