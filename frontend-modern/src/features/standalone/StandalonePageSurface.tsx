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
import { buildStandalonePath } from '@/routing/resourceLinks';
import { AvailabilityChecksTable } from './AvailabilityChecksTable';
import { AgentsMachinesTable } from './AgentsMachinesTable';
import { buildStandalonePageModel } from './standalonePageModel';

const STANDALONE_RESOURCE_QUERY = 'type=agent,network-endpoint';
const STANDALONE_TAB_SPECS = [
  { id: 'overview', label: 'Overview', path: buildStandalonePath('overview') },
  { id: 'availability', label: 'Availability checks', path: buildStandalonePath('availability') },
] as const;
type StandaloneTabId = (typeof STANDALONE_TAB_SPECS)[number]['id'];

const machineIcon = () => <ServerIcon class="h-6 w-6 text-slate-400" />;
const availabilityIcon = () => <ActivityIcon class="h-6 w-6 text-slate-400" />;

const resolveStandaloneTab = (pathname: string): StandaloneTabId =>
  pathname.replace(/\/+$/, '') === buildStandalonePath('availability')
    ? 'availability'
    : 'overview';

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

          <Show when={activeTab() === 'overview'}>
            <Show
              when={!showOverviewEmpty()}
              fallback={
                <PlatformTableEmptyState
                  icon={machineIcon()}
                  title="No standalone monitored endpoints"
                  description="Install Pulse Agent on standalone machines, or add agentless availability checks for devices and services that only need reachability monitoring."
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
                <Show when={model().machines.length > 0}>
                  <AgentsMachinesTable
                    resources={model().machines}
                    emptyIcon={machineIcon()}
                    emptyTitle="No standalone Pulse Agent machines"
                    emptyDescription="Install the Pulse Agent on Linux, macOS, Windows, or Unraid systems that are not already represented by a platform integration."
                  />
                </Show>
                <Show when={model().availabilityChecks.length > 0}>
                  <AvailabilityChecksTable
                    resources={model().availabilityChecks}
                    emptyIcon={availabilityIcon()}
                    emptyTitle="No availability checks"
                    emptyDescription="Add ping, TCP, MQTT, ESPHome, or HTTP checks for devices and services that cannot run Pulse Agent."
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
