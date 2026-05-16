import { useLocation } from '@solidjs/router';
import CpuIcon from 'lucide-solid/icons/cpu';
import { Show, createMemo } from 'solid-js';
import { WorkloadsSurface } from '@/components/Workloads/WorkloadsSurface';
import { VsphereDatastoresTable } from './VsphereDatastoresTable';
import { useUnifiedResources } from '@/hooks/useUnifiedResources';
import {
  PlatformErrorState,
  PlatformSectionTabs,
  PlatformTableEmptyState,
} from '@/features/platformPage/sharedPlatformPage';
import { VsphereHostsTable } from './VsphereHostsTable';
import { VMWARE_TAB_SPECS, buildVmwarePageModel, type VmwarePageTabId } from './vmwarePageModel';

// `datastore` is not a first-class type token at the API boundary; vSphere
// datastores are emitted as canonical `storage` rows. Including it
// triggers a 400 from `/api/resources`.
const VMWARE_RESOURCE_QUERY = 'type=agent,vm,storage';
const VMWARE_PLATFORM_FILTER = 'vmware-vsphere';
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
                description="VMware vSphere is in first-lab-ready readiness. Add a vCenter connection in Settings to populate this page."
              />
            }
          >
            <Show when={activeTab() === 'overview'}>
              <div class="space-y-4">
                <VsphereHostsTable
                  hosts={model().hosts}
                  scope={model().resources}
                  emptyIcon={vmwareIcon()}
                  emptyTitle="No vSphere hosts"
                  emptyDescription="Hosts appear here once the vCenter connection enumerates them."
                  showToolbar={false}
                />
                <WorkloadsSurface
                  vms={[]}
                  containers={[]}
                  nodes={[]}
                  useWorkloads
                  embedded
                  tableOnly
                  showFilterToolbar
                  suppressPlatformFilter
                  forcedPlatform={VMWARE_PLATFORM_FILTER}
                  compactGroupHeaders
                />
                <Show when={model().datastores.length > 0}>
                  <VsphereDatastoresTable
                    resources={model().datastores}
                    emptyIcon={vmwareIcon()}
                    emptyTitle="No vSphere datastores"
                    emptyDescription="Datastores appear here once a vCenter connection enumerates them."
                    showToolbar={false}
                  />
                </Show>
              </div>
            </Show>
            <Show when={activeTab() === 'vms'}>
              <WorkloadsSurface
                vms={[]}
                containers={[]}
                nodes={[]}
                useWorkloads
                embedded
                tableOnly
                showFilterToolbar
                suppressPlatformFilter
                forcedPlatform={VMWARE_PLATFORM_FILTER}
              />
            </Show>
            <Show when={activeTab() === 'storage'}>
              <VsphereDatastoresTable
                resources={model().datastores}
                emptyIcon={vmwareIcon()}
                emptyTitle="No vSphere datastores"
                emptyDescription="Datastores appear here once a vCenter connection enumerates them."
              />
            </Show>
          </Show>
        </Show>
      </Show>
    </div>
  );
}

export default VmwarePageSurface;
