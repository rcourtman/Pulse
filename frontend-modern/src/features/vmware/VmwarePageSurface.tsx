import { useLocation } from '@solidjs/router';
import CpuIcon from 'lucide-solid/icons/cpu';
import { Show, createMemo, createResource, type Accessor } from 'solid-js';
import { ResourceAPI } from '@/api/resources';
import { useUnifiedResources } from '@/hooks/useUnifiedResources';
import {
  PlatformErrorState,
  PlatformSectionTabs,
  PlatformTableEmptyState,
  PlatformTableLoadingState,
} from '@/features/platformPage/sharedPlatformPage';
import { VsphereHostsTable } from './VsphereHostsTable';
import {
  VMWARE_TAB_SPECS,
  buildVmwarePageModel,
  type VmwarePageModel,
  type VmwarePageTabId,
} from './vmwarePageModel';
import { VsphereAlertsTable } from './VsphereAlertsTable';
import { VsphereActivityTable } from './VsphereActivityTable';
import { VsphereDatastoresTable } from './VsphereDatastoresTable';
import { VsphereNetworksTable } from './VsphereNetworksTable';
import { VsphereVirtualMachinesTable } from './VsphereVirtualMachinesTable';

// vSphere phase 1 projects ESXi hosts as canonical `agent`, virtual machines
// as canonical `vm`, datastores as canonical `storage`, and vCenter networks
// as canonical `network`; provider-native topology stays in VMware metadata
// under those shared resources.
const VMWARE_RESOURCE_QUERY = 'type=agent,vm,storage,network';
const VALID_TABS = new Set<VmwarePageTabId>(VMWARE_TAB_SPECS.map((tab) => tab.id));

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
            <Show when={activeTab() === 'overview'}>
              <VmwareOverview model={model} />
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
}

function VmwareOverview(props: VmwareOverviewProps) {
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
      <VsphereVirtualMachinesTable
        vms={props.model().vms}
        scope={props.model().resources}
        emptyIcon={vmwareIcon()}
        emptyTitle="No vSphere VMs"
        emptyDescription="Virtual machines appear here once the vCenter connection enumerates them."
      />
    </div>
  );
}

export default VmwarePageSurface;
