import { createMemo, For, Index, Show } from 'solid-js';
import { GuestRow } from './GuestRow';
import { GuestDrawer } from './GuestDrawer';
import { getAlertStyles } from '@/utils/alerts';
import { ComponentErrorBoundary } from '@/components/ErrorBoundary';
import { InfrastructureSelector } from '@/components/shared/InfrastructureSelector';
import { DashboardFilter } from './DashboardFilter';
import { Card } from '@/components/shared/Card';
import {
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
  TableCell,
} from '@/components/shared/Table';
import { EmptyState } from '@/components/shared/EmptyState';
import { NodeGroupHeader } from '@/components/shared/NodeGroupHeader';
import { isNodeOnline } from '@/utils/status';
import { getCanonicalWorkloadId } from '@/utils/workloads';
import { WorkloadsSummary } from '@/components/Workloads/WorkloadsSummary';
import { ScrollToTopButton } from '@/components/shared/ScrollToTopButton';
import { useDashboardState, type DashboardProps, type WorkloadSortKey } from './useDashboardState';

export function Dashboard(props: DashboardProps) {
  const {
    activeAlerts,
    alertsEnabled,
    allGuests,
    bottomSpacerHeight,
    columnVisibility,
    connected,
    containerRuntime,
    containerRuntimeOptions,
    dashboardDisconnectedState,
    dashboardGuestsEmptyState,
    dashboardInfrastructureEmptyState,
    dashboardLoadingState,
    filteredGuests,
    getGroupLabel,
    groupedGuests,
    groupedWindowing,
    guestMetadata,
    guestParentNodeMap,
    handleBeforeAutoFocus,
    handleCustomUrlUpdate,
    handleNodeSelect,
    handleSort,
    handleTagClick,
    hoveredWorkloadId,
    initialDataReceived,
    isMobile,
    isSearchLocked,
    isWorkloadsRoute,
    kioskMode,
    kubernetesContextOptions,
    kubernetesNamespaceOptions,
    mobileVisibleColumnIds,
    mobileVisibleColumns,
    navigate,
    nodeByInstance,
    reconnect,
    search,
    selectedGuestId,
    selectedKubernetesContext,
    selectedKubernetesNamespace,
    selectedNode,
    setContainerRuntime,
    setGroupingMode,
    setHoveredWorkloadId,
    setSearch,
    setSelectedGuestId,
    setSelectedKubernetesContext,
    setSelectedKubernetesNamespace,
    setSortDirection,
    setSortKey,
    setStatusMode,
    setTableBodyRef,
    setTableWrapperRef,
    setViewMode,
    setWorkloadsSummaryCollapsed,
    setWorkloadsSummaryRange,
    sortDirection,
    sortKey,
    statusMode,
    topSpacerHeight,
    totalColumns,
    totalStats,
    viewMode,
    visibleColumns,
    visibleGroupKeys,
    windowedGroupedGuests,
    workloadIOEmphasis,
    workloadNodeOptions,
    workloads,
    workloadsSummaryCollapsed,
    workloadsSummaryFallbackCounts,
    workloadsSummaryFallbackSnapshots,
    workloadsSummaryRange,
    workloadsSummaryVisibleIds,
    ws,
    groupingMode,
  } = useDashboardState(props);

  return (
    <div class="space-y-3">
      <Show when={isWorkloadsRoute() && !workloadsSummaryCollapsed()}>
        <div class="hidden lg:block sticky-shield sticky top-0 z-20 bg-surface">
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

      {/* Infrastructure selector - infrastructure summary (hidden on workloads) */}
      <InfrastructureSelector
        currentTab="dashboard"
        globalTemperatureMonitoringEnabled={ws.state.temperatureMonitoringEnabled}
        onNodeSelect={handleNodeSelect}
        nodes={props.nodes}
        searchTerm={search()}
        showNodeSummary={!isWorkloadsRoute()}
      />

      {/* Loading State */}
      <Show when={connected() && !initialDataReceived()}>
        <Card padding="lg">
          <EmptyState
            icon={
              <svg
                class="mx-auto h-12 w-12 animate-spin text-slate-400"
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
            title={dashboardLoadingState().title}
            description={dashboardLoadingState().description}
          />
        </Card>
      </Show>

      {/* Empty state when no infrastructure has connected yet */}
      <Show
        when={
          connected() &&
          initialDataReceived() &&
          !workloads.loading() &&
          props.nodes.length === 0 &&
          allGuests().length === 0
        }
      >
        <Card padding="lg">
          <EmptyState
            icon={
              <svg
                class="h-12 w-12 text-slate-400"
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
            title={dashboardInfrastructureEmptyState().title}
            description={dashboardInfrastructureEmptyState().description}
            actions={
              !kioskMode() ? (
                <button
                  type="button"
                  onClick={() => navigate('/settings')}
                  class="inline-flex items-center px-3 py-1.5 border border-transparent text-xs font-medium rounded-md text-white bg-blue-600 hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500"
                >
                  {dashboardInfrastructureEmptyState().actionLabel}
                </button>
              ) : undefined
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
            title={dashboardDisconnectedState().title}
            description={dashboardDisconnectedState().description}
            tone="danger"
            actions={
              dashboardDisconnectedState().actionLabel ? (
                <button
                  onClick={() => reconnect()}
                  class="mt-2 inline-flex items-center px-4 py-2 text-xs font-medium rounded bg-red-600 text-white hover:bg-red-700 transition-colors"
                >
                  {dashboardDisconnectedState().actionLabel}
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
          onBeforeAutoFocus={handleBeforeAutoFocus}
          availableColumns={columnVisibility.availableToggles()}
          isColumnHidden={columnVisibility.isHiddenByUser}
          onColumnToggle={columnVisibility.toggle}
          onColumnReset={columnVisibility.resetToDefaults}
          chartsCollapsed={isWorkloadsRoute() ? workloadsSummaryCollapsed : undefined}
          onChartsToggle={
            isWorkloadsRoute() ? () => setWorkloadsSummaryCollapsed((c) => !c) : undefined
          }
          containerRuntimeFilter={(() => {
            if (!isWorkloadsRoute()) return undefined;
            if (viewMode() !== 'app-container') return undefined;
            const options = containerRuntimeOptions();
            if (options.length === 0) return undefined;
            return {
              id: 'workloads-container-runtime-filter',
              label: 'Runtime',
              value: containerRuntime(),
              options: [
                { value: '', label: 'All runtimes' },
                ...options.map((value) => ({ value, label: value })),
              ],
              onChange: (value: string) => setContainerRuntime(value),
            };
          })()}
          hostFilter={(() => {
            if (!isWorkloadsRoute()) return undefined;
            if (viewMode() === 'pod') {
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
              label: 'Node',
              value: selectedNode() ?? '',
              options: [{ value: '', label: 'All nodes' }, ...workloadNodeOptions()],
              onChange: (value: string) => {
                handleNodeSelect(value || null, value ? 'pve' : null);
              },
            };
          })()}
          namespaceFilter={(() => {
            if (!isWorkloadsRoute()) return undefined;
            if (viewMode() !== 'pod') return undefined;
            const options = kubernetesNamespaceOptions();
            if (options.length === 0) return undefined;
            return {
              id: 'workloads-k8s-namespace-filter',
              label: 'Namespace',
              value: selectedKubernetesNamespace() ?? '',
              options: [
                { value: '', label: 'All namespaces' },
                ...options.map((value) => ({ value, label: value })),
              ],
              onChange: (value: string) => setSelectedKubernetesNamespace(value || null),
            };
          })()}
        />
      </Show>

      {/* Table View */}
      <Show when={connected() && initialDataReceived() && filteredGuests().length > 0}>
        <ComponentErrorBoundary name="Guest Table">
          <Card padding="none" tone="card" class="mb-4 rounded-md">
            <div class="border-b border-border bg-surface-hover px-3 py-2 text-[11px] font-semibold uppercase tracking-wide text-muted">
              Workloads
            </div>
            <div class="overflow-x-auto">
              <Table
                wrapperRef={setTableWrapperRef}
                class="whitespace-nowrap min-w-[max-content]"
                style={{
                  'table-layout': 'fixed',
                  'min-width': isMobile() ? '100%' : 'max-content',
                }}
              >
                <TableHeader>
                  <TableRow class="bg-surface-alt text-muted border-b border-border">
                    <For each={mobileVisibleColumns()}>
                      {(col) => {
                        const isFirst = () => col.id === visibleColumns()[0]?.id;
                        const sortKeyForCol = col.sortKey as WorkloadSortKey | undefined;
                        const isSortable = !!sortKeyForCol;
                        const isSorted = () => sortKeyForCol && sortKey() === sortKeyForCol;

                        return (
                          <TableHead
                            class={`py-0.5 text-[11px] sm:text-xs font-medium uppercase tracking-wider whitespace-nowrap
 ${isFirst() ? 'pl-2 sm:pl-3 pr-1.5 sm:pr-2 text-left' : 'px-1.5 sm:px-2 text-center'}
 ${isSortable ? 'cursor-pointer hover:bg-surface-hover' : ''}`}
                            style={{
                              ...(['cpu', 'memory', 'disk'].includes(col.id)
                                ? { width: isMobile() ? '70px' : '140px' }
                                : ['netIo', 'diskIo'].includes(col.id)
                                  ? { width: isMobile() ? '170px' : '170px' }
                                  : isMobile() && col.id === 'name'
                                    ? { width: '100%', 'min-width': '120px' }
                                    : col.width
                                      ? { width: col.width }
                                      : {}),
                              'vertical-align': 'middle',
                            }}
                            onClick={() => isSortable && handleSort(sortKeyForCol!)}
                            title={col.icon ? col.label : undefined}
                          >
                            <div
                              class={`flex items-center gap-0.5 ${isFirst() ? 'justify-start' : 'justify-center'}`}
                              style={{ 'min-height': '14px' }}
                            >
                              {col.icon ? (
                                <span class="flex items-center">{col.icon}</span>
                              ) : (
                                col.label
                              )}
                              {isSorted() && (sortDirection() === 'asc' ? ' ▲' : ' ▼')}
                            </div>
                          </TableHead>
                        );
                      }}
                    </For>
                  </TableRow>
                </TableHeader>
                <TableBody ref={setTableBodyRef} class="divide-y divide-border">
                  <Show when={groupedWindowing.isWindowed() && topSpacerHeight() > 0}>
                    <TableRow aria-hidden="true">
                      <TableCell
                        colspan={totalColumns()}
                        style={{ height: `${topSpacerHeight()}px`, padding: '0', border: '0' }}
                      />
                    </TableRow>
                  </Show>
                  {/* Outer <For> uses string keys — strings compare by value so DOM is stable across data updates */}
                  <For each={visibleGroupKeys()} fallback={<></>}>
                    {(groupKey) => {
                      const groupGuests = () => windowedGroupedGuests()[groupKey] || [];
                      const fullGroupGuests = () => groupedGuests()[groupKey] || [];
                      const node = () => nodeByInstance()[groupKey];
                      return (
                        <>
                          <Show when={groupingMode() === 'grouped'}>
                            <Show
                              when={node()}
                              fallback={
                                <TableRow class="bg-surface-alt">
                                  <TableCell
                                    colspan={totalColumns()}
                                    class="py-0.5 pr-1.5 sm:pr-2 pl-2 sm:pl-3 text-[12px] sm:text-sm font-semibold text-base-content"
                                  >
                                    {(() => {
                                      const label = getGroupLabel(groupKey, fullGroupGuests());
                                      return (
                                        <div class="flex items-center gap-3">
                                          <span>{label.name}</span>
                                          <Show when={label.type}>
                                            <span class="inline-flex items-center rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300">
                                              {label.type}
                                            </span>
                                          </Show>
                                        </div>
                                      );
                                    })()}
                                  </TableCell>
                                </TableRow>
                              }
                            >
                              <NodeGroupHeader
                                node={node()!}
                                renderAs="tr"
                                colspan={totalColumns()}
                              />
                            </Show>
                          </Show>
                          {/* Inner <Index> tracks by position — updates props reactively instead of recreating DOM */}
                          <Index each={groupGuests()} fallback={<></>}>
                            {(guest) => {
                              const guestId = createMemo(() => getCanonicalWorkloadId(guest()));
                              const getMetadata = () =>
                                guestMetadata()[guestId()] ||
                                guestMetadata()[
                                  `${guest().instance}:${guest().node}:${guest().vmid}`
                                ];
                              const parentNode = () => node() ?? guestParentNodeMap()[guestId()];
                              const parentNodeOnline = () => {
                                const pn = parentNode();
                                return pn ? isNodeOnline(pn) : true;
                              };

                              return (
                                <ComponentErrorBoundary name="GuestRow">
                                  <GuestRow
                                    guest={guest()}
                                    alertStyles={getAlertStyles(
                                      guestId(),
                                      activeAlerts,
                                      alertsEnabled(),
                                    )}
                                    customUrl={getMetadata()?.customUrl}
                                    onTagClick={handleTagClick}
                                    activeSearch={search()}
                                    parentNodeOnline={parentNodeOnline()}
                                    onCustomUrlUpdate={handleCustomUrlUpdate}
                                    isGroupedView={groupingMode() === 'grouped'}
                                    visibleColumnIds={mobileVisibleColumnIds()}
                                    onClick={() =>
                                      setSelectedGuestId(
                                        selectedGuestId() === guestId() ? null : guestId(),
                                      )
                                    }
                                    isExpanded={selectedGuestId() === guestId()}
                                    ioEmphasis={workloadIOEmphasis()}
                                    onHoverChange={setHoveredWorkloadId}
                                  />
                                  <Show when={selectedGuestId() === guestId()}>
                                    <TableRow>
                                      <TableCell
                                        colspan={totalColumns()}
                                        class="p-0 border-b border-border bg-surface-alt"
                                      >
                                        <div
                                          class="px-2 sm:px-4 py-3 sm:py-4"
                                          onClick={(e) => e.stopPropagation()}
                                        >
                                          <GuestDrawer
                                            guest={guest()}
                                            onClose={() => setSelectedGuestId(null)}
                                            customUrl={getMetadata()?.customUrl}
                                            onCustomUrlChange={handleCustomUrlUpdate}
                                          />
                                        </div>
                                      </TableCell>
                                    </TableRow>
                                  </Show>
                                </ComponentErrorBoundary>
                              );
                            }}
                          </Index>
                        </>
                      );
                    }}
                  </For>
                  <Show when={groupedWindowing.isWindowed() && bottomSpacerHeight() > 0}>
                    <TableRow aria-hidden="true">
                      <TableCell
                        colspan={totalColumns()}
                        style={{ height: `${bottomSpacerHeight()}px`, padding: '0', border: '0' }}
                      />
                    </TableRow>
                  </Show>
                </TableBody>
              </Table>
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
                class="h-12 w-12 text-slate-400"
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
            title={dashboardGuestsEmptyState().title}
            description={dashboardGuestsEmptyState().description}
          />
        </Card>
      </Show>

      {/* Stats */}
      <Show when={connected() && initialDataReceived()}>
        <div class="mb-4">
          <div class="flex items-center gap-2 p-2 bg-surface-alt border border-border rounded">
            <span class="flex items-center gap-1 text-xs text-muted">
              <span class="h-2 w-2 bg-green-500 rounded-full"></span>
              {totalStats().running} running
            </span>
            <Show when={totalStats().degraded > 0}>
              <span class="text-slate-400">|</span>
              <span class="flex items-center gap-1 text-xs text-muted">
                <span class="h-2 w-2 bg-orange-500 rounded-full"></span>
                {totalStats().degraded} degraded
              </span>
            </Show>
            <span class="text-slate-400">|</span>
            <span class="flex items-center gap-1 text-xs text-muted">
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
