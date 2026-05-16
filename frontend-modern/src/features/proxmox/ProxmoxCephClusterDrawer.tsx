import { For, Show, type Component } from 'solid-js';
import XIcon from 'lucide-solid/icons/x';
import { Card } from '@/components/shared/Card';
import { StatusDot } from '@/components/shared/StatusDot';
import type { StatusIndicatorVariant } from '@/utils/status';
import { formatBytes } from '@/utils/format';
import { asTrimmedString } from '@/utils/stringUtils';
import type { Resource, ResourceCephServiceMeta } from '@/types/resource';

// Inline drawer that renders below the clicked ceph row. Mirrors the
// Pulse Workloads pattern (TableRow inserted under the clicked row,
// spanning all columns), not a slide-over. Surfaces the per-pool and
// per-service detail the row could only summarize as aggregates, plus
// a capacity utilization diagram. Deliberately avoids stat-card
// tiles — duplicating the row numbers in single-stat boxes adds
// nothing.

function classifyHealth(status: string | undefined): {
  variant: StatusIndicatorVariant;
  label: string;
} {
  const raw = (status ?? '').toUpperCase();
  if (raw === 'HEALTH_OK' || raw === 'OK') return { variant: 'success', label: 'Healthy' };
  if (raw === 'HEALTH_WARN' || raw === 'WARN') return { variant: 'warning', label: 'Warning' };
  if (raw === 'HEALTH_ERR' || raw === 'ERROR') return { variant: 'danger', label: 'Critical' };
  return { variant: 'muted', label: raw || 'Unknown' };
}

function classifyService(svc: ResourceCephServiceMeta): {
  variant: StatusIndicatorVariant;
  label: string;
} {
  if (svc.total === 0) return { variant: 'muted', label: 'None' };
  if (svc.running >= svc.total) return { variant: 'success', label: 'OK' };
  if (svc.running === 0) return { variant: 'danger', label: 'Down' };
  return { variant: 'warning', label: 'Partial' };
}

function CapacityBar(props: {
  used: number;
  total: number;
  percent: number;
}) {
  const clamped = Math.max(0, Math.min(100, props.percent));
  const tone =
    clamped >= 90
      ? 'bg-red-500 dark:bg-red-400'
      : clamped >= 75
        ? 'bg-amber-500 dark:bg-amber-400'
        : 'bg-emerald-500 dark:bg-emerald-400';
  const available = Math.max(0, props.total - props.used);
  return (
    <div class="space-y-2">
      <div class="flex items-end justify-between gap-2">
        <div class="min-w-0">
          <div class="text-[10px] uppercase tracking-wide text-muted">Raw capacity</div>
          <div class="flex items-baseline gap-2 text-base-content">
            <span class="text-lg font-semibold tabular-nums">{clamped.toFixed(1)}%</span>
            <span class="text-[11px] text-muted tabular-nums">
              {formatBytes(props.used)} of {formatBytes(props.total)} used
            </span>
          </div>
        </div>
        <div class="text-right">
          <div class="text-[10px] uppercase tracking-wide text-muted">Available</div>
          <div class="text-base-content tabular-nums text-[13px]">{formatBytes(available)}</div>
        </div>
      </div>
      <div
        class="relative h-2.5 w-full overflow-hidden rounded-full bg-surface-alt"
        role="progressbar"
        aria-valuenow={clamped}
        aria-valuemin={0}
        aria-valuemax={100}
      >
        <div
          class={`absolute inset-y-0 left-0 ${tone}`}
          style={{ width: `${clamped}%` }}
        />
      </div>
    </div>
  );
}

function PoolUsageBar(props: { percent: number }) {
  const clamped = Math.max(0, Math.min(100, props.percent));
  const tone =
    clamped >= 90
      ? 'bg-red-500 dark:bg-red-400'
      : clamped >= 75
        ? 'bg-amber-500 dark:bg-amber-400'
        : 'bg-emerald-500 dark:bg-emerald-400';
  return (
    <div class="relative h-1.5 w-32 overflow-hidden rounded-full bg-surface-alt">
      <div class={`absolute inset-y-0 left-0 ${tone}`} style={{ width: `${clamped}%` }} />
    </div>
  );
}

