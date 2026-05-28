import { Show, Suspense, createMemo, createSignal, type Component } from 'solid-js';

import type { HistoryTimeRange } from '@/api/charts';
import { DiscoveryTab } from '@/components/Discovery/DiscoveryTab';
import { DrawerSubjectHeading } from '@/components/shared/DrawerSubjectHeading';
import { Subtabs, type SubtabOption } from '@/components/shared/Subtabs';
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
      <DrawerSubjectHeading
        headingId={headingId()}
        title={displayName()}
        statusVariant={headerIndicator().variant}
        statusLabel={headerIndicator().label}
      />

      <Subtabs
        class="mb-1"
        ariaLabel="Node drawer sections"
        value={activeTab()}
        onChange={(value) => setActiveTab(value as NodeDrawerTab)}
        tabs={[
          { value: 'overview', label: 'Overview' },
          { value: 'history', label: 'History' },
          ...(props.discoveryTarget?.agentId
            ? [{ value: 'discovery', label: 'Discovery' } satisfies SubtabOption]
            : []),
        ]}
        trailing={
          <Show when={activeTab() === 'history'}>
            <GuestDrawerHistoryRangeSelect range={historyRange()} onRangeChange={setHistoryRange} />
          </Show>
        }
      />

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
