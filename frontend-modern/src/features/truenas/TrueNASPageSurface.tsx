import { useLocation, useSearchParams } from '@solidjs/router';
import { Show, createEffect, createMemo, createSignal, type Accessor } from 'solid-js';
import { buildInfrastructureAgentUpdatesPath } from '@/components/Settings/infrastructureWorkspaceModel';
import { getPlatformIcon } from '@/features/platformPage/platformIcon';
import { PlatformOutdatedAgentNotice } from '@/features/platformPage/PlatformOutdatedAgentNotice';
import {
  collectOutdatedAgentHosts,
  formatAgentVersionDisplay,
} from '@/features/platformPage/agentVersion';
import { useRecoveryPoints } from '@/hooks/useRecoveryPoints';
import { useUnifiedResources, type UnifiedResourceFacets } from '@/hooks/useUnifiedResources';
import { updateStore } from '@/stores/updates';
import type { Resource } from '@/types/resource';
import {
  PlatformErrorState,
  PlatformSectionTabs,
  PlatformTableEmptyState,
  PlatformTableLoadingState,
} from '@/features/platformPage/sharedPlatformPage';
import { TrueNASAppsTable } from './TrueNASAppsTable';
import { TrueNASAlertsTable } from './TrueNASAlertsTable';
import { TrueNASNetworkSharesTable } from './TrueNASNetworkSharesTable';
import { TrueNASProtectionTable } from './TrueNASProtectionTable';
import { TrueNASServicesTable } from './TrueNASServicesTable';
import { TrueNASStorageTopologyTable } from './TrueNASStorageTopologyTable';
import { TrueNASSystemsTable } from './TrueNASSystemsTable';
import { TrueNASVirtualMachinesTable } from './TrueNASVirtualMachinesTable';
import {
  TRUENAS_TAB_SPECS,
  buildTrueNASPageModel,
  getTrueNASPageTabSpecs,
  type TrueNASPageModel,
  type TrueNASPageTabId,
  type TrueNASStorageKindFilter,
} from './truenasPageModel';

// `pool` and `dataset` collapse into `storage` at the API boundary
// (with `storage.topology` differentiating them) — they are not
// first-class type tokens and including them triggers a 400 from
// `/api/resources`. The page model still buckets by topology
// client-side. Canonical TrueNAS systems retain `truenas` in their merged
// source set even when a Pulse agent enriches the same appliance, so the page
// can stay provider-scoped without hydrating every agent in the estate.
const TRUENAS_RESOURCE_QUERY_BY_TAB: Record<TrueNASPageTabId, string> = {
  overview: 'source=truenas&type=agent,vm,app-container,network-share,storage,physical_disk',
  storage: 'source=truenas&type=agent,storage,physical_disk',
  services: 'source=truenas&type=agent',
  apps: 'source=truenas&type=agent,app-container',
  vms: 'source=truenas&type=agent,vm',
  shares: 'source=truenas&type=agent,network-share',
  protection: 'source=truenas&type=agent',
};
const TRUENAS_PLATFORM_FILTER = 'truenas';
const VALID_TABS = new Set<TrueNASPageTabId>(TRUENAS_TAB_SPECS.map((tab) => tab.id));

const TrueNASIcon = getPlatformIcon('truenas');
const truenasIcon = () => <TrueNASIcon class="h-6 w-6 text-slate-400" />;

