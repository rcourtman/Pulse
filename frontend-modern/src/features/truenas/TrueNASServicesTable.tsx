import { For, Show, createMemo, type Component, type JSX } from 'solid-js';
import { InlineDetailTableRow } from '@/components/shared/InlineDetailTableRow';
import { StatusDot } from '@/components/shared/StatusDot';
import { TableCell, TableRow } from '@/components/shared/Table';
import { asTrimmedString } from '@/utils/stringUtils';
import {
  PlatformSortableTableHead,
  PlatformTableEmptyState,
  PlatformTableToolbar,
  createPlatformTableFilterState,
  createPlatformTableSortState,
  formatPlatformTableTitleCaseValue,
  getPlatformTableCellClassForKind,
  type PlatformTableFilterOption,
  type PlatformTableSortValue,
  PlatformTableShell,
} from '@/features/platformPage/sharedPlatformPage';
import {
  PlatformResourceDetailToggleButton,
  createPlatformResourceDetailState,
  getPlatformResourceDetailRowClass,
} from '@/features/platformPage/PlatformResourceDetailTableRow';
import {
  filterTrueNASServices,
  mapTrueNASServiceStatus,
  type TrueNASServiceRow,
  type TrueNASServiceStatusFilter,
} from './truenasPageModel';
import {
  InlineDetailPanel,
  compactDetailRows,
  compactDetailSections,
  makeDetailRow,
  type DetailSection,
  type DetailValueTone,
} from '@/components/shared/DetailSectionTable';

const TRUENAS_SERVICE_STATUS_OPTIONS: PlatformTableFilterOption<TrueNASServiceStatusFilter>[] = [
  { value: 'all', label: 'All' },
  { value: 'running', label: 'Running', tone: 'success' },
  { value: 'attention', label: 'Attention', tone: 'warning' },
  { value: 'stopped', label: 'Stopped', tone: 'danger' },
  { value: 'disabled', label: 'Disabled' },
];

const SERVICE_NAME_LABELS: Record<string, string> = {
  ftp: 'FTP',
  nfs: 'NFS',
  rsync: 'Rsync',
  s3: 'S3',
  smartd: 'SMART',
  smb: 'SMB',
  snmp: 'SNMP',
  ssh: 'SSH',
  ups: 'UPS',
};

const formatServiceName = (value: string | undefined): string => {
  const normalized = asTrimmedString(value);
  if (!normalized) return 'Unknown';
  return SERVICE_NAME_LABELS[normalized.toLowerCase()] ?? normalized;
};

const formatPIDs = (pids: number[] | undefined): { label: string; title: string } => {
  const values = (pids ?? []).filter((pid) => Number.isFinite(pid) && pid > 0);
  if (values.length === 0) return { label: '-', title: '' };
  const visible = values.slice(0, 3).map(String);
  const suffix = values.length > visible.length ? ` +${values.length - visible.length}` : '';
  return { label: `${visible.join(', ')}${suffix}`, title: values.join(', ') };
};

const serviceStatusVariant = (
  status: Exclude<TrueNASServiceStatusFilter, 'all'>,
): 'success' | 'warning' | 'danger' | 'muted' => {
  if (status === 'running') return 'success';
  if (status === 'attention') return 'warning';
  if (status === 'stopped') return 'danger';
  return 'muted';
};

type ServiceDetailTone = DetailValueTone;
type ServiceDetailSection = DetailSection;

const detailBool = (value?: boolean): string | null => {
  if (value === undefined) return null;
  return value ? 'Enabled' : 'Disabled';
};

const detailRow = makeDetailRow;

const serviceTone = (row: TrueNASServiceRow): ServiceDetailTone => {
  const status = mapTrueNASServiceStatus(row);
  if (status === 'running') return 'success';
  if (status === 'attention' || status === 'stopped') return 'warning';
  if (status === 'disabled') return 'muted';
  return 'default';
};

const formatAllPIDs = (pids: number[] | undefined): { label: string | null; title?: string } => {
  const values = (pids ?? []).filter((pid) => Number.isFinite(pid) && pid > 0);
  if (values.length === 0) return { label: null };
  const label = values.join(', ');
  return { label, title: label };
};

