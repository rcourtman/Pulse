import { createSignal, createMemo, createEffect, For, Index, Show, onMount } from 'solid-js';
import { useLocation, useNavigate } from '@solidjs/router';
import type { VM, Container, Node } from '@/types/api';
import type { WorkloadGuest, ViewMode } from '@/types/workloads';
import { GuestRow, GUEST_COLUMNS, VIEW_MODE_COLUMNS } from './GuestRow';
import type { WorkloadIOEmphasis } from './GuestRow';
import { GuestDrawer } from './GuestDrawer';
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
import { MigrationNoticeBanner } from '@/components/shared/MigrationNoticeBanner';
import { isNodeOnline, OFFLINE_HEALTH_STATUSES, DEGRADED_HEALTH_STATUSES } from '@/utils/status';
import { getNodeDisplayName } from '@/utils/nodes';
import { logger } from '@/utils/logger';
import { usePersistentSignal } from '@/hooks/usePersistentSignal';
import { useColumnVisibility } from '@/hooks/useColumnVisibility';
import { useBreakpoint } from '@/hooks/useBreakpoint';
import { useV2Workloads } from '@/hooks/useV2Workloads';
import { STORAGE_KEYS } from '@/utils/localStorage';
import { aiChatStore } from '@/stores/aiChat';
import { isKioskMode, subscribeToKioskMode } from '@/utils/url';
import { getCanonicalWorkloadId, resolveWorkloadType } from '@/utils/workloads';
import {
  WorkloadsSummary,
  type WorkloadSummarySnapshot,
} from '@/components/Workloads/WorkloadsSummary';
import {
  isSummaryTimeRange,
} from '@/components/shared/summaryTimeRange';
import {
  dismissMigrationNotice,
  isMigrationNoticeDismissed,
  resolveMigrationNotice,
} from '@/routing/migrationNotices';
import {
  buildWorkloadsPath,
  parseWorkloadsLinkSearch,
  WORKLOADS_PATH,
  WORKLOADS_QUERY_PARAMS,
} from '@/routing/resourceLinks';
import { ScrollToTopButton } from '@/components/shared/ScrollToTopButton';

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

const computeMedian = (values: number[]): number => {
  if (values.length === 0) return 0;
  const sorted = [...values].sort((a, b) => a - b);
  const mid = Math.floor(sorted.length / 2);
  if (sorted.length % 2 === 0) {
    return (sorted[mid - 1] + sorted[mid]) / 2;
  }
  return sorted[mid];
};

const computePercentile = (values: number[], percentile: number): number => {
  if (values.length === 0) return 0;
  const sorted = [...values].sort((a, b) => a - b);
  const clamped = Math.max(0, Math.min(1, percentile));
  const index = Math.max(0, Math.min(sorted.length - 1, Math.ceil(clamped * sorted.length) - 1));
  return sorted[index];
};

const buildIODistribution = (values: number[]) => {
  const valid = values.filter((value) => Number.isFinite(value) && value >= 0);
  if (valid.length === 0) {
    return { median: 0, mad: 0, max: 0, p97: 0, p99: 0, count: 0 };
  }
  const median = computeMedian(valid);
  const deviations = valid.map((value) => Math.abs(value - median));
  const mad = computeMedian(deviations);
  const max = Math.max(...valid, 0);
  const p97 = computePercentile(valid, 0.97);
  const p99 = computePercentile(valid, 0.99);
  return { median, mad, max, p97, p99, count: valid.length };
};

const computeWorkloadIOEmphasis = (guests: WorkloadGuest[]): WorkloadIOEmphasis => {
  const networkValues: number[] = [];
  const diskIOValues: number[] = [];

  for (const guest of guests) {
    const networkIn = Math.max(0, guest.networkIn ?? 0);
    const networkOut = Math.max(0, guest.networkOut ?? 0);
    const diskRead = Math.max(0, guest.diskRead ?? 0);
    const diskWrite = Math.max(0, guest.diskWrite ?? 0);
    networkValues.push(networkIn + networkOut);
    diskIOValues.push(diskRead + diskWrite);
  }

  return {
    network: buildIODistribution(networkValues),
    diskIO: buildIODistribution(diskIOValues),
  };
};

const workloadMetricPercent = (value: number | null | undefined): number => {
  if (typeof value !== 'number' || !Number.isFinite(value)) return 0;
  if (value <= 1) return Math.max(0, value * 100);
  return Math.max(0, value);
};

const workloadSummaryGuestId = (guest: WorkloadGuest): string =>
  getCanonicalWorkloadId(guest);

const workloadNodeScopeId = (guest: WorkloadGuest): string =>
  `${(guest.instance || '').trim()}-${(guest.node || '').trim()}`;

const getKubernetesContextKey = (guest: WorkloadGuest): string => {
  const candidates = [guest.contextLabel, guest.instance, guest.node, guest.namespace];
  for (const value of candidates) {
    const trimmed = (value || '').trim();
    if (trimmed.length > 0) {
      return trimmed;
    }
  }
  return '';
};

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
  useV2Workloads?: boolean;
}

