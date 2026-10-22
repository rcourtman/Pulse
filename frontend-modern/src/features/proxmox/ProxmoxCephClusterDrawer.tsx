import { For, Show, Suspense, createMemo, createSignal, lazy, type Component } from 'solid-js';
import type { HistoryTimeRange } from '@/api/charts';
import { useWebSocket } from '@/contexts/appRuntime';
import { ResourceOperatorStateSection } from '@/components/Infrastructure/ResourceOperatorStateSection';
import {
  getResourceMetricsHistoryCurrentMetrics,
  getResourceMetricsHistoryGroups,
  getResourceMetricsHistoryTarget,
} from '@/components/Infrastructure/resourceDetailDrawerMetricsHistoryModel';
import { DetailSectionTable } from '@/components/shared/DetailSectionTable';
import {
  DrawerAttentionSection,
  DrawerNoAlertsLine,
} from '@/components/shared/DrawerAttentionSection';
import { DrawerSubjectHeading } from '@/components/shared/DrawerSubjectHeading';
import { InlineNotice } from '@/components/shared/InlineNotice';
import { ObjectDrawerHeader } from '@/components/shared/ObjectDrawerHeader';
import { Subtabs } from '@/components/shared/Subtabs';
import {
  TechnicalDetailsDisclosure,
  TechnicalDetailsSection,
} from '@/components/shared/TechnicalDetailsDisclosure';
import { DrawerExceptionList } from '@/components/Workloads/DrawerExceptionList';
import { getDrawerChartAlerts } from '@/components/Workloads/drawerOverviewModel';
import { GUEST_DRAWER_HISTORY_DEFAULT_RANGE } from '@/components/Workloads/guestDrawerModel';
import { alertTypeDisplayLabel } from '@/features/alerts/helpers';
import { getAlertAttentionCopy } from '@/features/alerts/metricAlertPresentation';
import { useAlertsActivation } from '@/stores/alertsActivation';
import type { Resource } from '@/types/resource';
import { getAlertsForUnifiedResource, getOwnAlertsForUnifiedResource } from '@/utils/alerts';
import { useRelativeTimeNow } from '@/utils/relativeTimeClock';
import type { StatusIndicator } from '@/utils/status';
import { asTrimmedString } from '@/utils/stringUtils';
import {
  buildCephClusterContentSections,
  buildCephClusterDetailSections,
  getCephClusterExceptionRows,
  getCephClusterFacts,
} from './proxmoxCephClusterDrawerModel';

const GuestDrawerHistory = lazy(() =>
  import('@/components/Workloads/GuestDrawerHistory').then(({ GuestDrawerHistory }) => ({
    default: GuestDrawerHistory,
  })),
);

// The inline drawer under a Ceph cluster row, in the shape of every object
// drawer: the row's own dot and name, Overview and Manage tabs, alerts first,
// then what is worth checking, capacity history, the pools and daemons, a
// facts line, and the identifiers behind a closed Details.

interface ProxmoxCephClusterDrawerProps {
  cluster: Resource;
  /** The indicator behind the row's own health dot; the header repeats it. */
  statusIndicator: StatusIndicator;
  onClose: () => void;
}

type CephClusterDrawerTab = 'overview' | 'manage';

