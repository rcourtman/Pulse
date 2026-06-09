import { useLocation } from '@solidjs/router';
import { Show, createMemo, type Accessor } from 'solid-js';
import StorageSurface from '@/components/Storage/Storage';
import { WorkloadsFilter } from '@/components/Workloads/WorkloadsFilter';
import { WorkloadsSurface } from '@/components/Workloads/WorkloadsSurface';
import { useWorkloadsState } from '@/components/Workloads/useWorkloadsState';
import {
  DEFAULT_WORKLOADS_METRIC_DISPLAY_MODE,
  type WorkloadsStatusOption,
  type WorkloadsMetricDisplayMode,
} from '@/components/Workloads/workloadsFilterModel';
import {
  WORKLOAD_TABLE_HISTORY_DEFAULT_RANGE,
  isWorkloadTableMetricHistoryRange,
  type WorkloadTableMetricHistoryRange,
} from '@/components/Workloads/workloadMetricHistoryModel';
import { buildInfrastructureAgentUpdatesPath } from '@/components/Settings/infrastructureWorkspaceModel';
import {
  collectOutdatedAgentHosts,
  formatAgentVersionDisplay,
} from '@/features/platformPage/agentVersion';
import { getPlatformIcon } from '@/features/platformPage/platformIcon';
import { PlatformOutdatedAgentNotice } from '@/features/platformPage/PlatformOutdatedAgentNotice';
import { usePersistentSignal } from '@/hooks/usePersistentSignal';
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
import { ProxmoxReplicationTable } from './ProxmoxReplicationTable';
import { useUnifiedResources } from '@/hooks/useUnifiedResources';
import { updateStore } from '@/stores/updates';
import {
  PROXMOX_TAB_SPECS,
  buildProxmoxPageModel,
  buildVisibleProxmoxTabSpecs,
  filterProxmoxNodesForSearch,
  type ProxmoxPageModel,
  type ProxmoxPageTabId,
} from './proxmoxPageModel';

// `datastore`, `dataset`, and ZFS-style `pool` are not first-class
// resource type tokens at the API boundary — they all collapse to
// `storage` (with `storage.topology` differentiating dataset vs pool) or
// `ceph` (for Ceph pools). Including them in the type filter triggers a
// 400 from `/api/resources` which surfaces as "Could not load Proxmox
// resources" in the UI. The page model still filters those topologies
// client-side from the canonical `storage` rows.
const PROXMOX_RESOURCE_QUERY =
  'type=agent,vm,system-container,oci-container,storage,physical_disk,ceph,pbs,pmg';

const PROXMOX_PLATFORM_FILTER = 'proxmox-pve';
const PROXMOX_WORKLOAD_STATUS_STORAGE_SCOPE = 'proxmox';
const VALID_TABS = new Set<ProxmoxPageTabId>(PROXMOX_TAB_SPECS.map((tab) => tab.id));
const PROXMOX_WORKLOAD_STATUS_OPTIONS: readonly WorkloadsStatusOption[] = [
  { value: 'all', label: 'All' },
  { value: 'running', label: 'Running' },
  { value: 'degraded', label: 'Attention' },
  { value: 'stopped', label: 'Stopped' },
];

const ProxmoxIcon = getPlatformIcon('proxmox');
const proxmoxIcon = () => <ProxmoxIcon class="h-6 w-6 text-slate-400" />;

