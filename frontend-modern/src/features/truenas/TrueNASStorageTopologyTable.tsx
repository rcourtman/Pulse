import { For, Show, createMemo, type Component, type JSX } from 'solid-js';
import { StatusDot } from '@/components/shared/StatusDot';
import { ResponsiveMetricCell } from '@/components/shared/responsive';
import { TableCell, TableRow } from '@/components/shared/Table';
import { filterChipStatusDot } from '@/components/shared/FilterBar';
import { getSimpleStatusIndicator } from '@/utils/status';
import { asTrimmedString } from '@/utils/stringUtils';
import { buildMetricKeyForUnifiedResource } from '@/utils/metricsKeys';
import {
  PlatformSortableTableHead,
  PlatformTableEmptyState,
  PlatformTableNumberValue,
  PlatformTableTemperatureValue,
  PlatformTableToolbar,
  createPlatformTableFilterState,
  createPlatformTableSortState,
  formatPlatformTableBytesValue,
  formatPlatformTableTitleCaseValue,
  getPlatformTableCellClassForKind,
  type PlatformTableFilterOption,
  type PlatformTableSortState,
  type PlatformTableSortValue,
  PlatformTableShell,
} from '@/features/platformPage/sharedPlatformPage';
import {
  PlatformResourceDetailToggleButton,
  PlatformResourceDetailTableRow,
  createPlatformResourceDetailState,
  createPlatformResourceLabelResolver,
  getPlatformResourceDetailRowClass,
} from '@/features/platformPage/PlatformResourceDetailTableRow';
import type { Resource } from '@/types/resource';
import {
  buildTrueNASStorageTopologyRows,
  filterTrueNASStorageTopologyRows,
  getTrueNASResourceDisplayStatus,
  mapTrueNASStorageStatus,
  type TrueNASStorageStatusFilter,
  type TrueNASStorageTopologyKind,
  type TrueNASStorageTopologyRow,
} from './truenasPageModel';

const TRUENAS_STORAGE_STATUS_OPTIONS: PlatformTableFilterOption<TrueNASStorageStatusFilter>[] = [
  { value: 'all', label: 'All' },
  {
    value: 'healthy',
    label: 'Healthy',
    tone: 'success',
    leading: filterChipStatusDot('bg-emerald-500'),
  },
  {
    value: 'attention',
    label: 'Attention',
    tone: 'warning',
    leading: filterChipStatusDot('bg-amber-500'),
  },
  {
    value: 'offline',
    label: 'Offline',
    tone: 'danger',
    leading: filterChipStatusDot('bg-red-500'),
  },
];

const resourceName = (resource: Resource): string =>
  asTrimmedString(resource.displayName) || asTrimmedString(resource.name) || resource.id;

const kindLabel = (kind: TrueNASStorageTopologyKind): string => {
  if (kind === 'pool') return 'Pool';
  if (kind === 'dataset') return 'Dataset';
  return 'Disk';
};

const subtitle = (row: TrueNASStorageTopologyRow): string => {
  if (row.kind === 'pool') return row.resource.parentName || 'TrueNAS pool';
  if (row.kind === 'dataset') return row.resource.parentName || 'Dataset';
  return row.resource.physicalDisk?.serial || row.resource.physicalDisk?.devPath || 'Physical disk';
};

const capacityPercent = (resource: Resource): number | undefined => {
  if (typeof resource.disk?.current === 'number' && Number.isFinite(resource.disk.current)) {
    return resource.disk.current;
  }
  if (
    typeof resource.disk?.used === 'number' &&
    typeof resource.disk?.total === 'number' &&
    resource.disk.total > 0
  ) {
    return (resource.disk.used / resource.disk.total) * 100;
  }
  return undefined;
};

const diskSizeLabel = (row: TrueNASStorageTopologyRow): string => {
  const size = row.resource.physicalDisk?.sizeBytes;
  return formatPlatformTableBytesValue(size, '-');
};

const capacitySublabel = (row: TrueNASStorageTopologyRow): string | undefined => {
  if (typeof row.resource.disk?.used === 'number' && typeof row.resource.disk?.total === 'number') {
    return `${formatPlatformTableBytesValue(row.resource.disk.used, '-')} / ${formatPlatformTableBytesValue(row.resource.disk.total, '-')}`;
  }
  return undefined;
};

