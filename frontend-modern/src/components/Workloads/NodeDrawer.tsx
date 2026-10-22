import { Show, createMemo, createSignal, type Component } from 'solid-js';
import ExternalLinkIcon from 'lucide-solid/icons/external-link';

import type { HistoryTimeRange } from '@/api/charts';
import { ResourceOperatorStateSection } from '@/components/Infrastructure/ResourceOperatorStateSection';
import { getDrawerHeaderActionButtonClass } from '@/components/shared/buttonModel';
import { DrawerSubjectHeading } from '@/components/shared/DrawerSubjectHeading';
import { ObjectDrawerHeader } from '@/components/shared/ObjectDrawerHeader';
import { Subtabs } from '@/components/shared/Subtabs';
import { WebInterfaceLink } from '@/components/shared/WebInterfaceLink';
import { nodeOverrideIdCandidates } from '@/features/alerts/alertOverridesModel';
import { useAlertsActivation } from '@/stores/alertsActivation';
import type { Alert, Disk, Node } from '@/types/api';
import type { MetricDisplayThresholds } from '@/utils/metricThresholds';
import { getNodeDisplayName } from '@/utils/nodes';
import { getSimpleStatusIndicator } from '@/utils/status';

import { GUEST_DRAWER_HISTORY_DEFAULT_RANGE } from './guestDrawerModel';
import { NodeDrawerOverview } from './NodeDrawerOverview';
import {
  getNodeDrawerCurrentMetrics,
  getNodeDrawerHistoryGroups,
  getNodeDrawerHistoryTarget,
} from './nodeDrawerModel';

interface NodeDrawerProps {
  node: Node;
  disks?: Disk[];
  temperatureThresholds?: MetricDisplayThresholds | null;
  alerts?: Alert[];
  /**
   * The node's own alerts, matched by its identities rather than by node
   * name. `alerts` also lists its guests'; only these describe its metrics.
   */
  ownAlerts?: Alert[];
  /** The node's web interface, the same link its table row opens. */
  webInterfaceUrl?: string;
  onClose?: () => void;
}

type NodeDrawerTab = 'overview' | 'manage';

export const NodeDrawer: Component<NodeDrawerProps> = (props) => {
  const [activeTab, setActiveTab] = createSignal<NodeDrawerTab>('overview');
  const [historyRange, setHistoryRange] = createSignal<HistoryTimeRange>(
    GUEST_DRAWER_HISTORY_DEFAULT_RANGE,
  );
  const alertsActivation = useAlertsActivation();

  const headingId = () => `node-drawer-heading-${props.node.id}`;
  const displayName = createMemo(() => getNodeDisplayName(props.node));
  const historyTarget = createMemo(() => getNodeDrawerHistoryTarget(props.node));
  const historyGroups = createMemo(() => getNodeDrawerHistoryGroups(props.node));
  const currentMetrics = createMemo(() => getNodeDrawerCurrentMetrics(props.node));
  // The drawer lists alerts for the node's guests too; only the node's own
  // alerts describe the metrics on its charts.
  const ownAlerts = createMemo(() => {
    if (props.ownAlerts) return props.ownAlerts;
    const ids = new Set(nodeOverrideIdCandidates(props.node));
    return (props.alerts ?? []).filter((alert) => ids.has(alert.resourceId));
  });
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
            actions={
              <Show when={props.webInterfaceUrl?.trim()}>
                <WebInterfaceLink
                  url={props.webInterfaceUrl}
                  ariaLabel={`Open web interface for ${displayName()}`}
                  class={getDrawerHeaderActionButtonClass()}
                >
                  <ExternalLinkIcon class="h-3.5 w-3.5" aria-hidden="true" />
                  <span>Open</span>
                </WebInterfaceLink>
              </Show>
            }
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
          { value: 'manage', label: 'Manage' },
        ]}
      />

      {/* Use CSS hidden instead of Show to avoid mount/unmount which causes scroll jumps.
          overflow-anchor: none prevents browser scroll anchoring from jumping when display toggles. */}
      <div class={`[overflow-anchor:none] ${activeTab() === 'overview' ? '' : 'hidden'}`}>
        <NodeDrawerOverview
          node={props.node}
          disks={props.disks}
          temperatureThresholds={temperatureThresholds()}
          alerts={props.alerts}
          historyTarget={historyTarget()}
          historyRange={historyRange()}
          onHistoryRangeChange={setHistoryRange}
          historyGroups={historyGroups()}
          currentMetrics={currentMetrics()}
          historyEnabled={activeTab() === 'overview'}
          metricAlerts={ownAlerts()}
        />
      </div>

      <div class={`[overflow-anchor:none] ${activeTab() === 'manage' ? '' : 'hidden'}`}>
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
    </section>
  );
};
