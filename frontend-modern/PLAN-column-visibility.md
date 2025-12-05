# Plan: Toggleable Table Columns

## Problem
The current drawer pattern hides useful information (IPs, OS, backup status, node, tags) that users need to click to reveal. This goes against Pulse's core philosophy of dense, scannable, comparable data at a glance.

## Goal
Replace drawer-hidden info with optional table columns that:
1. Show data inline for easy comparison across rows
2. Are toggleable by the user (show/hide)
3. Auto-show based on available horizontal space
4. Persist user preferences

## Current State

### Infrastructure Already Exists
- `ColumnPriority` system: `'essential' | 'primary' | 'secondary' | 'supplementary' | 'detailed'`
- `PRIORITY_BREAKPOINTS` maps priorities to responsive breakpoints
- `usePersistentSignal` for localStorage persistence
- `STANDARD_COLUMNS` with predefined column configs
- `useBreakpoint` hook for responsive behavior

### Current Columns (Dashboard/Proxmox)
All marked `essential` (always visible):
- Name, Type, VMID, Uptime
- CPU, Memory, Disk (progress bars)
- Disk Read, Disk Write, Net In, Net Out

### Data Available but Hidden in Drawer
- IP addresses (`guest.ipAddresses`)
- OS name/version (`guest.osName`, `guest.osVersion`)
- Node (`guest.node`)
- Backup status (`guest.lastBackup`)
- Tags (`guest.tags`)
- CPUs allocated (`guest.cpus`)
- Agent version (`guest.agentVersion`)

## Implementation

### Phase 1: Add New Column Definitions

Update `GUEST_COLUMNS` in `GuestRow.tsx`:

```typescript
export const GUEST_COLUMNS: ColumnDef[] = [
  // Essential - always visible
  { id: 'name', label: 'Name', priority: 'essential' },
  { id: 'type', label: 'Type', priority: 'essential' },
  { id: 'vmid', label: 'VMID', priority: 'essential' },

  // Primary - visible on sm+ (640px)
  { id: 'cpu', label: 'CPU', priority: 'essential', minWidth: '55px', maxWidth: '156px' },
  { id: 'memory', label: 'Memory', priority: 'essential', minWidth: '75px', maxWidth: '156px' },
  { id: 'disk', label: 'Disk', priority: 'essential', minWidth: '75px', maxWidth: '156px' },

  // Secondary - visible on md+ (768px), user toggleable
  { id: 'ip', label: 'IP', priority: 'secondary', toggleable: true },
  { id: 'uptime', label: 'Uptime', priority: 'secondary', toggleable: true },
  { id: 'node', label: 'Node', priority: 'secondary', toggleable: true },

  // Supplementary - visible on lg+ (1024px), user toggleable
  { id: 'backup', label: 'Backup', priority: 'supplementary', toggleable: true },
  { id: 'os', label: 'OS', priority: 'supplementary', toggleable: true },
  { id: 'tags', label: 'Tags', priority: 'supplementary', toggleable: true },

  // Detailed - visible on xl+ (1280px), user toggleable
  { id: 'diskRead', label: 'D Read', priority: 'detailed', toggleable: true },
  { id: 'diskWrite', label: 'D Write', priority: 'detailed', toggleable: true },
  { id: 'netIn', label: 'Net In', priority: 'detailed', toggleable: true },
  { id: 'netOut', label: 'Net Out', priority: 'detailed', toggleable: true },
];
```

### Phase 2: Column Visibility State

Create a hook for managing column visibility:

```typescript
// hooks/useColumnVisibility.ts
export function useColumnVisibility(
  storageKey: string,
  columns: ColumnDef[]
) {
  // Get toggleable columns
  const toggleableIds = columns.filter(c => c.toggleable).map(c => c.id);

  // Default: all toggleable columns visible
  const defaultVisible = new Set(toggleableIds);

  // Persist to localStorage
  const [hiddenColumns, setHiddenColumns] = usePersistentSignal<Set<string>>(
    storageKey,
    new Set(),
    {
      serialize: (set) => JSON.stringify([...set]),
      deserialize: (str) => new Set(JSON.parse(str)),
    }
  );

  const isVisible = (id: string) => !hiddenColumns().has(id);
  const toggle = (id: string) => {
    const hidden = new Set(hiddenColumns());
    if (hidden.has(id)) {
      hidden.delete(id);
    } else {
      hidden.add(id);
    }
    setHiddenColumns(hidden);
  };

  return { isVisible, toggle, hiddenColumns, toggleableIds };
}
```

### Phase 3: Column Picker UI

Add a column picker dropdown to the filter bar:

