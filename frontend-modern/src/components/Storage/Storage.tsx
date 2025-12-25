import { Component, For, Show, createSignal, createMemo, createEffect } from 'solid-js';
import { useNavigate } from '@solidjs/router';
import { useWebSocket } from '@/App';
import { getAlertStyles } from '@/utils/alerts';
import { formatBytes, formatPercent } from '@/utils/format';
import type { Storage as StorageType, CephCluster } from '@/types/api';
import { ComponentErrorBoundary } from '@/components/ErrorBoundary';
import { UnifiedNodeSelector } from '@/components/shared/UnifiedNodeSelector';
import { StorageFilter } from './StorageFilter';
import { DiskList } from './DiskList';
import { ZFSHealthMap } from './ZFSHealthMap';
import { EnhancedStorageBar } from './EnhancedStorageBar';
import { Card } from '@/components/shared/Card';
import { EmptyState } from '@/components/shared/EmptyState';
import { NodeGroupHeader } from '@/components/shared/NodeGroupHeader';
import { ProxmoxSectionNav } from '@/components/Proxmox/ProxmoxSectionNav';
import { getNodeDisplayName } from '@/utils/nodes';
import { usePersistentSignal } from '@/hooks/usePersistentSignal';
import { useDebouncedValue } from '@/hooks/useDebouncedValue';
import { useAlertsActivation } from '@/stores/alertsActivation';

type StorageSortKey = 'name' | 'node' | 'type' | 'status' | 'usage' | 'free' | 'total';

