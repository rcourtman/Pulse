import { useLocation } from '@solidjs/router';
import { Show, createMemo, createResource, type Accessor } from 'solid-js';
import { buildInfrastructureAgentUpdatesPath } from '@/components/Settings/infrastructureWorkspaceModel';
import { getPlatformIcon } from '@/features/platformPage/platformIcon';
import { PlatformOutdatedAgentNotice } from '@/features/platformPage/PlatformOutdatedAgentNotice';
import {
  collectOutdatedAgentHosts,
  formatAgentVersionDisplay,
} from '@/features/platformPage/agentVersion';
import { ResourceAPI } from '@/api/resources';
import { useUnifiedResources } from '@/hooks/useUnifiedResources';
import { updateStore } from '@/stores/updates';
import {
  PlatformErrorState,
  PlatformSectionTabs,
  PlatformTableEmptyState,
  PlatformTableLoadingState,
} from '@/features/platformPage/sharedPlatformPage';
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
import { usePersistentSignal } from '@/hooks/usePersistentSignal';
import { STORAGE_KEYS } from '@/utils/localStorage';
import { VsphereHostsTable } from './VsphereHostsTable';
import {
  VMWARE_TAB_SPECS,
  buildVmwarePageModel,
  getVmwarePageTabSpecs,
  type VmwarePageModel,
  type VmwarePageTabId,
} from './vmwarePageModel';
import { VsphereAlertsTable } from './VsphereAlertsTable';
import { VsphereActivityTable } from './VsphereActivityTable';
import { VsphereDatastoresTable } from './VsphereDatastoresTable';
import { VsphereNetworksTable } from './VsphereNetworksTable';

// vSphere phase 1 projects ESXi hosts as canonical `agent`, virtual machines
// as canonical `vm`, datastores as canonical `storage`, and vCenter networks
// as canonical `network`; provider-native topology stays in VMware metadata
// under those shared resources.
const VMWARE_RESOURCE_QUERY = 'type=agent,vm,storage,network';
const VALID_TABS = new Set<VmwarePageTabId>(VMWARE_TAB_SPECS.map((tab) => tab.id));

const VMWARE_PLATFORM_FILTER = 'vmware-vsphere';
const VMWARE_WORKLOAD_STATUS_STORAGE_SCOPE = 'vmware';
const VMWARE_WORKLOAD_COLUMN_VISIBILITY_SCOPE = 'vmware-vms';
// Backup column on the workload table is driven exclusively by Proxmox
// vzdump / PBS data (`resource.proxmox.lastBackup` in useWorkloads).
// vCenter has no native backup concept — vSphere backups happen in
// third-party products (Veeam, Commvault, Rubrik, Cohesity, Dell
// PowerProtect) or VMware's separately-licensed Live Recovery / SRM,
// none of which surface through vCenter's inventory API. Hide the
// column by default rather than render a permanently blank cell.
const VMWARE_WORKLOAD_DEFAULT_HIDDEN_COLUMN_IDS: readonly string[] = ['backup'];
const VMWARE_WORKLOAD_STATUS_OPTIONS: readonly WorkloadsStatusOption[] = [
  { value: 'all', label: 'All' },
  { value: 'running', label: 'Powered on' },
  { value: 'degraded', label: 'Attention' },
  { value: 'stopped', label: 'Powered off' },
];

const VmwareIcon = getPlatformIcon('vmware');
const vmwareIcon = () => <VmwareIcon class="h-6 w-6 text-slate-400" />;

