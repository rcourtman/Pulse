import { Show, createEffect, createMemo, createSignal } from 'solid-js';
import { useNavigate } from '@solidjs/router';
import ServerIcon from 'lucide-solid/icons/server';
import SettingsIcon from 'lucide-solid/icons/settings';
import { buildInfrastructureOnboardingPath } from '@/components/Settings/infrastructureWorkspaceModel';
import {
  PlatformErrorState,
  PlatformSectionTabs,
  PlatformTableEmptyState,
  PlatformTableLoadingState,
} from '@/features/platformPage/sharedPlatformPage';
import { useUnifiedResources } from '@/hooks/useUnifiedResources';
import { AgentsMachinesTable } from './AgentsMachinesTable';
import { buildAgentsPageModel } from './agentsPageModel';

const AGENTS_RESOURCE_QUERY = 'type=agent';
const AGENTS_TAB_SPECS = [{ id: 'overview', label: 'Overview', path: '/agents/overview' }] as const;

const agentsIcon = () => <ServerIcon class="h-6 w-6 text-slate-400" />;

export function AgentsPageSurface() {
  const navigate = useNavigate();
  const { resources, loading, error, refetch } = useUnifiedResources({
    query: AGENTS_RESOURCE_QUERY,
    cacheKey: 'agents-workspace',
    initialHydration: 'prefer-ws-then-rest',
  });

  const [initialLoadComplete, setInitialLoadComplete] = createSignal(false);

  const model = createMemo(() => buildAgentsPageModel(resources()));
  const showLoading = createMemo(
    () => loading() && !initialLoadComplete() && model().resources.length === 0,
  );
  const showEmpty = createMemo(
    () => initialLoadComplete() && !error() && model().resources.length === 0,
  );

  createEffect(() => {
    if (!loading() && !initialLoadComplete()) {
      setInitialLoadComplete(true);
    }
  });

  return (
    <div data-testid="agents-page" class="space-y-4">
      <PlatformSectionTabs tabs={AGENTS_TAB_SPECS} active="overview" ariaLabel="Agents sections" />

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
          <Show
            when={!showEmpty()}
            fallback={
              <PlatformTableEmptyState
                icon={agentsIcon()}
                title="No Pulse Agent machines"
                description="Install the Pulse Agent on Linux, macOS, Windows, Unraid, or another host to populate this platform page."
                actions={
                  <button
                    type="button"
                    onClick={() => navigate(buildInfrastructureOnboardingPath('pick'))}
                    class="inline-flex items-center gap-2 rounded-md border border-border bg-surface px-3 py-1.5 text-xs font-medium text-base-content shadow-sm hover:bg-slate-50"
                  >
                    <SettingsIcon class="h-3.5 w-3.5" />
                    Add agent
                  </button>
                }
              />
            }
          >
            <AgentsMachinesTable
              resources={model().resources}
              emptyIcon={agentsIcon()}
              emptyTitle="No Pulse Agent machines"
              emptyDescription="Install the Pulse Agent on Linux, macOS, Windows, Unraid, or another host to populate this platform page."
            />
          </Show>
        </Show>
      </Show>
    </div>
  );
}

export default AgentsPageSurface;
