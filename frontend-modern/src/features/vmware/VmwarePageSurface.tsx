import { useLocation } from '@solidjs/router';
import CpuIcon from 'lucide-solid/icons/cpu';
import { Show, createMemo, type Accessor } from 'solid-js';
import StorageSurface from '@/components/Storage/Storage';
import { WorkloadsFilter } from '@/components/Workloads/WorkloadsFilter';
import { WorkloadsSurface } from '@/components/Workloads/WorkloadsSurface';
import { useWorkloadsState } from '@/components/Workloads/useWorkloadsState';
import type { WorkloadsStatusOption } from '@/components/Workloads/workloadsFilterModel';
import { useUnifiedResources } from '@/hooks/useUnifiedResources';
import { resourceMatchesSearch } from '@/utils/resourceSearchMatch';
import {
  PlatformErrorState,
  PlatformSectionTabs,
  PlatformTableEmptyState,
} from '@/features/platformPage/sharedPlatformPage';
import { VsphereHostsTable } from './VsphereHostsTable';
import {
  VMWARE_TAB_SPECS,
  buildVmwarePageModel,
  type VmwarePageModel,
  type VmwarePageTabId,
} from './vmwarePageModel';

// `datastore` is not a first-class type token at the API boundary; vSphere
// datastores are emitted as canonical `storage` rows. Including it
// triggers a 400 from `/api/resources`.
const VMWARE_RESOURCE_QUERY = 'type=agent,vm,storage';
const VMWARE_PLATFORM_FILTER = 'vmware-vsphere';
const VALID_TABS = new Set<VmwarePageTabId>(VMWARE_TAB_SPECS.map((tab) => tab.id));
const VMWARE_VM_STATUS_OPTIONS: readonly WorkloadsStatusOption[] = [
  { value: 'all', label: 'All' },
  { value: 'running', label: 'Powered on' },
  { value: 'degraded', label: 'Attention' },
  { value: 'stopped', label: 'Powered off' },
];

const vmwareIcon = () => <CpuIcon class="h-6 w-6 text-slate-400" />;

export function VmwarePageSurface() {
  const location = useLocation();
  const { resources, loading, error, refetch } = useUnifiedResources({
    query: VMWARE_RESOURCE_QUERY,
    cacheKey: 'vmware-workspace',
    initialHydration: 'prefer-ws-then-rest',
  });
  const activeTab = createMemo<VmwarePageTabId>(() => {
    const segment = location.pathname.split('/').filter(Boolean)[1] as VmwarePageTabId | undefined;
    return segment && VALID_TABS.has(segment) ? segment : 'overview';
  });
  const model = createMemo(() => buildVmwarePageModel(resources()));

  return (
    <div data-testid="vmware-page" class="space-y-3">
      <PlatformSectionTabs
        tabs={VMWARE_TAB_SPECS}
        active={activeTab()}
        ariaLabel="VMware sections"
      />

      <Show
        when={!loading() || model().resources.length > 0}
        fallback={
          <PlatformTableEmptyState
            icon={vmwareIcon()}
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
            <Show when={activeTab() === 'overview'}>
              <VmwareOverview model={model} />
            </Show>
            <Show when={activeTab() === 'storage'}>
              <StorageSurface
                embedded
                tableOnly
                showFilterToolbar
                forcedSourceFilter={VMWARE_PLATFORM_FILTER}
                forcedView="pools"
                filterAriaLabel="vSphere datastore filters"
                filterSearchPlaceholder="Search vSphere datastores by name, host, or capacity group"
                filterSearchEmptyMessage="Recent vSphere datastore searches appear here."
              />
            </Show>
          </Show>
        </Show>
      </Show>
    </div>
  );
}

interface VmwareOverviewProps {
  model: Accessor<VmwarePageModel>;
}

function VmwareOverview(props: VmwareOverviewProps) {
  const workloadsState = useWorkloadsState({
    vms: [],
    containers: [],
    nodes: [],
    useWorkloads: true,
    embedded: true,
    tableOnly: true,
    forcedPlatform: VMWARE_PLATFORM_FILTER,
    forcedViewMode: 'vm',
    showFilterToolbar: true,
    suppressPlatformFilter: true,
    allowEmbeddedScopeFilters: true,
    compactGroupHeaders: true,
  });
  const showSharedFilterToolbar = createMemo(
    () =>
      workloadsState.surfaceConnected() &&
      workloadsState.surfaceInitialDataReceived() &&
      workloadsState.allGuests().length > 0,
  );
  const filteredHosts = createMemo(() => {
    const term = workloadsState.search().trim();
    if (!term) return props.model().hosts;
    return props.model().hosts.filter((host) => resourceMatchesSearch(host, term));
  });

  return (
    <div class="space-y-4">
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
            ariaLabel="vSphere VM filters"
            searchPlaceholder="Search vSphere VMs by name, VM ID, host, or status"
            searchEmptyMessage="Recent vSphere VM searches appear here."
            statusOptions={VMWARE_VM_STATUS_OPTIONS}
            columnVisibility={workloadsState.workloadsFilterColumnVisibility()}
            containerRuntimeFilter={workloadsState.containerRuntimeFilterConfig()}
            hostFilter={workloadsState.hostFilterConfig()}
            namespaceFilter={workloadsState.namespaceFilterConfig()}
            platformFilter={undefined}
            suppressTypeFilter
            metricDisplayMode={workloadsState.workloadMetricDisplayMode}
            setMetricDisplayMode={workloadsState.setWorkloadMetricDisplayMode}
            metricHistoryRange={workloadsState.workloadMetricHistoryRange}
            setMetricHistoryRange={workloadsState.setWorkloadMetricHistoryRange}
            forcedPlatform={VMWARE_PLATFORM_FILTER}
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
      <VsphereHostsTable
        hosts={filteredHosts()}
        scope={props.model().resources}
        emptyIcon={vmwareIcon()}
        emptyTitle="No vSphere hosts"
        emptyDescription="Hosts appear here once the vCenter connection enumerates them."
        showToolbar={false}
      />
      <WorkloadsSurface
        state={workloadsState}
        vms={[]}
        containers={[]}
        nodes={[]}
        useWorkloads
        embedded
        tableOnly
        forcedPlatform={VMWARE_PLATFORM_FILTER}
        forcedViewMode="vm"
        compactGroupHeaders
      />
    </div>
  );
}

export default VmwarePageSurface;
