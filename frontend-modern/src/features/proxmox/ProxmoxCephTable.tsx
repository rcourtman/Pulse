import {
  For,
  Show,
  createEffect,
  createMemo,
  createSignal,
  type Component,
  type JSX,
} from 'solid-js';
import { filterChipStatusDot } from '@/components/shared/FilterBar';
import { InlineDetailTableRow } from '@/components/shared/InlineDetailTableRow';
import { StatusDot } from '@/components/shared/StatusDot';
import type { StatusIndicatorVariant } from '@/utils/status';
import { TableCell, TableHead, TableRow } from '@/components/shared/Table';
import { asTrimmedString } from '@/utils/stringUtils';
import {
  PlatformTableToolbar,
  formatPlatformTableBytesValue,
  formatPlatformTablePercentValue,
  getPlatformTableCellClassForKind,
  getPlatformTableContainerLayout,
  getPlatformTableHeadClassForKind,
  getPlatformTableWeightedColumnWidthStyle,
  PlatformResponsiveTableLabel,
  type PlatformTableFilterOption,
  PlatformTableEmptyState,
  PlatformTableShell,
  PlatformWindowedRows,
  withPlatformStatusCounts,
} from '@/features/platformPage/sharedPlatformPage';
import {
  getPlatformResourceDetailRowInteractionProps,
  PlatformResourceDetailToggleButton,
} from '@/features/platformPage/PlatformResourceDetailTableRow';
import { useObservedElementWidth } from '@/hooks/useObservedElementWidth';
import type { Resource, ResourceCephServiceMeta } from '@/types/resource';
import { ProxmoxCephClusterDrawer } from './ProxmoxCephClusterDrawer';
import { getCephClusterStaleNotice, type CephStaleNotice } from './proxmoxCephClusterDrawerModel';
import { buildPlatformResourceSearchSuggestions } from '@/features/platformPage/platformSearchSuggestions';
import { matchesSearchTermSplit, splitSearchExclusions } from '@/utils/searchQuery';

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

type CephHealth = 'healthy' | 'warning' | 'critical' | 'unknown';
type CephStatusFilter = 'all' | Exclude<CephHealth, 'unknown'>;

export type CephPhoneColumn = 'cluster' | 'health' | 'osds' | 'pools' | 'capacity';

export type CephColumn = CephPhoneColumn | 'quorum' | 'pgs' | 'services' | 'detail';

// One phone projection for every compact width: five values that each fit
// whole. A sixth column left the cluster name, the health word, and every
// figure clipped ("Heal", "MO…", "15/1…"); the monitor and manager counts it
// carried are itemised in the expansion's Services table.
export const CEPH_PHONE_COLUMNS: readonly CephPhoneColumn[] = [
  'cluster',
  'health',
  'osds',
  'pools',
  'capacity',
];

export const CEPH_PHONE_COLUMN_WIDTHS: Readonly<Record<CephPhoneColumn, number>> = {
  cluster: 36,
  health: 21,
  osds: 13,
  pools: 12,
  capacity: 18,
};

// Above the phone projection every visible column takes a weighted share of
// the row. Sizing only the always-on columns left PGs, Services, and Detail to
// split the remainder, which truncated both their headers and their values on
// a full-width desktop table.
export const CEPH_COLUMN_WEIGHTS: Readonly<Record<CephColumn, number>> = {
  cluster: 20,
  health: 10,
  quorum: 11,
  osds: 12,
  pgs: 6,
  pools: 13,
  capacity: 13,
  services: 24,
  detail: 14,
};

const STATUS_FILTER_OPTIONS: PlatformTableFilterOption<CephStatusFilter>[] = [
  { value: 'all', label: 'All' },
  {
    value: 'healthy',
    label: 'Healthy',
    tone: 'success',
    leading: filterChipStatusDot('bg-emerald-500'),
  },
  {
    value: 'warning',
    label: 'Warning',
    tone: 'warning',
    leading: filterChipStatusDot('bg-amber-500'),
  },
  {
    value: 'critical',
    label: 'Critical',
    tone: 'danger',
    leading: filterChipStatusDot('bg-red-500'),
  },
];

