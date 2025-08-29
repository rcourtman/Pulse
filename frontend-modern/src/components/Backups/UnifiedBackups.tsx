import { Component, createSignal, Show, For, createMemo, createEffect } from 'solid-js';
import { useWebSocket } from '@/App';
import { formatBytes, formatAbsoluteTime, formatRelativeTime, formatUptime } from '@/utils/format';
import { createLocalStorageBooleanSignal, STORAGE_KEYS } from '@/utils/localStorage';
import { parseFilterStack, evaluateFilterStack } from '@/utils/searchQuery';
import { UnifiedNodeSelector } from '@/components/shared/UnifiedNodeSelector';
import { MetricBar } from '@/components/Dashboard/MetricBar';
import { BackupsFilter } from './BackupsFilter';

type BackupType = 'snapshot' | 'local' | 'remote';
type GuestType = 'VM' | 'LXC' | 'Host' | 'Template' | 'ISO';
type FilterableGuestType = 'VM' | 'LXC' | 'Host';

interface UnifiedBackup {
  backupType: BackupType;
  vmid: number;
  name: string;
  type: GuestType;
  node: string;
  backupTime: number;
  backupName: string;
  description: string;
  status: string;
  size: number | null;
  storage: string | null;
  datastore: string | null;
  namespace: string | null;
  verified: boolean | null;
  protected: boolean;
  encrypted?: boolean;
  owner?: string;
}

// Types for PBS backups - temporarily disabled to avoid unused warnings
// type PBSBackup = any;
// type PBSSnapshot = any;

interface DateGroup {
  label: string;
  items: UnifiedBackup[];
}