const Storage: Component = () => {
  const navigate = useNavigate();
  const { state, connected, activeAlerts, initialDataReceived, reconnecting, reconnect } = useWebSocket();
  const alertsActivation = useAlertsActivation();
  const alertsEnabled = createMemo(() => alertsActivation.activationState() === 'active');
  const [viewMode, setViewMode] = usePersistentSignal<'node' | 'storage'>(
    'storageViewMode',
    'node',
    {
      deserialize: (raw) => (raw === 'storage' ? 'storage' : 'node'),
    },
  );
  const [tabView, setTabView] = createSignal<'pools' | 'disks'>('pools');
  const [searchTerm, setSearchTerm] = createSignal('');
  const [selectedNode, setSelectedNode] = createSignal<string | null>(null);
  const [expandedStorage, setExpandedStorage] = createSignal<string | null>(null);
  const [sortKey, setSortKey] = usePersistentSignal<StorageSortKey>('storageSortKey', 'name', {
    deserialize: (raw) =>
      (['name', 'node', 'type', 'status', 'usage', 'free', 'total'] as const).includes(
        raw as StorageSortKey,
      )
        ? (raw as StorageSortKey)
        : 'name',
  });
  const [sortDirection, setSortDirection] = usePersistentSignal<'asc' | 'desc'>(
    'storageSortDirection',
    'asc',
    {
      deserialize: (raw) => (raw === 'desc' ? 'desc' : 'asc'),
    },
  );
  const [statusFilter, setStatusFilter] = usePersistentSignal<'all' | 'available' | 'offline'>(
    'storageStatusFilter',
    'all',
    {
      deserialize: (raw) =>
        raw === 'all' || raw === 'available' || raw === 'offline' ? raw : 'all',
    },
  );

  // PERFORMANCE: Debounce search term to prevent jank during rapid typing
  const debouncedSearchTerm = useDebouncedValue(() => searchTerm(), 200);

  // Create a mapping from node instance ID to node object
  const nodeByInstance = createMemo(() => {
    const map: Record<string, typeof state.nodes[0]> = {};
    (state.nodes || []).forEach((node) => {
      map[node.id] = node;
    });
    return map;
  });

  const isCephType = (type?: string) => {
    const value = (type || '').toLowerCase();
    return value === 'rbd' || value === 'cephfs' || value === 'ceph';
  };

  const getCephHealthLabel = (health?: string) => {
    if (!health) return 'CEPH';
    const normalized = health.toUpperCase();
    return normalized.startsWith('HEALTH_') ? normalized.replace('HEALTH_', '') : normalized;
  };

  const getCephHealthStyles = (health?: string) => {
    const normalized = (health || '').toUpperCase();
    if (normalized === 'HEALTH_OK') {
      return 'bg-green-100 text-green-700 dark:bg-green-900/60 dark:text-green-300 border border-green-200 dark:border-green-800';
    }
    if (normalized === 'HEALTH_WARN' || normalized === 'HEALTH_WARNING') {
      return 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/60 dark:text-yellow-200 border border-yellow-300 dark:border-yellow-800';
    }
    if (normalized === 'HEALTH_ERR' || normalized === 'HEALTH_ERROR' || normalized === 'HEALTH_CRIT') {
      return 'bg-red-100 text-red-700 dark:bg-red-900/60 dark:text-red-200 border border-red-300 dark:border-red-800';
    }
    return 'bg-blue-100 text-blue-700 dark:bg-blue-900/50 dark:text-blue-200 border border-blue-200 dark:border-blue-700';
  };

  const cephClusters = createMemo(() => state.cephClusters || []);

  const visibleCephClusters = createMemo<CephCluster[]>(() => {
    const explicit = cephClusters();
    if (explicit && explicit.length > 0) {
      return explicit;
    }

    const storageList = state.storage || [];
    const summaryByInstance = new Map<
      string,
      {
        total: number;
        used: number;
        nodes: Set<string>;
        storages: number;
      }
    >();

    storageList.forEach((item) => {
      if (!isCephType(item?.type)) {
        return;
      }
      const instance = item.instance || 'ceph';
      let summary = summaryByInstance.get(instance);
      if (!summary) {
        summary = {
          total: 0,
          used: 0,
          nodes: new Set<string>(),
          storages: 0,
        };
        summaryByInstance.set(instance, summary);
      }

      summary.total += Math.max(0, item.total || 0);
      summary.used += Math.max(0, item.used || 0);
      summary.storages += 1;

      if (Array.isArray(item.nodes)) {
        item.nodes.forEach((node) => node && summary!.nodes.add(node));
      } else if (item.node) {
        summary.nodes.add(item.node);
      }
    });

    return Array.from(summaryByInstance.entries()).map(([instance, info], index) => {
      const totalBytes = info.total;
      const usedBytes = info.used;
      const availableBytes = Math.max(totalBytes - usedBytes, 0);
      const usagePercent = totalBytes > 0 ? (usedBytes / totalBytes) * 100 : 0;
      const numOsds = Math.max(1, info.storages * 2);
      const numMons = Math.min(3, Math.max(1, info.nodes.size));

      return {
        id: `${instance}-derived-${index}`,
        instance,
        name: `${instance} Ceph`,
        health: 'HEALTH_UNKNOWN',
        healthMessage: 'Derived from storage metrics – live Ceph telemetry unavailable.',
        totalBytes,
        usedBytes,
        availableBytes,
        usagePercent,
        numMons,
        numMgrs: numMons > 1 ? 2 : 1,
        numOsds,
        numOsdsUp: numOsds,
        numOsdsIn: numOsds,
        numPGs: Math.max(128, info.storages * 128),
        pools: undefined,
        services: undefined,
        lastUpdated: Date.now(),
      } as CephCluster;
    });
  });

  const cephClusterByInstance = createMemo<Record<string, CephCluster>>(() => {
    const map: Record<string, CephCluster> = {};
    visibleCephClusters().forEach((cluster) => {
      if (cluster?.instance) {
        map[cluster.instance] = cluster;
      }
    });
    return map;
  });


  const sortKeyOptions: { value: StorageSortKey; label: string }[] = [
    { value: 'name', label: 'Name' },
    { value: 'node', label: 'Node' },
    { value: 'type', label: 'Type' },
    { value: 'status', label: 'Status' },
    { value: 'usage', label: 'Usage %' },
    { value: 'free', label: 'Free Capacity' },
    { value: 'total', label: 'Total Capacity' },
  ];

  // Filter storage - in storage view, filter out 0 capacity and deduplicate
  const filteredStorage = createMemo(() => {
    let storage = state.storage || [];

    // In storage view, deduplicate identical storage and filter out 0 capacity
    if (viewMode() === 'storage') {
      // Filter out 0 capacity first
      storage = storage.filter((s) => s.total > 0);

      // Deduplicate storage entries that are identical
      const storageMap = new Map();
      storage.forEach((s) => {
        let key;
        const nodeId = `${s.instance}-${s.node}`;

        // For PBS storage, group by capacity since they're the same PBS server
        // PBS namespaces (pbs-node1, pbs-node2) pointing to same server should be grouped
        if (s.type === 'pbs' && s.shared) {
          // Group shared PBS by type, total size, and usage (same PBS server = same capacity)
          key = `pbs-${s.total}-${s.used}`;
        } else if (s.shared) {
          // Other shared storage should only appear once
          key = `${s.name}-${s.type}`;
        } else {
          // Non-shared storage (local, local-zfs) - NEVER deduplicate these!
          // Each node has its own physical storage
          key = `${s.node}-${s.name}-${s.type}`;
        }

        if (!storageMap.has(key)) {
          const backendNodes = (s.nodes ?? []).filter((node): node is string => Boolean(node));
          const backendNodeIds = ((s as { nodeIds?: string[] }).nodeIds ?? []).filter(
            (id): id is string => Boolean(id),
          );

          const rawNodes = backendNodes.length > 0 ? backendNodes : [s.node].filter(Boolean);
          const uniqueNodes = Array.from(new Set(rawNodes));
          const normalizedInitialNodes =
            uniqueNodes.length > 1 ? uniqueNodes.filter((node) => node !== 'cluster') : uniqueNodes;
          const nodesForStorage =
            normalizedInitialNodes.length > 0 ? normalizedInitialNodes : uniqueNodes;
          const initialNodeIds = Array.from(
            new Set(backendNodeIds.length > 0 ? backendNodeIds : [nodeId].filter(Boolean)),
          );

          const initialPBSNames =
            s.type === 'pbs'
              ? Array.from(new Set((s.pbsNames ?? [s.name]).filter((name): name is string => Boolean(name))))
              : undefined;

          // First occurrence - store it with node list
          storageMap.set(key, {
            ...s,
            name: s.type === 'pbs' ? 'PBS Storage' : s.name, // Generic name for PBS
            nodes: nodesForStorage,
            nodeIds: initialNodeIds,
            nodeCount: nodesForStorage.length,
            pbsNames: initialPBSNames, // Track individual PBS names
          });
        } else {
          // Duplicate - just add to node list
          const existing = storageMap.get(key);

          const backendNodes = (s.nodes ?? []).filter((node): node is string => Boolean(node));
          const rawCandidateNodes =
            backendNodes.length > 0 ? backendNodes : [s.node].filter(Boolean);
          const candidateNodes =
            rawCandidateNodes.length > 1
              ? rawCandidateNodes.filter((node) => node !== 'cluster')
              : rawCandidateNodes;
          candidateNodes.forEach((node) => {
            if (!existing.nodes.includes(node)) {
              existing.nodes.push(node);
            }
          });
          existing.nodeCount = existing.nodes.length;

          const backendNodeIds = ((s as { nodeIds?: string[] }).nodeIds ?? []).filter(
            (id): id is string => Boolean(id),
          );
          const candidateNodeIds =
            backendNodeIds.length > 0 ? backendNodeIds : [nodeId].filter(Boolean);

          if (!existing.nodeIds) {
            existing.nodeIds = [];
          }
          candidateNodeIds.forEach((id) => {
            if (!existing.nodeIds!.includes(id)) {
              existing.nodeIds!.push(id);
            }
          });

          // For PBS, collect all namespace names (merging backend-provided data)
          if (s.type === 'pbs') {
            if (!existing.pbsNames) {
              existing.pbsNames = [];
            }
            const incomingPBSNames = (s.pbsNames ?? [s.name]).filter(
              (name): name is string => Boolean(name),
            );
            incomingPBSNames.forEach((name) => {
              if (!existing.pbsNames!.includes(name)) {
                existing.pbsNames!.push(name);
              }
            });
          }
        }
      });

      // Convert back to array
      storage = Array.from(storageMap.values());
    }

    return storage;
  });

  // Sort and filter storage
  const sortedStorage = createMemo(() => {
    let storage = [...filteredStorage()];

    // Apply node selection filter with instance-aware matching
    const nodeFilter = selectedNode();
    if (nodeFilter) {
      const node = state.nodes?.find((n) => n.id === nodeFilter);
      if (node) {
        const nodeId = `${node.instance}-${node.name}`;
        storage = storage.filter((s) => {
          const belongsToNode = s.instance === node.instance && s.node === node.name;
          const aggregatedNodeIds = (s as { nodeIds?: string[] }).nodeIds ?? [];
          return belongsToNode || aggregatedNodeIds.includes(nodeId);
        });
      }
    }

    // Apply status filter
    if (statusFilter() !== 'all') {
      storage = storage.filter((s) => {
        if (statusFilter() === 'available') {
          return s.status === 'available';
        } else {
          // Offline includes anything not available
          return s.status !== 'available';
        }
      });
    }

    // Apply search filter - PERFORMANCE: Use debounced search term
    let search = debouncedSearchTerm().toLowerCase().trim();
    if (search) {
      const nodePattern = /node:([a-z0-9_.:-]+)/i;
      const nodeMatch = search.match(nodePattern);
      let nodeQuery: string | null = null;

      if (nodeMatch) {
        nodeQuery = nodeMatch[1].toLowerCase();
        search = search.replace(nodeMatch[0], '').trim();
      }

      if (nodeQuery) {
        storage = storage.filter((s) => {
          const primary = s.node?.toLowerCase();
          const extraNodes = s.nodes?.map((node) => node.toLowerCase()) || [];
          return primary === nodeQuery || extraNodes.includes(nodeQuery);
        });
      }

      if (search) {
        const terms = search.split(/\s+/).filter(Boolean);
        storage = storage.filter((s) => {
          const haystack = [
            s.name,
            s.node,
            s.type,
            s.content,
            s.status,
            ...(s.nodes ?? []),
            ...(s.pbsNames ?? []),
          ]
            .filter(Boolean)
            .map((value) => value!.toLowerCase());

          return terms.every((term) => haystack.some((entry) => entry.includes(term)));
        });
      }
    }

    const numericCompare = (a: number, b: number) => {
      const normalizedA = Number.isFinite(a) ? a : -Infinity;
      const normalizedB = Number.isFinite(b) ? b : -Infinity;
      if (normalizedA === normalizedB) return 0;
      return normalizedA < normalizedB ? -1 : 1;
    };

    const result = storage.sort((a, b) => {
      let comparison = 0;

      switch (sortKey()) {
        case 'node': {
          const nodeA = (a.nodes && a.nodes.length > 0 ? a.nodes[0] : a.node) ?? '';
          const nodeB = (b.nodes && b.nodes.length > 0 ? b.nodes[0] : b.node) ?? '';
          comparison = nodeA.localeCompare(nodeB, undefined, { sensitivity: 'base' });
          break;
        }
        case 'type':
          comparison = (a.type ?? '').localeCompare(b.type ?? '', undefined, {
            sensitivity: 'base',
          });
          break;
        case 'status':
          comparison = (a.status ?? '').localeCompare(b.status ?? '', undefined, {
            sensitivity: 'base',
          });
          break;
        case 'usage':
          comparison = numericCompare(a.usage ?? 0, b.usage ?? 0);
          break;
        case 'free':
          comparison = numericCompare(a.free ?? 0, b.free ?? 0);
          break;
        case 'total':
          comparison = numericCompare(a.total ?? 0, b.total ?? 0);
          break;
        case 'name':
        default:
          comparison = (a.name ?? '').localeCompare(b.name ?? '', undefined, {
            sensitivity: 'base',
          });
          break;
      }

      if (comparison === 0) {
        comparison = (a.name ?? '').localeCompare(b.name ?? '', undefined, { sensitivity: 'base' });
      }

      return sortDirection() === 'asc' ? comparison : -comparison;
    });

    return result;
  });

  // Group storage by node or storage
  const groupedStorage = createMemo(() => {
    const storage = sortedStorage();
    const mode = viewMode();

    if (mode === 'node') {
      // Group by node ID (instance + node name) to match Node.ID format
      const groups: Record<string, StorageType[]> = {};
      storage.forEach((s) => {
        if (s.shared) {
          const nodeCandidates = [
            ...((s.nodeIds ?? [])
              .map((id) => (id.startsWith(`${s.instance}-`) ? id.slice(s.instance.length + 1) : id))
              .filter((node): node is string => Boolean(node))),
            ...((s.nodes ?? []).filter((node): node is string => Boolean(node))),
          ];
          const normalizedNodes = Array.from(
            new Set(
              nodeCandidates
                .map((node) => node.trim())
                .filter((node) => node && node !== 'shared' && node !== 'cluster'),
            ),
          );
          const nodesForStorage =
            normalizedNodes.length > 0
              ? normalizedNodes
              : [s.node].map((node) => node?.trim()).filter((node): node is string => Boolean(node));

          nodesForStorage.forEach((nodeName) => {
            const key = `${s.instance}-${nodeName}`;
            if (!groups[key]) groups[key] = [];
            groups[key].push({ ...s, node: nodeName });
          });
          return;
        }

        const key = `${s.instance}-${s.node}`;
        if (!groups[key]) groups[key] = [];
        groups[key].push(s);
      });
      return groups;
    } else {
      // Group by storage name - show all storage as-is for maximum compatibility
      const groups: Record<string, StorageType[]> = {};

      storage.forEach((s) => {
        if (!groups[s.name]) groups[s.name] = [];
        groups[s.name].push(s);
      });

      return groups;
    }
  });



  const resetFilters = () => {
    setSearchTerm('');
    setSelectedNode(null);
    setViewMode('node');
    setSortKey('name');
    setSortDirection('asc');
    setStatusFilter('all');
  };

  // Handle keyboard shortcuts
  let searchInputRef: HTMLInputElement | undefined;

  createEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      // Ignore if user is typing in an input
      const target = e.target as HTMLElement;
      const isInputField =
        target.tagName === 'INPUT' ||
        target.tagName === 'TEXTAREA' ||
        target.tagName === 'SELECT' ||
        target.contentEditable === 'true';

      // Escape key behavior
      if (e.key === 'Escape') {
        // Clear search and reset filters
        if (
          searchTerm().trim() ||
          selectedNode() ||
          viewMode() !== 'node' ||
          statusFilter() !== 'all'
        ) {
          resetFilters();

          // Blur the search input if it's focused
          if (searchInputRef && document.activeElement === searchInputRef) {
            searchInputRef.blur();
          }
        }
      } else if (!isInputField && e.key.length === 1 && !e.ctrlKey && !e.metaKey && !e.altKey) {
        // If it's a printable character and user is not in an input field
        // Focus the search input and let the character be typed
        if (searchInputRef) {
          searchInputRef.focus();
        }
      }
    };

    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  });

  const handleNodeSelect = (nodeId: string | null) => {
    setSelectedNode(nodeId);
  };

  return (
    <div class="space-y-3">
      <ProxmoxSectionNav current="storage" />

      {/* Node Selector */}
      <UnifiedNodeSelector
        currentTab="storage"
        globalTemperatureMonitoringEnabled={state.temperatureMonitoringEnabled}
        onNodeSelect={handleNodeSelect}
        filteredStorage={sortedStorage()}
        searchTerm={searchTerm()}
      />

      {/* Tab Toggle */}
      <div class="mb-4">
        <nav class="flex items-center gap-4" aria-label="Storage tabs">
          <button
            onClick={() => setTabView('pools')}
            type="button"
            class={`inline-flex items-center px-2 sm:px-3 py-1 text-sm font-medium border-b-2 border-transparent text-gray-600 dark:text-gray-400 transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-400/60 focus-visible:ring-offset-2 focus-visible:ring-offset-white dark:focus-visible:ring-offset-gray-900 ${tabView() === 'pools'
              ? 'text-blue-600 dark:text-blue-300 border-blue-500 dark:border-blue-400'
              : 'hover:text-blue-500 dark:hover:text-blue-300 hover:border-blue-300/70 dark:hover:border-blue-500/50'
              }`}
          >
            Storage Pools
          </button>
          <button
            onClick={() => setTabView('disks')}
            type="button"
            class={`inline-flex items-center px-2 sm:px-3 py-1 text-sm font-medium border-b-2 border-transparent text-gray-600 dark:text-gray-400 transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-400/60 focus-visible:ring-offset-2 focus-visible:ring-offset-white dark:focus-visible:ring-offset-gray-900 ${tabView() === 'disks'
              ? 'text-blue-600 dark:text-blue-300 border-blue-500 dark:border-blue-400'
              : 'hover:text-blue-500 dark:hover:text-blue-300 hover:border-blue-300/70 dark:hover:border-blue-500/50'
              }`}
          >
            Physical Disks
          </button>
        </nav>
      </div>

      {/* Show Storage Filter only for pools */}
      <Show when={tabView() === 'pools'}>
        <StorageFilter
          search={searchTerm}
          setSearch={setSearchTerm}
          groupBy={viewMode}
          setGroupBy={setViewMode}
          sortOptions={sortKeyOptions}
          sortKey={sortKey}
          setSortKey={setSortKey}
          sortDirection={sortDirection}
          setSortDirection={setSortDirection}
          statusFilter={statusFilter}
          setStatusFilter={setStatusFilter}
          searchInputRef={(el) => (searchInputRef = el)}
        />
      </Show>

      {/* Show simple search for disks */}
      <Show when={tabView() === 'disks'}>
        <Card class="mb-3" padding="sm">
          <div class="relative">
            <input
              type="text"
              placeholder="Search disks by model, path, or serial..."
              value={searchTerm()}
              onInput={(e) => setSearchTerm(e.currentTarget.value)}
              ref={(el) => (searchInputRef = el)}
              class="w-full pl-9 pr-3 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded-lg 
                     bg-white dark:bg-gray-900 text-gray-800 dark:text-gray-200 placeholder-gray-400 dark:placeholder-gray-500
                     focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500 dark:focus:border-blue-400 outline-none transition-all"
            />
            <svg
              class="absolute left-3 top-2 h-4 w-4 text-gray-400 dark:text-gray-500"
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
          </div>
        </Card>
      </Show>

      {/* Loading State */}
      <Show when={connected() && !initialDataReceived()}>
        <Card padding="lg">
          <EmptyState
            icon={
              <div class="mx-auto flex h-12 w-12 items-center justify-center">
                <svg class="h-8 w-8 animate-spin text-gray-400" fill="none" viewBox="0 0 24 24">
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
              </div>
            }
            title="Loading storage data..."
            description="Connecting to monitoring service"
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

      {/* Helpful hint for no PVE nodes but still show content */}
      <Show
        when={
          connected() &&
          initialDataReceived() &&
          (state.nodes || []).filter((n) => n.type === 'pve').length === 0 &&
          sortedStorage().length === 0 &&
          searchTerm().trim() === ''
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
            title="No storage configured"
            description="Add a Proxmox VE or PBS node in the Settings tab to start monitoring storage."
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

      {/* Conditional rendering based on tab */}
      <Show when={tabView() === 'pools'}>
        {/* No results found message for storage pools */}
        <Show
          when={
            connected() &&
            initialDataReceived() &&
            sortedStorage().length === 0 &&
            searchTerm().trim() !== ''
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
                    d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
                  />
                </svg>
              }
              title="No storage found"
              description={`No storage matches your search "${searchTerm()}"`}
            />
          </Card>
        </Show>

        {/* Storage Table - shows for both PVE and PBS storage */}
        <Show when={connected() && initialDataReceived() && sortedStorage().length > 0}>
          <ComponentErrorBoundary name="Storage Table">
            <Card padding="none" tone="glass" class="mb-4 overflow-hidden">
              <div class="overflow-x-auto" style="scrollbar-width: none; -ms-overflow-style: none;">
                <style>{`
                .overflow-x-auto::-webkit-scrollbar { display: none; }
              `}</style>
                <table class="w-full" style={{ "min-width": "900px" }}>
                  <thead>
                    <tr class="bg-gray-50 dark:bg-gray-700/50 text-gray-600 dark:text-gray-300 border-b border-gray-200 dark:border-gray-600">
                      <th class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider w-auto">
                        Storage
                      </th>
                      <th class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider w-[10%]">
                        Type
                      </th>
                      <th class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider w-[15%]">
                        Content
                      </th>
                      <th class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider w-[10%]">
                        Status
                      </th>
                      <Show when={viewMode() === 'node'}>
                        <th class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider w-[8%]">
                          Shared
                        </th>
                      </Show>
                      <th class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider w-[25%] min-w-[150px]">
                        Usage
                      </th>
                      <th class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider w-[10%]">
                        Free
                      </th>
                      <th class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider w-[10%]">
                        Total
                      </th>
                      <th class="px-2 py-1.5 w-8"></th>
                    </tr>
                  </thead>
                  <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
                    <For
                      each={Object.entries(groupedStorage()).sort(([instanceIdA], [instanceIdB]) => {
                        // Sort by friendly node name for display when in node mode
                        if (viewMode() === 'node') {
                          const nodeA = nodeByInstance()[instanceIdA];
                          const nodeB = nodeByInstance()[instanceIdB];
                          const labelA = nodeA ? getNodeDisplayName(nodeA) : instanceIdA;
                          const labelB = nodeB ? getNodeDisplayName(nodeB) : instanceIdB;
                          return labelA.localeCompare(labelB);
                        }
                        // Sort by storage name when in storage mode
                        return instanceIdA.localeCompare(instanceIdB);
                      })}
                    >
                      {([groupKey, storages]) => {
                        const node = viewMode() === 'node' ? nodeByInstance()[groupKey] : null;
                        return (
                          <>
                            {/* Group Header */}
                            <Show when={viewMode() === 'node' && node}>
                              {(validNode) => <NodeGroupHeader node={validNode()} colspan={9} renderAs="tr" />}
                            </Show>

                            {/* Storage Rows */}
                            <For each={storages} fallback={<></>}>
                              {(storage) => {
                                const isDisabled = storage.status !== 'available';
                                const pbsNamesDisplay = createMemo(() => {
                                  const names = storage.pbsNames?.filter(
                                    (name): name is string => Boolean(name),
                                  );
                                  if (!names || names.length === 0) return '';
                                  return [...names].sort((a, b) => a.localeCompare(b)).join(', ');
                                });

                                const nodeListDisplay = createMemo(() => {
                                  const nodes =
                                    storage.nodes && storage.nodes.length > 0
                                      ? storage.nodes.filter((node): node is string => Boolean(node))
                                      : [storage.node].filter((node): node is string => Boolean(node));
                                  return nodes.join(', ');
                                });

                                const alertStyles = createMemo(() =>
                                  getAlertStyles(
                                    storage.id || `${storage.instance}-${storage.node}-${storage.name}`,
                                    activeAlerts,
                                    alertsEnabled(),
                                  ),
                                );

                                const parentNodeOnline = createMemo(() => {
                                  if (viewMode() === 'node' && node) {
                                    return node.status === 'online' && (node.uptime || 0) > 0;
                                  }
                                  return true;
                                });

                                const showAlertHighlight = createMemo(
                                  () => alertStyles().hasUnacknowledgedAlert && parentNodeOnline(),
                                );

                                const isCephStorage = createMemo(() => isCephType(storage.type));
                                const canExpand = createMemo(() => isCephStorage());
                                const zfsPool = storage.zfsPool;
                                const storageRowId = createMemo(
                                  () => storage.id || `${storage.instance}-${storage.node}-${storage.name}`,
                                );
                                const cephCluster = createMemo(
                                  () => cephClusterByInstance()[storage.instance],
                                );
                                const cephInstanceStorages = createMemo(() =>
                                  (state.storage || []).filter(
                                    (item) =>
                                      item.instance === storage.instance && isCephType(item.type),
                                  ),
                                );
                                const cephHealthLabel = createMemo(() => {
                                  const health = (cephCluster()?.health || '').toUpperCase();
                                  if (!health || health === 'HEALTH_UNKNOWN') {
                                    return '';
                                  }
                                  return getCephHealthLabel(health);
                                });
                                const cephHealthClass = createMemo(() => {
                                  const health = (cephCluster()?.health || '').toUpperCase();
                                  if (!health || health === 'HEALTH_UNKNOWN') {
                                    return '';
                                  }
                                  return getCephHealthStyles(health);
                                });
                                const drawerDisabled = createMemo(
                                  () => isDisabled || !parentNodeOnline(),
                                );
                                const cephSummaryText = createMemo(() => {
                                  const cluster = cephCluster();
                                  const parts: string[] = [];

                                  if (cluster && Number.isFinite(cluster.totalBytes)) {
                                    const total = Math.max(0, cluster.totalBytes || 0);
                                    const used = Math.max(0, cluster.usedBytes || 0);
                                    const percent = total > 0 ? (used / total) * 100 : 0;
                                    parts.push(
                                      `${formatBytes(used, 0)} / ${formatBytes(total, 0)} (${formatPercent(percent)})`,
                                    );
                                    if (
                                      Number.isFinite(cluster.numOsds) &&
                                      Number.isFinite(cluster.numOsdsUp)
                                    ) {
                                      parts.push(`OSDs ${cluster.numOsdsUp}/${cluster.numOsds}`);
                                    }
                                    if (Number.isFinite(cluster.numPGs) && cluster.numPGs > 0) {
                                      parts.push(`PGs ${cluster.numPGs.toLocaleString()}`);
                                    }
                                  } else {
                                    const storages = cephInstanceStorages();
                                    if (storages.length > 0) {
                                      const totals = storages.reduce(
                                        (acc, item) => {
                                          acc.total += Math.max(0, item.total || 0);
                                          acc.used += Math.max(0, item.used || 0);
                                          return acc;
                                        },
                                        { total: 0, used: 0 },
                                      );
                                      if (totals.total > 0) {
                                        const percent = (totals.used / totals.total) * 100;
                                        parts.push(
                                          `${formatBytes(totals.used, 0)} / ${formatBytes(totals.total, 0)} (${formatPercent(percent)})`,
                                        );
                                      }
                                    }
                                  }

                                  return parts.join(' • ');
                                });
                                const cephPoolsText = createMemo(() => {
                                  const cluster = cephCluster();
                                  if (cluster && cluster.pools && cluster.pools.length > 0) {
                                    return cluster.pools
                                      .slice(0, 2)
                                      .map((pool) => {
                                        if (!pool) return '';
                                        const total = Math.max(1, pool.storedBytes + pool.availableBytes);
                                        const percent = total > 0 ? (pool.storedBytes / total) * 100 : 0;
                                        return `${pool.name}: ${formatPercent(percent)}`;
                                      })
                                      .filter(Boolean)
                                      .join(', ');
                                  }

                                  const storages = cephInstanceStorages();
                                  if (storages.length === 0) {
                                    return '';
                                  }

                                  return storages
                                    .slice(0, 2)
                                    .map((item) => {
                                      const total = Math.max(1, item.total || 0);
                                      const used = Math.max(0, item.used || 0);
                                      const percent = total > 0 ? (used / total) * 100 : 0;
                                      return `${item.name}: ${formatPercent(percent)}`;
                                    })
                                    .filter(Boolean)
                                    .join(', ');
                                });
                                const cephHealthMessage = createMemo(() => {
                                  const cluster = cephCluster();
                                  if (cluster?.healthMessage) {
                                    return cluster.healthMessage;
                                  }
                                  if (cluster && cluster.health && cluster.health !== 'HEALTH_UNKNOWN') {
                                    return '';
                                  }
                                  if (cephInstanceStorages().length > 0) {
                                    return 'Derived from storage metrics – live Ceph telemetry unavailable.';
                                  }
                                  return '';
                                });
                                const isExpanded = createMemo(
                                  () => expandedStorage() === storageRowId(),
                                );

                                const hasAcknowledgedOnlyAlert = createMemo(
                                  () => alertStyles().hasAcknowledgedOnlyAlert && parentNodeOnline(),
                                );

                                const rowClass = createMemo(() => {
                                  const classes = [
                                    'transition-all duration-200',
                                    'hover:bg-gray-50 dark:hover:bg-gray-700/30',
                                    'hover:shadow-sm',
                                  ];

                                  if (showAlertHighlight()) {
                                    classes.push(
                                      alertStyles().severity === 'critical'
                                        ? 'bg-red-50 dark:bg-red-950/30'
                                        : 'bg-yellow-50 dark:bg-yellow-950/20',
                                    );
                                  } else if (hasAcknowledgedOnlyAlert()) {
                                    classes.push('bg-gray-50/40 dark:bg-gray-800/40');
                                  }

                                  if (isDisabled || !parentNodeOnline()) {
                                    classes.push('opacity-60');
                                  }

                                  if (canExpand()) {
                                    classes.push('cursor-pointer');
                                  }

                                  if (canExpand() && isExpanded()) {
                                    classes.push('bg-gray-50 dark:bg-gray-800/40');
                                  }

                                  return classes.join(' ');
                                });

                                const rowStyle = createMemo(() => {
                                  if (showAlertHighlight()) {
                                    const color =
                                      alertStyles().severity === 'critical' ? '#ef4444' : '#eab308';
                                    return {
                                      'box-shadow': `inset 4px 0 0 0 ${color}`,
                                    };
                                  }
                                  if (hasAcknowledgedOnlyAlert()) {
                                    return {
                                      'box-shadow': 'inset 4px 0 0 0 rgba(156, 163, 175, 0.8)',
                                    };
                                  }
                                  return {} as Record<string, string>;
                                });

                                const firstCellHasIndicator = createMemo(
                                  () => showAlertHighlight() || hasAcknowledgedOnlyAlert(),
                                );

                                const firstCellClass = createMemo(() => {
                                  if (viewMode() === 'node') {
                                    return firstCellHasIndicator()
                                      ? 'p-0.5 pl-7 pr-1.5'
                                      : 'p-0.5 pl-8 pr-1.5';
                                  }
                                  return firstCellHasIndicator()
                                    ? 'p-0.5 pl-3 pr-1.5'
                                    : 'p-0.5 pl-3 pr-1.5';
                                });

                                const toggleDrawer = () => {
                                  if (!canExpand()) return;
                                  setExpandedStorage((prev) =>
                                    prev === storageRowId() ? null : storageRowId(),
                                  );
                                };

                                return (
                                  <>
                                    <tr
                                      class={`${rowClass()} transition-colors`}
                                      style={rowStyle()}
                                      onClick={toggleDrawer}
                                      aria-expanded={canExpand() && isExpanded() ? 'true' : 'false'}
                                    >
                                      <td class={`${firstCellClass()} align-middle`}>
                                        <div class="flex items-center gap-2 min-w-0">
                                          <span
                                            class={`text-sm font-medium text-gray-900 dark:text-gray-100 truncate ${canExpand() ? 'max-w-[180px]' : 'max-w-[200px]'
                                              }`}
                                            title={storage.name}
                                          >
                                            {storage.name}
                                          </span>
                                          {/* ZFS Health Map */}
                                          <Show when={zfsPool && zfsPool.devices && zfsPool.devices.length > 0}>
                                            <div class="mx-1.5">
                                              <ZFSHealthMap pool={zfsPool!} />
                                            </div>
                                          </Show>
                                          {/* ZFS Health Badge */}
                                          <Show when={zfsPool && zfsPool.state !== 'ONLINE'}>
                                            <span
                                              class={`px-1.5 py-0.5 rounded text-[10px] font-medium ${zfsPool?.state === 'DEGRADED'
                                                ? 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900 dark:text-yellow-300'
                                                : 'bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300'
                                                }`}
                                            >
                                              {zfsPool?.state}
                                            </span>
                                          </Show>
                                          {/* ZFS Error Badge */}
                                          <Show
                                            when={
                                              zfsPool &&
                                              (zfsPool.readErrors > 0 ||
                                                zfsPool.writeErrors > 0 ||
                                                zfsPool.checksumErrors > 0)
                                            }
                                          >
                                            <span class="px-1.5 py-0.5 rounded text-[10px] font-medium bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300">
                                              ERRORS
                                            </span>
                                          </Show>
                                          <Show when={viewMode() === 'storage'}>
                                            <Show when={storage.pbsNames}>
                                              <span
                                                class="text-xs text-gray-500 dark:text-gray-400 whitespace-nowrap truncate max-w-[240px]"
                                                title={pbsNamesDisplay()}
                                              >
                                                ({pbsNamesDisplay()})
                                              </span>
                                            </Show>
                                            <Show when={!storage.pbsNames}>
                                              <span
                                                class="text-xs text-gray-500 dark:text-gray-400 whitespace-nowrap truncate max-w-[240px]"
                                                title={nodeListDisplay()}
                                              >
                                                ({nodeListDisplay()})
                                              </span>
                                            </Show>
                                          </Show>
                                          <Show when={isCephStorage() && cephHealthLabel()}>
                                            <span
                                              class={`px-1.5 py-0.5 rounded text-[10px] font-medium ${cephHealthClass()}`}
                                              title={cephCluster()?.healthMessage}
                                            >
                                              {cephHealthLabel()}
                                            </span>
                                          </Show>
                                        </div>
                                      </td>
                                      <td class="p-0.5 px-1.5">
                                        <span class="inline-block px-1.5 py-0.5 text-[10px] font-medium rounded bg-gray-100 text-gray-700 dark:bg-gray-700 dark:text-gray-300">
                                          {storage.type}
                                        </span>
                                      </td>
                                      <td class="p-0.5 px-1.5">
                                        <span
                                          class="text-xs text-gray-600 dark:text-gray-400 whitespace-nowrap truncate max-w-[220px]"
                                          title={storage.content || '-'}
                                        >
                                          {storage.content || '-'}
                                        </span>
                                      </td>
                                      <td class="p-0.5 px-1.5 text-xs whitespace-nowrap">
                                        <span
                                          class={`${storage.status === 'available'
                                            ? 'text-green-600 dark:text-green-400'
                                            : 'text-red-600 dark:text-red-400'
                                            }`}
                                        >
                                          {storage.status || 'unknown'}
                                        </span>
                                      </td>
                                      <Show when={viewMode() === 'node'}>
                                        <td class="p-0.5 px-1.5">
                                          <span class="text-xs text-gray-600 dark:text-gray-400">
                                            {storage.shared ? '✓' : '-'}
                                          </span>
                                        </td>
                                      </Show>

                                      <td class="p-0.5 px-1.5">
                                        <EnhancedStorageBar
                                          used={storage.used || 0}
                                          total={storage.total || 0}
                                          free={storage.free || 0}
                                          zfsPool={storage.zfsPool}
                                        />
                                      </td>
                                      <td class="p-0.5 px-1.5 text-xs whitespace-nowrap">
                                        {formatBytes(storage.free || 0, 0)}
                                      </td>
                                      <td class="p-0.5 px-1.5 text-xs whitespace-nowrap">
                                        {formatBytes(storage.total || 0, 0)}
                                      </td>
                                      <td class="p-0.5 px-1.5"></td>
                                    </tr>
                                    <Show when={isCephStorage() && isExpanded()}>
                                      <tr
                                        class={`text-[11px] border-t border-gray-200 dark:border-gray-700 ${drawerDisabled()
                                          ? 'bg-gray-100/70 text-gray-400 dark:bg-gray-900/30 dark:text-gray-500'
                                          : 'bg-gray-50/60 text-gray-700 dark:bg-gray-900/30 dark:text-gray-300'
                                          }`}
                                      >
                                        <td colSpan={9} class="px-4 py-3">
                                          <div
                                            class={`grid gap-3 md:grid-cols-2 xl:grid-cols-2 ${drawerDisabled() ? 'opacity-60 pointer-events-none' : ''
                                              }`}
                                          >
                                            <div class="rounded-lg border border-gray-200 bg-white/80 p-4 shadow-sm dark:border-gray-600/60 dark:bg-gray-900/30">
                                              <div class="flex items-center gap-2 text-xs font-semibold text-gray-700 dark:text-gray-200">
                                                <span>Ceph Cluster</span>
                                                <span>{cephCluster()?.name || storage.instance}</span>
                                                <Show when={cephHealthLabel()}>
                                                  <span class={`px-1.5 py-0.5 rounded text-[10px] font-medium ${cephHealthClass()}`}>
                                                    {cephHealthLabel()}
                                                  </span>
                                                </Show>
                                              </div>
                                              <Show when={cephSummaryText()}>
                                                <div class="mt-2 text-[12px] text-gray-600 dark:text-gray-300">
                                                  {cephSummaryText()}
                                                </div>
                                              </Show>
                                              <Show when={cephHealthMessage()}>
                                                <div class="mt-2 text-[11px] text-gray-500 dark:text-gray-400">
                                                  {cephHealthMessage()}
                                                </div>
                                              </Show>
                                            </div>
                                            <div class="rounded-lg border border-gray-200 bg-white/80 p-4 shadow-sm dark:border-gray-600/60 dark:bg-gray-900/30 text-gray-600 dark:text-gray-300">
                                              <div class="text-xs font-semibold text-gray-700 dark:text-gray-200">
                                                Pools
                                              </div>
                                              <Show
                                                when={cephPoolsText()}
                                                fallback={
                                                  <div class="mt-2 text-[12px] text-gray-500 dark:text-gray-400">
                                                    No pool data available
                                                  </div>
                                                }
                                              >
                                                <div class="mt-2 text-[12px]">{cephPoolsText()}</div>
                                              </Show>
                                            </div>
                                          </div>
                                        </td>
                                      </tr>
                                    </Show>
                                    <Show
                                      when={
                                        storage.zfsPool &&
                                        (storage.zfsPool.state !== 'ONLINE' ||
                                          storage.zfsPool.readErrors > 0 ||
                                          storage.zfsPool.writeErrors > 0 ||
                                          storage.zfsPool.checksumErrors > 0)
                                      }
                                    >
                                      <tr class="bg-yellow-50 dark:bg-yellow-950/20 border-l-4 border-yellow-500">
                                        <td colspan="9" class="p-2">
                                          <div class="text-xs space-y-1">
                                            <div class="flex items-center gap-2">
                                              <span class="font-semibold text-yellow-700 dark:text-yellow-400">
                                                ZFS Pool Status:
                                              </span>
                                              <span
                                                class={`px-1.5 py-0.5 rounded text-xs font-medium ${storage.zfsPool!.state === 'ONLINE'
                                                  ? 'bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300'
                                                  : storage.zfsPool!.state === 'DEGRADED'
                                                    ? 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900 dark:text-yellow-300'
                                                    : 'bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300'
                                                  }`}
                                              >
                                                {storage.zfsPool!.state}
                                              </span>
                                              <Show
                                                when={
                                                  storage.zfsPool!.readErrors > 0 ||
                                                  storage.zfsPool!.writeErrors > 0 ||
                                                  storage.zfsPool!.checksumErrors > 0
                                                }
                                              >
                                                <span class="text-red-600 dark:text-red-400">
                                                  Errors: {storage.zfsPool!.readErrors} read,{' '}
                                                  {storage.zfsPool!.writeErrors} write,{' '}
                                                  {storage.zfsPool!.checksumErrors} checksum
                                                </span>
                                              </Show>
                                            </div>
                                            <Show
                                              when={storage.zfsPool!.devices.some(
                                                (d) =>
                                                  d.state !== 'ONLINE' ||
                                                  d.readErrors > 0 ||
                                                  d.writeErrors > 0 ||
                                                  d.checksumErrors > 0,
                                              )}
                                            >
                                              <div class="ml-4 space-y-0.5">
                                                <For
                                                  each={storage.zfsPool!.devices.filter(
                                                    (d) =>
                                                      d.state !== 'ONLINE' ||
                                                      d.readErrors > 0 ||
                                                      d.writeErrors > 0 ||
                                                      d.checksumErrors > 0,
                                                  )}
                                                >
                                                  {(device) => (
                                                    <div class="flex items-center gap-2 text-xs">
                                                      <span class="text-gray-600 dark:text-gray-400">
                                                        Device {device.name}:
                                                      </span>
                                                      <span
                                                        class={`${device.state !== 'ONLINE' ? 'text-red-600 dark:text-red-400' : 'text-yellow-600 dark:text-yellow-400'}`}
                                                      >
                                                        {device.state}
                                                        <Show
                                                          when={
                                                            device.readErrors > 0 ||
                                                            device.writeErrors > 0 ||
                                                            device.checksumErrors > 0
                                                          }
                                                        >
                                                          <span class="ml-1">
                                                            ({device.readErrors}R/{device.writeErrors}
                                                            W/{device.checksumErrors}C errors)
                                                          </span>
                                                        </Show>
                                                        <Show when={device.message?.trim()}>
                                                          {(message) => (
                                                            <span class="ml-1 text-gray-500 dark:text-gray-400">
                                                              {message()}
                                                            </span>
                                                          )}
                                                        </Show>
                                                      </span>
                                                    </div>
                                                  )}
                                                </For>
                                              </div>
                                            </Show>
                                          </div>
                                        </td>
                                      </tr>
                                    </Show>
                                  </>
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
      </Show>

      {/* Physical Disks Tab */}
      <Show when={tabView() === 'disks'}>
        <DiskList
          disks={state.physicalDisks || []}
          selectedNode={selectedNode()}
          searchTerm={searchTerm()}
        />
      </Show>
    </div>
  );
};

export default Storage;