export function ProxmoxPageSurface() {
  const location = useLocation();
  const { resources, loading, error, refetch } = useUnifiedResources({
    query: PROXMOX_RESOURCE_QUERY,
    cacheKey: 'proxmox-workspace',
    initialHydration: 'prefer-ws-then-rest',
  });
  const model = createMemo(() => buildProxmoxPageModel(resources()));
  const visibleTabs = createMemo(() => buildVisibleProxmoxTabSpecs(model()));
  const visibleTabIds = createMemo(
    () => new Set<ProxmoxPageTabId>(visibleTabs().map((tab) => tab.id)),
  );
  const activeTab = createMemo<ProxmoxPageTabId>(() => {
    const segment = location.pathname.split('/').filter(Boolean)[1] as ProxmoxPageTabId | undefined;
    if (!segment || !VALID_TABS.has(segment)) return 'overview';
    return visibleTabIds().has(segment) ? segment : 'overview';
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

  return (
    <div data-testid="proxmox-page" class="space-y-3">
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
            <Show when={activeTab() === 'overview'}>
              <ProxmoxOverview
                model={model}
                metricDisplayMode={metricDisplayMode}
                setMetricDisplayMode={setMetricDisplayMode}
                metricHistoryRange={metricHistoryRange}
                setMetricHistoryRange={setMetricHistoryRange}
              />
            </Show>
            <Show when={activeTab() === 'storage'}>
              <StorageSurface
                forcedSourceFilter={PROXMOX_PLATFORM_FILTER}
                suppressNodeFilter
                filterAriaLabel="Proxmox storage filters"
                filterSearchPlaceholder="Search Proxmox storage by pool, datastore, node, or device"
                filterSearchEmptyMessage="Recent Proxmox storage searches appear here."
              />
            </Show>
            <Show when={activeTab() === 'replication'}>
              <ProxmoxReplicationTable
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
  metricDisplayMode: Accessor<WorkloadsMetricDisplayMode>;
  setMetricDisplayMode: (value: WorkloadsMetricDisplayMode) => void;
  metricHistoryRange: Accessor<WorkloadTableMetricHistoryRange>;
  setMetricHistoryRange: (value: WorkloadTableMetricHistoryRange) => void;
}

function ProxmoxOverview(props: ProxmoxOverviewProps) {
  const workloadsState = useWorkloadsState({
    vms: [],
    containers: [],
    nodes: [],
    useWorkloads: true,
    forcedPlatform: PROXMOX_PLATFORM_FILTER,
    suppressPlatformFilter: true,
    statusModeStorageScope: PROXMOX_WORKLOAD_STATUS_STORAGE_SCOPE,
    compactGroupHeaders: true,
    groupNodeDrawerMode: 'disabled',
    metricDisplayMode: props.metricDisplayMode,
    onMetricDisplayModeChange: props.setMetricDisplayMode,
    metricHistoryRange: props.metricHistoryRange,
    onMetricHistoryRangeChange: props.setMetricHistoryRange,
  });
  const showSharedFilterToolbar = createMemo(
    () =>
      workloadsState.surfaceConnected() &&
      workloadsState.surfaceInitialDataReceived() &&
      workloadsState.allGuests().length > 0,
  );
  const filteredNodes = createMemo(() =>
    filterProxmoxNodesForSearch(
      props.model().pveNodes,
      props.model().guests,
      workloadsState.search(),
    ),
  );

  return (
    <div class="space-y-4">
      <Show when={showSharedFilterToolbar()}>
        <div data-summary-clear-ignore>
          <WorkloadsFilter
            savedViewsKey={workloadsState.savedViewsKey()}
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
            statusOptions={PROXMOX_WORKLOAD_STATUS_OPTIONS}
            columnVisibility={workloadsState.workloadsFilterColumnVisibility()}
            containerRuntimeFilter={workloadsState.containerRuntimeFilterConfig()}
            hostFilter={workloadsState.hostFilterConfig()}
            namespaceFilter={undefined}
            platformFilter={undefined}
            metricDisplayMode={workloadsState.workloadMetricDisplayMode}
            setMetricDisplayMode={workloadsState.setWorkloadMetricDisplayMode}
            metricHistoryRange={workloadsState.workloadMetricHistoryRange}
            setMetricHistoryRange={workloadsState.setWorkloadMetricHistoryRange}
            forcedPlatform={PROXMOX_PLATFORM_FILTER}
            pinnedSelectionActive={() =>
              Boolean(
                workloadsState.selectedGuestId() || workloadsState.focusedSummaryWorkloadGroupId(),
              )
            }
            onClearPinnedSelection={workloadsState.clearPinnedSummaryScope}
          />
        </div>
      </Show>
      <ProxmoxNodesTable
        nodes={filteredNodes()}
        guests={props.model().guests}
        metricDisplayMode={props.metricDisplayMode}
        metricHistoryRange={props.metricHistoryRange}
        emptyIcon={<ProxmoxIcon class="h-6 w-6 text-slate-400" />}
        emptyTitle="No Proxmox VE nodes"
        emptyDescription="Proxmox VE nodes appear here once a PVE host reports inventory."
      />
      <WorkloadsSurface
        state={workloadsState}
        vms={[]}
        containers={[]}
        nodes={[]}
        useWorkloads
        forcedPlatform={PROXMOX_PLATFORM_FILTER}
        compactGroupHeaders
        groupNodeDrawerMode="disabled"
        suppressFilterToolbar
        emptyStateTitle="No Proxmox workloads"
        emptyStateDescription="Proxmox VMs and LXCs appear here when inventory is available."
      />
    </div>
  );
}

export default ProxmoxPageSurface;
