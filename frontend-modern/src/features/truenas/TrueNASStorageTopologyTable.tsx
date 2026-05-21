import { For, Show, createMemo, type Component, type JSX } from 'solid-js';
import { StatusDot } from '@/components/shared/StatusDot';
import { TableCard } from '@/components/shared/TableCard';
import { TableCardHeader } from '@/components/shared/TableCardHeader';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/shared/Table';
import { formatBytes, formatPercent } from '@/utils/format';
import { getSimpleStatusIndicator } from '@/utils/status';
import { asTrimmedString } from '@/utils/stringUtils';
import {
  PLATFORM_TABLE_BODY_CLASS,
  PLATFORM_TABLE_CARD_CLASS,
  PLATFORM_TABLE_HEADER_ROW_CLASS,
  PlatformTableEmptyState,
  PlatformTableToolbar,
  createPlatformTableFilterState,
  getPlatformTableCellClassForKind,
  getPlatformTableHeadClassForKind,
  type PlatformTableFilterOption,
} from '@/features/platformPage/sharedPlatformPage';
import {
  PlatformResourceDetailTableRow,
  createPlatformResourceDetailState,
  createPlatformResourceLabelResolver,
  getPlatformResourceDetailRowClass,
} from '@/features/platformPage/PlatformResourceDetailTableRow';
import type { Resource } from '@/types/resource';
import {
  buildTrueNASStorageTopologyRows,
  filterTrueNASStorageTopologyRows,
  mapTrueNASStorageStatus,
  type TrueNASStorageStatusFilter,
  type TrueNASStorageTopologyKind,
  type TrueNASStorageTopologyRow,
} from './truenasPageModel';

const TRUENAS_STORAGE_STATUS_OPTIONS: PlatformTableFilterOption<TrueNASStorageStatusFilter>[] = [
  { value: 'all', label: 'All' },
  { value: 'healthy', label: 'Healthy', tone: 'success' },
  { value: 'attention', label: 'Attention', tone: 'warning' },
  { value: 'offline', label: 'Offline', tone: 'danger' },
];

const titleCase = (value: string | undefined): string => {
  const normalized = asTrimmedString(value);
  if (!normalized) return 'Unknown';
  return normalized.charAt(0).toUpperCase() + normalized.slice(1).toLowerCase();
};

const resourceName = (resource: Resource): string =>
  asTrimmedString(resource.displayName) || asTrimmedString(resource.name) || resource.id;

const kindLabel = (kind: TrueNASStorageTopologyKind): string => {
  if (kind === 'pool') return 'Pool';
  if (kind === 'dataset') return 'Dataset';
  return 'Disk';
};