const CapacityCell: Component<{ row: TrueNASStorageTopologyRow }> = (props) => {
  if (props.row.kind === 'disk') {
    return (
      <span class="text-base-content tabular-nums" title={diskSizeLabel(props.row)}>
        {diskSizeLabel(props.row)}
      </span>
    );
  }
  const percent = () => capacityPercent(props.row.resource);
  const sublabel = () => capacitySublabel(props.row);
  const metricsKey = () => buildMetricKeyForUnifiedResource(props.row.resource);
  return (
    <ResponsiveMetricCell
      class="w-full"
      value={percent() ?? 0}
      type="disk"
      sublabel={sublabel()}
      resourceId={metricsKey()}
      isRunning={percent() !== undefined}
      showMobile={false}
    />
  );
};

const riskLabel = (row: TrueNASStorageTopologyRow): string => {
  const risk =
    asTrimmedString(row.resource.storage?.risk?.level) ||
    asTrimmedString(row.resource.physicalDisk?.risk?.level);
  if (risk) return formatPlatformTableTitleCaseValue(risk);
  const incidentCount = row.resource.incidentCount ?? row.resource.incidents?.length ?? 0;
  if (incidentCount > 0) return `${incidentCount} alert${incidentCount === 1 ? '' : 's'}`;
  const mapped = mapTrueNASStorageStatus(row.resource);
  if (mapped === 'attention') return 'Attention';
  if (mapped === 'offline') return 'Offline';
  if (mapped === 'healthy') return 'Healthy';
  return 'Unknown';
};

const riskPillClass = (row: TrueNASStorageTopologyRow): string => {
  const mapped = mapTrueNASStorageStatus(row.resource);
  if (mapped === 'attention') {
    return 'border-amber-300/50 bg-amber-500/10 text-amber-700 dark:text-amber-300';
  }
  if (mapped === 'offline') {
    return 'border-red-300/50 bg-red-500/10 text-red-700 dark:text-red-300';
  }
  if (mapped === 'healthy') {
    return 'border-emerald-300/50 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300';
  }
  return 'border-border bg-surface-alt text-muted';
};

const RiskPill: Component<{ row: TrueNASStorageTopologyRow }> = (props) => (
  <span
    class={`inline-flex items-center rounded border px-1.5 py-0.5 text-[10px] font-medium ${riskPillClass(
      props.row,
    )}`}
  >
    {riskLabel(props.row)}
  </span>
);

// Columns a user can sort by. Usage / Size orders pools and datasets on their
// used percentage and disks on their raw size, so same-kind siblings compare
// on the number their cell actually shows.
const TRUENAS_STORAGE_SORT_KEYS = ['resource', 'kind', 'usage', 'disks', 'temp', 'health'] as const;

type TrueNASStorageSortKey = (typeof TRUENAS_STORAGE_SORT_KEYS)[number];

const getTrueNASStorageSortValue = (
  row: TrueNASStorageTopologyRow,
  key: TrueNASStorageSortKey,
): PlatformTableSortValue => {
  switch (key) {
    case 'resource':
      return resourceName(row.resource);
    case 'kind':
      return kindLabel(row.kind);
    case 'usage': {
      if (row.kind === 'disk') {
        const size = row.resource.physicalDisk?.sizeBytes;
        return typeof size === 'number' && Number.isFinite(size) ? size : null;
      }
      return capacityPercent(row.resource) ?? null;
    }
    case 'disks':
      return row.kind === 'pool' ? row.counts.disks : null;
    case 'temp': {
      if (row.kind !== 'disk') return null;
      const temperature = row.resource.physicalDisk?.temperature;
      return typeof temperature === 'number' && Number.isFinite(temperature) ? temperature : null;
    }
    case 'health':
      return riskLabel(row);
    default:
      key satisfies never;
      return null;
  }
};

