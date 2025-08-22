import { Component, For, Show, createSignal, createMemo, createEffect } from 'solid-js';
import { useWebSocket } from '@/App';
import { getAlertStyles } from '@/utils/alerts';
import { AlertIndicator, AlertCountBadge } from '@/components/shared/AlertIndicators';
import { formatBytes } from '@/utils/format';
import { createTooltipSystem } from '@/components/shared/Tooltip';
import type { Storage as StorageType } from '@/types/api';
import { ComponentErrorBoundary } from '@/components/ErrorBoundary';


const Storage: Component = () => {
  const { state, connected, activeAlerts, initialDataReceived } = useWebSocket();
  const [viewMode, setViewMode] = createSignal<'node' | 'storage'>('node');
  const [searchTerm, setSearchTerm] = createSignal('');
  const [showFilters, setShowFilters] = createSignal(
    localStorage.getItem('storageShowFilters') !== null 
      ? localStorage.getItem('storageShowFilters') === 'true'
      : false // Default to collapsed
  );
  
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
  
  createEffect(() => {
    localStorage.setItem('storageShowFilters', showFilters().toString());
  });
  
  // Filter storage - in storage view, filter out 0 capacity
  const filteredStorage = createMemo(() => {
    const storage = state.storage || [];
    if (viewMode() === 'storage') {
      return storage.filter((s) => s.total > 0);
    }
    return storage;
  });
  
  // Sort and filter storage
  const sortedStorage = createMemo(() => {
    let storage = [...filteredStorage()];
    
    // Apply search filter
    const search = searchTerm().toLowerCase();
    if (search) {
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
    // Match MetricBar component styling - use the same disk/generic logic
    if (usage >= 90) return 'bg-red-500/60 dark:bg-red-500/50';
    if (usage >= 80) return 'bg-yellow-500/60 dark:bg-yellow-500/50';
    if (usage >= 70) return 'bg-amber-500/60 dark:bg-amber-500/50';
    if (usage >= 60) return 'bg-yellow-500/60 dark:bg-yellow-500/50';
    return 'bg-emerald-500/60 dark:bg-emerald-500/50';
  };
  
  const resetFilters = () => {
    setSearchTerm('');
    setViewMode('node');
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
        // First check if we have search/filters to clear
        if (searchTerm().trim() || viewMode() !== 'node') {
          // Clear search and reset filters
          resetFilters();
          
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
  
  return (
    <div>
      
      {/* Filters and Search */}
      <div class="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg mb-4 overflow-hidden">
        <button type="button"
          onClick={() => setShowFilters(!showFilters())}
          class="w-full px-4 py-3 flex items-center justify-between hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors cursor-pointer"
        >
          <span class="flex items-center gap-2 text-sm font-medium text-gray-700 dark:text-gray-300">
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
            <Show when={searchTerm() || viewMode() !== 'node'}>
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
                  placeholder="Search by name, node, type, content, or status..."
                  value={searchTerm()}
                  onInput={(e) => setSearchTerm(e.currentTarget.value)}
                  class="w-full pl-9 pr-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg 
                         bg-white dark:bg-gray-900 text-gray-800 dark:text-gray-200 placeholder-gray-400 dark:placeholder-gray-500
                         focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500 dark:focus:border-blue-400 outline-none transition-all"
                />
                <svg class="absolute left-3 top-2.5 h-4 w-4 text-gray-400 dark:text-gray-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
                </svg>
              </div>
              
              {/* Reset Button */}
              <button 
                onClick={resetFilters}
                title="Reset all filters"
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
              {/* View Mode Toggle */}
              <div class="flex items-center gap-2">
                <span class="text-xs font-medium text-gray-600 dark:text-gray-400">Group by:</span>
                <div class="inline-flex rounded-lg bg-gray-100 dark:bg-gray-700 p-0.5">
                  <button type="button"
                    onClick={() => setViewMode('node')}
                    class={`px-3 py-1.5 text-xs font-medium rounded-md transition-all ${
                      viewMode() === 'node'
                        ? 'bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 shadow-sm'
                        : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
                    }`}
                  >
                    Node
                  </button>
                  <button type="button"
                    onClick={() => setViewMode('storage')}
                    class={`px-3 py-1.5 text-xs font-medium rounded-md transition-all ${
                      viewMode() === 'storage'
                        ? 'bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 shadow-sm'
                        : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
                    }`}
                  >
                    Storage
                  </button>
                </div>
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
            <h3 class="text-sm font-medium text-gray-900 dark:text-gray-100 mb-2">Loading storage data...</h3>
            <p class="text-xs text-gray-600 dark:text-gray-400">Connecting to monitoring service</p>
          </div>
        </div>
      </Show>

      {/* Helpful hint for no PVE nodes but still show content */}
      <Show when={connected() && initialDataReceived() && (state.nodes || []).filter((n) => n.type === 'pve').length === 0 && sortedStorage().length === 0}>
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
      
      {/* Storage Table - shows for both PVE and PBS storage */}
      <Show when={connected() && initialDataReceived() && sortedStorage().length > 0}>
        <ComponentErrorBoundary name="Storage Table">
          <div class="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded overflow-x-auto">
            <table class="w-full text-xs">
              <thead>
                <tr class="bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300 border-b border-gray-300 dark:border-gray-600">
                  <th class="p-1 px-2 text-left text-[10px] sm:text-xs font-medium uppercase tracking-wider">Storage</th>
                  <Show when={viewMode() === 'node'}>
                    <th class="p-1 px-2 text-left text-[10px] sm:text-xs font-medium uppercase tracking-wider hidden sm:table-cell">Node</th>
                  </Show>
                  <th class="p-1 px-2 text-left text-[10px] sm:text-xs font-medium uppercase tracking-wider hidden md:table-cell">Type</th>
                  <th class="p-1 px-2 text-left text-[10px] sm:text-xs font-medium uppercase tracking-wider hidden lg:table-cell">Content</th>
                  <th class="p-1 px-2 text-left text-[10px] sm:text-xs font-medium uppercase tracking-wider hidden sm:table-cell">Status</th>
                  <Show when={viewMode() === 'node'}>
                    <th class="p-1 px-2 text-left text-[10px] sm:text-xs font-medium uppercase tracking-wider hidden lg:table-cell">Shared</th>
                  </Show>
                  <th class="p-1 px-2 text-left text-[10px] sm:text-xs font-medium uppercase tracking-wider min-w-[100px] sm:min-w-[150px] md:min-w-[200px]">Usage</th>
                  <th class="p-1 px-2 text-left text-[10px] sm:text-xs font-medium uppercase tracking-wider hidden sm:table-cell">Free</th>
                  <th class="p-1 px-2 text-left text-[10px] sm:text-xs font-medium uppercase tracking-wider">Total</th>
                </tr>
              </thead>
              <tbody>
                <For each={Object.entries(groupedStorage()).sort(([a], [b]) => a.localeCompare(b))}>
                  {([groupName, storages]) => (
                    <>
                      {/* Group Header */}
                      <Show when={viewMode() === 'node'}>
                        <tr class="bg-gray-50 dark:bg-gray-700/50 font-semibold text-gray-700 dark:text-gray-300 text-xs">
                          <td class="px-2 py-1 text-xs font-medium text-gray-500 dark:text-gray-400">
                            <a 
                              href={nodeHostMap()[groupName] || (groupName.includes(':') ? `https://${groupName}` : `https://${groupName}:8006`)} 
                              target="_blank" 
                              rel="noopener noreferrer" 
                              class="text-gray-500 dark:text-gray-400 hover:text-blue-600 dark:hover:text-blue-400 transition-colors duration-150 cursor-pointer"
                              title={`Open ${groupName} web interface`}
                            >
                              {groupName}
                            </a>
                          </td>
                          <td class="px-2 py-1 text-xs font-medium text-gray-500 dark:text-gray-400" colspan="8">
                            <span class="text-[10px]">
                              {getTotalByNode(storages).total > 0 && (
                                <span>
                                  {formatBytes(getTotalByNode(storages).used)} / {formatBytes(getTotalByNode(storages).total)} ({calculateOverallUsage(storages).toFixed(1)}%)
                                </span>
                              )}
                            </span>
                          </td>
                        </tr>
                      </Show>
                      
                      {/* Storage Rows */}
                      <For each={storages} fallback={<></>}>
                        {(storage) => {
                          const usagePercent = storage.total > 0 ? (storage.used / storage.total * 100) : 0;
                          const isDisabled = storage.status !== 'available';
                          
                          const alertStyles = getAlertStyles(storage.id || `${storage.instance}-${storage.name}`, activeAlerts);
                          const rowClass = `${isDisabled ? 'opacity-60' : ''} ${alertStyles.rowClass} hover:shadow-sm transition-all duration-200`;
                          
                          return (
                            <tr class={rowClass}>
                              <td class="p-1 px-2">
                                <div class="flex items-center gap-2">
                                  <span class="font-medium text-gray-900 dark:text-gray-100">
                                    {storage.name}
                                  </span>
                                  <Show when={alertStyles.hasAlert}>
                                    <div class="flex items-center gap-1">
                                      <AlertIndicator severity={alertStyles.severity} alerts={[]} />
                                      <Show when={alertStyles.alertCount > 1}>
                                        <AlertCountBadge count={alertStyles.alertCount} severity={alertStyles.severity!} alerts={[]} />
                                      </Show>
                                    </div>
                                  </Show>
                                </div>
                              </td>
                              <Show when={viewMode() === 'node'}>
                                <td class="p-1 px-2 text-xs text-gray-600 dark:text-gray-400 hidden sm:table-cell">{storage.node}</td>
                              </Show>
                              <td class="p-1 px-2 hidden md:table-cell">
                                <span class={`inline-block px-1.5 py-0.5 text-[10px] font-medium rounded ${
                                  storage.type === 'dir' ? 'bg-gray-100 text-gray-700 dark:bg-gray-700 dark:text-gray-300' :
                                  storage.type === 'pbs' ? 'bg-blue-100 text-blue-700 dark:bg-blue-900/50 dark:text-blue-300' :
                                  'bg-purple-100 text-purple-700 dark:bg-purple-900/50 dark:text-purple-300'
                                }`}>
                                  {storage.type}
                                </span>
                              </td>
                              <td class="p-1 px-2 text-xs text-gray-600 dark:text-gray-400 hidden lg:table-cell">
                                {storage.content || '-'}
                              </td>
                              <td class="p-1 px-2 text-xs hidden sm:table-cell">
                                <span class={`${
                                  storage.status === 'available' ? 'text-green-600 dark:text-green-400' : 
                                  'text-red-600 dark:text-red-400'
                                }`}>
                                  {storage.status || 'unknown'}
                                </span>
                              </td>
                              <Show when={viewMode() === 'node'}>
                                <td class="p-1 px-2 text-xs text-gray-600 dark:text-gray-400 hidden lg:table-cell">
                                  {storage.shared ? '✓' : '-'}
                                </td>
                              </Show>
                              
                              <td class="p-1 px-2">
                                <div class="relative w-full h-3.5 rounded overflow-hidden bg-gray-200 dark:bg-gray-600">
                                  <div 
                                    class={`absolute top-0 left-0 h-full ${getProgressBarColor(usagePercent)}`}
                                    style={{ width: `${usagePercent}%` }}
                                  />
                                  <span class="absolute inset-0 flex items-center justify-center text-[10px] font-medium text-gray-800 dark:text-gray-100 leading-none">
                                    <span class="truncate px-1">
                                      <span class="sm:hidden">{usagePercent.toFixed(0)}%</span>
                                      <span class="hidden sm:inline">{formatBytes(storage.used || 0)} / {formatBytes(storage.total || 0)} ({usagePercent.toFixed(1)}%)</span>
                                    </span>
                                  </span>
                                </div>
                              </td>
                              <td class="p-1 px-2 text-xs hidden sm:table-cell">{formatBytes(storage.free || 0)}</td>
                              <td class="p-1 px-2 text-xs">{formatBytes(storage.total || 0)}</td>
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
        </ComponentErrorBoundary>
      </Show>
      
      {/* Tooltip System */}
      <TooltipComponent />
    </div>
  );
};

export default Storage;