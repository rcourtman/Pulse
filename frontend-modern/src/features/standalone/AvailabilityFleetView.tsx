import { For, Show, createMemo, createSignal, type Component } from 'solid-js';
import RefreshCwIcon from 'lucide-solid/icons/refresh-cw';
import { ResourceDetailDrawer } from '@/components/Infrastructure/ResourceDetailDrawer';
import { Button } from '@/components/shared/Button';
import { MetadataBadge } from '@/components/shared/MetadataBadge';
import { StatusDot } from '@/components/shared/StatusDot';
import type {
  AvailabilityHistoryBucket,
  AvailabilityHistoryTarget,
} from '@/api/availabilityHistory';
import type { Resource, ResourceAvailabilityMeta } from '@/types/resource';
import { formatRelativeTime } from '@/utils/format';
import {
  getAvailabilityProbeEndpointLabel,
  getAvailabilityProbePresentation,
} from '@/utils/availabilityProbePresentation';
import { getProbeSourceChipLabel, type ProbeAgentOption } from '@/utils/availabilityProbeAgents';
import { getStandaloneResourceStatusIndicator } from './standalonePageModel';

type AvailabilityFleetHistoryState = 'reachable' | 'unreachable' | 'indeterminate' | 'unknown';

const statePresentation: Record<
  AvailabilityFleetHistoryState,
  { label: string; className: string }
> = {
  reachable: { label: 'Reachable', className: 'bg-emerald-500 dark:bg-emerald-400' },
  unreachable: { label: 'Unreachable', className: 'bg-rose-500 dark:bg-rose-400' },
  indeterminate: { label: 'Indeterminate', className: 'bg-amber-400 dark:bg-amber-300' },
  unknown: { label: 'Unknown', className: 'bg-slate-300 dark:bg-slate-600' },
};

const availabilityFor = (resource: Resource): ResourceAvailabilityMeta | undefined =>
  resource.availability ??
  (resource.platformData?.availability as ResourceAvailabilityMeta | undefined);

const historyState = (bucket: AvailabilityHistoryBucket): AvailabilityFleetHistoryState => {
  const durations: Array<[AvailabilityFleetHistoryState, number]> = [
    ['unreachable', bucket.unreachableSeconds],
    ['indeterminate', bucket.indeterminateSeconds],
    ['reachable', bucket.reachableSeconds],
    ['unknown', bucket.unknownSeconds],
  ];
  durations.sort((left, right) => right[1] - left[1]);
  return durations[0]?.[0] ?? 'unknown';
};

const formatPercent = (value: number): string =>
  `${value.toLocaleString(undefined, { maximumFractionDigits: 2 })}%`;

const availabilityText = (history: AvailabilityHistoryTarget | undefined): string => {
  if (!history?.summary) return 'History unavailable';
  const { summary } = history;
  if (summary.coveragePercent < 90 || summary.availabilityPercent === undefined) {
    const observed =
      summary.reachableSeconds + summary.unreachableSeconds + summary.indeterminateSeconds;
    const observedMinutes = Math.round(observed / 60);
    return `Insufficient coverage · ${observedMinutes.toLocaleString()}m observed`;
  }
  return `${formatPercent(summary.availabilityPercent)} available · ${formatPercent(summary.coveragePercent)} observed`;
};

const latencyPaths = (
  buckets: readonly AvailabilityHistoryBucket[],
): { path: string; label: string }[] => {
  const values = buckets
    .map((bucket) => bucket.latencyMillis?.average)
    .filter((value): value is number => typeof value === 'number' && Number.isFinite(value));
  if (values.length === 0) return [];
  const min = Math.min(...values);
  const max = Math.max(...values);
  const spread = Math.max(1, max - min);
  const denominator = Math.max(1, buckets.length - 1);
  const paths: { path: string; label: string }[] = [];
  let points: string[] = [];
  const flush = () => {
    if (points.length > 0) {
      paths.push({ path: points.join(' '), label: `${min}–${max} ms reachable latency` });
      points = [];
    }
  };
  buckets.forEach((bucket, index) => {
    const latency = bucket.latencyMillis?.average;
    if (typeof latency !== 'number' || !Number.isFinite(latency)) {
      flush();
      return;
    }
    const x = (index / denominator) * 100;
    const y = 22 - ((latency - min) / spread) * 18;
    points.push(`${x.toFixed(2)},${y.toFixed(2)}`);
  });
  flush();
  return paths;
};

