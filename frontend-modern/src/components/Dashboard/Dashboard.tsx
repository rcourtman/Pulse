import { createSignal, createMemo, createEffect, For, Show } from 'solid-js';
import type { VM, Container, Node } from '@/types/api';
import { GuestRow } from './GuestRow';
import NodeCard from './NodeCard';
import CompactNodeCard from './CompactNodeCard';
import { useWebSocket } from '@/App';
import { getAlertStyles } from '@/utils/alerts';
import { createTooltipSystem, showTooltip, hideTooltip } from '@/components/shared/Tooltip';
import { ComponentErrorBoundary } from '@/components/ErrorBoundary';
import { ScrollableTable } from '@/components/shared/ScrollableTable';
import { parseFilterStack, evaluateFilterStack } from '@/utils/searchQuery';

interface DashboardProps {
  vms: VM[];
  containers: Container[];
  nodes: Node[];
}

type ViewMode = 'all' | 'vm' | 'lxc';
type StatusMode = 'all' | 'running' | 'stopped';


export function Dashboard(props: DashboardProps) {
  const { connected, activeAlerts, initialDataReceived } = useWebSocket();
  const [search, setSearch] = createSignal('');
  const [isSearchLocked, setIsSearchLocked] = createSignal(false);
  
  // Initialize from localStorage with proper type checking
  const storedViewMode = localStorage.getItem('dashboardViewMode');
  const [viewMode, setViewMode] = createSignal<ViewMode>(
    (storedViewMode === 'all' || storedViewMode === 'vm' || storedViewMode === 'lxc') ? storedViewMode : 'all'
  );
  
  const storedStatusMode = localStorage.getItem('dashboardStatusMode');
  const [statusMode, setStatusMode] = createSignal<StatusMode>(
    (storedStatusMode === 'all' || storedStatusMode === 'running' || storedStatusMode === 'stopped') ? storedStatusMode : 'all'
  );
  
  const [showFilters, setShowFilters] = createSignal(
    localStorage.getItem('dashboardShowFilters') !== null 
      ? localStorage.getItem('dashboardShowFilters') === 'true'
      : false // Default to collapsed
  );
  
  // Sorting state - default to VMID ascending (matches Proxmox order)
  const [sortKey, setSortKey] = createSignal<keyof (VM | Container) | null>('vmid');
  const [sortDirection, setSortDirection] = createSignal<'asc' | 'desc'>('asc');
  
  // Create tooltip system
  const TooltipComponent = createTooltipSystem();
  
  
  
  // Create a mapping from node name to host URL
  const nodeHostMap = createMemo(() => {
    const map: Record<string, string> = {};
    props.nodes.forEach(node => {
      if (node.host) {
        map[node.name] = node.host;
      }
    });
    return map;
  });
  
  // Persist filter states to localStorage
  createEffect(() => {
    localStorage.setItem('dashboardViewMode', viewMode());
  });
  
  createEffect(() => {
    localStorage.setItem('dashboardStatusMode', statusMode());
  });
  
  
  
  createEffect(() => {
    localStorage.setItem('dashboardShowFilters', showFilters().toString());
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
      if (key === 'cpu' || key === 'memory' || key === 'disk' || key === 'diskRead' || 
          key === 'diskWrite' || key === 'networkIn' || key === 'networkOut' || key === 'uptime') {
        setSortDirection('desc');
      } else {
        setSortDirection('asc');
      }
    }
  };

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
        // First check if we have search/filters to clear
        if (search().trim() || sortKey() !== 'vmid' || sortDirection() !== 'asc') {
          // Clear search and reset filters
          setSearch('');
          setIsSearchLocked(false);
          setSortKey('vmid');
          setSortDirection('asc');
          
          // Blur the search input if it's focused
          if (searchInputRef && document.activeElement === searchInputRef) {
            searchInputRef.blur();
          }
        } else if (showFilters()) {
          // No search/filters active, so collapse the filters section
          setShowFilters(false);
        }
        // If filters are already collapsed, do nothing
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
  

  // Combine VMs and containers into a single list
  const allGuests = createMemo(() => {
    const vms = props.vms || [];
    const containers = props.containers || [];
    const guests: (VM | Container)[] = [...vms, ...containers];
    return guests;
  });


  // Filter guests based on current settings
  const filteredGuests = createMemo(() => {
    let guests = allGuests();

    // Filter by type
    if (viewMode() === 'vm') {
      guests = guests.filter(g => g.type === 'qemu');
    } else if (viewMode() === 'lxc') {
      guests = guests.filter(g => g.type === 'lxc');
    }

    // Filter by status
    if (statusMode() === 'running') {
      guests = guests.filter(g => g.status === 'running');
    } else if (statusMode() === 'stopped') {
      guests = guests.filter(g => g.status !== 'running');
    }

    // Apply search/filter
    const searchTerm = search().trim();
    if (searchTerm) {
      // Split by commas first
      const searchParts = searchTerm.split(',').map(t => t.trim()).filter(t => t);
      
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
          guests = guests.filter(g => evaluateFilterStack(g, stack));
        }
      }
      
      // Apply text search if any
      if (textSearches.length > 0) {
        guests = guests.filter(g => 
          textSearches.some(term => 
            g.name.toLowerCase().includes(term) ||
            g.vmid.toString().includes(term) ||
            g.node.toLowerCase().includes(term) ||
            g.status.toLowerCase().includes(term)
          )
        );
      }
    }


    // Don't filter by thresholds anymore - dimming is handled in GuestRow component

    return guests;
  });

  // Group by node
  const groupedGuests = createMemo(() => {
    const guests = filteredGuests();
    
    const groups: Record<string, (VM | Container)[]> = {};
    guests.forEach(guest => {
      if (!groups[guest.node]) {
        groups[guest.node] = [];
      }
      groups[guest.node].push(guest);
    });

    // Sort within each node group
    const key = sortKey();
    const dir = sortDirection();
    if (key) {
      Object.keys(groups).forEach(node => {
        groups[node] = groups[node].sort((a, b) => {
          let aVal: string | number | boolean | null | undefined = a[key] as string | number | boolean | null | undefined;
          let bVal: string | number | boolean | null | undefined = b[key] as string | number | boolean | null | undefined;
          
          // Special handling for percentage-based columns
          if (key === 'cpu') {
            // CPU is displayed as percentage
            aVal = a.cpu * 100;
            bVal = b.cpu * 100;
          } else if (key === 'memory') {
            // Memory is displayed as percentage (use pre-calculated usage)
            aVal = a.memory ? (a.memory.usage || 0) : 0;
            bVal = b.memory ? (b.memory.usage || 0) : 0;
          } else if (key === 'disk') {
            // Disk is displayed as percentage
            aVal = a.disk.total > 0 ? (a.disk.used / a.disk.total) * 100 : 0;
            bVal = b.disk.total > 0 ? (b.disk.used / b.disk.total) * 100 : 0;
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
    const running = guests.filter(g => g.status === 'running').length;
    const vms = guests.filter(g => g.type === 'qemu').length;
    const containers = guests.filter(g => g.type === 'lxc').length;
    return {
      total: guests.length,
      running,
      stopped: guests.length - running,
      vms,
      containers
    };
  });


  return (
    <div>
      {/* Node Summary Cards - Adaptive Layout */}
      <div id="node-summary-cards-container" class="mb-3">
        <div>
        <Show when={props.nodes.length > 0}>
          {/* Regular cards for 1-4 nodes */}
          <Show when={props.nodes.length <= 4}>
            <div class="flex flex-wrap gap-2">
              <For each={props.nodes}>
                {(node) => (
                  <div 
                    class="flex-1 min-w-[250px] cursor-pointer transition-transform hover:scale-[1.02]"
                    onClick={() => {
                      const currentSearch = search();
                      const nodeFilter = `node:${node.name}`;
                      
                      // Check if this node filter is already in the search
                      if (currentSearch.includes(nodeFilter)) {
                        // Remove the node filter
                        setSearch(currentSearch.replace(nodeFilter, '').trim().replace(/,\s*,/g, ',').replace(/^,|,$/g, ''));
                        setIsSearchLocked(false);
                      } else {
                        // Clear any existing node: filters and add the new one
                        const cleanedSearch = currentSearch.replace(/node:\w+/g, '').trim().replace(/,\s*,/g, ',').replace(/^,|,$/g, '');
                        const newSearch = cleanedSearch ? `${cleanedSearch}, ${nodeFilter}` : nodeFilter;
                        setSearch(newSearch);
                        setIsSearchLocked(true);
                        
                        // Expand filters if collapsed
                        if (!showFilters()) {
                          setShowFilters(true);
                        }
                      }
                    }}
                  >
                    <ComponentErrorBoundary name="NodeCard">
                      <NodeCard 
                        node={node} 
                        isSelected={search().includes(`node:${node.name}`)}
                      />
                    </ComponentErrorBoundary>
                  </div>
                )}
              </For>
            </div>
          </Show>

          {/* Compact cards for 5-9 nodes */}
          <Show when={props.nodes.length > 4 && props.nodes.length <= 9}>
            <div class="grid grid-cols-2 lg:grid-cols-3 gap-2">
              <For each={props.nodes}>
                {(node) => (
                  <ComponentErrorBoundary name="CompactNodeCard">
                    <CompactNodeCard 
                      node={node} 
                      variant="compact"
                      isSelected={search().includes(`node:${node.name}`)}
                      onClick={() => {
                        const currentSearch = search();
                        const nodeFilter = `node:${node.name}`;
                        
                        // Check if this node filter is already in the search
                        if (currentSearch.includes(nodeFilter)) {
                          // Remove the node filter
                          setSearch(currentSearch.replace(nodeFilter, '').trim().replace(/,\s*,/g, ',').replace(/^,|,$/g, ''));
                          setIsSearchLocked(false);
                        } else {
                          // Clear any existing node: filters and add the new one
                          const cleanedSearch = currentSearch.replace(/node:\w+/g, '').trim().replace(/,\s*,/g, ',').replace(/^,|,$/g, '');
                          const newSearch = cleanedSearch ? `${cleanedSearch}, ${nodeFilter}` : nodeFilter;
                          setSearch(newSearch);
                          setIsSearchLocked(true);
                          
                          // Expand filters if collapsed
                          if (!showFilters()) {
                            setShowFilters(true);
                          }
                        }
                      }}
                    />
                  </ComponentErrorBoundary>
                )}
              </For>
            </div>
          </Show>

          {/* Ultra-compact list for 10+ nodes */}
          <Show when={props.nodes.length >= 10}>
            <div class="space-y-1">
              <For each={props.nodes}>
                {(node) => (
                  <ComponentErrorBoundary name="CompactNodeCard">
                    <CompactNodeCard 
                      node={node} 
                      variant="ultra-compact"
                      isSelected={search().includes(`node:${node.name}`)}
                      onClick={() => {
                        const currentSearch = search();
                        const nodeFilter = `node:${node.name}`;
                        
                        // Check if this node filter is already in the search
                        if (currentSearch.includes(nodeFilter)) {
                          // Remove the node filter
                          setSearch(currentSearch.replace(nodeFilter, '').trim().replace(/,\s*,/g, ',').replace(/^,|,$/g, ''));
                          setIsSearchLocked(false);
                        } else {
                          // Clear any existing node: filters and add the new one
                          const cleanedSearch = currentSearch.replace(/node:\w+/g, '').trim().replace(/,\s*,/g, ',').replace(/^,|,$/g, '');
                          const newSearch = cleanedSearch ? `${cleanedSearch}, ${nodeFilter}` : nodeFilter;
                          setSearch(newSearch);
                          setIsSearchLocked(true);
                          
                          // Expand filters if collapsed
                          if (!showFilters()) {
                            setShowFilters(true);
                          }
                        }
                      }}
                    />
                  </ComponentErrorBoundary>
                )}
              </For>
            </div>
          </Show>
        </Show>
        </div>
      </div>
      
      {/* Dashboard Filter */}
      <div class="dashboard-filter mb-3 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg shadow-sm">
        {/* Filter toggle - now visible on all screen sizes */}
        <button type="button"
          onClick={() => setShowFilters(!showFilters())}
          class="w-full flex items-center justify-between px-4 py-3 text-sm font-medium text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700/50 rounded-lg transition-colors"
        >
          <span class="flex items-center gap-2">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <line x1="4" y1="21" x2="4" y2="14"></line>
              <line x1="4" y1="10" x2="4" y2="3"></line>
              <line x1="12" y1="21" x2="12" y2="12"></line>
              <line x1="12" y1="8" x2="12" y2="3"></line>
              <line x1="20" y1="21" x2="20" y2="16"></line>
              <line x1="20" y1="12" x2="20" y2="3"></line>
              <line x1="1" y1="14" x2="7" y2="14"></line>
              <line x1="9" y1="8" x2="15" y2="8"></line>
              <line x1="17" y1="16" x2="23" y2="16"></line>
            </svg>
            Filters & Search
            <Show when={search() || viewMode() !== 'all' || statusMode() !== 'all'}>
              <span class="text-xs bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300 px-2 py-0.5 rounded-full font-medium">
                Active
              </span>
            </Show>
          </span>
          <svg 
            width="16" 
            height="16" 
            viewBox="0 0 24 24" 
            fill="none" 
            stroke="currentColor" 
            stroke-width="2"
            class={`transform transition-transform ${showFilters() ? 'rotate-180' : ''}`}
          >
            <polyline points="6 9 12 15 18 9"></polyline>
          </svg>
        </button>
        
        <div class={`filter-controls-wrapper ${showFilters() ? 'block' : 'hidden'} p-3 lg:p-4 border-t border-gray-200 dark:border-gray-700`}>
          <div class="flex flex-col gap-3">
            {/* Search Bar Row */}
            <div class="flex gap-2">
              <div class="relative flex-1">
                <input
                  ref={searchInputRef}
                  type="text"
                  placeholder="Search: name, jellyfin,plex, or cpu>80"
                  value={search()}
                  onInput={(e) => {
                    if (!isSearchLocked()) {
                      setSearch(e.currentTarget.value);
                    }
                  }}
                  disabled={isSearchLocked()}
                  class={`w-full pl-9 pr-9 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg 
                         bg-white dark:bg-gray-900 text-gray-800 dark:text-gray-200 placeholder-gray-400 dark:placeholder-gray-500
                         focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500 dark:focus:border-blue-400 outline-none transition-all
                         ${isSearchLocked() ? 'opacity-60 cursor-not-allowed' : ''}`}
                  title="Search guests or use filters like cpu>80"
                />
                <svg class="absolute left-3 top-2.5 h-4 w-4 text-gray-400 dark:text-gray-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
                </svg>
                <button type="button"
                  class="absolute right-3 top-2.5 text-gray-400 dark:text-gray-500 hover:text-gray-600 dark:hover:text-gray-300 transition-colors"
                  onMouseEnter={(e) => {
                    const rect = e.currentTarget.getBoundingClientRect();
                    const tooltipContent = `
                      <div class="space-y-2 p-1">
                        <div class="font-semibold mb-2">Search Examples:</div>
                        <div class="space-y-1">
                          <div><span class="font-mono bg-gray-700 px-1 rounded">jellyfin</span> - Find guests with "jellyfin" in name</div>
                          <div><span class="font-mono bg-gray-700 px-1 rounded">plex,media</span> - Find guests with "plex" OR "media"</div>
                          <div><span class="font-mono bg-gray-700 px-1 rounded">cpu>80</span> - Guests using >80% CPU</div>
                          <div><span class="font-mono bg-gray-700 px-1 rounded">memory<20</span> - Guests using <20% memory</div>
                          <div><span class="font-mono bg-gray-700 px-1 rounded">disk>90</span> - Guests using >90% disk</div>
                          <div><span class="font-mono bg-gray-700 px-1 rounded">node:pve1</span> - Guests on specific node</div>
                          <div><span class="font-mono bg-gray-700 px-1 rounded">vmid:104</span> - Find specific VM/container</div>
                        </div>
                        <div class="mt-2 pt-2 border-t border-gray-600">
                          <div class="font-semibold mb-1">Combine searches:</div>
                          <div><span class="font-mono bg-gray-700 px-1 rounded">media,cpu>50</span> - "media" in name AND >50% CPU</div>
                          <div><span class="font-mono bg-gray-700 px-1 rounded">plex,jellyfin,disk>80</span> - Multiple names AND disk filter</div>
                        </div>
                      </div>
                    `;
                    showTooltip(tooltipContent, rect.left, rect.top);
                  }}
                  onMouseLeave={() => hideTooltip()}
                  onClick={(e) => e.preventDefault()}
                  aria-label="Search help"
                >
                  <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                  </svg>
                </button>
              </div>
              
              {/* Reset Button */}
              <button 
                onClick={() => {
                  setSearch('');
                  setIsSearchLocked(false);
                  setSortKey('vmid');
                  setSortDirection('asc');
                  setViewMode('all');
                  setStatusMode('all');
                }}
                title="Reset all filters (Esc)"
                class="flex items-center justify-center px-3 py-2 text-sm font-medium text-gray-600 dark:text-gray-400 
                       bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600 
                       rounded-lg transition-colors"
              >
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                  <path d="M3 12a9 9 0 0 1 9-9 9.75 9.75 0 0 1 6.74 2.74L21 8"/>
                  <path d="M21 3v5h-5"/>
                  <path d="M21 12a9 9 0 0 1-9 9 9.75 9.75 0 0 1-6.74-2.74L3 16"/>
                  <path d="M8 16H3v5"/>
                </svg>
                <span class="ml-1.5 hidden sm:inline">Reset</span>
              </button>
            </div>
            

            {/* Filters Row */}
            <div class="flex flex-col sm:flex-row gap-2">

              {/* Type Filter */}
              <div class="inline-flex rounded-lg bg-gray-100 dark:bg-gray-700 p-0.5">
                <button type="button"
                  onClick={() => setViewMode('all')}
                  class={`px-3 py-1.5 text-xs font-medium rounded-md transition-all ${
                    viewMode() === 'all'
                      ? 'bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 shadow-sm' 
                      : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
                  }`}
                >
                  All
                </button>
                <button type="button"
                  onClick={() => setViewMode('vm')}
                  class={`px-3 py-1.5 text-xs font-medium rounded-md transition-all ${
                    viewMode() === 'vm'
                      ? 'bg-white dark:bg-gray-800 text-blue-600 dark:text-blue-400 shadow-sm' 
                      : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
                  }`}
                >
                  VMs
                </button>
                <button type="button"
                  onClick={() => setViewMode('lxc')}
                  class={`px-3 py-1.5 text-xs font-medium rounded-md transition-all ${
                    viewMode() === 'lxc'
                      ? 'bg-white dark:bg-gray-800 text-green-600 dark:text-green-400 shadow-sm' 
                      : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
                  }`}
                >
                  LXCs
                </button>
              </div>

              <div class="h-6 w-px bg-gray-200 dark:bg-gray-600 hidden sm:block"></div>

              {/* Status Filter */}
              <div class="inline-flex rounded-lg bg-gray-100 dark:bg-gray-700 p-0.5">
                <button type="button"
                  onClick={() => setStatusMode('all')}
                  class={`px-3 py-1.5 text-xs font-medium rounded-md transition-all ${
                    statusMode() === 'all'
                      ? 'bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 shadow-sm' 
                      : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
                  }`}
                >
                  All
                </button>
                <button type="button"
                  onClick={() => setStatusMode('running')}
                  class={`px-3 py-1.5 text-xs font-medium rounded-md transition-all ${
                    statusMode() === 'running'
                      ? 'bg-white dark:bg-gray-800 text-green-600 dark:text-green-400 shadow-sm' 
                      : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
                  }`}
                >
                  Running
                </button>
                <button type="button"
                  onClick={() => setStatusMode('stopped')}
                  class={`px-3 py-1.5 text-xs font-medium rounded-md transition-all ${
                    statusMode() === 'stopped'
                      ? 'bg-white dark:bg-gray-800 text-red-600 dark:text-red-400 shadow-sm' 
                      : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
                  }`}
                >
                  Stopped
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>



      {/* Loading State */}
      <Show when={connected() && !initialDataReceived()}>
        <div class="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg p-8">
          <div class="text-center">
            <svg class="animate-spin mx-auto h-12 w-12 text-gray-400 mb-4" fill="none" viewBox="0 0 24 24">
              <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
              <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
            </svg>
            <h3 class="text-sm font-medium text-gray-900 dark:text-gray-100 mb-2">Loading dashboard data...</h3>
            <p class="text-xs text-gray-600 dark:text-gray-400">Connecting to monitoring service</p>
          </div>
        </div>
      </Show>

      {/* Empty State - No PVE Nodes Configured */}
      <Show when={connected() && initialDataReceived() && props.nodes.filter(n => n.type === 'pve').length === 0 && props.vms.length === 0 && props.containers.length === 0}>
        <div class="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg p-8">
          <div class="text-center">
            <svg class="mx-auto h-12 w-12 text-gray-400 mb-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
            </svg>
            <h3 class="text-sm font-medium text-gray-900 dark:text-gray-100 mb-2">No Proxmox VE nodes configured</h3>
            <p class="text-xs text-gray-600 dark:text-gray-400 mb-4">Add a Proxmox VE node in the Settings tab to start monitoring your infrastructure.</p>
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

      {/* Disconnected State */}
      <Show when={!connected()}>
        <div class="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-600 rounded-lg p-8">
          <div class="text-center">
            <svg class="mx-auto h-12 w-12 text-red-400 mb-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
            <h3 class="text-sm font-medium text-red-800 dark:text-red-200 mb-2">Connection Lost</h3>
            <p class="text-xs text-red-700 dark:text-red-300">Unable to connect to the backend server. Attempting to reconnect...</p>
          </div>
        </div>
      </Show>

      {/* Table View */}
      <Show when={connected() && initialDataReceived() && (props.nodes.length > 0 || props.vms.length > 0 || props.containers.length > 0)}>
        <ComponentErrorBoundary name="Guest Table">
          <ScrollableTable 
            class="mb-2 border border-gray-200 dark:border-gray-700 rounded overflow-hidden"
            minWidth="900px"
          >
          <table class="w-full min-w-[900px] text-xs sm:text-sm table-fixed">
            <thead>
              <tr class="bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300 border-b border-gray-300 dark:border-gray-600">
                <th 
                  class="p-1 px-2 text-left text-[10px] sm:text-xs font-medium uppercase tracking-wider w-[200px] cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600 focus:outline-none focus:ring-2 focus:ring-inset focus:ring-blue-500"
                  onClick={() => handleSort('name')}
                  onKeyDown={(e) => e.key === 'Enter' && handleSort('name')}
                  tabindex="0"
                  role="button"
                  aria-label={`Sort by name ${sortKey() === 'name' ? (sortDirection() === 'asc' ? 'ascending' : 'descending') : ''}`}
                >
                  Name {sortKey() === 'name' && (sortDirection() === 'asc' ? '▲' : '▼')}
                </th>
                <th 
                  class="p-1 px-2 text-left text-[10px] sm:text-xs font-medium uppercase tracking-wider w-[60px] cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600"
                  onClick={() => handleSort('type')}
                >
                  Type {sortKey() === 'type' && (sortDirection() === 'asc' ? '▲' : '▼')}
                </th>
                <th 
                  class="p-1 px-2 text-left text-[10px] sm:text-xs font-medium uppercase tracking-wider w-[70px] cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600"
                  onClick={() => handleSort('vmid')}
                >
                  VMID {sortKey() === 'vmid' && (sortDirection() === 'asc' ? '▲' : '▼')}
                </th>
                <th 
                  class="p-1 px-2 text-left text-[10px] sm:text-xs font-medium uppercase tracking-wider w-[100px] cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600"
                  onClick={() => handleSort('uptime')}
                >
                  Uptime {sortKey() === 'uptime' && (sortDirection() === 'asc' ? '▲' : '▼')}
                </th>
                <th 
                  class="p-1 px-2 text-left text-[10px] sm:text-xs font-medium uppercase tracking-wider w-[140px] cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600"
                  onClick={() => handleSort('cpu')}
                >
                  CPU {sortKey() === 'cpu' && (sortDirection() === 'asc' ? '▲' : '▼')}
                </th>
                <th 
                  class="p-1 px-2 text-left text-[10px] sm:text-xs font-medium uppercase tracking-wider w-[140px] cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600"
                  onClick={() => handleSort('memory')}
                >
                  Memory {sortKey() === 'memory' && (sortDirection() === 'asc' ? '▲' : '▼')}
                </th>
                <th 
                  class="p-1 px-2 text-left text-[10px] sm:text-xs font-medium uppercase tracking-wider w-[140px] cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600"
                  onClick={() => handleSort('disk')}
                >
                  Disk {sortKey() === 'disk' && (sortDirection() === 'asc' ? '▲' : '▼')}
                </th>
                <th 
                  class="p-1 px-2 text-left text-[10px] sm:text-xs font-medium uppercase tracking-wider w-[90px] cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600"
                  onClick={() => handleSort('diskRead')}
                >
                  Disk Read {sortKey() === 'diskRead' && (sortDirection() === 'asc' ? '▲' : '▼')}
                </th>
                <th 
                  class="p-1 px-2 text-left text-[10px] sm:text-xs font-medium uppercase tracking-wider w-[90px] cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600"
                  onClick={() => handleSort('diskWrite')}
                >
                  Disk Write {sortKey() === 'diskWrite' && (sortDirection() === 'asc' ? '▲' : '▼')}
                </th>
                <th 
                  class="p-1 px-2 text-left text-[10px] sm:text-xs font-medium uppercase tracking-wider w-[90px] cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600"
                  onClick={() => handleSort('networkIn')}
                >
                  Net In {sortKey() === 'networkIn' && (sortDirection() === 'asc' ? '▲' : '▼')}
                </th>
                <th 
                  class="p-1 px-2 text-left text-[10px] sm:text-xs font-medium uppercase tracking-wider w-[90px] cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600"
                  onClick={() => handleSort('networkOut')}
                >
                  Net Out {sortKey() === 'networkOut' && (sortDirection() === 'asc' ? '▲' : '▼')}
                </th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
              <For each={Object.entries(groupedGuests()).sort(([a], [b]) => a.localeCompare(b))} fallback={<></>}>
                {([node, guests]) => (
                  <>
                    <Show when={node}>
                      <tr class="node-header bg-gray-50 dark:bg-gray-700/50 font-semibold text-gray-700 dark:text-gray-300 text-xs">
                        <td class="px-2 py-1 text-xs font-medium text-gray-500 dark:text-gray-400 w-[200px]">
                          <a 
                            href={nodeHostMap()[node] || (node.includes(':') ? `https://${node}` : `https://${node}:8006`)} 
                            target="_blank" 
                            rel="noopener noreferrer" 
                            class="text-gray-500 dark:text-gray-400 hover:text-blue-600 dark:hover:text-blue-400 transition-colors duration-150 cursor-pointer"
                            title={`Open ${node} web interface`}
                          >
                            {node}
                          </a>
                        </td>
                        <td class="px-2 py-1 text-xs font-medium text-gray-500 dark:text-gray-400 w-[60px]"></td>
                        <td class="px-2 py-1 text-xs font-medium text-gray-500 dark:text-gray-400 w-[70px]"></td>
                        <td class="px-2 py-1 text-xs font-medium text-gray-500 dark:text-gray-400 w-[100px]"></td>
                        <td class="px-2 py-1 text-xs font-medium text-gray-500 dark:text-gray-400 w-[140px]"></td>
                        <td class="px-2 py-1 text-xs font-medium text-gray-500 dark:text-gray-400 w-[140px]"></td>
                        <td class="px-2 py-1 text-xs font-medium text-gray-500 dark:text-gray-400 w-[140px]"></td>
                        <td class="px-2 py-1 text-xs font-medium text-gray-500 dark:text-gray-400 w-[90px]"></td>
                        <td class="px-2 py-1 text-xs font-medium text-gray-500 dark:text-gray-400 w-[90px]"></td>
                        <td class="px-2 py-1 text-xs font-medium text-gray-500 dark:text-gray-400 w-[90px]"></td>
                        <td class="px-2 py-1 text-xs font-medium text-gray-500 dark:text-gray-400 w-[90px]"></td>
                      </tr>
                    </Show>
                    <For each={guests} fallback={<></>}>
                      {(guest) => (
                        <ComponentErrorBoundary name="GuestRow">
                          {(() => {
                            const guestId = guest.id || `${guest.instance}-${guest.name}-${guest.vmid}`;
                            return (
                              <GuestRow 
                                guest={guest} 
                                showNode={false} 
                                alertStyles={getAlertStyles(guestId, activeAlerts)}
                              />
                            );
                          })()}
                        </ComponentErrorBoundary>
                      )}
                    </For>
                  </>
                )}
              </For>
            </tbody>
          </table>
        </ScrollableTable>
        </ComponentErrorBoundary>
      </Show>

      <Show when={connected() && initialDataReceived() && filteredGuests().length === 0 && props.nodes.filter(n => n.type === 'pve').length > 0}>
        <div class="text-center py-12 text-gray-500 dark:text-gray-400">
          <svg class="mx-auto h-12 w-12 text-gray-400 mb-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.172 16.172a4 4 0 015.656 0M9 10h.01M15 10h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
          <p class="mt-2">No guests found matching your filters</p>
        </div>
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
            <span class="h-2 w-2 bg-gray-400 rounded-full"></span>
            {totalStats().stopped} stopped
          </span>
        </div>
      </div>
      </Show>
      
      
      {/* Tooltip System */}
      <TooltipComponent />
    </div>
  );
}