export function VmwarePageSurface() {
  const location = useLocation();
  const { resources, loading, error, refetch } = useUnifiedResources({
    query: VMWARE_RESOURCE_QUERY,
    cacheKey: 'vmware-workspace',
    initialHydration: 'prefer-ws-then-rest',
  });
  const requestedTab = createMemo<VmwarePageTabId>(() => {
    const segment = location.pathname.split('/').filter(Boolean)[1] as VmwarePageTabId | undefined;
    return segment && VALID_TABS.has(segment) ? segment : 'overview';
  });
  const [activityTimeline, { refetch: refetchActivityTimeline }] = createResource(
    () => (resources().length > 0 ? 'vmware-activity' : undefined),
    async () => {
      const response = await ResourceAPI.getGlobalTimeline({
        limit: 100,
        kind: 'activity',
        sourceType: 'platform_event',
        sourceAdapter: 'vmware_adapter',
      });
      return response.recentChanges ?? [];
    },
  );
  const model = createMemo(() => buildVmwarePageModel(resources(), activityTimeline() ?? []));
  const tabs = createMemo(() => getVmwarePageTabSpecs(model()));
  const activeTab = createMemo<VmwarePageTabId>(() =>
    tabs().some((tab) => tab.id === requestedTab()) ? requestedTab() : 'overview',
  );
  const agentUpdateTargetVersion = createMemo(
    () => updateStore.versionInfo()?.agentUpdateTargetVersion,
  );
  const outdatedAgentVMs = createMemo(() =>
    collectOutdatedAgentHosts(model().vms, agentUpdateTargetVersion()),
  );
  const outdatedAgentUpdatePath = createMemo(() =>
    buildInfrastructureAgentUpdatesPath(outdatedAgentVMs().map((vm) => vm.agentId)),
  );
  const serverVersionDisplay = createMemo(() =>
    formatAgentVersionDisplay(agentUpdateTargetVersion()),
  );

  // Hosts table on top and the embedded WorkloadsSurface below share the
  // bars/sparklines toggle (and the sparkline history range that ships with
  // it). Owning the persistent signals at the page level keeps the same shape
  // Proxmox uses, so the in-toolbar segmented control drives both.
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
    <div data-testid="vmware-page" class="space-y-3">
      <PlatformSectionTabs tabs={tabs()} active={activeTab()} ariaLabel="VMware sections" />

      <Show
        when={!loading() || model().resources.length > 0}
        fallback={
          <PlatformTableLoadingState
            title="Loading VMware resources"
            description="Pulse is loading the VMware vSphere resource snapshot."
          />
        }
      >
        <Show
          when={!error()}
          fallback={
            <PlatformErrorState
              title="Could not load VMware resources"
              description="Refresh the resource snapshot or check the API connection state."
              onRefresh={() => void refetch()}
            />
          }
        >
          <Show
            when={model().resources.length > 0}
            fallback={
              <PlatformTableEmptyState
                icon={vmwareIcon()}
                title="No vSphere hosts"
                description="Add a vCenter connection in Settings to populate this page."
              />
            }
          >
            <PlatformOutdatedAgentNotice
              hosts={outdatedAgentVMs()}
              targetVersion={serverVersionDisplay()}
              missingLabel="in-guest telemetry and command support"
              copyVariant="latest-detail"
              actionHref={outdatedAgentUpdatePath()}
              actionLabel="Open agent upgrade commands"
              subjectSingular="VM"
              subjectPlural="VMs"
            />
            <Show when={activeTab() === 'overview'}>
              <VmwareOverview
                model={model}
                metricDisplayMode={metricDisplayMode}
                setMetricDisplayMode={setMetricDisplayMode}
                metricHistoryRange={metricHistoryRange}
                setMetricHistoryRange={setMetricHistoryRange}
              />
            </Show>
            <Show when={activeTab() === 'storage'}>
              <VsphereDatastoresTable
                datastores={model().datastores}
                scope={model().resources}
                emptyIcon={vmwareIcon()}
                emptyTitle="No vSphere datastores"
                emptyDescription="Datastores appear here once the vCenter connection enumerates them."
              />
            </Show>
            <Show when={activeTab() === 'networks'}>
              <VsphereNetworksTable
                networks={model().networks}
                scope={model().resources}
                emptyIcon={vmwareIcon()}
                emptyTitle="No vSphere networks"
                emptyDescription="Networks appear here once the vCenter connection enumerates them."
              />
            </Show>
            <Show when={activeTab() === 'health'}>
              <VsphereAlertsTable
                incidents={model().incidents}
                emptyIcon={vmwareIcon()}
                emptyTitle="No active vSphere health signals"
                emptyDescription="vSphere triggered alarms and overall health signals appear here when vCenter reports them."
              />
            </Show>
            <Show when={activeTab() === 'activity'}>
              <Show
                when={!activityTimeline.error || model().activity.length > 0}
                fallback={
                  <PlatformErrorState
                    title="Could not load vSphere activity"
                    description="Refresh the vSphere activity timeline or check the API connection state."
                    onRefresh={() => void refetchActivityTimeline()}
                  />
                }
              >
                <Show
                  when={!activityTimeline.loading || model().activity.length > 0}
                  fallback={
                    <PlatformTableLoadingState
                      title="Loading vSphere activity"
                      description="Pulse is loading recent vCenter tasks and events."
                    />
                  }
                >
                  <VsphereActivityTable
                    activity={model().activity}
                    emptyIcon={vmwareIcon()}
                    emptyTitle="No vSphere activity"
                    emptyDescription="Recent vCenter tasks and events appear here when the vCenter connection reports them."
                  />
                </Show>
              </Show>
            </Show>
          </Show>
        </Show>
      </Show>
    </div>
  );
}

