import { useLocation } from '@solidjs/router';
import {
  Show,
  createEffect,
  createMemo,
  createResource,
  createSignal,
  onCleanup,
  onMount,
  type Accessor,
} from 'solid-js';
import StorageSurface from '@/components/Storage/Storage';
import { WorkloadsFilter } from '@/components/Workloads/WorkloadsFilter';
import { WorkloadsSurface } from '@/components/Workloads/WorkloadsSurface';
import { useWorkloadsState } from '@/components/Workloads/useWorkloadsState';
import {
  DEFAULT_WORKLOADS_METRIC_DISPLAY_MODE,
  type WorkloadsStatusOption,
  type WorkloadsMemoryDisplayBasis,
  type WorkloadsMetricDisplayMode,
} from '@/components/Workloads/workloadsFilterModel';
import {
  WORKLOAD_TABLE_HISTORY_DEFAULT_RANGE,
  isWorkloadTableMetricHistoryRange,
  type WorkloadTableMetricHistoryRange,
} from '@/components/Workloads/workloadMetricHistoryModel';
import {
  buildInfrastructureAgentUpdatesPath,
  buildInfrastructureWorkspacePath,
} from '@/components/Settings/infrastructureWorkspaceModel';
import {
  collectOutdatedAgentHosts,
  formatAgentVersionDisplay,
} from '@/features/platformPage/agentVersion';
import { getPlatformIcon } from '@/features/platformPage/platformIcon';
import { PlatformOutdatedAgentNotice } from '@/features/platformPage/PlatformOutdatedAgentNotice';
import { PlatformOutdatedSensorSetupNotice } from '@/features/platformPage/PlatformOutdatedSensorSetupNotice';
import {
  PLATFORM_ESTATE_COUNTS_STORAGE_KEY,
  buildProxmoxEstateTopology,
  deserializePlatformEstateCountsVisibility,
} from '@/features/platformPage/platformEstateOverviewModel';
import { collectOutdatedSensorSetupNodes } from '@/features/platformPage/sensorSetup';
import { usePersistentSignal } from '@/hooks/usePersistentSignal';
import { useObservedElementWidth } from '@/hooks/useObservedElementWidth';
import { STORAGE_KEYS } from '@/utils/localStorage';
import {
  PlatformErrorState,
  PlatformSectionTabs,
  PlatformTableEmptyState,
  PlatformTableLoadingState,
} from '@/features/platformPage/sharedPlatformPage';
import { ProxmoxBackupsTable } from './ProxmoxBackupsTable';
import { ProxmoxCephTable } from './ProxmoxCephTable';
import { ProxmoxMailGatewayTable } from './ProxmoxMailGatewayTable';
import { ProxmoxNodesTable } from './ProxmoxNodesTable';
import { ProxmoxReplicationTable, fetchReplicationJobs } from './ProxmoxReplicationTable';
import { useUnifiedResources } from '@/hooks/useUnifiedResources';
import type { Resource } from '@/types/resource';
import { updateStore } from '@/stores/updates';
import {
  PROXMOX_TAB_SPECS,
  buildProxmoxPageModel,
  buildVisibleProxmoxTabSpecsFromCounts,
  type ProxmoxPageModel,
  type ProxmoxPageTabId,
} from './proxmoxPageModel';

// Each workflow hydrates only the resource families it consumes. The first
// REST page also carries global type aggregations, so evidence-gated tabs do
// not require the old 1,000+ row workspace request.
const PROXMOX_RESOURCE_QUERY_BY_TAB: Record<ProxmoxPageTabId, string> = {
  overview: 'type=agent,vm,system-container,oci-container',
  storage: 'type=agent,pbs,storage,physical_disk,ceph',
  replication: 'type=agent',
  backups: 'type=agent,vm,system-container,pbs',
  ceph: 'type=ceph',
  mail: 'type=pmg',
};

