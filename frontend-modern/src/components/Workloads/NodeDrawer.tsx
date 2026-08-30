import { Show, Suspense, createEffect, createMemo, createSignal, type Component } from 'solid-js';

import type { HistoryTimeRange } from '@/api/charts';
import { DiscoveryTab } from '@/components/Discovery/DiscoveryTab';
import { useDiscoveryFeatureAvailability } from '@/components/Discovery/useDiscoveryFeatureAvailability';
import { DiscoveryLoadingFallback } from '@/components/shared/DiscoveryLoadingFallback';
import { ResourceOperatorStateSection } from '@/components/Infrastructure/ResourceOperatorStateSection';
import { DrawerSubjectHeading } from '@/components/shared/DrawerSubjectHeading';
import { ObjectDrawerHeader } from '@/components/shared/ObjectDrawerHeader';
import { Subtabs, type SubtabOption } from '@/components/shared/Subtabs';
import { nodeOverrideIdCandidates } from '@/features/alerts/alertOverridesModel';
import { useAlertsActivation } from '@/stores/alertsActivation';
import type { Alert, Disk, Node } from '@/types/api';
import type { MetricDisplayThresholds } from '@/utils/metricThresholds';
import { getNodeDisplayName } from '@/utils/nodes';
import { getSimpleStatusIndicator } from '@/utils/status';

import { GuestDrawerHistory, GuestDrawerHistoryRangeSelect } from './GuestDrawerHistory';
import { GUEST_DRAWER_HISTORY_DEFAULT_RANGE } from './guestDrawerModel';
import { NodeDrawerOverview } from './NodeDrawerOverview';
import {
  getNodeDrawerCurrentMetrics,
  getNodeDrawerHistoryGroups,
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
  alerts?: Alert[];
  onClose?: () => void;
}

type NodeDrawerTab = 'overview' | 'history' | 'manage' | 'discovery';

export const NodeDrawer: Component<NodeDrawerProps> = (props) => {
  const [activeTab, setActiveTab] = createSignal<NodeDrawerTab>('overview');
  const [historyRange, setHistoryRange] = createSignal<HistoryTimeRange>(
    GUEST_DRAWER_HISTORY_DEFAULT_RANGE,
  );
  const alertsActivation = useAlertsActivation();
  const { discoveryFeatureEnabled } = useDiscoveryFeatureAvailability();

  const headingId = () => `node-drawer-heading-${props.node.id}`;
  const displayName = createMemo(() => getNodeDisplayName(props.node));
  const historyTarget = createMemo(() => getNodeDrawerHistoryTarget(props.node));
  const historyGroups = createMemo(() => getNodeDrawerHistoryGroups(props.node));
  const currentMetrics = createMemo(() => getNodeDrawerCurrentMetrics(props.node));
  const headerIndicator = createMemo(() => getSimpleStatusIndicator(props.node.status));
  const enabledDiscoveryTarget = createMemo(() =>
    discoveryFeatureEnabled() && props.discoveryTarget?.agentId ? props.discoveryTarget : null,
  );
  const hasDiscoverySupport = createMemo(() => Boolean(enabledDiscoveryTarget()));
  const temperatureThresholds = createMemo(() =>
    props.temperatureThresholds !== undefined
      ? props.temperatureThresholds
      : alertsActivation.getMetricThresholds(
          'node',
          'temperature',
          nodeOverrideIdCandidates(props.node),
        ),
  );

  createEffect(() => {
    if (activeTab() === 'discovery' && !hasDiscoverySupport()) {
      setActiveTab('overview');
    }
  });

  return (
    <section class="space-y-3" aria-labelledby={headingId()} data-testid="node-drawer">
      <Show
        when={props.onClose}
        fallback={
          <DrawerSubjectHeading
            headingId={headingId()}
            title={displayName()}
            statusVariant={headerIndicator().variant}
            statusLabel={headerIndicator().label}
          />
        }
      >
        {(onClose) => (
          <ObjectDrawerHeader
            collapseLabel={`Collapse ${displayName()} details`}
            onCollapse={onClose()}
          >
            <DrawerSubjectHeading
              headingId={headingId()}
              title={displayName()}
              statusVariant={headerIndicator().variant}
              statusLabel={headerIndicator().label}
            />
          </ObjectDrawerHeader>
        )}
      </Show>

      <Subtabs
        class="mb-1"
        ariaLabel="Node drawer sections"
        value={activeTab()}
        onChange={(value) => setActiveTab(value as NodeDrawerTab)}
        tabs={[
          { value: 'overview', label: 'Overview' },
          { value: 'history', label: 'History' },
          { value: 'manage', label: 'Manage' },
          ...(hasDiscoverySupport()
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
          alerts={props.alerts}
        />
      </div>

      <div class={activeTab() === 'history' ? '' : 'hidden'} style={{ 'overflow-anchor': 'none' }}>
        <GuestDrawerHistory
          currentMetrics={currentMetrics()}
          groups={historyGroups()}
          range={historyRange()}
          target={historyTarget()}
        />
      </div>

      <div class={activeTab() === 'manage' ? '' : 'hidden'} style={{ 'overflow-anchor': 'none' }}>
        <Show when={activeTab() === 'manage'}>
          <div data-testid="node-manage-tab">
            <ResourceOperatorStateSection
              resourceId={props.node.id}
              resourceType="node"
              platformType="proxmox"
            />
          </div>
        </Show>
      </div>

      <Show when={enabledDiscoveryTarget()}>
        {(target) => (
          <div
            class={activeTab() === 'discovery' ? '' : 'hidden'}
            style={{ 'overflow-anchor': 'none' }}
          >
            <Suspense fallback={<DiscoveryLoadingFallback />}>
              <DiscoveryTab
                resourceType="agent"
                agentId={target().agentId}
                resourceId={target().agentId}
                hostname={target().hostname || displayName()}
                canonicalResourceId={props.node.id}
                showManualRunAction
              />
            </Suspense>
          </div>
        )}
      </Show>
    </section>
  );
};
