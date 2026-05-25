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
import { AvailabilityChecksTable } from './AvailabilityChecksTable';
import { AgentsMachinesTable } from './AgentsMachinesTable';
import { buildAgentsPageModel } from './agentsPageModel';

const AGENTS_RESOURCE_QUERY = 'type=agent,network-endpoint';
const AGENTS_TAB_SPECS = [
  { id: 'overview', label: 'Overview', path: '/agents/overview' },
  { id: 'availability', label: 'Availability checks', path: '/agents/availability' },
] as const;
type AgentsTabId = (typeof AGENTS_TAB_SPECS)[number]['id'];

const agentsIcon = () => <ServerIcon class="h-6 w-6 text-slate-400" />;
const availabilityIcon = () => <ActivityIcon class="h-6 w-6 text-slate-400" />;

const resolveAgentsTab = (pathname: string): AgentsTabId =>
  pathname.replace(/\/+$/, '') === '/agents/availability' ? 'availability' : 'overview';

export function AgentsPageSurface() {
  const location = useLocation();
  const navigate = useNavigate();
  const { resources, loading, error, refetch } = useUnifiedResources({
    query: AGENTS_RESOURCE_QUERY,
    cacheKey: 'agents-workspace',
    initialHydration: 'prefer-ws-then-rest',
  });

  const [initialLoadComplete, setInitialLoadComplete] = createSignal(false);

  const model = createMemo(() => buildAgentsPageModel(resources()));
  const activeTab = createMemo(() => resolveAgentsTab(location.pathname));
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
    <div data-testid="agents-page" class="space-y-4">
      <PlatformSectionTabs tabs={AGENTS_TAB_SPECS} active={activeTab()} ariaLabel="Agents sections" />

      <Show
        when={!showLoading()}
        fallback={
          <PlatformTableLoadingState
            title="Loading Pulse Agent resources"
            description="Pulse is loading the agent-backed machine snapshot."
          />
        }
      >
        <Show
          when={!error()}
          fallback={
            <PlatformErrorState
              title="Could not load Pulse Agent resources"
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
                  icon={agentsIcon()}
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
                    emptyIcon={agentsIcon()}
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

export default AgentsPageSurface;