const PROXMOX_PLATFORM_FILTER = 'proxmox-all';
const PROXMOX_WORKLOAD_STATUS_STORAGE_SCOPE = 'proxmox';
const PROXMOX_WORKLOAD_EXCLUDED_TYPES = ['app-container'] as const;
const PHONE_MOUNTED_TAB_LIMIT = 2;
const PHONE_BACKGROUND_HYDRATION_INTERVAL_MS = 750;
const VALID_TABS = new Set<ProxmoxPageTabId>(PROXMOX_TAB_SPECS.map((tab) => tab.id));
const PROXMOX_WORKLOAD_STATUS_OPTIONS: readonly WorkloadsStatusOption[] = [
  { value: 'all', label: 'All' },
  { value: 'running', label: 'Running' },
  { value: 'degraded', label: 'Attention' },
  { value: 'stopped', label: 'Stopped' },
];

const ProxmoxIcon = getPlatformIcon('proxmox');
const proxmoxIcon = () => <ProxmoxIcon class="h-6 w-6 text-slate-400" />;
const EMPTY_PROXMOX_PAGE_MODEL = buildProxmoxPageModel([]);

export function ProxmoxPageSurface() {
  const location = useLocation();
  const phoneViewport = typeof window !== 'undefined' && window.innerWidth < 640;
  const requestedTab = createMemo<ProxmoxPageTabId>(() => {
    const segment = location.pathname.split('/').filter(Boolean)[1] as ProxmoxPageTabId | undefined;
    return segment && VALID_TABS.has(segment) ? segment : 'overview';
  });
  const [resourceAggregations, setResourceAggregations] =
    createSignal<ReturnType<ReturnType<typeof useUnifiedResources>['aggregations']>>(null);
  // Replication jobs come straight from /api/replication/jobs (they bypass
  // the unified-resource pipeline), so the surface owns the fetch: the job
  // count gates the Replication tab and the same data feeds the table.
  // Reading an errored resource throws, hence the `.error` guards.
  const [replicationJobs, { refetch: refetchReplicationJobs }] =
    createResource(fetchReplicationJobs);
  const replicationJobCount = createMemo(() =>
    replicationJobs.error ? 0 : (replicationJobs() ?? []).length,
  );
  const visibleTabs = createMemo(() => {
    const aggregations = resourceAggregations();
    if (aggregations) {
      return buildVisibleProxmoxTabSpecsFromCounts(aggregations.byType, replicationJobCount());
    }
    return PROXMOX_TAB_SPECS;
  });
  const visibleTabIds = createMemo(
    () => new Set<ProxmoxPageTabId>(visibleTabs().map((tab) => tab.id)),
  );
  const activeTab = createMemo<ProxmoxPageTabId>(() => {
    const requested = requestedTab();
    return visibleTabIds().has(requested) ? requested : 'overview';
  });
  const [backgroundHydrationTabs, setBackgroundHydrationTabs] = createSignal<Set<ProxmoxPageTabId>>(
    new Set(),
  );
  onMount(() => {
    // A phone retains only two tab trees, so downloading every route in the
    // background spends memory and main-thread time on views that will be
    // evicted before they are likely to be used. Storage is the common
    // Overview transition and the only worthwhile phone prewarm.
    const hydrationQueue: ProxmoxPageTabId[] = phoneViewport
      ? ['storage']
      : ['storage', 'backups', 'replication', 'ceph', 'mail', 'overview'];
    const idleHandles: number[] = [];
    const timeoutHandles: number[] = [];

    const hydrateNext = (index: number) => {
      if (index >= hydrationQueue.length) return;
      const hydrate = () => {
        setBackgroundHydrationTabs((current) => {
          const next = new Set(current);
          next.add(hydrationQueue[index]);
          return next;
        });
        if (phoneViewport) {
          timeoutHandles.push(
            window.setTimeout(() => hydrateNext(index + 1), PHONE_BACKGROUND_HYDRATION_INTERVAL_MS),
          );
        } else {
          hydrateNext(index + 1);
        }
      };

      if (typeof window.requestIdleCallback === 'function') {
        idleHandles.push(window.requestIdleCallback(hydrate, { timeout: 1_000 }));
      } else {
        timeoutHandles.push(window.setTimeout(hydrate, 250));
      }
    };

    hydrateNext(0);
    onCleanup(() => {
      idleHandles.forEach((handle) => window.cancelIdleCallback(handle));
      timeoutHandles.forEach((handle) => window.clearTimeout(handle));
    });
  });
  const shouldHydrateTab = (tab: ProxmoxPageTabId) =>
    activeTab() === tab || backgroundHydrationTabs().has(tab);
  const overviewResources = useUnifiedResources({
    query: PROXMOX_RESOURCE_QUERY_BY_TAB.overview,
    cacheKey: 'proxmox-overview',
    enabled: () => shouldHydrateTab('overview'),
    realtimeEnabled: () => activeTab() === 'overview',
  });
  const storageResources = useUnifiedResources({
    query: PROXMOX_RESOURCE_QUERY_BY_TAB.storage,
    cacheKey: 'proxmox-storage-shell',
    enabled: () => shouldHydrateTab('storage'),
    realtimeEnabled: () => activeTab() === 'storage',
  });
  const replicationResources = useUnifiedResources({
    query: PROXMOX_RESOURCE_QUERY_BY_TAB.replication,
    cacheKey: 'proxmox-replication-shell',
    enabled: () => shouldHydrateTab('replication'),
    realtimeEnabled: () => activeTab() === 'replication',
  });
  const backupResources = useUnifiedResources({
    query: PROXMOX_RESOURCE_QUERY_BY_TAB.backups,
    cacheKey: 'proxmox-backups-shell',
    enabled: () => shouldHydrateTab('backups'),
    realtimeEnabled: () => activeTab() === 'backups',
  });
  const cephResources = useUnifiedResources({
    query: PROXMOX_RESOURCE_QUERY_BY_TAB.ceph,
    cacheKey: 'proxmox-ceph',
    enabled: () => shouldHydrateTab('ceph'),
    realtimeEnabled: () => activeTab() === 'ceph',
  });
  const mailResources = useUnifiedResources({
    query: PROXMOX_RESOURCE_QUERY_BY_TAB.mail,
    cacheKey: 'proxmox-mail',
    enabled: () => shouldHydrateTab('mail'),
    realtimeEnabled: () => activeTab() === 'mail',
  });
  const [mountedTabs, setMountedTabs] = createSignal<Set<ProxmoxPageTabId>>(
    new Set([requestedTab()]),
  );
  const mountTab = (tab: ProxmoxPageTabId) =>
    setMountedTabs((current) => {
      const mountedOrder = [...current];
      if (mountedOrder[mountedOrder.length - 1] === tab) return current;
      const next = new Set(current);
      // Refresh insertion order so the least-recently-used tab is evicted.
      next.delete(tab);
      next.add(tab);
      if (phoneViewport) {
        while (next.size > PHONE_MOUNTED_TAB_LIMIT) {
          const oldestTab = next.values().next().value as ProxmoxPageTabId | undefined;
          if (!oldestTab) break;
          next.delete(oldestTab);
        }
      }
      return next;
    });
  createEffect(() => {
    mountTab(activeTab());
  });
  let storagePrewarmScheduled = false;
  createEffect(() => {
    if (
      storagePrewarmScheduled ||
      activeTab() !== 'overview' ||
      !backgroundHydrationTabs().has('storage') ||
      storageResources.resources().length === 0
    ) {
      return;
    }
    storagePrewarmScheduled = true;
    if (typeof window.requestIdleCallback === 'function') {
      const idleHandle = window.requestIdleCallback(() => mountTab('storage'), { timeout: 2_000 });
      onCleanup(() => window.cancelIdleCallback(idleHandle));
      return;
    }
    const timeoutHandle = window.setTimeout(() => mountTab('storage'), 250);
    onCleanup(() => window.clearTimeout(timeoutHandle));
  });
  const resourceState = createMemo(() => {
    switch (activeTab()) {
      case 'storage':
        return storageResources;
      case 'replication':
        return replicationResources;
      case 'backups':
        return backupResources;
      case 'ceph':
        return cephResources;
      case 'mail':
        return mailResources;
      default:
        return overviewResources;
    }
  });
  const loading = () => resourceState().loading();
  const error = () => resourceState().error();
  const refetch = () => resourceState().refetch();
  createEffect(() => {
    const next = resourceState().aggregations?.();
    if (next) setResourceAggregations(next);
  });
  const buildModel = (snapshot: Resource[] | undefined) =>
    buildProxmoxPageModel(Array.isArray(snapshot) ? snapshot : []);
  const overviewModel = createMemo(() => buildModel(overviewResources.resources()));
  const model = createMemo(() => {
    if (activeTab() === 'overview') return overviewModel();
    return buildModel(resourceState().resources());
  });
  const agentUpdateTargetVersion = createMemo(
    () => updateStore.versionInfo()?.agentUpdateTargetVersion,
  );
  const outdatedAgentHosts = createMemo(() =>
    collectOutdatedAgentHosts(model().pveNodes, agentUpdateTargetVersion()),
  );
  const outdatedAgentUpdatePath = createMemo(() =>
    buildInfrastructureAgentUpdatesPath(outdatedAgentHosts().map((host) => host.agentId)),
  );
  const serverVersionDisplay = createMemo(() =>
    formatAgentVersionDisplay(agentUpdateTargetVersion()),
  );
  const outdatedSensorSetupNodes = createMemo(() =>
    collectOutdatedSensorSetupNodes(
      model().pveNodes,
      model().resources.filter((resource) => resource.type === 'physical_disk'),
    ),
  );

  // The hosts table at the top and the embedded WorkloadsSurface below share
  // the bars/sparklines toggle (and the sparkline history range that ships
  // with it). Owning the persistent signals at the page level lets one
  // segmented control in the workloads filter drive both tables; the surface
  // accepts these as overrides so it skips its own internal persistent
  // signal and tracks the shared state directly.
  const [metricDisplayMode, setMetricDisplayMode] = usePersistentSignal<WorkloadsMetricDisplayMode>(
    STORAGE_KEYS.WORKLOADS_METRIC_DISPLAY_MODE,
    DEFAULT_WORKLOADS_METRIC_DISPLAY_MODE,
    {
      deserialize: (raw) =>
        raw === 'bars' || raw === 'sparklines' ? raw : DEFAULT_WORKLOADS_METRIC_DISPLAY_MODE,
    },
  );
  const [metricHistoryRange, setMetricHistoryRange] =
    usePersistentSignal<WorkloadTableMetricHistoryRange>(
      STORAGE_KEYS.WORKLOADS_METRIC_HISTORY_RANGE,
      WORKLOAD_TABLE_HISTORY_DEFAULT_RANGE,
      {
        deserialize: (raw) =>
          isWorkloadTableMetricHistoryRange(raw) ? raw : WORKLOAD_TABLE_HISTORY_DEFAULT_RANGE,
      },
    );
  const [memoryDisplayBasis, setMemoryDisplayBasis] =
    usePersistentSignal<WorkloadsMemoryDisplayBasis>(
      STORAGE_KEYS.WORKLOADS_MEMORY_DISPLAY_BASIS,
      'guest',
      {
        deserialize: (raw) => (raw === 'host' ? 'host' : 'guest'),
      },
    );
  const [inventoryCountsVisible, setInventoryCountsVisible] = usePersistentSignal(
    PLATFORM_ESTATE_COUNTS_STORAGE_KEY,
    true,
    { deserialize: deserializePlatformEstateCountsVisibility },
  );

  return (
    <div data-testid="proxmox-page" class="pulse-wide-data-surface space-y-3">
      <PlatformSectionTabs tabs={visibleTabs()} active={activeTab()} ariaLabel="Proxmox sections" />

      <Show
        when={!loading() || model().resources.length > 0}
        fallback={
          <PlatformTableLoadingState
            title="Loading Proxmox resources"
            description="Pulse is loading the Proxmox resource snapshot."
          />
        }
      >
        <Show
          when={!error()}
          fallback={
            <PlatformErrorState
              title="Could not load Proxmox resources"
              description="Refresh the resource snapshot or check the API connection state."
              onRefresh={() => void refetch()}
            />
          }
        >
          <Show
            when={model().resources.length > 0}
            fallback={
              <PlatformTableEmptyState
                icon={proxmoxIcon()}
                title="No Proxmox resources"
                description="Add Proxmox VE, Proxmox Backup Server, or Proxmox Mail Gateway in Settings to populate this platform page."
              />
            }
          >
            <PlatformOutdatedAgentNotice
              hosts={outdatedAgentHosts()}
              targetVersion={serverVersionDisplay()}
              missingLabel="agent-contributed Proxmox node detail and command support"
              copyVariant="latest-detail"
              actionHref={outdatedAgentUpdatePath()}
              actionLabel="Open agent upgrade commands"
            />
            <PlatformOutdatedSensorSetupNotice
              nodes={outdatedSensorSetupNodes()}
              actionHref={buildInfrastructureWorkspacePath()}
            />
            <Show when={mountedTabs().has('overview')}>
              <div
                class="space-y-4"
                classList={{ hidden: activeTab() !== 'overview' }}
                aria-hidden={activeTab() !== 'overview' ? 'true' : undefined}
              >
                <ProxmoxOverview
                  model={overviewModel}
                  active={() => activeTab() === 'overview'}
                  metricDisplayMode={metricDisplayMode}
                  setMetricDisplayMode={setMetricDisplayMode}
                  metricHistoryRange={metricHistoryRange}
                  setMetricHistoryRange={setMetricHistoryRange}
                  memoryDisplayBasis={memoryDisplayBasis}
                  setMemoryDisplayBasis={setMemoryDisplayBasis}
                  resourceSnapshot={() =>
                    overviewResources.loading() && overviewModel().resources.length === 0
                      ? undefined
                      : overviewModel().resources
                  }
                  resourceSnapshotRefetch={() => overviewResources.refetch()}
                  inventoryCountsVisible={inventoryCountsVisible}
                  setInventoryCountsVisible={setInventoryCountsVisible}
                />
              </div>
            </Show>
            <Show when={mountedTabs().has('storage')}>
              <div
                classList={{ hidden: activeTab() !== 'storage' }}
                aria-hidden={activeTab() !== 'storage' ? 'true' : undefined}
              >
                <StorageSurface
                  forcedSourceFilter={PROXMOX_PLATFORM_FILTER}
                  resourceSource={storageResources}
                  suppressNodeFilter
                  filterAriaLabel="Proxmox storage filters"
                  filterSearchPlaceholder="Search Proxmox storage by pool, datastore, node, or device"
                  filterSearchEmptyMessage="Recent Proxmox storage searches appear here."
                />
              </div>
            </Show>
            <Show when={activeTab() === 'replication'}>
              <ProxmoxReplicationTable
                jobs={replicationJobs.error ? undefined : replicationJobs()}
                error={replicationJobs.error}
                onRetry={() => void refetchReplicationJobs()}
                emptyIcon={<ProxmoxIcon class="h-6 w-6 text-slate-400" />}
                emptyTitle="No replication jobs"
                emptyDescription="Replication jobs appear here once PVE is configured to replicate guests between nodes."
              />
            </Show>
            <Show when={activeTab() === 'backups'}>
              <ProxmoxBackupsTable
                emptyIcon={<ProxmoxIcon class="h-6 w-6 text-slate-400" />}
                workloads={model().guests}
                servers={model().pbs}
              />
            </Show>
            <Show when={activeTab() === 'ceph'}>
              <ProxmoxCephTable
                resources={model().ceph.filter((resource) => resource.type === 'ceph')}
                emptyIcon={<ProxmoxIcon class="h-6 w-6 text-slate-400" />}
                emptyTitle="No Ceph clusters"
                emptyDescription="Ceph clusters appear here once a Proxmox VE node reports cluster topology."
              />
            </Show>
            <Show when={activeTab() === 'mail'}>
              <ProxmoxMailGatewayTable
                resources={model().pmg}
                emptyTitle="No Proxmox Mail Gateway instances"
                emptyDescription="PMG instances appear here once a Proxmox Mail Gateway connection reports them."
              />
            </Show>
          </Show>
        </Show>
      </Show>
    </div>
  );
}

