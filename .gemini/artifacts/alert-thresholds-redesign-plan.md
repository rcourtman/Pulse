# Alert Thresholds Page Redesign

## Executive Summary

The current Alert Thresholds page suffers from information overload, poor scalability, and a monolithic codebase (~3000 lines in a single component). This plan outlines a comprehensive redesign focused on:

1. **Collapsible accordion-based layout** - Users can focus on what matters
2. **Component decomposition** - Maintainable, testable code
3. **Progressive disclosure** - Show summaries first, details on demand
4. **Responsive design** - Works on all screen sizes
5. **Improved visual hierarchy** - Clear information architecture

---

## Current Problems

### User Experience Issues
| Problem | Impact | Current State |
|---------|--------|---------------|
| Information overload | High | 6+ tables stacked vertically, no way to collapse |
| Wide tables | High | 7+ columns cause horizontal scroll |
| No visual hierarchy | Medium | Everything looks equal priority |
| Help banner always visible | Low | Takes space after users understand |
| No density controls | Medium | Can't see more resources at once |
| Unclear tab labels | Low | "Proxmox / PBS" bundles too much |

### Technical Debt
| Problem | Impact |
|---------|--------|
| `ThresholdsTable.tsx` is ~3000 lines | Very hard to maintain |
| Tightly coupled rendering and state | Difficult to test |
| Repeated code patterns | Inconsistent behavior |
| No clear component boundaries | Hard to extend |

---

## Proposed Architecture

### New Component Structure

```
src/components/Alerts/Thresholds/
├── index.ts                      # Public exports
├── ThresholdsPage.tsx            # Main page layout (~200 lines)
├── ThresholdsContext.tsx         # State management context
├── sections/
│   ├── CollapsibleSection.tsx    # Reusable accordion section
│   ├── ProxmoxNodesSection.tsx   # Nodes-specific logic
│   ├── GuestsSection.tsx         # VMs/CTs with node grouping
│   ├── StorageSection.tsx        # Storage devices
│   ├── PBSSection.tsx            # PBS servers
│   ├── BackupsSection.tsx        # Backup thresholds
│   └── SnapshotsSection.tsx      # Snapshot thresholds
├── components/
│   ├── ResourceCard.tsx          # Expandable resource card
│   ├── ThresholdBadge.tsx        # Colored threshold pill
│   ├── ThresholdEditor.tsx       # Inline/modal threshold editing
│   ├── GlobalDefaultsRow.tsx     # Editable defaults row
│   ├── SearchBar.tsx             # Enhanced search/filter
│   └── ViewToggle.tsx            # List/Compact toggle
├── hooks/
│   ├── useThresholds.ts          # Threshold state management
│   ├── useCollapsedSections.ts   # Persist collapsed state
│   └── useResourceFilter.ts      # Search/filter logic
└── types.ts                      # TypeScript interfaces
```

### Component Responsibilities

#### `ThresholdsPage.tsx` (~200 lines)
- Page layout and header
- Tab navigation (Proxmox/PBS, Mail Gateway, Hosts, Containers)
- Search bar and view toggle
- Renders appropriate section components based on active tab

#### `CollapsibleSection.tsx` (~150 lines)
- Reusable accordion wrapper
- Expand/collapse with animation
- Header with title, count, and actions
- Persists collapsed state to localStorage

#### `ResourceCard.tsx` (~200 lines)
- Compact collapsed view: Name, status, key thresholds as pills
- Expanded view: Full threshold editing grid
- Handles inline editing
- Shows "Custom" badge when overridden

#### `ThresholdBadge.tsx` (~50 lines)
- Colored pill showing threshold value
- Color indicates severity (green = conservative, red = aggressive, gray = disabled)
- Clickable to edit

---

## New Layout Design

### Page Structure

