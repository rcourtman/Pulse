import { Show, createMemo, type Component, type JSX } from 'solid-js';
import { InlineDetailTableRow } from '@/components/shared/InlineDetailTableRow';
import { TableCell, TableRow } from '@/components/shared/Table';
import { asTrimmedString } from '@/utils/stringUtils';
import {
  PlatformWindowedRows,
  PlatformSortableTableHead,
  createPlatformTableFilterState,
  createPlatformTableSortState,
  formatPlatformTableTitleCaseValue,
  getPlatformTableCellClassForKind,
  type PlatformTableFilterOption,
  type PlatformTableSortValue,
} from '@/features/platformPage/sharedPlatformPage';
import {
  TrueNASPhoneStateNote,
  TrueNASRowLead,
  TrueNASStateNoteDetails,
  TrueNASTableFrame,
  createTrueNASDetailState,
  resolveTrueNASAttentionNote,
  type TrueNASStateNote,
} from './TrueNASTableFrame';
import { getPlatformResourceDetailRowClass } from '@/features/platformPage/PlatformResourceDetailTableRow';
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

// A service set to start at boot that is not running is the one state a red
// dot alone cannot explain, so the State cell says so beside the raw state.
// Rows stay single-line (the shared platform-table rhythm), so the full
// sentence rides on the title and in the drawer. PIDs are process detail
// nobody acts on from the list, so they live in the drawer too.
const serviceStateNote = (
  row: TrueNASServiceRow,
  status: Exclude<TrueNASServiceStatusFilter, 'all'>,
): TrueNASStateNote | null => {
  if (status === 'stopped') {
    return {
      short: 'should be running',
      full: 'Set to start at boot but not running',
      tone: 'danger',
    };
  }
  return status === 'attention' ? resolveTrueNASAttentionNote(row.system, row.service.state) : null;
};

// The drawer value cell is one line on a phone, so it carries the short form
// with a capital, and the Boot row beside it says why it should be running.
const serviceConditionLabel = (row: TrueNASServiceRow): string | null => {
  const note = serviceStateNote(row, mapTrueNASServiceStatus(row));
  if (!note) return null;
  if (!note.short) return note.full;
  return note.short.charAt(0).toUpperCase() + note.short.slice(1);
};

const SERVICE_STATUS_VARIANTS: Record<
  Exclude<TrueNASServiceStatusFilter, 'all'>,
  'success' | 'warning' | 'danger' | 'muted'
> = { running: 'success', attention: 'warning', stopped: 'danger', disabled: 'muted' };

const serviceStatusVariant = (status: Exclude<TrueNASServiceStatusFilter, 'all'>) =>
  SERVICE_STATUS_VARIANTS[status];

const serviceTone = (row: TrueNASServiceRow): DetailValueTone => {
  const status = mapTrueNASServiceStatus(row);
  return status === 'running' ? 'success' : status === 'disabled' ? 'muted' : 'warning';
};

const detailBool = (value?: boolean): string | null =>
  value === undefined ? null : value ? 'Enabled' : 'Disabled';

const detailRow = makeDetailRow;

const formatAllPIDs = (pids: number[] | undefined): { label: string | null; title?: string } => {
  const values = (pids ?? []).filter((pid) => Number.isFinite(pid) && pid > 0);
  if (values.length === 0) return { label: null };
  const label = values.join(', ');
  return { label, title: label };
};

