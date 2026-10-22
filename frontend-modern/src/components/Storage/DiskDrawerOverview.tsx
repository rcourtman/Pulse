import { Component, For, Show, Suspense, createMemo, lazy } from 'solid-js';
import type { HistoryTimeRange } from '@/api/charts';
import type { DetailSection } from '@/components/shared/DetailSectionTable';
import {
  DrawerAttentionSection,
  DrawerNoAlertsLine,
} from '@/components/shared/DrawerAttentionSection';
import {
  TechnicalDetailsDisclosure,
  TechnicalDetailsSection,
} from '@/components/shared/TechnicalDetailsDisclosure';
import { DrawerExceptionList } from '@/components/Workloads/DrawerExceptionList';
import {
  getAlertStatedRowLabels,
  getDrawerChartAlerts,
  partitionDrawerExceptionRows,
} from '@/components/Workloads/drawerOverviewModel';
import {
  GUEST_DRAWER_HISTORY_DEFAULT_RANGE,
  type GuestDrawerHistoryTarget,
} from '@/components/Workloads/guestDrawerModel';
import { alertTypeDisplayLabel } from '@/features/alerts/helpers';
import { getAlertAttentionCopy } from '@/features/alerts/metricAlertPresentation';
import {
  buildDiskDrawerDetailSections,
  buildDiskDrawerHealthSection,
  getDiskDrawerCurrentMetrics,
  getDiskDrawerDeferredMetrics,
  getDiskDrawerFacts,
  getDiskDrawerHistoryGroups,
  getDiskDrawerNotices,
  isDiskDrawerIOAvailable,
} from '@/features/storageBackups/diskDrawerPresentation';
import type { PhysicalDiskPresentationData } from '@/features/storageBackups/diskPresentation';
import type { Alert } from '@/types/api';
import type { Resource } from '@/types/resource';
import { useRelativeTimeNow } from '@/utils/relativeTimeClock';
import { resolvePhysicalDiskHistoryResourceId } from './diskResourceUtils';

// Loaded once a disk drawer opens, so the Storage table chunk does not carry it.
const GuestDrawerHistory = lazy(() =>
  import('@/components/Workloads/GuestDrawerHistory').then(({ GuestDrawerHistory }) => ({
    default: GuestDrawerHistory,
  })),
);

interface DiskDrawerOverviewProps {
  disk: Resource;
  /** The disk's presentation, judged under its reporting machine's thresholds. */
  diskData: PhysicalDiskPresentationData;
  /** The disk's own open alerts. */
  alerts?: Alert[];
  historyRange?: HistoryTimeRange;
  onHistoryRangeChange?: (range: HistoryTimeRange) => void;
  /** True while Overview is the visible tab; hidden charts do not poll. */
  historyEnabled?: boolean;
}

const cleanText = (value: string | null | undefined): string => (value || '').trim();

/**
 * The disk drawer's Overview: its alerts, what is worth checking, its
 * temperature, I/O and wear history, a facts strip, the SMART health card,
 * then identity, every SMART attribute and the I/O counters behind a closed
 * Details disclosure.
 */
