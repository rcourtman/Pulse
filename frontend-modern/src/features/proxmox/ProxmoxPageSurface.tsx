import { A, useLocation } from '@solidjs/router';
import TriangleAlertIcon from 'lucide-solid/icons/triangle-alert';
import { For, Show, createMemo } from 'solid-js';
import RecoverySurface from '@/components/Recovery/Recovery';
import StorageSurface from '@/components/Storage/Storage';
import { WorkloadsSurface } from '@/components/Workloads/WorkloadsSurface';
import { ProxmoxIcon } from '@/components/icons/ProxmoxIcon';
import { EmptyState } from '@/components/shared/EmptyState';
import { TableCard } from '@/components/shared/TableCard';
import { ProxmoxMailGatewayTable } from './ProxmoxMailGatewayTable';
import { useUnifiedResources } from '@/hooks/useUnifiedResources';
import {
  PROXMOX_TAB_SPECS,
  buildProxmoxPageModel,
  type ProxmoxPageTabId,
} from './proxmoxPageModel';

const PROXMOX_RESOURCE_QUERY =
  'type=agent,vm,system-container,oci-container,storage,datastore,pool,dataset,physical_disk,ceph,pbs,pmg';

const PROXMOX_PLATFORM_FILTER = 'proxmox-pve';
const VALID_TABS = new Set<ProxmoxPageTabId>(PROXMOX_TAB_SPECS.map((tab) => tab.id));

function ProxmoxSectionTabs(props: { active: ProxmoxPageTabId }) {
  return (
    <nav
      class="flex flex-wrap items-center gap-1 border-b border-border"
      aria-label="Proxmox sections"
    >
      <For each={PROXMOX_TAB_SPECS}>
        {(tab) => (
          <A
            href={tab.path}
            class={`inline-flex min-h-10 items-center border-b-2 px-3 text-sm font-medium transition-colors ${
              props.active === tab.id
                ? 'border-blue-500 text-blue-600 dark:text-blue-300'
                : 'border-transparent text-muted hover:border-border hover:text-base-content'
            }`}
            aria-current={props.active === tab.id ? 'page' : undefined}
          >
            {tab.label}
          </A>
        )}
      </For>
    </nav>
  );
}

function ProxmoxTableEmptyState(props: { title: string; description: string }) {
  return (
    <TableCard>
      <div class="p-6">
        <EmptyState
          icon={<ProxmoxIcon class="h-6 w-6 text-slate-400" />}
          title={props.title}
          description={props.description}
        />
      </div>
    </TableCard>
  );
}

export function ProxmoxPageSurface() {
  const location = useLocation();
  const { resources, loading, error, refetch } = useUnifiedResources({
    query: PROXMOX_RESOURCE_QUERY,
    cacheKey: 'proxmox-workspace',
    initialHydration: 'prefer-ws-then-rest',
  });
  const activeTab = createMemo<ProxmoxPageTabId>(() => {
    const segment = location.pathname.split('/').filter(Boolean)[1] as ProxmoxPageTabId | undefined;
    return segment && VALID_TABS.has(segment) ? segment : 'overview';
  });
  const model = createMemo(() => buildProxmoxPageModel(resources()));

  return (
    <div data-testid="proxmox-page" class="space-y-3">
      <ProxmoxSectionTabs active={activeTab()} />

      <Show
        when={!loading() || model().resources.length > 0}
        fallback={
          <ProxmoxTableEmptyState
            title="Loading Proxmox resources"
            description="Pulse is loading the Proxmox resource snapshot."
          />
        }
      >
        <Show
          when={!error()}
          fallback={
            <TableCard>
              <div class="p-6">
                <EmptyState
                  icon={<TriangleAlertIcon class="h-6 w-6 text-slate-400" />}
                  title="Could not load Proxmox resources"
                  description="Refresh the resource snapshot or check the API connection state."
                  actions={
                    <button
                      type="button"
                      onClick={() => void refetch()}
                      class="inline-flex min-h-10 items-center rounded-md border border-border px-3 py-2 text-sm font-medium hover:bg-surface-hover"
                    >
                      Refresh
                    </button>
                  }
                />
              </div>
            </TableCard>
          }
        >
          <Show
            when={model().resources.length > 0}
            fallback={
              <ProxmoxTableEmptyState
                title="No Proxmox resources"
                description="Add Proxmox VE, Proxmox Backup Server, or Proxmox Mail Gateway in Settings to populate this platform page."
              />
            }
          >
            <Show when={activeTab() === 'overview'}>
              <WorkloadsSurface
                vms={[]}
                containers={[]}
                nodes={[]}
                useWorkloads
                embedded
                tableOnly
                showFilterToolbar
                suppressPlatformFilter
                forcedPlatform={PROXMOX_PLATFORM_FILTER}
                forcedGroupingMode="grouped"
              />
            </Show>
            <Show when={activeTab() === 'storage'}>
              <StorageSurface
                embedded
                tableOnly
                showFilterToolbar
                forcedSourceFilter={PROXMOX_PLATFORM_FILTER}
              />
            </Show>
            <Show when={activeTab() === 'replication'}>
              <RecoverySurface embedded tableOnly forcedPlatformFilter={PROXMOX_PLATFORM_FILTER} />
            </Show>
            <Show when={activeTab() === 'backups'}>
              <RecoverySurface embedded tableOnly forcedPlatformFilter={PROXMOX_PLATFORM_FILTER} />
            </Show>
            <Show when={activeTab() === 'ceph'}>
              <StorageSurface
                embedded
                tableOnly
                showFilterToolbar
                forcedView="pools"
                forcedSourceFilter={PROXMOX_PLATFORM_FILTER}
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

export default ProxmoxPageSurface;