const buildServiceDetailSections = (row: TrueNASServiceRow): DetailSection[] => {
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
        detailRow('Condition', serviceConditionLabel(row), {
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

// Columns a user can sort by.
const TRUENAS_SERVICE_SORT_KEYS = ['service', 'state', 'boot', 'system'] as const;

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
  /** The page's URL search, when it owns one; the table then reads and writes it. */
  search?: () => string;
  onSearchChange?: (value: string) => void;
}> = (props) => {
  const tableState = createPlatformTableFilterState({
    resources: () => props.services,
    initialStatus: 'all' as TrueNASServiceStatusFilter,
    filter: filterTrueNASServices,
    externalSearch: props.search,
    onExternalSearchChange: props.onSearchChange,
  });
  const detail = createTrueNASDetailState('truenas-service-detail');
  const sort = createPlatformTableSortState({
    storageKey: 'truenasServices',
    sortKeys: TRUENAS_SERVICE_SORT_KEYS,
  });
  const sortedRows = createMemo(() =>
    sort.sortRows(tableState.filtered(), getTrueNASServiceSortValue),
  );

  return (
    <TrueNASTableFrame
      hasItems={props.services.length > 0}
      tableState={tableState}
      common={props}
      searchPlaceholder="Search TrueNAS services"
      statusOptions={TRUENAS_SERVICE_STATUS_OPTIONS}
      rowNoun="services"
      title="Services"
      tableClass="min-w-full table-fixed text-xs md:min-w-[900px]"
      header={
        <>
          <PlatformSortableTableHead
            kind="name"
            sort={sort}
            sortKey="service"
            class="platform-table-mobile-w-30 md:w-[34%]"
          >
            Service
          </PlatformSortableTableHead>
          <PlatformSortableTableHead
            kind="badge"
            sort={sort}
            sortKey="state"
            class="platform-table-phone-hidden md:w-[16%]"
          >
            State
          </PlatformSortableTableHead>
          <PlatformSortableTableHead
            kind="badge"
            sort={sort}
            sortKey="boot"
            class="platform-table-mobile-w-15 md:w-[16%]"
          >
            Boot
          </PlatformSortableTableHead>
          <PlatformSortableTableHead
            kind="text"
            sort={sort}
            sortKey="system"
            class="platform-table-phone-hidden md:w-[34%]"
          >
            System
          </PlatformSortableTableHead>
        </>
      }
      body={
        <>
          <PlatformWindowedRows items={sortedRows} estimatedRowHeight={32}>
            {(row) => {
              const status = () => mapTrueNASServiceStatus(row);
              const note = () => serviceStateNote(row, status());
              const rawState = () => asTrimmedString(row.service.state) || '-';
              const detailRowId = () => detail.detailRowId(row);
              const isExpanded = () => detail.isExpanded(row);
              return (
                <>
                  <TableRow
                    class={`${getPlatformResourceDetailRowClass(isExpanded())} text-[11px] sm:text-xs`}
                    data-truenas-service-row={row.id}
                    onClick={() => detail.toggle(row)}
                  >
                    <TableCell class={getPlatformTableCellClassForKind('name')}>
                      <TrueNASRowLead
                        item={row}
                        drawer={detail}
                        label={formatServiceName(row.service.service)}
                        variant={serviceStatusVariant(status())}
                        dotTitle={formatPlatformTableTitleCaseValue(status())}
                      >
                        <div class="flex min-w-0 flex-1 items-center gap-1.5">
                          <div class="min-w-0 truncate font-medium text-base-content">
                            {formatServiceName(row.service.service)}
                          </div>
                          {/* A phone hides State; a state that needs attention joins the
                                    name on its line. */}
                          <TrueNASPhoneStateNote
                            note={note()}
                            stateLabel={formatPlatformTableTitleCaseValue(rawState())}
                            dataAttr="service"
                          />
                        </div>
                      </TrueNASRowLead>
                    </TableCell>
                    <TableCell
                      class={`${getPlatformTableCellClassForKind('badge')} platform-table-phone-hidden`}
                    >
                      <TrueNASStateNoteDetails
                        note={note()}
                        labelClass="inline-flex shrink-0 rounded-full border border-border px-2 py-0.5 text-[11px] font-medium text-base-content"
                        label={formatPlatformTableTitleCaseValue(rawState())}
                        dataAttr="service"
                      />
                    </TableCell>
                    <TableCell class={getPlatformTableCellClassForKind('badge')}>
                      <span
                        class={`inline-flex rounded-full border px-2 py-0.5 text-[11px] font-medium ${
                          row.service.enabled
                            ? 'border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-800 dark:bg-emerald-950/25 dark:text-emerald-300'
                            : 'border-border bg-surface-alt text-muted'
                        }`}
                      >
                        {row.service.enabled ? 'Enabled' : 'Disabled'}
                      </span>
                    </TableCell>
                    <TableCell
                      class={`${getPlatformTableCellClassForKind('text')} platform-table-phone-hidden`}
                    >
                      <div class="truncate text-base-content">{row.systemName}</div>
                    </TableCell>
                  </TableRow>
                  <Show when={isExpanded()}>
                    <InlineDetailTableRow
                      cellId={detailRowId()}
                      colspan={4}
                      data-inline-detail-for={row.id}
                      data-truenas-service-detail-row={row.id}
                    >
                      <InlineDetailPanel
                        testId="truenas-service-detail"
                        detailFor={row.id}
                        title={formatServiceName(row.service.service)}
                        status={{
                          variant: serviceStatusVariant(status()),
                          label: formatPlatformTableTitleCaseValue(status()),
                        }}
                        sections={buildServiceDetailSections(row)}
                        detailAttributes={{ 'data-truenas-service-detail-for': row.id }}
                        onClose={() => detail.close(row)}
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
  );
};

export default TrueNASServicesTable;
