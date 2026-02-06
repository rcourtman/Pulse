import { Component, createSignal, Show, For, createMemo, createEffect } from 'solid-js';
import { useNavigate } from '@solidjs/router';
import { useWebSocket } from '@/App';
import { formatBytes, formatAbsoluteTime, formatRelativeTime, formatUptime, formatPercent } from '@/utils/format';
import { createLocalStorageBooleanSignal, STORAGE_KEYS } from '@/utils/localStorage';
import { parseFilterStack, evaluateFilterStack } from '@/utils/searchQuery';
import { UnifiedNodeSelector } from '@/components/shared/UnifiedNodeSelector';
import { MetricBar } from '@/components/Dashboard/MetricBar';
import { BackupsFilter } from './BackupsFilter';
import { Card } from '@/components/shared/Card';
import { EmptyState } from '@/components/shared/EmptyState';
import { SectionHeader } from '@/components/shared/SectionHeader';
import { showTooltip, hideTooltip } from '@/components/shared/Tooltip';
import type { BackupType, GuestType, UnifiedBackup } from '@/types/backups';
import { usePersistentSignal } from '@/hooks/usePersistentSignal';
import { useDebouncedValue } from '@/hooks/useDebouncedValue';
import { useColumnVisibility, type ColumnDef } from '@/hooks/useColumnVisibility';
import { logger } from '@/utils/logger';

type BackupSortKey = keyof Pick<
  UnifiedBackup,
  | 'backupTime'
  | 'name'
  | 'node'
  | 'vmid'
  | 'backupType'
  | 'size'
  | 'storage'
  | 'verified'
  | 'type'
  | 'owner'
>;
const BACKUP_SORT_KEY_VALUES: readonly BackupSortKey[] = [
  'backupTime',
  'name',
  'node',
  'vmid',
  'backupType',
  'size',
  'storage',
  'verified',
  'type',
  'owner',
] as const;

type FilterableGuestType = 'VM' | 'LXC' | 'Host';

interface DateGroup {
  label: string;
  items: UnifiedBackup[];
}