const buildServiceDetailSections = (row: TrueNASServiceRow): ServiceDetailSection[] => {
  const pids = formatAllPIDs(row.service.pids);
  const hostName = asTrimmedString(row.system.truenas?.hostname) || row.systemName;
  return compactDetailSections([
    {
      label: 'Service',
      rows: compactDetailRows([
        detailRow('Name', formatServiceName(row.service.service)),
        detailRow('TrueNAS ID', asTrimmedString(row.service.id)),
        detailRow('System', row.systemName),
        detailRow('System ID', row.systemId),
      ]),
    },
    {
      label: 'Runtime',
      rows: compactDetailRows([
        detailRow('State', formatPlatformTableTitleCaseValue(row.service.state), {
          tone: serviceTone(row),
        }),
        detailRow('Boot', detailBool(row.service.enabled), {
          tone: row.service.enabled ? 'success' : 'muted',
        }),
        detailRow('PIDs', pids.label, { title: pids.title }),
        detailRow('PID count', row.service.pids?.length ? String(row.service.pids.length) : null),
      ]),
    },
    {
      label: 'Host',
      rows: compactDetailRows([
        detailRow('Hostname', hostName),
        detailRow('Version', asTrimmedString(row.system.truenas?.version)),
      ]),
    },
  ]);
};

const ServiceDetailTable: Component<{ row: TrueNASServiceRow; onClose: () => void }> = (props) => (
  <InlineDetailPanel
    testId="truenas-service-detail"
    detailFor={props.row.id}
    title="Service detail"
    summary={`${formatServiceName(props.row.service.service)} · ${formatPlatformTableTitleCaseValue(
      mapTrueNASServiceStatus(props.row),
    )}`}
    sections={buildServiceDetailSections(props.row)}
    detailAttributes={{ 'data-truenas-service-detail-for': props.row.id }}
    onClose={props.onClose}
  />
);

// Columns a user can sort by. PIDs orders on the process count so the busiest
// services surface first.
const TRUENAS_SERVICE_SORT_KEYS = ['service', 'state', 'boot', 'pids', 'system'] as const;

type TrueNASServiceSortKey = (typeof TRUENAS_SERVICE_SORT_KEYS)[number];

const getTrueNASServiceSortValue = (
  row: TrueNASServiceRow,
  key: TrueNASServiceSortKey,
): PlatformTableSortValue => {
  switch (key) {
    case 'service':
      return formatServiceName(row.service.service);
    case 'state':
      return asTrimmedString(row.service.state) || null;
    case 'boot':
      return row.service.enabled ? 'Enabled' : 'Disabled';
    case 'pids': {
      const count = (row.service.pids ?? []).filter(
        (pid) => Number.isFinite(pid) && pid > 0,
      ).length;
      return count > 0 ? count : null;
    }
    case 'system':
      return asTrimmedString(row.systemName) || null;
    default:
      key satisfies never;
      return null;
  }
};