function classifyReported(resource: Resource): CephHealth {
  // Prefer native health when reported; topology and capacity alone say
  // nothing about whether the cluster is healthy.
  const raw = (asTrimmedString(resource.ceph?.healthStatus) || resource.status || '').toUpperCase();
  switch (raw) {
    case 'HEALTH_OK':
    case 'OK':
    case 'ONLINE':
      return 'healthy';
    case 'HEALTH_WARN':
    case 'WARN':
    case 'WARNING':
    case 'DEGRADED':
      return 'warning';
    case 'HEALTH_ERR':
    case 'ERR':
    case 'ERROR':
    case 'CRITICAL':
    case 'OFFLINE':
      return 'critical';
    default:
      return 'unknown';
  }
}

function classify(resource: Resource): CephHealth {
  const reported = classifyReported(resource);
  if (reported === 'critical') return 'critical';
  if (getCephClusterStaleNotice(resource)) return 'warning';
  return reported;
}

const HEALTH_INDICATORS: Record<
  CephHealth,
  { variant: StatusIndicatorVariant; label: string; tone: string }
> = {
  healthy: { variant: 'success', label: 'Healthy', tone: 'text-emerald-600 dark:text-emerald-300' },
  warning: { variant: 'warning', label: 'Warning', tone: 'text-amber-600 dark:text-amber-300' },
  critical: { variant: 'danger', label: 'Critical', tone: 'text-red-600 dark:text-red-300' },
  unknown: { variant: 'muted', label: 'Unknown', tone: 'text-muted' },
};

const STALE_INDICATOR: { variant: StatusIndicatorVariant; label: string; tone: string } = {
  variant: 'warning',
  label: 'Stale',
  tone: 'text-amber-600 dark:text-amber-300',
};

function getClusterIndicator(
  resource: Resource,
  staleNotice: CephStaleNotice | null,
): { variant: StatusIndicatorVariant; label: string; tone: string } {
  const reported = classifyReported(resource);
  if (staleNotice && (reported === 'healthy' || reported === 'unknown')) {
    return STALE_INDICATOR;
  }
  return HEALTH_INDICATORS[reported];
}

function servicesCell(services: ResourceCephServiceMeta[] | undefined): JSX.Element {
  const reported = (services ?? []).filter((svc) => svc.total > 0);
  if (reported.length === 0) return <span class="text-muted">—</span>;
  return (
    <span class="tabular-nums">
      <For each={reported}>
        {(svc, idx) => {
          const degraded = svc.running < svc.total;
          const down = svc.running === 0;
          const label = `${svc.type.toUpperCase()} ${svc.running}/${svc.total}`;
          return (
            <>
              <Show when={idx() > 0}>
                <span class="text-muted"> · </span>
              </Show>
              <span
                class={
                  down
                    ? 'font-semibold text-red-600 dark:text-red-300'
                    : degraded
                      ? 'font-semibold text-amber-600 dark:text-amber-300'
                      : undefined
                }
              >
                {label}
              </span>
            </>
          );
        }}
      </For>
    </span>
  );
}

function poolsLabel(resource: Resource): JSX.Element {
  const pools = resource.ceph?.pools ?? [];
  if (pools.length === 0) return <span class="text-muted">—</span>;
  const stored = pools.reduce((sum, p) => sum + (p.storedBytes ?? 0), 0);
  return (
    <span class="tabular-nums">
      {pools.length}
      <span class="platform-table-phone-hidden-inline text-muted text-[10px]">
        {' · '}
        {formatPlatformTableBytesValue(stored, '0 B')} stored
      </span>
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
    <span
      class={
        allUp ? 'tabular-nums' : 'tabular-nums text-amber-600 dark:text-amber-300 font-semibold'
      }
    >
      {up}/{total}
      <span class="platform-table-phone-hidden-inline text-muted text-[10px]">
        {' '}
        up · {inService} in
      </span>
    </span>
  );
}