export const DiskDrawerOverview: Component<DiskDrawerOverviewProps> = (props) => {
  const now = useRelativeTimeNow();

  // Snapshot replacement keeps the same coordinates; only a real change of
  // disk identity starts another history read.
  const historyResourceId = createMemo(() => resolvePhysicalDiskHistoryResourceId(props.disk));
  const historyTarget = createMemo((): GuestDrawerHistoryTarget | null => {
    const resourceId = historyResourceId();
    return resourceId ? { resourceType: 'disk', resourceId } : null;
  });
  const historyGroups = createMemo(() =>
    getDiskDrawerHistoryGroups(props.diskData, isDiskDrawerIOAvailable(props.diskData)),
  );
  const chartAlerts = createMemo(() =>
    getDrawerChartAlerts(
      props.alerts,
      historyGroups().flatMap((group) => group.series.map((series) => series.metric)),
    ),
  );

  const healthSection = createMemo(() => buildDiskDrawerHealthSection(props.diskData));
  const detailSections = createMemo(() =>
    buildDiskDrawerDetailSections(props.diskData, props.disk),
  );
  const allSections = (): DetailSection[] => [healthSection(), ...detailSections()];
  // A warning or danger fact moves in front; it is not repeated where it
  // lives. A fact one of the disk's own alerts already states stays put, so
  // the alert above is not restated beneath it.
  const inventory = createMemo(() =>
    partitionDrawerExceptionRows(allSections(), getAlertStatedRowLabels(props.alerts ?? [])),
  );
  const exceptionRows = () => inventory().lifted;
  const frontSections = createMemo(() =>
    inventory().details.filter((section) => section.testId === healthSection().testId),
  );
  const remainingDetailSections = createMemo(() =>
    inventory().details.filter((section) => section.testId !== healthSection().testId),
  );
  const notices = createMemo(() => getDiskDrawerNotices(props.diskData));
  const facts = createMemo(() => getDiskDrawerFacts(props.diskData, props.disk));

  return (
    <div class="space-y-3" data-testid="disk-drawer-overview">
      <DrawerAttentionSection
        items={(props.alerts ?? []).map((alert) => ({
          id: alert.id,
          ...getAlertAttentionCopy(alert, now),
          subject: cleanText(alert.resourceName) || props.diskData.model || props.disk.name,
          metric: alertTypeDisplayLabel(alert.type),
          severity: alert.level,
          acknowledged: alert.acknowledged,
        }))}
      />
      <Show when={(props.alerts ?? []).length === 0}>
        <DrawerNoAlertsLine />
      </Show>

      <DrawerExceptionList testId="disk-drawer-exceptions" rows={exceptionRows()} />

      {/* What the collector could not read says so once, above the charts it
          affects, rather than as an empty chart. */}
      <Show when={notices().length > 0}>
        <div data-testid="disk-drawer-notices" role="status" class="space-y-0.5 text-xs text-muted">
          <For each={notices()}>{(message) => <p>{message}</p>}</For>
        </div>
      </Show>

      <Show
        when={historyTarget()}
        fallback={
          <p data-testid="disk-drawer-history-unavailable" class="text-xs text-muted">
            History is unavailable until Pulse can resolve a stable identity for this disk.
          </p>
        }
      >
        <Suspense fallback={<div class="h-48" aria-hidden="true" />}>
          <GuestDrawerHistory
            target={historyTarget()}
            range={props.historyRange ?? GUEST_DRAWER_HISTORY_DEFAULT_RANGE}
            onRangeChange={props.onHistoryRangeChange}
            enabled={props.historyEnabled ?? true}
            groups={historyGroups()}
            currentMetrics={getDiskDrawerCurrentMetrics(props.diskData)}
            deferredMetrics={getDiskDrawerDeferredMetrics(props.diskData)}
            alerts={chartAlerts()}
          />
        </Suspense>
      </Show>

      <Show when={facts().length > 0}>
        <div
          data-testid="disk-drawer-facts"
          class="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-muted"
        >
          <For each={facts()}>
            {(fact) => <span class="max-w-full whitespace-normal wrap-anywhere">{fact}</span>}
          </For>
        </div>
      </Show>

      <Show when={frontSections().length > 0}>
        <TechnicalDetailsSection dataTestId="disk-drawer-health-card" sections={frontSections()} />
      </Show>

      <Show when={remainingDetailSections().length > 0}>
        <TechnicalDetailsDisclosure
          dataTestId="disk-technical-details"
          title="Details"
          keepMounted
          subtitle={remainingDetailSections()
            .map((section) => section.label.toLowerCase())
            .join(', ')}
          class="border-t border-border pt-2"
          contentClass="mt-2"
          sections={remainingDetailSections()}
        />
      </Show>
    </div>
  );
};