export const TrueNASServicesTable: Component<{
  services: TrueNASServiceRow[];
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
  showToolbar?: boolean;
}> = (props) => {
  const tableState = createPlatformTableFilterState({
    resources: () => props.services,
    initialStatus: 'all' as TrueNASServiceStatusFilter,
    filter: filterTrueNASServices,
  });
  const detail = createPlatformResourceDetailState({ idPrefix: 'truenas-service-detail' });
  const sort = createPlatformTableSortState({
    storageKey: 'truenasServices',
    sortKeys: TRUENAS_SERVICE_SORT_KEYS,
    descendingFirst: ['pids'],
  });
  const sortedRows = createMemo(() =>
    sort.sortRows(tableState.filtered(), getTrueNASServiceSortValue),
  );

  return (
    <Show
      when={props.services.length > 0}
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
            searchPlaceholder="Search TrueNAS services"
            status={tableState.status()}
            onStatusChange={tableState.setStatus}
            statusOptions={TRUENAS_SERVICE_STATUS_OPTIONS}
            visible={tableState.visible()}
            total={tableState.total()}
            rowNoun="services"
          />
        </Show>

        <Show
          when={tableState.filtered().length > 0}
          fallback={
            <PlatformTableEmptyState
              icon={props.emptyIcon}
              title="No services match current filters"
              description="Adjust the search or status filter to see more TrueNAS services."
            />
          }
        >
          <PlatformTableShell
            title="Services"
            tableClass="min-w-full table-fixed text-xs md:min-w-[900px]"
            header={
              <>
                <PlatformSortableTableHead
                  kind="name"
                  sort={sort}
                  sortKey="service"
                  class="md:w-[24%]"
                >
                  Service
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="badge"
                  sort={sort}
                  sortKey="state"
                  class="md:w-[14%]"
                >
                  State
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="badge"
                  sort={sort}
                  sortKey="boot"
                  class="md:w-[14%]"
                >
                  Boot
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  sortKey="pids"
                  class="hidden sm:table-cell md:w-[20%]"
                >
                  PIDs
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  sortKey="system"
                  class="md:w-[28%]"
                >
                  System
                </PlatformSortableTableHead>
              </>
            }
            body={
              <>
                <For each={sortedRows()}>
                  {(row) => {
                    const status = () => mapTrueNASServiceStatus(row);
                    const pids = createMemo(() => formatPIDs(row.service.pids));
                    const rawState = () => asTrimmedString(row.service.state) || '-';
                    const detailRowId = () => detail.detailRowId(row);
                    const isExpanded = () => detail.isExpanded(row);
                    return (
                      <>
                        <TableRow
                          class={`${getPlatformResourceDetailRowClass(isExpanded())} text-[11px] sm:text-xs`}
                          aria-controls={isExpanded() ? detailRowId() : undefined}
                          aria-expanded={isExpanded() ? 'true' : 'false'}
                          data-truenas-service-row={row.id}
                          onClick={() => detail.toggle(row)}
                          onKeyDown={detail.handleActivationKey(row)}
                          tabIndex={0}
                        >
                          <TableCell class={getPlatformTableCellClassForKind('name')}>
                            <div class="flex min-w-0 items-center gap-2">
                              <PlatformResourceDetailToggleButton
                                expanded={isExpanded()}
                                resourceLabel={formatServiceName(row.service.service)}
                                controlsId={detailRowId()}
                                onToggle={() => detail.toggle(row)}
                              />
                              <StatusDot
                                size="sm"
                                variant={serviceStatusVariant(status())}
                                title={formatPlatformTableTitleCaseValue(status())}
                              />
                              <div class="min-w-0 truncate font-medium text-base-content">
                                {formatServiceName(row.service.service)}
                              </div>
                            </div>
                          </TableCell>
                          <TableCell class={getPlatformTableCellClassForKind('badge')}>
                            <span class="inline-flex rounded-full border border-border px-2 py-0.5 text-[11px] font-medium text-base-content">
                              {formatPlatformTableTitleCaseValue(rawState())}
                            </span>
                          </TableCell>
                          <TableCell class={getPlatformTableCellClassForKind('badge')}>
                            <span
                              class={`inline-flex rounded-full border px-2 py-0.5 text-[11px] font-medium ${
                                row.service.enabled
                                  ? 'border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-800 dark:bg-emerald-950 dark:text-emerald-300'
                                  : 'border-border bg-surface-alt text-muted'
                              }`}
                            >
                              {row.service.enabled ? 'Enabled' : 'Disabled'}
                            </span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden sm:table-cell`}
                            title={pids().title || undefined}
                          >
                            <span class="tabular-nums text-base-content">{pids().label}</span>
                          </TableCell>
                          <TableCell class={getPlatformTableCellClassForKind('text')}>
                            <div class="truncate text-base-content">{row.systemName}</div>
                          </TableCell>
                        </TableRow>
                        <Show when={isExpanded()}>
                          <InlineDetailTableRow
                            cellId={detailRowId()}
                            colspan={5}
                            data-inline-detail-for={row.id}
                            data-truenas-service-detail-row={row.id}
                          >
                            <ServiceDetailTable row={row} onClose={() => detail.close(row)} />
                          </InlineDetailTableRow>
                        </Show>
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

export default TrueNASServicesTable;
