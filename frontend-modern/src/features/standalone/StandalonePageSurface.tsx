import { Show, createEffect, createMemo, createSignal, type Component, type JSX } from 'solid-js';
import { useLocation, useNavigate } from '@solidjs/router';
import ActivityIcon from 'lucide-solid/icons/activity';
import CircleAlertIcon from 'lucide-solid/icons/circle-alert';
import ServerIcon from 'lucide-solid/icons/server';
import SettingsIcon from 'lucide-solid/icons/settings';
import { Button, ButtonLink } from '@/components/shared/Button';
import { MetadataBadge, type MetadataBadgeTone } from '@/components/shared/MetadataBadge';
import {
  buildInfrastructureAgentUpdatesPath,
  buildInfrastructureOnboardingPath,
} from '@/components/Settings/infrastructureWorkspaceModel';
import { buildAvailabilitySettingsPath } from '@/components/Settings/availabilitySettingsModel';
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
} from '@/features/platformPage/sharedPlatformPage';
import { useUnifiedResources } from '@/hooks/useUnifiedResources';
import { STANDALONE_PATH, buildStandalonePath } from '@/routing/resourceLinks';
import { updateStore } from '@/stores/updates';
import { formatRelativeTime } from '@/utils/format';
import { AvailabilityChecksTable } from './AvailabilityChecksTable';
import { AgentsMachinesTable } from './AgentsMachinesTable';
import {
  buildStandalonePageModel,
  buildStandalonePostureSummary,
  type StandalonePostureSummary,
} from './standalonePageModel';

const STANDALONE_RESOURCE_QUERY = 'type=agent,network-endpoint';
const STANDALONE_TAB_SPECS = [
  { id: 'machines', label: 'Machines', path: buildStandalonePath('machines') },
  { id: 'availability', label: 'Availability checks', path: buildStandalonePath('availability') },
] as const;
type StandaloneTabId = (typeof STANDALONE_TAB_SPECS)[number]['id'];

const machineIcon = () => <ServerIcon class="h-6 w-6 text-slate-400" />;
const availabilityIcon = () => <ActivityIcon class="h-6 w-6 text-slate-400" />;

