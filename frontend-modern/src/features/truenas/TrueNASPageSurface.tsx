import { useLocation } from '@solidjs/router';
import DatabaseIcon from 'lucide-solid/icons/database';
import { Show, createMemo } from 'solid-js';
import StorageSurface from '@/components/Storage/Storage';
import { WorkloadsSurface } from '@/components/Workloads/WorkloadsSurface';
import { useUnifiedResources } from '@/hooks/useUnifiedResources';
import {
  PlatformErrorState,
  PlatformSectionTabs,
  PlatformTableEmptyState,
} from '@/features/platformPage/sharedPlatformPage';
import { TrueNASDisksTable } from './TrueNASDisksTable';
import { TrueNASSystemsTable } from './TrueNASSystemsTable';
import {
  TRUENAS_TAB_SPECS,
  buildTrueNASPageModel,
  type TrueNASPageTabId,
} from './truenasPageModel';

// `pool` and `dataset` collapse into `storage` at the API boundary
// (with `storage.topology` differentiating them) — they are not
// first-class type tokens and including them triggers a 400 from
// `/api/resources`. The page model still buckets by topology
// client-side.
const TRUENAS_RESOURCE_QUERY = 'type=agent,app-container,storage,physical_disk';
const TRUENAS_PLATFORM_FILTER = 'truenas';
const VALID_TABS = new Set<TrueNASPageTabId>(TRUENAS_TAB_SPECS.map((tab) => tab.id));

const truenasIcon = () => <DatabaseIcon class="h-6 w-6 text-slate-400" />;

export function TrueNASPageSurface() {
  const location = useLocation();
  const { resources, loading, error, refetch } = useUnifiedResources({
    query: TRUENAS_RESOURCE_QUERY,
    cacheKey: 'truenas-workspace',
    initialHydration: 'prefer-ws-then-rest',
  });
  const activeTab = createMemo<TrueNASPageTabId>(() => {
    const segment = location.pathname.split('/').filter(Boolean)[1] as TrueNASPageTabId | undefined;
    return segment && VALID_TABS.has(segment) ? segment : 'overview';
  });
  const model = createMemo(() => buildTrueNASPageModel(resources()));

  return (
    <div data-testid="truenas-page" class="space-y-3">
      <PlatformSectionTabs
        tabs={TRUENAS_TAB_SPECS}
        active={activeTab()}
        ariaLabel="TrueNAS sections"
      />

      <Show
        when={!loading() || model().resources.length > 0}
        fallback={
          <PlatformTableEmptyState
            icon={truenasIcon()}
            title="Loading TrueNAS resources"
            description="Pulse is loading the TrueNAS resource snapshot."
          />
        }
      >
        <Show
          when={!error()}
          fallback={
            <PlatformErrorState
              title="Could not load TrueNAS resources"
              description="Refresh the resource snapshot or check the API connection state."
              onRefresh={() => void refetch()}
            />
          }
        >
          <Show
            when={model().resources.length > 0}
            fallback={
              <PlatformTableEmptyState
                icon={truenasIcon()}
                title="No TrueNAS systems"
                description="Add a TrueNAS connection in Settings or install the Pulse agent on a TrueNAS host."
              />
            }
          >
            <Show when={activeTab() === 'overview'}>
              <div class="space-y-4">
                <TrueNASSystemsTable
                  systems={model().systems}
                  scope={model().resources}
                  emptyIcon={truenasIcon()}
                  emptyTitle="No TrueNAS systems"
                  emptyDescription="TrueNAS systems appear here once a TrueNAS connection reports its top-level appliance."
                  showToolbar={false}
                />
                <StorageSurface
                  embedded
                  tableOnly
                  forcedView="pools"
                  forcedSourceFilter={TRUENAS_PLATFORM_FILTER}
                />
                <Show when={model().disks.length > 0}>
                  <TrueNASDisksTable
                    resources={model().disks}
                    emptyIcon={truenasIcon()}
                    emptyTitle="No TrueNAS disks reported"
                    emptyDescription="Physical disks appear here once a TrueNAS connection enumerates them."
                    showToolbar={false}
                  />
                </Show>
                <Show when={model().apps.length > 0}>
                  <WorkloadsSurface
                    vms={[]}
                    containers={[]}
                    nodes={[]}
                    useWorkloads
                    embedded
                    tableOnly
                    showFilterToolbar
                    suppressPlatformFilter
                    forcedPlatform={TRUENAS_PLATFORM_FILTER}
                    compactGroupHeaders
                  />
                </Show>
              </div>
            </Show>
            <Show when={activeTab() === 'storage'}>
              <StorageSurface
                embedded
                tableOnly
                showFilterToolbar
                forcedSourceFilter={TRUENAS_PLATFORM_FILTER}
              />
            </Show>
            <Show when={activeTab() === 'disks'}>
              <TrueNASDisksTable
                resources={model().disks}
                emptyIcon={truenasIcon()}
                emptyTitle="No TrueNAS disks reported"
                emptyDescription="Physical disks appear here once a TrueNAS connection enumerates them."
              />
            </Show>
            <Show when={activeTab() === 'apps'}>
              <WorkloadsSurface
                vms={[]}
                containers={[]}
                nodes={[]}
                useWorkloads
                embedded
                tableOnly
                showFilterToolbar
                suppressPlatformFilter
                forcedPlatform={TRUENAS_PLATFORM_FILTER}
              />
            </Show>
          </Show>
        </Show>
      </Show>
    </div>
  );
}

export default TrueNASPageSurface;
