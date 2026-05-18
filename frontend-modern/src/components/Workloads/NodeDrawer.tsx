import { Show, createMemo, createSignal, type Component } from 'solid-js';

import type { HistoryTimeRange } from '@/api/charts';
import type { Disk, Node } from '@/types/api';
import { getNodeDisplayName } from '@/utils/nodes';

import { GuestDrawerHistory, GuestDrawerHistoryRangeSelect } from './GuestDrawerHistory';
import { GUEST_DRAWER_HISTORY_DEFAULT_RANGE } from './guestDrawerModel';
import { NodeDrawerOverview } from './NodeDrawerOverview';
import {
  NODE_DRAWER_HISTORY_GROUPS,
  getNodeDrawerHistoryFallbackMetrics,
  getNodeDrawerHistoryTarget,
} from './nodeDrawerModel';

interface NodeDrawerProps {
  node: Node;
  disks?: Disk[];
}

type NodeDrawerTab = 'overview' | 'history';

export const NodeDrawer: Component<NodeDrawerProps> = (props) => {
  const [activeTab, setActiveTab] = createSignal<NodeDrawerTab>('overview');
  const [historyRange, setHistoryRange] = createSignal<HistoryTimeRange>(
    GUEST_DRAWER_HISTORY_DEFAULT_RANGE,
  );

  const headingId = () => `node-drawer-heading-${props.node.id}`;
  const displayName = createMemo(() => getNodeDisplayName(props.node));
  const historyTarget = createMemo(() => getNodeDrawerHistoryTarget(props.node));
  const fallbackMetrics = createMemo(() => getNodeDrawerHistoryFallbackMetrics(props.node));

  return (
    <section class="space-y-3" aria-labelledby={headingId()} data-testid="node-drawer">
      <h2 id={headingId()} class="sr-only">
        {displayName()} details
      </h2>

      <div class="mb-1 flex items-center justify-between gap-3 border-b border-border px-1">
        <div class="flex items-center gap-6">
          <button
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
    </section>
  );
};
