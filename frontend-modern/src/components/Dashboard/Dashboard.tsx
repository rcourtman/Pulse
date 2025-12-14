import { createSignal, createMemo, createEffect, For, Show, onMount } from 'solid-js';
import { useNavigate } from '@solidjs/router';
import type { VM, Container, Node } from '@/types/api';
import { GuestRow, GUEST_COLUMNS, type GuestColumnDef } from './GuestRow';
import { useWebSocket } from '@/App';
import { getAlertStyles } from '@/utils/alerts';
import { useAlertsActivation } from '@/stores/alertsActivation';
import { ComponentErrorBoundary } from '@/components/ErrorBoundary';
import { parseFilterStack, evaluateFilterStack } from '@/utils/searchQuery';
import { UnifiedNodeSelector } from '@/components/shared/UnifiedNodeSelector';
import { DashboardFilter } from './DashboardFilter';
import { GuestMetadataAPI } from '@/api/guestMetadata';
import type { GuestMetadata } from '@/api/guestMetadata';
import { Card } from '@/components/shared/Card';
import { EmptyState } from '@/components/shared/EmptyState';
import { NodeGroupHeader } from '@/components/shared/NodeGroupHeader';
import { ProxmoxSectionNav } from '@/components/Proxmox/ProxmoxSectionNav';
import { isNodeOnline, OFFLINE_HEALTH_STATUSES, DEGRADED_HEALTH_STATUSES } from '@/utils/status';
import { getNodeDisplayName } from '@/utils/nodes';
import { logger } from '@/utils/logger';
import { usePersistentSignal } from '@/hooks/usePersistentSignal';
import { useColumnVisibility } from '@/hooks/useColumnVisibility';
import { STORAGE_KEYS } from '@/utils/localStorage';
import { getBackupInfo } from '@/utils/format';
import { aiChatStore } from '@/stores/aiChat';

type GuestMetadataRecord = Record<string, GuestMetadata>;
type IdleCallbackHandle = number;
type IdleCallback = (deadline?: { didTimeout: boolean; timeRemaining: () => number }) => void;
type IdleCapableWindow = Window & {
  requestIdleCallback?: (callback: IdleCallback, options?: { timeout?: number }) => IdleCallbackHandle;
  cancelIdleCallback?: (handle: IdleCallbackHandle) => void;
};

let cachedGuestMetadata: GuestMetadataRecord | null = null;
let lastPersistedGuestMetadataJSON: string | null = null;
let pendingPersistMetadata: GuestMetadataRecord | null = null;
let persistHandle: number | null = null;
let persistHandleType: 'idle' | 'timeout' | null = null;

const instrumentationEnabled = import.meta.env.DEV && typeof performance !== 'undefined';

const readGuestMetadataCache = (): GuestMetadataRecord => {
  if (cachedGuestMetadata) {
    return cachedGuestMetadata;
  }

  if (typeof window === 'undefined') {
    cachedGuestMetadata = {};
    return cachedGuestMetadata;
  }

  try {
    const raw = window.localStorage.getItem(STORAGE_KEYS.GUEST_METADATA);
    if (!raw) {
      cachedGuestMetadata = {};
      lastPersistedGuestMetadataJSON = null;
      return cachedGuestMetadata;
    }
    const parsed = JSON.parse(raw);
    if (parsed && typeof parsed === 'object') {
      cachedGuestMetadata = parsed as GuestMetadataRecord;
      lastPersistedGuestMetadataJSON = raw;
      return cachedGuestMetadata;
    }
  } catch (err) {
    logger.warn('Failed to parse cached guest metadata', err);
  }

  cachedGuestMetadata = {};
  lastPersistedGuestMetadataJSON = null;
  return cachedGuestMetadata;
};

const clearPendingPersistHandle = (idleWindow: IdleCapableWindow) => {
  if (persistHandle === null || persistHandleType === null) {
    return;
  }

  if (persistHandleType === 'idle' && idleWindow.cancelIdleCallback) {
    idleWindow.cancelIdleCallback(persistHandle);
  } else if (persistHandleType === 'timeout') {
    window.clearTimeout(persistHandle);
  }

  persistHandle = null;
  persistHandleType = null;
};

