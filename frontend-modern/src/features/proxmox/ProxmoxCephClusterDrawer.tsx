import { For, Show, type Component } from 'solid-js';
import XIcon from 'lucide-solid/icons/x';
import { Card } from '@/components/shared/Card';
import { ProgressBar } from '@/components/shared/ProgressBar';
import { StatusDot } from '@/components/shared/StatusDot';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/shared/Table';
import {
  PLATFORM_TABLE_BODY_CLASS,
  PLATFORM_TABLE_HEADER_ROW_CLASS,
  getPlatformTableCellClassForKind,
  getPlatformTableHeadClassForKind,
} from '@/features/platformPage/sharedPlatformPage';
import type { StatusIndicatorVariant } from '@/utils/status';
import { formatBytes } from '@/utils/format';
import { getMetricColorClass } from '@/utils/metricThresholds';
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

// Capacity utilization bar — matches the Workloads row convention:
// the label sits on top of the fill, not in a legend below. Reuses
// the shared metric color tokens (bg-metric-normal-bg / warning /
// critical) so the green/amber/red match the rest of the app exactly
// (raw bg-emerald-500 was brighter and clashed with the dark label
// text overlay).
function capacityToneFor(percent: number): string {
  return getMetricColorClass(percent, 'disk');
}

function CapacityBar(props: { used: number; total: number; percent: number }) {
  const clamped = Math.max(0, Math.min(100, props.percent));
  return (
    <ProgressBar
      value={clamped}
      class="h-5"
      fillClass={capacityToneFor(clamped)}
      label={
        <span class="absolute inset-0 flex items-center justify-center text-[10px] font-semibold text-base-content leading-none tabular-nums">
          {clamped.toFixed(1)}% · {formatBytes(props.used)} / {formatBytes(props.total)}
        </span>
      }
    />
  );
}

function PoolUsageBar(props: { percent: number }) {
  const clamped = Math.max(0, Math.min(100, props.percent));
  return (
    <ProgressBar
      value={clamped}
      class="h-4 w-32"
      fillClass={capacityToneFor(clamped)}
      label={
        <span class="absolute inset-0 flex items-center justify-center text-[10px] font-semibold text-base-content leading-none tabular-nums">
          {clamped.toFixed(1)}%
        </span>
      }
    />
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

      <CapacityBar used={usedCapacity()} total={totalCapacity()} percent={usagePercent()} />

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
            <Table class="text-xs">
              <TableHeader>
                <TableRow class={PLATFORM_TABLE_HEADER_ROW_CLASS}>
                  <TableHead class={getPlatformTableHeadClassForKind('name')}>Pool</TableHead>
                  <TableHead class={getPlatformTableHeadClassForKind('numeric-value')}>
                    Objects
                  </TableHead>
                  <TableHead class={getPlatformTableHeadClassForKind('numeric-value')}>
                    Stored
                  </TableHead>
                  <TableHead class={getPlatformTableHeadClassForKind('numeric-value')}>
                    Avail
                  </TableHead>
                  <TableHead class={getPlatformTableHeadClassForKind('numeric-value')}>
                    Used
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody class={PLATFORM_TABLE_BODY_CLASS}>
                <For each={pools()}>
                  {(pool) => (
                    <TableRow>
                      <TableCell
                        class={`${getPlatformTableCellClassForKind('name')} font-medium text-base-content`}
                      >
                        {pool.name || '—'}
                      </TableCell>
                      <TableCell
                        class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content tabular-nums`}
                      >
                        {pool.objects.toLocaleString()}
                      </TableCell>
                      <TableCell
                        class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content tabular-nums`}
                      >
                        {formatBytes(pool.storedBytes)}
                      </TableCell>
                      <TableCell
                        class={`${getPlatformTableCellClassForKind('numeric-value')} text-muted tabular-nums`}
                      >
                        {formatBytes(pool.availableBytes)}
                      </TableCell>
                      <TableCell class={getPlatformTableCellClassForKind('numeric-value')}>
                        <div class="flex items-center justify-end">
                          <PoolUsageBar percent={pool.percentUsed} />
                        </div>
                      </TableCell>
                    </TableRow>
                  )}
                </For>
              </TableBody>
            </Table>
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
            <Table class="text-xs">
              <TableHeader>
                <TableRow class={PLATFORM_TABLE_HEADER_ROW_CLASS}>
                  <TableHead class={getPlatformTableHeadClassForKind('name')}>Service</TableHead>
                  <TableHead class={getPlatformTableHeadClassForKind('text')}>Status</TableHead>
                  <TableHead class={getPlatformTableHeadClassForKind('numeric-value')}>
                    Running
                  </TableHead>
                  <TableHead class={getPlatformTableHeadClassForKind('numeric-value')}>
                    Total
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody class={PLATFORM_TABLE_BODY_CLASS}>
                <For each={services()}>
                  {(svc) => {
                    const cls = classifyService(svc);
                    return (
                      <TableRow>
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('name')} font-mono text-[11px] font-semibold text-base-content uppercase`}
                        >
                          {svc.type}
                        </TableCell>
                        <TableCell class={getPlatformTableCellClassForKind('text')}>
                          <div class="flex items-center gap-2">
                            <StatusDot
                              size="sm"
                              variant={cls.variant}
                              title={cls.label}
                              ariaHidden
                            />
                            <span class="text-[11px] font-medium text-base-content">
                              {cls.label}
                            </span>
                          </div>
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content tabular-nums`}
                        >
                          {svc.running}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('numeric-value')} text-muted tabular-nums`}
                        >
                          {svc.total}
                        </TableCell>
                      </TableRow>
                    );
                  }}
                </For>
              </TableBody>
            </Table>
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