interface VmwareOverviewProps {
  model: Accessor<VmwarePageModel>;
  metricDisplayMode: Accessor<WorkloadsMetricDisplayMode>;
  setMetricDisplayMode: (value: WorkloadsMetricDisplayMode) => void;
  metricHistoryRange: Accessor<WorkloadTableMetricHistoryRange>;
  setMetricHistoryRange: (value: WorkloadTableMetricHistoryRange) => void;
}

function VmwareOverview(props: VmwareOverviewProps) {
  const workloadsState = useWorkloadsState({
    vms: [],
    containers: [],
    nodes: [],
    useWorkloads: true,
    forcedPlatform: VMWARE_PLATFORM_FILTER,
    forcedViewMode: 'vm',
    suppressPlatformFilter: true,
    statusModeStorageScope: VMWARE_WORKLOAD_STATUS_STORAGE_SCOPE,
    columnVisibilityStorageScope: VMWARE_WORKLOAD_COLUMN_VISIBILITY_SCOPE,
    additionalDefaultHiddenColumnIds: [...VMWARE_WORKLOAD_DEFAULT_HIDDEN_COLUMN_IDS],
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

  return (
    <div class="space-y-4">
      <VsphereHostsTable
        hosts={props.model().hosts}
        scope={props.model().resources}
        emptyIcon={vmwareIcon()}
        emptyTitle="No vSphere hosts"
        emptyDescription="Hosts appear here once the vCenter connection enumerates them."
        showToolbar={false}
      />
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
            ariaLabel="vSphere workload filters"
            searchPlaceholder="Search vSphere VMs by name, host, cluster, or status"
            searchEmptyMessage="Recent vSphere workload searches appear here."
            statusOptions={VMWARE_WORKLOAD_STATUS_OPTIONS}
            suppressTypeFilter
            columnVisibility={workloadsState.workloadsFilterColumnVisibility()}
            containerRuntimeFilter={workloadsState.containerRuntimeFilterConfig()}
            hostFilter={workloadsState.hostFilterConfig()}
            namespaceFilter={undefined}
            clusterFilter={workloadsState.clusterFilterConfig()}
            platformFilter={undefined}
            metricDisplayMode={workloadsState.workloadMetricDisplayMode}
            setMetricDisplayMode={workloadsState.setWorkloadMetricDisplayMode}
            metricHistoryRange={workloadsState.workloadMetricHistoryRange}
            setMetricHistoryRange={workloadsState.setWorkloadMetricHistoryRange}
            forcedPlatform={VMWARE_PLATFORM_FILTER}
            pinnedSelectionActive={() =>
              Boolean(
                workloadsState.selectedGuestId() || workloadsState.focusedSummaryWorkloadGroupId(),
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
        forcedPlatform={VMWARE_PLATFORM_FILTER}
        forcedViewMode="vm"
        compactGroupHeaders
        groupNodeDrawerMode="disabled"
        suppressFilterToolbar
        emptyStateTitle="No vSphere VMs"
        emptyStateDescription="Virtual machines appear here once the vCenter connection enumerates them."
      />
    </div>
  );
}

export default VmwarePageSurface;