const runGuestMetadataPersist = () => {
  if (typeof window === 'undefined' || !pendingPersistMetadata) {
    pendingPersistMetadata = null;
    return;
  }

  const metadata = pendingPersistMetadata;
  pendingPersistMetadata = null;

  const markBase = instrumentationEnabled ? `guest-metadata:persist:${Date.now()}` : null;
  if (markBase) {
    performance.mark(`${markBase}:start`);
  }

  let serialized: string;
  try {
    serialized = JSON.stringify(metadata);
  } catch (err) {
    if (markBase) {
      performance.mark(`${markBase}:end`);
      performance.measure(markBase, `${markBase}:start`, `${markBase}:end`);
      performance.clearMarks(`${markBase}:start`);
      performance.clearMarks(`${markBase}:end`);
      performance.clearMeasures(markBase);
    }
    logger.warn('Failed to serialize guest metadata cache', err);
    return;
  }

  if (serialized === lastPersistedGuestMetadataJSON) {
    if (markBase) {
      performance.mark(`${markBase}:end`);
      performance.measure(markBase, `${markBase}:start`, `${markBase}:end`);
      const entries = performance.getEntriesByName(markBase);
      const entry = entries[entries.length - 1];
      if (entry) {
        logger.debug('[guestMetadataCache] skipped persist (unchanged)', {
          durationMs: entry.duration,
        });
      }
      performance.clearMarks(`${markBase}:start`);
      performance.clearMarks(`${markBase}:end`);
      performance.clearMeasures(markBase);
    }
    return;
  }

  try {
    window.localStorage.setItem(STORAGE_KEYS.GUEST_METADATA, serialized);
    lastPersistedGuestMetadataJSON = serialized;
    if (markBase) {
      performance.mark(`${markBase}:end`);
      performance.measure(markBase, `${markBase}:start`, `${markBase}:end`);
      const entries = performance.getEntriesByName(markBase);
      const entry = entries[entries.length - 1];
      if (entry) {
        logger.debug('[guestMetadataCache] persisted entries', {
          count: Object.keys(metadata).length,
          durationMs: entry.duration,
        });
      }
      performance.clearMarks(`${markBase}:start`);
      performance.clearMarks(`${markBase}:end`);
      performance.clearMeasures(markBase);
    }
  } catch (err) {
    if (markBase) {
      performance.mark(`${markBase}:end`);
      performance.measure(markBase, `${markBase}:start`, `${markBase}:end`);
      performance.clearMarks(`${markBase}:start`);
      performance.clearMarks(`${markBase}:end`);
      performance.clearMeasures(markBase);
    }
    logger.warn('Failed to persist guest metadata cache', err);
  }
};

const queueGuestMetadataPersist = (metadata: GuestMetadataRecord) => {
  cachedGuestMetadata = metadata;

  if (typeof window === 'undefined') {
    return;
  }

  pendingPersistMetadata = metadata;
  const idleWindow = window as IdleCapableWindow;

  clearPendingPersistHandle(idleWindow);

  const schedule: IdleCallback = () => {
    persistHandle = null;
    persistHandleType = null;
    runGuestMetadataPersist();
  };

  if (idleWindow.requestIdleCallback) {
    persistHandleType = 'idle';
    persistHandle = idleWindow.requestIdleCallback(schedule, { timeout: 750 });
  } else {
    persistHandleType = 'timeout';
    persistHandle = window.setTimeout(schedule, 0);
  }
};

interface DashboardProps {
  vms: VM[];
  containers: Container[];
  nodes: Node[];
}

type ViewMode = 'all' | 'vm' | 'lxc';
type StatusMode = 'all' | 'running' | 'degraded' | 'stopped';
type BackupMode = 'all' | 'needs-backup';
type GroupingMode = 'grouped' | 'flat';
type ProblemsMode = 'all' | 'problems';