const AvailabilityStateStrip: Component<{ buckets: readonly AvailabilityHistoryBucket[] }> = (
  props,
) => {
  const states = createMemo(() => props.buckets.map(historyState));
  const description = createMemo(() => {
    const counts = new Map<AvailabilityFleetHistoryState, number>();
    for (const state of states()) counts.set(state, (counts.get(state) ?? 0) + 1);
    return (Object.keys(statePresentation) as AvailabilityFleetHistoryState[])
      .filter((state) => (counts.get(state) ?? 0) > 0)
      .map((state) => `${counts.get(state)} ${statePresentation[state].label.toLowerCase()}`)
      .join(', ');
  });
  return (
    <div
      class="flex h-3 w-full overflow-hidden rounded-sm bg-slate-200 dark:bg-slate-700"
      role="img"
      aria-label={`24-hour state history: ${description() || 'no observations'}`}
      data-testid="availability-state-strip"
    >
      <For each={states()}>
        {(state) => (
          <span
            class={`min-w-px flex-1 ${statePresentation[state].className}`}
            title={statePresentation[state].label}
          />
        )}
      </For>
    </div>
  );
};

const AvailabilityLatencyLine: Component<{ buckets: readonly AvailabilityHistoryBucket[] }> = (
  props,
) => {
  const paths = createMemo(() => latencyPaths(props.buckets));
  return (
    <Show
      when={paths().length > 0}
      fallback={
        <div class="flex h-7 items-center text-[10px] text-muted">No reachable latency</div>
      }
    >
      <svg
        viewBox="0 0 100 26"
        preserveAspectRatio="none"
        class="h-7 w-full overflow-visible"
        role="img"
        aria-label={paths()[0]?.label ?? 'Reachable latency'}
        data-testid="availability-latency-line"
      >
        <For each={paths()}>
          {(path) => (
            <polyline
              points={path.path}
              fill="none"
              stroke="currentColor"
              stroke-width="1.75"
              vector-effect="non-scaling-stroke"
              class="text-sky-500 dark:text-sky-300"
            />
          )}
        </For>
      </svg>
    </Show>
  );
};

