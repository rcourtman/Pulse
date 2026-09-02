import { For, Show, createSignal, type Component } from 'solid-js';
import { Card } from '@/components/shared/Card';
import { DrawerSubjectHeading } from '@/components/shared/DrawerSubjectHeading';
import { InlineDetailTableRow } from '@/components/shared/InlineDetailTableRow';
import { ObjectDrawerHeader } from '@/components/shared/ObjectDrawerHeader';
import { ProgressBar } from '@/components/shared/ProgressBar';
import { StatusDot } from '@/components/shared/StatusDot';
import { TableCell, TableHead, TableRow } from '@/components/shared/Table';
import {
  PlatformDetailTable,
  PlatformDetailTableBody,
  PlatformDetailTableHeader,
  formatPlatformTableIntegerValue,
  formatPlatformTablePercentValue,
  getPlatformTableCellClassForKind,
  getPlatformTableHeadClassForKind,
  PlatformResponsiveTableLabel,
  PlatformTableNumberValue,
} from '@/features/platformPage/sharedPlatformPage';
import { PlatformResourceDetailToggleButton } from '@/features/platformPage/PlatformResourceDetailTableRow';
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
          {formatPlatformTablePercentValue(clamped)} · {formatBytes(props.used)} /{' '}
          {formatBytes(props.total)}
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
      class="h-4 w-full min-w-0 max-w-32 md:min-w-[3.5rem]"
      fillClass={capacityToneFor(clamped)}
      label={
        <span class="absolute inset-0 flex items-center justify-center text-[10px] font-semibold text-base-content leading-none tabular-nums">
          {formatPlatformTablePercentValue(clamped)}
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
  const headingId = () => `proxmox-ceph-cluster-drawer-heading-${props.cluster.id}`;
  const [expandedPoolKey, setExpandedPoolKey] = createSignal<string | null>(null);

  return (
    <section class="space-y-3" aria-labelledby={headingId()}>
      <ObjectDrawerHeader
        collapseLabel={`Collapse ${asTrimmedString(props.cluster.name) || props.cluster.id} details`}
        onCollapse={props.onClose}
      >
        <div class="min-w-0 space-y-1">
          <DrawerSubjectHeading
            headingId={headingId()}
            title={asTrimmedString(props.cluster.name) || props.cluster.id}
            statusVariant={health().variant}
            statusLabel={health().label}
            trailing={
              <span class="shrink-0 text-[10px] font-mono text-muted">
                {meta()?.healthStatus ?? ''}
              </span>
            }
          />
          <div class="font-mono text-[10px] text-muted">
            FSID <span class="break-all">{fsid()}</span>
          </div>
          <Show when={meta()?.healthMessage}>
            <p class="text-xs text-amber-700 dark:text-amber-300">{meta()?.healthMessage}</p>
          </Show>
        </div>
      </ObjectDrawerHeader>

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
            <PlatformDetailTable class="min-w-0 table-fixed text-xs">
              <PlatformDetailTableHeader>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('name')} platform-table-mobile-w-30 md:w-[24%]`}
                >
                  Pool
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('numeric-value')} platform-table-mobile-w-15 md:w-[16%]`}
                >
                  <PlatformResponsiveTableLabel compact="Obj" full="Objects" />
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('numeric-value')} platform-table-mobile-w-20 md:w-[20%]`}
                >
                  <PlatformResponsiveTableLabel compact="Store" full="Stored" />
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('numeric-value')} platform-table-mobile-w-20 md:w-[20%]`}
                >
                  <PlatformResponsiveTableLabel compact="Avail" full="Available" />
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('numeric-value')} platform-table-mobile-w-15 md:w-[20%]`}
                >
                  Used
                </TableHead>
              </PlatformDetailTableHeader>
              <PlatformDetailTableBody>
                <For each={pools()}>
                  {(pool, index) => {
                    const poolKey = () => pool.name || `pool-${index()}`;
                    const isExpanded = () => expandedPoolKey() === poolKey();
                    const detailRowId = () =>
                      `proxmox-ceph-pool-detail-${props.cluster.id}-${index()}`;
                    const toggle = () =>
                      setExpandedPoolKey((current) => (current === poolKey() ? null : poolKey()));
                    return (
                      <>
                        <TableRow class="cursor-pointer" onClick={toggle}>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('name')} font-medium text-base-content`}
                          >
                            <div class="flex min-w-0 items-center gap-1">
                              <PlatformResourceDetailToggleButton
                                expanded={isExpanded()}
                                resourceLabel={pool.name || 'pool'}
                                controlsId={detailRowId()}
                                onToggle={toggle}
                              />
                              <span class="truncate" title={pool.name || '—'}>
                                {pool.name || '—'}
                              </span>
                            </div>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                          >
                            <PlatformTableNumberValue
                              value={pool.objects}
                              format={formatPlatformTableIntegerValue}
                            />
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
                        <Show when={isExpanded()}>
                          <InlineDetailTableRow
                            cellId={detailRowId()}
                            colspan={5}
                            data-inline-proxmox-ceph-pool-detail-for={poolKey()}
                          >
                            <dl class="grid gap-x-4 gap-y-2 text-[11px] sm:grid-cols-2">
                              <div class="sm:col-span-2 text-xs font-semibold text-base-content">
                                {pool.name || 'Pool'}
                              </div>
                              <div>
                                <dt class="text-[10px] font-medium uppercase tracking-wide text-muted">
                                  Objects
                                </dt>
                                <dd class="font-mono text-base-content">
                                  {formatPlatformTableIntegerValue(pool.objects)}
                                </dd>
                              </div>
                              <div>
                                <dt class="text-[10px] font-medium uppercase tracking-wide text-muted">
                                  Stored
                                </dt>
                                <dd class="font-mono text-base-content">
                                  {formatBytes(pool.storedBytes)}
                                </dd>
                              </div>
                              <div>
                                <dt class="text-[10px] font-medium uppercase tracking-wide text-muted">
                                  Available
                                </dt>
                                <dd class="font-mono text-base-content">
                                  {formatBytes(pool.availableBytes)}
                                </dd>
                              </div>
                              <div>
                                <dt class="text-[10px] font-medium uppercase tracking-wide text-muted">
                                  Used
                                </dt>
                                <dd class="font-mono text-base-content">
                                  {formatPlatformTablePercentValue(pool.percentUsed)}
                                </dd>
                              </div>
                            </dl>
                          </InlineDetailTableRow>
                        </Show>
                      </>
                    );
                  }}
                </For>
              </PlatformDetailTableBody>
            </PlatformDetailTable>
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
            <PlatformDetailTable class="min-w-0 table-fixed text-xs">
              <PlatformDetailTableHeader>
                <TableHead class={getPlatformTableHeadClassForKind('name')}>Service</TableHead>
                <TableHead class={getPlatformTableHeadClassForKind('text')}>Status</TableHead>
                <TableHead class={getPlatformTableHeadClassForKind('numeric-value')}>
                  <PlatformResponsiveTableLabel compact="Up" full="Running" />
                </TableHead>
                <TableHead class={getPlatformTableHeadClassForKind('numeric-value')}>
                  Total
                </TableHead>
              </PlatformDetailTableHeader>
              <PlatformDetailTableBody>
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
              </PlatformDetailTableBody>
            </PlatformDetailTable>
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
    </section>
  );
};

export default ProxmoxCephClusterDrawer;
