import {
  For,
  Show,
  createMemo,
  createSignal,
  type Component,
  type JSX,
} from 'solid-js';
import { Card } from '@/components/shared/Card';
import { EmptyState } from '@/components/shared/EmptyState';
import { FilterButtonGroup, type FilterOption } from '@/components/shared/FilterButtonGroup';
import { SearchInput } from '@/components/shared/SearchInput';
import { StatusDot } from '@/components/shared/StatusDot';
import type { StatusIndicatorVariant } from '@/utils/status';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/shared/Table';
import { formatBytes } from '@/utils/format';
import { asTrimmedString } from '@/utils/stringUtils';
import type { Resource, ResourceCephServiceMeta } from '@/types/resource';

// Ceph clusters are first-class Resources (type='ceph') with structured
// metadata: pools, monitors, managers, OSDs, placement groups, health.
// The previous /proxmox/ceph view shoehorned this into the generic
// Storage component with `forcedView="pools"` + a Proxmox source
// filter, which (a) showed the Storage page's Pools/Physical-disks
// sub-tabs that get reset on every click by the forcedView effect and
// (b) collapsed Ceph's topology back into generic storage rows. This
// bespoke table renders one row per cluster with the operational facts
// a Proxmox operator looks at: health, MON/MGR quorum, OSD up/in,
// placement groups, pool count and capacity utilisation.

type CephStatusFilter = 'all' | 'healthy' | 'warning' | 'critical';

const STATUS_FILTER_OPTIONS: FilterOption<CephStatusFilter>[] = [
  { value: 'all', label: 'All' },
  { value: 'healthy', label: 'Healthy' },
  { value: 'warning', label: 'Warning' },
  { value: 'critical', label: 'Critical' },
];

function classify(resource: Resource): CephStatusFilter {
  const raw = (resource.ceph?.healthStatus ?? resource.status ?? '').toUpperCase();
  if (raw === 'HEALTH_OK' || raw === 'OK' || raw === 'ONLINE') return 'healthy';
  if (raw === 'HEALTH_ERR' || raw === 'ERROR' || raw === 'CRITICAL' || raw === 'OFFLINE') {
    return 'critical';
  }
  if (raw === 'HEALTH_WARN' || raw === 'WARN' || raw === 'WARNING' || raw === 'DEGRADED') {
    return 'warning';
  }
  return 'healthy';
}

function indicatorFor(category: CephStatusFilter): {
  variant: StatusIndicatorVariant;
  label: string;
  tone: string;
} {
  switch (category) {
    case 'healthy':
      return { variant: 'success', label: 'Healthy', tone: 'text-emerald-600 dark:text-emerald-300' };
    case 'warning':
      return { variant: 'warning', label: 'Warning', tone: 'text-amber-600 dark:text-amber-300' };
    case 'critical':
      return { variant: 'danger', label: 'Critical', tone: 'text-red-600 dark:text-red-300' };
    default:
      return { variant: 'muted', label: '—', tone: 'text-muted' };
  }
}

function summarizeServices(services: ResourceCephServiceMeta[] | undefined): string {
  if (!services || services.length === 0) return '—';
  return services
    .map((svc) => `${svc.type}:${svc.running}/${svc.total}`)
    .join(' · ');
}

function poolsLabel(resource: Resource): JSX.Element {
  const pools = resource.ceph?.pools ?? [];
  if (pools.length === 0) return <span class="text-muted">—</span>;
  const stored = pools.reduce((sum, p) => sum + (p.storedBytes ?? 0), 0);
  return (
    <span class="tabular-nums">
      {pools.length}
      <span class="text-muted text-[10px]"> · {formatBytes(stored)} stored</span>
    </span>
  );
}

function osdLabel(resource: Resource): JSX.Element {
  const meta = resource.ceph;
  if (!meta) return <span class="text-muted">—</span>;
  const total = meta.numOsds ?? 0;
  if (total === 0) return <span class="text-muted">—</span>;
  const up = meta.numOsdsUp ?? 0;
  const inService = meta.numOsdsIn ?? 0;
  const allUp = up === total && inService === total;
  return (
    <span class={allUp ? 'tabular-nums' : 'tabular-nums text-amber-600 dark:text-amber-300 font-semibold'}>
      {up}/{total}
      <span class="text-muted text-[10px]"> up · {inService} in</span>
    </span>
  );
}

function quorumLabel(meta: { numMons: number; numMgrs: number } | undefined): JSX.Element {
  if (!meta) return <span class="text-muted">—</span>;
  return (
    <span class="tabular-nums">
      MON {meta.numMons}
      <span class="text-muted"> · </span>
      MGR {meta.numMgrs}
    </span>
  );
}

function capacityLabel(resource: Resource): JSX.Element {
  const pct = resource.disk?.current;
  if (typeof pct !== 'number') return <span class="text-muted">—</span>;
  const total = resource.disk?.total;
  if (typeof total === 'number' && total > 0) {
    return (
      <span class="tabular-nums">
        {pct.toFixed(1)}%
        <span class="text-muted text-[10px]"> of {formatBytes(total)}</span>
      </span>
    );
  }
  return <span class="tabular-nums">{pct.toFixed(1)}%</span>;
}

function healthMessageCell(resource: Resource): JSX.Element {
  const msg = asTrimmedString(resource.ceph?.healthMessage);
  if (!msg) return <span class="text-muted">—</span>;
  return (
    <span class="inline-block max-w-[20rem] truncate" title={msg}>
      {msg}
    </span>
  );
}