const StandalonePostureCard: Component<{
  actions?: JSX.Element;
  label: string;
  noun: string;
  summary: StandalonePostureSummary;
}> = (props) => {
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
      ? formatRelativeTime(props.summary.latestUpdateAt, { compact: true, emptyText: 'unknown' })
      : 'unknown';

  return (
    <section
      aria-label={`${props.label} status`}
      data-testid="standalone-posture-summary"
      class="flex flex-col gap-3 rounded-md border border-border bg-surface-alt/50 px-3 py-3 sm:flex-row sm:items-center sm:justify-between"
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

const resolveStandaloneTab = (pathname: string): StandaloneTabId =>
  pathname.replace(/\/+$/, '') === buildStandalonePath('availability')
    ? 'availability'
    : 'machines';

export function StandalonePageSurface() {
  const location = useLocation();
  const navigate = useNavigate();
  const { resources, loading, error, refetch } = useUnifiedResources({
    query: STANDALONE_RESOURCE_QUERY,
    cacheKey: 'standalone-workspace',
    initialHydration: 'prefer-ws-then-rest',
  });

  const [initialLoadComplete, setInitialLoadComplete] = createSignal(false);

  const model = createMemo(() => buildStandalonePageModel(resources()));
  const activeTab = createMemo(() => resolveStandaloneTab(location.pathname));
  const machinePosture = createMemo(() => buildStandalonePostureSummary(model().machines));
  const availabilityPosture = createMemo(() =>
    buildStandalonePostureSummary(model().availabilityChecks),
  );
  const showLoading = createMemo(
    () =>
      loading() &&
      !initialLoadComplete() &&
      model().machines.length === 0 &&
      model().availabilityChecks.length === 0,
  );
  const showOverviewEmpty = createMemo(
    () =>
      initialLoadComplete() &&
      !error() &&
      model().machines.length === 0 &&
      model().availabilityChecks.length === 0,
  );
  const agentUpdateTargetVersion = createMemo(
    () => updateStore.versionInfo()?.agentUpdateTargetVersion,
  );
  const outdatedAgentHosts = createMemo(() =>
    collectOutdatedAgentHosts(model().machines, agentUpdateTargetVersion()),
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
    if (normalizedPath === STANDALONE_PATH || normalizedPath === `${STANDALONE_PATH}/overview`) {
      navigate(buildStandalonePath('machines'), { replace: true });
    }
  });

  return (
    <div data-testid="standalone-page" class="space-y-4">
      <PlatformSectionTabs
        tabs={STANDALONE_TAB_SPECS}
        active={activeTab()}
        ariaLabel="Machines sections"
      />

      <Show
        when={!showLoading()}
        fallback={
          <PlatformTableLoadingState
            title="Loading machines"
            description="Pulse is loading Pulse Agent machines and availability checks."
          />
        }
      >
        <Show
          when={!error()}
          fallback={
            <PlatformErrorState
              title="Could not load machines"
              description="Refresh the resource snapshot or check the API connection state."
              onRefresh={() => void refetch()}
            />
          }
        >
          <Show when={activeTab() === 'availability'}>
            <div class="space-y-3">
              <Show when={model().availabilityChecks.length > 0}>
                <StandalonePostureCard
                  label="Availability"
                  noun="check"
                  summary={availabilityPosture()}
                  actions={
                    <ButtonLink
                      href={buildAvailabilitySettingsPath()}
                      variant="secondary"
                      size="sm"
                      class="gap-2"
                    >
                      <SettingsIcon class="h-3.5 w-3.5" />
                      Manage checks
                    </ButtonLink>
                  }
                />
              </Show>
              <AvailabilityChecksTable
                resources={model().availabilityChecks}
                emptyIcon={availabilityIcon()}
                emptyTitle="No availability checks"
                emptyDescription="Add ping, TCP, MQTT, ESPHome, or HTTP checks for devices and services that cannot run Pulse Agent."
              />
            </div>
          </Show>

          <Show when={activeTab() === 'machines'}>
            <Show
              when={!showOverviewEmpty()}
              fallback={
                <PlatformTableEmptyState
                  icon={machineIcon()}
                  title="No Pulse Agent machines"
                  description="Install Pulse Agent on servers, laptops, and desktops for CPU, memory, disk, and network telemetry. Agentless reachability checks live in Availability checks."
                  actions={
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
                      <ButtonLink
                        href={buildStandalonePath('availability')}
                        variant="secondary"
                        size="sm"
                        class="gap-2 px-3"
                      >
                        <ActivityIcon class="h-3.5 w-3.5" />
                        View checks
                      </ButtonLink>
                    </div>
                  }
                />
              }
            >
              <div class="space-y-4">
                <Show when={model().machines.length > 0}>
                  <StandalonePostureCard
                    label="Pulse Agents"
                    noun="machine"
                    summary={machinePosture()}
                    actions={
                      <Button
                        type="button"
                        variant="secondary"
                        size="sm"
                        class="gap-2"
                        onClick={() => navigate(buildInfrastructureOnboardingPath('pick'))}
                      >
                        <SettingsIcon class="h-3.5 w-3.5" />
                        Add agent
                      </Button>
                    }
                  />
                </Show>
                <PlatformOutdatedAgentNotice
                  hosts={outdatedAgentHosts()}
                  targetVersion={serverVersionDisplay()}
                  missingLabel="agent command support and agent-managed platform detail"
                  copyVariant="latest-detail"
                  actionHref={outdatedAgentUpdatePath()}
                  actionLabel="Open agent upgrade commands"
                />
                <Show
                  when={model().machines.length > 0}
                  fallback={
                    <Show when={model().availabilityChecks.length > 0}>
                      <PlatformTableEmptyState
                        icon={machineIcon()}
                        title="No Pulse Agent machines"
                        description="Availability checks, including ICMP checks for computers, stay in the Availability checks tab. Install Pulse Agent when you need full machine telemetry."
                        actions={
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
                            <ButtonLink
                              href={buildStandalonePath('availability')}
                              variant="secondary"
                              size="sm"
                              class="gap-2 px-3"
                            >
                              <ActivityIcon class="h-3.5 w-3.5" />
                              View checks
                            </ButtonLink>
                          </div>
                        }
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