const stateLabel = (row: TrueNASStorageTopologyRow): string => {
  if (row.kind === 'pool') {
    return (
      asTrimmedString(row.resource.storage?.zfsPoolState)?.toUpperCase() ||
      titleCase(row.resource.status)
    );
  }
  if (row.kind === 'disk') {
    return (
      asTrimmedString(row.resource.physicalDisk?.health)?.toUpperCase() ||
      titleCase(row.resource.status)
    );
  }
  return titleCase(row.resource.status);
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

const capacityLabel = (row: TrueNASStorageTopologyRow): string => {
  if (row.kind === 'disk') {
    const size = row.resource.physicalDisk?.sizeBytes;
    return typeof size === 'number' && size > 0 ? formatBytes(size) : '-';
  }
  if (typeof row.resource.disk?.used === 'number' && typeof row.resource.disk?.total === 'number') {
    return `${formatBytes(row.resource.disk.used)} / ${formatBytes(row.resource.disk.total)}`;
  }
  const percent = capacityPercent(row.resource);
  return percent === undefined ? '-' : formatPercent(percent);
};

const CapacityCell: Component<{ row: TrueNASStorageTopologyRow }> = (props) => {
  const percent = () =>
    props.row.kind === 'disk' ? undefined : capacityPercent(props.row.resource);
  return (
    <div class="min-w-0">
      <div class="truncate text-base-content" title={capacityLabel(props.row)}>
        {capacityLabel(props.row)}
      </div>
      <Show when={percent() !== undefined}>
        <div class="mt-1 h-1.5 overflow-hidden rounded bg-surface-alt">
          <div
            class="h-full rounded bg-emerald-500"
            style={{ width: `${Math.max(0, Math.min(100, percent() ?? 0))}%` }}
          />
        </div>
      </Show>
    </div>
  );
};

const riskLabel = (row: TrueNASStorageTopologyRow): string => {
  const risk =
    asTrimmedString(row.resource.storage?.risk?.level) ||
    asTrimmedString(row.resource.physicalDisk?.risk?.level);
  if (risk) return titleCase(risk);
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

const temperatureLabel = (row: TrueNASStorageTopologyRow): string => {
  if (row.kind !== 'disk') return '-';
  const value = row.resource.physicalDisk?.temperature;
  return typeof value === 'number' && Number.isFinite(value) ? `${Math.round(value)}C` : '-';
};

const diskCountLabel = (row: TrueNASStorageTopologyRow): string => {
  if (row.kind === 'pool') return String(row.counts.disks);
  return '-';
};

const shareCountLabel = (row: TrueNASStorageTopologyRow): string => {
  if (row.kind === 'disk') return '-';
  return String(row.counts.shares);
};

const rowIndentClass = (depth: number): string => (depth > 0 ? 'pl-5 sm:pl-7' : '');

const ResourceCell: Component<{ row: TrueNASStorageTopologyRow }> = (props) => {
  const indicator = () => getSimpleStatusIndicator(props.row.resource.status);
  const name = () => resourceName(props.row.resource);
  return (
    <div class={`flex min-w-0 items-center gap-2 ${rowIndentClass(props.row.depth)}`}>
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
          <TableCard class={PLATFORM_TABLE_CARD_CLASS}>
            <TableCardHeader title="Storage Topology" />
            <Table class="min-w-full table-fixed text-xs md:min-w-[960px]">
              <TableHeader>
                <TableRow class={PLATFORM_TABLE_HEADER_ROW_CLASS}>
                  <TableHead class={`${getPlatformTableHeadClassForKind('name')} md:w-[27%]`}>
                    Resource
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClassForKind('text')} md:w-[9%]`}>
                    Kind
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('text')} hidden sm:table-cell md:w-[10%]`}
                  >
                    State
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClassForKind('metric-bar')} md:w-[22%]`}>
                    Usage / Size
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('numeric-value')} hidden md:table-cell md:w-[8%]`}
                  >
                    Shares
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('numeric-value')} hidden md:table-cell md:w-[8%]`}
                  >
                    Disks
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('numeric-value')} hidden lg:table-cell md:w-[6%]`}
                  >
                    Temp
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClassForKind('badge')} md:w-[10%]`}>
                    Health
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody class={PLATFORM_TABLE_BODY_CLASS}>
                <For each={tableState.filtered()}>
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
                          onClick={() => drawer.toggle(resource())}
                          onKeyDown={drawer.handleActivationKey(resource())}
                          tabIndex={0}
                        >
                          <TableCell class={getPlatformTableCellClassForKind('name')}>
                            <ResourceCell row={row} />
                          </TableCell>
                          <TableCell class={getPlatformTableCellClassForKind('text')}>
                            <span class="text-base-content">{kindLabel(row.kind)}</span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content sm:table-cell`}
                          >
                            <span class="font-mono text-[11px]">{stateLabel(row)}</span>
                          </TableCell>
                          <TableCell class={getPlatformTableCellClassForKind('metric-bar')}>
                            <CapacityCell row={row} />
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} hidden text-base-content tabular-nums md:table-cell`}
                          >
                            {shareCountLabel(row)}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} hidden text-base-content tabular-nums md:table-cell`}
                          >
                            {diskCountLabel(row)}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} hidden text-base-content tabular-nums lg:table-cell`}
                          >
                            {temperatureLabel(row)}
                          </TableCell>
                          <TableCell class={getPlatformTableCellClassForKind('badge')}>
                            <RiskPill row={row} />
                          </TableCell>
                        </TableRow>
                        <PlatformResourceDetailTableRow
                          resource={resource()}
                          open={isExpanded()}
                          detailRowId={detailRowId()}
                          colSpan={8}
                          resolveResourceLabel={resolveResourceLabel}
                          onClose={() => drawer.close(resource())}
                        />
                      </>
                    );
                  }}
                </For>
              </TableBody>
            </Table>
          </TableCard>
        </Show>
      </div>
    </Show>
  );
};

export default TrueNASStorageTopologyTable;