export const ProxmoxCephTable: Component<{
  resources: Resource[];
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
}> = (props) => {
  const [search, setSearch] = createSignal('');
  const [status, setStatus] = createSignal<CephStatusFilter>('all');

  const filtered = createMemo(() => {
    const term = search().trim().toLowerCase();
    const want = status();
    return props.resources.filter((cluster) => {
      if (want !== 'all' && classify(cluster) !== want) return false;
      if (!term) return true;
      const haystack = [
        cluster.name,
        cluster.displayName,
        cluster.ceph?.fsid,
        cluster.ceph?.healthMessage,
        cluster.platformId,
        ...(cluster.ceph?.pools?.map((p) => p.name) ?? []),
      ]
        .filter(Boolean)
        .join(' ')
        .toLowerCase();
      return haystack.includes(term);
    });
  });

  const total = createMemo(() => props.resources.length);
  const visible = createMemo(() => filtered().length);

  return (
    <Show
      when={total() > 0}
      fallback={
        <Card padding="lg">
          <EmptyState
            icon={props.emptyIcon}
            title={props.emptyTitle}
            description={props.emptyDescription}
          />
        </Card>
      }
    >
      <div class="space-y-3">
        <div class="flex flex-wrap items-center gap-2">
          <div class="min-w-[200px] flex-1 sm:max-w-xs">
            <SearchInput
              value={search}
              onChange={setSearch}
              placeholder="Search clusters, pools, FSID"
            />
          </div>
          <FilterButtonGroup options={STATUS_FILTER_OPTIONS} value={status()} onChange={setStatus} />
          <span class="ml-auto whitespace-nowrap text-xs font-medium text-muted">
            <Show when={visible() !== total()} fallback={<>{total()} clusters</>}>
              {visible()} of {total()} clusters
            </Show>
          </span>
        </div>

        <Show
          when={filtered().length > 0}
          fallback={
            <Card padding="lg">
              <EmptyState
                icon={props.emptyIcon}
                title="No clusters match current filters"
                description="Adjust the search or status filter to see more clusters."
              />
            </Card>
          }
        >
          <Card padding="none" tone="card" class="overflow-hidden">
            <Table class="w-full min-w-[1100px] border-collapse text-xs">
              <TableHeader class="bg-surface-alt text-muted border-b border-border">
                <TableRow class="text-left text-[10px] uppercase tracking-wide">
                  <TableHead class="px-3 py-2 font-medium">Cluster</TableHead>
                  <TableHead class="px-3 py-2 font-medium">Health</TableHead>
                  <TableHead class="px-3 py-2 font-medium">FSID</TableHead>
                  <TableHead class="px-3 py-2 font-medium">Quorum</TableHead>
                  <TableHead class="px-3 py-2 font-medium">OSDs</TableHead>
                  <TableHead class="px-3 py-2 font-medium text-right">PGs</TableHead>
                  <TableHead class="px-3 py-2 font-medium">Pools</TableHead>
                  <TableHead class="px-3 py-2 font-medium">Capacity</TableHead>
                  <TableHead class="px-3 py-2 font-medium">Services</TableHead>
                  <TableHead class="px-3 py-2 font-medium">Detail</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody class="divide-y divide-border-subtle">
                <For each={filtered()}>
                  {(cluster) => {
                    const ind = indicatorFor(classify(cluster));
                    const name = asTrimmedString(cluster.name) || cluster.id;
                    const fsid = asTrimmedString(cluster.ceph?.fsid) || '—';
                    return (
                      <TableRow class="hover:bg-surface-hover">
                        <TableCell class="px-3 py-2">
                          <div class="flex items-center gap-2 min-w-0">
                            <span class="font-semibold text-base-content truncate" title={name}>
                              {name}
                            </span>
                          </div>
                          <Show when={cluster.platformId}>
                            <div class="text-[10px] text-muted font-mono truncate" title={cluster.platformId}>
                              {cluster.platformId}
                            </div>
                          </Show>
                        </TableCell>
                        <TableCell class="px-3 py-2">
                          <div class="flex items-center gap-2">
                            <StatusDot size="sm" variant={ind.variant} title={ind.label} ariaHidden />
                            <span class={`text-[11px] font-medium ${ind.tone}`}>{ind.label}</span>
                          </div>
                          <Show when={!!cluster.ceph?.healthStatus}>
                            <div class="text-[10px] text-muted font-mono">{cluster.ceph?.healthStatus}</div>
                          </Show>
                        </TableCell>
                        <TableCell class="px-3 py-2 text-base-content font-mono text-[11px]">
                          <span class="inline-block max-w-[10rem] truncate" title={fsid}>
                            {fsid}
                          </span>
                        </TableCell>
                        <TableCell class="px-3 py-2 text-base-content">{quorumLabel(cluster.ceph)}</TableCell>
                        <TableCell class="px-3 py-2 text-base-content">{osdLabel(cluster)}</TableCell>
                        <TableCell class="px-3 py-2 text-right text-base-content tabular-nums">
                          <Show when={(cluster.ceph?.numPGs ?? 0) > 0} fallback={<span class="text-muted">—</span>}>
                            {cluster.ceph?.numPGs}
                          </Show>
                        </TableCell>
                        <TableCell class="px-3 py-2 text-base-content">{poolsLabel(cluster)}</TableCell>
                        <TableCell class="px-3 py-2 text-base-content">{capacityLabel(cluster)}</TableCell>
                        <TableCell class="px-3 py-2 text-base-content font-mono text-[11px]">
                          {summarizeServices(cluster.ceph?.services)}
                        </TableCell>
                        <TableCell class="px-3 py-2 text-base-content">{healthMessageCell(cluster)}</TableCell>
                      </TableRow>
                    );
                  }}
                </For>
              </TableBody>
            </Table>
          </Card>
        </Show>
      </div>
    </Show>
  );
};

export default ProxmoxCephTable;