const UnifiedBackups: Component = () => {
  const navigate = useNavigate();
  const { state, connected, initialDataReceived } = useWebSocket();
  const pveBackupsState = createMemo(() => state.backups?.pve ?? state.pveBackups);
  const pbsBackupsState = createMemo(() => state.backups?.pbs ?? state.pbsBackups);
  const pmgBackupsState = createMemo(() => state.backups?.pmg ?? state.pmgBackups);
  const [searchTerm, setSearchTerm] = createSignal('');

  // PBS enhancement banner: Check if user has PBS storage via PVE passthrough but no direct PBS connection
  const [pbsBannerDismissed, setPbsBannerDismissed] = createLocalStorageBooleanSignal(
    'pulse.pbsEnhancementBannerDismissed',
    false
  );

  // Detect PBS storage accessed via PVE passthrough (storage.type === 'pbs')
  const hasPBSViaPassthrough = createMemo(() => {
    const storageList = state.storage || [];
    return storageList.some((s) => s.type === 'pbs');
  });

  // Check if user has direct PBS instances configured
  const hasDirectPBS = createMemo(() => {
    return (state.pbs || []).length > 0;
  });

  // Show banner if: PBS via passthrough exists, no direct PBS, and not dismissed
  const showPBSEnhancementBanner = createMemo(() => {
    return hasPBSViaPassthrough() && !hasDirectPBS() && !pbsBannerDismissed();
  });
  const [selectedNode, setSelectedNode] = createSignal<string | null>(null);
  const [selectedNodeType, setSelectedNodeType] = createSignal<'pve' | 'pbs' | null>(null);
  const [typeFilter, setTypeFilter] = createSignal<'all' | FilterableGuestType>('all');
  const [backupTypeFilter, setBackupTypeFilter] = createSignal<'all' | BackupType>('all');
  const [statusFilter, setStatusFilter] = createSignal<'all' | 'verified' | 'unverified'>('all');
  const [groupByMode, setGroupByMode] = createSignal<'date' | 'guest'>('date');

  // Convert between UI filter and internal filter for BackupsFilter component
  const uiBackupTypeFilter = createMemo(() => {
    const filter = backupTypeFilter();
    if (filter === 'all') return 'all';
    if (filter === 'snapshot') return 'snapshot';
    if (filter === 'local') return 'pve';
    if (filter === 'remote') return 'pbs';
    return 'all';
  });

  const setUiBackupTypeFilter = (value: 'all' | 'snapshot' | 'pve' | 'pbs') => {
    if (value === 'all') setBackupTypeFilter('all');
    else if (value === 'snapshot') setBackupTypeFilter('snapshot');
    else if (value === 'pve') setBackupTypeFilter('local');
    else if (value === 'pbs') setBackupTypeFilter('remote');
  };
  const [sortKey, setSortKey] = usePersistentSignal<BackupSortKey>('backupsSortKey', 'backupTime', {
    deserialize: (raw) =>
      BACKUP_SORT_KEY_VALUES.includes(raw as BackupSortKey) ? (raw as BackupSortKey) : 'backupTime',
  });
  const [sortDirection, setSortDirection] = usePersistentSignal<'asc' | 'desc'>(
    'backupsSortDirection',
    'desc',
    {
      deserialize: (raw) => (raw === 'asc' || raw === 'desc' ? raw : 'desc'),
    },
  );
  const [selectedDateRange, setSelectedDateRange] = createSignal<{
    start: number;
    end: number;
  } | null>(null);
  const [chartTimeRange, setChartTimeRange] = createSignal(30);
  const [isSearchLocked, setIsSearchLocked] = createSignal(false);

  // Pagination state - limit rows to prevent UI lockup with large backup counts
  const ITEMS_PER_PAGE = 100;
  const [currentPage, setCurrentPage] = createSignal(1);
  const availableBackupsTooltipText =
    'Daily counts of backups still available for restore across snapshots, PVE storage, and PBS.';

  const sortKeyOptions: { value: BackupSortKey; label: string }[] = [
    { value: 'backupTime', label: 'Time' },
    { value: 'name', label: 'Guest Name' },
    { value: 'node', label: 'Node' },
    { value: 'vmid', label: 'VMID' },
    { value: 'backupType', label: 'Backup Type' },
    { value: 'size', label: 'Size' },
    { value: 'storage', label: 'Storage' },
    { value: 'verified', label: 'Verified' },
    { value: 'type', label: 'Guest Type' },
    { value: 'owner', label: 'Owner' },
  ];

  // Extract PBS instance from search term
  const selectedPBSInstance = createMemo(() => {
    const search = searchTerm();
    const match = search.match(/node:(\S+)/);
    if (match && state.pbs?.some((pbs) => pbs.name === match[1])) {
      return match[1];
    }
    return null;
  });

  // Auto-set backup type filter when PBS instance is selected
  createEffect(() => {
    const pbsInstance = selectedPBSInstance();
    if (pbsInstance) {
      setBackupTypeFilter('remote');
    } else if (!isSearchLocked()) {
      setBackupTypeFilter('all');
    }
  });

  const [useRelativeTime] = createLocalStorageBooleanSignal(
    STORAGE_KEYS.BACKUPS_USE_RELATIVE_TIME,
    false, // Default to absolute time
  );

  // Column visibility definitions for the backup table
  const BACKUP_COLUMNS: ColumnDef[] = [
    { id: 'vmid', label: 'VMID/ID' },
    { id: 'type', label: 'Type' },
    { id: 'name', label: 'Name' },
    { id: 'node', label: 'Node' },
    { id: 'owner', label: 'Owner', toggleable: true },
    { id: 'backupTime', label: 'Time' },
    { id: 'size', label: 'Size', toggleable: true },
    { id: 'backupType', label: 'Backup Type' },
    { id: 'storage', label: 'Location', toggleable: true },
    { id: 'verified', label: 'Verified', toggleable: true },
    { id: 'comment', label: 'Comment', toggleable: true },
    { id: 'details', label: 'Details' },
  ];

  // Column visibility with persistence - hide less useful columns by default
  const columnVisibility = useColumnVisibility(
    STORAGE_KEYS.BACKUPS_HIDDEN_COLUMNS,
    BACKUP_COLUMNS,
    ['comment'] // Comment hidden by default (often empty)
  );
  const visibleColumns = columnVisibility.visibleColumns;
  const isColumnVisible = (id: string) => visibleColumns().some(col => col.id === id);

  // Helper functions
  const getDaySuffix = (day: number) => {
    if (day >= 11 && day <= 13) return 'th';
    switch (day % 10) {
      case 1:
        return 'st';
      case 2:
        return 'nd';
      case 3:
        return 'rd';
      default:
        return 'th';
    }
  };

  const truncateMiddle = (str: string, maxLength: number) => {
    if (!str || str.length <= maxLength) return str;
    const start = Math.ceil(maxLength / 2) - 2;
    const end = Math.floor(maxLength / 2) - 2;
    return str.substring(0, start) + '...' + str.substring(str.length - end);
  };

  const formatTime = (timestamp: number) => {
    return useRelativeTime() ? formatRelativeTime(timestamp) : formatAbsoluteTime(timestamp);
  };

  const normalizePBSNamespace = (namespace: string | null | undefined) => {
    const trimmed = (namespace ?? '').trim();
    if (!trimmed || trimmed === '/') return 'root';
    if (trimmed.startsWith('/')) return `root${trimmed}`;
    return trimmed;
  };

  // PERFORMANCE: Parse search term once and share between filteredData and chartData
  // This eliminates duplicate string parsing and filter stack creation
  interface ParsedSearchFilters {
    isEmpty: boolean;
    isPbsNamespaceFilter: boolean;
    pbsFilter?: { instanceName: string; datastoreName: string; namespace: string };
    advancedFilters: string[];
    textSearches: string[];
    filterStack: ReturnType<typeof parseFilterStack> | null;
  }

  // PERFORMANCE: Debounce search term to prevent jank during rapid typing
  const debouncedSearchTerm = useDebouncedValue(() => searchTerm(), 200);

  const parsedSearchFilters = createMemo<ParsedSearchFilters>(() => {
    const search = debouncedSearchTerm().toLowerCase();

    if (!search) {
      return { isEmpty: true, isPbsNamespaceFilter: false, advancedFilters: [], textSearches: [], filterStack: null };
    }

    // Check for special PBS namespace filter format
    if (search.startsWith('pbs:')) {
      const parts = search.split(':');
      if (parts.length >= 4) {
        const [, instanceName, datastoreName, ...namespaceParts] = parts;
        const namespace = namespaceParts.join(':');
        return {
          isEmpty: false,
          isPbsNamespaceFilter: true,
          pbsFilter: { instanceName, datastoreName, namespace },
          advancedFilters: [],
          textSearches: [],
          filterStack: null,
        };
      }
    }

    // Split by commas and separate filters from text searches
    const searchParts = search.split(',').map((t) => t.trim()).filter((t) => t);
    const advancedFilters: string[] = [];
    const textSearches: string[] = [];

    searchParts.forEach((part) => {
      if (part.includes('>') || part.includes('<') || part.includes(':')) {
        advancedFilters.push(part);
      } else {
        textSearches.push(part.toLowerCase());
      }
    });

    // Pre-parse the filter stack if there are advanced filters
    let filterStack: ReturnType<typeof parseFilterStack> | null = null;
    if (advancedFilters.length > 0) {
      const filterString = advancedFilters.join(' AND ');
      filterStack = parseFilterStack(filterString);
      if (filterStack.filters.length === 0) {
        filterStack = null;
      }
    }

    return {
      isEmpty: false,
      isPbsNamespaceFilter: false,
      advancedFilters,
      textSearches,
      filterStack,
    };
  });

  // PERFORMANCE: Helper function to apply parsed search filters to data
  // Used by both filteredData and chartData to avoid code duplication
  const applySearchFilters = (data: UnifiedBackup[]): UnifiedBackup[] => {
    const parsed = parsedSearchFilters();

    if (parsed.isEmpty) {
      return data;
    }

    // Apply PBS namespace filter
    if (parsed.isPbsNamespaceFilter && parsed.pbsFilter) {
      const { instanceName, datastoreName, namespace } = parsed.pbsFilter;
      return data.filter((item) => {
        if (item.backupType !== 'remote') return false;
        if (item.node !== instanceName) return false;
        if (item.datastore !== datastoreName) return false;
        const itemNamespace = normalizePBSNamespace(item.namespace);
        const searchNamespace = normalizePBSNamespace(namespace);
        return itemNamespace === searchNamespace;
      });
    }

    let result = data;

    // Apply advanced filters
    if (parsed.filterStack) {
      result = result.filter((item) => evaluateFilterStack(item, parsed.filterStack!));
    }

    // Apply text search
    if (parsed.textSearches.length > 0) {
      result = result.filter((item) =>
        parsed.textSearches.some((term) => {
          const searchFields = [
            item.vmid?.toString(),
            item.name,
            item.node,
            item.backupName,
            item.description,
            item.storage,
            item.datastore,
            item.namespace,
          ]
            .filter(Boolean)
            .map((field) => field!.toString().toLowerCase());

          return searchFields.some((field) => field.includes(term));
        }),
      );
    }

    return result;
  };

  // Check if we have any backup data yet
  const isLoading = createMemo(() => !connected() || !initialDataReceived());

  // Normalize all backup data into unified format
  const normalizedData = createMemo(() => {
    const unified: UnifiedBackup[] = [];
    const seenBackups = new Set<string>(); // Track backups to avoid duplicates

    // Debug mode - can be enabled via console: localStorage.setItem('debug-pmg', 'true')
    const debugMode = typeof window !== 'undefined' && localStorage.getItem('debug-pmg') === 'true';

    // Normalize snapshots
    pveBackupsState()?.guestSnapshots?.forEach((snapshot) => {
      // Try to find the guest name by matching VMID and instance (not hostname)
      let guestName = '';
      const vm = state.vms?.find(
        (v) => v.vmid === snapshot.vmid && v.instance === snapshot.instance,
      );
      const ct = state.containers?.find(
        (c) => c.vmid === snapshot.vmid && c.instance === snapshot.instance,
      );
      if (vm) {
        guestName = vm.name || '';
      } else if (ct) {
        guestName = ct.name || '';
      }

      unified.push({
        backupType: 'snapshot',
        vmid: snapshot.vmid,
        name: guestName || `VM ${snapshot.vmid}`, // Use guest name if found, otherwise fallback to VMID
        type: snapshot.type === 'qemu' ? 'VM' : 'LXC',
        node: snapshot.node,
        instance: snapshot.instance, // Unique instance ID for handling duplicate node names
        backupTime: snapshot.time ? new Date(snapshot.time).getTime() / 1000 : 0,
        backupName: snapshot.name, // This is the snapshot name like "current", "pre-upgrade"
        description: snapshot.description || '',
        status: 'ok',
        size: typeof snapshot.sizeBytes === 'number' ? snapshot.sizeBytes : null,
        storage: null,
        datastore: null,
        namespace: null,
        verified: null,
        protected: false,
      });
    });

    // Process PBS backups FIRST from the new Go backend (state.pbsBackups)
    // This ensures we have the complete PBS data with namespaces
    // Filter by selected PBS instance if one is selected
    const pbsBackupsToProcess = selectedPBSInstance()
      ? pbsBackupsState()?.filter((b) => b.instance === selectedPBSInstance())
      : pbsBackupsState();

    pbsBackupsToProcess?.forEach((backup) => {
      const backupDate = new Date(backup.backupTime);
      const dateStr = backupDate.toISOString().split('T')[0];
      const timeStr = backupDate.toISOString().split('T')[1].split('.')[0].replace(/:/g, '');
      const backupName = `${backup.backupType}/${backup.vmid}/${dateStr}_${timeStr}`;

      // Create a key that matches the format used by PVE storage backups
      // Use just the timestamp in seconds (Unix time) to match ctime format
      const backupTimeSeconds = Math.floor(backupDate.getTime() / 1000);
      const backupKey = `${backup.vmid}-${backupTimeSeconds}`;
      seenBackups.add(backupKey);

      if (debugMode) {
        logger.debug(
          `PBS backup: vmid=${backup.vmid}, time=${backupTimeSeconds}, key=${backupKey}, verified=${backup.verified}`,
        );
      }

      // Check if any files have encryption
      const isEncrypted =
        backup.files &&
        Array.isArray(backup.files) &&
        backup.files.some((file) => {
          if (typeof file === 'string') return file.includes('.enc');
          const fileObj = file as Record<string, unknown>;
          return (
            fileObj.crypt ||
            fileObj.encrypted ||
            (typeof fileObj.filename === 'string' && fileObj.filename.includes('.enc'))
          );
        });

      // Determine the display type based on VMID and backup type
      // VMID 0 = host config backup (e.g. PMG)
      // PBS stores PMG backups as 'ct' type with VMID='0'
      let displayType: GuestType;

      // Check for VMID=0 which indicates host backup (handle both string and number)
      const vmidAsNumber =
        typeof backup.vmid === 'string' ? parseInt(backup.vmid, 10) : backup.vmid;
      const isVmidZero = vmidAsNumber === 0;

      if (debugMode && isVmidZero) {
        logger.debug('[PMG Debug] PBS backup with VMID=0:', {
          vmid: backup.vmid,
          vmidType: typeof backup.vmid,
          backupType: backup.backupType,
          instance: backup.instance,
          comment: backup.comment,
          willBeMarkedAs: isVmidZero ? 'Host' : backup.backupType === 'ct' ? 'LXC' : 'Unknown',
        });
      }

      if (isVmidZero || backup.backupType === 'host') {
        displayType = 'Host';
      } else if (backup.backupType === 'vm' || backup.backupType === 'VM') {
        displayType = 'VM';
      } else if (backup.backupType === 'ct' || backup.backupType === 'lxc') {
        displayType = 'LXC';
      } else {
        // Default fallback
        displayType = 'LXC';
      }

      unified.push({
        backupType: 'remote',
        vmid: displayType === 'Host' ? backup.vmid : parseInt(backup.vmid) || 0,
        name: backup.comment || '',
        type: displayType,
        node: backup.instance || 'PBS',
        instance: backup.instance || 'PBS',
        backupTime: backupTimeSeconds,
        backupName: backupName,
        description: backup.comment || '',
        status: backup.verified ? 'verified' : 'unverified',
        size: backup.size || null,
        storage: null,
        datastore: backup.datastore || null,
        namespace: backup.namespace || 'root',
        verified: backup.verified || false,
        protected: backup.protected || false,
        encrypted: isEncrypted,
        owner: backup.owner,
        comment: backup.comment,
      });
    });

    pmgBackupsState()?.forEach((backup) => {
      const backupTimeMs = backup.backupTime ? Date.parse(backup.backupTime) : Number.NaN;
      const backupTimeSeconds = Number.isFinite(backupTimeMs) ? Math.floor(backupTimeMs / 1000) : 0;
      const backupKey = backup.id || `${backup.instance}:${backup.node}:${backup.filename}`;
      if (seenBackups.has(backupKey)) {
        return;
      }
      seenBackups.add(backupKey);

      const displayName = backup.node || backup.filename || 'PMG Host Backup';
      unified.push({
        backupType: 'local',
        vmid: backup.node || backup.filename,
        name: displayName,
        type: 'Host',
        node: backup.node || 'PMG',
        instance: backup.instance || 'PMG',
        backupTime: backupTimeSeconds,
        backupName: backup.filename || displayName,
        description: backup.filename || '',
        status: 'ok',
        size: backup.size || null,
        storage: 'PMG',
        datastore: null,
        namespace: null,
        verified: null,
        protected: false,
      });
    });

    // Normalize local backups (including PBS through PVE storage)
    pveBackupsState()?.storageBackups?.forEach((backup) => {
      // Skip templates and ISOs - they're not backups
      if (backup.type === 'vztmpl' || backup.type === 'iso') {
        return;
      }

      // Determine if this is actually a PBS backup based on storage
      const backupType = backup.isPBS ? 'remote' : 'local';

      // Create a key to check for duplicates - use VMID and timestamp
      // This prevents the same backup from appearing twice if it's in multiple storages
      const backupKey = `${backup.vmid}-${backup.ctime}`;

      // Skip if we've already seen this backup (PBS or local duplicate)
      if (seenBackups.has(backupKey)) {
        if (debugMode) {
          logger.debug(
            `PVE storage backup duplicate skipped: vmid=${backup.vmid}, ctime=${backup.ctime}, key=${backupKey}, storage=${backup.storage}`,
          );
        }
        return; // Skip duplicate
      }
      seenBackups.add(backupKey); // Mark as seen to prevent duplicates

      // Determine the display type based on backup.type and VMID
      // VMID 0 = host config backup (e.g. PMG/PVE host configs)
      let displayType: GuestType;

      // Check for VMID=0 which indicates host backup
      const vmidAsNumber =
        typeof backup.vmid === 'string' ? parseInt(backup.vmid, 10) : backup.vmid;
      const isVmidZero = vmidAsNumber === 0;

      if (debugMode && isVmidZero) {
        logger.debug('[PMG Debug] Storage backup with VMID=0:', {
          vmid: backup.vmid,
          vmidType: typeof backup.vmid,
          type: backup.type,
          volid: backup.volid,
          notes: backup.notes,
          willBeMarkedAs: isVmidZero ? 'Host' : backup.type === 'lxc' ? 'LXC' : 'Unknown',
        });
      }

      // Check for host backups - VMID 0 or type 'host' (for PMG/PVE host configs)
      if (isVmidZero || backup.type === 'host') {
        displayType = 'Host';
      } else if (backup.type === 'qemu' || backup.type === 'vm') {
        displayType = 'VM';
      } else if (backup.type === 'lxc' || backup.type === 'ct') {
        displayType = 'LXC';
      } else {
        // Default fallback
        displayType = 'LXC';
      }

      // For PBS backups through storage: show Proxmox node in Node column, PBS storage in Location
      // For regular backups: show Proxmox node in Node column, local storage in Location
      unified.push({
        backupType: backupType,
        vmid: displayType === 'Host' ? backup.vmid : backup.vmid || 0,
        name: backup.notes || backup.volid?.split('/').pop() || '',
        type: displayType,
        node: backup.node || '', // Proxmox node that has access to this backup
        instance: backup.instance || '', // Unique instance identifier
        backupTime: backup.ctime || 0,
        backupName: backup.volid?.split('/').pop() || '',
        description: backup.notes || '', // Use notes field for PBS backup descriptions
        status: backup.verified ? 'verified' : 'unverified',
        size: backup.size || null,
        storage: backup.storage || null, // Storage name (PBS storage or local storage)
        datastore: null, // Only set for direct PBS API backups
        namespace: null, // Only set for direct PBS API backups
        verified: backup.verified || false,
        protected: backup.protected || false,
        encrypted: backup.encryption ? true : false, // Check encryption field from Proxmox API
      });
    });

    return unified;
  });

  // Check if there are any Host type backups
  const hasHostBackups = createMemo(() => {
    const data = normalizedData();
    return data.some((backup) => backup.type === 'Host');
  });

  const hasAnyBackups = createMemo(() => normalizedData().length > 0);

  // Apply filters
  const filteredData = createMemo(() => {
    let data = normalizedData();
    const type = typeFilter();
    const backupType = backupTypeFilter();
    const status = statusFilter();
    const dateRange = selectedDateRange();
    const nodeFilter = selectedNode();

    // Date range filter
    if (dateRange) {
      data = data.filter(
        (item) => item.backupTime >= dateRange.start && item.backupTime <= dateRange.end,
      );
    }

    // Node selection filter using both instance and node name for uniqueness
    if (nodeFilter) {
      const nodeType = selectedNodeType();
      if (nodeType === 'pbs') {
        // PBS instance selected - filter by instance name
        data = data.filter((item) => item.instance === nodeFilter);
      } else {
        // PVE node selected - filter by instance and node name
        const node = state.nodes?.find((n) => n.id === nodeFilter);
        if (node) {
          data = data.filter((item) => item.instance === node.instance && item.node === node.name);
        }
      }
    }

    // PERFORMANCE: Use shared search filter helper (eliminates ~70 lines of duplicate code)
    data = applySearchFilters(data);

    // Type filter
    if (type !== 'all') {
      data = data.filter((item) => item.type === type);
    }

    // Backup type filter
    if (backupType !== 'all') {
      data = data.filter((item) => item.backupType === backupType);
    }

    // Status filter
    if (status !== 'all') {
      data = data.filter((item) => {
        if (status === 'verified') return item.verified === true;
        if (status === 'unverified') return item.verified === false;
        return true;
      });
    }

    // Sort
    const key = sortKey();
    const dir = sortDirection();
    data = [...data].sort((a, b) => {
      let aVal = a[key];
      let bVal = b[key];

      // Handle null/undefined/empty values - put at end for both asc and desc
      const aIsEmpty = aVal === null || aVal === undefined || aVal === '';
      const bIsEmpty = bVal === null || bVal === undefined || bVal === '';

      if (aIsEmpty && bIsEmpty) return 0;
      if (aIsEmpty) return 1;
      if (bIsEmpty) return -1;

      // Type-specific value preparation
      if (key === 'size' || key === 'vmid' || key === 'backupTime') {
        // Ensure numeric comparison
        aVal = typeof aVal === 'number' ? aVal : Number(aVal) || 0;
        bVal = typeof bVal === 'number' ? bVal : Number(bVal) || 0;
      }

      // Type-safe comparison
      if (typeof aVal === 'number' && typeof bVal === 'number') {
        if (aVal === bVal) return 0;
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

    return data;
  });

  // Group by date or guest
  const groupedData = createMemo(() => {
    const mode = groupByMode();
    const data = filteredData();

    // Group by guest mode
    if (mode === 'guest') {
      const groups: DateGroup[] = [];
      const groupMap = new Map<string, UnifiedBackup[]>();

      // Group by VMID and name combination
      data.forEach((item) => {
        // Create a unique key for each guest
        const guestKey = `${item.type} ${item.vmid}${item.name ? ` - ${item.name}` : ''}`;

        if (!groupMap.has(guestKey)) {
          groupMap.set(guestKey, []);
        }
        groupMap.get(guestKey)!.push(item);
      });

      // Convert to array and sort by VMID
      groupMap.forEach((items, label) => {
        groups.push({ label, items });
      });

      // Sort groups by VMID (extract from label)
      groups.sort((a, b) => {
        const vmidA = parseInt(a.label.match(/\d+/)?.[0] || '0');
        const vmidB = parseInt(b.label.match(/\d+/)?.[0] || '0');
        return vmidA - vmidB;
      });

      return groups;
    }

    // Group by date mode (default)
    // If not sorting by time, show all items in a single group to preserve sort order
    if (sortKey() !== 'backupTime') {
      return [
        {
          label: 'All Backups',
          items: data,
        },
      ];
    }

    const groups: DateGroup[] = [];
    const groupMap = new Map<string, UnifiedBackup[]>();
    const now = new Date();
    const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());
    const yesterday = new Date(today);
    yesterday.setDate(yesterday.getDate() - 1);

    const months = [
      'January',
      'February',
      'March',
      'April',
      'May',
      'June',
      'July',
      'August',
      'September',
      'October',
      'November',
      'December',
    ];

    const currentYear = now.getFullYear();

    data.forEach((item) => {
      const date = new Date(item.backupTime * 1000);
      const dateOnly = new Date(date.getFullYear(), date.getMonth(), date.getDate());
      const backupYear = date.getFullYear();

      let label: string;
      const month = months[date.getMonth()];
      const day = date.getDate();
      const suffix = getDaySuffix(day);
      // Show year for non-current year backups
      const absoluteDate = backupYear === currentYear
        ? `${month} ${day}${suffix}`
        : `${month} ${day}${suffix}, ${backupYear}`;

      if (dateOnly.getTime() === today.getTime()) {
        label = `Today (${absoluteDate})`;
      } else if (dateOnly.getTime() === yesterday.getTime()) {
        label = `Yesterday (${absoluteDate})`;
      } else {
        label = absoluteDate;
      }

      if (!groupMap.has(label)) {
        groupMap.set(label, []);
      }
      groupMap.get(label)!.push(item);
    });

    // Convert to array
    groupMap.forEach((items, label) => {
      groups.push({ label, items });
    });

    // Sort groups based on sort direction
    if (sortDirection() === 'desc') {
      // Most recent first
      groups.sort((a, b) => {
        if (a.label.includes('Today')) return -1;
        if (b.label.includes('Today')) return 1;
        if (a.label.includes('Yesterday')) return b.label.includes('Today') ? 1 : -1;
        if (b.label.includes('Yesterday')) return a.label.includes('Today') ? -1 : 1;

        // For other dates, use the first item's date
        const dateA = a.items[0]?.backupTime || 0;
        const dateB = b.items[0]?.backupTime || 0;
        return dateB - dateA;
      });
    } else {
      // Oldest first
      groups.sort((a, b) => {
        if (a.label.includes('Today')) return 1;
        if (b.label.includes('Today')) return -1;
        if (a.label.includes('Yesterday')) return a.label.includes('Today') ? -1 : 1;
        if (b.label.includes('Yesterday')) return b.label.includes('Today') ? 1 : -1;

        // For other dates, use the first item's date
        const dateA = a.items[0]?.backupTime || 0;
        const dateB = b.items[0]?.backupTime || 0;
        return dateA - dateB;
      });
    }

    // Sort items within each group by time (already sorted by filteredData, but we need to maintain it)
    // The items come pre-sorted from filteredData(), so we don't need to re-sort them

    return groups;
  });

  // Total item count across all groups (for pagination)
  const totalItems = createMemo(() => {
    return groupedData().reduce((sum, group) => sum + group.items.length, 0);
  });

  // Total pages
  const totalPages = createMemo(() => {
    return Math.max(1, Math.ceil(totalItems() / ITEMS_PER_PAGE));
  });

  // Reset to page 1 when filters change
  createEffect(() => {
    // Track filter dependencies
    searchTerm();
    selectedNode();
    typeFilter();
    backupTypeFilter();
    statusFilter();
    selectedDateRange();
    // Reset page
    setCurrentPage(1);
  });

  // Paginated data - slice across groups to get ITEMS_PER_PAGE items
  const paginatedData = createMemo(() => {
    const groups = groupedData();
    const start = (currentPage() - 1) * ITEMS_PER_PAGE;
    const end = start + ITEMS_PER_PAGE;

    // Flatten all items with their group info, slice, then regroup
    const allItems: { group: DateGroup; item: UnifiedBackup }[] = [];
    for (const group of groups) {
      for (const item of group.items) {
        allItems.push({ group, item });
      }
    }

    const sliced = allItems.slice(start, end);

    // Regroup the sliced items
    const resultGroups: DateGroup[] = [];
    const groupMap = new Map<string, UnifiedBackup[]>();

    for (const { group, item } of sliced) {
      if (!groupMap.has(group.label)) {
        groupMap.set(group.label, []);
      }
      groupMap.get(group.label)!.push(item);
    }

    // Preserve original group order
    for (const group of groups) {
      const items = groupMap.get(group.label);
      if (items && items.length > 0) {
        resultGroups.push({ label: group.label, items });
      }
    }

    return resultGroups;
  });

  // Sort handler
  const handleSort = (key: BackupSortKey) => {
    if (sortKey() === key) {
      // Toggle direction for the same column
      const newDir = sortDirection() === 'asc' ? 'desc' : 'asc';
      setSortDirection(newDir);
    } else {
      // New column - set key and default direction
      setSortKey(key);
      // Set default sort direction based on column type
      // For time and size, default to descending (newest/largest first)
      // For others, default to ascending
      if (key === 'backupTime' || key === 'size') {
        setSortDirection('desc');
      } else {
        setSortDirection('asc');
      }
    }
  };

  // Reset filters
  const resetFilters = () => {
    setSearchTerm('');
    setSelectedNode(null);
    setSelectedNodeType(null);
    setIsSearchLocked(false);
    setTypeFilter('all');
    setBackupTypeFilter('all');
    setStatusFilter('all');
    setGroupByMode('date');
    setSortKey('backupTime');
    setSortDirection('desc');
    setSelectedDateRange(null);
    setChartTimeRange(30);
  };

  // localStorage persistence is now handled by createLocalStorageBooleanSignal

  // Handle keyboard shortcuts
  let searchInputRef: HTMLInputElement | undefined;

  createEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      // Escape key behavior
      if (e.key === 'Escape') {
        // Clear search and reset filters
        if (
          searchTerm().trim() ||
          selectedNode() ||
          typeFilter() !== 'all' ||
          backupTypeFilter() !== 'all' ||
          statusFilter() !== 'all' ||
          selectedDateRange() !== null ||
          sortKey() !== 'backupTime' ||
          sortDirection() !== 'desc'
        ) {
          resetFilters();

          // Blur the search input if it's focused
          if (searchInputRef && document.activeElement === searchInputRef) {
            searchInputRef.blur();
          }
        }
      }
    };

    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  });

  // Get age color class
  const getAgeColorClass = (timestamp: number) => {
    if (!timestamp) return 'text-gray-500 dark:text-gray-400';

    const now = Date.now() / 1000;
    const diff = now - timestamp;
    const days = diff / 86400;

    if (days < 3) return 'text-green-600 dark:text-green-400';
    if (days < 7) return 'text-yellow-600 dark:text-yellow-400';
    if (days < 30) return 'text-orange-500 dark:text-orange-400';
    return 'text-red-600 dark:text-red-400';
  };

  // Get size color class
  const getSizeColor = (size: number | null) => {
    if (!size) return '';
    const gb = size / (1024 * 1024 * 1024);
    if (gb < 5) return 'text-green-600 dark:text-green-400';
    if (gb < 20) return 'text-yellow-600 dark:text-yellow-400';
    if (gb < 50) return 'text-orange-600 dark:text-orange-400';
    return 'text-red-600 dark:text-red-400';
  };

  // Calculate deduplication factor for PBS backups
  const dedupFactor = createMemo(() => {
    // Get all PBS instances with datastores
    if (!state.pbs || state.pbs.length === 0) return null;

    // Collect all deduplication factors from all datastores
    const dedupFactors: number[] = [];
    state.pbs.forEach((instance) => {
      if (instance.datastores) {
        instance.datastores.forEach((ds) => {
          if (ds.deduplicationFactor && ds.deduplicationFactor > 0) {
            dedupFactors.push(ds.deduplicationFactor);
          }
        });
      }
    });

    if (dedupFactors.length === 0) return null;

    // Calculate average deduplication factor across all datastores
    const avgFactor = dedupFactors.reduce((sum, f) => sum + f, 0) / dedupFactors.length;

    // Format as multiplication factor
    return avgFactor.toFixed(1) + 'x';
  });

  const parseDateKey = (dateKey: string) => {
    const [yearStr, monthStr, dayStr] = dateKey.split('-');
    const year = Number(yearStr);
    const month = Number(monthStr);
    const day = Number(dayStr);
    if (!Number.isFinite(year) || !Number.isFinite(month) || !Number.isFinite(day)) {
      return new Date(dateKey);
    }
    return new Date(year, Math.max(0, month - 1), Math.max(1, day));
  };

  // Calculate available backup counts for chart
  const chartData = createMemo(() => {
    const days = chartTimeRange();
    const now = new Date();

    // Initialize data structure for each day
    const dailyData: {
      [key: string]: { snapshots: number; pve: number; pbs: number; total: number };
    } = {};

    // Create entries for each day in the range, including today
    for (let i = days - 1; i >= 0; i--) {
      const date = new Date(now);
      date.setDate(date.getDate() - i);
      // Use local date string format (YYYY-MM-DD) instead of ISO to avoid timezone issues
      const year = date.getFullYear();
      const month = String(date.getMonth() + 1).padStart(2, '0');
      const day = String(date.getDate()).padStart(2, '0');
      const dateKey = `${year}-${month}-${day}`;
      dailyData[dateKey] = { snapshots: 0, pve: 0, pbs: 0, total: 0 };
    }

    // Calculate the actual start and end times for filtering
    const startDate = new Date(now);
    startDate.setDate(startDate.getDate() - (days - 1));
    startDate.setHours(0, 0, 0, 0);
    const startTime = startDate.getTime();

    const endDate = new Date(now);
    endDate.setHours(23, 59, 59, 999);
    const endTime = endDate.getTime();

    // Use filtered data but WITHOUT date range filter for the chart
    // The chart should show the time range, and filters should affect what's counted
    let dataForChart = normalizedData();
    const type = typeFilter();
    const backupType = backupTypeFilter();

    // PERFORMANCE: Use shared search filter helper (eliminates ~75 lines of duplicate code)
    dataForChart = applySearchFilters(dataForChart);

    // Apply type filter
    if (type !== 'all') {
      dataForChart = dataForChart.filter((item) => item.type === type);
    }

    // Apply backup type filter
    if (backupType !== 'all') {
      dataForChart = dataForChart.filter((item) => item.backupType === backupType);
    }

    // Count backups per day within the chart time range
    dataForChart.forEach((backup) => {
      const backupTime = backup.backupTime * 1000;
      if (backupTime >= startTime && backupTime <= endTime) {
        const date = new Date(backupTime);
        // Use local date string format (YYYY-MM-DD) to match the keys we created
        const year = date.getFullYear();
        const month = String(date.getMonth() + 1).padStart(2, '0');
        const day = String(date.getDate()).padStart(2, '0');
        const dateKey = `${year}-${month}-${day}`;

        if (dailyData[dateKey]) {
          dailyData[dateKey].total++;
          if (backup.backupType === 'snapshot') {
            dailyData[dateKey].snapshots++;
          } else if (backup.backupType === 'local') {
            dailyData[dateKey].pve++;
          } else if (backup.backupType === 'remote') {
            dailyData[dateKey].pbs++;
          }
        }
      }
    });

    // Convert to array and calculate max value for scaling
    const dataArray = Object.entries(dailyData).map(([date, counts]) => ({
      date,
      ...counts,
    }));

    const maxValue = Math.max(...dataArray.map((d) => d.total), 1);

    return { data: dataArray, maxValue };
  });

  // Sort PBS instances by status then by name
  const sortedPBSInstances = createMemo(() => {
    if (!state.pbs) return [];
    return [...state.pbs].sort((a, b) => {
      // Online instances first
      const aOnline = a.status === 'healthy' || a.status === 'online';
      const bOnline = b.status === 'healthy' || b.status === 'online';
      if (aOnline !== bOnline) return aOnline ? -1 : 1;
      // Then by name
      return a.name.localeCompare(b.name);
    });
  });

  return (
    <div class="space-y-4">
      {/* Empty State - No nodes at all configured */}
      <Show
        when={
          !isLoading() && (state.nodes || []).length === 0 && (!state.pbs || state.pbs.length === 0)
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
            title="No backup sources configured"
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

      {/* Unified Node Selector */}
      <UnifiedNodeSelector
        currentTab="backups"
        globalTemperatureMonitoringEnabled={state.temperatureMonitoringEnabled}
        onNodeSelect={(nodeId, nodeType) => {
          setSelectedNode(nodeId);
          setSelectedNodeType(nodeType ?? null);
        }}
        onNamespaceSelect={(namespaceFilter) => {
          setSearchTerm(namespaceFilter);
          // Only lock if we're setting a filter, unlock if clearing
          setIsSearchLocked(namespaceFilter !== '');
        }}
        searchTerm={searchTerm()}
        showNodeSummary={false}
      />

      {/* PBS Enhancement Banner - shown when PBS storage exists via PVE but no direct PBS connection */}
      <Show when={showPBSEnhancementBanner()}>
        <div class="relative bg-blue-50 dark:bg-blue-900/20 rounded-lg border border-blue-200 dark:border-blue-800 p-4">
          <button
            type="button"
            onClick={() => setPbsBannerDismissed(true)}
            class="absolute top-2 right-2 p-1 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 transition-colors"
            title="Dismiss"
          >
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <line x1="18" y1="6" x2="6" y2="18" />
              <line x1="6" y1="6" x2="18" y2="18" />
            </svg>
          </button>
          <div class="flex items-start gap-3 pr-8">
            <div class="flex-shrink-0 mt-0.5">
              <svg class="w-5 h-5 text-blue-600 dark:text-blue-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                <circle cx="12" cy="12" r="10" />
                <path d="M12 16v-4M12 8h.01" />
              </svg>
            </div>
            <div class="flex-1 min-w-0">
              <h4 class="text-sm font-medium text-blue-900 dark:text-blue-100">
                Get more from your PBS backups
              </h4>
              <p class="mt-1 text-xs text-blue-700 dark:text-blue-300">
                You're viewing PBS backups through your PVE connection. Add your PBS server directly to unlock:
              </p>
              <ul class="mt-2 text-xs text-blue-600 dark:text-blue-400 space-y-1">
                <li class="flex items-center gap-1.5">
                  <svg class="w-3.5 h-3.5 text-green-500 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
                    <polyline points="20 6 9 17 4 12" />
                  </svg>
                  <span>Deduplication factor and storage efficiency stats</span>
                </li>
                <li class="flex items-center gap-1.5">
                  <svg class="w-3.5 h-3.5 text-green-500 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
                    <polyline points="20 6 9 17 4 12" />
                  </svg>
                  <span>PBS server health monitoring (CPU, memory, uptime)</span>
                </li>
                <li class="flex items-center gap-1.5">
                  <svg class="w-3.5 h-3.5 text-green-500 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
                    <polyline points="20 6 9 17 4 12" />
                  </svg>
                  <span>Sync, verify, prune, and GC job status</span>
                </li>
              </ul>
              <button
                type="button"
                onClick={() => navigate('/settings')}
                class="mt-3 inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium text-white bg-blue-600 hover:bg-blue-700 rounded-md transition-colors"
              >
                Add PBS Server
                <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                  <path d="M9 5l7 7-7 7" />
                </svg>
              </button>
            </div>
          </div>
        </div>
      </Show>

      {/* Removed old PBS table */}
      <Show
        when={
          // eslint-disable-next-line no-constant-binary-expression
          false && sortedPBSInstances().length > 0
        }
      >
        <Card padding="none" class="overflow-hidden">
          <div class="overflow-x-auto" style="scrollbar-width: none; -ms-overflow-style: none;">
            <style>{`
              .overflow-x-auto::-webkit-scrollbar { display: none; }
            `}</style>
            <table class="w-full">
              <thead>
                <tr class="border-b border-gray-200 dark:border-gray-700">
                  <th class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider">
                    PBS Instance
                  </th>
                  <th class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider">
                    Status
                  </th>
                  <th class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider">
                    CPU
                  </th>
                  <th class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider">
                    Memory
                  </th>
                  <th class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider">
                    Storage
                  </th>
                  <th class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider">
                    Datastores
                  </th>
                  <th class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider">
                    Backups
                  </th>
                  <th class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider">
                    Uptime
                  </th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
                <For each={sortedPBSInstances()}>
                  {(pbs) => {
                    const isOnline = () => pbs.status === 'healthy' || pbs.status === 'online';
                    const cpuPercent = () => Math.round(pbs.cpu || 0);
                    const memPercent = () =>
                      pbs.memoryTotal ? Math.round((pbs.memoryUsed / pbs.memoryTotal) * 100) : 0;

                    // Calculate total storage across all datastores
                    const totalStorage = () => {
                      if (!pbs.datastores) return { used: 0, total: 0, percent: 0 };
                      const totals = pbs.datastores.reduce(
                        (acc, ds) => {
                          acc.used += ds.used || 0;
                          acc.total += ds.total || 0;
                          return acc;
                        },
                        { used: 0, total: 0 },
                      );
                      return {
                        ...totals,
                        percent:
                          totals.total > 0 ? Math.round((totals.used / totals.total) * 100) : 0,
                      };
                    };

                    const storage = totalStorage();

                    // Count backups for this PBS instance
                    const pbsBackups = () =>
                      pbsBackupsState()?.filter((b) => b.instance === pbs.name).length || 0;

                    const isSelected = () => selectedPBSInstance() === pbs.name;

                    return (
                      <tr
                        class={`hover:bg-gray-50 dark:hover:bg-gray-700/30 cursor-pointer transition-colors ${isSelected() ? 'bg-blue-50 dark:bg-blue-900/20' : ''
                          }`}
                        onClick={() => {
                          const currentSearch = searchTerm();
                          const nodeFilter = `node:${pbs.name}`;

                          if (currentSearch.includes(nodeFilter)) {
                            setSearchTerm(
                              currentSearch
                                .replace(nodeFilter, '')
                                .trim()
                                .replace(/,\s*,/g, ',')
                                .replace(/^,|,$/g, ''),
                            );
                            setIsSearchLocked(false);
                          } else {
                            const cleanedSearch = currentSearch
                              .replace(/node:\S+/g, '')
                              .trim()
                              .replace(/,\s*,/g, ',')
                              .replace(/^,|,$/g, '');
                            const newSearch = cleanedSearch
                              ? `${cleanedSearch}, ${nodeFilter}`
                              : nodeFilter;
                            setSearchTerm(newSearch);
                            setIsSearchLocked(true);
                          }
                        }}
                      >
                        <td class="p-0.5 px-1.5 whitespace-nowrap">
                          <div class="flex items-center gap-1">
                            <a
                              href={pbs.host || `https://${pbs.name}:8007`}
                              target="_blank"
                              onClick={(e) => e.stopPropagation()}
                              class="font-medium text-xs text-gray-900 dark:text-gray-100 hover:text-blue-600 dark:hover:text-blue-400"
                            >
                              {pbs.name}
                            </a>
                            <Show when={pbs.version}>
                              <span class="text-[9px] text-gray-500 dark:text-gray-400">
                                v{pbs.version}
                              </span>
                            </Show>
                          </div>
                        </td>
                        <td class="p-0.5 px-1.5 whitespace-nowrap">
                          <div class="flex items-center gap-1">
                            <span
                              class={`h-2 w-2 rounded-full ${isOnline() ? 'bg-green-500' : 'bg-red-500'
                                }`}
                            />
                            <span class="text-xs text-gray-600 dark:text-gray-400">
                              {isOnline() ? 'Online' : 'Offline'}
                            </span>
                          </div>
                        </td>
                        <td class="p-0.5 px-1.5 min-w-[180px]">
                          <MetricBar value={cpuPercent()} label={formatPercent(cpuPercent())} type="cpu" />
                        </td>
                        <td class="p-0.5 px-1.5 min-w-[180px]">
                          <MetricBar
                            value={memPercent()}
                            label={formatPercent(memPercent())}
                            sublabel={
                              pbs.memoryTotal
                                ? `${formatBytes(pbs.memoryUsed)}/${formatBytes(pbs.memoryTotal)}`
                                : undefined
                            }
                            type="memory"
                          />
                        </td>
                        <td class="p-0.5 px-1.5 min-w-[180px]">
                          <MetricBar
                            value={storage.percent}
                            label={formatPercent(storage.percent)}
                            sublabel={`${formatBytes(storage.used)}/${formatBytes(storage.total)}`}
                            type="disk"
                          />
                        </td>
                        <td class="p-0.5 px-1.5 whitespace-nowrap text-center">
                          <span class="text-xs text-gray-700 dark:text-gray-300">
                            {pbs.datastores?.length || 0}
                          </span>
                        </td>
                        <td class="p-0.5 px-1.5 whitespace-nowrap text-center">
                          <span class="text-xs text-gray-700 dark:text-gray-300">
                            {pbsBackups()}
                          </span>
                        </td>
                        <td class="p-0.5 px-1.5 whitespace-nowrap">
                          <span class="text-xs text-gray-600 dark:text-gray-400">
                            <Show when={isOnline() && pbs.uptime} fallback="-">
                              {formatUptime(pbs.uptime)}
                            </Show>
                          </span>
                        </td>
                      </tr>
                    );
                  }}
                </For>
              </tbody>
            </table>
          </div>
        </Card>
      </Show>

      {/* Main Content - show when any nodes or PBS are configured */}
      <Show when={(state.nodes || []).length > 0 || (state.pbs && state.pbs.length > 0)}>
        {/* Available Backups chart - keep visible while backups exist or a date filter is active */}
        <Show when={hasAnyBackups() || selectedDateRange() !== null}>
          <Card padding="md">
            <div class="mb-3 flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
              <SectionHeader
                title={
                  <div class="flex items-center gap-1.5">
                    <span>Available backups</span>
                    <button
                      type="button"
                      class="text-gray-400 hover:text-gray-600 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-400 dark:text-gray-500 dark:hover:text-gray-300"
                      aria-label={availableBackupsTooltipText}
                      onMouseEnter={(e) => {
                        const rect = e.currentTarget.getBoundingClientRect();
                        showTooltip(
                          availableBackupsTooltipText,
                          rect.left + rect.width / 2,
                          rect.top,
                          {
                            align: 'center',
                            direction: 'up',
                          },
                        );
                      }}
                      onFocus={(e) => {
                        const rect = e.currentTarget.getBoundingClientRect();
                        showTooltip(
                          availableBackupsTooltipText,
                          rect.left + rect.width / 2,
                          rect.top,
                          {
                            align: 'center',
                            direction: 'up',
                          },
                        );
                      }}
                      onMouseLeave={() => hideTooltip()}
                      onBlur={() => hideTooltip()}
                    >
                      <svg
                        class="h-4 w-4"
                        fill="none"
                        stroke="currentColor"
                        stroke-width="1.6"
                        viewBox="0 0 24 24"
                        aria-hidden="true"
                      >
                        <path
                          stroke-linecap="round"
                          stroke-linejoin="round"
                          d="M11.25 11.25h1.5v3.75m-.75-9.75h.008v.008H12.75V5.25H11.25Zm0 0a7.5 7.5 0 1 1-7.5 7.5 7.5 7.5 0 0 1 7.5-7.5Z"
                        />
                      </svg>
                    </button>
                  </div>
                }
                size="sm"
                class="flex-1"
              />
              <div class="flex flex-col items-start gap-2 sm:items-end">
                <Show when={selectedDateRange()}>
                  {(selection) => (
                    <div class="inline-flex items-center gap-2 rounded-full border border-blue-200 dark:border-blue-700 bg-blue-50 dark:bg-blue-900/30 px-3 py-1 text-xs text-blue-700 dark:text-blue-200">
                      <span class="font-medium uppercase tracking-wide text-[10px] text-blue-600 dark:text-blue-300">
                        Filtered Date
                      </span>
                      <span class="font-mono text-[11px]">
                        {new Date(selection().start * 1000).toLocaleDateString('en-US', {
                          month: 'short',
                          day: 'numeric',
                          year: 'numeric',
                        })}
                      </span>
                    </div>
                  )}
                </Show>
                <div class="flex flex-wrap items-center justify-start gap-2 sm:justify-end">
                  <div class="flex items-center gap-1">
                    <button
                      type="button"
                      onClick={() => setChartTimeRange(7)}
                      class={`p-0.5 px-1.5 text-xs border rounded transition-colors ${chartTimeRange() === 7
                        ? 'bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300 border-blue-300 dark:border-blue-700'
                        : 'border-gray-300 dark:border-gray-600 hover:bg-gray-100 dark:hover:bg-gray-700'
                        }`}
                    >
                      7d
                    </button>
                    <button
                      type="button"
                      onClick={() => setChartTimeRange(30)}
                      class={`p-0.5 px-1.5 text-xs border rounded transition-colors ${chartTimeRange() === 30
                        ? 'bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300 border-blue-300 dark:border-blue-700'
                        : 'border-gray-300 dark:border-gray-600 hover:bg-gray-100 dark:hover:bg-gray-700'
                        }`}
                    >
                      30d
                    </button>
                    <button
                      type="button"
                      onClick={() => setChartTimeRange(90)}
                      class={`p-0.5 px-1.5 text-xs border rounded transition-colors ${chartTimeRange() === 90
                        ? 'bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300 border-blue-300 dark:border-blue-700'
                        : 'border-gray-300 dark:border-gray-600 hover:bg-gray-100 dark:hover:bg-gray-700'
                        }`}
                    >
                      90d
                    </button>
                    <button
                      type="button"
                      onClick={() => setChartTimeRange(365)}
                      class={`p-0.5 px-1.5 text-xs border rounded transition-colors ${chartTimeRange() === 365
                        ? 'bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300 border-blue-300 dark:border-blue-700'
                        : 'border-gray-300 dark:border-gray-600 hover:bg-gray-100 dark:hover:bg-gray-700'
                        }`}
                    >
                      1y
                    </button>
                  </div>
                  <div class="hidden h-4 w-px bg-gray-300 dark:bg-gray-600 sm:block"></div>
                  <span class="text-gray-500 dark:text-gray-400">Last {chartTimeRange()} days</span>
                  <Show when={selectedDateRange()}>
                    <button
                      type="button"
                      onClick={() => {
                        setSelectedDateRange(null);
                        hideTooltip();
                      }}
                      class="p-0.5 px-1.5 text-xs bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300 rounded hover:bg-blue-200 dark:hover:bg-blue-800/50 transition-colors"
                    >
                      Clear filter
                    </button>
                  </Show>
                </div>
              </div>
            </div>
            <div class="relative h-32 bg-gray-100 dark:bg-gray-800 rounded overflow-hidden">
              <Show
                when={chartData().data.length > 0}
                fallback={
                  <div class="flex h-full items-center justify-center">
                    <EmptyState
                      class="max-w-xs"
                      align="center"
                      icon={
                        <svg
                          class="h-10 w-10 text-gray-400"
                          fill="none"
                          viewBox="0 0 24 24"
                          stroke="currentColor"
                        >
                          <path
                            stroke-linecap="round"
                            stroke-linejoin="round"
                            stroke-width="2"
                            d="M4 19h16M7 10h2v9H7zm4-5h2v14h-2zm4 8h2v6h-2z"
                          />
                        </svg>
                      }
                      title="No backup data"
                      description="Adjust filters or expand the time range to see activity."
                    />
                  </div>
                }
              >
                <svg
                  class="available-backups-svg w-full h-full"
                  role="img"
                  aria-label="Available backups chart"
                  style="cursor: pointer; touch-action: pan-y;"
                  ref={(el) => {
                    // Use createEffect to reactively update the chart
                    createEffect(() => {
                      if (!el) return;

                      const data = chartData().data;
                      if (data.length === 0) return;

                      // Wait for next frame to ensure dimensions are available
                      requestAnimationFrame(() => {
                        const rect = el.getBoundingClientRect();
                        if (rect.width === 0 || rect.height === 0) return;

                        const margin = { top: 10, right: 10, bottom: 30, left: 30 };
                        const width = Math.max(rect.width - margin.left - margin.right, 1);
                        const height = 128 - margin.top - margin.bottom;

                        el.setAttribute('viewBox', `0 0 ${rect.width} 128`);
                        // Clear existing content safely
                        while (el.firstChild) {
                          el.removeChild(el.firstChild);
                        }

                        // Create main group
                        const g = document.createElementNS('http://www.w3.org/2000/svg', 'g');
                        g.setAttribute('transform', `translate(${margin.left},${margin.top})`);
                        el.appendChild(g);

                        const data = chartData().data;
                        const maxValue = chartData().maxValue;
                        const xScale = width / Math.max(data.length, 1);
                        const barWidth = Math.max(1, Math.min(xScale - 2, 50));
                        const yScale = height / maxValue;

                        // Add grid lines
                        const gridGroup = document.createElementNS(
                          'http://www.w3.org/2000/svg',
                          'g',
                        );
                        gridGroup.setAttribute('class', 'grid-lines');
                        g.appendChild(gridGroup);

                        // Y-axis grid lines
                        const gridCount = 5;
                        for (let i = 0; i <= gridCount; i++) {
                          const y = height - (i * height) / gridCount;
                          const line = document.createElementNS(
                            'http://www.w3.org/2000/svg',
                            'line',
                          );
                          line.setAttribute('x1', '0');
                          line.setAttribute('y1', y.toString());
                          line.setAttribute('x2', width.toString());
                          line.setAttribute('y2', y.toString());
                          line.setAttribute('stroke', 'currentColor');
                          line.setAttribute('stroke-opacity', '0.1');
                          line.setAttribute('class', 'text-gray-300 dark:text-gray-600');
                          gridGroup.appendChild(line);
                        }

                        // Add Y-axis labels
                        if (maxValue <= 5) {
                          for (let i = 0; i <= maxValue; i++) {
                            const y = height - (i * height) / maxValue;
                            const text = document.createElementNS(
                              'http://www.w3.org/2000/svg',
                              'text',
                            );
                            text.setAttribute('x', '-5');
                            text.setAttribute('y', (y + 3).toString());
                            text.setAttribute('text-anchor', 'end');
                            text.setAttribute(
                              'class',
                              'text-[10px] fill-gray-500 dark:fill-gray-400',
                            );
                            text.textContent = i.toString();
                            g.appendChild(text);
                          }
                        } else {
                          for (let i = 0; i <= gridCount; i++) {
                            const value = Math.round((i * maxValue) / gridCount);
                            const y = height - (i * height) / gridCount;

                            if (i === 0 || value !== Math.round(((i - 1) * maxValue) / gridCount)) {
                              const text = document.createElementNS(
                                'http://www.w3.org/2000/svg',
                                'text',
                              );
                              text.setAttribute('x', '-5');
                              text.setAttribute('y', (y + 3).toString());
                              text.setAttribute('text-anchor', 'end');
                              text.setAttribute(
                                'class',
                                'text-[10px] fill-gray-500 dark:fill-gray-400',
                              );
                              text.textContent = value.toString();
                              g.appendChild(text);
                            }
                          }
                        }

                        // Add bars
                        const barsGroup = document.createElementNS(
                          'http://www.w3.org/2000/svg',
                          'g',
                        );
                        barsGroup.setAttribute('class', 'bars');
                        g.appendChild(barsGroup);

                        data.forEach((d, i) => {
                          const barHeight = d.total * yScale;
                          const x = Math.max(0, i * xScale + (xScale - barWidth) / 2);
                          const y = height - barHeight;
                          const barDate = parseDateKey(d.date);
                          const barDateMs = barDate.getTime();

                          const barGroup = document.createElementNS(
                            'http://www.w3.org/2000/svg',
                            'g',
                          );
                          barGroup.setAttribute('class', 'bar-group');
                          barGroup.setAttribute('data-date', d.date);
                          barGroup.style.cursor = 'pointer';

                          // Background track for all slots
                          const track = document.createElementNS(
                            'http://www.w3.org/2000/svg',
                            'rect',
                          );
                          track.setAttribute('x', x.toString());
                          track.setAttribute('y', (height - 2).toString());
                          track.setAttribute('width', barWidth.toString());
                          track.setAttribute('height', '2');
                          track.setAttribute('rx', '1');
                          track.setAttribute('fill', '#d1d5db');
                          track.setAttribute('fill-opacity', '0.3');
                          barGroup.appendChild(track);

                          // Click area
                          const clickRect = document.createElementNS(
                            'http://www.w3.org/2000/svg',
                            'rect',
                          );
                          clickRect.setAttribute('x', (i * xScale).toString());
                          clickRect.setAttribute('y', '0');
                          clickRect.setAttribute('width', Math.max(1, xScale).toString());
                          clickRect.setAttribute('height', height.toString());
                          clickRect.setAttribute('fill', 'transparent');
                          clickRect.style.cursor = 'pointer';
                          barGroup.appendChild(clickRect);

                          // Main bar
                          const rect = document.createElementNS(
                            'http://www.w3.org/2000/svg',
                            'rect',
                          );
                          rect.setAttribute('x', x.toString());
                          rect.setAttribute('y', y.toString());
                          rect.setAttribute('width', barWidth.toString());
                          rect.setAttribute('height', barHeight.toString());
                          rect.setAttribute('rx', '2');
                          rect.setAttribute('class', 'backup-bar');
                          rect.setAttribute('data-date', d.date);

                          // Color based on count
                          let barColor = '#e5e7eb';
                          if (d.total > 0 && d.total <= 5) barColor = '#60a5fa';
                          else if (d.total <= 10) barColor = '#34d399';
                          else if (d.total > 10) barColor = '#a78bfa';

                          rect.setAttribute('fill', barColor);
                          rect.style.transition = 'all 0.2s ease';

                          // Highlight selected date
                          const isSelected =
                            selectedDateRange() &&
                            barDateMs >= selectedDateRange()!.start * 1000 &&
                            barDateMs <= selectedDateRange()!.end * 1000;

                          if (isSelected) {
                            rect.setAttribute('fill-opacity', '1');
                            rect.setAttribute('stroke', '#2563eb');
                            rect.setAttribute('stroke-width', '2');
                            rect.setAttribute('filter', 'brightness(1.1)');
                          } else {
                            rect.setAttribute('fill-opacity', '0.8');
                            rect.setAttribute('stroke', 'none');
                          }

                          barGroup.appendChild(rect);

                          // Stacked segments
                          if (d.total > 0) {
                            // PBS (bottom)
                            if (d.pbs > 0) {
                              const pbsHeight = (d.pbs / d.total) * barHeight;
                              const pbsRect = document.createElementNS(
                                'http://www.w3.org/2000/svg',
                                'rect',
                              );
                              pbsRect.setAttribute('x', x.toString());
                              pbsRect.setAttribute('y', (y + barHeight - pbsHeight).toString());
                              pbsRect.setAttribute('width', barWidth.toString());
                              pbsRect.setAttribute('height', pbsHeight.toString());
                              pbsRect.setAttribute('rx', '2');
                              pbsRect.setAttribute('fill', '#8b5cf6');
                              pbsRect.setAttribute('fill-opacity', '0.9');
                              barGroup.appendChild(pbsRect);
                            }

                            // PVE (middle)
                            if (d.pve > 0) {
                              const pveHeight = (d.pve / d.total) * barHeight;
                              const pveY = y + (d.snapshots / d.total) * barHeight;
                              const pveRect = document.createElementNS(
                                'http://www.w3.org/2000/svg',
                                'rect',
                              );
                              pveRect.setAttribute('x', x.toString());
                              pveRect.setAttribute('y', pveY.toString());
                              pveRect.setAttribute('width', barWidth.toString());
                              pveRect.setAttribute('height', pveHeight.toString());
                              pveRect.setAttribute('fill', '#f97316');
                              pveRect.setAttribute('fill-opacity', '0.9');
                              barGroup.appendChild(pveRect);
                            }

                            // Snapshots (top)
                            if (d.snapshots > 0) {
                              const snapshotHeight = (d.snapshots / d.total) * barHeight;
                              const snapshotRect = document.createElementNS(
                                'http://www.w3.org/2000/svg',
                                'rect',
                              );
                              snapshotRect.setAttribute('x', x.toString());
                              snapshotRect.setAttribute('y', y.toString());
                              snapshotRect.setAttribute('width', barWidth.toString());
                              snapshotRect.setAttribute('height', snapshotHeight.toString());
                              snapshotRect.setAttribute('rx', '2');
                              snapshotRect.setAttribute('fill', '#eab308');
                              snapshotRect.setAttribute('fill-opacity', '0.9');
                              barGroup.appendChild(snapshotRect);
                            }
                          }

                          // Hover effects with tooltips
                          barGroup.addEventListener('mouseenter', (event) => {
                            const e = event as MouseEvent;
                            rect.setAttribute('fill-opacity', '1');
                            rect.setAttribute('filter', 'brightness(1.2)');

                            // Show tooltip
                            const formattedDate = barDate.toLocaleDateString('en-US', {
                              weekday: 'short',
                              month: 'short',
                              day: 'numeric',
                            });

                            let tooltipText = `${formattedDate}`;

                            if (d.total > 0) {
                              tooltipText += `\nAvailable: ${d.total} backup${d.total > 1 ? 's' : ''
                                }`;

                              const breakdown: string[] = [];
                              if (d.snapshots > 0) breakdown.push(`Snapshots: ${d.snapshots}`);
                              if (d.pve > 0) breakdown.push(`PVE: ${d.pve}`);
                              if (d.pbs > 0) breakdown.push(`PBS: ${d.pbs}`);

                              if (breakdown.length > 0) {
                                tooltipText += `\n${breakdown.join(' • ')}`;
                              }
                            } else {
                              tooltipText += '\nNo backups available';
                            }

                            const mouseX = e.clientX;
                            const mouseY = e.clientY;

                            showTooltip(tooltipText, mouseX, mouseY, {
                              align: 'center',
                              direction: 'up',
                            });
                          });

                          barGroup.addEventListener('mouseleave', () => {
                            rect.setAttribute('fill-opacity', '0.8');
                            rect.removeAttribute('filter');
                            hideTooltip();
                          });

                          // Click to filter
                          barGroup.addEventListener('click', () => {
                            const startOfDay = new Date(barDate);
                            startOfDay.setHours(0, 0, 0, 0);
                            const endOfDay = new Date(barDate);
                            endOfDay.setHours(23, 59, 59, 999);
                            const clickedRange = {
                              start: startOfDay.getTime() / 1000,
                              end: endOfDay.getTime() / 1000,
                            };

                            const currentRange = selectedDateRange();
                            if (
                              currentRange &&
                              currentRange.start === clickedRange.start &&
                              currentRange.end === clickedRange.end
                            ) {
                              setSelectedDateRange(null);
                            } else {
                              setSelectedDateRange(clickedRange);
                            }

                            hideTooltip();
                          });

                          barsGroup.appendChild(barGroup);

                          // Date labels
                          let showLabel = false;
                          if (chartTimeRange() <= 7) {
                            showLabel = true;
                          } else if (chartTimeRange() <= 30) {
                            showLabel =
                              i % Math.ceil(data.length / 10) === 0 || i === data.length - 1;
                          } else if (chartTimeRange() <= 90) {
                            const dayOfWeek = barDate.getDay();
                            showLabel = dayOfWeek === 0 || i === 0 || i === data.length - 1;
                          } else {
                            showLabel = barDate.getDate() === 1 || i === 0 || i === data.length - 1;
                          }

                          if (showLabel) {
                            const text = document.createElementNS(
                              'http://www.w3.org/2000/svg',
                              'text',
                            );
                            text.setAttribute('x', (x + barWidth / 2).toString());
                            text.setAttribute('y', (height + 20).toString());
                            text.setAttribute('text-anchor', 'middle');
                            text.setAttribute(
                              'class',
                              'text-[8px] fill-gray-500 dark:fill-gray-400',
                            );

                            // Use shorter format for horizontal labels
                            let labelText;
                            if (chartTimeRange() <= 7) {
                              // For 7 days, show month/day
                              labelText = `${barDate.getMonth() + 1}/${barDate.getDate()}`;
                            } else if (chartTimeRange() <= 30) {
                              // For 30 days, show day only (or month/day for first of month)
                              labelText =
                                barDate.getDate() === 1
                                  ? `${barDate.getMonth() + 1}/1`
                                  : barDate.getDate().toString();
                            } else {
                              // For longer ranges, show month/day
                              labelText = `${barDate.getMonth() + 1}/${barDate.getDate()}`;
                            }
                            text.textContent = labelText;
                            g.appendChild(text);
                          }
                        });
                      });
                    });
                  }}
                />
              </Show>
            </div>
            <div class="flex justify-between items-center text-xs mt-2">
              <Show when={dedupFactor()}>
                <div class="flex items-center gap-1">
                  <span class="text-gray-500 dark:text-gray-400">Deduplication:</span>
                  <span class="font-medium text-green-600 dark:text-green-400">
                    {dedupFactor()}
                  </span>
                </div>
              </Show>
              <Show when={!dedupFactor()}>
                <div></div>
              </Show>
              <div class="flex items-center gap-3">
                <span class="flex items-center gap-1">
                  <span class="inline-block w-3 h-3 rounded bg-yellow-500"></span>
                  <span class="text-gray-600 dark:text-gray-400">Snapshots</span>
                </span>
                <span class="flex items-center gap-1">
                  <span class="inline-block w-3 h-3 rounded bg-orange-500"></span>
                  <span class="text-gray-600 dark:text-gray-400">PVE</span>
                </span>
                <span class="flex items-center gap-1">
                  <span class="inline-block w-3 h-3 rounded bg-violet-500"></span>
                  <span class="text-gray-600 dark:text-gray-400">PBS</span>
                </span>
              </div>
            </div>
          </Card>
        </Show>

        {/* Backups Filter */}
        <BackupsFilter
          search={searchTerm}
          setSearch={setSearchTerm}
          viewMode={uiBackupTypeFilter}
          setViewMode={setUiBackupTypeFilter}
          groupBy={groupByMode}
          setGroupBy={setGroupByMode}
          searchInputRef={(el) => (searchInputRef = el)}
          sortOptions={sortKeyOptions}
          sortKey={sortKey}
          setSortKey={(value) => setSortKey(value as BackupSortKey)}
          sortDirection={sortDirection}
          setSortDirection={setSortDirection}
          onReset={resetFilters}
          columnVisibility={columnVisibility}
        />

        {/* Table */}
        <Card padding="none" tone="glass" class="mb-4 overflow-hidden">
          <div class="overflow-x-auto" style="scrollbar-width: none; -ms-overflow-style: none; -webkit-overflow-scrolling: touch;">
            <style>{`
          .overflow-x-auto::-webkit-scrollbar { display: none; }
          .backup-table {
            width: 100%;
          }
          .backup-table th,
          .backup-table td {
            overflow: hidden;
            text-overflow: ellipsis;
            white-space: nowrap;
          }
        `}</style>
            <Show
              when={!isLoading()}
              fallback={
                <Card padding="lg">
                  <EmptyState
                    icon={
                      <svg
                        class="h-12 w-12 animate-spin text-gray-400"
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
                    title="Loading backup data..."
                    description="This may take up to 20 seconds on the first load."
                  />
                </Card>
              }
            >
              <Show
                when={groupedData().length > 0}
                fallback={
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
                      title="No backups match your filters"
                      description="Try adjusting filters or selecting a different time range."
                    />
                  </Card>
                }
              >
                {/* Mobile Card View removed in favor of scrollable table */}

                {/* Desktop Table View */}
                <table class="backup-table" style={{ "min-width": "900px" }}>
                  <thead>
                    <tr class="bg-gray-50 dark:bg-gray-700/50 text-gray-600 dark:text-gray-300 border-b border-gray-200 dark:border-gray-600">
                      <th
                        class="px-1.5 sm:px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600"
                        onClick={() => handleSort('vmid')}
                      >
                        {hasHostBackups() ? 'ID' : 'VMID'}{' '}
                        {sortKey() === 'vmid' && (sortDirection() === 'asc' ? '▲' : '▼')}
                      </th>
                      <th
                        class="px-1.5 sm:px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600"
                        onClick={() => handleSort('type')}
                      >
                        Type {sortKey() === 'type' && (sortDirection() === 'asc' ? '▲' : '▼')}
                      </th>
                      <th
                        class="px-1.5 sm:px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600"
                        onClick={() => handleSort('name')}
                      >
                        Name {sortKey() === 'name' && (sortDirection() === 'asc' ? '▲' : '▼')}
                      </th>
                      <th
                        class="px-1.5 sm:px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600"
                        onClick={() => handleSort('node')}
                      >
                        Node {sortKey() === 'node' && (sortDirection() === 'asc' ? '▲' : '▼')}
                      </th>
                      <Show when={isColumnVisible('owner') && (backupTypeFilter() === 'all' || backupTypeFilter() === 'remote')}>
                        <th
                          class="px-1.5 sm:px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600"
                          onClick={() => handleSort('owner')}
                        >
                          Owner {sortKey() === 'owner' && (sortDirection() === 'asc' ? '▲' : '▼')}
                        </th>
                      </Show>
                      <th
                        class="px-1.5 sm:px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600"
                        onClick={() => handleSort('backupTime')}
                      >
                        Time {sortKey() === 'backupTime' && (sortDirection() === 'asc' ? '▲' : '▼')}
                      </th>
                      <Show when={isColumnVisible('size') && backupTypeFilter() !== 'snapshot'}>
                        <th
                          class="px-1.5 sm:px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600"
                          onClick={() => handleSort('size')}
                        >
                          Size {sortKey() === 'size' && (sortDirection() === 'asc' ? '▲' : '▼')}
                        </th>
                      </Show>
                      <th
                        class="px-1.5 sm:px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600"
                        onClick={() => handleSort('backupType')}
                      >
                        Backup{' '}
                        {sortKey() === 'backupType' && (sortDirection() === 'asc' ? '▲' : '▼')}
                      </th>
                      <Show when={isColumnVisible('storage') && backupTypeFilter() !== 'snapshot'}>
                        <th
                          class="px-1.5 sm:px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600"
                          onClick={() => handleSort('storage')}
                        >
                          Location{' '}
                          {sortKey() === 'storage' && (sortDirection() === 'asc' ? '▲' : '▼')}
                        </th>
                      </Show>
                      <Show when={isColumnVisible('verified') && (backupTypeFilter() === 'all' || backupTypeFilter() === 'remote')}>
                        <th
                          class="px-2 py-1.5 text-center text-[11px] sm:text-xs font-medium uppercase tracking-wider cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600"
                          onClick={() => handleSort('verified')}
                        >
                          <span title="Verified">✓</span>
                          {sortKey() === 'verified' && (sortDirection() === 'asc' ? ' ▲' : ' ▼')}
                        </th>
                      </Show>
                      <Show when={isColumnVisible('comment') && (backupTypeFilter() === 'all' || backupTypeFilter() === 'remote')}>
                        <th class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider">
                          Comment
                        </th>
                      </Show>
                      <th
                        class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider"
                      >
                        Details
                      </th>
                    </tr>
                  </thead>
                  <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
                    <For each={paginatedData()}>
                      {(group) => (
                        <>
                          <tr class="bg-gray-50 dark:bg-gray-900/40">
                            <td
                              colspan={20 /* Full-width row across all responsive columns */}
                              class="py-1 pr-2 pl-3 text-[12px] sm:text-sm font-semibold text-slate-700 dark:text-slate-100"
                            >
                              <div class="flex items-center justify-between gap-3">
                                <span class="truncate" title={group.label}>
                                  {group.label}
                                </span>
                                <span class="text-[10px] font-medium text-slate-500 dark:text-slate-400">
                                  {group.items.length}
                                </span>
                              </div>
                            </td>
                          </tr>
                          <For each={group.items}>
                            {(item) => (
                              <tr class="border-t border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700/30">
                                <td class="p-0.5 pl-3 sm:pl-5 pr-1 sm:pr-1.5 text-xs sm:text-sm align-middle">{item.vmid}</td>
                                <td class="p-0.5 px-1 sm:px-1.5 align-middle">
                                  <span
                                    class={`inline-flex items-center px-1.5 py-0.5 rounded text-xs font-medium ${item.type === 'VM'
                                      ? 'bg-blue-100 text-blue-700 dark:bg-blue-900/50 dark:text-blue-300'
                                      : item.type === 'Host'
                                        ? 'bg-orange-100 text-orange-700 dark:bg-orange-900/50 dark:text-orange-300'
                                        : 'bg-green-100 text-green-700 dark:bg-green-900/50 dark:text-green-300'
                                      }`}
                                  >
                                    {item.type}
                                  </span>
                                </td>
                                <td class="p-0.5 px-1.5 text-sm align-middle">
                                  {item.name || '-'}
                                </td>
                                <td class="p-0.5 px-1.5 text-sm align-middle">{item.node}</td>
                                <Show
                                  when={
                                    isColumnVisible('owner') && (backupTypeFilter() === 'all' || backupTypeFilter() === 'remote')
                                  }
                                >
                                  <td class="p-0.5 px-1.5 text-xs align-middle text-gray-500 dark:text-gray-400">
                                    {item.owner ? item.owner.split('@')[0] : '-'}
                                  </td>
                                </Show>
                                <td
                                  class={`p-0.5 px-1.5 text-xs align-middle ${getAgeColorClass(item.backupTime)}`}
                                >
                                  {formatTime(item.backupTime * 1000)}
                                </td>
                                <Show when={isColumnVisible('size') && backupTypeFilter() !== 'snapshot'}>
                                  <td
                                    class={`p-0.5 px-1.5 align-middle ${getSizeColor(item.size)}`}
                                  >
                                    {item.size ? formatBytes(item.size) : '-'}
                                  </td>
                                </Show>
                                <td class="p-0.5 px-1 sm:px-1.5 align-middle">
                                  <div class="flex items-center gap-1">
                                    <span
                                      class={`inline-flex items-center px-1.5 py-0.5 rounded text-xs font-medium ${item.backupType === 'snapshot'
                                        ? 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/20 dark:text-yellow-300'
                                        : item.backupType === 'local'
                                          ? 'bg-orange-100 text-orange-700 dark:bg-orange-900/50 dark:text-orange-300'
                                          : 'bg-purple-100 text-purple-700 dark:bg-purple-900/50 dark:text-purple-300'
                                        }`}
                                    >
                                      {item.backupType === 'snapshot'
                                        ? 'Snapshot'
                                        : item.backupType === 'local'
                                          ? 'PVE'
                                          : 'PBS'}
                                    </span>
                                    {/* Data source indicator for PBS via passthrough */}
                                    <Show when={item.backupType === 'remote' && item.storage && !item.datastore}>
                                      <span
                                        class="text-[9px] text-gray-400 dark:text-gray-500 font-medium"
                                        title="PBS backup accessed via PVE storage - add PBS directly for more detail"
                                      >
                                        via PVE
                                      </span>
                                    </Show>
                                    <Show when={item.encrypted}>
                                      <span
                                        title="Encrypted backup"
                                        class="text-green-600 dark:text-green-400 inline-block ml-1"
                                      >
                                        <svg
                                          xmlns="http://www.w3.org/2000/svg"
                                          class="h-4 w-4"
                                          viewBox="0 0 20 20"
                                          fill="currentColor"
                                        >
                                          <path
                                            fill-rule="evenodd"
                                            d="M5 9V7a5 5 0 0110 0v2a2 2 0 012 2v5a2 2 0 01-2 2H5a2 2 0 01-2-2v-5a2 2 0 012-2zm8-2v2H7V7a3 3 0 016 0z"
                                            clip-rule="evenodd"
                                          />
                                        </svg>
                                      </span>
                                    </Show>
                                    <Show when={item.protected}>
                                      <span
                                        title="Protected backup"
                                        class="text-blue-600 dark:text-blue-400 inline-block ml-1"
                                      >
                                        <svg
                                          xmlns="http://www.w3.org/2000/svg"
                                          class="h-4 w-4"
                                          viewBox="0 0 20 20"
                                          fill="currentColor"
                                        >
                                          <path
                                            fill-rule="evenodd"
                                            d="M2.166 4.999A11.954 11.954 0 0010 1.944 11.954 11.954 0 0017.834 5c.11.65.166 1.32.166 2.001 0 5.225-3.34 9.67-8 11.317C5.34 16.67 2 12.225 2 7c0-.682.057-1.35.166-2.001zm11.541 3.708a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z"
                                            clip-rule="evenodd"
                                          />
                                        </svg>
                                      </span>
                                    </Show>
                                  </div>
                                </td>
                                <Show when={isColumnVisible('storage') && backupTypeFilter() !== 'snapshot'}>
                                  <td class="p-0.5 px-1.5 text-sm align-middle">
                                    {item.storage ||
                                      (item.datastore &&
                                        (item.namespace && item.namespace !== 'root'
                                          ? `${item.datastore}/${item.namespace}`
                                          : item.datastore)) ||
                                      '-'}
                                  </td>
                                </Show>
                                <Show
                                  when={
                                    isColumnVisible('verified') && (backupTypeFilter() === 'all' || backupTypeFilter() === 'remote')
                                  }
                                >
                                  <td class="p-0.5 px-1.5 text-center align-middle">
                                    {item.backupType === 'remote' ? (
                                      item.verified ? (
                                        <span title="PBS backup verified">
                                          <svg
                                            class="w-4 h-4 text-green-500 dark:text-green-400 inline"
                                            fill="none"
                                            viewBox="0 0 24 24"
                                            stroke="currentColor"
                                          >
                                            <path
                                              stroke-linecap="round"
                                              stroke-linejoin="round"
                                              stroke-width="2"
                                              d="M5 13l4 4L19 7"
                                            />
                                          </svg>
                                        </span>
                                      ) : (
                                        <span title="PBS backup not yet verified">
                                          <svg
                                            class="w-4 h-4 text-gray-400 dark:text-gray-500 inline"
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
                                        </span>
                                      )
                                    ) : (
                                      <span
                                        class="text-gray-400 dark:text-gray-500"
                                        title="Verification only available for PBS backups"
                                      >
                                        -
                                      </span>
                                    )}
                                  </td>
                                </Show>
                                <Show
                                  when={
                                    isColumnVisible('comment') && (backupTypeFilter() === 'all' || backupTypeFilter() === 'remote')
                                  }
                                >
                                  <td class="p-0.5 px-1.5 text-xs align-middle text-gray-500 dark:text-gray-400 max-w-[150px] truncate" title={item.comment || ''}>
                                    {item.comment || '-'}
                                  </td>
                                </Show>
                                <td
                                  class="p-0.5 px-1.5 align-middle"
                                  onMouseEnter={(e) => {
                                    const details = [];

                                    if (item.backupType === 'snapshot') {
                                      details.push(item.backupName);
                                      if (item.description) {
                                        details.push(item.description);
                                      }
                                    } else if (item.backupType === 'local') {
                                      details.push(item.backupName);
                                    } else if (item.backupType === 'remote') {
                                      if (item.protected) details.push('Protected');
                                      // For PBS backups, show the notes field which contains the backup description
                                      const pbsDescription =
                                        item.description ||
                                        (item.name && item.name !== '-' ? item.name : '');
                                      if (pbsDescription && pbsDescription.trim()) {
                                        details.push(pbsDescription);
                                      }
                                    }

                                    const fullText = details.join(' • ') || '-';
                                    if (fullText.length > 35) {
                                      const rect = e.currentTarget.getBoundingClientRect();
                                      showTooltip(fullText, rect.left, rect.top, {
                                        align: 'left',
                                        direction: 'up',
                                        maxWidth: 260,
                                      });
                                    }
                                  }}
                                  onMouseLeave={() => {
                                    hideTooltip();
                                  }}
                                >
                                  {(() => {
                                    const details = [];

                                    if (item.backupType === 'snapshot') {
                                      details.push(item.backupName);
                                      if (item.description) {
                                        details.push(item.description);
                                      }
                                    } else if (item.backupType === 'local') {
                                      details.push(truncateMiddle(item.backupName, 30));
                                    } else if (item.backupType === 'remote') {
                                      if (item.protected) details.push('Protected');
                                      // For PBS backups, show the notes field which contains the backup description
                                      const pbsDescription =
                                        item.description ||
                                        (item.name && item.name !== '-' ? item.name : '');
                                      if (pbsDescription && pbsDescription.trim()) {
                                        details.push(pbsDescription);
                                      }
                                    }

                                    const fullText = details.join(' • ') || '-';
                                    const displayText =
                                      fullText.length > 35
                                        ? fullText.substring(0, 32) + '...'
                                        : fullText;

                                    return displayText;
                                  })()}
                                </td>
                              </tr>
                            )}
                          </For>
                        </>
                      )}
                    </For>
                  </tbody>
                </table>

                {/* Pagination Controls */}
                <Show when={totalPages() > 1}>
                  <div class="flex items-center justify-between px-4 py-3 border-t border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800/50">
                    <div class="text-sm text-gray-600 dark:text-gray-400">
                      Showing {((currentPage() - 1) * ITEMS_PER_PAGE) + 1} - {Math.min(currentPage() * ITEMS_PER_PAGE, totalItems())} of {totalItems()} backups
                    </div>
                    <div class="flex items-center gap-2">
                      <button
                        onClick={() => setCurrentPage(1)}
                        disabled={currentPage() === 1}
                        class="px-2 py-1 text-sm rounded border border-gray-300 dark:border-gray-600 disabled:opacity-50 disabled:cursor-not-allowed hover:bg-gray-100 dark:hover:bg-gray-700"
                      >
                        First
                      </button>
                      <button
                        onClick={() => setCurrentPage(p => Math.max(1, p - 1))}
                        disabled={currentPage() === 1}
                        class="px-2 py-1 text-sm rounded border border-gray-300 dark:border-gray-600 disabled:opacity-50 disabled:cursor-not-allowed hover:bg-gray-100 dark:hover:bg-gray-700"
                      >
                        Previous
                      </button>
                      <span class="text-sm text-gray-600 dark:text-gray-400 px-2">
                        Page {currentPage()} of {totalPages()}
                      </span>
                      <button
                        onClick={() => setCurrentPage(p => Math.min(totalPages(), p + 1))}
                        disabled={currentPage() === totalPages()}
                        class="px-2 py-1 text-sm rounded border border-gray-300 dark:border-gray-600 disabled:opacity-50 disabled:cursor-not-allowed hover:bg-gray-100 dark:hover:bg-gray-700"
                      >
                        Next
                      </button>
                      <button
                        onClick={() => setCurrentPage(totalPages())}
                        disabled={currentPage() === totalPages()}
                        class="px-2 py-1 text-sm rounded border border-gray-300 dark:border-gray-600 disabled:opacity-50 disabled:cursor-not-allowed hover:bg-gray-100 dark:hover:bg-gray-700"
                      >
                        Last
                      </button>
                    </div>
                  </div>
                </Show>
              </Show>
            </Show>
          </div>
        </Card>
      </Show>
    </div>
  );
};

export default UnifiedBackups;