const CephClusterOverview: Component<{
  cluster: Resource;
  historyRange: HistoryTimeRange;
  onHistoryRangeChange: (range: HistoryTimeRange) => void;
  historyEnabled: boolean;
}> = (props) => {
  const now = useRelativeTimeNow();
  const { activeAlerts } = useWebSocket();
  const alertsActivation = useAlertsActivation();
  const name = () => asTrimmedString(props.cluster.name) || props.cluster.id;
  const alerts = createMemo(() =>
    getAlertsForUnifiedResource(props.cluster, activeAlerts, alertsActivation.detectionEnabled()),
  );
  const historyTarget = createMemo(() => getResourceMetricsHistoryTarget(props.cluster));
  const historyGroups = createMemo(() => getResourceMetricsHistoryGroups(props.cluster));
  // The charts describe the cluster itself, so only its own alerts draw a
  // trigger line on them.
  const chartAlerts = createMemo(() =>
    getDrawerChartAlerts(
      getOwnAlertsForUnifiedResource(
        props.cluster,
        activeAlerts,
        alertsActivation.detectionEnabled(),
      ),
      historyGroups().flatMap((group) => group.series.map((series) => series.metric)),
    ),
  );
  const exceptionRows = createMemo(() =>
    getCephClusterExceptionRows(
      props.cluster,
      alerts().map((alert) => alert.message ?? ''),
    ),
  );
  const contentSections = createMemo(() => buildCephClusterContentSections(props.cluster));
  const hasPools = () => (props.cluster.ceph?.pools ?? []).length > 0;
  const hasServices = () => (props.cluster.ceph?.services ?? []).length > 0;
  const facts = createMemo(() => getCephClusterFacts(props.cluster));
  const detailSections = createMemo(() => buildCephClusterDetailSections(props.cluster));
  const detailSubtitle = () =>
    detailSections()
      .map((section) => section.label.toLowerCase())
      .join(', ');

  return (
    <div class="space-y-3" data-testid="ceph-cluster-drawer-overview">
      <DrawerAttentionSection
        items={alerts().map((alert) => ({
          id: alert.id,
          ...getAlertAttentionCopy(alert, now),
          subject: asTrimmedString(alert.resourceName) || name(),
          metric: alertTypeDisplayLabel(alert.type),
          severity: alert.level,
          acknowledged: alert.acknowledged,
        }))}
      />
      <Show when={alerts().length === 0}>
        <DrawerNoAlertsLine />
      </Show>

      <DrawerExceptionList testId="ceph-cluster-drawer-exceptions" rows={exceptionRows()} />

      <Show when={historyTarget()}>
        {(target) => (
          <Suspense fallback={<div class="h-48" aria-hidden="true" />}>
            <GuestDrawerHistory
              target={target()}
              range={props.historyRange}
              onRangeChange={props.onHistoryRangeChange}
              enabled={props.historyEnabled}
              groups={historyGroups()}
              currentMetrics={getResourceMetricsHistoryCurrentMetrics(props.cluster)}
              alerts={chartAlerts()}
            />
          </Suspense>
        )}
      </Show>

      <Show when={facts().length > 0}>
        <div
          data-testid="ceph-cluster-drawer-facts"
          class="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-muted"
        >
          <For each={facts()}>{(fact) => <span class="wrap-break-word">{fact}</span>}</For>
        </div>
      </Show>

      <Show when={contentSections().length > 0}>
        <TechnicalDetailsSection dataTestId="ceph-cluster-cards" sections={contentSections()} />
      </Show>
      <Show when={!hasPools() || !hasServices()}>
        <InlineNotice tone="info" role="status" data-testid="ceph-cluster-content-empty">
          {!hasPools() && !hasServices()
            ? 'This cluster has not reported pools or services yet.'
            : !hasPools()
              ? 'This cluster has not reported any pools yet.'
              : 'This cluster has not reported its services yet.'}
        </InlineNotice>
      </Show>

      <Show when={detailSections().length > 0}>
        <TechnicalDetailsDisclosure
          dataTestId="ceph-cluster-technical-details"
          title="Details"
          keepMounted
          subtitle={detailSubtitle()}
          class="border-t border-border pt-2"
          contentClass="mt-2"
        >
          <DetailSectionTable sections={detailSections()} />
        </TechnicalDetailsDisclosure>
      </Show>
    </div>
  );
};

const ProxmoxCephClusterDrawerContent: Component<ProxmoxCephClusterDrawerProps> = (props) => {
  const [activeTab, setActiveTab] = createSignal<CephClusterDrawerTab>('overview');
  const [historyRange, setHistoryRange] = createSignal<HistoryTimeRange>(
    GUEST_DRAWER_HISTORY_DEFAULT_RANGE,
  );
  const name = () => asTrimmedString(props.cluster.name) || props.cluster.id;
  const headingId = () => `proxmox-ceph-cluster-drawer-heading-${props.cluster.id}`;

  return (
    <section class="space-y-3" aria-labelledby={headingId()} data-testid="ceph-cluster-drawer">
      <ObjectDrawerHeader collapseLabel={`Collapse ${name()} details`} onCollapse={props.onClose}>
        <DrawerSubjectHeading
          headingId={headingId()}
          title={name()}
          statusVariant={props.statusIndicator.variant}
          statusLabel={props.statusIndicator.label}
        />
      </ObjectDrawerHeader>

      <Subtabs
        class="mb-1"
        ariaLabel="Ceph cluster drawer sections"
        value={activeTab()}
        onChange={(value) => setActiveTab(value as CephClusterDrawerTab)}
        tabs={[
          { value: 'overview', label: 'Overview' },
          { value: 'manage', label: 'Manage' },
        ]}
      />

      {/* Hidden with CSS rather than unmounted, so switching tabs keeps the
          scroll position; overflow-anchor stops the browser jumping. */}
      <div class={`[overflow-anchor:none] ${activeTab() === 'overview' ? '' : 'hidden'}`}>
        <CephClusterOverview
          cluster={props.cluster}
          historyRange={historyRange()}
          onHistoryRangeChange={setHistoryRange}
          historyEnabled={activeTab() === 'overview'}
        />
      </div>

      <div class={`[overflow-anchor:none] ${activeTab() === 'manage' ? '' : 'hidden'}`}>
        <Show when={activeTab() === 'manage'}>
          <div data-testid="ceph-cluster-manage-tab">
            <ResourceOperatorStateSection
              resourceId={props.cluster.id}
              resourceType={props.cluster.type}
              platformType={props.cluster.platformType}
              capabilities={props.cluster.capabilities}
            />
          </div>
        </Show>
      </div>
    </section>
  );
};

// Another cluster must not inherit this one's tab, range or operator form.
export const ProxmoxCephClusterDrawer: Component<ProxmoxCephClusterDrawerProps> = (props) => (
  <Show when={props.cluster.id} keyed>
    {(_clusterId) => <ProxmoxCephClusterDrawerContent {...props} />}
  </Show>
);

export default ProxmoxCephClusterDrawer;
