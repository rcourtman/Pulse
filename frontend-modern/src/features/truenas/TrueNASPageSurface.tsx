import { useLocation } from '@solidjs/router';
import { Show, createMemo, type Accessor } from 'solid-js';
import { getPlatformIcon } from '@/features/platformPage/platformIcon';
import { useRecoveryPoints } from '@/hooks/useRecoveryPoints';
import { useUnifiedResources } from '@/hooks/useUnifiedResources';
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
  type TrueNASPageModel,
  type TrueNASPageTabId,
} from './truenasPageModel';

// `pool` and `dataset` collapse into `storage` at the API boundary
// (with `storage.topology` differentiating them) — they are not
// first-class type tokens and including them triggers a 400 from
// `/api/resources`. The page model still buckets by topology
// client-side. Keep `agent` in the source filter so a TrueNAS host that
// reports through the Pulse agent can still appear as the top-level appliance
// while native TrueNAS API inventory remains the primary source.
const TRUENAS_RESOURCE_QUERY =
  'source=truenas,agent&type=agent,vm,app-container,network-share,storage,physical_disk';
const TRUENAS_PLATFORM_FILTER = 'truenas';
const VALID_TABS = new Set<TrueNASPageTabId>(TRUENAS_TAB_SPECS.map((tab) => tab.id));

const TrueNASIcon = getPlatformIcon('truenas');
const truenasIcon = () => <TrueNASIcon class="h-6 w-6 text-slate-400" />;

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
            <Show when={activeTab() === 'overview'}>
              <TrueNASOverview model={model} />
            </Show>
            <Show when={activeTab() === 'storage'}>
              <TrueNASStorage model={model} />
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
              <TrueNASProtection />
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

function TrueNASStorage(props: TrueNASOverviewProps) {
  return (
    <TrueNASStorageTopologyTable
      resources={props.model().resources}
      scope={props.model().resources}
      emptyIcon={truenasIcon()}
      emptyTitle="No TrueNAS storage inventory"
      emptyDescription="Pools, datasets, and physical disks appear here once the TrueNAS API reports storage inventory."
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

function TrueNASProtection() {
  const recoveryPoints = useRecoveryPoints(() => ({
    platform: TRUENAS_PLATFORM_FILTER,
    page: 1,
    limit: 200,
  }));

  return (
    <TrueNASProtectionTable
      points={recoveryPoints.points()}
      loading={recoveryPoints.response.loading}
      error={recoveryPoints.response.error}
      onRefresh={() => void recoveryPoints.refetch()}
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