```typescript
// components/shared/ColumnPicker.tsx
<div class="relative">
  <button
    onClick={() => setOpen(!open())}
    class="inline-flex items-center gap-1.5 px-2.5 py-1.5 text-xs font-medium rounded-lg
           bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300
           hover:bg-gray-200 dark:hover:bg-gray-600"
  >
    <ColumnsIcon class="w-3.5 h-3.5" />
    Columns
  </button>

  <Show when={open()}>
    <div class="absolute right-0 mt-1 w-48 rounded-lg border bg-white dark:bg-gray-800 shadow-lg z-50">
      <For each={toggleableColumns}>
        {(col) => (
          <label class="flex items-center gap-2 px-3 py-2 hover:bg-gray-50 dark:hover:bg-gray-700 cursor-pointer">
            <input
              type="checkbox"
              checked={isVisible(col.id)}
              onChange={() => toggle(col.id)}
            />
            <span class="text-sm">{col.label}</span>
          </label>
        )}
      </For>
    </div>
  </Show>
</div>
```

### Phase 4: Render New Column Cells

Add cell renderers in `GuestRow.tsx`:

```typescript
// IP column
<Show when={visibleColumns().includes('ip')}>
  <td class="...">
    <Show when={guest.ipAddresses?.length}>
      <span class="text-xs font-mono truncate" title={guest.ipAddresses?.join(', ')}>
        {guest.ipAddresses?.[0]}
        {guest.ipAddresses?.length > 1 && ` +${guest.ipAddresses.length - 1}`}
      </span>
    </Show>
  </td>
</Show>

// Backup column
<Show when={visibleColumns().includes('backup')}>
  <td class="...">
    <BackupStatusBadge lastBackup={guest.lastBackup} />
  </td>
</Show>

// Node column
<Show when={visibleColumns().includes('node')}>
  <td class="...">
    <span class="text-xs truncate">{guest.node}</span>
  </td>
</Show>

// OS column
<Show when={visibleColumns().includes('os')}>
  <td class="...">
    <span class="text-xs truncate" title={`${guest.osName} ${guest.osVersion}`}>
      {guest.osName || '—'}
    </span>
  </td>
</Show>

// Tags column
<Show when={visibleColumns().includes('tags')}>
  <td class="...">
    <TagBadges tags={guest.tags} compact />
  </td>
</Show>
```

### Phase 5: Update Table Header

Dynamically render headers based on visible columns:

```typescript
<thead>
  <tr>
    <For each={visibleColumns()}>
      {(col) => (
        <th
          class="..."
          onClick={() => col.sortable && handleSort(col.id)}
        >
          {col.label}
          <Show when={sortKey() === col.id}>
            <SortIndicator direction={sortDirection()} />
          </Show>
        </th>
      )}
    </For>
  </tr>
</thead>
```

### Phase 6: Responsive Behavior

Combine user preferences with breakpoint-based visibility:

```typescript
const visibleColumns = createMemo(() => {
  const breakpoint = useBreakpoint();

  return GUEST_COLUMNS.filter(col => {
    // Always show essential columns
    if (col.priority === 'essential') return true;

    // Check if breakpoint supports this priority
    const minBreakpoint = PRIORITY_BREAKPOINTS[col.priority];
    const hasSpace = breakpointIndex(breakpoint()) >= breakpointIndex(minBreakpoint);

    // If toggleable, also check user preference
    if (col.toggleable) {
      return hasSpace && isVisible(col.id);
    }

    return hasSpace;
  });
});
```

## Files to Modify

1. **`src/components/Dashboard/GuestRow.tsx`**
   - Expand `GUEST_COLUMNS` with new columns
   - Add cell renderers for IP, backup, node, OS, tags
   - Accept `visibleColumns` prop

2. **`src/components/Dashboard/Dashboard.tsx`**
   - Import and use column visibility hook
   - Pass visible columns to header and rows

3. **`src/components/Dashboard/DashboardFilter.tsx`**
   - Add ColumnPicker component

4. **`src/hooks/useColumnVisibility.ts`** (new)
   - Create the column visibility management hook

5. **`src/components/shared/ColumnPicker.tsx`** (new)
   - Create the column picker dropdown component

6. **`src/utils/localStorage.ts`**
   - Add `DASHBOARD_COLUMN_VISIBILITY` storage key

7. **Repeat for Hosts and Docker tabs**
   - Similar changes to `HostsOverview.tsx`
   - Similar changes to `DockerUnifiedTable.tsx`

## What Happens to Drawers?

After columns are implemented:
- **Keep drawers** but make them optional/minimal
- Drawer becomes a place for:
  - AI Context annotations (already there)
  - Very detailed info (full filesystem list, all network interfaces)
  - Actions (future: start/stop/migrate buttons)
- Or **remove drawers entirely** if columns cover everything needed

## Migration Path

1. Implement columns first (Phase 1-6)
2. Test with real data
3. Decide what remains valuable in drawers
4. Either slim down drawers or remove them

## Estimated Scope

- New hook: ~50 lines
- ColumnPicker component: ~80 lines
- GuestRow changes: ~150 lines (new cells)
- Dashboard changes: ~30 lines (wiring)
- Header changes: ~40 lines
- Repeat for Hosts: ~200 lines
- Repeat for Docker: ~200 lines

**Total: ~750 lines of new/modified code**
