import { Component, For, Show, createSignal, createMemo, createEffect } from 'solid-js';
import { useWebSocket } from '@/App';
import { getAlertStyles } from '@/utils/alerts';
import { formatBytes } from '@/utils/format';
import { createTooltipSystem } from '@/components/shared/Tooltip';
import type { Storage as StorageType } from '@/types/api';
import { ComponentErrorBoundary } from '@/components/ErrorBoundary';
import { UnifiedNodeSelector } from '@/components/shared/UnifiedNodeSelector';
import { StorageFilter } from './StorageFilter';


const Storage: Component = () => {
  const { state, connected, activeAlerts, initialDataReceived } = useWebSocket();
  const [viewMode, setViewMode] = createSignal<'node' | 'storage'>('node');
  const [searchTerm, setSearchTerm] = createSignal('');
  const [selectedNode, setSelectedNode] = createSignal<string | null>(null);
  // TODO: Implement sorting in sortedStorage function
  // const [sortKey, setSortKey] = createSignal('name');
  // const [sortDirection, setSortDirection] = createSignal<'asc' | 'desc'>('asc');
  
  // Create tooltip system
  const TooltipComponent = createTooltipSystem();
  
  // Create a mapping from node name to host URL
  const nodeHostMap = createMemo(() => {
    const map: Record<string, string> = {};
    (state.nodes || []).forEach(node => {
      if (node.host) {
        map[node.name] = node.host;
      }
    });
    return map;
  });
  
  // Load preferences from localStorage
  createEffect(() => {
    const savedViewMode = localStorage.getItem('storageViewMode');
    if (savedViewMode === 'storage') setViewMode('storage');
  });
  
  // Save preferences to localStorage
  createEffect(() => {
    localStorage.setItem('storageViewMode', viewMode());
  });
  
  
  // Filter storage - in storage view, filter out 0 capacity
  const filteredStorage = createMemo(() => {
    let storage = state.storage || [];
    
    // In storage view, filter out 0 capacity
    if (viewMode() === 'storage') {
      storage = storage.filter(s => s.total > 0);
    }
    
    return storage;
  });
  
  // Sort and filter storage
  const sortedStorage = createMemo(() => {
    let storage = [...filteredStorage()];
    
    // Apply node selection filter
    const nodeFilter = selectedNode();
    if (nodeFilter) {
      storage = storage.filter(s => s.node.toLowerCase() === nodeFilter.toLowerCase());
    }
    
    // Apply search filter
    const search = searchTerm().toLowerCase();
    if (search) {
      // Regular search
      storage = storage.filter(s => 
        s.name.toLowerCase().includes(search) ||
        s.node.toLowerCase().includes(search) ||
        s.type.toLowerCase().includes(search) ||
        s.content?.toLowerCase().includes(search) ||
        (s.status && s.status.toLowerCase().includes(search))
      );
    }
    
    // Always sort by name alphabetically for consistent order
    return storage.sort((a, b) => a.name.localeCompare(b.name));
  });
  
  // Group storage by node or storage
  const groupedStorage = createMemo(() => {
    const storage = sortedStorage();
    const mode = viewMode();
    
    if (mode === 'node') {
      const groups: Record<string, StorageType[]> = {};
      storage.forEach(s => {
        if (!groups[s.node]) groups[s.node] = [];
        groups[s.node].push(s);
      });
      return groups;
    } else {
      // Group by storage name - show all storage as-is for maximum compatibility
      const groups: Record<string, StorageType[]> = {};
      
      storage.forEach(s => {
        if (!groups[s.name]) groups[s.name] = [];
        groups[s.name].push(s);
      });
      
      return groups;
    }
  });
  
  const getProgressBarColor = (usage: number) => {
    // Match MetricBar component styling exactly - disk type thresholds
    if (usage >= 90) return 'bg-red-500/60 dark:bg-red-500/50';
    if (usage >= 80) return 'bg-yellow-500/60 dark:bg-yellow-500/50';
    return 'bg-green-500/60 dark:bg-green-500/50';
  };
  
  const resetFilters = () => {
    setSearchTerm('');
    setSelectedNode(null);
    setViewMode('node');
    // setSortKey('name');
    // setSortDirection('asc');
  };
  
  const getTotalByNode = (storages: StorageType[]) => {
    const totals = { used: 0, total: 0, free: 0 };
    storages.forEach(s => {
      totals.used += s.used || 0;
      totals.total += s.total || 0;
      totals.free += s.free || 0;
    });
    return totals;
  };
  
  const calculateOverallUsage = (storages: StorageType[]) => {
    const totals = getTotalByNode(storages);
    return totals.total > 0 ? (totals.used / totals.total * 100) : 0;
  };
  
  // Handle keyboard shortcuts
  let searchInputRef: HTMLInputElement | undefined;
  
  createEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      // Ignore if user is typing in an input
      const target = e.target as HTMLElement;
      const isInputField = target.tagName === 'INPUT' || 
                          target.tagName === 'TEXTAREA' || 
                          target.tagName === 'SELECT' ||
                          target.contentEditable === 'true';
      
      // Escape key behavior
      if (e.key === 'Escape') {
        // Clear search and reset filters
        if (searchTerm().trim() || selectedNode() || viewMode() !== 'node') {
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
    <div>
      {/* Node Selector */}
      <UnifiedNodeSelector 
        currentTab="storage" 
        onNodeSelect={handleNodeSelect}
        filteredStorage={sortedStorage()}
        searchTerm={searchTerm()}
      />
      
      {/* Storage Filter */}
      <StorageFilter
        search={searchTerm}
        setSearch={setSearchTerm}
        groupBy={viewMode}
        setGroupBy={setViewMode}
        setSortKey={() => {}}
        setSortDirection={() => {}}
        searchInputRef={(el) => searchInputRef = el}
      />
      
      {/* Loading State */}
      <Show when={connected() && !initialDataReceived()}>
        <div class="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg p-8">
          <div class="text-center">
            <svg class="animate-spin mx-auto h-12 w-12 text-gray-400 mb-4" fill="none" viewBox="0 0 24 24">
              <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
              <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
            </svg>
            <h3 class="text-sm font-medium text-gray-900 dark:text-gray-100 mb-2">Loading storage data...</h3>
            <p class="text-xs text-gray-600 dark:text-gray-400">Connecting to monitoring service</p>
          </div>
        </div>
      </Show>

      {/* Helpful hint for no PVE nodes but still show content */}
      <Show when={connected() && initialDataReceived() && (state.nodes || []).filter((n) => n.type === 'pve').length === 0 && sortedStorage().length === 0 && searchTerm().trim() === ''}>
        <div class="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg p-8">
          <div class="text-center">
            <svg class="mx-auto h-12 w-12 text-gray-400 mb-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
            </svg>
            <h3 class="text-sm font-medium text-gray-900 dark:text-gray-100 mb-2">No storage configured</h3>
            <p class="text-xs text-gray-600 dark:text-gray-400 mb-4">Add a Proxmox VE or PBS node in the Settings tab to start monitoring storage.</p>
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
      
      {/* No results found message */}
      <Show when={connected() && initialDataReceived() && sortedStorage().length === 0 && searchTerm().trim() !== ''}>
        <div class="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg p-8">
          <div class="text-center">
            <svg class="mx-auto h-12 w-12 text-gray-400 mb-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
            </svg>
            <h3 class="text-sm font-medium text-gray-900 dark:text-gray-100 mb-2">No storage found</h3>
            <p class="text-xs text-gray-600 dark:text-gray-400">No storage matches your search "{searchTerm()}"</p>
          </div>
        </div>
      </Show>
      
      {/* Storage Table - shows for both PVE and PBS storage */}
      <Show when={connected() && initialDataReceived() && sortedStorage().length > 0}>
        <ComponentErrorBoundary name="Storage Table">
          <div class="mb-4 bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700">
            <div class="overflow-x-auto" style="scrollbar-width: none; -ms-overflow-style: none;">
              <style>{`
                .overflow-x-auto::-webkit-scrollbar { display: none; }
              `}</style>
            <table class="w-full">
              <thead>
                <tr class="bg-gray-50 dark:bg-gray-700/50 text-gray-600 dark:text-gray-300 border-b border-gray-200 dark:border-gray-600">
                  <th class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider">Storage</th>
                  <Show when={viewMode() === 'node'}>
                    <th class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider hidden sm:table-cell">Node</th>
                  </Show>
                  <th class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider hidden md:table-cell">Type</th>
                  <th class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider hidden lg:table-cell">Content</th>
                  <th class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider hidden sm:table-cell">Status</th>
                  <Show when={viewMode() === 'node'}>
                    <th class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider hidden lg:table-cell">Shared</th>
                  </Show>
                  <th class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider w-[200px]">Usage</th>
                  <th class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider hidden sm:table-cell">Free</th>
                  <th class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider">Total</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
                <For each={Object.entries(groupedStorage()).sort(([a], [b]) => a.localeCompare(b))}>
                  {([groupName, storages]) => (
                    <>
                      {/* Group Header */}
                      <Show when={viewMode() === 'node'}>
                        <tr class="bg-gray-50/50 dark:bg-gray-700/30">
                          <td class="p-0.5 px-1.5 text-xs font-medium text-gray-600 dark:text-gray-400">
                            <a 
                              href={nodeHostMap()[groupName] || (groupName.includes(':') ? `https://${groupName}` : `https://${groupName}:8006`)} 
                              target="_blank" 
                              rel="noopener noreferrer" 
                              class="text-gray-600 dark:text-gray-400 hover:text-blue-600 dark:hover:text-blue-400 transition-colors duration-150 cursor-pointer"
                              title={`Open ${groupName} web interface`}
                            >
                              {groupName}
                            </a>
                          </td>
                          <td class="p-0.5 px-1.5 text-[10px] text-gray-500 dark:text-gray-400" colspan="8">
                            {getTotalByNode(storages).total > 0 && (
                              <span>
                                {formatBytes(getTotalByNode(storages).used)} / {formatBytes(getTotalByNode(storages).total)} ({calculateOverallUsage(storages).toFixed(1)}%)
                              </span>
                            )}
                          </td>
                        </tr>
                      </Show>
                      
                      {/* Storage Rows */}
                      <For each={storages} fallback={<></>}>
                        {(storage) => {
                          const usagePercent = storage.total > 0 ? (storage.used / storage.total * 100) : 0;
                          const isDisabled = storage.status !== 'available';
                          
                          const alertStyles = getAlertStyles(storage.id || `${storage.instance}-${storage.name}`, activeAlerts);
                          const alertBg = alertStyles.hasAlert 
                            ? (alertStyles.severity === 'critical' 
                              ? 'bg-red-50 dark:bg-red-950/30' 
                              : 'bg-yellow-50 dark:bg-yellow-950/20')
                            : '';
                          const rowClass = `${isDisabled ? 'opacity-60' : ''} ${alertBg} hover:shadow-sm transition-all duration-200`;
                          
                          const firstCellClass = alertStyles.hasAlert
                            ? (alertStyles.severity === 'critical'
                              ? 'p-0.5 px-1.5 border-l-4 border-l-red-500 dark:border-l-red-400'
                              : 'p-0.5 px-1.5 border-l-4 border-l-yellow-500 dark:border-l-yellow-400')
                            : 'p-0.5 px-1.5';
                          
                          return (
                            <tr class={`${rowClass} hover:bg-gray-50 dark:hover:bg-gray-700/30 transition-colors`}>
                              <td class={firstCellClass}>
                                <div class="flex items-center gap-2">
                                  <span class="text-sm font-medium text-gray-900 dark:text-gray-100">
                                    {storage.name}
                                  </span>
                                </div>
                              </td>
                              <Show when={viewMode() === 'node'}>
                                <td class="p-0.5 px-1.5 text-xs text-gray-600 dark:text-gray-400 hidden sm:table-cell">{storage.node}</td>
                              </Show>
                              <td class="p-0.5 px-1.5 hidden md:table-cell">
                                <span class={`inline-block px-1.5 py-0.5 text-[10px] font-medium rounded ${
                                  storage.type === 'dir' ? 'bg-gray-100 text-gray-700 dark:bg-gray-700 dark:text-gray-300' :
                                  storage.type === 'pbs' ? 'bg-blue-100 text-blue-700 dark:bg-blue-900/50 dark:text-blue-300' :
                                  'bg-purple-100 text-purple-700 dark:bg-purple-900/50 dark:text-purple-300'
                                }`}>
                                  {storage.type}
                                </span>
                              </td>
                              <td class="p-0.5 px-1.5 text-xs text-gray-600 dark:text-gray-400 hidden lg:table-cell">
                                {storage.content || '-'}
                              </td>
                              <td class="p-0.5 px-1.5 text-xs hidden sm:table-cell">
                                <span class={`${
                                  storage.status === 'available' ? 'text-green-600 dark:text-green-400' : 
                                  'text-red-600 dark:text-red-400'
                                }`}>
                                  {storage.status || 'unknown'}
                                </span>
                              </td>
                              <Show when={viewMode() === 'node'}>
                                <td class="p-0.5 px-1.5 text-xs text-gray-600 dark:text-gray-400 hidden lg:table-cell">
                                  {storage.shared ? '✓' : '-'}
                                </td>
                              </Show>
                              
                              <td class="p-0.5 px-1.5">
                                <div class="relative w-[200px] h-3.5 rounded overflow-hidden bg-gray-200 dark:bg-gray-600">
                                  <div 
                                    class={`absolute top-0 left-0 h-full ${getProgressBarColor(usagePercent)}`}
                                    style={{ width: `${usagePercent}%` }}
                                  />
                                  <span class="absolute inset-0 flex items-center justify-center text-[10px] font-medium text-gray-800 dark:text-gray-100 leading-none">
                                    <span class="whitespace-nowrap px-0.5">
                                      {usagePercent.toFixed(0)}% ({formatBytes(storage.used || 0)}/{formatBytes(storage.total || 0)})
                                    </span>
                                  </span>
                                </div>
                              </td>
                              <td class="p-0.5 px-1.5 text-xs hidden sm:table-cell">{formatBytes(storage.free || 0)}</td>
                              <td class="p-0.5 px-1.5 text-xs">{formatBytes(storage.total || 0)}</td>
                            </tr>
                          );
                        }}
                      </For>
                    </>
                  )}
                </For>
              </tbody>
            </table>
            </div>
          </div>
        </ComponentErrorBoundary>
      </Show>
      
      {/* Tooltip System */}
      <TooltipComponent />
    </div>
  );
};

export default Storage;