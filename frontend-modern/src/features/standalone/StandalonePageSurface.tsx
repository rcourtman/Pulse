import { Show, createEffect, createMemo, createSignal, type Component, type JSX } from 'solid-js';
import { useLocation, useNavigate, useSearchParams } from '@solidjs/router';
import ActivityIcon from 'lucide-solid/icons/activity';
import CircleAlertIcon from 'lucide-solid/icons/circle-alert';
import MonitorIcon from 'lucide-solid/icons/monitor';
import ServerIcon from 'lucide-solid/icons/server';
import SettingsIcon from 'lucide-solid/icons/settings';
import { FeaturePreviewTag } from '@/components/FeatureRelease';
import { Button, ButtonLink } from '@/components/shared/Button';
import { MetadataBadge, type MetadataBadgeTone } from '@/components/shared/MetadataBadge';
import {
  buildInfrastructureAgentUpdatesPath,
  buildInfrastructureOnboardingPath,
} from '@/components/Settings/infrastructureWorkspaceModel';
import { PlatformOutdatedAgentNotice } from '@/features/platformPage/PlatformOutdatedAgentNotice';
import {
  collectOutdatedAgentHosts,
  formatAgentVersionDisplay,
} from '@/features/platformPage/agentVersion';
import {
  PlatformErrorState,
  PlatformSectionTabs,
  PlatformTableEmptyState,
  PlatformTableLoadingState,
  normalizePlatformResourceStatusFilter,
  type PlatformResourceStatusFilter,
} from '@/features/platformPage/sharedPlatformPage';
import { useUnifiedResources } from '@/hooks/useUnifiedResources';
import {
  AVAILABILITY_PATH,
  STANDALONE_PATH,
  STANDALONE_QUERY_PARAMS,
  buildAvailabilityPath,
  buildStandaloneHostVmsPath,
  buildStandalonePath,
  isStandaloneVmsRoute,
} from '@/routing/resourceLinks';
import { updateStore } from '@/stores/updates';
import { isFeatureAvailable } from '@/utils/featureReleases';
import { formatRelativeTime } from '@/utils/format';
import { useRelativeTimeNow } from '@/utils/relativeTimeClock';
import { AgentsMachinesTable } from './AgentsMachinesTable';
import { StandaloneVirtualMachinesTable } from './StandaloneVirtualMachinesTable';
import { collectHostIdentityConflictHosts } from './hostIdentityConflict';
import { HostIdentityConflictNotice } from './HostIdentityConflictNotice';
import {
  buildStandaloneMachineVmCounts,
  buildStandalonePageModel,
  getStandaloneTabSpecs,
  normalizeStandaloneVMStatusFilter,
  type StandalonePostureSummary,
  type StandaloneTabId,
  type StandaloneVMStatusFilter,
} from './standalonePageModel';

// Machines owns standalone Pulse Agent hosts, standalone hypervisor VMs
// reported by those agents (libvirt/KVM, XCP-ng), and standalone availability
// targets. Provider-owned agents and VMs are excluded at the API boundary so
// this surface does not hydrate the rest of the infrastructure estate and
// discard it again in buildStandalonePageModel. Docker remains additive here
// because a standalone Pulse Agent host can also contribute a Docker facet.
const STANDALONE_RESOURCE_QUERY =
  'source=agent,availability&excludeSource=proxmox,kubernetes,truenas,vmware&type=agent,vm,network-endpoint';

const machineIcon = () => <ServerIcon class="h-6 w-6 text-slate-400" />;
const vmIcon = () => <MonitorIcon class="h-6 w-6 text-slate-400" />;

