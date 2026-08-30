import { useLocation } from '@solidjs/router';
import {
  For,
  Show,
  createEffect,
  createMemo,
  createResource,
  createSignal,
  type Accessor,
} from 'solid-js';
import AlertTriangle from 'lucide-solid/icons/triangle-alert';
import { buildInfrastructureAgentUpdatesPath } from '@/components/Settings/infrastructureWorkspaceModel';
import { getPlatformIcon } from '@/features/platformPage/platformIcon';
import { PlatformOutdatedAgentNotice } from '@/features/platformPage/PlatformOutdatedAgentNotice';
import {
  collectOutdatedAgentHosts,
  formatAgentVersionDisplay,
} from '@/features/platformPage/agentVersion';
import { ResourceAPI } from '@/api/resources';
import {
  RuntimeInventorySourcesAPI,
  type RuntimeInventorySource,
} from '@/api/runtimeInventorySources';
import { useUnifiedResources, type UnifiedResourceFacets } from '@/hooks/useUnifiedResources';
import { createNonSuspendingQuery } from '@/hooks/createNonSuspendingQuery';
import { updateStore } from '@/stores/updates';
import {
  PlatformErrorState,
  PlatformSectionTabs,
  PlatformTableEmptyState,
  PlatformTableLoadingState,
} from '@/features/platformPage/sharedPlatformPage';
import { WorkloadsFilter } from '@/components/Workloads/WorkloadsFilter';
import { WorkloadsSurface } from '@/components/Workloads/WorkloadsSurface';
import {
  useWorkloadsState,
  type WorkloadsInventorySourcesQuery,
} from '@/components/Workloads/useWorkloadsState';
import {
  DEFAULT_WORKLOADS_METRIC_DISPLAY_MODE,
  getWorkloadsMetricFilterProps,
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
import type { Resource } from '@/types/resource';
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
const VMWARE_RESOURCE_QUERY_BY_TAB: Record<VmwarePageTabId, string> = {
  overview: 'type=agent,vm&source=vmware-vsphere',
  storage: 'type=agent,storage&source=vmware-vsphere',
  networks: 'type=agent,network&source=vmware-vsphere',
  health: 'type=agent,vm,storage,network&source=vmware-vsphere',
  activity: 'type=agent&source=vmware-vsphere',
};
const VALID_TABS = new Set<VmwarePageTabId>(VMWARE_TAB_SPECS.map((tab) => tab.id));

const VMWARE_PLATFORM_FILTER = 'vmware-vsphere';
const VMWARE_WORKLOAD_STATUS_STORAGE_SCOPE = 'vmware';
const VMWARE_INVENTORY_SOURCES_POLL_MS = 15_000;
const VMWARE_WORKLOAD_COLUMN_VISIBILITY_SCOPE = 'vmware-vms';
// Backup column on the workload table is driven exclusively by Proxmox
// vzdump / PBS data (`resource.proxmox.lastBackup` in useWorkloads).
// vCenter has no native backup concept — vSphere backups happen in
// third-party products (Veeam, Commvault, Rubrik, Cohesity, Dell
// PowerProtect) or VMware's separately-licensed Live Recovery / SRM,
// none of which surface through vCenter's inventory API. Hide the
// column by default rather than render a permanently blank cell.
//
// `tags` used to be hidden here too, as a stopgap: the adapter filled
// `Resource.Tags` with fixed provenance strings and never read vCenter's own
// tag/category system, so the column repeated the same dots on every row. The
// adapter now ingests real vCenter tags through the CIS tagging service
// (`internal/vmware/client_tags.go`), `useWorkloads` reads the operator's tags
// from the `vmware.tags` facet rather than the mixed keyword set, and the
// column carries per-row meaning, so the hide is gone.
const VMWARE_WORKLOAD_DEFAULT_HIDDEN_COLUMN_IDS: readonly string[] = ['backup'];
const VMWARE_WORKLOAD_STATUS_OPTIONS: readonly WorkloadsStatusOption[] = [
  { value: 'all', label: 'All' },
  { value: 'running', label: 'Powered on' },
  { value: 'degraded', label: 'Attention' },
  { value: 'stopped', label: 'Powered off' },
];

const VmwareIcon = getPlatformIcon('vmware');
const vmwareIcon = () => <VmwareIcon class="h-6 w-6 text-slate-400" />;

function VmwareInventoryCompletenessNotice(props: {
  error: unknown;
  onRetry: () => void;
  sources: RuntimeInventorySource[];
}) {
  const degradedSources = createMemo(() =>
    props.sources.filter(
      (source) => source.type === 'vmware' && source.completeness?.state === 'degraded',
    ),
  );

  return (
    <Show when={props.error || degradedSources().length > 0}>
      <div
        class="rounded-sm border border-amber-300 bg-amber-50/70 px-3 py-2.5 text-sm text-amber-950 dark:border-amber-900/70 dark:bg-amber-950/20 dark:text-amber-100"
        data-testid="vmware-inventory-completeness-notice"
      >
        <div class="flex items-start gap-2">
          <AlertTriangle class="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
          <div class="min-w-0 flex-1 space-y-1">
            <p class="font-semibold">
              {props.error
                ? 'vSphere inventory completeness is unavailable'
                : 'Some vSphere inventory details are incomplete'}
            </p>
            <Show
              when={!props.error}
              fallback={
                <p class="text-xs leading-5 text-amber-800 dark:text-amber-200">
                  Pulse could not read collection diagnostics, so this page cannot confirm that the
                  current vCenter inventory is complete.{' '}
                  <button class="font-semibold underline" type="button" onClick={props.onRetry}>
                    Retry
                  </button>
                </p>
              }
            >
              <ul class="space-y-1 text-xs leading-5 text-amber-800 dark:text-amber-200">
                <For each={degradedSources()}>
                  {(source) => (
                    <li>
                      {source.name}: the last successful collection reported{' '}
                      {source.completeness?.issueCount ?? 0}{' '}
                      {(source.completeness?.issueCount ?? 0) === 1
                        ? 'optional read issue'
                        : 'optional read issues'}
                      . Affected details may be absent from this page.
                    </li>
                  )}
                </For>
              </ul>
            </Show>
          </div>
        </div>
      </div>
    </Show>
  );
}

export function VmwarePageSurface() {
  const location = useLocation();
  const inventorySources = createNonSuspendingQuery({
    source: () => 'enabled',
    fetcher: () => RuntimeInventorySourcesAPI.list(),
    initialValue: { sources: [] },
    cacheKey: (key) => `workloads-inventory-sources:${key}`,
    pollMs: VMWARE_INVENTORY_SOURCES_POLL_MS,
  });
  const requestedTab = createMemo<VmwarePageTabId>(() => {
    const segment = location.pathname.split('/').filter(Boolean)[1] as VmwarePageTabId | undefined;
    return segment && VALID_TABS.has(segment) ? segment : 'overview';
  });
  const [navigationResources, setNavigationResources] = createSignal<Resource[]>([]);
  const [resourceFacets, setResourceFacets] = createSignal<UnifiedResourceFacets | null>(null);
  const [activityEvidence] = createResource(async () =>
    ResourceAPI.getGlobalTimeline({
      limit: 1,
      kind: 'activity',
      sourceType: 'platform_event',
      sourceAdapter: 'vmware_adapter',
    }),
  );
  const navigationModel = createMemo(() => buildVmwarePageModel(navigationResources(), []));
  const tabs = createMemo(() => {
    const facets = resourceFacets();
    if (!facets || activityEvidence.loading || activityEvidence.error) {
      return navigationResources().length > 0
        ? getVmwarePageTabSpecs(navigationModel(), {
            hasActivityInventory: (activityEvidence()?.recentChanges?.length ?? 0) > 0,
          })
        : VMWARE_TAB_SPECS;
    }
    return getVmwarePageTabSpecs(navigationModel(), {
      typeCounts: facets.byType,
      incidentCount: facets.incidentCount,
      hasActivityInventory: (activityEvidence()?.recentChanges?.length ?? 0) > 0,
    });
  });
  const activeTab = createMemo<VmwarePageTabId>(() =>
    tabs().some((tab) => tab.id === requestedTab()) ? requestedTab() : 'overview',
  );
  const shouldHydrateTab = (tab: VmwarePageTabId) => activeTab() === tab;
  const createTabResources = (tab: VmwarePageTabId) =>
    useUnifiedResources({
      query: VMWARE_RESOURCE_QUERY_BY_TAB[tab],
      cacheKey: `vmware-${tab}`,
      initialHydration: 'prefer-ws-then-rest',
      enabled: () => shouldHydrateTab(tab),
      realtimeEnabled: () => shouldHydrateTab(tab),
    });
  const overviewResources = createTabResources('overview');
  const storageResources = createTabResources('storage');
  const networkResources = createTabResources('networks');
  const healthResources = createTabResources('health');
  const activityResources = createTabResources('activity');
  const resourceState = createMemo(() => {
    switch (activeTab()) {
      case 'storage':
        return storageResources;
      case 'networks':
        return networkResources;
      case 'health':
        return healthResources;
      case 'activity':
        return activityResources;
      default:
        return overviewResources;
    }
  });
  const resources = () => resourceState().resources();
  const loading = () => resourceState().loading();
  const error = () => resourceState().error();
  const refetch = () => resourceState().refetch();
  createEffect(() => {
    const state = resourceState();
    setNavigationResources(state.resources());
    const nextFacets = state.facets?.();
    if (nextFacets) setResourceFacets(nextFacets);
  });
  const [activityTimeline, { refetch: refetchActivityTimeline }] = createResource(
    () => (activeTab() === 'activity' ? 'vmware-activity' : undefined),
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
    <div data-testid="vmware-page" class="pulse-wide-data-surface space-y-3">
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
          <VmwareInventoryCompletenessNotice
            error={inventorySources.error()}
            onRetry={() => void inventorySources.refetch()}
            sources={inventorySources.value().sources ?? []}
          />
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
              <div class="space-y-4">
                <VmwareOverview
                  model={model}
                  metricDisplayMode={metricDisplayMode}
                  setMetricDisplayMode={setMetricDisplayMode}
                  metricHistoryRange={metricHistoryRange}
                  setMetricHistoryRange={setMetricHistoryRange}
                  inventorySourcesQuery={inventorySources}
                  resourceSnapshot={() =>
                    loading() && model().resources.length === 0 ? undefined : model().resources
                  }
                  resourceSnapshotRefetch={() => refetch()}
                />
              </div>
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
  inventorySourcesQuery: WorkloadsInventorySourcesQuery;
  resourceSnapshot: Accessor<Resource[] | undefined>;
  resourceSnapshotRefetch: () => Promise<unknown>;
}

function VmwareOverview(props: VmwareOverviewProps) {
  const workloadsState = useWorkloadsState({
    vms: [],
    containers: [],
    nodes: [],
    useWorkloads: true,
    resourceSnapshot: props.resourceSnapshot,
    resourceSnapshotRefetch: props.resourceSnapshotRefetch,
    forcedPlatform: VMWARE_PLATFORM_FILTER,
    forcedViewMode: 'vm',
    suppressPlatformFilter: true,
    statusModeStorageScope: VMWARE_WORKLOAD_STATUS_STORAGE_SCOPE,
    columnVisibilityStorageScope: VMWARE_WORKLOAD_COLUMN_VISIBILITY_SCOPE,
    additionalDefaultHiddenColumnIds: [...VMWARE_WORKLOAD_DEFAULT_HIDDEN_COLUMN_IDS],
    compactGroupHeaders: true,
    inventorySourcesQuery: props.inventorySourcesQuery,
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
            searchSuggestionWorkloads={workloadsState.allGuests}
            statusOptions={VMWARE_WORKLOAD_STATUS_OPTIONS}
            inventoryStats={workloadsState.inventoryStats}
            suppressTypeFilter
            columnVisibility={workloadsState.workloadsFilterColumnVisibility()}
            containerRuntimeFilter={workloadsState.containerRuntimeFilterConfig()}
            hostFilter={workloadsState.hostFilterConfig()}
            namespaceFilter={undefined}
            clusterFilter={workloadsState.clusterFilterConfig()}
            platformFilter={undefined}
            {...getWorkloadsMetricFilterProps(workloadsState)}
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