interface ProxmoxOverviewProps {
  model: Accessor<ProxmoxPageModel>;
  active: Accessor<boolean>;
  metricDisplayMode: Accessor<WorkloadsMetricDisplayMode>;
  setMetricDisplayMode: (value: WorkloadsMetricDisplayMode) => void;
  metricHistoryRange: Accessor<WorkloadTableMetricHistoryRange>;
  setMetricHistoryRange: (value: WorkloadTableMetricHistoryRange) => void;
  memoryDisplayBasis: Accessor<WorkloadsMemoryDisplayBasis>;
  setMemoryDisplayBasis: (value: WorkloadsMemoryDisplayBasis) => void;
  resourceSnapshot: Accessor<Resource[] | undefined>;
  resourceSnapshotRefetch: () => Promise<unknown>;
  inventoryCountsVisible: Accessor<boolean>;
  setInventoryCountsVisible: (visible: boolean) => void;
}

function ProxmoxOverview(props: ProxmoxOverviewProps) {
  const currentModel = createMemo(() => props.model?.() ?? EMPTY_PROXMOX_PAGE_MODEL);
  const overviewWidth = useObservedElementWidth();
  const workloadsState = useWorkloadsState({
    vms: [],
    containers: [],
    nodes: [],
    layoutWidth: overviewWidth.width,
    useWorkloads: true,
    resourceSnapshot: props.resourceSnapshot,
    resourceSnapshotRefetch: props.resourceSnapshotRefetch,
    forcedPlatform: PROXMOX_PLATFORM_FILTER,
    excludedWorkloadTypes: PROXMOX_WORKLOAD_EXCLUDED_TYPES,
    showNestedExcludedWorkloads: true,
    suppressPlatformFilter: true,
    // The polymorphic 'info' column always holds the VMID on a pure-Proxmox
    // surface, so label it with the platform vocabulary instead of 'Info'.
    columnLabelOverrides: { info: 'ID' },
    statusModeStorageScope: PROXMOX_WORKLOAD_STATUS_STORAGE_SCOPE,
    compactGroupHeaders: true,
    groupNodeDrawerMode: 'disabled',
    metricDisplayMode: props.metricDisplayMode,
    onMetricDisplayModeChange: props.setMetricDisplayMode,
    metricHistoryRange: props.metricHistoryRange,
    onMetricHistoryRangeChange: props.setMetricHistoryRange,
    memoryDisplayBasis: props.memoryDisplayBasis,
    routeStateEnabled: props.active,
  });
  const showSharedFilterToolbar = createMemo(
    () =>
      workloadsState.surfaceConnected() &&
      workloadsState.surfaceInitialDataReceived() &&
      workloadsState.allGuests().length > 0,
  );
  const estateTopology = createMemo(() => buildProxmoxEstateTopology(currentModel().resources));

  return (
    <div ref={overviewWidth.setElement} class="pulse-wide-data-surface flex flex-col gap-4">
      <section>
        <ProxmoxNodesTable
          nodes={currentModel().pveNodes}
          guests={currentModel().guests}
          search={workloadsState.search}
          metricDisplayMode={props.metricDisplayMode}
          metricHistoryRange={props.metricHistoryRange}
          layoutWidth={overviewWidth.width}
          emptyIcon={<ProxmoxIcon class="h-6 w-6 text-slate-400" />}
          emptyTitle="No Proxmox VE nodes"
          emptyDescription="Proxmox VE nodes appear here once a PVE host reports inventory."
          topology={estateTopology()}
          inventoryCountsVisible={props.inventoryCountsVisible}
        />
      </section>
      <section
        id="proxmox-guests-section"
        aria-label="Guests"
        class="space-y-3 scroll-mt-4"
        data-testid="proxmox-guests-section"
      >
        <Show when={showSharedFilterToolbar()}>
          <div data-summary-clear-ignore>
            <WorkloadsFilter
              search={workloadsState.search}
              setSearch={workloadsState.setSearch}
              viewMode={workloadsState.viewMode}
              setViewMode={workloadsState.setViewMode}
              statusMode={workloadsState.statusMode}
              setStatusMode={workloadsState.setStatusMode}
              groupingMode={workloadsState.groupingMode}
              setGroupingMode={workloadsState.setGroupingMode}
              setSortKey={workloadsState.setSortKey}
              setSortDirection={workloadsState.setSortDirection}
              onBeforeAutoFocus={workloadsState.handleBeforeAutoFocus}
              ariaLabel="Proxmox workload filters"
              searchPlaceholder="Search VMs and LXCs by name, VMID, node, or status"
              searchEmptyMessage="Recent Proxmox workload searches appear here."
              searchSuggestionWorkloads={workloadsState.allGuests}
              statusOptions={PROXMOX_WORKLOAD_STATUS_OPTIONS}
              inventoryStats={workloadsState.inventoryStats}
              inventoryCountsVisible={props.inventoryCountsVisible}
              setInventoryCountsVisible={props.setInventoryCountsVisible}
              columnVisibility={workloadsState.workloadsFilterColumnVisibility()}
              containerRuntimeFilter={workloadsState.containerRuntimeFilterConfig()}
              hostFilter={workloadsState.hostFilterConfig()}
              namespaceFilter={undefined}
              platformFilter={undefined}
              metricDisplayMode={workloadsState.workloadMetricDisplayMode}
              setMetricDisplayMode={workloadsState.setWorkloadMetricDisplayMode}
              metricHistoryRange={workloadsState.workloadMetricHistoryRange}
              setMetricHistoryRange={workloadsState.setWorkloadMetricHistoryRange}
              memoryDisplayBasis={workloadsState.workloadMemoryDisplayBasis}
              setMemoryDisplayBasis={props.setMemoryDisplayBasis}
              forcedPlatform={PROXMOX_PLATFORM_FILTER}
              pinnedSelectionActive={() =>
                Boolean(
                  workloadsState.selectedGuestId() ||
                  workloadsState.focusedSummaryWorkloadGroupId(),
                )
              }
              onClearPinnedSelection={workloadsState.clearPinnedSummaryScope}
            />
          </div>
        </Show>
        <WorkloadsSurface
          state={workloadsState}
          vms={[]}
          containers={[]}
          nodes={[]}
          useWorkloads
          forcedPlatform={PROXMOX_PLATFORM_FILTER}
          excludedWorkloadTypes={PROXMOX_WORKLOAD_EXCLUDED_TYPES}
          showNestedExcludedWorkloads
          compactGroupHeaders
          groupNodeDrawerMode="disabled"
          suppressFilterToolbar
          emptyStateTitle="No Proxmox workloads"
          emptyStateDescription="Proxmox VMs and LXCs appear here when inventory is available."
          tableTitle={
            <h2 id="proxmox-guests-heading" class="inline-flex items-center gap-1.5">
              Guests
              <Show when={showSharedFilterToolbar()}>
                <span class="font-semibold tabular-nums text-base-content">
                  {workloadsState.allGuests().length}
                </span>
              </Show>
            </h2>
          }
        />
      </section>
    </div>
  );
}

export default ProxmoxPageSurface;