export const StandalonePostureCard: Component<{
  actions?: JSX.Element;
  label: string;
  noun: string;
  summary: StandalonePostureSummary;
}> = (props) => {
  const now = useRelativeTimeNow();
  const headline = () => {
    if (props.summary.critical > 0) {
      return `${props.summary.critical} ${props.noun}${props.summary.critical === 1 ? '' : 's'} offline`;
    }
    if (props.summary.warning > 0) {
      return `${props.summary.warning} ${props.noun}${props.summary.warning === 1 ? ' needs' : 's need'} attention`;
    }
    if (props.summary.unknown > 0) {
      return `${props.summary.unknown} ${props.noun}${props.summary.unknown === 1 ? '' : 's'} awaiting status`;
    }
    return `All ${props.summary.total} ${props.noun}${props.summary.total === 1 ? '' : 's'} reporting normally`;
  };
  const tone = (): MetadataBadgeTone => {
    if (props.summary.critical > 0) return 'danger';
    if (props.summary.warning > 0) return 'warning';
    if (props.summary.unknown > 0) return 'info';
    return 'success';
  };
  const latestUpdate = () =>
    props.summary.latestUpdateAt
      ? formatRelativeTime(props.summary.latestUpdateAt, {
          compact: true,
          emptyText: 'unknown',
          now: now(),
        })
      : 'unknown';

  return (
    <section
      aria-label={`${props.label} status`}
      data-testid="standalone-posture-summary"
      class="flex flex-col gap-3 rounded-md border border-border px-3 py-3 sm:flex-row sm:items-center sm:justify-between"
    >
      <div class="min-w-0">
        <div class="flex flex-wrap items-center gap-2">
          <span class="text-xs font-semibold uppercase tracking-wider text-muted">
            {props.label}
          </span>
          <MetadataBadge tone={tone()} size="xs" shape="rounded">
            {headline()}
          </MetadataBadge>
        </div>
        <div class="mt-1.5 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted">
          <span>{props.summary.total} total</span>
          <span>{props.summary.normal} healthy</span>
          <Show when={props.summary.attention > 0}>
            <span class="inline-flex items-center gap-1 text-amber-700 dark:text-amber-300">
              <CircleAlertIcon class="h-3.5 w-3.5" aria-hidden="true" />
              {props.summary.attention} need attention
            </span>
          </Show>
          <Show when={props.summary.unknown > 0}>
            <span>{props.summary.unknown} unknown</span>
          </Show>
          <span>latest data {latestUpdate()}</span>
        </div>
      </div>
      <Show when={props.actions}>
        <div class="flex shrink-0 flex-wrap items-center gap-2">{props.actions}</div>
      </Show>
    </section>
  );
};

const resolveStandaloneTab = (pathname: string, vmsAvailable: boolean): StandaloneTabId => {
  const normalized = pathname.replace(/\/+$/, '');
  if (normalized === buildStandalonePath('availability')) return 'availability';
  if (vmsAvailable && isStandaloneVmsRoute(normalized)) return 'vms';
  return 'machines';
};