export const AvailabilityFleetView: Component<{
  resources: readonly Resource[];
  historyByTarget: ReadonlyMap<string, AvailabilityHistoryTarget>;
  historyLoading: boolean;
  historyError?: string;
  probeAgentOptions?: readonly ProbeAgentOption[];
  onRetryHistory?: () => void;
}> = (props) => {
  const [selectedResource, setSelectedResource] = createSignal<Resource>();
  const resolveResourceLabel = (resourceId: string): string | undefined =>
    props.resources.find((resource) => resource.id === resourceId)?.name;

  return (
    <section aria-label="Availability fleet" class="space-y-3">
      <div class="flex flex-wrap items-center justify-between gap-2 rounded-md border border-border bg-surface-alt/40 px-3 py-2 text-xs text-muted">
        <div class="flex flex-wrap items-center gap-x-3 gap-y-1" aria-label="State history legend">
          <For each={Object.values(statePresentation)}>
            {(presentation) => (
              <span class="inline-flex items-center gap-1.5">
                <span
                  class={`h-2.5 w-2.5 rounded-sm ${presentation.className}`}
                  aria-hidden="true"
                />
                {presentation.label}
              </span>
            )}
          </For>
          <span class="text-[10px]">24-hour evidence · reachable latency only</span>
        </div>
        <Show when={props.historyError}>
          <div class="flex items-center gap-2 text-rose-700 dark:text-rose-300">
            <span>History unavailable. Current health is unchanged.</span>
            <Show when={props.onRetryHistory}>
              <Button type="button" size="xs" variant="secondary" onClick={props.onRetryHistory}>
                <RefreshCwIcon class="h-3 w-3" />
                Retry
              </Button>
            </Show>
          </div>
        </Show>
      </div>

      <div
        class="grid grid-cols-1 gap-2 sm:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4"
        data-testid="availability-fleet-grid"
      >
        <For each={props.resources}>
          {(resource) => {
            const availability = () => availabilityFor(resource);
            const probe = () => getAvailabilityProbePresentation(resource);
            const indicator = () => getStandaloneResourceStatusIndicator(resource);
            const targetID = () => availability()?.targetId ?? resource.platformId ?? resource.id;
            const history = () => props.historyByTarget.get(targetID());
            const buckets = () => history()?.buckets ?? [];
            const source = () =>
              getProbeSourceChipLabel(
                props.probeAgentOptions ?? [],
                availability()?.probeAgentId,
              ) ?? 'Local Pulse';
            const endpoint = () => {
              const current = availability();
              return current
                ? getAvailabilityProbeEndpointLabel(current) || resource.name
                : resource.name;
            };
            const checked = () =>
              formatRelativeTime(availability()?.lastChecked, {
                compact: true,
                emptyText: 'Not checked',
              });
            const latency = () => {
              const value = availability()?.latencyMillis;
              return typeof value === 'number' && Number.isFinite(value) && value > 0
                ? `${value.toLocaleString()} ms`
                : undefined;
            };

            return (
              <button
                type="button"
                class="group min-w-0 rounded-md border border-border bg-surface p-3 text-left shadow-sm transition hover:border-blue-400/60 hover:bg-surface-hover focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-500"
                aria-label={`Open details for ${resource.name}`}
                data-availability-fleet-tile={resource.id}
                onClick={() => setSelectedResource(resource)}
              >
                <div class="flex items-start justify-between gap-3">
                  <div class="min-w-0">
                    <div class="flex min-w-0 items-center gap-2">
                      <StatusDot
                        size="sm"
                        variant={indicator().variant}
                        title={indicator().label}
                        ariaHidden
                      />
                      <span
                        class="truncate text-sm font-semibold text-base-content"
                        title={resource.name}
                      >
                        {resource.name}
                      </span>
                    </div>
                    <div class="mt-1 truncate text-[11px] text-muted" title={endpoint()}>
                      {endpoint()}
                    </div>
                  </div>
                  <span class={probe()?.toneClassName ?? 'text-muted'}>
                    {probe()?.resultLabel ?? indicator().label}
                  </span>
                </div>

                <div class="mt-2 flex flex-wrap items-center gap-1.5 text-[10px] text-muted">
                  <MetadataBadge tone="muted" size="xs" appearance="outline">
                    {probe()?.methodLabel ?? availability()?.protocol ?? 'Probe'}
                  </MetadataBadge>
                  <MetadataBadge tone="muted" size="xs" appearance="outline">
                    {source()}
                  </MetadataBadge>
                  <span>Checked {checked()}</span>
                  <Show when={latency()}>{(value) => <span>{value()}</span>}</Show>
                </div>

                <div class="mt-3 space-y-1.5">
                  <Show
                    when={!props.historyLoading}
                    fallback={
                      <div
                        class="h-3 animate-pulse rounded-sm bg-surface-hover"
                        aria-label="Loading history"
                      />
                    }
                  >
                    <AvailabilityStateStrip buckets={buckets()} />
                  </Show>
                  <AvailabilityLatencyLine buckets={buckets()} />
                  <div class="flex items-center justify-between gap-2 text-[10px] text-muted">
                    <span>{availabilityText(history())}</span>
                    <Show when={history()?.revisionBoundaries?.length}>
                      <span title="The check configuration changed during this window">
                        {history()?.revisionBoundaries?.length} revision
                        {history()?.revisionBoundaries?.length === 1 ? '' : 's'}
                      </span>
                    </Show>
                  </div>
                </div>
              </button>
            );
          }}
        </For>
      </div>

      <Show when={selectedResource()}>
        {(resource) => (
          <ResourceDetailDrawer
            resource={resource()}
            resolveResourceLabel={resolveResourceLabel}
            onClose={() => setSelectedResource(undefined)}
          />
        )}
      </Show>
    </section>
  );
};

export default AvailabilityFleetView;