const UnifiedBackups: Component = () => {
  const { state } = useWebSocket();
  const [searchTerm, setSearchTerm] = createSignal('');
  const [selectedNode, setSelectedNode] = createSignal<string | null>(null);
  const [typeFilter, setTypeFilter] = createSignal<'all' | FilterableGuestType>('all');
  const [backupTypeFilter, setBackupTypeFilter] = createSignal<'all' | BackupType>('all');
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
  const [sortKey, setSortKey] = createSignal<keyof UnifiedBackup>('backupTime');
  const [sortDirection, setSortDirection] = createSignal<'asc' | 'desc'>('desc');
  const [selectedDateRange, setSelectedDateRange] = createSignal<{ start: number; end: number } | null>(null);
  const [chartTimeRange, setChartTimeRange] = createSignal(30);
  const [tooltip, setTooltip] = createSignal<{ text: string; x: number; y: number } | null>(null);
  const [isSearchLocked, setIsSearchLocked] = createSignal(false);
  
  // Extract PBS instance from search term
  const selectedPBSInstance = createMemo(() => {
    const search = searchTerm();
    const match = search.match(/node:(\S+)/);
    if (match && state.pbs?.some(pbs => pbs.name === match[1])) {
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
    false // Default to absolute time
  );
  // TODO: Add time format toggle to BackupsFilter component
  // const setUseRelativeTime = ...; 

  // Helper functions
  const getDaySuffix = (day: number) => {
    if (day >= 11 && day <= 13) return 'th';
    switch (day % 10) {
      case 1: return 'st';
      case 2: return 'nd';
      case 3: return 'rd';
      default: return 'th';
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

  // Check if we have any backup data yet
  const isLoading = createMemo(() => {
    return !state.pveBackups?.guestSnapshots && 
           !state.pveBackups?.storageBackups && 
           !state.pbsBackups?.length && 
           !state.pbs?.length;
  });

  // Normalize all backup data into unified format
  const normalizedData = createMemo(() => {
    const unified: UnifiedBackup[] = [];
    const seenBackups = new Set<string>(); // Track backups to avoid duplicates
    
    // Debug mode - remove in production
    const debugMode = false;

    // Normalize snapshots
    state.pveBackups?.guestSnapshots?.forEach((snapshot) => {
      // Try to find the guest name by matching VMID
      let guestName = '';
      const vm = state.vms?.find(v => v.vmid === snapshot.vmid && v.node === snapshot.node);
      const ct = state.containers?.find(c => c.vmid === snapshot.vmid && c.node === snapshot.node);
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
        backupTime: snapshot.time ? new Date(snapshot.time).getTime() / 1000 : 0,
        backupName: snapshot.name, // This is the snapshot name like "current", "pre-upgrade"
        description: snapshot.description || '',
        status: 'ok',
        size: null,
        storage: null,
        datastore: null,
        namespace: null,
        verified: null,
        protected: false
      });
    });

    // Process PBS backups FIRST from the new Go backend (state.pbsBackups)
    // This ensures we have the complete PBS data with namespaces
    // Filter by selected PBS instance if one is selected
    const pbsBackupsToProcess = selectedPBSInstance() 
      ? state.pbsBackups?.filter(b => b.instance === selectedPBSInstance())
      : state.pbsBackups;
    
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
        console.log(`PBS backup: vmid=${backup.vmid}, time=${backupTimeSeconds}, key=${backupKey}, verified=${backup.verified}`);
      }
      
      // Check if any files have encryption
      const isEncrypted = backup.files && Array.isArray(backup.files) && 
        backup.files.some((file: any) => {
          if (typeof file === 'string') return false;
          return file.crypt || file.encrypted || (file.filename && file.filename.includes('.enc'));
        });
      
      unified.push({
        backupType: 'remote',
        vmid: parseInt(backup.vmid) || 0,
        name: backup.comment || '',
        type: (backup.backupType === 'vm' || backup.backupType === 'VM') ? 'VM' : 'LXC',
        node: backup.instance || 'PBS',
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
        owner: backup.owner
      });
    });

    // Normalize local backups (including PBS through PVE storage)
    state.pveBackups?.storageBackups?.forEach((backup) => {
      // Skip templates and ISOs - they're not backups
      if (backup.type === 'vztmpl' || backup.type === 'iso') {
        return;
      }
      
      // Determine if this is actually a PBS backup based on storage
      const backupType = backup.isPBS ? 'remote' : 'local';
      
      // Skip PBS backups that we already have from direct PBS API
      if (backup.isPBS && backup.volid) {
        // Check if we already have this from PBS API using the same key format
        const backupKey = `${backup.vmid}-${backup.ctime}`;
        
        if (debugMode) {
          console.log(`PVE storage backup: vmid=${backup.vmid}, ctime=${backup.ctime}, key=${backupKey}, isPBS=${backup.isPBS}, skip=${seenBackups.has(backupKey)}`);
        }
        
        if (seenBackups.has(backupKey)) {
          return; // Skip duplicate
        }
      }
      
      // Determine the display type based on backup.type
      let displayType: GuestType;
      if (backup.type === 'qemu') {
        displayType = 'VM';
      } else if (backup.type === 'lxc') {
        displayType = 'LXC';
      } else if (backup.type === 'host') {
        displayType = 'Host'; // PMG host config backups
      } else {
        displayType = 'LXC'; // Default fallback (most people have more containers than VMs)
      }
      
      // For PBS backups through storage: show Proxmox node in Node column, PBS storage in Location
      // For regular backups: show Proxmox node in Node column, local storage in Location
      unified.push({
        backupType: backupType,
        vmid: backup.vmid || 0,
        name: backup.notes || backup.volid?.split('/').pop() || '',
        type: displayType,
        node: backup.node || '',  // Proxmox node that has access to this backup
        backupTime: backup.ctime || 0,
        backupName: backup.volid?.split('/').pop() || '',
        description: backup.notes || '', // Use notes field for PBS backup descriptions
        status: backup.verified ? 'verified' : 'unverified',
        size: backup.size || null,
        storage: backup.storage || null,  // Storage name (PBS storage or local storage)
        datastore: null,  // Only set for direct PBS API backups
        namespace: null,  // Only set for direct PBS API backups
        verified: backup.verified || false,
        protected: backup.protected || false,
        encrypted: backup.encryption ? true : false  // Check encryption field from Proxmox API
      });
    });


    // Normalize PBS backups
    // NOTE: Legacy code - PBS backups are now handled differently in the Go backend
    // The 'backups' field doesn't exist on PBSInstance anymore, and 'snapshots' field
    // doesn't exist on PBSDatastore. This code is kept for reference but commented out.
    
    /*
    state.pbs?.forEach((pbsInstance) => {
      // Check if backups are at the instance level
      if (pbsInstance.backups && Array.isArray(pbsInstance.backups)) {
        pbsInstance.backups.forEach((backup: PBSBackup) => {
          unified.push({
            backupType: 'remote',
            vmid: backup.vmid || 0,
            name: backup.guestName || '',
            type: backup.type === 'vm' || backup.type === 'qemu' ? 'VM' : 'LXC',
            node: pbsInstance.name || 'PBS',
            backupTime: backup.ctime || backup.backupTime || 0,
            backupName: `${backup.vmid}/${new Date((backup.ctime || backup.backupTime || 0) * 1000).toISOString().split('T')[0]}`,
            description: backup.notes || backup.comment || '',
            status: backup.verified ? 'verified' : 'unverified',
            size: backup.size || null,
            storage: null,
            datastore: backup.datastore || null,
            namespace: backup.namespace || 'root',
            verified: backup.verified || false,
            protected: backup.protected || false
          });
        });
      }
      
      // Also check datastores for snapshots (original JS structure)
      if (pbsInstance.datastores && Array.isArray(pbsInstance.datastores)) {
        pbsInstance.datastores?.forEach((datastore) => {
          if (datastore.snapshots && Array.isArray(datastore.snapshots)) {
            datastore.snapshots.forEach((backup: PBSSnapshot) => {
              let totalSize = 0;
              if (backup.files && Array.isArray(backup.files)) {
                totalSize = backup.files.reduce((sum: number, file) => sum + (file.size || 0), 0);
              }
              
              unified.push({
                backupType: 'remote',
                vmid: backup['backup-id'] || 0,
                name: backup.comment || '',
                type: backup['backup-type'] === 'vm' || backup['backup-type'] === 'qemu' ? 'VM' : 'LXC',
                node: pbsInstance.name || 'PBS',
                backupTime: backup['backup-time'] || 0,
                backupName: `${backup['backup-id']}/${new Date((backup['backup-time'] || 0) * 1000).toISOString().split('T')[0]}`,
                description: backup.comment || '',
                status: backup.verified ? 'verified' : 'unverified',
                size: totalSize || null,
                storage: null,
                datastore: datastore.name || null,
                namespace: backup.namespace || 'root',
                verified: backup.verified || false,
                protected: backup.protected || false
              });
            });
          }
        });
      }
    });
    */

    return unified;
  });

  // Check if there are any Host type backups
  const hasHostBackups = createMemo(() => {
    const data = normalizedData();
    return data.some(backup => backup.type === 'Host');
  });

  // Apply filters
  const filteredData = createMemo(() => {
    let data = normalizedData();
    const search = searchTerm().toLowerCase();
    const type = typeFilter();
    const backupType = backupTypeFilter();
    const dateRange = selectedDateRange();
    const nodeFilter = selectedNode();

    // Date range filter
    if (dateRange) {
      data = data.filter(item => 
        item.backupTime >= dateRange.start && item.backupTime <= dateRange.end
      );
    }

    // Node selection filter
    if (nodeFilter) {
      data = data.filter(item => item.node.toLowerCase() === nodeFilter.toLowerCase());
    }

    // Search filter - with advanced filtering support like Dashboard
    if (search) {
      // Check for special PBS namespace filter format first
      if (search.startsWith('pbs:')) {
        const parts = search.split(':');
        if (parts.length >= 4) {
          // Format: pbs:instanceName:datastoreName:namespace
          const [, instanceName, datastoreName, ...namespaceParts] = parts;
          const namespace = namespaceParts.join(':'); // Handle namespaces with colons
          
          data = data.filter(item => {
            // Only PBS backups
            if (item.backupType !== 'remote') return false;
            // Match instance
            if (item.node !== instanceName) return false;
            // Match datastore
            if (item.datastore !== datastoreName) return false;
            // Match namespace (root namespace is represented as '/' or empty)
            const itemNamespace = item.namespace || '/';
            const searchNamespace = namespace || '/';
            return itemNamespace === searchNamespace;
          });
        }
      } else {
        // Split by commas first
        const searchParts = search.split(',').map(t => t.trim()).filter(t => t);
        
        // Separate filters from text searches
        const filters: string[] = [];
        const textSearches: string[] = [];
        
        searchParts.forEach(part => {
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
            data = data.filter(item => evaluateFilterStack(item, stack));
          }
        }
      
        // Apply text search if any
        if (textSearches.length > 0) {
          data = data.filter(item => 
            textSearches.some(term => {
              const searchFields = [
                item.vmid?.toString(),
                item.name,
                item.node,
                item.backupName,
                item.description,
                item.storage,
                item.datastore,
                item.namespace
              ].filter(Boolean).map(field => field!.toString().toLowerCase());
              
              return searchFields.some(field => field.includes(term));
            })
          );
        }
      }
    }

    // Type filter
    if (type !== 'all') {
      data = data.filter(item => item.type === type);
    }

    // Backup type filter
    if (backupType !== 'all') {
      data = data.filter(item => item.backupType === backupType);
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

  // Group by date
  const groupedData = createMemo(() => {
    // If sorting by time, show date groups
    // Otherwise, show all items in a single group to preserve sort order
    if (sortKey() !== 'backupTime') {
      return [{
        label: 'All Backups',
        items: filteredData()
      }];
    }

    const groups: DateGroup[] = [];
    const groupMap = new Map<string, UnifiedBackup[]>();
    const now = new Date();
    const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());
    const yesterday = new Date(today);
    yesterday.setDate(yesterday.getDate() - 1);

    const months = ['January', 'February', 'March', 'April', 'May', 'June',
                    'July', 'August', 'September', 'October', 'November', 'December'];

    filteredData().forEach(item => {
      const date = new Date(item.backupTime * 1000);
      const dateOnly = new Date(date.getFullYear(), date.getMonth(), date.getDate());
      
      let label: string;
      const month = months[date.getMonth()];
      const day = date.getDate();
      const suffix = getDaySuffix(day);
      const absoluteDate = `${month} ${day}${suffix}`;
      
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

  // Sort handler
  const handleSort = (key: keyof UnifiedBackup) => {
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
    setIsSearchLocked(false);
    setTypeFilter('all');
    setBackupTypeFilter('all');
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
      // Ignore if user is typing in an input, textarea, or contenteditable
      const target = e.target as HTMLElement;
      const isInputField = target.tagName === 'INPUT' || 
                          target.tagName === 'TEXTAREA' || 
                          target.tagName === 'SELECT' ||
                          target.contentEditable === 'true';
      
      // Escape key behavior
      if (e.key === 'Escape') {
        // Clear search and reset filters
        if (searchTerm().trim() || selectedNode() || typeFilter() !== 'all' || backupTypeFilter() !== 'all' || 
            selectedDateRange() !== null || sortKey() !== 'backupTime' || sortDirection() !== 'desc') {
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
    state.pbs.forEach(instance => {
      if (instance.datastores) {
        instance.datastores.forEach(ds => {
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



  // Calculate backup frequency data for chart
  const chartData = createMemo(() => {
    const days = chartTimeRange();
    const now = new Date();
    
    // Initialize data structure for each day
    const dailyData: { [key: string]: { snapshots: number; pve: number; pbs: number; total: number } } = {};
    
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
    const search = searchTerm().toLowerCase();
    const type = typeFilter();
    const backupType = backupTypeFilter();
    
    // Apply search filter - with advanced filtering support like the table
    if (search) {
      // Check for special PBS namespace filter first
      if (search.startsWith('pbs:')) {
        const parts = search.split(':');
        if (parts.length >= 4) {
          // Format: pbs:instanceName:datastoreName:namespace
          const [, instanceName, datastoreName, ...namespaceParts] = parts;
          const namespace = namespaceParts.join(':'); // Handle namespaces with colons
          
          dataForChart = dataForChart.filter(item => {
            // Only PBS backups
            if (item.backupType !== 'remote') return false;
            // Match instance
            if (item.node !== instanceName) return false;
            // Match datastore
            if (item.datastore !== datastoreName) return false;
            // Match namespace (root namespace is represented as '/' or empty)
            const itemNamespace = item.namespace || '/';
            const searchNamespace = namespace || '/';
            return itemNamespace === searchNamespace;
          });
        }
      } else {
        // Split by commas first
        const searchParts = search.split(',').map(t => t.trim()).filter(t => t);
        
        // Separate filters from text searches
        const filters: string[] = [];
        const textSearches: string[] = [];
        
        searchParts.forEach(part => {
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
            dataForChart = dataForChart.filter(item => evaluateFilterStack(item, stack));
          }
        }
        
        // Apply text search if any
        if (textSearches.length > 0) {
          dataForChart = dataForChart.filter(item => 
            textSearches.some(term => {
              const searchFields = [
                item.vmid?.toString(),
                item.name,
                item.node,
                item.backupName,
                item.description,
                item.storage,
                item.datastore,
                item.namespace
              ].filter(Boolean).map(field => field!.toString().toLowerCase());
              
              return searchFields.some(field => field.includes(term));
            })
          );
        }
      }
    }
    
    // Apply type filter
    if (type !== 'all') {
      dataForChart = dataForChart.filter(item => item.type === type);
    }
    
    // Apply backup type filter
    if (backupType !== 'all') {
      dataForChart = dataForChart.filter(item => item.backupType === backupType);
    }
    
    // Count backups per day within the chart time range
    dataForChart.forEach(backup => {
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
      ...counts
    }));
    
    const maxValue = Math.max(...dataArray.map(d => d.total), 1);
    
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
      <Show when={!isLoading() && (state.nodes || []).length === 0 && (!state.pbs || state.pbs.length === 0)}>
        <div class="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg p-8">
          <div class="text-center">
            <svg class="mx-auto h-12 w-12 text-gray-400 mb-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
            </svg>
            <h3 class="text-sm font-medium text-gray-900 dark:text-gray-100 mb-2">No backup sources configured</h3>
            <p class="text-xs text-gray-600 dark:text-gray-400 mb-4">Add a Proxmox VE or PBS node in the Settings tab to start monitoring backups.</p>
            <button type="button"
              onClick={() => {
                const settingsTab = document.querySelector('[role="tab"]:last-child') as HTMLElement;
                settingsTab?.click();
              }}
              class="inline-flex items-center px-3 py-1.5 border border-transparent text-xs font-medium rounded-md text-white bg-blue-600 hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500"
            >
              Go to Settings
            </button>
          </div>
        </div>
      </Show>

      {/* Unified Node Selector */}
      <UnifiedNodeSelector 
        currentTab="backups"
        onNodeSelect={(nodeId) => {
          setSelectedNode(nodeId);
          setIsSearchLocked(!!nodeId);
        }}
        onNamespaceSelect={(namespaceFilter) => {
          setSearchTerm(namespaceFilter);
          // Only lock if we're setting a filter, unlock if clearing
          setIsSearchLocked(namespaceFilter !== '');
        }}
        filteredBackups={(searchTerm() || backupTypeFilter() !== 'all') ? filteredData() : undefined}
        searchTerm={searchTerm()}
      />

      {/* Removed old PBS table */}
      <Show when={false && sortedPBSInstances().length > 0}>
        <div class="bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700">
          <div class="overflow-x-auto" style="scrollbar-width: none; -ms-overflow-style: none;">
            <style>{`
              .overflow-x-auto::-webkit-scrollbar { display: none; }
            `}</style>
            <table class="w-full">
              <thead>
                <tr class="border-b border-gray-200 dark:border-gray-700">
                  <th class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider">PBS Instance</th>
                  <th class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider">Status</th>
                  <th class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider">CPU</th>
                  <th class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider">Memory</th>
                  <th class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider">Storage</th>
                  <th class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider">Datastores</th>
                  <th class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider">Backups</th>
                  <th class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider">Uptime</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
                <For each={sortedPBSInstances()}>
                  {(pbs) => {
                    const isOnline = () => pbs.status === 'healthy' || pbs.status === 'online';
                    const cpuPercent = () => Math.round(pbs.cpu || 0);
                    const memPercent = () => pbs.memoryTotal ? Math.round((pbs.memoryUsed / pbs.memoryTotal) * 100) : 0;
                    
                    // Calculate total storage across all datastores
                    const totalStorage = () => {
                      if (!pbs.datastores) return { used: 0, total: 0, percent: 0 };
                      const totals = pbs.datastores.reduce((acc, ds) => {
                        acc.used += ds.used || 0;
                        acc.total += ds.total || 0;
                        return acc;
                      }, { used: 0, total: 0 });
                      return {
                        ...totals,
                        percent: totals.total > 0 ? Math.round((totals.used / totals.total) * 100) : 0
                      };
                    };
                    
                    const storage = totalStorage();
                    
                    // Count backups for this PBS instance
                    const pbsBackups = () => state.pbsBackups?.filter(b => b.instance === pbs.name).length || 0;
                    
                    const isSelected = () => selectedPBSInstance() === pbs.name;
                    
                    return (
                      <tr 
                        class={`hover:bg-gray-50 dark:hover:bg-gray-700/30 cursor-pointer transition-colors ${
                          isSelected() ? 'bg-blue-50 dark:bg-blue-900/20' : ''
                        }`}
                        onClick={() => {
                          const currentSearch = searchTerm();
                          const nodeFilter = `node:${pbs.name}`;
                          
                          if (currentSearch.includes(nodeFilter)) {
                            setSearchTerm(currentSearch.replace(nodeFilter, '').trim().replace(/,\s*,/g, ',').replace(/^,|,$/g, ''));
                            setIsSearchLocked(false);
                          } else {
                            const cleanedSearch = currentSearch.replace(/node:\S+/g, '').trim().replace(/,\s*,/g, ',').replace(/^,|,$/g, '');
                            const newSearch = cleanedSearch ? `${cleanedSearch}, ${nodeFilter}` : nodeFilter;
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
                            <span class={`h-2 w-2 rounded-full ${
                              isOnline() ? 'bg-green-500' : 'bg-red-500'
                            }`} />
                            <span class="text-xs text-gray-600 dark:text-gray-400">
                              {isOnline() ? 'Online' : 'Offline'}
                            </span>
                          </div>
                        </td>
                        <td class="p-0.5 px-1.5 w-[180px]">
                          <MetricBar 
                            value={cpuPercent()} 
                            label={`${cpuPercent()}%`}
                            type="cpu"
                          />
                        </td>
                        <td class="p-0.5 px-1.5 w-[180px]">
                          <MetricBar 
                            value={memPercent()} 
                            label={`${memPercent()}%`}
                            sublabel={pbs.memoryTotal ? `${formatBytes(pbs.memoryUsed)}/${formatBytes(pbs.memoryTotal)}` : undefined}
                            type="memory"
                          />
                        </td>
                        <td class="p-0.5 px-1.5 w-[180px]">
                          <MetricBar 
                            value={storage.percent} 
                            label={`${storage.percent}%`}
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
                          <span class="text-xs text-gray-700 dark:text-gray-300">{pbsBackups()}</span>
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
        </div>
      </Show>

      {/* Main Content - show when any nodes or PBS are configured */}
      <Show when={(state.nodes || []).length > 0 || (state.pbs && state.pbs.length > 0)}>
      {/* Backup Frequency Chart - hide when no backups match the filter */}
      <Show when={filteredData().length > 0}>
      <div class="p-4 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg shadow-sm">
        <div class="flex justify-between items-center mb-3">
          <div class="flex items-center gap-4">
            <h3 class="text-sm font-medium text-gray-700 dark:text-gray-300">Backup Frequency</h3>
          </div>
          <div class="flex items-center gap-2 text-xs">
            <div class="flex items-center gap-1">
              <button type="button"
                onClick={() => setChartTimeRange(7)}
                class={`p-0.5 px-1.5 text-xs border rounded transition-colors ${
                  chartTimeRange() === 7
                    ? 'bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300 border-blue-300 dark:border-blue-700'
                    : 'border-gray-300 dark:border-gray-600 hover:bg-gray-100 dark:hover:bg-gray-700'
                }`}
              >
                7d
              </button>
              <button type="button"
                onClick={() => setChartTimeRange(30)}
                class={`p-0.5 px-1.5 text-xs border rounded transition-colors ${
                  chartTimeRange() === 30
                    ? 'bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300 border-blue-300 dark:border-blue-700'
                    : 'border-gray-300 dark:border-gray-600 hover:bg-gray-100 dark:hover:bg-gray-700'
                }`}
              >
                30d
              </button>
              <button type="button"
                onClick={() => setChartTimeRange(90)}
                class={`p-0.5 px-1.5 text-xs border rounded transition-colors ${
                  chartTimeRange() === 90
                    ? 'bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300 border-blue-300 dark:border-blue-700'
                    : 'border-gray-300 dark:border-gray-600 hover:bg-gray-100 dark:hover:bg-gray-700'
                }`}
              >
                90d
              </button>
              <button type="button"
                onClick={() => setChartTimeRange(365)}
                class={`p-0.5 px-1.5 text-xs border rounded transition-colors ${
                  chartTimeRange() === 365
                    ? 'bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300 border-blue-300 dark:border-blue-700'
                    : 'border-gray-300 dark:border-gray-600 hover:bg-gray-100 dark:hover:bg-gray-700'
                }`}
              >
                1y
              </button>
            </div>
            <div class="h-4 w-px bg-gray-300 dark:bg-gray-600"></div>
            <span class="text-gray-500 dark:text-gray-400">
              Last {chartTimeRange()} days
            </span>
            <Show when={selectedDateRange()}>
              <button type="button"
                onClick={() => setSelectedDateRange(null)}
                class="p-0.5 px-1.5 text-xs bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300 rounded hover:bg-blue-200 dark:hover:bg-blue-800/50 transition-colors"
              >
                Clear filter
              </button>
            </Show>
          </div>
        </div>
        <div class="h-32 relative bg-gray-100 dark:bg-gray-800 rounded overflow-hidden">
          <Show 
            when={chartData().data.length > 0}
            fallback={
              <div class="h-full flex items-center justify-center">
                <p class="text-sm text-gray-500 dark:text-gray-400">No backup data for selected time range</p>
              </div>
            }
          >
            <svg 
              class="backup-frequency-svg w-full h-full" 
              style="cursor: pointer"
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
                    const width = rect.width - margin.left - margin.right;
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
                const gridGroup = document.createElementNS('http://www.w3.org/2000/svg', 'g');
                gridGroup.setAttribute('class', 'grid-lines');
                g.appendChild(gridGroup);
                
                // Y-axis grid lines
                const gridCount = 5;
                for (let i = 0; i <= gridCount; i++) {
                  const y = height - (i * height / gridCount);
                  const line = document.createElementNS('http://www.w3.org/2000/svg', 'line');
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
                    const y = height - (i * height / maxValue);
                    const text = document.createElementNS('http://www.w3.org/2000/svg', 'text');
                    text.setAttribute('x', '-5');
                    text.setAttribute('y', (y + 3).toString());
                    text.setAttribute('text-anchor', 'end');
                    text.setAttribute('class', 'text-[10px] fill-gray-500 dark:fill-gray-400');
                    text.textContent = i.toString();
                    g.appendChild(text);
                  }
                } else {
                  for (let i = 0; i <= gridCount; i++) {
                    const value = Math.round(i * maxValue / gridCount);
                    const y = height - (i * height / gridCount);
                    
                    if (i === 0 || value !== Math.round((i - 1) * maxValue / gridCount)) {
                      const text = document.createElementNS('http://www.w3.org/2000/svg', 'text');
                      text.setAttribute('x', '-5');
                      text.setAttribute('y', (y + 3).toString());
                      text.setAttribute('text-anchor', 'end');
                      text.setAttribute('class', 'text-[10px] fill-gray-500 dark:fill-gray-400');
                      text.textContent = value.toString();
                      g.appendChild(text);
                    }
                  }
                }
                
                // Add bars
                const barsGroup = document.createElementNS('http://www.w3.org/2000/svg', 'g');
                barsGroup.setAttribute('class', 'bars');
                g.appendChild(barsGroup);
                
                data.forEach((d, i) => {
                  const barHeight = d.total * yScale;
                  const x = Math.max(0, i * xScale + (xScale - barWidth) / 2);
                  const y = height - barHeight;
                  
                  const barGroup = document.createElementNS('http://www.w3.org/2000/svg', 'g');
                  barGroup.setAttribute('class', 'bar-group');
                  barGroup.setAttribute('data-date', d.date);
                  barGroup.style.cursor = 'pointer';
                  
                  // Background track for all slots
                  const track = document.createElementNS('http://www.w3.org/2000/svg', 'rect');
                  track.setAttribute('x', x.toString());
                  track.setAttribute('y', (height - 2).toString());
                  track.setAttribute('width', barWidth.toString());
                  track.setAttribute('height', '2');
                  track.setAttribute('rx', '1');
                  track.setAttribute('fill', '#d1d5db');
                  track.setAttribute('fill-opacity', '0.3');
                  barGroup.appendChild(track);
                  
                  // Click area
                  const clickRect = document.createElementNS('http://www.w3.org/2000/svg', 'rect');
                  clickRect.setAttribute('x', (i * xScale).toString());
                  clickRect.setAttribute('y', '0');
                  clickRect.setAttribute('width', Math.max(1, xScale).toString());
                  clickRect.setAttribute('height', height.toString());
                  clickRect.setAttribute('fill', 'transparent');
                  clickRect.style.cursor = 'pointer';
                  barGroup.appendChild(clickRect);
                  
                  // Main bar
                  const rect = document.createElementNS('http://www.w3.org/2000/svg', 'rect');
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
                  rect.setAttribute('fill-opacity', '0.8');
                  rect.style.transition = 'fill-opacity 0.2s ease';
                  
                  // Highlight selected date
                  if (selectedDateRange() && 
                      new Date(d.date).getTime() >= selectedDateRange()!.start * 1000 && 
                      new Date(d.date).getTime() <= selectedDateRange()!.end * 1000) {
                    rect.classList.add('ring-2', 'ring-blue-500');
                  }
                  
                  barGroup.appendChild(rect);
                  
                  // Stacked segments
                  if (d.total > 0) {
                    // PBS (bottom)
                    if (d.pbs > 0) {
                      const pbsHeight = (d.pbs / d.total) * barHeight;
                      const pbsRect = document.createElementNS('http://www.w3.org/2000/svg', 'rect');
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
                      const pveRect = document.createElementNS('http://www.w3.org/2000/svg', 'rect');
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
                      const snapshotRect = document.createElementNS('http://www.w3.org/2000/svg', 'rect');
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
                  barGroup.addEventListener('mouseenter', (e) => {
                    rect.setAttribute('fill-opacity', '1');
                    rect.setAttribute('filter', 'brightness(1.2)');
                    
                    // Show tooltip
                    const date = new Date(d.date);
                    const formattedDate = date.toLocaleDateString('en-US', { 
                      weekday: 'short', 
                      month: 'short', 
                      day: 'numeric' 
                    });
                    
                    let tooltipText = `${formattedDate}`;
                    
                    if (d.total > 0) {
                      tooltipText += `\nTotal: ${d.total} backup${d.total > 1 ? 's' : ''}`;
                      
                      const breakdown = [];
                      if (d.snapshots > 0) breakdown.push(`${d.snapshots} Snapshot${d.snapshots > 1 ? 's' : ''}`);
                      if (d.pve > 0) breakdown.push(`${d.pve} PVE`);
                      if (d.pbs > 0) breakdown.push(`${d.pbs} PBS`);
                      
                      if (breakdown.length > 0) {
                        tooltipText += `\n${breakdown.join(', ')}`;
                      }
                    } else {
                      tooltipText += '\nNo backups';
                    }
                    
                    // Get mouse position relative to the page
                    const mouseX = e.pageX || e.clientX + window.scrollX;
                    const mouseY = e.pageY || e.clientY + window.scrollY;
                    
                    setTooltip({
                      text: tooltipText,
                      x: mouseX,
                      y: mouseY - 60
                    });
                  });
                  
                  barGroup.addEventListener('mouseleave', () => {
                    rect.setAttribute('fill-opacity', '0.8');
                    rect.removeAttribute('filter');
                    setTooltip(null);
                  });
                  
                  // Click to filter
                  barGroup.addEventListener('click', () => {
                    const clickedDate = new Date(d.date);
                    const startOfDay = new Date(clickedDate.setHours(0, 0, 0, 0)).getTime() / 1000;
                    const endOfDay = new Date(clickedDate.setHours(23, 59, 59, 999)).getTime() / 1000;
                    setSelectedDateRange({ start: startOfDay, end: endOfDay });
                  });
                  
                  barsGroup.appendChild(barGroup);
                  
                  // Date labels
                  let showLabel = false;
                  if (chartTimeRange() <= 7) {
                    showLabel = true;
                  } else if (chartTimeRange() <= 30) {
                    showLabel = i % Math.ceil(data.length / 10) === 0 || i === data.length - 1;
                  } else if (chartTimeRange() <= 90) {
                    const dayOfWeek = new Date(d.date).getDay();
                    showLabel = dayOfWeek === 0 || i === 0 || i === data.length - 1;
                  } else {
                    const date = new Date(d.date);
                    showLabel = date.getDate() === 1 || i === 0 || i === data.length - 1;
                  }
                  
                  if (showLabel) {
                    const text = document.createElementNS('http://www.w3.org/2000/svg', 'text');
                    text.setAttribute('x', (x + barWidth / 2).toString());
                    text.setAttribute('y', (height + 20).toString());
                    text.setAttribute('text-anchor', 'middle');
                    text.setAttribute('class', 'text-[8px] fill-gray-500 dark:fill-gray-400');
                    
                    // Use shorter format for horizontal labels
                    const date = new Date(d.date);
                    let labelText;
                    if (chartTimeRange() <= 7) {
                      // For 7 days, show month/day
                      labelText = `${date.getMonth() + 1}/${date.getDate()}`;
                    } else if (chartTimeRange() <= 30) {
                      // For 30 days, show day only (or month/day for first of month)
                      labelText = date.getDate() === 1 ? `${date.getMonth() + 1}/1` : date.getDate().toString();
                    } else {
                      // For longer ranges, show month/day
                      labelText = `${date.getMonth() + 1}/${date.getDate()}`;
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
              <span class="font-medium text-green-600 dark:text-green-400">{dedupFactor()}</span>
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
      </div>
      </Show>

      {/* Backups Filter */}
      <BackupsFilter
        search={searchTerm}
        setSearch={setSearchTerm}
        viewMode={uiBackupTypeFilter}
        setViewMode={setUiBackupTypeFilter}
        groupBy={groupByMode}
        setGroupBy={setGroupByMode}
        searchInputRef={(el) => searchInputRef = el}
        typeFilter={typeFilter}
        setTypeFilter={setTypeFilter}
        hasHostBackups={hasHostBackups}
      />

      {/* Table */}
      <div class="mb-4 bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700">
        <div class="overflow-x-auto" style="scrollbar-width: none; -ms-overflow-style: none;">
        <style>{`
          .overflow-x-auto::-webkit-scrollbar { display: none; }
          .backup-table {
            table-layout: fixed;
            width: 100%;
            min-width: 1200px;
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
            <div class="text-center py-8 text-gray-500 dark:text-gray-400">
              <div class="flex flex-col items-center gap-4">
                <div class="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-500"></div>
                <p class="text-lg">Loading backup data...</p>
                <p class="text-sm">This may take up to 20 seconds on first load</p>
              </div>
            </div>
          }
        >
          <Show
            when={groupedData().length > 0}
            fallback={
              <div class="text-center py-8 text-gray-500 dark:text-gray-400">
                <p class="text-lg">No backups found</p>
                <p class="text-sm mt-2">No backups, snapshots, or remote backups match your filters</p>
              </div>
            }
          >
          {/* Mobile Card View - Compact */}
          <div class="block lg:hidden space-y-3">
            <For each={groupedData()}>
              {(group) => (
                <div class="space-y-1">
                  <div class="text-xs font-medium text-gray-600 dark:text-gray-400 px-2 py-1 sticky top-0 bg-gray-50 dark:bg-gray-900 z-10">
                    {group.label} ({group.items.length})
                  </div>
                  <For each={group.items}>
                    {(item) => (
                      <div class="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded p-2 hover:shadow-sm transition-shadow">
                        {/* Compact header row */}
                        <div class="flex items-center justify-between gap-2 mb-1">
                          <div class="flex items-center gap-2 min-w-0 flex-1">
                            <span class={`inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-medium shrink-0 ${
                              item.type === 'VM'
                                ? 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200'
                                : 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200'
                            }`}>
                              {item.type}
                            </span>
                            <span class="text-xs text-gray-500 shrink-0">{item.vmid}</span>
                            <span class="font-medium text-xs truncate">{item.name || 'Unnamed'}</span>
                          </div>
                          <div class="flex items-center gap-2 shrink-0">
                            <span class={`inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-medium ${
                              item.backupType === 'snapshot'
                                ? 'bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-200'
                                : item.backupType === 'local'
                                ? 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200'
                                : 'bg-cyan-100 text-cyan-800 dark:bg-cyan-900 dark:text-cyan-200'
                            }`}>
                              {item.backupType === 'snapshot' ? 'SNAP' :
                               item.backupType === 'local' ? 'PVE' : 'PBS'}
                            </span>
                          </div>
                        </div>
                        
                        {/* Compact info row */}
                        <div class="flex items-center justify-between gap-2 text-[11px]">
                          <div class="flex items-center gap-3 text-gray-600 dark:text-gray-400">
                            <span>{item.node}</span>
                            <span class={getAgeColorClass(item.backupTime)}>
                              {formatTime(item.backupTime * 1000)}
                            </span>
                            <Show when={item.size}>
                              <span class={getSizeColor(item.size)}>
                                {formatBytes(item.size!)}
                              </span>
                            </Show>
                            <Show when={item.backupType === 'remote' && item.verified}>
                              <svg class="w-4 h-4 text-green-600 dark:text-green-400 inline" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
                              </svg>
                            </Show>
                          </div>
                          <Show when={(item.storage || item.datastore) && item.backupType !== 'snapshot'}>
                            <span class="text-gray-500 dark:text-gray-400 text-[10px] truncate max-w-[100px]">
                              {item.storage || (item.datastore && (
                                item.namespace && item.namespace !== 'root'
                                  ? `${item.datastore}/${item.namespace}`
                                  : item.datastore
                              )) || '-'}
                            </span>
                          </Show>
                        </div>
                      </div>
                    )}
                  </For>
                </div>
              )}
            </For>
          </div>
          
          {/* Desktop Table View */}
          <table class="backup-table hidden lg:table">
            <thead>
              <tr class="bg-gray-50 dark:bg-gray-700/50 text-gray-600 dark:text-gray-300 border-b border-gray-200 dark:border-gray-600">
                <th
                  class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600"
                  onClick={() => handleSort('vmid')}
                  style="width: 60px;"
                >
                  VMID {sortKey() === 'vmid' && (sortDirection() === 'asc' ? '▲' : '▼')}
                </th>
                <th
                  class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600"
                  onClick={() => handleSort('type')}
                  style="width: 60px;"
                >
                  Type {sortKey() === 'type' && (sortDirection() === 'asc' ? '▲' : '▼')}
                </th>
                <th 
                  class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600"
                  onClick={() => handleSort('name')}
                  style="width: 150px;"
                >
                  Name {sortKey() === 'name' && (sortDirection() === 'asc' ? '▲' : '▼')}
                </th>
                <th
                  class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600"
                  onClick={() => handleSort('node')}
                  style="width: 100px;"
                >
                  Node {sortKey() === 'node' && (sortDirection() === 'asc' ? '▲' : '▼')}
                </th>
                <Show when={backupTypeFilter() === 'all' || backupTypeFilter() === 'remote'}>
                  <th
                    class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600"
                    onClick={() => handleSort('owner')}
                    style="width: 80px;"
                  >
                    Owner {sortKey() === 'owner' && (sortDirection() === 'asc' ? '▲' : '▼')}
                  </th>
                </Show>
                <th
                  class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600"
                  onClick={() => handleSort('backupTime')}
                  style="width: 140px;"
                >
                  Time {sortKey() === 'backupTime' && (sortDirection() === 'asc' ? '▲' : '▼')}
                </th>
                <Show when={backupTypeFilter() !== 'snapshot'}>
                  <th
                    class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600"
                    onClick={() => handleSort('size')}
                    style="width: 80px;"
                  >
                    Size {sortKey() === 'size' && (sortDirection() === 'asc' ? '▲' : '▼')}
                  </th>
                </Show>
                <th
                  class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600"
                  onClick={() => handleSort('backupType')}
                  style="width: 80px;"
                >
                  Backup {sortKey() === 'backupType' && (sortDirection() === 'asc' ? '▲' : '▼')}
                </th>
                <Show when={backupTypeFilter() !== 'snapshot'}>
                  <th 
                    class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600"
                    onClick={() => handleSort('storage')}
                    style="width: 150px;"
                  >
                    Location {sortKey() === 'storage' && (sortDirection() === 'asc' ? '▲' : '▼')}
                  </th>
                </Show>
                <Show when={backupTypeFilter() === 'all' || backupTypeFilter() === 'remote'}>
                  <th 
                    class="px-2 py-1.5 text-center text-[11px] sm:text-xs font-medium uppercase tracking-wider cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600"
                    onClick={() => handleSort('verified')}
                    style="width: 60px;"
                  >
                    Verified {sortKey() === 'verified' && (sortDirection() === 'asc' ? '▲' : '▼')}
                  </th>
                </Show>
                <th class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider" style="width: 200px;">
                  Details
                </th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
              <For each={groupedData()}>
                {(group) => (
                  <>
                    <tr class="bg-gray-50/50 dark:bg-gray-700/30">
                      <td colspan={(() => {
                        let cols = 7; // Base columns: VMID, Type, Name, Node, Time, Backup, Details
                        if (backupTypeFilter() === 'all' || backupTypeFilter() === 'remote') cols++; // Add Owner column
                        if (backupTypeFilter() !== 'snapshot') cols++; // Add Size column
                        if (backupTypeFilter() === 'all' || backupTypeFilter() === 'remote') cols++; // Add Verified column
                        if (backupTypeFilter() !== 'snapshot') cols++; // Add Location column
                        return cols;
                      })()} class="p-0.5 px-1.5 text-xs font-medium text-gray-600 dark:text-gray-400 whitespace-nowrap">
                        {group.label} ({group.items.length})
                      </td>
                    </tr>
                    <For each={group.items}>
                      {(item) => (
                        <tr class="border-t border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700/30">
                          <td class="p-0.5 px-1.5 text-sm align-middle">{item.vmid}</td>
                          <td class="p-0.5 px-1.5 align-middle">
                            <span class={`inline-flex items-center px-1.5 py-0.5 rounded text-xs font-medium ${
                              item.type === 'VM'
                                ? 'bg-blue-100 text-blue-700 dark:bg-blue-900/50 dark:text-blue-300'
                                : item.type === 'Host'
                                ? 'bg-orange-100 text-orange-700 dark:bg-orange-900/50 dark:text-orange-300'
                                : 'bg-green-100 text-green-700 dark:bg-green-900/50 dark:text-green-300'
                            }`}>
                              {item.type}
                            </span>
                          </td>
                          <td class="p-0.5 px-1.5 text-sm align-middle">
                            {item.name || '-'}
                          </td>
                          <td class="p-0.5 px-1.5 text-sm align-middle">
                            {item.node}
                          </td>
                          <Show when={backupTypeFilter() === 'all' || backupTypeFilter() === 'remote'}>
                            <td class="p-0.5 px-1.5 text-xs align-middle text-gray-500 dark:text-gray-400">
                              {item.owner ? item.owner.split('@')[0] : '-'}
                            </td>
                          </Show>
                          <td class={`p-0.5 px-1.5 text-xs align-middle ${getAgeColorClass(item.backupTime)}`}>
                            {formatTime(item.backupTime * 1000)}
                          </td>
                          <Show when={backupTypeFilter() !== 'snapshot'}>
                            <td class={`p-0.5 px-1.5 align-middle ${getSizeColor(item.size)}`}>
                              {item.size ? formatBytes(item.size) : '-'}
                            </td>
                          </Show>
                          <td class="p-0.5 px-1.5 align-middle">
                            <div class="flex items-center gap-1">
                              <span class={`inline-flex items-center px-1.5 py-0.5 rounded text-xs font-medium ${
                                item.backupType === 'snapshot'
                                  ? 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/20 dark:text-yellow-300'
                                  : item.backupType === 'local'
                                  ? 'bg-orange-100 text-orange-700 dark:bg-orange-900/50 dark:text-orange-300'
                                  : 'bg-purple-100 text-purple-700 dark:bg-purple-900/50 dark:text-purple-300'
                              }`}>
                                {item.backupType === 'snapshot' ? 'Snapshot' : item.backupType === 'local' ? 'PVE' : 'PBS'}
                              </span>
                              <Show when={item.encrypted}>
                                <span title="Encrypted backup" class="text-green-600 dark:text-green-400 inline-block ml-1">
                                  <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" viewBox="0 0 20 20" fill="currentColor">
                                    <path fill-rule="evenodd" d="M5 9V7a5 5 0 0110 0v2a2 2 0 012 2v5a2 2 0 01-2 2H5a2 2 0 01-2-2v-5a2 2 0 012-2zm8-2v2H7V7a3 3 0 016 0z" clip-rule="evenodd" />
                                  </svg>
                                </span>
                              </Show>
                              <Show when={item.protected}>
                                <span title="Protected backup" class="text-blue-600 dark:text-blue-400 inline-block ml-1">
                                  <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" viewBox="0 0 20 20" fill="currentColor">
                                    <path fill-rule="evenodd" d="M2.166 4.999A11.954 11.954 0 0010 1.944 11.954 11.954 0 0017.834 5c.11.65.166 1.32.166 2.001 0 5.225-3.34 9.67-8 11.317C5.34 16.67 2 12.225 2 7c0-.682.057-1.35.166-2.001zm11.541 3.708a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd" />
                                  </svg>
                                </span>
                              </Show>
                            </div>
                          </td>
                          <Show when={backupTypeFilter() !== 'snapshot'}>
                            <td class="p-0.5 px-1.5 text-sm align-middle">
                              {item.storage || (item.datastore && (
                                item.namespace && item.namespace !== 'root'
                                  ? `${item.datastore}/${item.namespace}`
                                  : item.datastore
                              )) || '-'}
                            </td>
                          </Show>
                          <Show when={backupTypeFilter() === 'all' || backupTypeFilter() === 'remote'}>
                            <td class="p-0.5 px-1.5 text-center align-middle">
                              {item.backupType === 'remote' ? (
                                item.verified ? (
                                  <span title="PBS backup verified">
                                    <svg class="w-4 h-4 text-green-500 dark:text-green-400 inline" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
                                    </svg>
                                  </span>
                                ) : (
                                  <span title="PBS backup not yet verified">
                                    <svg class="w-4 h-4 text-gray-400 dark:text-gray-500 inline" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                                    </svg>
                                  </span>
                                )
                              ) : (
                                <span class="text-gray-400 dark:text-gray-500" title="Verification only available for PBS backups">-</span>
                              )}
                            </td>
                          </Show>
                          <td 
                            class="p-0.5 px-1.5 cursor-help align-middle"
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
                                const pbsDescription = item.description || (item.name && item.name !== '-' ? item.name : '');
                                if (pbsDescription && pbsDescription.trim()) {
                                  details.push(pbsDescription);
                                }
                              }
                              
                              const fullText = details.join(' • ') || '-';
                              if (fullText.length > 35) {
                                const rect = e.currentTarget.getBoundingClientRect();
                                setTooltip({
                                  text: fullText,
                                  x: rect.left,
                                  y: rect.top - 5
                                });
                              }
                            }}
                            onMouseLeave={() => {
                              setTooltip(null);
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
                                const pbsDescription = item.description || (item.name && item.name !== '-' ? item.name : '');
                                if (pbsDescription && pbsDescription.trim()) {
                                  details.push(pbsDescription);
                                }
                              }
                              
                              const fullText = details.join(' • ') || '-';
                              const displayText = fullText.length > 35 ? fullText.substring(0, 32) + '...' : fullText;
                              
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
          </Show>
        </Show>
        </div>
      </div>

      {/* Tooltip */}
      <Show when={tooltip()}>
        <div
          class="fixed z-[9999] px-3 py-2 text-sm bg-black text-white rounded-lg shadow-xl pointer-events-none"
          style={{
            left: `${tooltip()!.x - 75}px`,
            top: `${tooltip()!.y - 35}px`,
            "max-width": "200px",
            "white-space": "pre-line",
            "font-family": "system-ui, -apple-system, sans-serif"
          }}
        >
          {tooltip()!.text}
        </div>
      </Show>
      </Show>
    </div>
  );
};

export default UnifiedBackups;