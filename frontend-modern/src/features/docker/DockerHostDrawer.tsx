import { Show, Suspense, createMemo, createSignal, type Component } from 'solid-js';

import type { HistoryTimeRange } from '@/api/charts';
import { DiscoveryTab } from '@/components/Discovery/DiscoveryTab';
import { DrawerSubjectHeading } from '@/components/shared/DrawerSubjectHeading';
import { Subtabs, type SubtabOption } from '@/components/shared/Subtabs';
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
      <DrawerSubjectHeading
        headingId={headingId()}
        title={displayName()}
        statusVariant={headerIndicator().variant}
        statusLabel={headerIndicator().label}
      />

      <Subtabs
        class="mb-1"
        ariaLabel="Docker host drawer sections"
        value={activeTab()}
        onChange={(value) => setActiveTab(value as DockerHostDrawerTab)}
        tabs={[
          { value: 'overview', label: 'Overview' },
          { value: 'history', label: 'History' },
          ...(discoveryConfig()
            ? [{ value: 'discovery', label: 'Discovery' } satisfies SubtabOption]
            : []),
        ]}
        trailing={
          <Show when={activeTab() === 'history'}>
            <GuestDrawerHistoryRangeSelect range={historyRange()} onRangeChange={setHistoryRange} />
          </Show>
        }
      />

      {/* Use CSS hidden instead of Show to avoid mount/unmount which causes scroll jumps.
          overflow-anchor: none prevents browser scroll anchoring from jumping when display toggles. */}
      <div class={activeTab() === 'overview' ? '' : 'hidden'} style={{ 'overflow-anchor': 'none' }}>
        <DockerHostDrawerOverview host={props.host} />
      </div>

      <div class={activeTab() === 'history' ? '' : 'hidden'} style={{ 'overflow-anchor': 'none' }}>
        <GuestDrawerHistory
          fallbackMetrics={fallbackMetrics()}
          groups={DOCKER_HOST_DRAWER_HISTORY_GROUPS}
          range={historyRange()}
          target={historyTarget()}
        />
      </div>

      <Show when={discoveryConfig()}>
        {(config) => (
          <div
            class={activeTab() === 'discovery' ? '' : 'hidden'}
            style={{ 'overflow-anchor': 'none' }}
          >
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
          </div>
        )}
      </Show>
    </section>
  );
};

export default DockerHostDrawer;
