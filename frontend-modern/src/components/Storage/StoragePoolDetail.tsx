import { Component, For, Show, Suspense, createMemo, lazy } from 'solid-js';
import type { HistoryTimeRange } from '@/api/charts';
import {
  compactDetailRows,
  makeDetailRow,
  type DetailRow,
  type DetailSection,
} from '@/components/shared/DetailSectionTable';
import {
  DrawerAttentionSection,
  DrawerNoAlertsLine,
} from '@/components/shared/DrawerAttentionSection';
import { StatusDot } from '@/components/shared/StatusDot';
import {
  TechnicalDetailsDisclosure,
  TechnicalDetailsSection,
} from '@/components/shared/TechnicalDetailsDisclosure';
import { DrawerExceptionList } from '@/components/Workloads/DrawerExceptionList';
import { getDrawerChartAlerts } from '@/components/Workloads/drawerOverviewModel';
import { GUEST_DRAWER_HISTORY_DEFAULT_RANGE } from '@/components/Workloads/guestDrawerModel';
import { alertTypeDisplayLabel } from '@/features/alerts/helpers';
import { getAlertAttentionCopy } from '@/features/alerts/metricAlertPresentation';
import {
  getLinkedDiskHealthDotVariant,
  getLinkedDiskTemperatureSeverity,
  getLinkedDiskTemperatureTextClass,
} from '@/features/storageBackups/diskDetailPresentation';
import {
  PHYSICAL_DISK_TEMPERATURE_LAST_KNOWN_CLASS,
  type PhysicalDiskAlertResourceIdResolver,
} from '@/features/storageBackups/diskPresentation';
import type { StorageRecord } from '@/features/storageBackups/models';
import type { StoragePoolDetailLinkedDisk } from '@/features/storageBackups/storagePoolDetailPresentation';
import {
  STORAGE_POOL_DRAWER_CHARTED_METRICS,
  STORAGE_POOL_DRAWER_HISTORY_GROUPS,
  buildStoragePoolDrawerDetailSections,
  buildStoragePoolDrawerExceptionRows,
  buildStoragePoolDrawerZfsSections,
  getStoragePoolDrawerCurrentMetrics,
  getStoragePoolDrawerDeferredMetrics,
  getStoragePoolDrawerFacts,
  getStoragePoolLinkedDiskSummary,
} from '@/features/storageBackups/storagePoolDrawerPresentation';
import { useAlertsActivation } from '@/stores/alertsActivation';
import type { Alert } from '@/types/api';
import type { Resource } from '@/types/resource';
import { useRelativeTimeNow } from '@/utils/relativeTimeClock';
import { formatTemperature } from '@/utils/temperature';
import { useStoragePoolDetailModel } from './useStoragePoolDetailModel';

// Loaded once a pool drawer opens, so the Storage table chunk does not carry it.
const GuestDrawerHistory = lazy(() =>
  import('@/components/Workloads/GuestDrawerHistory').then(({ GuestDrawerHistory }) => ({
    default: GuestDrawerHistory,
  })),
);

interface StoragePoolDetailProps {
  record: StorageRecord;
  physicalDisks: Resource[];
  getDiskAlertResourceIds?: PhysicalDiskAlertResourceIdResolver;
  /** The pool's own open alerts, matched the way its row matches them. */
  alerts?: Alert[];
  historyRange?: HistoryTimeRange;
  onHistoryRangeChange?: (range: HistoryTimeRange) => void;
  /** True while Overview is the visible tab; hidden charts do not poll. */
  historyEnabled?: boolean;
}

const cleanText = (value: string | null | undefined): string => (value || '').trim();

/**
 * The pool drawer's Overview: its alerts, what is worth checking, its
 * capacity trend, a facts strip, its ZFS devices and linked disks, then the
 * configuration, topology and datasets behind a closed Details disclosure.
 */
