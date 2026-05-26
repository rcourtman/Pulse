import { Show, Suspense, createMemo, createSignal, type Component } from 'solid-js';

import type { HistoryTimeRange } from '@/api/charts';
import { DiscoveryTab } from '@/components/Discovery/DiscoveryTab';
import { StatusDot } from '@/components/shared/StatusDot';
import { getSimpleStatusIndicator } from '@/utils/status';
import { GuestDrawerHistory, GuestDrawerHistoryRangeSelect } from '@/components/Workloads/GuestDrawerHistory';
import { GUEST_DRAWER_HISTORY_DEFAULT_RANGE } from '@/components/Workloads/guestDrawerModel';
import { toDiscoveryConfig } from '@/components/Infrastructure/resourceDetailDiscoveryModel';
import type { Resource } from '@/types/resource';
import { getDiscoveryLoadingState } from '@/utils/discoveryPresentation';
import { asTrimmedString } from '@/utils/stringUtils';

import { DockerHostDrawerOverview } from './DockerHostDrawerOverview';
import {
  DOCKER_HOST_DRAWER_HISTORY_GROUPS,
  getDockerHostDrawerHistoryFallbackMetrics,
  getDockerHostDrawerHistoryTarget,
} from './dockerHostDrawerModel';

interface DockerHostDrawerProps {
  host: Resource;
}

type DockerHostDrawerTab = 'overview' | 'history' | 'discovery';

export const DockerHostDrawer: Component<DockerHostDrawerProps> = (props) => {
  const [activeTab, setActiveTab] = createSignal<DockerHostDrawerTab>('overview');
  const [historyRange, setHistoryRange] = createSignal<HistoryTimeRange>(
    GUEST_DRAWER_HISTORY_DEFAULT_RANGE,
  );

  const headingId = () => `docker-host-drawer-heading-${props.host.id}`;
  const displayName = createMemo(() => asTrimmedString(props.host.name) || props.host.id);
  const historyTarget = createMemo(() => getDockerHostDrawerHistoryTarget(props.host));
  const fallbackMetrics = createMemo(() => getDockerHostDrawerHistoryFallbackMetrics(props.host));
  const discoveryConfig = createMemo(() => toDiscoveryConfig(props.host));
  const headerIndicator = createMemo(() => getSimpleStatusIndicator(props.host.status));

  return (
    <section
      class="space-y-3"
      aria-labelledby={headingId()}
      data-testid="docker-host-drawer"
    >
      <div class="flex items-center gap-2 min-w-0">
        <StatusDot
          size="sm"
          variant={headerIndicator().variant}
          title={headerIndicator().label}
          ariaLabel={headerIndicator().label}
        />
        <h2
          id={headingId()}
          class="text-sm font-semibold text-base-content truncate m-0"
          title={displayName()}
        >
          {displayName()}
        </h2>
      </div>

      <div class="mb-1 flex items-center justify-between gap-3 border-b border-border px-1">
        <div class="flex items-center gap-6">
          <button
            type="button"
            onClick={() => setActiveTab('overview')}
            class={`pb-2 text-sm font-medium transition-colors relative ${
              activeTab() === 'overview' ? 'text-blue-600 dark:text-blue-400' : ' hover:text-muted'
            }`}
          >
            Overview
            {activeTab() === 'overview' && (
              <div class="absolute bottom-0 left-0 right-0 h-0.5 bg-blue-600 dark:bg-blue-400 rounded-t-full" />
            )}
          </button>
          <button
            type="button"
            onClick={() => setActiveTab('history')}
            class={`pb-2 text-sm font-medium transition-colors relative ${
              activeTab() === 'history' ? 'text-blue-600 dark:text-blue-400' : ' hover:text-muted'
            }`}
          >
            History
            {activeTab() === 'history' && (
              <div class="absolute bottom-0 left-0 right-0 h-0.5 bg-blue-600 dark:bg-blue-400 rounded-t-full" />
            )}
          </button>
          <Show when={discoveryConfig()}>
            <button
              type="button"
              onClick={() => setActiveTab('discovery')}
              class={`pb-2 text-sm font-medium transition-colors relative ${
                activeTab() === 'discovery'
                  ? 'text-blue-600 dark:text-blue-400'
                  : ' hover:text-muted'
              }`}
            >
              Discovery
              {activeTab() === 'discovery' && (
                <div class="absolute bottom-0 left-0 right-0 h-0.5 bg-blue-600 dark:bg-blue-400 rounded-t-full" />
              )}
            </button>
          </Show>
        </div>
        <Show when={activeTab() === 'history'}>
          <div class="pb-1">
            <GuestDrawerHistoryRangeSelect range={historyRange()} onRangeChange={setHistoryRange} />
          </div>
        </Show>
      </div>

      <Show when={activeTab() === 'overview'}>
        <DockerHostDrawerOverview host={props.host} />
      </Show>

      <Show when={activeTab() === 'history'}>
        <GuestDrawerHistory
          fallbackMetrics={fallbackMetrics()}
          groups={DOCKER_HOST_DRAWER_HISTORY_GROUPS}
          range={historyRange()}
          target={historyTarget()}
        />
      </Show>

      <Show when={activeTab() === 'discovery' && discoveryConfig()}>
        {(config) => (
          <Suspense
            fallback={
              <div class="flex items-center justify-center py-8">
                <div class="animate-spin h-6 w-6 border-2 border-blue-500 border-t-transparent rounded-full" />
                <span class="ml-2 text-sm text-muted">{getDiscoveryLoadingState().text}</span>
              </div>
            }
          >
            <DiscoveryTab
              resourceType={config().resourceType}
              agentId={config().agentId}
              resourceId={config().resourceId}
              hostname={config().hostname}
              showManualRunAction
            />
          </Suspense>
        )}
      </Show>
    </section>
  );
};

export default DockerHostDrawer;