type StatusMode = 'all' | 'running' | 'degraded' | 'stopped';
type GroupingMode = 'grouped' | 'flat';
type WorkloadSortKey = keyof WorkloadGuest | 'diskIo' | 'netIo';
export function Dashboard(props: DashboardProps) {
  const navigate = useNavigate();
  const location = useLocation();
  const ws = useWebSocket();
  const { connected, activeAlerts, initialDataReceived, reconnecting, reconnect } = ws;
  const { isMobile } = useBreakpoint();
  const alertsActivation = useAlertsActivation();
  const alertsEnabled = createMemo(() => alertsActivation.activationState() === 'active');
  const isWorkloadsRoute = () => location.pathname === WORKLOADS_PATH;

  // Kiosk mode - hide filter panel for clean dashboard display
  // Usage: Add ?kiosk=1 to URL or use the toggle button in the header
  const [kioskMode, setKioskMode] = createSignal(isKioskMode());

  // Subscribe to kiosk mode changes from toggle button or URL params
  onMount(() => {
    const unsubscribe = subscribeToKioskMode((enabled) => {
      setKioskMode(enabled);
    });
    // Cleanup on unmount would go here, but Dashboard is always mounted
    return unsubscribe;
  });

  const [search, setSearch] = createSignal('');
  const [isSearchLocked, setIsSearchLocked] = createSignal(false);
  const [selectedNode, setSelectedNode] = createSignal<string | null>(null);
  const [selectedKubernetesContext, setSelectedKubernetesContext] = createSignal<string | null>(null);
  const [selectedGuestId, setSelectedGuestIdRaw] = createSignal<string | null>(null);
  const [hoveredWorkloadId, setHoveredWorkloadId] = createSignal<string | null>(null);
  const [handledResourceId, setHandledResourceId] = createSignal<string | null>(null);
  const [handledTypeParam, setHandledTypeParam] = createSignal<string | null>(null);
  const [handledContextParam, setHandledContextParam] = createSignal('');
  const [handledHostParam, setHandledHostParam] = createSignal('');
  const [selectedHostHint, setSelectedHostHint] = createSignal<string | null>(null);
  const [hideMigrationNotice, setHideMigrationNotice] = createSignal(true);

  const migrationNotice = createMemo(() => {
    const notice = resolveMigrationNotice(location.search);
    if (!notice || notice.target !== 'workloads') return null;
    return notice;
  });

  createEffect(() => {
    const notice = migrationNotice();
    if (!notice) {
      setHideMigrationNotice(true);
      return;
    }
    setHideMigrationNotice(isMigrationNoticeDismissed(notice.id));
  });

  const handleDismissMigrationNotice = () => {
    const notice = migrationNotice();
    if (!notice) return;
    dismissMigrationNotice(notice.id);
    setHideMigrationNotice(true);
  };

  createEffect(() => {
    const { resource: resourceId } = parseWorkloadsLinkSearch(location.search);
    if (!resourceId || resourceId === handledResourceId()) return;
    setSelectedGuestId(resourceId);
    const [instance, node, vmid] = resourceId.split(':');
    if (instance && node && vmid) {
      setSelectedNode(`${instance}-${node}`);
    }
    setHandledResourceId(resourceId);
  });


  // Wrap setSelectedGuestId to preserve scroll position. Opening/closing the
  // drawer mounts/unmounts GuestDrawer (which contains DiscoveryTab). The
  // DiscoveryTab initialization triggers SolidJS to recreate the <For> row,
  // which detaches/reattaches DOM and resets the scroll container's scrollTop.
  // We find the scroll container, save its position, and restore it after.
  let tableRef: HTMLDivElement | undefined;
  const setSelectedGuestId = (id: string | null) => {
    // Find the nearest ancestor scroll container from the table
    let scroller: HTMLElement | null = tableRef ?? null;
    while (scroller) {
      const { overflowY } = getComputedStyle(scroller);
      if ((overflowY === 'auto' || overflowY === 'scroll') && scroller.scrollHeight > scroller.clientHeight) {
        break;
      }
      scroller = scroller.parentElement;
    }
    const scrollTop = scroller?.scrollTop ?? 0;
    setSelectedGuestIdRaw(id);
    // Restore scroll position — opening the drawer can cause layout changes
    if (scroller) scroller.scrollTop = scrollTop;
    requestAnimationFrame(() => {
      if (scroller) scroller.scrollTop = scrollTop;
    });
  };
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

  const v2Enabled = createMemo(() => props.useV2Workloads === true);
  const v2Workloads = useV2Workloads(v2Enabled);

  // Keep workload identities stable across v2 polling updates.
  const dedupeGuests = (guests: WorkloadGuest[]): WorkloadGuest[] => {
    const seen = new Set<string>();
    const deduped: WorkloadGuest[] = [];
    for (const guest of guests) {
      const canonicalId = getCanonicalWorkloadId(guest);
      if (seen.has(canonicalId)) continue;
      seen.add(canonicalId);
      deduped.push(guest);
    }
    return deduped;
  };

  const allGuests = createMemo<WorkloadGuest[]>(() =>
    v2Enabled() ? dedupeGuests(v2Workloads.workloads()) : [],
  );

  const workloadNodeOptions = createMemo(() => {
    const labelsByScope = new Map<string, string>();
    const nodeNameCounts = new Map<string, number>();

    for (const guest of allGuests()) {
      const type = resolveWorkloadType(guest);
      if (type === 'k8s') continue;
      const scope = workloadNodeScopeId(guest);
      if (!scope || scope === '-') continue;
      const nodeName = (guest.node || '').trim();
      if (!nodeName) continue;
      nodeNameCounts.set(nodeName, (nodeNameCounts.get(nodeName) || 0) + 1);
    }

    for (const guest of allGuests()) {
      const type = resolveWorkloadType(guest);
      if (type === 'k8s') continue;
      const scope = workloadNodeScopeId(guest);
      if (!scope || scope === '-' || labelsByScope.has(scope)) continue;
      const nodeName = (guest.node || '').trim();
      const instance = (guest.instance || '').trim();
      if (!nodeName) continue;
      const hasDuplicateNodeName = (nodeNameCounts.get(nodeName) || 0) > 1;
      const label = hasDuplicateNodeName && instance ? `${nodeName} (${instance})` : nodeName;
      labelsByScope.set(scope, label);
    }

    return Array.from(labelsByScope.entries())
      .map(([value, label]) => ({ value, label }))
      .sort((a, b) => a.label.localeCompare(b.label));
  });

  createEffect(() => {
    if (viewMode() === 'k8s') return;
    const hostHint = selectedHostHint();
    if (!hostHint || selectedNode() !== null) return;
    const normalizedHint = hostHint.trim().toLowerCase();
    if (!normalizedHint) return;
    const option = workloadNodeOptions().find((candidate) => {
      const label = candidate.label.toLowerCase();
      const value = candidate.value.toLowerCase();
      return label === normalizedHint || value === normalizedHint || label.includes(normalizedHint);
    });
    if (!option) return;
    setSelectedNode(option.value);
    setSelectedHostHint(null);
  });

  const kubernetesContextOptions = createMemo(() => {
    const contexts = new Set<string>();
    for (const guest of allGuests()) {
      if (resolveWorkloadType(guest) !== 'k8s') continue;
      const context = getKubernetesContextKey(guest);
      if (context) {
        contexts.add(context);
      }
    }
    return Array.from(contexts).sort((a, b) => a.localeCompare(b));
  });

  // Initialize from localStorage with proper type checking
  const [viewMode, setViewMode] = usePersistentSignal<ViewMode>('dashboardViewMode', 'all', {
    deserialize: (raw) =>
      raw === 'all' || raw === 'vm' || raw === 'lxc' || raw === 'docker' || raw === 'k8s'
        ? raw
        : 'all',
  });

  createEffect(() => {
    if (!isWorkloadsRoute()) return;
    if (viewMode() === 'k8s') {
      if (selectedNode() !== null) {
        setSelectedNode(null);
      }
      if (selectedHostHint() !== null) {
        setSelectedHostHint(null);
      }
      return;
    }
    if (selectedKubernetesContext() !== null) {
      setSelectedKubernetesContext(null);
    }
  });

  const normalizeTypeParam = (value: string): ViewMode | null => {
    const normalized = value.trim().toLowerCase();
    if (normalized === 'all') return 'all';
    if (normalized === 'vm' || normalized === 'qemu') return 'vm';
    if (normalized === 'lxc') return 'lxc';
    if (normalized === 'docker' || normalized === 'container' || normalized === 'containers') return 'docker';
    if (normalized === 'k8s' || normalized === 'kubernetes' || normalized === 'pod') return 'k8s';
    return null;
  };

  createEffect(() => {
    const { type: typeParam } = parseWorkloadsLinkSearch(location.search);
    if (!typeParam || typeParam === handledTypeParam()) return;
    const nextMode = normalizeTypeParam(typeParam);
    if (!nextMode) return;
    setViewMode(nextMode);
    setHandledTypeParam(typeParam);
  });

  createEffect(() => {
    const { context: contextParam } = parseWorkloadsLinkSearch(location.search);
    if (contextParam === handledContextParam()) return;

    if (contextParam) {
      if (viewMode() !== 'k8s') {
        setViewMode('k8s');
      }
      setSelectedKubernetesContext(contextParam);
      if (!showFilters()) {
        setShowFilters(true);
      }
      setHandledContextParam(contextParam);
      return;
    }

    setSelectedKubernetesContext(null);
    setHandledContextParam('');
  });

  createEffect(() => {
    const { host: hostParam } = parseWorkloadsLinkSearch(location.search);
    if (hostParam === handledHostParam()) return;

    if (hostParam) {
      setSelectedHostHint(hostParam);
      if (!showFilters()) {
        setShowFilters(true);
      }
      setHandledHostParam(hostParam);
      return;
    }

    setSelectedHostHint(null);
    setHandledHostParam('');
  });

  createEffect(() => {
    if (!isWorkloadsRoute()) return;

    const params = new URLSearchParams(location.search);
    const nextType = viewMode() === 'all' ? '' : viewMode();
    const nextContext = viewMode() === 'k8s' ? selectedKubernetesContext() ?? '' : '';
    const nextHost = viewMode() === 'k8s' ? '' : selectedNode() ?? selectedHostHint() ?? '';

    const managedPath = buildWorkloadsPath({
      type: nextType || null,
      context: nextContext || null,
      host: nextHost || null,
    });
    const managedUrl = new URL(managedPath, 'http://pulse.local');
    params.delete(WORKLOADS_QUERY_PARAMS.type);
    params.delete(WORKLOADS_QUERY_PARAMS.context);
    params.delete(WORKLOADS_QUERY_PARAMS.host);
    managedUrl.searchParams.forEach((value, key) => {
      params.set(key, value);
    });

    const nextSearch = params.toString();
    const nextPath = nextSearch ? `${WORKLOADS_PATH}?${nextSearch}` : WORKLOADS_PATH;
    const currentPath = `${location.pathname}${location.search || ''}`;
    if (nextPath !== currentPath) {
      navigate(nextPath, { replace: true });
    }
  });

  const [statusMode, setStatusMode] = usePersistentSignal<StatusMode>('dashboardStatusMode', 'all', {
    deserialize: (raw) =>
      raw === 'all' || raw === 'running' || raw === 'degraded' || raw === 'stopped'
        ? (raw as StatusMode)
        : 'all',
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
  const [workloadsSummaryRange, setWorkloadsSummaryRange] = usePersistentSignal(
    STORAGE_KEYS.WORKLOADS_SUMMARY_RANGE,
    '1h',
    {
      deserialize: (raw) => (isSummaryTimeRange(raw) ? raw : '1h'),
    },
  );

  // Sorting state - default to type ascending so VMs/LXCs/Docker/K8s cluster together
  const [sortKey, setSortKey] = createSignal<WorkloadSortKey | null>('type');
  const [sortDirection, setSortDirection] = createSignal<'asc' | 'desc'>('asc');

  // Column visibility management
  // OS and IP columns are hidden by default since they require guest agent and may show dashes
  const relevantColumns = createMemo(() => {
    const base = VIEW_MODE_COLUMNS[viewMode()];
    if (!base) return null;
    // Hide the node column when grouped by node — it's already shown in the group header
    if (groupingMode() === 'grouped' && base.has('node')) {
      const filtered = new Set(base);
      filtered.delete('node');
      return filtered;
    }
    return base;
  });
  const columnVisibility = useColumnVisibility(
    STORAGE_KEYS.DASHBOARD_HIDDEN_COLUMNS,
    GUEST_COLUMNS,
    ['os', 'ip'],  // Default hidden columns for cleaner first-run experience
    relevantColumns
  );
  const visibleColumns = columnVisibility.visibleColumns;
  const visibleColumnIds = createMemo(() => visibleColumns().map(c => c.id));

  // Total columns for colspan calculations
  const totalColumns = createMemo(() => visibleColumns().length);

  // Helper function to refresh guest metadata from server
  const refreshGuestMetadata = async () => {
    try {
      const metadata = await GuestMetadataAPI.getAllMetadata();
      updateGuestMetadataState(() => metadata || {});
      logger.debug('Guest metadata refreshed');
    } catch (err) {
      logger.debug('Failed to refresh guest metadata', err);
    }
  };

  // Load all guest metadata on mount (single API call for all guests)
  onMount(async () => {
    await refreshGuestMetadata();

    // Listen for metadata changes from AI or other sources
    const handleMetadataChanged = (event: Event) => {
      const customEvent = event as CustomEvent;
      logger.debug('[Dashboard] Metadata changed event received', customEvent.detail);

      // Handle optimistic update if payload is present - this fixes the "guest url not appearing straight away" issue
      if (customEvent.detail?.payload) {
        let { guestId, url } = customEvent.detail.payload;
        if (guestId) {
          // Normalize guestId if it's in the canonical AI format (instance:node:vmid)
          // Frontend uses 'instance-vmid' (e.g., 'delly-101') but AI sends 'delly:delly:101'
          if (guestId.includes(':')) {
            const parts = guestId.split(':');
            if (parts.length === 3) {
              const [instance, _node, vmid] = parts;
              // Construct frontend ID format
              guestId = `${instance}-${vmid}`;
              logger.debug('[Dashboard] Normalized optimistic guestId', { original: customEvent.detail.payload.guestId, normalized: guestId });
            }
          }

          logger.debug('[Dashboard] Applying optimistic metadata update', { guestId, url });
          // Ensure url is a string (handle null/undefined for removal)
          handleCustomUrlUpdate(guestId, url || '');
          // Skip immediate refresh to prevent race condition with backend consistency
          // The optimistic update is authoritative for this action
          return;
        }
      }

      logger.debug('Metadata changed event received, refreshing...');
      refreshGuestMetadata();
    };

    logger.debug('[Dashboard] Adding pulse:metadata-changed listener');
    window.addEventListener('pulse:metadata-changed', handleMetadataChanged);

    // Note: SolidJS onMount doesn't support cleanup return, so we rely on component unmount
    // In practice, Dashboard is always mounted so this is fine
  });

  let lastConnected = connected();
  createEffect(() => {
    const isConnected = connected();
    if (v2Enabled() && isConnected && !lastConnected) {
      void v2Workloads.refetch();
    }
    lastConnected = isConnected;
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
      const canonicalGuestId = getCanonicalWorkloadId(guest);
      // Try guest.id-based lookup first
      if (guest.id) {
        const lastDash = guest.id.lastIndexOf('-');
        if (lastDash > 0) {
          const nodeId = guest.id.slice(0, lastDash);
          if (nodes[nodeId]) {
            mapping.set(canonicalGuestId, nodes[nodeId]);
            return;
          }
        }
      }
      // Fallback to composite key
      const compositeKey = `${guest.instance}-${guest.node}`;
      if (nodes[compositeKey]) {
        mapping.set(canonicalGuestId, nodes[compositeKey]);
      }
    });

    return mapping;
  });

  // Sort handler
  const handleSort = (key: WorkloadSortKey) => {
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
        key === 'diskIo' ||
        key === 'netIo' ||
        key === 'uptime'
      ) {
        setSortDirection('desc');
      } else {
        setSortDirection('asc');
      }
    }
  };

  const getDiskUsagePercent = (guest: WorkloadGuest): number | null => {
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

    // Stable tiebreaker: when primary sort values are equal, compare by
    // name then id so that row positions never shift between polls.
    // Without this, <Index> position changes cause the drawer to glitch.
    const tiebreak = (a: WorkloadGuest, b: WorkloadGuest): number => {
      const nameA = (a.name || '').toLowerCase();
      const nameB = (b.name || '').toLowerCase();
      if (nameA !== nameB) return nameA < nameB ? -1 : 1;
      return a.id < b.id ? -1 : a.id > b.id ? 1 : 0;
    };

    return (a: WorkloadGuest, b: WorkloadGuest): number => {
      let aVal: string | number | boolean | null | undefined = null;
      let bVal: string | number | boolean | null | undefined = null;

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
      } else if (key === 'diskIo') {
        aVal = Math.max(0, a.diskRead || 0) + Math.max(0, a.diskWrite || 0);
        bVal = Math.max(0, b.diskRead || 0) + Math.max(0, b.diskWrite || 0);
      } else if (key === 'netIo') {
        aVal = Math.max(0, a.networkIn || 0) + Math.max(0, a.networkOut || 0);
        bVal = Math.max(0, b.networkIn || 0) + Math.max(0, b.networkOut || 0);
      } else {
        aVal = a[key] as string | number | boolean | null | undefined;
        bVal = b[key] as string | number | boolean | null | undefined;
      }

      // Handle null/undefined/empty values - put at end for both asc and desc
      const aIsEmpty = aVal === null || aVal === undefined || aVal === '';
      const bIsEmpty = bVal === null || bVal === undefined || bVal === '';

      if (aIsEmpty && bIsEmpty) return tiebreak(a, b);
      if (aIsEmpty) return 1;
      if (bIsEmpty) return -1;

      // Type-specific comparison
      if (typeof aVal === 'number' && typeof bVal === 'number') {
        if (aVal === bVal) return tiebreak(a, b);
        const comparison = aVal < bVal ? -1 : 1;
        return dir === 'asc' ? comparison : -comparison;
      } else {
        const aStr = String(aVal).toLowerCase();
        const bStr = String(bVal).toLowerCase();

        if (aStr === bStr) return tiebreak(a, b);
        const comparison = aStr < bStr ? -1 : 1;
        return dir === 'asc' ? comparison : -comparison;
      }
    };
  });

  // Handle keyboard shortcuts
  let searchInputRef: HTMLInputElement | undefined;

  createEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      // Escape key behavior
      if (e.key === 'Escape') {
        // First check if we have search/filters to clear (including tag filters and node selection)
        const hasActiveFilters =
          search().trim() ||
          sortKey() !== 'type' ||
          sortDirection() !== 'asc' ||
          selectedNode() !== null ||
          selectedHostHint() !== null ||
          selectedKubernetesContext() !== null ||
          viewMode() !== 'all' ||
          statusMode() !== 'all';

        if (hasActiveFilters) {
          // Clear ALL filters including search text, tag filters, node selection, and view modes
          setSearch('');
          setIsSearchLocked(false);
          setSortKey('type');
          setSortDirection('asc');
          setSelectedNode(null);
          setSelectedHostHint(null);
          setSelectedKubernetesContext(null);
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
      }
    };

    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  });

  // Filter guests based on current settings
  const filteredGuests = createMemo(() => {
    let guests = allGuests();

    // Filter by selected non-kubernetes node scope.
    const nodeScope = selectedNode();
    if (nodeScope && viewMode() !== 'k8s') {
      guests = guests.filter((g) => workloadNodeScopeId(g) === nodeScope);
    }
    const hostHint = (selectedHostHint() || '').trim().toLowerCase();
    if (!nodeScope && hostHint && viewMode() !== 'k8s') {
      guests = guests.filter((g) => {
        if (resolveWorkloadType(g) === 'k8s') return false;
        const candidates = [g.node, g.instance, g.contextLabel];
        return candidates.some((candidate) => (candidate || '').toLowerCase().includes(hostHint));
      });
    }
    const k8sContext = selectedKubernetesContext();
    if (k8sContext && viewMode() === 'k8s') {
      guests = guests.filter((g) => resolveWorkloadType(g) === 'k8s' && getKubernetesContextKey(g) === k8sContext);
    }

    // Filter by type
    if (viewMode() !== 'all') {
      guests = guests.filter((g) => resolveWorkloadType(g) === viewMode());
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

  const workloadsSummaryVisibleIds = createMemo<string[]>(() =>
    filteredGuests().map((guest) => workloadSummaryGuestId(guest)),
  );

  const workloadsSummaryFallbackCounts = createMemo(() => {
    const guests = filteredGuests();
    const running = guests.filter((guest) => guest.status === 'running' || guest.status === 'online').length;
    return {
      total: guests.length,
      running,
      stopped: Math.max(0, guests.length - running),
    };
  });

  const workloadsSummaryFallbackSnapshots = createMemo<WorkloadSummarySnapshot[]>(() =>
    filteredGuests().map((guest) => {
      const guestId = workloadSummaryGuestId(guest);
      const memoryUsage = workloadMetricPercent(guest.memory?.usage);
      let diskUsage = workloadMetricPercent(guest.disk?.usage);
      if (
        (!diskUsage || diskUsage <= 0) &&
        typeof guest.disk?.used === 'number' &&
        typeof guest.disk?.total === 'number' &&
        Number.isFinite(guest.disk.used) &&
        Number.isFinite(guest.disk.total) &&
        guest.disk.total > 0
      ) {
        diskUsage = (guest.disk.used / guest.disk.total) * 100;
      }

      return {
        id: guestId,
        name: guest.name || guestId,
        cpu: workloadMetricPercent(guest.cpu),
        memory: memoryUsage,
        disk: Math.max(0, diskUsage),
        network: Math.max(0, guest.networkIn || 0) + Math.max(0, guest.networkOut || 0),
      };
    }),
  );

  createEffect(() => {
    const hoveredId = hoveredWorkloadId();
    if (!hoveredId) return;
    const exists = filteredGuests().some((guest) => workloadSummaryGuestId(guest) === hoveredId);
    if (!exists) {
      setHoveredWorkloadId(null);
    }
  });

  const getGroupKey = (guest: WorkloadGuest): string => {
    const type = resolveWorkloadType(guest);
    if (type === 'vm' || type === 'lxc') {
      return `${guest.instance}-${guest.node}`;
    }
    const context = guest.contextLabel || guest.node || guest.instance || guest.namespace || guest.id;
    return `${type}:${context}`;
  };

  const getGroupLabel = (groupKey: string, guests: WorkloadGuest[]): { type: string; name: string } => {
    const node = nodeByInstance()[groupKey];
    if (node) return { type: '', name: getNodeDisplayName(node) };
    const [prefix, ...rest] = groupKey.split(':');
    const context = rest.length > 0 ? rest.join(':') : groupKey;
    if (prefix === 'docker') return { type: 'Docker', name: context };
    if (prefix === 'k8s') return { type: 'K8s', name: context };
    if (prefix === 'vm') return { type: 'VM', name: context };
    if (prefix === 'lxc') return { type: 'LXC', name: context };
    return { type: '', name: guests[0]?.contextLabel || context };
  };

  // Group by node or return flat list based on grouping mode
  const groupedGuests = createMemo(() => {
    const guests = filteredGuests();

    // If flat mode, return all guests in a single group
    if (groupingMode() === 'flat') {
      const groups: Record<string, WorkloadGuest[]> = { '': guests };
      // PERFORMANCE: Use memoized sort comparator (eliminates ~50 lines of duplicate code)
      const comparator = guestSortComparator();
      if (comparator) {
        groups[''] = groups[''].sort(comparator);
      }
      return groups;
    }

    // Group by node ID (instance + node name) to match Node.ID format
    const groups: Record<string, WorkloadGuest[]> = {};
    guests.forEach((guest) => {
      const nodeId = getGroupKey(guest);

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

  // Stable sorted group keys for <For> — strings compare by value so DOM stays stable across data updates
  const sortedGroupKeys = createMemo(() => {
    const groups = groupedGuests();
    const nodes = nodeByInstance();
    return Object.keys(groups).sort((a, b) => {
      const nodeA = nodes[a];
      const nodeB = nodes[b];
      const labelA = nodeA ? getNodeDisplayName(nodeA) : getGroupLabel(a, groups[a]).name;
      const labelB = nodeB ? getNodeDisplayName(nodeB) : getGroupLabel(b, groups[b]).name;
      return labelA.localeCompare(labelB) || a.localeCompare(b);
    });
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
    const vms = guests.filter((g) => resolveWorkloadType(g) === 'vm').length;
    const containers = guests.filter((g) => resolveWorkloadType(g) === 'lxc').length;
    const docker = guests.filter((g) => resolveWorkloadType(g) === 'docker').length;
    const k8s = guests.filter((g) => resolveWorkloadType(g) === 'k8s').length;
    return {
      total: guests.length,
      running,
      degraded,
      stopped,
      vms,
      containers,
      docker,
      k8s,
    };
  });

  const workloadIOEmphasis = createMemo(() => computeWorkloadIOEmphasis(filteredGuests()));

  const handleNodeSelect = (nodeId: string | null, nodeType: 'pve' | 'pbs' | 'pmg' | null) => {
    logger.debug('handleNodeSelect called', { nodeId, nodeType });

    // Track selected node for filtering (independent of search)
    if (nodeType === 'pve' || nodeType === null) {
      setSelectedHostHint(null);
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



  return (
    <div class="space-y-3">
      <Show when={isWorkloadsRoute()}>
        <div class="sticky-shield sticky top-0 z-20 bg-white dark:bg-gray-800">
          <WorkloadsSummary
            timeRange={workloadsSummaryRange()}
            onTimeRangeChange={setWorkloadsSummaryRange}
            selectedNodeId={selectedNode()}
            fallbackGuestCounts={workloadsSummaryFallbackCounts()}
            fallbackSnapshots={workloadsSummaryFallbackSnapshots()}
            visibleWorkloadIds={workloadsSummaryVisibleIds()}
            hoveredWorkloadId={hoveredWorkloadId()}
            focusedWorkloadId={selectedGuestId()}
          />
        </div>
      </Show>

      <Show when={isWorkloadsRoute() && migrationNotice() && !hideMigrationNotice()}>
        <MigrationNoticeBanner
          title={migrationNotice()!.title}
          message={migrationNotice()!.message}
          learnMoreHref={migrationNotice()!.learnMoreHref}
          onDismiss={handleDismissMigrationNotice}
        />
      </Show>

      {/* Unified Node Selector - infrastructure summary (hidden on workloads) */}
      <UnifiedNodeSelector
        currentTab="dashboard"
        globalTemperatureMonitoringEnabled={ws.state.temperatureMonitoringEnabled}
        onNodeSelect={handleNodeSelect}
        nodes={props.nodes}
        filteredVms={filteredGuests().filter((g) => resolveWorkloadType(g) === 'vm') as VM[]}
        filteredContainers={filteredGuests().filter((g) => resolveWorkloadType(g) === 'lxc') as Container[]}
        searchTerm={search()}
        showNodeSummary={!isWorkloadsRoute()}
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
          props.nodes.length === 0 &&
          allGuests().length === 0
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
            description="Install the Pulse agent for extra capabilities (temperature monitoring and Pulse Patrol automation), or add a node via API token in Settings → Proxmox."
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

      {/* Dashboard Filter - hidden in kiosk mode */}
      <Show when={!kioskMode() && connected() && initialDataReceived() && allGuests().length > 0}>
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
          onBeforeAutoFocus={() => {
            if (aiChatStore.focusInput()) return true;
            if (!showFilters()) setShowFilters(true);
            return false;
          }}
          availableColumns={columnVisibility.availableToggles()}
          isColumnHidden={columnVisibility.isHiddenByUser}
          onColumnToggle={columnVisibility.toggle}
          onColumnReset={columnVisibility.resetToDefaults}
          hostFilter={(() => {
            if (!isWorkloadsRoute()) return undefined;
            if (viewMode() === 'k8s') {
              return {
                id: 'workloads-k8s-context-filter',
                label: 'Cluster',
                value: selectedKubernetesContext() ?? '',
                options: [
                  { value: '', label: 'All clusters' },
                  ...kubernetesContextOptions().map((context) => ({
                    value: context,
                    label: context,
                  })),
                ],
                onChange: (value: string) => setSelectedKubernetesContext(value || null),
              };
            }
            return {
              id: 'workloads-node-filter',
              label: 'Host',
              value: selectedNode() ?? '',
              options: [
                { value: '', label: 'All nodes' },
                ...workloadNodeOptions(),
              ],
              onChange: (value: string) => {
                setSelectedHostHint(null);
                handleNodeSelect(value || null, value ? 'pve' : null);
              },
            };
          })()}
        />
      </Show>

      {/* Table View */}
      <Show when={connected() && initialDataReceived() && filteredGuests().length > 0}>
        <ComponentErrorBoundary name="Guest Table">
          <Card padding="none" tone="glass" class="mb-4 overflow-hidden">
            <div
              ref={tableRef}
              class="overflow-x-auto"
              style={{ '-webkit-overflow-scrolling': 'touch' }}
            >
              <table class="w-full border-collapse whitespace-nowrap" style={{ "table-layout": "fixed", "min-width": isMobile() ? "800px" : "900px" }}>
                <thead>
                  <tr class="bg-gray-50 dark:bg-gray-700/50 text-gray-600 dark:text-gray-300 border-b border-gray-200 dark:border-gray-700">
                    <For each={visibleColumns()}>
                      {(col) => {
                        const isFirst = () => col.id === visibleColumns()[0]?.id;
                        const sortKeyForCol = col.sortKey as WorkloadSortKey | undefined;
                        const isSortable = !!sortKeyForCol;
                        const isSorted = () => sortKeyForCol && sortKey() === sortKeyForCol;

                        return (
                          <th
                            class={`py-1 text-[11px] sm:text-xs font-medium uppercase tracking-wider whitespace-nowrap
                                  ${isFirst() ? 'pl-4 pr-2 text-left' : 'px-2 text-center'}
                                  ${isSortable ? 'cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600' : ''}`}
                            style={{
                              ...((['cpu', 'memory', 'disk'].includes(col.id))
                                ? { "width": isMobile() ? "60px" : "140px" }
                                : (['netIo', 'diskIo'].includes(col.id))
                                  ? { "width": isMobile() ? "130px" : "170px" }
                                : (col.width ? { "width": col.width } : {})),
                              "vertical-align": 'middle',
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
                  {/* Outer <For> uses string keys — strings compare by value so DOM is stable across data updates */}
                  <For each={sortedGroupKeys()} fallback={<></>}>
                    {(groupKey) => {
                      const groupGuests = () => groupedGuests()[groupKey] || [];
                      const node = () => nodeByInstance()[groupKey];
                      return (
                        <>
                          <Show when={groupingMode() === 'grouped'}>
                            <Show
                              when={node()}
                              fallback={
                                <tr class="bg-gray-50 dark:bg-gray-900/40">
                                  <td
                                    colspan={totalColumns()}
                                    class="py-1 pr-2 pl-4 text-[12px] sm:text-sm font-semibold text-slate-700 dark:text-slate-100"
                                  >
                                    {(() => {
                                      const label = getGroupLabel(groupKey, groupGuests());
                                      return (
                                        <div class="flex items-center gap-3">
                                          <span>{label.name}</span>
                                          <Show when={label.type}>
                                            <span class="rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap bg-slate-200 text-slate-600 dark:bg-slate-700/60 dark:text-slate-300">
                                              {label.type}
                                            </span>
                                          </Show>
                                        </div>
                                      );
                                    })()}
                                  </td>
                                </tr>
                              }
                            >
                              <NodeGroupHeader node={node()!} renderAs="tr" colspan={totalColumns()} />
                            </Show>
                          </Show>
                          {/* Inner <Index> tracks by position — updates props reactively instead of recreating DOM */}
                          <Index each={groupGuests()} fallback={<></>}>
                            {(guest) => {
                              const guestId = () => {
                                const g = guest();
                                return getCanonicalWorkloadId(g);
                              };
                              const getMetadata = () =>
                                guestMetadata()[guestId()] ||
                                guestMetadata()[`${guest().instance}:${guest().node}:${guest().vmid}`];
                              const parentNode = () => node() ?? guestParentNodeMap().get(guestId());
                              const parentNodeOnline = () => {
                                const pn = parentNode();
                                return pn ? isNodeOnline(pn) : true;
                              };

                              return (
                                <ComponentErrorBoundary name="GuestRow">
                                  <GuestRow
                                    guest={guest()}
                                    alertStyles={getAlertStyles(guestId(), activeAlerts, alertsEnabled())}
                                    customUrl={getMetadata()?.customUrl}
                                    onTagClick={handleTagClick}
                                    activeSearch={search()}
                                    parentNodeOnline={parentNodeOnline()}
                                    onCustomUrlUpdate={handleCustomUrlUpdate}
                                    isGroupedView={groupingMode() === 'grouped'}
                                    visibleColumnIds={visibleColumnIds()}
                                    onClick={() => setSelectedGuestId(selectedGuestId() === guestId() ? null : guestId())}
                                    isExpanded={selectedGuestId() === guestId()}
                                    ioEmphasis={workloadIOEmphasis()}
                                    onHoverChange={setHoveredWorkloadId}
                                  />
                                  <Show when={selectedGuestId() === guestId()}>
                                    <tr>
                                      <td colspan={totalColumns()} class="p-0 border-b border-gray-200 dark:border-gray-700 bg-gray-50/50 dark:bg-gray-800/50">
                                        <div class="p-4" onClick={(e) => e.stopPropagation()}>
                                          <GuestDrawer
                                            guest={guest()}
                                            onClose={() => setSelectedGuestId(null)}
                                            customUrl={getMetadata()?.customUrl}
                                            onCustomUrlChange={handleCustomUrlUpdate}
                                          />
                                        </div>
                                      </td>
                                    </tr>
                                  </Show>
                                </ComponentErrorBoundary>
                              );
                            }}
                          </Index>
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
          allGuests().length > 0
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

      <ScrollToTopButton />
    </div>
  );
}