export const ProxmoxCephClusterDrawer: Component<{
  cluster: Resource;
  onClose: () => void;
}> = (props) => {
  const meta = () => props.cluster.ceph;
  const health = () => classifyHealth(meta()?.healthStatus);
  const pools = () => meta()?.pools ?? [];
  const services = () => meta()?.services ?? [];
  const totalCapacity = () => props.cluster.disk?.total ?? 0;
  const usedCapacity = () => props.cluster.disk?.used ?? 0;
  const usagePercent = () => props.cluster.disk?.current ?? 0;
  const fsid = () => asTrimmedString(meta()?.fsid) || '—';

  return (
    <div class="space-y-4">
      <header class="flex items-start justify-between gap-3">
        <div class="min-w-0 space-y-1">
          <div class="flex items-center gap-2 min-w-0">
            <StatusDot size="md" variant={health().variant} title={health().label} ariaHidden />
            <h3 class="truncate text-sm font-semibold text-base-content">
              {asTrimmedString(props.cluster.name) || props.cluster.id}
            </h3>
            <span class="shrink-0 text-[10px] font-mono text-muted">
              {meta()?.healthStatus ?? ''}
            </span>
          </div>
          <div class="font-mono text-[10px] text-muted">
            FSID <span class="break-all">{fsid()}</span>
          </div>
          <Show when={meta()?.healthMessage}>
            <p class="text-xs text-amber-700 dark:text-amber-300">{meta()?.healthMessage}</p>
          </Show>
        </div>
        <button
          type="button"
          onClick={props.onClose}
          class="shrink-0 inline-flex h-7 w-7 items-center justify-center rounded-sm text-muted hover:bg-surface-hover hover:text-base-content"
          aria-label="Close ceph cluster drawer"
        >
          <XIcon class="h-4 w-4" />
        </button>
      </header>

      <Card padding="md">
        <CapacityBar
          used={usedCapacity()}
          total={totalCapacity()}
          percent={usagePercent()}
        />
      </Card>

      <div class="grid gap-3 lg:grid-cols-2">
        <Card padding="md">
          <div class="mb-2 flex items-baseline justify-between">
            <h4 class="text-xs font-semibold uppercase tracking-wide text-muted">Pools</h4>
            <span class="text-[10px] text-muted tabular-nums">
              {pools().length} pool{pools().length === 1 ? '' : 's'}
            </span>
          </div>
          <Show
            when={pools().length > 0}
            fallback={<p class="text-xs text-muted">No pools reported.</p>}
          >
            <table class="w-full text-xs">
              <thead class="text-[10px] uppercase tracking-wide text-muted">
                <tr>
                  <th class="pb-2 text-left font-medium">Pool</th>
                  <th class="pb-2 text-right font-medium">Objects</th>
                  <th class="pb-2 text-right font-medium">Stored</th>
                  <th class="pb-2 text-right font-medium">Avail</th>
                  <th class="pb-2 text-right font-medium">Used</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-border-subtle">
                <For each={pools()}>
                  {(pool) => (
                    <tr>
                      <td class="py-2 font-medium text-base-content">{pool.name || '—'}</td>
                      <td class="py-2 text-right text-base-content tabular-nums">
                        {pool.objects.toLocaleString()}
                      </td>
                      <td class="py-2 text-right text-base-content tabular-nums">
                        {formatBytes(pool.storedBytes)}
                      </td>
                      <td class="py-2 text-right text-muted tabular-nums">
                        {formatBytes(pool.availableBytes)}
                      </td>
                      <td class="py-2 text-right">
                        <div class="flex items-center justify-end gap-2">
                          <PoolUsageBar percent={pool.percentUsed} />
                          <span class="w-12 text-right text-base-content tabular-nums">
                            {pool.percentUsed.toFixed(1)}%
                          </span>
                        </div>
                      </td>
                    </tr>
                  )}
                </For>
              </tbody>
            </table>
          </Show>
        </Card>

        <Card padding="md">
          <div class="mb-2 flex items-baseline justify-between">
            <h4 class="text-xs font-semibold uppercase tracking-wide text-muted">Services</h4>
            <span class="text-[10px] text-muted tabular-nums">
              {services().length} daemon type{services().length === 1 ? '' : 's'}
            </span>
          </div>
          <Show
            when={services().length > 0}
            fallback={<p class="text-xs text-muted">No services reported.</p>}
          >
            <table class="w-full text-xs">
              <thead class="text-[10px] uppercase tracking-wide text-muted">
                <tr>
                  <th class="pb-2 text-left font-medium">Service</th>
                  <th class="pb-2 text-left font-medium">Status</th>
                  <th class="pb-2 text-right font-medium">Running</th>
                  <th class="pb-2 text-right font-medium">Total</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-border-subtle">
                <For each={services()}>
                  {(svc) => {
                    const cls = classifyService(svc);
                    return (
                      <tr>
                        <td class="py-2 font-mono text-[11px] font-semibold text-base-content uppercase">
                          {svc.type}
                        </td>
                        <td class="py-2">
                          <div class="flex items-center gap-2">
                            <StatusDot size="sm" variant={cls.variant} title={cls.label} ariaHidden />
                            <span class="text-[11px] font-medium text-base-content">{cls.label}</span>
                          </div>
                        </td>
                        <td class="py-2 text-right text-base-content tabular-nums">{svc.running}</td>
                        <td class="py-2 text-right text-muted tabular-nums">{svc.total}</td>
                      </tr>
                    );
                  }}
                </For>
              </tbody>
            </table>
          </Show>
        </Card>
      </div>

      <Show when={(props.cluster.tags ?? []).length > 0}>
        <div class="flex flex-wrap items-center gap-1">
          <span class="text-[10px] uppercase tracking-wide text-muted">Tags</span>
          <For each={props.cluster.tags ?? []}>
            {(tag) => (
              <span class="inline-flex items-center rounded-sm bg-surface-alt px-1.5 py-0.5 text-[10px] font-mono text-base-content">
                {tag}
              </span>
            )}
          </For>
        </div>
      </Show>
    </div>
  );
};

export default ProxmoxCephClusterDrawer;