function quorumLabel(
  meta:
    | {
        numMons?: number;
        numMgrs?: number;
        services?: ResourceCephServiceMeta[];
      }
    | undefined,
): JSX.Element {
  if (!meta) return <span class="text-muted">—</span>;
  const monSvc = meta.services?.find((svc) => svc.type.toLowerCase() === 'mon' && svc.total > 0);
  if (monSvc) {
    const majority = Math.floor(monSvc.total / 2) + 1;
    const detail = `${monSvc.running} of ${monSvc.total} monitors running (quorum requires ${majority})`;
    if (monSvc.running < majority) {
      return (
        <span class="font-semibold text-red-600 dark:text-red-300" title={detail}>
          No quorum
        </span>
      );
    }
    if (monSvc.running < monSvc.total) {
      return (
        <span class="font-semibold text-amber-600 dark:text-amber-300" title={detail}>
          Degraded
        </span>
      );
    }
    return <span title={detail}>In quorum</span>;
  }
  const mons = meta.numMons ?? 0;
  if (mons <= 0) {
    return (
      <span class="font-semibold text-red-600 dark:text-red-300" title="No Ceph monitors reported">
        No quorum
      </span>
    );
  }
  const mgrs = meta.numMgrs ?? 0;
  return (
    <span
      title={`${mons} ${mons === 1 ? 'monitor' : 'monitors'} · ${mgrs} ${mgrs === 1 ? 'manager' : 'managers'}`}
    >
      In quorum
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
        {formatPlatformTablePercentValue(pct)}
        <span class="platform-table-phone-hidden-inline text-muted text-[10px]">
          {' '}
          of {formatPlatformTableBytesValue(total)}
        </span>
      </span>
    );
  }
  return <span class="tabular-nums">{formatPlatformTablePercentValue(pct)}</span>;
}

function healthMessageCell(
  resource: Resource,
  staleNotice: CephStaleNotice | null,
): JSX.Element {
  const msg = asTrimmedString(resource.ceph?.healthMessage);
  if (msg) {
    return (
      <span class="block truncate" title={msg}>
        {msg}
      </span>
    );
  }
  if (staleNotice) {
    return (
      <span class="block truncate text-amber-700 dark:text-amber-300" title={staleNotice.detail}>
        {staleNotice.detail}
      </span>
    );
  }
  return <span class="text-muted">—</span>;
}