// The topology renders as a flattened tree (pool → datasets → disks with
// indentation), so a flat re-order would tear children away from their
// parents. Sort each sibling group instead and re-emit depth-first: pools
// re-order against pools, and every parent keeps its subtree directly below
// it. Rows whose parent was filtered out re-root, matching how the filter
// already presents them.
const sortTrueNASStorageTopologyRows = (
  rows: readonly TrueNASStorageTopologyRow[],
  sort: PlatformTableSortState<TrueNASStorageSortKey>,
): readonly TrueNASStorageTopologyRow[] => {
  if (!sort.sortKey()) return rows;
  const present = new Set(rows.map((row) => row.id));
  const roots: TrueNASStorageTopologyRow[] = [];
  const childrenByParent = new Map<string, TrueNASStorageTopologyRow[]>();
  for (const row of rows) {
    const parentRowId = row.parentRowId && present.has(row.parentRowId) ? row.parentRowId : '';
    if (!parentRowId) {
      roots.push(row);
      continue;
    }
    const siblings = childrenByParent.get(parentRowId) ?? [];
    siblings.push(row);
    childrenByParent.set(parentRowId, siblings);
  }
  const ordered: TrueNASStorageTopologyRow[] = [];
  const emit = (siblings: TrueNASStorageTopologyRow[]) => {
    for (const row of sort.sortRows(siblings, getTrueNASStorageSortValue)) {
      ordered.push(row);
      const children = childrenByParent.get(row.id);
      if (children) emit(children);
    }
  };
  emit(roots);
  return ordered;
};

export const getTrueNASStorageTopologyIndentClass = (depth: number): string => {
  if (depth <= 0) return '';
  if (depth === 1) return 'pl-5 sm:pl-7';
  if (depth === 2) return 'pl-9 sm:pl-11';
  return 'pl-12 sm:pl-16';
};

const ResourceCell: Component<{ row: TrueNASStorageTopologyRow; detailToggle?: JSX.Element }> = (
  props,
) => {
  const displayStatus = () => getTrueNASResourceDisplayStatus(props.row.resource);
  const indicator = () => getSimpleStatusIndicator(displayStatus());
  const name = () => resourceName(props.row.resource);
  return (
    <div
      class={`flex min-w-0 items-center gap-2 ${getTrueNASStorageTopologyIndentClass(
        props.row.depth,
      )}`}
      data-truenas-storage-indent-depth={props.row.depth}
    >
      {props.detailToggle}
      <StatusDot size="sm" variant={indicator().variant} title={indicator().label} />
      <div class="min-w-0">
        <div class="truncate font-medium text-base-content" title={name()}>
          {name()}
        </div>
        <div class="truncate text-[10px] text-muted" title={subtitle(props.row)}>
          {subtitle(props.row)}
        </div>
      </div>
    </div>
  );
};

