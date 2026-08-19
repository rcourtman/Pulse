import { For, Show, createMemo, type Component } from 'solid-js';
import { Card } from '@/components/shared/Card';
import type { WorkloadsInventoryStats, WorkloadsInventoryTopology } from './workloadsFilterModel';

const formatCount = (value: number): string => value.toLocaleString();

const segmentWidth = (value: number, total: number): string =>
  `${total > 0 ? Math.max(0, (value / total) * 100) : 0}%`;

type DistributionItem = { label: string; value: number; class: string };

const Distribution: Component<{
  label: string;
  items: DistributionItem[];
  total: number;
}> = (props) => (
  <div class="space-y-2">
    <div class="flex items-center justify-between gap-3">
      <h3 class="text-[10px] font-semibold uppercase tracking-wide text-muted">{props.label}</h3>
      <div class="flex flex-wrap items-center justify-end gap-x-3 gap-y-1 text-[11px] text-muted">
        <For each={props.items.filter((item) => item.value > 0)}>
          {(item) => (
            <span class="inline-flex items-center gap-1.5 whitespace-nowrap">
              <span aria-hidden="true" class={`h-1.5 w-1.5 rounded-full ${item.class}`} />
              <strong class="font-semibold tabular-nums text-base-content">
                {formatCount(item.value)}
              </strong>{' '}
              {item.label}
            </span>
          )}
        </For>
      </div>
    </div>
    <div
      class="flex h-2.5 overflow-hidden rounded-full bg-surface-alt ring-1 ring-inset ring-border-subtle"
      role="img"
      aria-label={`${props.label}: ${props.items
        .map((item) => `${formatCount(item.value)} ${item.label}`)
        .join(', ')}`}
    >
      <For each={props.items}>
        {(item) => (
          <Show when={item.value > 0}>
            <span class={item.class} style={{ width: segmentWidth(item.value, props.total) }} />
          </Show>
        )}
      </For>
    </div>
  </div>
);

export const WorkloadsInventorySummary: Component<{
  stats: WorkloadsInventoryStats;
  topology?: WorkloadsInventoryTopology;
  containerLabel?: string;
}> = (props) => {
  const containerCount = () => props.stats.containers + props.stats.appContainers;
  const primaryStats = createMemo(() => {
    const topology = props.topology;
    if (topology) {
      return [
        { label: 'Workloads', value: props.stats.total },
        { label: 'Nodes', value: topology.nodes },
        { label: 'Clusters', value: topology.clusters },
        { label: 'Standalone', value: topology.standalone },
      ];
    }
    return [
      { label: 'Workloads', value: props.stats.total },
      { label: 'Running', value: props.stats.running },
      { label: 'Attention', value: props.stats.degraded },
    ];
  });
  const typeItems = createMemo<DistributionItem[]>(() =>
    [
      { label: 'VMs', value: props.stats.vms, class: 'bg-sky-500' },
      {
        label: props.containerLabel ?? 'containers',
        value: containerCount(),
        class: 'bg-violet-500',
      },
      { label: 'pods', value: props.stats.pods, class: 'bg-cyan-400' },
    ].filter((item) => item.value > 0),
  );
  const statusItems = createMemo<DistributionItem[]>(() => [
    { label: 'running', value: props.stats.running, class: 'bg-emerald-500' },
    { label: 'attention', value: props.stats.degraded, class: 'bg-amber-500' },
    { label: 'stopped', value: props.stats.stopped, class: 'bg-red-500' },
  ]);

  return (
    <Card
      padding="md"
      class="mb-2 sm:mb-4"
      role="region"
      aria-label="Estate overview"
      data-testid="workloads-estate-overview"
    >
      <div class="mb-3 flex items-center justify-between gap-3">
        <h2 class="text-xs font-semibold text-base-content">Estate overview</h2>
        <span class="text-[10px] font-medium uppercase tracking-wide text-muted">
          Current inventory
        </span>
      </div>
      <div class="grid gap-4 lg:grid-cols-[minmax(18rem,0.8fr)_minmax(28rem,1.2fr)] lg:items-center lg:gap-6">
        <div
          class={`grid grid-cols-2 divide-x divide-border-subtle ${
            props.topology ? 'sm:grid-cols-4' : 'sm:grid-cols-3'
          }`}
        >
          <For each={primaryStats()}>
            {(item, index) => (
              <div class={index() === 0 ? 'pr-3' : 'px-3'}>
                <div class="text-xl font-semibold leading-none tabular-nums text-base-content sm:text-2xl">
                  {formatCount(item.value)}
                </div>
                <div class="mt-1 text-[11px] font-medium text-muted">{item.label}</div>
              </div>
            )}
          </For>
        </div>
        <div class="grid gap-4 sm:grid-cols-2 sm:gap-5">
          <Distribution label="Workload mix" items={typeItems()} total={props.stats.total} />
          <Distribution label="Health" items={statusItems()} total={props.stats.total} />
        </div>
      </div>
    </Card>
  );
};

export default WorkloadsInventorySummary;