export function TrueNASPageSurface() {
  const location = useLocation();
  const [searchParams, setSearchParams] = useSearchParams();
  const requestedTab = createMemo<TrueNASPageTabId>(() => {
    const segment = location.pathname.split('/').filter(Boolean)[1] as TrueNASPageTabId | undefined;
    return segment && VALID_TABS.has(segment) ? segment : 'overview';
  });
  const [navigationResources, setNavigationResources] = createSignal<Resource[]>([]);
  const [resourceFacets, setResourceFacets] = createSignal<UnifiedResourceFacets | null>(null);
  const navigationModel = createMemo(() => buildTrueNASPageModel(navigationResources()));
  const tabs = createMemo(() => {
    const facets = resourceFacets();
    if (!facets) {
      return navigationResources().length > 0
        ? getTrueNASPageTabSpecs(navigationModel())
        : TRUENAS_TAB_SPECS;
    }
    return getTrueNASPageTabSpecs(navigationModel(), {
      typeCounts: facets.byType,
    });
  });
  const activeTab = createMemo<TrueNASPageTabId>(() =>
    tabs().some((tab) => tab.id === requestedTab()) ? requestedTab() : 'overview',
  );
  const shouldHydrateTab = (tab: TrueNASPageTabId) => activeTab() === tab;
  const createTabResources = (tab: TrueNASPageTabId) =>
    useUnifiedResources({
      query: TRUENAS_RESOURCE_QUERY_BY_TAB[tab],
      cacheKey: `truenas-${tab}`,
      initialHydration: 'prefer-ws-then-rest',
      enabled: () => shouldHydrateTab(tab),
      realtimeEnabled: () => shouldHydrateTab(tab),
    });
  const overviewResources = createTabResources('overview');
  const storageResources = createTabResources('storage');
  const serviceResources = createTabResources('services');
  const appResources = createTabResources('apps');
  const vmResources = createTabResources('vms');
  const shareResources = createTabResources('shares');
  const protectionResources = createTabResources('protection');
  const resourceState = createMemo(() => {
    switch (activeTab()) {
      case 'storage':
        return storageResources;
      case 'services':
        return serviceResources;
      case 'apps':
        return appResources;
      case 'vms':
        return vmResources;
      case 'shares':
        return shareResources;
      case 'protection':
        return protectionResources;
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
  const model = createMemo(() => buildTrueNASPageModel(resources()));
  const protection = useRecoveryPoints(() =>
    requestedTab() === 'protection' && model().systems.length > 0
      ? {
          platform: TRUENAS_PLATFORM_FILTER,
          page: 1,
          limit: 200,
        }
      : null,
  );
  const storageKindFilter = (): TrueNASStorageKindFilter => {
    const value = searchParams.kind;
    return value === 'volumes' || value === 'disks' ? value : 'all';
  };
  const setStorageKindFilter = (value: TrueNASStorageKindFilter) =>
    setSearchParams({ kind: value === 'all' ? null : value }, { replace: true });
  const agentUpdateTargetVersion = createMemo(
    () => updateStore.versionInfo()?.agentUpdateTargetVersion,
  );
  const outdatedAgentHosts = createMemo(() =>
    collectOutdatedAgentHosts(model().systems, agentUpdateTargetVersion()),
  );
  const outdatedAgentUpdatePath = createMemo(() =>
    buildInfrastructureAgentUpdatesPath(outdatedAgentHosts().map((host) => host.agentId)),
  );
  const serverVersionDisplay = createMemo(() =>
    formatAgentVersionDisplay(agentUpdateTargetVersion()),
  );

  return (
    <div data-testid="truenas-page" class="pulse-wide-data-surface space-y-3">
      <PlatformSectionTabs tabs={tabs()} active={activeTab()} ariaLabel="TrueNAS sections" />

      <Show
        when={!loading() || model().resources.length > 0}
        fallback={
          <PlatformTableLoadingState
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
            <PlatformOutdatedAgentNotice
              hosts={outdatedAgentHosts()}
              targetVersion={serverVersionDisplay()}
              missingLabel="agent-contributed TrueNAS system detail and command support"
              copyVariant="latest-detail"
              actionHref={outdatedAgentUpdatePath()}
              actionLabel="Open agent upgrade commands"
            />
            <Show when={activeTab() === 'overview'}>
              <div class="space-y-4">
                <TrueNASOverview model={model} />
              </div>
            </Show>
            <Show when={activeTab() === 'storage'}>
              <TrueNASStorage
                model={model}
                kindFilter={storageKindFilter()}
                onKindFilterChange={setStorageKindFilter}
              />
            </Show>
            <Show when={activeTab() === 'services'}>
              <TrueNASServices model={model} />
            </Show>
            <Show when={activeTab() === 'apps'}>
              <TrueNASApps model={model} />
            </Show>
            <Show when={activeTab() === 'vms'}>
              <TrueNASVMs model={model} />
            </Show>
            <Show when={activeTab() === 'shares'}>
              <TrueNASShares model={model} />
            </Show>
            <Show when={activeTab() === 'protection'}>
              <TrueNASProtection recoveryPoints={protection} />
            </Show>
          </Show>
        </Show>
      </Show>
    </div>
  );
}

interface TrueNASOverviewProps {
  model: Accessor<TrueNASPageModel>;
}

interface TrueNASStorageProps extends TrueNASOverviewProps {
  kindFilter: TrueNASStorageKindFilter;
  onKindFilterChange: (value: TrueNASStorageKindFilter) => void;
}

function TrueNASStorage(props: TrueNASStorageProps) {
  return (
    <TrueNASStorageTopologyTable
      resources={props.model().resources}
      scope={props.model().resources}
      emptyIcon={truenasIcon()}
      emptyTitle="No TrueNAS storage inventory"
      emptyDescription="Pools, datasets, and physical disks appear here once the TrueNAS API reports storage inventory."
      kindFilter={props.kindFilter}
      onKindFilterChange={props.onKindFilterChange}
    />
  );
}

function TrueNASApps(props: TrueNASOverviewProps) {
  return (
    <TrueNASAppsTable
      apps={props.model().apps}
      scope={props.model().resources}
      emptyIcon={truenasIcon()}
      emptyTitle="No TrueNAS apps"
      emptyDescription="Apps appear here once the TrueNAS API reports app.query inventory."
    />
  );
}

function TrueNASServices(props: TrueNASOverviewProps) {
  return (
    <TrueNASServicesTable
      services={props.model().services}
      emptyIcon={truenasIcon()}
      emptyTitle="No TrueNAS services"
      emptyDescription="System services appear here once the TrueNAS API reports service.query inventory."
    />
  );
}

function TrueNASVMs(props: TrueNASOverviewProps) {
  return (
    <TrueNASVirtualMachinesTable
      vms={props.model().vms}
      scope={props.model().resources}
      emptyIcon={truenasIcon()}
      emptyTitle="No TrueNAS VMs"
      emptyDescription="VMs appear here once the TrueNAS API reports vm.query inventory."
    />
  );
}

function TrueNASShares(props: TrueNASOverviewProps) {
  return (
    <TrueNASNetworkSharesTable
      shares={props.model().shares}
      scope={props.model().resources}
      emptyIcon={truenasIcon()}
      emptyTitle="No TrueNAS shares"
      emptyDescription="Shares appear here once the TrueNAS API reports SMB or NFS sharing inventory."
    />
  );
}

function TrueNASProtection(props: { recoveryPoints: ReturnType<typeof useRecoveryPoints> }) {
  return (
    <TrueNASProtectionTable
      points={props.recoveryPoints.points()}
      loading={props.recoveryPoints.response.loading}
      error={props.recoveryPoints.response.error}
      onRefresh={() => void props.recoveryPoints.refetch()}
      emptyIcon={truenasIcon()}
      emptyTitle="No TrueNAS protection activity"
      emptyDescription="ZFS snapshots and replication tasks appear here once the TrueNAS API reports snapshot or replication activity."
    />
  );
}

function TrueNASOverview(props: TrueNASOverviewProps) {
  return (
    <div class="space-y-4">
      <TrueNASSystemsTable
        systems={props.model().systems}
        scope={props.model().resources}
        emptyIcon={truenasIcon()}
        emptyTitle="No TrueNAS systems"
        emptyDescription="TrueNAS systems appear here once a TrueNAS connection reports its top-level appliance."
        showToolbar={false}
      />
      <Show when={props.model().incidents.length > 0}>
        <TrueNASAlertsTable
          incidents={props.model().incidents}
          scope={props.model().resources}
          emptyIcon={truenasIcon()}
          emptyTitle="No active TrueNAS alerts"
          emptyDescription="TrueNAS health alerts appear here when the TrueNAS API reports active system, pool, or disk incidents."
        />
      </Show>
    </div>
  );
}

export default TrueNASPageSurface;