export const TrueNASStorageTopologyTable: Component<{
  resources: Resource[];
  scope: Resource[];
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
  showToolbar?: boolean;
}> = (props) => {
  const rows = createMemo(() => buildTrueNASStorageTopologyRows(props.resources));
  const tableState = createPlatformTableFilterState({
    resources: rows,
    initialStatus: 'all' as TrueNASStorageStatusFilter,
    filter: filterTrueNASStorageTopologyRows,
  });
  const drawer = createPlatformResourceDetailState({ idPrefix: 'truenas-storage-drawer' });
  const resolveResourceLabel = createPlatformResourceLabelResolver(() => props.scope);
  const sort = createPlatformTableSortState({
    storageKey: 'truenasStorage',
    sortKeys: TRUENAS_STORAGE_SORT_KEYS,
    descendingFirst: ['usage', 'disks', 'temp'],
  });
  const sortedRows = createMemo(() => sortTrueNASStorageTopologyRows(tableState.filtered(), sort));

  return (
    <Show
      when={rows().length > 0}
      fallback={
        <PlatformTableEmptyState
          icon={props.emptyIcon}
          title={props.emptyTitle}
          description={props.emptyDescription}
        />
      }
    >
      <div class="space-y-3">
        <Show when={props.showToolbar !== false}>
          <PlatformTableToolbar
            search={tableState.search}
            onSearchChange={tableState.setSearch}
            searchPlaceholder="Search TrueNAS pools, datasets, or disks"
            status={tableState.status()}
            onStatusChange={tableState.setStatus}
            statusOptions={TRUENAS_STORAGE_STATUS_OPTIONS}
            visible={tableState.visible()}
            total={tableState.total()}
            rowNoun="items"
          />
        </Show>

        <Show
          when={tableState.filtered().length > 0}
          fallback={
            <PlatformTableEmptyState
              icon={props.emptyIcon}
              title="No storage items match current filters"
              description="Adjust the search or status filter to see more TrueNAS storage."
            />
          }
        >
          <PlatformTableShell
            title="Storage"
            tableClass="min-w-full table-fixed text-xs md:min-w-[960px]"
            header={
              <>
                <PlatformSortableTableHead
                  kind="name"
                  sort={sort}
                  sortKey="resource"
                  class="md:w-[32%]"
                >
                  Resource
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  sortKey="kind"
                  class="md:w-[10%]"
                >
                  Kind
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="metric-bar"
                  sort={sort}
                  sortKey="usage"
                  class="md:w-[28%]"
                >
                  Usage / Size
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="numeric-value"
                  sort={sort}
                  sortKey="disks"
                  class="hidden md:table-cell md:w-[8%]"
                >
                  Disks
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="numeric-value"
                  sort={sort}
                  sortKey="temp"
                  class="hidden lg:table-cell md:w-[8%]"
                >
                  Temp
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="badge"
                  sort={sort}
                  sortKey="health"
                  class="md:w-[14%]"
                >
                  Health
                </PlatformSortableTableHead>
              </>
            }
            body={
              <>
                <For each={sortedRows()}>
                  {(row) => {
                    const resource = () => row.resource;
                    const detailRowId = () => drawer.detailRowId(resource());
                    const isExpanded = () => drawer.isExpanded(resource());
                    return (
                      <>
                        <TableRow
                          class={`${getPlatformResourceDetailRowClass(isExpanded())} text-[11px] sm:text-xs`}
                          aria-controls={isExpanded() ? detailRowId() : undefined}
                          aria-expanded={isExpanded() ? 'true' : 'false'}
                          data-truenas-storage-row={row.id}
                          data-truenas-storage-kind={row.kind}
                          data-truenas-storage-resource={resource().id}
                          data-truenas-storage-depth={row.depth}
                          onClick={() => drawer.toggle(resource())}
                          onKeyDown={drawer.handleActivationKey(resource())}
                          tabIndex={0}
                        >
                          <TableCell class={getPlatformTableCellClassForKind('name')}>
                            <ResourceCell
                              row={row}
                              detailToggle={
                                <PlatformResourceDetailToggleButton
                                  expanded={isExpanded()}
                                  resourceLabel={resourceName(resource())}
                                  controlsId={detailRowId()}
                                  onToggle={() => drawer.toggle(resource())}
                                />
                              }
                            />
                          </TableCell>
                          <TableCell class={getPlatformTableCellClassForKind('text')}>
                            <span class="text-base-content">{kindLabel(row.kind)}</span>
                          </TableCell>
                          <TableCell class={getPlatformTableCellClassForKind('metric-bar')}>
                            <CapacityCell row={row} />
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} hidden text-base-content md:table-cell`}
                          >
                            <PlatformTableNumberValue
                              value={row.kind === 'pool' ? row.counts.disks : undefined}
                              emptyText="-"
                            />
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} hidden text-base-content lg:table-cell`}
                          >
                            <PlatformTableTemperatureValue
                              value={
                                row.kind === 'disk'
                                  ? row.resource.physicalDisk?.temperature
                                  : undefined
                              }
                              emptyText="-"
                            />
                          </TableCell>
                          <TableCell class={getPlatformTableCellClassForKind('badge')}>
                            <RiskPill row={row} />
                          </TableCell>
                        </TableRow>
                        <PlatformResourceDetailTableRow
                          resource={resource()}
                          open={isExpanded()}
                          detailRowId={detailRowId()}
                          colSpan={6}
                          resolveResourceLabel={resolveResourceLabel}
                          onClose={() => drawer.close(resource())}
                        />
                      </>
                    );
                  }}
                </For>
              </>
            }
          />
        </Show>
      </div>
    </Show>
  );
};

export default TrueNASStorageTopologyTable;