export function Dashboard(props: DashboardProps) {
  const navigate = useNavigate();
  const ws = useWebSocket();
  const { connected, activeAlerts, initialDataReceived, reconnecting, reconnect } = ws;
  const alertsActivation = useAlertsActivation();
  const alertsEnabled = createMemo(() => alertsActivation.activationState() === 'active');
  const [search, setSearch] = createSignal('');
  const [isSearchLocked, setIsSearchLocked] = createSignal(false);
  const [selectedNode, setSelectedNode] = createSignal<string | null>(null);
  const [guestMetadata, setGuestMetadata] = createSignal<GuestMetadataRecord>(
    readGuestMetadataCache(),
  );

  const updateGuestMetadataState = (updater: (prev: GuestMetadataRecord) => GuestMetadataRecord) =>
    setGuestMetadata((prev) => {
      const next = updater(prev);
      if (next === prev) {
        return prev;
      }
      queueGuestMetadataPersist(next);
      return next;
    });

  // Combine VMs and containers into a single list for filtering
  const allGuests = createMemo<(VM | Container)[]>(() => [...props.vms, ...props.containers]);

  // Initialize from localStorage with proper type checking
  const [viewMode, setViewMode] = usePersistentSignal<ViewMode>('dashboardViewMode', 'all', {
    deserialize: (raw) => (raw === 'all' || raw === 'vm' || raw === 'lxc' ? raw : 'all'),
  });

  const [statusMode, setStatusMode] = usePersistentSignal<StatusMode>('dashboardStatusMode', 'all', {
    deserialize: (raw) =>
      raw === 'all' || raw === 'running' || raw === 'degraded' || raw === 'stopped'
        ? (raw as StatusMode)
        : 'all',
  });

  // Backup filter mode - filter by backup status
  const [backupMode, setBackupMode] = usePersistentSignal<BackupMode>('dashboardBackupMode', 'all', {
    deserialize: (raw) => (raw === 'all' || raw === 'needs-backup' ? raw : 'all'),
  });

  // Grouping mode - grouped by node or flat list
  const [groupingMode, setGroupingMode] = usePersistentSignal<GroupingMode>(
    'dashboardGroupingMode',
    'grouped',
    {
      deserialize: (raw) => (raw === 'grouped' || raw === 'flat' ? raw : 'grouped'),
    },
  );

  // Problems mode - show only guests with issues
  const [problemsMode, setProblemsMode] = usePersistentSignal<ProblemsMode>(
    'dashboardProblemsMode',
    'all',
    {
      deserialize: (raw) => (raw === 'all' || raw === 'problems' ? raw : 'all'),
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

  // Column visibility management
  // OS and IP columns are hidden by default since they require guest agent and may show dashes
  const columnVisibility = useColumnVisibility(
    STORAGE_KEYS.DASHBOARD_HIDDEN_COLUMNS,
    GUEST_COLUMNS as GuestColumnDef[],
    ['os', 'ip']  // Default hidden columns for cleaner first-run experience
  );
  const visibleColumns = columnVisibility.visibleColumns;
  const visibleColumnIds = createMemo(() => visibleColumns().map(c => c.id));

  // Total columns for colspan calculations
  const totalColumns = createMemo(() => visibleColumns().length);

  // Load all guest metadata on mount (single API call for all guests)
  onMount(async () => {
    try {
      const metadata = await GuestMetadataAPI.getAllMetadata();
      updateGuestMetadataState(() => metadata || {});
    } catch (err) {
      // Silently fail - metadata is optional for display
      logger.debug('Failed to load guest metadata', err);
    }
  });

  // Callback to update a guest's custom URL in metadata
  const handleCustomUrlUpdate = (guestId: string, url: string) => {
    const trimmedUrl = url.trim();
    const nextUrl = trimmedUrl === '' ? undefined : trimmedUrl;
    const currentUrl = guestMetadata()[guestId]?.customUrl;
    if (currentUrl === nextUrl) {
      return;
    }

    updateGuestMetadataState((prev) => {
      const previousEntry = prev[guestId];

      if (nextUrl === undefined) {
        if (!previousEntry || typeof previousEntry.customUrl === 'undefined') {
          return prev;
        }
        const { customUrl: _removed, ...restEntry } = previousEntry;
        const hasAdditionalMetadata = Object.entries(restEntry).some(
          ([key, value]) => key !== 'id' && value !== undefined,
        );

        if (!hasAdditionalMetadata) {
          const { [guestId]: _omit, ...rest } = prev;
          return rest;
        }

        return {
          ...prev,
          [guestId]: {
            ...restEntry,
            id: restEntry.id ?? guestId,
          },
        };
      }

      if (previousEntry && previousEntry.customUrl === nextUrl) {
        return prev;
      }

      const nextEntry: GuestMetadata = {
        ...(previousEntry || { id: guestId }),
        customUrl: nextUrl,
      };

      return {
        ...prev,
        [guestId]: nextEntry,
      };
    });
  };

  // Create a mapping from node ID to node object
  // Also maps by instance-nodeName for guest grouping compatibility
  const nodeByInstance = createMemo(() => {
    const map: Record<string, Node> = {};
    props.nodes.forEach((node) => {
      // Map by node.id (may be clusterName-nodeName or instance-nodeName)
      map[node.id] = node;
      // Also map by instance-nodeName for guest grouping (guests use instance-node format)
      const legacyKey = `${node.instance}-${node.name}`;
      if (!map[legacyKey]) {
        map[legacyKey] = node;
      }
    });
    return map;
  });

  // PERFORMANCE: Pre-compute guest-to-parent-node mapping for faster lookups
  // This avoids repeated node lookups for each guest during render
  const guestParentNodeMap = createMemo(() => {
    const nodes = nodeByInstance();
    const mapping = new Map<string, Node>();

    allGuests().forEach((guest) => {
      // Try guest.id-based lookup first
      if (guest.id) {
        const lastDash = guest.id.lastIndexOf('-');
        if (lastDash > 0) {
          const nodeId = guest.id.slice(0, lastDash);
          if (nodes[nodeId]) {
            mapping.set(guest.id, nodes[nodeId]);
            return;
          }
        }
      }
      // Fallback to composite key
      const compositeKey = `${guest.instance}-${guest.node}`;
      if (nodes[compositeKey]) {
        mapping.set(guest.id || `${guest.instance}-${guest.vmid}`, nodes[compositeKey]);
      }
    });

    return mapping;
  });

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

  // PERFORMANCE: Memoized sort comparator to avoid duplicating sorting logic
  // This comparator is reused by both flat and grouped modes in groupedGuests
  const guestSortComparator = createMemo(() => {
    const key = sortKey();
    const dir = sortDirection();

    if (!key) {
      return null;
    }

    return (a: VM | Container, b: VM | Container): number => {
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
        aVal = a.cpu * 100;
        bVal = b.cpu * 100;
      } else if (key === 'memory') {
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

      // Type-specific comparison
      if (typeof aVal === 'number' && typeof bVal === 'number') {
        const comparison = aVal < bVal ? -1 : 1;
        return dir === 'asc' ? comparison : -comparison;
      } else {
        const aStr = String(aVal).toLowerCase();
        const bStr = String(bVal).toLowerCase();

        if (aStr === bStr) return 0;
        const comparison = aStr < bStr ? -1 : 1;
        return dir === 'asc' ? comparison : -comparison;
      }
    };
  });

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
          statusMode() !== 'all' ||
          backupMode() !== 'all' ||
          problemsMode() !== 'all';

        if (hasActiveFilters) {
          // Clear ALL filters including search text, tag filters, node selection, and view modes
          setSearch('');
          setIsSearchLocked(false);
          setSortKey('vmid');
          setSortDirection('asc');
          setSelectedNode(null);
          setViewMode('all');
          setStatusMode('all');
          setBackupMode('all');
          setProblemsMode('all');

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
        // Check if AI chat is open - if so, focus that instead
        if (aiChatStore.focusInput()) {
          // AI chat input was focused, let the character be typed there
          return;
        }
        // Otherwise, focus the search input
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

  // Compute guests with problems - used for AI investigation regardless of filter state
  const problemGuests = createMemo(() => {
    return allGuests().filter((g) => {
      // Skip templates
      if (g.template) return false;

      // Check for degraded status
      const status = (g.status || '').toLowerCase();
      const isDegraded = DEGRADED_HEALTH_STATUSES.has(status) ||
        (status !== 'running' && !OFFLINE_HEALTH_STATUSES.has(status) && status !== 'stopped');
      if (isDegraded) return true;

      // Check for backup issues
      const backupInfo = getBackupInfo(g.lastBackup);
      if (backupInfo.status === 'stale' || backupInfo.status === 'critical' || backupInfo.status === 'never') {
        return true;
      }

      // Check for high resource usage (>90%)
      if (g.cpu > 0.9) return true;
      if (g.memory && g.memory.usage && g.memory.usage > 90) return true;

      return false;
    });
  });

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
      // Include both traditional LXC and OCI containers (Proxmox 9.1+)
      guests = guests.filter((g) => g.type === 'lxc' || g.type === 'oci');
    }

    // Filter by status
    if (statusMode() === 'running') {
      guests = guests.filter((g) => g.status === 'running');
    } else if (statusMode() === 'degraded') {
      guests = guests.filter((g) => {
        const status = (g.status || '').toLowerCase();
        return (
          DEGRADED_HEALTH_STATUSES.has(status) ||
          (status !== 'running' && !OFFLINE_HEALTH_STATUSES.has(status))
        );
      });
    } else if (statusMode() === 'stopped') {
      guests = guests.filter((g) => g.status !== 'running');
    }

    // Filter by backup status
    if (backupMode() === 'needs-backup') {
      guests = guests.filter((g) => {
        // Skip templates - they don't need backups
        if (g.template) return false;
        const backupInfo = getBackupInfo(g.lastBackup);
        // Show guests that need backup: stale, critical, or never backed up
        return backupInfo.status === 'stale' || backupInfo.status === 'critical' || backupInfo.status === 'never';
      });
    }

    // Filter by problems mode - show guests that need attention
    if (problemsMode() === 'problems') {
      guests = guests.filter((g) => {
        // Skip templates
        if (g.template) return false;

        // Check for degraded status
        const status = (g.status || '').toLowerCase();
        const isDegraded = DEGRADED_HEALTH_STATUSES.has(status) ||
          (status !== 'running' && !OFFLINE_HEALTH_STATUSES.has(status) && status !== 'stopped');
        if (isDegraded) return true;

        // Check for backup issues
        const backupInfo = getBackupInfo(g.lastBackup);
        if (backupInfo.status === 'stale' || backupInfo.status === 'critical' || backupInfo.status === 'never') {
          return true;
        }

        // Check for high resource usage (>90%)
        if (g.cpu > 0.9) return true;
        if (g.memory && g.memory.usage && g.memory.usage > 90) return true;

        return false;
      });
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
      // PERFORMANCE: Use memoized sort comparator (eliminates ~50 lines of duplicate code)
      const comparator = guestSortComparator();
      if (comparator) {
        groups[''] = groups[''].sort(comparator);
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

    // PERFORMANCE: Use memoized sort comparator (eliminates ~50 lines of duplicate code)
    const comparator = guestSortComparator();
    if (comparator) {
      Object.keys(groups).forEach((node) => {
        groups[node] = groups[node].sort(comparator);
      });
    }

    return groups;
  });

  const totalStats = createMemo(() => {
    const guests = filteredGuests();
    const running = guests.filter((g) => g.status === 'running').length;
    const degraded = guests.filter((g) => {
      const status = (g.status || '').toLowerCase();
      // Count as degraded if explicitly in degraded list, or if not running and not offline/stopped
      return (
        DEGRADED_HEALTH_STATUSES.has(status) ||
        (status !== 'running' && !OFFLINE_HEALTH_STATUSES.has(status))
      );
    }).length;
    const stopped = guests.length - running - degraded;
    const vms = guests.filter((g) => g.type === 'qemu').length;
    const containers = guests.filter((g) => g.type === 'lxc' || g.type === 'oci').length;
    return {
      total: guests.length,
      running,
      degraded,
      stopped,
      vms,
      containers,
    };
  });

  const handleNodeSelect = (nodeId: string | null, nodeType: 'pve' | 'pbs' | 'pmg' | null) => {
    logger.debug('handleNodeSelect called', { nodeId, nodeType });

    // If AI sidebar is open, add node to AI context instead of filtering
    if (aiChatStore.isOpen && nodeId && nodeType === 'pve') {
      const node = props.nodes.find((n) => n.id === nodeId);
      if (node) {
        // Toggle: remove if already in context, add if not
        if (aiChatStore.hasContextItem(nodeId)) {
          aiChatStore.removeContextItem(nodeId);
        } else {
          aiChatStore.addContextItem('node', nodeId, node.name, {
            nodeName: node.name,
            name: node.name,
            type: 'Proxmox Node',
            status: node.status,
            instance: node.instance,
          });
        }
      }
      return;
    }

    // Track selected node for filtering (independent of search)
    if (nodeType === 'pve' || nodeType === null) {
      setSelectedNode(nodeId);
      logger.debug('Set selected node', { nodeId });
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

  // Handle row click - add guest to AI context (works even when sidebar is closed)
  const handleGuestRowClick = (guest: VM | Container) => {
    // Only enable if AI is configured
    if (!aiChatStore.enabled) return;

    const guestId = guest.id || `${guest.instance}-${guest.vmid}`;
    const guestType = guest.type === 'qemu' ? 'vm' : 'container';
    const isOCI = guest.type === 'oci' || ('isOci' in guest && guest.isOci === true);

    // Toggle: remove if already in context, add if not
    if (aiChatStore.hasContextItem(guestId)) {
      aiChatStore.removeContextItem(guestId);
      // If no items left in context and sidebar is open, keep it open for now
    } else {
      // Build context with OCI-specific info when applicable
      const contextData: Record<string, unknown> = {
        guestName: guest.name,
        name: guest.name,
        type: guest.type === 'qemu' ? 'Virtual Machine' : (isOCI ? 'OCI Container' : 'LXC Container'),
        vmid: guest.vmid,
        node: guest.node,
        status: guest.status,
      };

      // Add OCI image info if available
      if (isOCI && 'osTemplate' in guest && guest.osTemplate) {
        contextData.ociImage = guest.osTemplate;
      }

      aiChatStore.addContextItem(guestType, guestId, guest.name, contextData);
      // Auto-open the sidebar when first item is selected
      if (!aiChatStore.isOpen) {
        aiChatStore.open();
      }
    }
  };

  return (
    <div class="space-y-3">
      <ProxmoxSectionNav current="overview" />

      {/* Unified Node Selector */}
      <UnifiedNodeSelector
        currentTab="dashboard"
        globalTemperatureMonitoringEnabled={ws.state.temperatureMonitoringEnabled}
        onNodeSelect={handleNodeSelect}
        nodes={props.nodes}
        filteredVms={filteredGuests().filter((g) => g.type === 'qemu')}
        filteredContainers={filteredGuests().filter((g) => g.type === 'lxc' || g.type === 'oci')}
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
        backupMode={backupMode}
        setBackupMode={setBackupMode}
        problemsMode={problemsMode}
        setProblemsMode={setProblemsMode}
        filteredProblemGuests={problemGuests}
        groupingMode={groupingMode}
        setGroupingMode={setGroupingMode}
        setSortKey={setSortKey}
        setSortDirection={setSortDirection}
        searchInputRef={(el) => (searchInputRef = el)}
        availableColumns={columnVisibility.availableToggles()}
        isColumnHidden={columnVisibility.isHiddenByUser}
        onColumnToggle={columnVisibility.toggle}
        onColumnReset={columnVisibility.resetToDefaults}
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
          <Card padding="none" tone="glass" class="mb-4 overflow-hidden">
            <div class="overflow-x-auto">
              <table class="w-full border-collapse whitespace-nowrap" style={{ "min-width": "900px" }}>
                <thead>
                  <tr class="bg-gray-50 dark:bg-gray-700/50 text-gray-600 dark:text-gray-300 border-b border-gray-200 dark:border-gray-700">
                    <For each={visibleColumns()}>
                      {(col) => {
                        const isFirst = () => col.id === visibleColumns()[0]?.id;
                        const sortKeyForCol = col.sortKey as keyof (VM | Container) | undefined;
                        const isSortable = !!sortKeyForCol;
                        const isSorted = () => sortKeyForCol && sortKey() === sortKeyForCol;

                        return (
                          <th
                            class={`py-1 text-[11px] sm:text-xs font-medium uppercase tracking-wider whitespace-nowrap
                              ${isFirst() ? 'pl-4 pr-2 text-left' : 'px-2 text-center'}
                              ${isSortable ? 'cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600' : ''}`}
                            style={{
                              ...(col.width ? { "min-width": col.width } : {}),
                              "vertical-align": 'middle'
                            }}
                            onClick={() => isSortable && handleSort(sortKeyForCol!)}
                            title={col.icon ? col.label : undefined}
                          >
                            <div class={`flex items-center gap-0.5 ${isFirst() ? 'justify-start' : 'justify-center'}`} style={{ "min-height": "14px" }}>
                              {col.icon ? (
                                <span class="flex items-center">{col.icon}</span>
                              ) : (
                                col.label
                              )}
                              {isSorted() && (sortDirection() === 'asc' ? ' ▲' : ' ▼')}
                            </div>
                          </th>
                        );
                      }}
                    </For>
                  </tr>
                </thead>
                <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
                  <For
                    each={Object.entries(groupedGuests()).sort(([instanceIdA], [instanceIdB]) => {
                      const nodeA = nodeByInstance()[instanceIdA];
                      const nodeB = nodeByInstance()[instanceIdB];
                      const labelA = nodeA ? getNodeDisplayName(nodeA) : instanceIdA;
                      const labelB = nodeB ? getNodeDisplayName(nodeB) : instanceIdB;
                      return labelA.localeCompare(labelB) || instanceIdA.localeCompare(instanceIdB);
                    })}
                    fallback={<></>}
                  >
                    {([instanceId, guests]) => {
                      const node = nodeByInstance()[instanceId];
                      return (
                        <>
                          <Show when={node && groupingMode() === 'grouped'}>
                            <NodeGroupHeader node={node!} renderAs="tr" colspan={totalColumns()} />
                          </Show>
                          <For each={guests} fallback={<></>}>
                            {(guest, index) => {
                              const guestId = guest.id || `${guest.instance}-${guest.vmid}`;
                              const metadata =
                                guestMetadata()[guestId] ||
                                guestMetadata()[`${guest.node}-${guest.vmid}`];
                              // PERFORMANCE: Use pre-computed parent node map instead of resolveParentNode
                              const parentNode = node ?? guestParentNodeMap().get(guestId);
                              const parentNodeOnline = parentNode ? isNodeOnline(parentNode) : true;

                              // Get adjacent guest IDs for merged AI context borders
                              const prevGuest = guests[index() - 1];
                              const nextGuest = guests[index() + 1];
                              const prevGuestId = prevGuest ? (prevGuest.id || `${prevGuest.instance}-${prevGuest.vmid}`) : null;
                              const nextGuestId = nextGuest ? (nextGuest.id || `${nextGuest.instance}-${nextGuest.vmid}`) : null;

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
                                    isGroupedView={groupingMode() === 'grouped'}
                                    visibleColumnIds={visibleColumnIds()}
                                    aboveGuestId={prevGuestId}
                                    belowGuestId={nextGuestId}
                                    onRowClick={aiChatStore.enabled ? handleGuestRowClick : undefined}
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
            </div>
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
            <Show when={totalStats().degraded > 0}>
              <span class="text-gray-400">|</span>
              <span class="flex items-center gap-1 text-xs text-gray-600 dark:text-gray-400">
                <span class="h-2 w-2 bg-orange-500 rounded-full"></span>
                {totalStats().degraded} degraded
              </span>
            </Show>
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