export const ProxmoxCephTable: Component<{
  resources: Resource[];
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
}> = (props) => {
  const [search, setSearch] = createSignal('');
  const [status, setStatus] = createSignal<CephStatusFilter>('all');
  const [selectedId, setSelectedId] = createSignal<string | null>(null);
  // Single-cluster pages have an almost empty surface unless the user
  // expands the only row, so auto-open it on first load. Once the user
  // interacts (toggle or close), we stop overriding their choice.
  const [userPicked, setUserPicked] = createSignal(false);
  createEffect(() => {
    if (userPicked()) return;
    setSelectedId(props.resources.length === 1 ? props.resources[0].id : null);
  });
  const toggleSelected = (id: string) => {
    setUserPicked(true);
    setSelectedId((current) => (current === id ? null : id));
  };
  const closeSelected = () => {
    setUserPicked(true);
    setSelectedId(null);
  };

  const filterByStatus = (want: CephStatusFilter) => {
    const split = splitSearchExclusions(search());
    return props.resources.filter((cluster) => {
      if (want !== 'all' && classify(cluster) !== want) return false;
      if (!split.needle && split.excludes.length === 0) return true;
      const staleNotice = getCephClusterStaleNotice(cluster);
      const haystack = [
        cluster.name,
        cluster.displayName,
        cluster.ceph?.fsid,
        cluster.ceph?.healthMessage,
        cluster.platformId,
        staleNotice?.badge,
        staleNotice?.detail,
        ...(cluster.ceph?.pools?.map((p) => p.name) ?? []),
      ]
        .filter(Boolean)
        .join(' ')
        .toLowerCase();
      return matchesSearchTermSplit(haystack, split);
    });
  };
  const filtered = createMemo(() => filterByStatus(status()));
  const countForStatus = (value: CephStatusFilter): number => filterByStatus(value).length;

  const total = createMemo(() => props.resources.length);
  const visible = createMemo(() => filtered().length);
  const searchSuggestions = createMemo(() =>
    buildPlatformResourceSearchSuggestions(props.resources),
  );
  const observedWidth = useObservedElementWidth();
  const layout = createMemo(() =>
    getPlatformTableContainerLayout(observedWidth.width() ?? 1920, [520, 720, 960, 1200]),
  );
  // Below the operational width the row cannot hold quorum beside the OSD,
  // pool, and capacity figures without clipping all four.
  const showQuorum = createMemo(() => ['operational', 'expanded', 'full'].includes(layout()));
  // The health message says what is wrong, so it outranks the daemon tally,
  // which needs the widest layout to fit and is itemised in the expansion.
  // The FSID is forensic detail: searchable, and shown whole in the expansion.
  const showExpanded = createMemo(() => ['expanded', 'full'].includes(layout()));
  const showServices = createMemo(() => layout() === 'full');
  const visibleColumns = createMemo<readonly CephColumn[]>(() => {
    if (layout() === 'compact') return CEPH_PHONE_COLUMNS;
    const columns: CephColumn[] = ['cluster', 'health'];
    if (showQuorum()) columns.push('quorum');
    columns.push('osds');
    if (showExpanded()) columns.push('pgs');
    columns.push('pools', 'capacity');
    if (showServices()) columns.push('services');
    if (showExpanded()) columns.push('detail');
    return columns;
  });
  const visibleColumnCount = createMemo(() => visibleColumns().length);
  const columnWidthStyle = (column: CephColumn) => {
    if (layout() !== 'compact') {
      return getPlatformTableWeightedColumnWidthStyle(
        column,
        CEPH_COLUMN_WEIGHTS,
        visibleColumns(),
      );
    }
    const phoneWidths: Partial<Record<CephColumn, number>> = CEPH_PHONE_COLUMN_WIDTHS;
    return { width: `${phoneWidths[column] ?? 0}%` };
  };

  return (
    <Show
      when={total() > 0}
      fallback={
        <PlatformTableEmptyState
          icon={props.emptyIcon}
          title={props.emptyTitle}
          description={props.emptyDescription}
        />
      }
    >
      <div ref={observedWidth.setElement} class="space-y-3" data-proxmox-ceph-layout={layout()}>
        <PlatformTableToolbar
          search={search}
          onSearchChange={setSearch}
          searchPlaceholder="Search clusters, pools, FSID"
          searchSuggestions={searchSuggestions}
          status={status()}
          onStatusChange={setStatus}
          statusOptions={withPlatformStatusCounts(STATUS_FILTER_OPTIONS, countForStatus)}
          visible={visible()}
          total={total()}
          rowNoun="clusters"
        />

        <Show
          when={filtered().length > 0}
          fallback={
            <PlatformTableEmptyState
              icon={props.emptyIcon}
              title="No clusters match current filters"
              description="Adjust the search or status filter to see more clusters."
            />
          }
        >
          <PlatformTableShell
            tableClass="min-w-0 table-fixed text-xs"
            colgroup={
              <colgroup>
                <For each={visibleColumns()}>
                  {(column) => (
                    <col style={columnWidthStyle(column)} data-proxmox-ceph-column={column} />
                  )}
                </For>
              </colgroup>
            }
            header={
              <>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('name')} platform-table-mobile-w-30`}
                >
                  Cluster
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('text')} platform-table-mobile-w-15`}
                >
                  Health
                </TableHead>
                <Show when={showQuorum()}>
                  <TableHead class={getPlatformTableHeadClassForKind('text')}>Quorum</TableHead>
                </Show>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('numeric-value')} platform-table-mobile-w-15`}
                >
                  OSDs
                </TableHead>
                <Show when={showExpanded()}>
                  <TableHead class={getPlatformTableHeadClassForKind('numeric-value')}>
                    PGs
                  </TableHead>
                </Show>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('numeric-value')} platform-table-mobile-w-15`}
                >
                  Pools
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('numeric-value')} platform-table-mobile-w-15`}
                >
                  <PlatformResponsiveTableLabel compact="Used" full="Capacity" />
                </TableHead>
                <Show when={showServices()}>
                  <TableHead class={getPlatformTableHeadClassForKind('text')}>Services</TableHead>
                </Show>
                <Show when={showExpanded()}>
                  <TableHead class={getPlatformTableHeadClassForKind('text')}>Detail</TableHead>
                </Show>
              </>
            }
            body={
              <>
                <PlatformWindowedRows items={filtered} estimatedRowHeight={32}>
                  {(cluster) => {
                    // One verdict for the row's dot and the drawer header.
                    const staleNotice = () => getCephClusterStaleNotice(cluster);
                    const ind = () => getClusterIndicator(cluster, staleNotice());
                    const name = asTrimmedString(cluster.name) || cluster.id;
                    const isOpen = () => selectedId() === cluster.id;
                    const detailRowId = () => `proxmox-ceph-detail-${cluster.id}`;
                    return (
                      <>
                        <TableRow
                          {...getPlatformResourceDetailRowInteractionProps({
                            expanded: isOpen(),
                            onToggle: () => toggleSelected(cluster.id),
                          })}
                        >
                          <TableCell
                            class={getPlatformTableCellClassForKind('name')}
                            title={[name, cluster.platformId, staleNotice()?.detail]
                              .filter(Boolean)
                              .join(' · ')}
                          >
                            <div class="flex items-center gap-2 min-w-0">
                              <PlatformResourceDetailToggleButton
                                expanded={isOpen()}
                                resourceLabel={name}
                                controlsId={detailRowId()}
                                onToggle={() => toggleSelected(cluster.id)}
                              />
                              <div class="min-w-0 flex-1">
                                <span
                                  class={`block font-semibold text-base-content ${
                                    layout() === 'compact'
                                      ? 'whitespace-normal wrap-anywhere leading-tight'
                                      : 'truncate'
                                  }`}
                                  title={name}
                                >
                                  {name}
                                </span>
                                <Show when={staleNotice()}>
                                  {(notice) => (
                                    <span
                                      class="block truncate text-[10px] leading-tight font-medium text-amber-700 dark:text-amber-300"
                                      data-proxmox-ceph-stale="cluster"
                                      title={notice().detail}
                                    >
                                      {notice().badge}
                                    </span>
                                  )}
                                </Show>
                              </div>
                            </div>
                          </TableCell>
                          <TableCell
                            class={getPlatformTableCellClassForKind('text')}
                            title={[
                              ind().label,
                              cluster.ceph?.healthStatus
                                ? staleNotice()
                                  ? `Last reported ${cluster.ceph.healthStatus}`
                                  : cluster.ceph.healthStatus
                                : undefined,
                              staleNotice()?.detail,
                            ]
                              .filter(Boolean)
                              .join(' · ')}
                          >
                            <div class="flex items-center gap-2">
                              <StatusDot
                                size="sm"
                                variant={ind().variant}
                                title={ind().label}
                                ariaHidden
                              />
                              <span class={`text-[11px] font-medium ${ind().tone}`}>
                                {ind().label}
                              </span>
                            </div>
                          </TableCell>
                          <Show when={showQuorum()}>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                            >
                              {quorumLabel(cluster.ceph)}
                            </TableCell>
                          </Show>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                          >
                            {osdLabel(cluster)}
                          </TableCell>
                          <Show when={showExpanded()}>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content tabular-nums`}
                            >
                              <Show
                                when={(cluster.ceph?.numPGs ?? 0) > 0}
                                fallback={<span class="text-muted">—</span>}
                              >
                                {cluster.ceph?.numPGs}
                              </Show>
                            </TableCell>
                          </Show>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                          >
                            {poolsLabel(cluster)}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                          >
                            {capacityLabel(cluster)}
                          </TableCell>
                          <Show when={showServices()}>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('text')} text-base-content text-[11px]`}
                            >
                              {servicesCell(cluster.ceph?.services)}
                            </TableCell>
                          </Show>
                          <Show when={showExpanded()}>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                            >
                              {healthMessageCell(cluster, staleNotice())}
                            </TableCell>
                          </Show>
                        </TableRow>
                        <Show when={isOpen()}>
                          <InlineDetailTableRow
                            cellId={detailRowId()}
                            colspan={visibleColumnCount()}
                            data-inline-detail-for={cluster.id}
                          >
                            <ProxmoxCephClusterDrawer
                              cluster={cluster}
                              statusIndicator={ind()}
                              onClose={closeSelected}
                            />
                          </InlineDetailTableRow>
                        </Show>
                      </>
                    );
                  }}
                </PlatformWindowedRows>
              </>
            }
          />
        </Show>
      </div>
    </Show>
  );
};

export default ProxmoxCephTable;
