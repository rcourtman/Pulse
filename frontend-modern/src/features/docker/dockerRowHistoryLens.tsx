import { Show, batch, createMemo, createSignal, type Accessor } from 'solid-js';

import { createSummaryInteractiveRowPreviewHandlers } from '@/components/shared/summaryInteractionA11y';
import {
  MetricHistoryRangeSegmentedControl,
  MetricHoverModeSegmentedControl,
} from '@/components/Workloads/MetricDisplayModeSegmentedControl';
import {
  createRowMetricHistoryQueue,
  type RowMetricHistoryTarget,
} from '@/components/Workloads/rowMetricHistoryQueue';
import {
  WORKLOAD_TABLE_HISTORY_DEFAULT_RANGE,
  getMetricSparklineSeriesFromChartData,
  hasRenderableMetricSeries,
  isWorkloadTableMetricHistoryRange,
  type HostTableMetric,
  type WorkloadMetricSparklineSeries,
  type WorkloadTableMetricHistoryRange,
} from '@/components/Workloads/workloadMetricHistoryModel';
import {
  DEFAULT_WORKLOADS_METRIC_HOVER_MODE,
  type WorkloadsMetricHoverMode,
} from '@/components/Workloads/workloadsFilterModel';
import { usePersistentSignal } from '@/hooks/usePersistentSignal';
import type { Resource } from '@/types/resource';
import { STORAGE_KEYS } from '@/utils/localStorage';

import { getDockerHostDrawerHistoryTarget } from './dockerHostDrawerModel';

/**
 * The Docker page's row-hover History lens. It follows the same saved Row hover
 * and range preferences as the workloads tables, and reads each hovered row's
 * own history through the shared on-demand row queue. Nothing is read until
 * the first hover.
 */
export interface DockerRowHistoryLens {
  hoverMode: Accessor<WorkloadsMetricHoverMode>;
  setHoverMode: (value: WorkloadsMetricHoverMode) => void;
  range: Accessor<WorkloadTableMetricHistoryRange>;
  setRange: (value: WorkloadTableMetricHistoryRange) => void;
  previewRow: (row: Resource) => void;
  clearPreview: (row: Resource) => void;
  seriesFor: (row: Resource, metric: HostTableMetric) => WorkloadMetricSparklineSeries[];
}

const containerHistoryTarget = (container: Resource): RowMetricHistoryTarget | null => {
  const resourceType = container.metricsTarget?.resourceType;
  const resourceId = container.metricsTarget?.resourceId?.trim();
  return resourceType && resourceId ? { rowId: container.id, resourceType, resourceId } : null;
};

const hostHistoryTarget = (host: Resource): RowMetricHistoryTarget | null => {
  const target = getDockerHostDrawerHistoryTarget(host);
  return target ? { rowId: host.id, ...target } : null;
};

export function createDockerRowHistoryLens(options: {
  hosts: Accessor<readonly Resource[]>;
  /** Containers in display order, so the rows beside a hovered one warm next. */
  containers: Accessor<readonly Resource[]>;
}): DockerRowHistoryLens {
  const [hoverMode, setHoverMode] = usePersistentSignal<WorkloadsMetricHoverMode>(
    STORAGE_KEYS.WORKLOADS_METRIC_HOVER_MODE,
    DEFAULT_WORKLOADS_METRIC_HOVER_MODE,
    {
      deserialize: (raw) =>
        raw === 'details' || raw === 'history' ? raw : DEFAULT_WORKLOADS_METRIC_HOVER_MODE,
    },
  );
  const [range, setRange] = usePersistentSignal<WorkloadTableMetricHistoryRange>(
    STORAGE_KEYS.WORKLOADS_METRIC_HISTORY_RANGE,
    WORKLOAD_TABLE_HISTORY_DEFAULT_RANGE,
    {
      deserialize: (raw) =>
        isWorkloadTableMetricHistoryRange(raw) ? raw : WORKLOAD_TABLE_HISTORY_DEFAULT_RANGE,
    },
  );
  const [activeRow, setActiveRow] = createSignal<Resource | null>(null);
  const [hovered, setHovered] = createSignal(false);
  const hostIds = createMemo(() => new Set(options.hosts().map((host) => host.id)));
  const candidates = createMemo(() => [...options.hosts(), ...options.containers()]);

  const queue = createRowMetricHistoryQueue<Resource>({
    range,
    onDemand: () => hoverMode() === 'history' && hovered(),
    activeRow,
    candidates,
    targetFor: (row) =>
      hostIds().has(row.id) ? hostHistoryTarget(row) : containerHistoryTarget(row),
  });

  return {
    hoverMode,
    setHoverMode,
    range,
    setRange,
    previewRow: (row) => {
      if (hoverMode() !== 'history') return;
      // One update, so the queue first sees this row active rather than an
      // idle hover that would warm the top of the list ahead of it.
      batch(() => {
        setHovered(true);
        setActiveRow(row);
      });
    },
    clearPreview: (row) => {
      if (activeRow()?.id === row.id) setActiveRow(null);
    },
    seriesFor: (row, metric) =>
      getMetricSparklineSeriesFromChartData(queue.getHistory(row)?.metrics, metric),
  };
}

/**
 * One row's lens state: whether its sparklines replace the bars, the pointer
 * handlers that open and close it, and the cursor its sparklines share.
 */
export function createDockerRowLensState(
  lens: DockerRowHistoryLens | undefined,
  row: Resource,
  metrics: readonly HostTableMetric[],
) {
  const [active, setActive] = createSignal(false);
  const [cursorRatio, setCursorRatio] = createSignal<number | null>(null);
  const visible = createMemo(
    () =>
      lens !== undefined &&
      lens.hoverMode() === 'history' &&
      active() &&
      metrics.some((metric) => hasRenderableMetricSeries(lens.seriesFor(row, metric))),
  );
  const handlers = createSummaryInteractiveRowPreviewHandlers({
    onPreview: () => {
      if (!lens || lens.hoverMode() !== 'history') return;
      setActive(true);
      lens.previewRow(row);
    },
    onPreviewClear: () => {
      setActive(false);
      setCursorRatio(null);
      lens?.clearPreview(row);
    },
  });
  return {
    visible,
    handlers,
    series: (metric: HostTableMetric) => lens?.seriesFor(row, metric) ?? [],
    cursorProps: () => ({
      cursorRatio: cursorRatio(),
      onCursorRatioChange: setCursorRatio,
      showTooltip: true,
    }),
  };
}

/** The lens settings in the Docker toolbar's View popover. */
export function DockerRowHistoryViewOptions(props: { lens: DockerRowHistoryLens }) {
  return (
    <>
      <div>
        <div class="mb-1.5 text-[10px] font-semibold uppercase tracking-wide text-muted">
          Row hover
        </div>
        <MetricHoverModeSegmentedControl
          value={props.lens.hoverMode()}
          onChange={props.lens.setHoverMode}
        />
      </div>
      <Show when={props.lens.hoverMode() === 'history'}>
        <div>
          <div class="mb-1.5 text-[10px] font-semibold uppercase tracking-wide text-muted">
            History range
          </div>
          <MetricHistoryRangeSegmentedControl
            range={props.lens.range()}
            onRangeChange={props.lens.setRange}
          />
        </div>
      </Show>
    </>
  );
}
