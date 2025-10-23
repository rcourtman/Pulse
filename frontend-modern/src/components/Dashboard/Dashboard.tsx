import { createSignal, createMemo, createEffect, For, Show, onMount } from 'solid-js';
import { createStore, reconcile } from 'solid-js/store';
import { useNavigate } from '@solidjs/router';
import type { VM, Container, Node } from '@/types/api';
import { GuestRow } from './GuestRow';
import { useWebSocket } from '@/App';
import { getAlertStyles } from '@/utils/alerts';
import { useAlertsActivation } from '@/stores/alertsActivation';
import { ComponentErrorBoundary } from '@/components/ErrorBoundary';
import { ScrollableTable } from '@/components/shared/ScrollableTable';
import { parseFilterStack, evaluateFilterStack } from '@/utils/searchQuery';
import { UnifiedNodeSelector } from '@/components/shared/UnifiedNodeSelector';
import { DashboardFilter } from './DashboardFilter';
import { GuestMetadataAPI } from '@/api/guestMetadata';
import type { GuestMetadata } from '@/api/guestMetadata';
import { Card } from '@/components/shared/Card';
import { EmptyState } from '@/components/shared/EmptyState';
import { NodeGroupHeader } from '@/components/shared/NodeGroupHeader';
import { ProxmoxSectionNav } from '@/components/Proxmox/ProxmoxSectionNav';
import { isNodeOnline } from '@/utils/status';
import { getNodeDisplayName } from '@/utils/nodes';
import { usePersistentSignal } from '@/hooks/usePersistentSignal';

interface DashboardProps {
  vms: VM[];
  containers: Container[];
  nodes: Node[];
}

type ViewMode = 'all' | 'vm' | 'lxc';
type StatusMode = 'all' | 'running' | 'stopped';
type GroupingMode = 'grouped' | 'flat';