export const StoragePoolDetail: Component<StoragePoolDetailProps> = (props) => {
  const now = useRelativeTimeNow();
  const { getDiskTemperatureThresholds } = useAlertsActivation();
  const { historyTarget, configRows, topologyRows, zfsSummary, linkedDisks } =
    useStoragePoolDetailModel({
      record: () => props.record,
      physicalDisks: () => props.physicalDisks,
      getDiskAlertResourceIds: props.getDiskAlertResourceIds,
    });

  const diskThresholds = (disk: StoragePoolDetailLinkedDisk) =>
    getDiskTemperatureThresholds(disk.diskType, disk.alertResourceIds);

  const exceptionRows = createMemo(() =>
    buildStoragePoolDrawerExceptionRows({
      record: props.record,
      zfsSummary: zfsSummary(),
      linkedDisks: linkedDisks(),
      getDiskHeat: (disk) =>
        getLinkedDiskTemperatureSeverity(disk.temperature, diskThresholds(disk)),
    }),
  );

  const linkedDiskRow = (disk: StoragePoolDetailLinkedDisk): DetailRow | null =>
    makeDetailRow(disk.devPath, getStoragePoolLinkedDiskSummary(disk) || disk.model, {
      layout: 'stacked',
      wrap: true,
      valueContent: (
        <span class="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-0.5 text-left font-normal whitespace-normal">
          <StatusDot variant={getLinkedDiskHealthDotVariant(disk.hasIssue)} size="sm" ariaHidden />
          <span class="wrap-break-word text-base-content">{disk.model}</span>
          <Show when={disk.role}>
            <span class="text-muted">{disk.role}</span>
          </Show>
          <Show when={disk.sizeLabel}>
            <span class="text-muted">{disk.sizeLabel}</span>
          </Show>
          <Show when={disk.state}>
            <span class="text-muted">{disk.state}</span>
          </Show>
          <Show when={disk.ioLabel}>
            <span class="text-muted">{disk.ioLabel}</span>
          </Show>
          <Show when={disk.spunDown}>
            <span class="text-muted">spun down</span>
          </Show>
          <Show when={disk.errorCount > 0}>
            <span class="font-semibold text-amber-700 dark:text-amber-300">
              {disk.errorCount.toLocaleString()} errors
            </span>
          </Show>
          <Show when={disk.temperature > 0}>
            <span
              class={`font-medium ${
                disk.temperatureLastKnownTitle
                  ? PHYSICAL_DISK_TEMPERATURE_LAST_KNOWN_CLASS
                  : getLinkedDiskTemperatureTextClass(disk.temperature, diskThresholds(disk))
              }`}
              title={disk.temperatureLastKnownTitle}
            >
              {formatTemperature(disk.temperature)}
              <Show when={disk.temperatureLastKnownTitle}>
                <span class="sr-only">, last known</span>
              </Show>
            </span>
          </Show>
        </span>
      ),
    });

  // The pool's device tree and the physical disks behind it stay in front:
  // they answer "which disk" when Worth checking names a problem.
  const deviceSections = createMemo((): DetailSection[] => [
    ...buildStoragePoolDrawerZfsSections(zfsSummary()),
    ...(linkedDisks().length > 0
      ? [
          {
            label: `Physical disks (${linkedDisks().length})`,
            testId: 'storage-pool-linked-disks',
            rows: compactDetailRows(linkedDisks().map(linkedDiskRow)),
          },
        ]
      : []),
  ]);

  const detailSections = createMemo(() =>
    buildStoragePoolDrawerDetailSections({
      configRows: configRows(),
      topologyRows: topologyRows(),
      zfsSummary: zfsSummary(),
    }),
  );
  const detailSubtitle = () =>
    detailSections()
      .map((section) => section.label.replace(/\s*\(\d+\)$/, '').toLowerCase())
      .join(', ');

  const facts = createMemo(() => getStoragePoolDrawerFacts(props.record));
  const chartAlerts = createMemo(() =>
    getDrawerChartAlerts(props.alerts, STORAGE_POOL_DRAWER_CHARTED_METRICS),
  );

  return (
    <div class="space-y-3" data-testid="storage-pool-drawer-overview">
      <DrawerAttentionSection
        items={(props.alerts ?? []).map((alert) => ({
          id: alert.id,
          ...getAlertAttentionCopy(alert, now),
          subject: cleanText(alert.resourceName) || props.record.name,
          metric: alertTypeDisplayLabel(alert.type),
          severity: alert.level,
          acknowledged: alert.acknowledged,
        }))}
      />
      <Show when={(props.alerts ?? []).length === 0}>
        <DrawerNoAlertsLine />
      </Show>

      <DrawerExceptionList testId="storage-pool-drawer-exceptions" rows={exceptionRows()} />

      <Suspense fallback={<div class="h-48" aria-hidden="true" />}>
        <GuestDrawerHistory
          target={historyTarget()}
          range={props.historyRange ?? GUEST_DRAWER_HISTORY_DEFAULT_RANGE}
          onRangeChange={props.onHistoryRangeChange}
          enabled={props.historyEnabled ?? true}
          groups={STORAGE_POOL_DRAWER_HISTORY_GROUPS}
          currentMetrics={getStoragePoolDrawerCurrentMetrics(props.record)}
          deferredMetrics={getStoragePoolDrawerDeferredMetrics(props.record)}
          alerts={chartAlerts()}
        />
      </Suspense>

      <Show when={facts().length > 0}>
        <div
          data-testid="storage-pool-drawer-facts"
          class="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-muted"
        >
          <For each={facts()}>
            {(fact) => <span class="max-w-full whitespace-normal wrap-anywhere">{fact}</span>}
          </For>
        </div>
      </Show>

      <Show when={deviceSections().length > 0}>
        <TechnicalDetailsSection dataTestId="storage-pool-devices" sections={deviceSections()} />
      </Show>

      <Show when={detailSections().length > 0}>
        <TechnicalDetailsDisclosure
          dataTestId="storage-pool-technical-details"
          title="Details"
          keepMounted
          subtitle={detailSubtitle()}
          class="border-t border-border pt-2"
          contentClass="mt-2"
          sections={detailSections()}
        />
      </Show>
    </div>
  );
};
