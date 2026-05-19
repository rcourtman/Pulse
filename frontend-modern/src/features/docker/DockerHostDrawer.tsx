import { Show, createMemo, createSignal, type Component } from 'solid-js';

import type { HistoryTimeRange } from '@/api/charts';
import { GuestDrawerHistory, GuestDrawerHistoryRangeSelect } from '@/components/Workloads/GuestDrawerHistory';
import { GUEST_DRAWER_HISTORY_DEFAULT_RANGE } from '@/components/Workloads/guestDrawerModel';
import type { Resource } from '@/types/resource';
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

type DockerHostDrawerTab = 'overview' | 'history';

export const DockerHostDrawer: Component<DockerHostDrawerProps> = (props) => {
  const [activeTab, setActiveTab] = createSignal<DockerHostDrawerTab>('overview');
  const [historyRange, setHistoryRange] = createSignal<HistoryTimeRange>(
    GUEST_DRAWER_HISTORY_DEFAULT_RANGE,
  );

  const headingId = () => `docker-host-drawer-heading-${props.host.id}`;
  const displayName = createMemo(() => asTrimmedString(props.host.name) || props.host.id);
  const historyTarget = createMemo(() => getDockerHostDrawerHistoryTarget(props.host));
  const fallbackMetrics = createMemo(() => getDockerHostDrawerHistoryFallbackMetrics(props.host));

  return (
    <section
      class="space-y-3"
      aria-labelledby={headingId()}
      data-testid="docker-host-drawer"
    >
      <h2 id={headingId()} class="sr-only">
        {displayName()} details
      </h2>

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
    </section>
  );
};

export default DockerHostDrawer;
