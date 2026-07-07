import { Show, Suspense, createMemo, createSignal, type Component } from 'solid-js';

import type { HistoryTimeRange } from '@/api/charts';
import { DiscoveryTab } from '@/components/Discovery/DiscoveryTab';
import { DiscoveryLoadingFallback } from '@/components/shared/DiscoveryLoadingFallback';
import { DrawerSubjectHeading } from '@/components/shared/DrawerSubjectHeading';
import { Subtabs, type SubtabOption } from '@/components/shared/Subtabs';
import { nodeOverrideIdCandidates } from '@/features/alerts/alertOverridesModel';
import { useAlertsActivation } from '@/stores/alertsActivation';
import type { Disk, Node } from '@/types/api';
import type { MetricDisplayThresholds } from '@/utils/metricThresholds';
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
  temperatureThresholds?: MetricDisplayThresholds | null;
}

type NodeDrawerTab = 'overview' | 'history' | 'discovery';

export const NodeDrawer: Component<NodeDrawerProps> = (props) => {
  const [activeTab, setActiveTab] = createSignal<NodeDrawerTab>('overview');
  const [historyRange, setHistoryRange] = createSignal<HistoryTimeRange>(
    GUEST_DRAWER_HISTORY_DEFAULT_RANGE,
  );
  const alertsActivation = useAlertsActivation();

  const headingId = () => `node-drawer-heading-${props.node.id}`;
  const displayName = createMemo(() => getNodeDisplayName(props.node));
  const historyTarget = createMemo(() => getNodeDrawerHistoryTarget(props.node));
  const fallbackMetrics = createMemo(() => getNodeDrawerHistoryFallbackMetrics(props.node));
  const headerIndicator = createMemo(() => getSimpleStatusIndicator(props.node.status));
  const temperatureThresholds = createMemo(() =>
    props.temperatureThresholds !== undefined
      ? props.temperatureThresholds
      : alertsActivation.getMetricThresholds(
          'node',
          'temperature',
          nodeOverrideIdCandidates(props.node),
        ),
  );

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

      {/* Use CSS hidden instead of Show to avoid mount/unmount which causes scroll jumps.
          overflow-anchor: none prevents browser scroll anchoring from jumping when display toggles. */}
      <div class={activeTab() === 'overview' ? '' : 'hidden'} style={{ 'overflow-anchor': 'none' }}>
        <NodeDrawerOverview
          node={props.node}
          disks={props.disks}
          temperatureThresholds={temperatureThresholds()}
        />
      </div>

      <div class={activeTab() === 'history' ? '' : 'hidden'} style={{ 'overflow-anchor': 'none' }}>
        <GuestDrawerHistory
          fallbackMetrics={fallbackMetrics()}
          groups={NODE_DRAWER_HISTORY_GROUPS}
          range={historyRange()}
          target={historyTarget()}
        />
      </div>

      {props.discoveryTarget?.agentId && (
        <div
          class={activeTab() === 'discovery' ? '' : 'hidden'}
          style={{ 'overflow-anchor': 'none' }}
        >
          <Suspense fallback={<DiscoveryLoadingFallback />}>
            <DiscoveryTab
              resourceType="agent"
              agentId={props.discoveryTarget.agentId}
              resourceId={props.discoveryTarget.agentId}
              hostname={props.discoveryTarget?.hostname || displayName()}
              showManualRunAction
            />
          </Suspense>
        </div>
      )}
    </section>
  );
};
