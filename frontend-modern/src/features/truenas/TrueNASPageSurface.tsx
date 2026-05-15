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
import {
  TRUENAS_TAB_SPECS,
  buildTrueNASPageModel,
  type TrueNASPageTabId,
} from './truenasPageModel';

const TRUENAS_RESOURCE_QUERY =
  'type=app-container,storage,pool,dataset,physical_disk';
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
    return segment && VALID_TABS.has(segment) ? segment : 'storage';
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
            <Show when={activeTab() === 'storage'}>
              <StorageSurface
                embedded
                tableOnly
                showFilterToolbar
                forcedSourceFilter={TRUENAS_PLATFORM_FILTER}
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
