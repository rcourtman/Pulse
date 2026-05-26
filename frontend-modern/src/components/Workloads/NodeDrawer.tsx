import { Show, Suspense, createMemo, createSignal, type Component } from 'solid-js';

import type { HistoryTimeRange } from '@/api/charts';
import { DiscoveryTab } from '@/components/Discovery/DiscoveryTab';
import { StatusDot } from '@/components/shared/StatusDot';
import type { Disk, Node } from '@/types/api';
import { getDiscoveryLoadingState } from '@/utils/discoveryPresentation';
import { getNodeDisplayName } from '@/utils/nodes';
import { getSimpleStatusIndicator } from '@/utils/status';

import { GuestDrawerHistory, GuestDrawerHistoryRangeSelect } from './GuestDrawerHistory';
import { GUEST_DRAWER_HISTORY_DEFAULT_RANGE } from './guestDrawerModel';
import { NodeDrawerOverview } from './NodeDrawerOverview';
import {
  NODE_DRAWER_HISTORY_GROUPS,
  getNodeDrawerHistoryFallbackMetrics,
  getNodeDrawerHistoryTarget,
} from './nodeDrawerModel';

export interface NodeDrawerDiscoveryTarget {
  agentId: string;
  hostname?: string;
}

interface NodeDrawerProps {
  node: Node;
  disks?: Disk[];
  discoveryTarget?: NodeDrawerDiscoveryTarget;
}

type NodeDrawerTab = 'overview' | 'history' | 'discovery';

export const NodeDrawer: Component<NodeDrawerProps> = (props) => {
  const [activeTab, setActiveTab] = createSignal<NodeDrawerTab>('overview');
  const [historyRange, setHistoryRange] = createSignal<HistoryTimeRange>(
    GUEST_DRAWER_HISTORY_DEFAULT_RANGE,
  );

  const headingId = () => `node-drawer-heading-${props.node.id}`;
  const displayName = createMemo(() => getNodeDisplayName(props.node));
  const historyTarget = createMemo(() => getNodeDrawerHistoryTarget(props.node));
  const fallbackMetrics = createMemo(() => getNodeDrawerHistoryFallbackMetrics(props.node));
  const headerIndicator = createMemo(() => getSimpleStatusIndicator(props.node.status));

  return (
    <section class="space-y-3" aria-labelledby={headingId()} data-testid="node-drawer">
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
          <Show when={props.discoveryTarget?.agentId}>
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
        <NodeDrawerOverview node={props.node} disks={props.disks} />
      </Show>

      <Show when={activeTab() === 'history'}>
        <GuestDrawerHistory
          fallbackMetrics={fallbackMetrics()}
          groups={NODE_DRAWER_HISTORY_GROUPS}
          range={historyRange()}
          target={historyTarget()}
        />
      </Show>

      <Show when={activeTab() === 'discovery' && props.discoveryTarget?.agentId}>
        {(agentId) => (
          <Suspense
            fallback={
              <div class="flex items-center justify-center py-8">
                <div class="animate-spin h-6 w-6 border-2 border-blue-500 border-t-transparent rounded-full" />
                <span class="ml-2 text-sm text-muted">{getDiscoveryLoadingState().text}</span>
              </div>
            }
          >
            <DiscoveryTab
              resourceType="agent"
              agentId={agentId()}
              resourceId={agentId()}
              hostname={props.discoveryTarget?.hostname || displayName()}
              showManualRunAction
            />
          </Suspense>
        )}
      </Show>
    </section>
  );
};
