import { Show, createEffect, createMemo, createSignal } from 'solid-js';
import { A, useLocation, useNavigate } from '@solidjs/router';
import ActivityIcon from 'lucide-solid/icons/activity';
import PlusIcon from 'lucide-solid/icons/plus';
import ServerIcon from 'lucide-solid/icons/server';
import SettingsIcon from 'lucide-solid/icons/settings';
import { buildAvailabilityTargetAddPath } from '@/components/Settings/availabilitySettingsModel';
import { buildInfrastructureOnboardingPath } from '@/components/Settings/infrastructureWorkspaceModel';
import {
  PlatformErrorState,
  PlatformSectionTabs,
  PlatformTableEmptyState,
  PlatformTableLoadingState,
} from '@/features/platformPage/sharedPlatformPage';
import { useUnifiedResources } from '@/hooks/useUnifiedResources';
import { STANDALONE_PATH, buildStandalonePath } from '@/routing/resourceLinks';
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
        ariaLabel="Standalone sections"
      />

      <Show
        when={!showLoading()}
        fallback={
          <PlatformTableLoadingState
            title="Loading standalone resources"
            description="Pulse is loading standalone machines and availability checks."
          />
        }
      >
        <Show
          when={!error()}
          fallback={
            <PlatformErrorState
              title="Could not load standalone resources"
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
                  title="No standalone monitored endpoints"
                  description="Install Pulse Agent on servers, laptops, and desktops for full telemetry, or add an agentless machine check when reachability is enough."
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
                      <A
                        href={buildAvailabilityTargetAddPath()}
                        class="inline-flex items-center gap-2 rounded-md border border-border bg-surface px-3 py-1.5 text-xs font-medium text-base-content shadow-sm hover:bg-slate-50"
                      >
                        <PlusIcon class="h-3.5 w-3.5" />
                        Add check
                      </A>
                    </div>
                  }
                />
              }
            >
              <div class="space-y-4">
                <Show
                  when={model().machines.length > 0}
                  fallback={
                    <Show when={model().availabilityChecks.length > 0}>
                      <PlatformTableEmptyState
                        icon={machineIcon()}
                        title="No standalone machines"
                        description="Service and device checks are monitored from the Availability checks tab. Mark an availability target as a machine when it represents a server, laptop, or desktop."
                        actions={
                          <A href={buildStandalonePath('availability')} class={overviewActionClass}>
                            <ActivityIcon class="h-3.5 w-3.5" />
                            View checks
                          </A>
                        }
                      />
                    </Show>
                  }
                >
                  <AgentsMachinesTable
                    resources={model().machines}
                    emptyIcon={machineIcon()}
                    emptyTitle="No standalone machines"
                    emptyDescription="Install the Pulse Agent on Linux, macOS, Windows, or Unraid systems for full telemetry, or add an agentless machine check when reachability is enough."
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
