import { Show, createEffect, createMemo, createSignal } from 'solid-js';
import { A, useLocation, useNavigate } from '@solidjs/router';
import ActivityIcon from 'lucide-solid/icons/activity';
import ServerIcon from 'lucide-solid/icons/server';
import SettingsIcon from 'lucide-solid/icons/settings';
import {
  buildInfrastructureOnboardingPath,
  buildInfrastructureWorkspacePath,
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
} from '@/features/platformPage/sharedPlatformPage';
import { useUnifiedResources } from '@/hooks/useUnifiedResources';
import { STANDALONE_PATH, buildStandalonePath } from '@/routing/resourceLinks';
import { updateStore } from '@/stores/updates';
import { AvailabilityChecksTable } from './AvailabilityChecksTable';
import { AgentsMachinesTable } from './AgentsMachinesTable';
import { buildStandalonePageModel } from './standalonePageModel';

const STANDALONE_RESOURCE_QUERY = 'type=agent,network-endpoint';
const STANDALONE_TAB_SPECS = [
  { id: 'machines', label: 'Machines', path: buildStandalonePath('machines') },
  { id: 'availability', label: 'Availability checks', path: buildStandalonePath('availability') },
] as const;
type StandaloneTabId = (typeof STANDALONE_TAB_SPECS)[number]['id'];

const machineIcon = () => <ServerIcon class="h-6 w-6 text-slate-400" />;
const availabilityIcon = () => <ActivityIcon class="h-6 w-6 text-slate-400" />;
const overviewActionClass =
  'inline-flex items-center gap-2 rounded-md border border-border bg-surface px-3 py-1.5 text-xs font-medium text-base-content shadow-sm hover:bg-slate-50';

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
  const outdatedAgentHosts = createMemo(() =>
    collectOutdatedAgentHosts(model().machines, updateStore.versionInfo()?.version),
  );
  const serverVersionDisplay = createMemo(() =>
    formatAgentVersionDisplay(updateStore.versionInfo()?.version),
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
            <AvailabilityChecksTable
              resources={model().availabilityChecks}
              emptyIcon={availabilityIcon()}
              emptyTitle="No availability checks"
              emptyDescription="Add ping, TCP, MQTT, ESPHome, or HTTP checks for devices and services that cannot run Pulse Agent."
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
                  actions={
                    <div class="flex flex-wrap items-center justify-center gap-2">
                      <button
                        type="button"
                        onClick={() => navigate(buildInfrastructureOnboardingPath('pick'))}
                        class="inline-flex items-center gap-2 rounded-md border border-border bg-surface px-3 py-1.5 text-xs font-medium text-base-content shadow-sm hover:bg-slate-50"
                      >
                        <SettingsIcon class="h-3.5 w-3.5" />
                        Add agent
                      </button>
                      <A href={buildStandalonePath('availability')} class={overviewActionClass}>
                        <ActivityIcon class="h-3.5 w-3.5" />
                        View checks
                      </A>
                    </div>
                  }
                />
              }
            >
              <div class="space-y-4">
                <PlatformOutdatedAgentNotice
                  hosts={outdatedAgentHosts()}
                  targetVersion={serverVersionDisplay()}
                  missingLabel="agent command support and agent-managed platform detail"
                  copyVariant="latest-detail"
                  actionHref={buildInfrastructureWorkspacePath()}
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
                            <button
                              type="button"
                              onClick={() => navigate(buildInfrastructureOnboardingPath('pick'))}
                              class={overviewActionClass}
                            >
                              <SettingsIcon class="h-3.5 w-3.5" />
                              Add agent
                            </button>
                            <A
                              href={buildStandalonePath('availability')}
                              class={overviewActionClass}
                            >
                              <ActivityIcon class="h-3.5 w-3.5" />
                              View checks
                            </A>
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
                    targetAgentVersion={updateStore.versionInfo()?.version}
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