export function StandalonePageSurface() {
  const location = useLocation();
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const { resources, loading, error, refetch } = useUnifiedResources({
    query: STANDALONE_RESOURCE_QUERY,
    cacheKey: 'standalone-workspace',
    initialHydration: 'prefer-ws-then-rest',
  });

  const [initialLoadComplete, setInitialLoadComplete] = createSignal(false);

  const model = createMemo(() => buildStandalonePageModel(resources()));
  const vmsAvailable = () => isFeatureAvailable('standaloneVms');
  // An inventory can contain these guests before the view ships. They must
  // not change the Machines empty/loading states or supply host badges.
  const visibleVms = createMemo(() => (vmsAvailable() ? model().vms : []));
  const activeTab = createMemo(() => resolveStandaloneTab(location.pathname, vmsAvailable()));
  const tabs = createMemo(() => getStandaloneTabSpecs(visibleVms().length > 0, activeTab()));
  const vmCountByMachineId = createMemo(() =>
    buildStandaloneMachineVmCounts(model().machines, visibleVms()),
  );
  const machineSearchFilter = createMemo(() => {
    const value = searchParams[STANDALONE_QUERY_PARAMS.query];
    return typeof value === 'string' ? value : '';
  });
  const setMachineSearchFilter = (value: string) => {
    setSearchParams({ [STANDALONE_QUERY_PARAMS.query]: value || null }, { replace: true });
  };
  const machineStatusFilter = createMemo(() =>
    normalizePlatformResourceStatusFilter(searchParams[STANDALONE_QUERY_PARAMS.status]),
  );
  const setMachineStatusFilter = (status: PlatformResourceStatusFilter) => {
    setSearchParams(
      { [STANDALONE_QUERY_PARAMS.status]: status === 'all' ? null : status },
      { replace: true },
    );
  };
  const vmStatusFilter = createMemo(() =>
    normalizeStandaloneVMStatusFilter(searchParams[STANDALONE_QUERY_PARAMS.status]),
  );
  const setVmStatusFilter = (status: StandaloneVMStatusFilter) => {
    setSearchParams(
      { [STANDALONE_QUERY_PARAMS.status]: status === 'all' ? null : status },
      { replace: true },
    );
  };
  const vmHostFilter = createMemo(() => {
    const value = searchParams[STANDALONE_QUERY_PARAMS.host];
    return (Array.isArray(value) ? value[0] : value)?.trim() || undefined;
  });
  const clearVmHostFilter = () => {
    setSearchParams({ [STANDALONE_QUERY_PARAMS.host]: null }, { replace: true });
  };
  const resetMachineFilters = () => {
    setSearchParams(
      {
        [STANDALONE_QUERY_PARAMS.query]: null,
        [STANDALONE_QUERY_PARAMS.status]: null,
      },
      { replace: true },
    );
  };
  const resetVmFilters = () => {
    setSearchParams(
      {
        [STANDALONE_QUERY_PARAMS.query]: null,
        [STANDALONE_QUERY_PARAMS.status]: null,
        [STANDALONE_QUERY_PARAMS.host]: null,
      },
      { replace: true },
    );
  };
  const showLoading = createMemo(
    () =>
      loading() &&
      !initialLoadComplete() &&
      model().machines.length === 0 &&
      visibleVms().length === 0 &&
      model().availabilityChecks.length === 0,
  );
  const showOverviewEmpty = createMemo(
    () =>
      initialLoadComplete() &&
      !error() &&
      model().machines.length === 0 &&
      visibleVms().length === 0 &&
      model().availabilityChecks.length === 0,
  );
  const agentUpdateTargetVersion = createMemo(
    () => updateStore.versionInfo()?.agentUpdateTargetVersion,
  );
  const outdatedAgentHosts = createMemo(() =>
    collectOutdatedAgentHosts(model().machines, agentUpdateTargetVersion()),
  );
  const identityConflictHosts = createMemo(() =>
    collectHostIdentityConflictHosts(model().machines),
  );
  const outdatedAgentUpdatePath = createMemo(() =>
    buildInfrastructureAgentUpdatesPath(outdatedAgentHosts().map((host) => host.agentId)),
  );
  const serverVersionDisplay = createMemo(() =>
    formatAgentVersionDisplay(agentUpdateTargetVersion()),
  );

  createEffect(() => {
    if (!loading() && !initialLoadComplete()) {
      setInitialLoadComplete(true);
    }
  });

  createEffect(() => {
    const normalizedPath = location.pathname.replace(/\/+$/, '');
    if (normalizedPath === buildStandalonePath('availability')) {
      navigate(`${AVAILABILITY_PATH}${location.search ?? ''}${location.hash ?? ''}`, {
        replace: true,
      });
      return;
    }
    if (normalizedPath === STANDALONE_PATH || normalizedPath === `${STANDALONE_PATH}/overview`) {
      navigate(buildStandalonePath('machines'), { replace: true });
    }
  });

  const renderEmptyMachineActions = () => (
    <div class="flex flex-wrap items-center justify-center gap-2">
      <Button
        type="button"
        variant="secondary"
        size="sm"
        class="gap-2 px-3"
        onClick={() => navigate(buildInfrastructureOnboardingPath('pick'))}
      >
        <SettingsIcon class="h-3.5 w-3.5" />
        Add agent
      </Button>
      <ButtonLink href={buildAvailabilityPath()} variant="secondary" size="sm" class="gap-2 px-3">
        <ActivityIcon class="h-3.5 w-3.5" />
        View checks
      </ButtonLink>
    </div>
  );

  return (
    <div data-testid="standalone-page" class="pulse-wide-data-surface space-y-3">
      <Show when={tabs().length > 1}>
        <PlatformSectionTabs tabs={tabs()} active={activeTab()} ariaLabel="Machines sections" />
      </Show>

      <Show
        when={!showLoading()}
        fallback={
          <PlatformTableLoadingState
            title={activeTab() === 'vms' ? 'Loading virtual machines' : 'Loading machines'}
            description={
              activeTab() === 'vms'
                ? 'Pulse is loading standalone virtual machines.'
                : 'Pulse is loading Pulse Agent machines and availability checks.'
            }
          />
        }
      >
        <Show
          when={!error()}
          fallback={
            <PlatformErrorState
              title={
                activeTab() === 'vms'
                  ? 'Could not load virtual machines'
                  : 'Could not load machines'
              }
              description="Refresh the resource snapshot or check the API connection state."
              onRefresh={() => void refetch()}
            />
          }
        >
          <Show when={activeTab() === 'vms'}>
            <StandaloneVirtualMachinesTable
              titleMeta={<FeaturePreviewTag feature="standaloneVms" />}
              vms={visibleVms()}
              machines={model().machines}
              selectedHostId={vmHostFilter}
              onClearHostScope={clearVmHostFilter}
              externalSearch={machineSearchFilter}
              onExternalSearchChange={setMachineSearchFilter}
              externalStatus={vmStatusFilter}
              onExternalStatusChange={setVmStatusFilter}
              onResetFilters={resetVmFilters}
              emptyIcon={vmIcon()}
              emptyTitle="No virtual machines"
              emptyDescription="Virtual machines reported by standalone Pulse Agents (Unraid, Linux libvirt/KVM, and XCP-ng) appear here automatically."
            />
          </Show>

          <Show when={activeTab() === 'machines'}>
            <Show
              when={!showOverviewEmpty()}
              fallback={
                <PlatformTableEmptyState
                  icon={machineIcon()}
                  title="No Pulse Agent machines"
                  description="Install Pulse Agent on servers, laptops, and desktops for CPU, memory, disk, and network telemetry. Agentless reachability checks live in Availability checks."
                  actions={renderEmptyMachineActions()}
                />
              }
            >
              <div class="space-y-4">
                <HostIdentityConflictNotice hosts={identityConflictHosts()} />
                <PlatformOutdatedAgentNotice
                  hosts={outdatedAgentHosts()}
                  targetVersion={serverVersionDisplay()}
                  missingLabel="fixes and machine details"
                  copyVariant="latest-detail"
                  actionHref={outdatedAgentUpdatePath()}
                  actionLabel="Open agent upgrade commands"
                />
                <Show
                  when={model().machines.length > 0}
                  fallback={
                    <Show when={model().availabilityChecks.length > 0 || visibleVms().length > 0}>
                      <PlatformTableEmptyState
                        icon={machineIcon()}
                        title="No Pulse Agent machines"
                        description="Availability checks, including ICMP checks for computers, stay in the Availability checks tab. Install Pulse Agent when you need full machine telemetry."
                        actions={renderEmptyMachineActions()}
                      />
                    </Show>
                  }
                >
                  <AgentsMachinesTable
                    resources={model().machines}
                    emptyIcon={machineIcon()}
                    emptyTitle="No machines"
                    emptyDescription="Install Pulse Agent on Linux, macOS, Windows, or Unraid systems for full CPU, memory, disk, and network telemetry."
                    targetAgentVersion={agentUpdateTargetVersion()}
                    vmCountByMachineId={vmCountByMachineId}
                    onOpenHostVms={
                      vmsAvailable()
                        ? (machine) => navigate(buildStandaloneHostVmsPath(machine.id))
                        : undefined
                    }
                    externalSearch={machineSearchFilter}
                    onExternalSearchChange={setMachineSearchFilter}
                    externalStatus={machineStatusFilter}
                    onExternalStatusChange={setMachineStatusFilter}
                    onResetFilters={resetMachineFilters}
                  />
                </Show>
              </div>
            </Show>
          </Show>
        </Show>
      </Show>
    </div>
  );
}

export default StandalonePageSurface;
