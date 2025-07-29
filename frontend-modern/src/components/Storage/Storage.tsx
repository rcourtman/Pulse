import { Component, For, Show, createSignal, createMemo, createEffect, onCleanup } from 'solid-js';
import { useWebSocket } from '@/App';
import { getAlertStyles } from '@/utils/alerts';
import { AlertIndicator, AlertCountBadge } from '@/components/shared/AlertIndicators';
import { formatBytes } from '@/utils/format';
import { DynamicChart } from '@/components/shared/DynamicChart';
import { createTooltipSystem } from '@/components/shared/Tooltip';
import { fetchChartData, getStorageChartData, shouldFetchChartData } from '@/stores/charts';
import { POLLING_INTERVALS } from '@/constants';
import type { Storage as StorageType } from '@/types/api';
import { ComponentErrorBoundary } from '@/components/ErrorBoundary';


const Storage: Component = () => {
  const { state, connected, activeAlerts } = useWebSocket();
  const [viewMode, setViewMode] = createSignal<'node' | 'storage'>('node');
  const [chartsEnabled, setChartsEnabled] = createSignal(false);
  const [timeRange, setTimeRange] = createSignal<string>('1h');
  const [searchTerm, setSearchTerm] = createSignal('');
  const [showFilters, setShowFilters] = createSignal(
    localStorage.getItem('storageShowFilters') !== null 
      ? localStorage.getItem('storageShowFilters') === 'true'
      : false // Default to collapsed
  );
  
  // Create tooltip system
  const TooltipComponent = createTooltipSystem();
  
  // Time range options - match dashboard format
  const timeRanges = [
    { value: '5m', label: '5m' },
    { value: '15m', label: '15m' },
    { value: '30m', label: '30m' },
    { value: '1h', label: '1h' },
    { value: '4h', label: '4h' },
    { value: '12h', label: '12h' },
    { value: '24h', label: '24h' },
    { value: '7d', label: '7d' }
  ];
  
  // Load preferences from localStorage
  createEffect(() => {
    const savedViewMode = localStorage.getItem('storageViewMode');
    if (savedViewMode === 'storage') setViewMode('storage');
    
    const savedChartsEnabled = localStorage.getItem('storageChartsEnabled');
    if (savedChartsEnabled === 'true') setChartsEnabled(true);
  });
  
  // Save preferences to localStorage
  createEffect(() => {
    localStorage.setItem('storageViewMode', viewMode());
  });
  
  createEffect(() => {
    localStorage.setItem('storageChartsEnabled', chartsEnabled().toString());
  });
  
  createEffect(() => {
    localStorage.setItem('storageShowFilters', showFilters().toString());
  });
  
  // Chart update interval
  let chartUpdateInterval: number | undefined;
  
  // Fetch chart data when charts are enabled or time range changes
  createEffect(() => {
    if (chartsEnabled() && connected()) {
      // Initial fetch
      fetchChartData(timeRange());
      
      // Setup periodic updates
      chartUpdateInterval = window.setInterval(() => {
        if (shouldFetchChartData()) {
          fetchChartData(timeRange());
        }
      }, POLLING_INTERVALS.CHART_UPDATE);
    } else {
      // Clear interval when charts not enabled
      if (chartUpdateInterval) {
        window.clearInterval(chartUpdateInterval);
        chartUpdateInterval = undefined;
      }
    }
  });
  
  // Update charts when time range changes
  createEffect(() => {
    if (chartsEnabled()) {
      fetchChartData(timeRange());
    }
  });
  
  // Cleanup on unmount
  onCleanup(() => {
    if (chartUpdateInterval) {
      window.clearInterval(chartUpdateInterval);
    }
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
    return 'bg-green-500/60 dark:bg-green-500/50';
  };
  
  // Reset filters
  const resetFilters = () => {
    setSearchTerm('');
    setViewMode('node');
    setChartsEnabled(false);
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
        if (searchTerm().trim() || viewMode() !== 'node' || chartsEnabled()) {
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
    <div id="storage" class="tab-content bg-white dark:bg-gray-800 rounded-b rounded-tr shadow mb-2">
      <div class="p-3">
        {/* Filter Section */}
        <div class="storage-filter bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg shadow-sm mb-3">
          {/* Filter toggle - visible on all screen sizes */}
          <button
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
              <Show when={searchTerm() || viewMode() !== 'node' || chartsEnabled()}>
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
          
          <div class={`filter-controls-wrapper ${showFilters() ? 'block' : 'hidden'} p-3 border-t border-gray-200 dark:border-gray-700`}>
            <div class="flex flex-col gap-3">
              {/* Search Row */}
              <div class="flex gap-2">
                <div class="relative flex-1">
                  <input
                    ref={searchInputRef}
                    type="text"
                    placeholder="Search by name, node, type, or content..."
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
                
                <button
                  onClick={resetFilters}
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
              
              {/* View Controls Row */}
              <div class="flex flex-col sm:flex-row gap-3">
                {/* View Controls */}
                <div class="flex items-center gap-2">
                  <span class="text-xs font-medium text-gray-600 dark:text-gray-400 whitespace-nowrap">Display:</span>
                  
                  {/* Charts Toggle */}
                  <div class="inline-flex rounded-lg bg-gray-100 dark:bg-gray-700 p-0.5">
                    <button
                      onClick={() => setChartsEnabled(!chartsEnabled())}
                      class={`px-3 py-1.5 text-xs font-medium rounded-md transition-all ${
                        chartsEnabled()
                          ? 'bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 shadow-sm'
                          : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
                      }`}
                    >
                      Charts {chartsEnabled() ? 'On' : 'Off'}
                    </button>
                  </div>
                
                <div class="h-6 w-px bg-gray-200 dark:bg-gray-600"></div>
                
                {/* View Mode Toggle */}
                <div class="inline-flex rounded-lg bg-gray-100 dark:bg-gray-700 p-0.5">
                  <button
                    onClick={() => setViewMode('node')}
                    class={`px-3 py-1.5 text-xs font-medium rounded-md transition-all ${
                      viewMode() === 'node'
                        ? 'bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 shadow-sm'
                        : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
                    }`}
                  >
                    By Node
                  </button>
                  <button
                    onClick={() => setViewMode('storage')}
                    class={`px-3 py-1.5 text-xs font-medium rounded-md transition-all ${
                      viewMode() === 'storage'
                        ? 'bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 shadow-sm'
                        : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
                    }`}
                  >
                    By Storage
                  </button>
                </div>
              </div>
              
              {/* Chart Time Range Controls - Show when charts enabled */}
              <Show when={chartsEnabled()}>
                <div class="flex items-center gap-2">
                  <div class="h-6 w-px bg-gray-200 dark:bg-gray-600"></div>
                  <span class="text-xs font-medium text-gray-600 dark:text-gray-400">Time Range:</span>
                  <div class="inline-flex rounded-lg bg-gray-100 dark:bg-gray-700 p-0.5">
                    <For each={timeRanges}>
                      {(range) => (
                        <button
                          onClick={() => setTimeRange(range.value)}
                          class={`px-2 py-1.5 text-xs font-medium rounded-md transition-all ${
                            timeRange() === range.value
                              ? 'bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 shadow-sm'
                              : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
                          }`}
                        >
                          {range.label}
                        </button>
                      )}
                    </For>
                  </div>
                </div>
              </Show>
            </div>
          </div>
        </div>
      </div>
    </div>
      
      {/* Table Section */}
      <div class="px-3 pb-3">
        <ComponentErrorBoundary name="Storage Table">
          <div class="table-container overflow-x-auto mb-2 border border-gray-200 dark:border-gray-700 rounded overflow-hidden scrollbar">
            <table id="storage-table" class="w-full text-sm border-collapse min-w-full" style="table-layout: fixed;">
            <thead>
              <tr class="border-b border-gray-200 dark:border-gray-600">
                <th class="bg-gray-100 dark:bg-gray-700 p-1 px-2 text-left text-xs font-medium text-gray-600 dark:text-gray-300 uppercase tracking-wider" style="width: 130px;">
                  Storage
                </th>
                <Show when={viewMode() === 'storage'}>
                  <th class="bg-gray-100 dark:bg-gray-700 p-1 px-2 text-left text-xs font-medium text-gray-600 dark:text-gray-300 uppercase tracking-wider" style="width: 120px;">
                    Nodes
                  </th>
                  <th class="bg-gray-100 dark:bg-gray-700 p-1 px-2 text-left text-xs font-medium text-gray-600 dark:text-gray-300 uppercase tracking-wider" style="width: 80px;">
                    Type
                  </th>
                </Show>
                <Show when={viewMode() === 'node'}>
                  <th class="bg-gray-100 dark:bg-gray-700 p-1 px-2 text-left text-xs font-medium text-gray-600 dark:text-gray-300 uppercase tracking-wider" style="width: 180px;">
                    Content
                  </th>
                  <th class="bg-gray-100 dark:bg-gray-700 p-1 px-2 text-left text-xs font-medium text-gray-600 dark:text-gray-300 uppercase tracking-wider" style="width: 80px;">
                    Type
                  </th>
                  <th class="bg-gray-50 dark:bg-gray-700 p-1 px-2 text-center text-xs font-medium text-gray-700 dark:text-gray-300 uppercase tracking-wider" style="width: 50px;">
                    Shared
                  </th>
                </Show>
                <th class="bg-gray-100 dark:bg-gray-700 p-1 px-2 text-left text-xs font-medium text-gray-600 dark:text-gray-300 uppercase tracking-wider" style="width: 200px;">
                  Usage
                </th>
                <th class="bg-gray-100 dark:bg-gray-700 p-1 px-2 text-left text-xs font-medium text-gray-600 dark:text-gray-300 uppercase tracking-wider" style="width: 80px;">
                  Avail
                </th>
                <th class="bg-gray-100 dark:bg-gray-700 p-1 px-2 text-left text-xs font-medium text-gray-600 dark:text-gray-300 uppercase tracking-wider" style="width: 80px;">
                  Total
                </th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-200 dark:divide-gray-600">
              <Show when={connected() && state.storage && state.storage.length > 0}>
                <For each={Object.entries(groupedStorage()).sort(([a], [b]) => a.localeCompare(b))} fallback={<></>}>
                  {([groupName, storages]) => (
                    <>
                      {/* Group Header for Node View */}
                      <Show when={viewMode() === 'node'}>
                        <tr class="node-storage-header bg-gray-50 dark:bg-gray-700/50 font-semibold text-gray-700 dark:text-gray-300 text-xs">
                          <td colspan="7" class="px-2 py-1 text-xs font-medium text-gray-500 dark:text-gray-400">
                            <a 
                              href={`https://${groupName}:8006`} 
                              target="_blank" 
                              rel="noopener noreferrer" 
                              class="hover:text-blue-600 dark:hover:text-blue-400"
                            >
                              {groupName}
                            </a>
                            <span class="text-[10px] text-gray-500 dark:text-gray-400 ml-2">
                              ({storages.length} storage{storages.length !== 1 ? 's' : ''})
                            </span>
                          </td>
                        </tr>
                      </Show>
                      
                      {/* Storage Rows */}
                      <For each={storages} fallback={<></>}>
                        {(storage) => {
                          const usagePercent = storage.total > 0 ? (storage.used / storage.total * 100) : 0;
                          // Get chart data from unified store
                          const storageChartData = createMemo(() => getStorageChartData(storage.id, 'disk'));
                          const isDisabled = storage.status !== 'available';
                          
                          const alertStyles = getAlertStyles(storage.id || `${storage.instance}-${storage.name}`, activeAlerts);
                          const rowClass = `${isDisabled ? 'opacity-60' : ''} ${alertStyles.rowClass} hover:shadow-sm transition-all duration-200`;
                          
                          return (
                            <tr class={rowClass}>
                              <td class="p-1 px-2 text-xs font-medium">
                                <div class="flex items-center gap-1">
                                  <span>{storage.name}</span>
                                  <Show when={isDisabled}>
                                    <span class="text-gray-500 dark:text-gray-400 text-[10px]">({storage.status})</span>
                                  </Show>
                                  <Show when={alertStyles.hasAlert}>
                                    <div class="flex items-center gap-1 ml-auto">
                                      <AlertIndicator severity={alertStyles.severity} />
                                      <Show when={alertStyles.alertCount > 1}>
                                        <AlertCountBadge count={alertStyles.alertCount} severity={alertStyles.severity!} />
                                      </Show>
                                    </div>
                                  </Show>
                                </div>
                              </td>
                              
                              <Show when={viewMode() === 'storage'}>
                                <td class="p-1 px-2 text-xs text-gray-600 dark:text-gray-400">
                                  <Show when={storage.shared} fallback={
                                    <span>{storage.node} <span class="text-[10px] text-gray-500">(Local)</span></span>
                                  }>
                                    <span class="text-green-600 dark:text-green-400">
                                      All Nodes
                                      <Show when={storage.instance}>
                                        <span class="text-[10px] text-gray-500 ml-1">({storage.instance})</span>
                                      </Show>
                                    </span>
                                  </Show>
                                </td>
                                <td class="p-1 px-2 text-xs text-gray-600 dark:text-gray-400">
                                  {storage.type}
                                </td>
                              </Show>
                              
                              <Show when={viewMode() === 'node'}>
                                <td class="p-1 px-2 text-xs text-gray-600 dark:text-gray-400 truncate" style="max-width: 180px;" title={storage.content}>
                                  {storage.content}
                                </td>
                                <td class="p-1 px-2 text-xs text-gray-600 dark:text-gray-400">
                                  {storage.type}
                                </td>
                                <td class="p-1 px-2 text-xs text-center">
                                  {storage.shared ? '✓' : '-'}
                                </td>
                              </Show>
                              
                              <td class="p-1 px-2" style="width: 200px;">
                                <Show when={chartsEnabled()}>
                                  <DynamicChart
                                    data={storageChartData()}
                                    metric="disk"
                                    guestId={storage.id}
                                    chartType="storage"
                                    filled
                                    forceGray
                                  />
                                </Show>
                                <Show when={!chartsEnabled()}>
                                  <div class="relative w-full h-3.5 rounded overflow-hidden bg-gray-200 dark:bg-gray-600">
                                    <div 
                                      class={`absolute top-0 left-0 h-full ${getProgressBarColor(usagePercent)}`}
                                      style={{ width: `${usagePercent}%` }}
                                    />
                                    <span class="absolute inset-0 flex items-center justify-center text-[10px] font-medium text-gray-800 dark:text-gray-100 leading-none">
                                      <span class="truncate px-1">{formatBytes(storage.used || 0)} / {formatBytes(storage.total || 0)} ({usagePercent.toFixed(1)}%)</span>
                                    </span>
                                  </div>
                                </Show>
                              </td>
                              <td class="p-1 px-2 text-xs">{formatBytes(storage.free || 0)}</td>
                              <td class="p-1 px-2 text-xs">{formatBytes(storage.total || 0)}</td>
                            </tr>
                          );
                        }}
                      </For>
                    </>
                  )}
                </For>
              </Show>
              
              {/* Empty State */}
              <Show when={connected() && (!state.storage || state.storage.length === 0 || (viewMode() === 'storage' && filteredStorage().length === 0))}>
                <tr>
                  <td colspan={viewMode() === 'node' ? 7 : 7} class="p-8 text-center">
                    <div class="text-gray-500 dark:text-gray-400">
                      <svg class="mx-auto h-12 w-12 text-gray-400 mb-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                      </svg>
                      <p class="text-sm font-medium mb-1">No Active Storage</p>
                      <p class="text-xs">
                        {viewMode() === 'storage' 
                          ? 'No storage with capacity found in the cluster.' 
                          : 'No storage configured.'}
                      </p>
                    </div>
                  </td>
                </tr>
              </Show>
              
              {/* Disconnected State */}
              <Show when={!connected()}>
                <tr>
                  <td colspan={viewMode() === 'node' ? 7 : 7} class="p-8 text-center">
                    <div class="text-red-600 dark:text-red-400">
                      <svg class="mx-auto h-12 w-12 mb-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                      </svg>
                      <p class="text-sm font-medium mb-1">Connection Lost</p>
                      <p class="text-xs">Unable to connect to the backend server.</p>
                    </div>
                  </td>
                </tr>
              </Show>
            </tbody>
          </table>
        </div>
        </ComponentErrorBoundary>
      </div>
      
      
      <style>{`
        .node-storage-header {
          font-weight: 600;
        }
        
        .table-container {
          max-height: calc(100vh - 250px);
        }
        
        .scrollbar {
          scrollbar-width: thin;
          scrollbar-color: #6b7280 transparent;
        }
        
        .scrollbar::-webkit-scrollbar {
          width: 8px;
          height: 8px;
        }
        
        .scrollbar::-webkit-scrollbar-track {
          background: transparent;
        }
        
        .scrollbar::-webkit-scrollbar-thumb {
          background-color: #6b7280;
          border-radius: 4px;
        }
        
        .scrollbar::-webkit-scrollbar-thumb:hover {
          background-color: #4b5563;
        }
        
      `}</style>
      
      {/* Tooltip System */}
      <TooltipComponent />
    </div>
  );
};

export default Storage;