```
┌─────────────────────────────────────────────────────────────────┐
│  Alert Thresholds                                               │
│  Tune resource thresholds and override rules                    │
├─────────────────────────────────────────────────────────────────┤
│  [🔍 Search resources...]           [List ▼] [💡 Tips]          │
├─────────────────────────────────────────────────────────────────┤
│  [Proxmox/PBS] [Mail Gateway] [Host Agents] [Containers]        │
╞═════════════════════════════════════════════════════════════════╡
│                                                                 │
│  ▼ Proxmox Nodes                    2 resources  [Edit Defaults]│
│  ┌─────────────────────────────────────────────────────────────┐│
│  │ Global Defaults                                             ││
│  │ [CPU 80%] [Mem 85%] [Disk 90%] [Temp 80°C]                 ││
│  ├─────────────────────────────────────────────────────────────┤│
│  │ ✓ delly          Online    [CPU 80%] [Mem 85%]  [▼]        ││
│  │ ✓ minipc         Online    [CPU 80%] [Mem 85%]  [▼]        ││
│  └─────────────────────────────────────────────────────────────┘│
│                                                                 │
│  ▼ VMs & Containers                24 resources  [Edit Defaults]│
│  ┌─────────────────────────────────────────────────────────────┐│
│  │ Global Defaults                                             ││
│  │ [CPU 80%] [Mem 85%] [Disk 90%] [I/O: Off]                  ││
│  ├─────────────────────────────────────────────────────────────┤│
│  │ ▼ delly                                          12 guests  ││
│  │   ✓ homeassistant    Running  [Custom]  [CPU 70%] [▼]      ││
│  │   ✓ frigate          Running           [CPU 80%] [▼]       ││
│  │   ✓ mqtt             Running           [CPU 80%] [▼]       ││
│  │   ... 9 more                                                ││
│  ├─────────────────────────────────────────────────────────────┤│
│  │ ► minipc                                         12 guests  ││
│  └─────────────────────────────────────────────────────────────┘│
│                                                                 │
│  ► Storage                          4 resources  [Edit Defaults]│
│                                                                 │
│  ► PBS Servers                      0 resources  [Edit Defaults]│
│                                                                 │
│  ► Backups                                       [Edit Defaults]│
│                                                                 │
│  ► Snapshots                                     [Edit Defaults]│
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### Expanded Resource Card

When a resource is expanded:

```
┌─────────────────────────────────────────────────────────────────┐
│ homeassistant                [Custom]    [Alerts: ON]  [▲ Close]│
│ VM 100 • 192.168.1.100 • delly                                  │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  Performance Thresholds                                         │
│  ┌─────────────┬─────────────┬─────────────┬─────────────┐     │
│  │   CPU       │   Memory    │    Disk     │   Temp      │     │
│  │   [70 %]    │   [85 %]    │   [90 %]    │   [80°C]    │     │
│  └─────────────┴─────────────┴─────────────┴─────────────┘     │
│                                                                 │
│  I/O Thresholds                                                 │
│  ┌─────────────┬─────────────┬─────────────┬─────────────┐     │
│  │  Disk Read  │ Disk Write  │   Net In    │  Net Out    │     │
│  │   [Off]     │   [Off]     │   [Off]     │   [Off]     │     │
│  └─────────────┴─────────────┴─────────────┴─────────────┘     │
│                                                                 │
│  Offline Alerts: [Warning ▼]                                    │
│                                                                 │
│  Note: [Production HA instance                              ]   │
│                                                                 │
│                                      [Reset to Defaults] [Save] │
└─────────────────────────────────────────────────────────────────┘
```

---

## Key UX Improvements

### 1. Progressive Disclosure
- **Collapsed by default**: Sections show count and summary only
- **One-click expand**: Click anywhere on header to expand
- **Nested grouping**: VMs/Containers grouped by node, nodes collapsible
- **Remember state**: Collapsed/expanded state persisted in localStorage

### 2. Visual Hierarchy
- **Section headers**: Large, bold, with resource counts
- **Global defaults**: Always visible at top of each section
- **Custom indicators**: Blue "Custom" badge for overridden resources
- **Status colors**: Green checkmarks for healthy, warning/critical indicators

### 3. Threshold Badges
Color-coded pills that instantly communicate threshold severity:
- **Gray**: Disabled (Off)
- **Green**: Conservative (≥85%)
- **Yellow**: Moderate (70-84%)
- **Orange**: Aggressive (50-69%)
- **Red**: Very aggressive (<50%)

### 4. Search & Filter
Enhanced command bar supporting:
- Simple text search: `homeassistant`
- Property filters: `node:delly`, `type:vm`, `custom:true`
- Threshold filters: `cpu>80`, `memory<70`
- Combination: `node:delly custom:true`

### 5. Responsive Design
- **Wide screens**: Full grid layout with all columns
- **Medium screens**: Hide I/O thresholds, show on expand
- **Narrow screens**: Single column cards, full expand for editing

---

## Implementation Phases

### Phase 1: Component Decomposition (Foundation)
**Goal**: Break up `ThresholdsTable.tsx` without changing UI

1. Extract shared types to `types.ts`
2. Create `ThresholdsContext.tsx` for state management
3. Extract `ResourceCard.tsx` from table row rendering
4. Extract `ThresholdBadge.tsx` for threshold display
5. Create section components that wrap current logic
6. Update imports and ensure tests pass

**Deliverable**: Same UI, cleaner code, easier to modify

### Phase 2: Collapsible Sections
**Goal**: Add accordion behavior to sections

1. Create `CollapsibleSection.tsx` component
2. Add collapse/expand animation
3. Persist collapsed state to localStorage
4. Add resource counts to headers
5. Move "Edit Defaults" to section headers

**Deliverable**: Users can collapse sections they don't need

### Phase 3: Resource Cards
**Goal**: Replace table rows with expandable cards

1. Create compact card view (collapsed)
2. Create full editor view (expanded)
3. Add transition animation
4. Implement inline editing
5. Show threshold pills in collapsed view

**Deliverable**: Cleaner resource display, less horizontal scroll

### Phase 4: Enhanced Filtering
**Goal**: Powerful search and filter

1. Create `SearchBar.tsx` with command palette style
2. Implement filter parsers
3. Add quick filter buttons
4. Keyboard navigation support
5. Search highlighting in results

**Deliverable**: Users can quickly find specific resources

### Phase 5: Polish & Accessibility
**Goal**: Production-ready quality

1. Keyboard navigation throughout
2. Screen reader labels
3. Focus management
4. Loading states
5. Error handling
6. Empty states
7. Responsive testing

**Deliverable**: Accessible, polished experience

---

## Success Metrics

| Metric | Current | Target |
|--------|---------|--------|
| Lines in main component | ~3000 | <300 |
| Horizontal scroll needed | Often | Rarely |
| Clicks to find resource | 2-5 + scroll | 1-2 |
| Time to understand page | ~30s | <10s |
| Mobile usability | Poor | Good |

---

## Files to Create/Modify

### New Files
- `src/components/Alerts/Thresholds/index.ts`
- `src/components/Alerts/Thresholds/ThresholdsPage.tsx`
- `src/components/Alerts/Thresholds/ThresholdsContext.tsx`
- `src/components/Alerts/Thresholds/types.ts`
- `src/components/Alerts/Thresholds/sections/CollapsibleSection.tsx`
- `src/components/Alerts/Thresholds/sections/ProxmoxNodesSection.tsx`
- `src/components/Alerts/Thresholds/sections/GuestsSection.tsx`
- `src/components/Alerts/Thresholds/sections/StorageSection.tsx`
- `src/components/Alerts/Thresholds/components/ResourceCard.tsx`
- `src/components/Alerts/Thresholds/components/ThresholdBadge.tsx`
- `src/components/Alerts/Thresholds/components/ThresholdEditor.tsx`
- `src/components/Alerts/Thresholds/components/GlobalDefaultsRow.tsx`
- `src/components/Alerts/Thresholds/hooks/useThresholds.ts`
- `src/components/Alerts/Thresholds/hooks/useCollapsedSections.ts`

### Modified Files
- `src/components/Alerts/ThresholdsTable.tsx` → Eventually deprecated
- `src/pages/Alerts.tsx` → Use new ThresholdsPage
- `src/components/Alerts/ResourceTable.tsx` → Simplify or deprecate

---

## Risk Mitigation

1. **Incremental migration**: Keep old component working during transition
2. **Feature flags**: Can switch between old/new implementations
3. **Comprehensive tests**: Add tests for new components before replacing old
4. **User feedback**: Consider A/B testing or beta flag

---

## Next Steps

1. ✅ Create this implementation plan
2. ⬜ Generate visual mockup for approval
3. ⬜ Begin Phase 1: Component decomposition
4. ⬜ Add tests for extracted components
5. ⬜ Proceed through remaining phases