export function Dashboard(props: DashboardProps) {
  const navigate = useNavigate();
  const ws = useWebSocket();
  const { connected, activeAlerts, initialDataReceived, reconnecting, reconnect } = ws;
  const alertsActivation = useAlertsActivation();
  const alertsEnabled = createMemo(() => alertsActivation.activationState() === 'active');
  const [search, setSearch] = createSignal('');
  const [isSearchLocked, setIsSearchLocked] = createSignal(false);
  const [selectedNode, setSelectedNode] = createSignal<string | null>(null);
  const [guestMetadata, setGuestMetadata] = createSignal<Record<string, GuestMetadata>>({});

  // Stable guest store using reconcile to prevent row remounting during websocket updates
  const guestKey = (g: VM | Container) =>
    g.id ?? (g.instance === g.node ? `${g.node}-${g.vmid}` : `${g.instance}-${g.node}-${g.vmid}`);
  const [guestStore, setGuestStore] = createStore<(VM | Container)[]>([]);

  // Reconcile guests whenever props change
  createEffect(() => {
    setGuestStore(reconcile([...props.vms, ...props.containers], { key: guestKey, merge: true }));
  });

  // Initialize from localStorage with proper type checking
  const [viewMode, setViewMode] = usePersistentSignal<ViewMode>('dashboardViewMode', 'all', {
    deserialize: (raw) => (raw === 'all' || raw === 'vm' || raw === 'lxc' ? raw : 'all'),
  });

  const [statusMode, setStatusMode] = usePersistentSignal<StatusMode>('dashboardStatusMode', 'all', {
    deserialize: (raw) =>
      raw === 'all' || raw === 'running' || raw === 'stopped' ? raw : ('all' as StatusMode),
  });

  // Grouping mode - grouped by node or flat list
  const [groupingMode, setGroupingMode] = usePersistentSignal<GroupingMode>(
    'dashboardGroupingMode',
    'grouped',
    {
      deserialize: (raw) => (raw === 'grouped' || raw === 'flat' ? raw : 'grouped'),
    },
  );

  const [showFilters, setShowFilters] = usePersistentSignal<boolean>(
    'dashboardShowFilters',
    false,
    {
      deserialize: (raw) => raw === 'true',
      serialize: (value) => String(value),
    },
  );

  // Sorting state - default to VMID ascending (matches Proxmox order)
  const [sortKey, setSortKey] = createSignal<keyof (VM | Container) | null>('vmid');
  const [sortDirection, setSortDirection] = createSignal<'asc' | 'desc'>('asc');


  // Load all guest metadata on mount (single API call for all guests)
  onMount(async () => {
    try {
      const metadata = await GuestMetadataAPI.getAllMetadata();
      setGuestMetadata(metadata || {});
    } catch (err) {
      // Silently fail - metadata is optional for display
      console.debug('Failed to load guest metadata:', err);
    }
  });

  // Callback to update a guest's custom URL in metadata
  const handleCustomUrlUpdate = (guestId: string, url: string) => {
    setGuestMetadata((prev) => ({
      ...prev,
      [guestId]: {
        ...(prev[guestId] || { id: guestId }),
        customUrl: url || undefined,
      },
    }));
  };

  // Create a mapping from node ID to node object
  const nodeByInstance = createMemo(() => {
    const map: Record<string, Node> = {};
    props.nodes.forEach((node) => {
      map[node.id] = node;
    });
    return map;
  });

  const resolveParentNode = (guest: VM | Container): Node | undefined => {
    if (!guest) return undefined;
    const nodes = nodeByInstance();

    if (guest.id) {
      const lastDash = guest.id.lastIndexOf('-');
      if (lastDash > 0) {
        const nodeId = guest.id.slice(0, lastDash);
        if (nodes[nodeId]) {
          return nodes[nodeId];
        }
      }
    }

    const compositeKey = `${guest.instance}-${guest.node}`;
    if (nodes[compositeKey]) {
      return nodes[compositeKey];
    }

    return undefined;
  };
  // Sort handler
  const handleSort = (key: keyof (VM | Container)) => {
    if (sortKey() === key) {
      // Toggle direction for the same column
      setSortDirection(sortDirection() === 'asc' ? 'desc' : 'asc');
    } else {
      // New column - set key and default direction
      setSortKey(key);
      // Set default sort direction based on column type
      if (
        key === 'cpu' ||
        key === 'memory' ||
        key === 'disk' ||
        key === 'diskRead' ||
        key === 'diskWrite' ||
        key === 'networkIn' ||
        key === 'networkOut' ||
        key === 'uptime'
      ) {
        setSortDirection('desc');
      } else {
        setSortDirection('asc');
      }
    }
  };

  const getDiskUsagePercent = (guest: VM | Container): number | null => {
    const disk = guest?.disk;
    if (!disk) return null;

    const clamp = (value: number) => Math.min(100, Math.max(0, value));

    if (typeof disk.usage === 'number' && Number.isFinite(disk.usage)) {
      // Some sources report usage as a ratio (0-1), others as a percentage (0-100)
      const usageValue = disk.usage > 1 ? disk.usage : disk.usage * 100;
      return clamp(usageValue);
    }

    if (
      typeof disk.used === 'number' &&
      Number.isFinite(disk.used) &&
      typeof disk.total === 'number' &&
      Number.isFinite(disk.total) &&
      disk.total > 0
    ) {
      return clamp((disk.used / disk.total) * 100);
    }

    return null;
  };

  // Handle keyboard shortcuts
  let searchInputRef: HTMLInputElement | undefined;

  createEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      // Ignore if user is typing in an input, textarea, or contenteditable
      const target = e.target as HTMLElement;
      const isInputField =
        target.tagName === 'INPUT' ||
        target.tagName === 'TEXTAREA' ||
        target.tagName === 'SELECT' ||
        target.contentEditable === 'true';

      // Escape key behavior
      if (e.key === 'Escape') {
        // First check if we have search/filters to clear (including tag filters and node selection)
        const hasActiveFilters =
          search().trim() ||
          sortKey() !== 'vmid' ||
          sortDirection() !== 'asc' ||
          selectedNode() !== null ||
          viewMode() !== 'all' ||
          statusMode() !== 'all';

        if (hasActiveFilters) {
          // Clear ALL filters including search text, tag filters, node selection, and view modes
          setSearch('');
          setIsSearchLocked(false);
          setSortKey('vmid');
          setSortDirection('asc');
          setSelectedNode(null);
          setViewMode('all');
          setStatusMode('all');

          // Blur the search input if it's focused
          if (searchInputRef && document.activeElement === searchInputRef) {
            searchInputRef.blur();
          }
        } else {
          // No active filters, toggle the filters section visibility
          setShowFilters(!showFilters());
        }
      } else if (!isInputField && e.key.length === 1 && !e.ctrlKey && !e.metaKey && !e.altKey) {
        // If it's a printable character and user is not in an input field
        // Expand filters section if collapsed
        if (!showFilters()) {
          setShowFilters(true);
        }
        // Focus the search input and let the character be typed
        if (searchInputRef) {
          searchInputRef.focus();
          // Don't prevent default - let the character be typed
        }
      }
    };

    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  });

  // Use the stable guest store
  const allGuests = createMemo(() => guestStore);

  // Filter guests based on current settings
  const filteredGuests = createMemo(() => {
    let guests = allGuests();

    // Filter by selected node using both instance and node name for uniqueness
    const selectedNodeId = selectedNode();
    if (selectedNodeId) {
      // Find the node to get both instance and name for precise matching
      const node = props.nodes.find((n) => n.id === selectedNodeId);
      if (node) {
        guests = guests.filter(
          (g) => g.instance === node.instance && g.node === node.name,
        );
      }
    }

    // Filter by type
    if (viewMode() === 'vm') {
      guests = guests.filter((g) => g.type === 'qemu');
    } else if (viewMode() === 'lxc') {
      guests = guests.filter((g) => g.type === 'lxc');
    }

    // Filter by status
    if (statusMode() === 'running') {
      guests = guests.filter((g) => g.status === 'running');
    } else if (statusMode() === 'stopped') {
      guests = guests.filter((g) => g.status !== 'running');
    }

    // Apply search/filter
    const searchTerm = search().trim();
    if (searchTerm) {
      // Split by commas first
      const searchParts = searchTerm
        .split(',')
        .map((t) => t.trim())
        .filter((t) => t);

      // Separate filters from text searches
      const filters: string[] = [];
      const textSearches: string[] = [];

      searchParts.forEach((part) => {
        if (part.includes('>') || part.includes('<') || part.includes(':')) {
          filters.push(part);
        } else {
          textSearches.push(part.toLowerCase());
        }
      });

      // Apply filters if any
      if (filters.length > 0) {
        // Join filters with AND operator
        const filterString = filters.join(' AND ');
        const stack = parseFilterStack(filterString);
        if (stack.filters.length > 0) {
          guests = guests.filter((g) => evaluateFilterStack(g, stack));
        }
      }

      // Apply text search if any
      if (textSearches.length > 0) {
        guests = guests.filter((g) =>
          textSearches.some(
            (term) =>
              g.name.toLowerCase().includes(term) ||
              g.vmid.toString().includes(term) ||
              g.node.toLowerCase().includes(term) ||
              g.status.toLowerCase().includes(term),
          ),
        );
      }
    }

    // Don't filter by thresholds anymore - dimming is handled in GuestRow component

    return guests;
  });

  // Group by node or return flat list based on grouping mode
  const groupedGuests = createMemo(() => {
    const guests = filteredGuests();

    // If flat mode, return all guests in a single group
    if (groupingMode() === 'flat') {
      const groups: Record<string, (VM | Container)[]> = { '': guests };
      // Sort the flat list
      const key = sortKey();
      const dir = sortDirection();
      if (key) {
        groups[''] = groups[''].sort((a, b) => {
          let aVal: string | number | boolean | null | undefined = a[key] as
            | string
            | number
            | boolean
            | null
            | undefined;
          let bVal: string | number | boolean | null | undefined = b[key] as
            | string
            | number
            | boolean
            | null
            | undefined;

          // Special handling for percentage-based columns
          if (key === 'cpu') {
            // CPU is displayed as percentage
            aVal = a.cpu * 100;
            bVal = b.cpu * 100;
          } else if (key === 'memory') {
            // Memory is displayed as percentage (use pre-calculated usage)
            aVal = a.memory ? a.memory.usage || 0 : 0;
            bVal = b.memory ? b.memory.usage || 0 : 0;
          } else if (key === 'disk') {
            aVal = getDiskUsagePercent(a);
            bVal = getDiskUsagePercent(b);
          }

          // Handle null/undefined/empty values - put at end for both asc and desc
          const aIsEmpty = aVal === null || aVal === undefined || aVal === '';
          const bIsEmpty = bVal === null || bVal === undefined || bVal === '';

          if (aIsEmpty && bIsEmpty) return 0;
          if (aIsEmpty) return 1;
          if (bIsEmpty) return -1;

          // Type-specific value preparation
          if (typeof aVal === 'number' && typeof bVal === 'number') {
            // Numeric comparison
            const comparison = aVal < bVal ? -1 : 1;
            return dir === 'asc' ? comparison : -comparison;
          } else {
            // String comparison (case-insensitive)
            const aStr = String(aVal).toLowerCase();
            const bStr = String(bVal).toLowerCase();

            if (aStr === bStr) return 0;
            const comparison = aStr < bStr ? -1 : 1;
            return dir === 'asc' ? comparison : -comparison;
          }
        });
      }
      return groups;
    }

    // Group by node ID (instance + node name) to match Node.ID format
    const groups: Record<string, (VM | Container)[]> = {};
    guests.forEach((guest) => {
      // Node.ID is formatted as "instance-nodename", so we need to match that
      const nodeId = `${guest.instance}-${guest.node}`;
      if (!groups[nodeId]) {
        groups[nodeId] = [];
      }
      groups[nodeId].push(guest);
    });

    // Sort within each node group
    const key = sortKey();
    const dir = sortDirection();
    if (key) {
      Object.keys(groups).forEach((node) => {
        groups[node] = groups[node].sort((a, b) => {
          let aVal: string | number | boolean | null | undefined = a[key] as
            | string
            | number
            | boolean
            | null
            | undefined;
          let bVal: string | number | boolean | null | undefined = b[key] as
            | string
            | number
            | boolean
            | null
            | undefined;

          // Special handling for percentage-based columns
          if (key === 'cpu') {
            // CPU is displayed as percentage
            aVal = a.cpu * 100;
            bVal = b.cpu * 100;
          } else if (key === 'memory') {
            // Memory is displayed as percentage (use pre-calculated usage)
            aVal = a.memory ? a.memory.usage || 0 : 0;
            bVal = b.memory ? b.memory.usage || 0 : 0;
          } else if (key === 'disk') {
            aVal = getDiskUsagePercent(a);
            bVal = getDiskUsagePercent(b);
          }

          // Handle null/undefined/empty values - put at end for both asc and desc
          const aIsEmpty = aVal === null || aVal === undefined || aVal === '';
          const bIsEmpty = bVal === null || bVal === undefined || bVal === '';

          if (aIsEmpty && bIsEmpty) return 0;
          if (aIsEmpty) return 1;
          if (bIsEmpty) return -1;

          // Type-specific value preparation
          if (typeof aVal === 'number' && typeof bVal === 'number') {
            // Numeric comparison
            const comparison = aVal < bVal ? -1 : 1;
            return dir === 'asc' ? comparison : -comparison;
          } else {
            // String comparison (case-insensitive)
            const aStr = String(aVal).toLowerCase();
            const bStr = String(bVal).toLowerCase();

            if (aStr === bStr) return 0;
            const comparison = aStr < bStr ? -1 : 1;
            return dir === 'asc' ? comparison : -comparison;
          }
        });
      });
    }

    return groups;
  });

  const totalStats = createMemo(() => {
    const guests = filteredGuests();
    const running = guests.filter((g) => g.status === 'running').length;
    const vms = guests.filter((g) => g.type === 'qemu').length;
    const containers = guests.filter((g) => g.type === 'lxc').length;
    return {
      total: guests.length,
      running,
      stopped: guests.length - running,
      vms,
      containers,
    };
  });

  const handleNodeSelect = (nodeId: string | null, nodeType: 'pve' | 'pbs' | 'pmg' | null) => {
    console.log('handleNodeSelect called:', nodeId, nodeType);
    // Track selected node for filtering (independent of search)
    if (nodeType === 'pve' || nodeType === null) {
      setSelectedNode(nodeId);
      console.log('Set selected node to:', nodeId);
      // Show filters if a node is selected
      if (nodeId && !showFilters()) {
        setShowFilters(true);
      }
    }
  };

  const handleTagClick = (tag: string) => {
    const currentSearch = search().trim();
    const tagFilter = `tags:${tag}`;

    // Check if this tag filter already exists
    if (currentSearch.includes(tagFilter)) {
      // Remove the tag filter
      let newSearch = currentSearch;

      // Handle different cases of where the tag filter might be
      if (currentSearch === tagFilter) {
        // It's the only filter
        newSearch = '';
      } else if (currentSearch.startsWith(tagFilter + ',')) {
        // It's at the beginning
        newSearch = currentSearch.replace(tagFilter + ',', '').trim();
      } else if (currentSearch.endsWith(', ' + tagFilter)) {
        // It's at the end
        newSearch = currentSearch.replace(', ' + tagFilter, '').trim();
      } else if (currentSearch.includes(', ' + tagFilter + ',')) {
        // It's in the middle
        newSearch = currentSearch.replace(', ' + tagFilter + ',', ',').trim();
      } else if (currentSearch.includes(tagFilter + ', ')) {
        // It's at the beginning with space after comma
        newSearch = currentSearch.replace(tagFilter + ', ', '').trim();
      }

      setSearch(newSearch);
      if (!newSearch) {
        setIsSearchLocked(false);
      }
    } else {
      // Add the tag filter
      if (!currentSearch || isSearchLocked()) {
        setSearch(tagFilter);
        setIsSearchLocked(false);
      } else {
        // Add tag filter to existing search with comma separator
        setSearch(`${currentSearch}, ${tagFilter}`);
      }

      // Make sure filters are visible
      if (!showFilters()) {
        setShowFilters(true);
      }
    }
  };

  return (
    <div class="space-y-3">
      <ProxmoxSectionNav current="overview" />

      {/* Unified Node Selector */}
      <UnifiedNodeSelector
        currentTab="dashboard"
        onNodeSelect={handleNodeSelect}
        nodes={props.nodes}
        filteredVms={filteredGuests().filter((g) => g.type === 'qemu')}
        filteredContainers={filteredGuests().filter((g) => g.type === 'lxc')}
        searchTerm={search()}
      />

      {/* Dashboard Filter */}
      <DashboardFilter
        search={search}
        setSearch={setSearch}
        isSearchLocked={isSearchLocked}
        viewMode={viewMode}
        setViewMode={setViewMode}
        statusMode={statusMode}
        setStatusMode={setStatusMode}
        groupingMode={groupingMode}
        setGroupingMode={setGroupingMode}
        setSortKey={setSortKey}
        setSortDirection={setSortDirection}
        searchInputRef={(el) => (searchInputRef = el)}
      />

      {/* Loading State */}
      <Show when={connected() && !initialDataReceived()}>
        <Card padding="lg">
          <EmptyState
            icon={
              <svg
                class="mx-auto h-12 w-12 animate-spin text-gray-400"
                fill="none"
                viewBox="0 0 24 24"
              >
                <circle
                  class="opacity-25"
                  cx="12"
                  cy="12"
                  r="10"
                  stroke="currentColor"
                  stroke-width="4"
                />
                <path
                  class="opacity-75"
                  fill="currentColor"
                  d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                />
              </svg>
            }
            title="Loading dashboard data..."
            description={
              reconnecting()
                ? 'Reconnecting to monitoring service…'
                : 'Connecting to monitoring service'
            }
          />
        </Card>
      </Show>

      {/* Empty State - No PVE Nodes Configured */}
      <Show
        when={
          connected() &&
          initialDataReceived() &&
          props.nodes.filter((n) => n.type === 'pve').length === 0 &&
          props.vms.length === 0 &&
          props.containers.length === 0
        }
      >
        <Card padding="lg">
          <EmptyState
            icon={
              <svg
                class="h-12 w-12 text-gray-400"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"
                />
              </svg>
            }
            title="No Proxmox VE nodes configured"
            description="Add a Proxmox VE node in the Settings tab to start monitoring your infrastructure."
            actions={
              <button
                type="button"
                onClick={() => navigate('/settings')}
                class="inline-flex items-center px-3 py-1.5 border border-transparent text-xs font-medium rounded-md text-white bg-blue-600 hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500"
              >
                Go to Settings
              </button>
            }
          />
        </Card>
      </Show>

      {/* Disconnected State */}
      <Show when={!connected()}>
        <Card padding="lg" tone="danger">
          <EmptyState
            icon={
              <svg
                class="h-12 w-12 text-red-400"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
                />
              </svg>
            }
            title="Connection lost"
            description={
              reconnecting()
                ? 'Attempting to reconnect…'
                : 'Unable to connect to the backend server'
            }
            tone="danger"
            actions={
              !reconnecting() ? (
                <button
                  onClick={() => reconnect()}
                  class="mt-2 inline-flex items-center px-4 py-2 text-xs font-medium rounded bg-red-600 text-white hover:bg-red-700 transition-colors"
                >
                  Reconnect now
                </button>
              ) : undefined
            }
          />
        </Card>
      </Show>

      {/* Table View */}
      <Show when={connected() && initialDataReceived() && filteredGuests().length > 0}>
        <ComponentErrorBoundary name="Guest Table">
          <Card padding="none" class="mb-4 overflow-hidden">
            <ScrollableTable minWidth="760px">
              <table class="w-full min-w-[760px] md:min-w-[900px] table-fixed border-collapse">
                <thead>
                  <tr class="bg-gray-50 dark:bg-gray-700/50 text-gray-600 dark:text-gray-300 border-b border-gray-200 dark:border-gray-600">
                    <th
                      class="pl-4 pr-2 py-1 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider w-[130px] sm:w-[150px] lg:w-[180px] xl:w-[200px] 2xl:w-[240px] cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600 focus:outline-none focus:ring-2 focus:ring-inset focus:ring-blue-500"
                      onClick={() => handleSort('name')}
                      onKeyDown={(e) => e.key === 'Enter' && handleSort('name')}
                      tabindex="0"
                      role="button"
                      aria-label={`Sort by name ${sortKey() === 'name' ? (sortDirection() === 'asc' ? 'ascending' : 'descending') : ''}`}
                    >
                      Name {sortKey() === 'name' && (sortDirection() === 'asc' ? '▲' : '▼')}
                    </th>
                    <th
                      class="px-2 py-1 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider w-[48px] sm:w-[56px] lg:w-[60px] xl:w-[64px] 2xl:w-[72px] cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600"
                      onClick={() => handleSort('type')}
                    >
                      Type {sortKey() === 'type' && (sortDirection() === 'asc' ? '▲' : '▼')}
                    </th>
                    <th
                      class="px-2 py-1 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider w-[56px] sm:w-[64px] lg:w-[72px] xl:w-[80px] 2xl:w-[90px] cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600"
                      onClick={() => handleSort('vmid')}
                    >
                      VMID {sortKey() === 'vmid' && (sortDirection() === 'asc' ? '▲' : '▼')}
                    </th>
                    <th
                      class="px-2 py-1 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider w-[76px] sm:w-[86px] lg:w-[96px] xl:w-[108px] 2xl:w-[128px] cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600"
                      onClick={() => handleSort('uptime')}
                    >
                      Uptime {sortKey() === 'uptime' && (sortDirection() === 'asc' ? '▲' : '▼')}
                    </th>
                    <th
                      class="px-2 py-1 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider w-[100px] sm:w-[110px] lg:w-[130px] xl:w-[150px] 2xl:w-[180px] cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600"
                      onClick={() => handleSort('cpu')}
                    >
                      CPU {sortKey() === 'cpu' && (sortDirection() === 'asc' ? '▲' : '▼')}
                    </th>
                    <th
                      class="px-2 py-1 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider w-[100px] sm:w-[110px] lg:w-[130px] xl:w-[150px] 2xl:w-[180px] cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600"
                      onClick={() => handleSort('memory')}
                    >
                      Memory {sortKey() === 'memory' && (sortDirection() === 'asc' ? '▲' : '▼')}
                    </th>
                    <th
                      class="px-2 py-1 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider w-[100px] sm:w-[110px] lg:w-[130px] xl:w-[150px] 2xl:w-[180px] cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600"
                      onClick={() => handleSort('disk')}
                    >
                      Disk {sortKey() === 'disk' && (sortDirection() === 'asc' ? '▲' : '▼')}
                    </th>
                    <th
                      class="px-2 py-1 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider w-[56px] sm:w-[62px] lg:w-[70px] xl:w-[78px] 2xl:w-[96px] cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600"
                      onClick={() => handleSort('diskRead')}
                    >
                      Disk Read{' '}
                      {sortKey() === 'diskRead' && (sortDirection() === 'asc' ? '▲' : '▼')}
                    </th>
                    <th
                      class="px-2 py-1 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider w-[56px] sm:w-[62px] lg:w-[70px] xl:w-[78px] 2xl:w-[96px] cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600"
                      onClick={() => handleSort('diskWrite')}
                    >
                      Disk Write{' '}
                      {sortKey() === 'diskWrite' && (sortDirection() === 'asc' ? '▲' : '▼')}
                    </th>
                    <th
                      class="px-2 py-1 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider w-[56px] sm:w-[62px] lg:w-[70px] xl:w-[78px] 2xl:w-[96px] cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600"
                      onClick={() => handleSort('networkIn')}
                    >
                      Net In {sortKey() === 'networkIn' && (sortDirection() === 'asc' ? '▲' : '▼')}
                    </th>
                    <th
                      class="px-2 py-1 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider w-[56px] sm:w-[62px] lg:w-[70px] xl:w-[78px] 2xl:w-[96px] cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600"
                      onClick={() => handleSort('networkOut')}
                    >
                      Net Out{' '}
                      {sortKey() === 'networkOut' && (sortDirection() === 'asc' ? '▲' : '▼')}
                    </th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
                  <For
                    each={Object.entries(groupedGuests()).sort(([instanceIdA], [instanceIdB]) => {
                      // Sort by friendly node name first, falling back to instance ID for stability
                      const nodeA = nodeByInstance()[instanceIdA];
                      const nodeB = nodeByInstance()[instanceIdB];
                      const labelA = nodeA ? getNodeDisplayName(nodeA) : instanceIdA;
                      const labelB = nodeB ? getNodeDisplayName(nodeB) : instanceIdB;
                      const nameCompare = labelA.localeCompare(labelB);
                      if (nameCompare !== 0) return nameCompare;
                      // If labels match (unlikely), fall back to the instance IDs
                      return instanceIdA.localeCompare(instanceIdB);
                    })}
                    fallback={<></>}
                  >
                    {([instanceId, guests]) => {
                      const node = nodeByInstance()[instanceId];

                      return (
                      <>
                        <Show when={node && groupingMode() === 'grouped'}>
                          <NodeGroupHeader node={node!} colspan={11} />
                        </Show>
                        <For each={guests} fallback={<></>}>
                          {(guest) => {
                            // Match backend ID generation logic: standalone nodes use "node-vmid", clusters use "instance-node-vmid"
                            const guestId =
                              guest.id ||
                              (guest.instance === guest.node
                                ? `${guest.node}-${guest.vmid}`
                                : `${guest.instance}-${guest.node}-${guest.vmid}`);
                            const metadata =
                              guestMetadata()[guestId] ||
                              guestMetadata()[`${guest.node}-${guest.vmid}`];
                            const parentNode = node ?? resolveParentNode(guest);
                            const parentNodeOnline = parentNode ? isNodeOnline(parentNode) : true;
                            return (
                              <ComponentErrorBoundary name="GuestRow">
                                <GuestRow
                                  guest={guest}
                                  alertStyles={getAlertStyles(guestId, activeAlerts, alertsEnabled())}
                                  customUrl={metadata?.customUrl}
                                  onTagClick={handleTagClick}
                                  activeSearch={search()}
                                  parentNodeOnline={parentNodeOnline}
                                  onCustomUrlUpdate={handleCustomUrlUpdate}
                                />
                              </ComponentErrorBoundary>
                            );
                          }}
                        </For>
                      </>
                    );
                    }}
                  </For>
                </tbody>
              </table>
            </ScrollableTable>
          </Card>
        </ComponentErrorBoundary>
      </Show>

      <Show
        when={
          connected() &&
          initialDataReceived() &&
          filteredGuests().length === 0 &&
          (props.vms.length > 0 || props.containers.length > 0)
        }
      >
        <Card padding="lg" class="mb-4">
          <EmptyState
            icon={
              <svg
                class="h-12 w-12 text-gray-400"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
                />
              </svg>
            }
            title="No guests found"
            description={
              search() && search().trim() !== ''
                ? `No guests match your search "${search()}"`
                : 'No guests match your current filters'
            }
          />
        </Card>
      </Show>

      {/* Stats */}
      <Show when={connected() && initialDataReceived()}>
        <div class="mb-4">
          <div class="flex items-center gap-2 p-2 bg-gray-50 dark:bg-gray-700/50 border border-gray-200 dark:border-gray-700 rounded">
            <span class="flex items-center gap-1 text-xs text-gray-600 dark:text-gray-400">
              <span class="h-2 w-2 bg-green-500 rounded-full"></span>
              {totalStats().running} running
            </span>
            <span class="text-gray-400">|</span>
            <span class="flex items-center gap-1 text-xs text-gray-600 dark:text-gray-400">
              <span class="h-2 w-2 bg-red-500 rounded-full"></span>
              {totalStats().stopped} stopped
            </span>
          </div>
        </div>
      </Show>

    </div>
  );
}
