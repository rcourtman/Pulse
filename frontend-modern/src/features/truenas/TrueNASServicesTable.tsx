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
  TrueNASInlineDetailTable,
  compactTrueNASDetailRows,
  compactTrueNASDetailSections,
  makeTrueNASDetailRow,
  type TrueNASDetailSection,
  type TrueNASDetailTone,
} from '@/components/Infrastructure/TrueNASDetailTable';

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

const titleCase = (value: string | undefined): string => {
  const normalized = asTrimmedString(value);
  if (!normalized) return 'Unknown';
  return normalized.charAt(0).toUpperCase() + normalized.slice(1).toLowerCase();
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

type ServiceDetailTone = TrueNASDetailTone;
type ServiceDetailSection = TrueNASDetailSection;

const detailBool = (value?: boolean): string | null => {
  if (value === undefined) return null;
  return value ? 'Enabled' : 'Disabled';
};

const detailRow = makeTrueNASDetailRow;
const compactDetailRows = compactTrueNASDetailRows;
const compactDetailSections = compactTrueNASDetailSections;

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
        detailRow('State', titleCase(asTrimmedString(row.service.state) || undefined), {
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
  <TrueNASInlineDetailTable
    testId="truenas-service-detail"
    detailFor={props.row.id}
    detailKind="service"
    title="Service detail"
    summary={`${formatServiceName(props.row.service.service)} · ${titleCase(
      mapTrueNASServiceStatus(props.row),
    )}`}
    sections={buildServiceDetailSections(props.row)}
    onClose={props.onClose}
  />
);

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
          <TableCard class={PLATFORM_TABLE_CARD_CLASS}>
            <TableCardHeader title="Services" />
            <Table class="min-w-full table-fixed text-xs md:min-w-[900px]">
              <TableHeader>
                <TableRow class={PLATFORM_TABLE_HEADER_ROW_CLASS}>
                  <TableHead class={`${getPlatformTableHeadClassForKind('name')} md:w-[24%]`}>
                    Service
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClassForKind('badge')} md:w-[14%]`}>
                    State
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClassForKind('badge')} md:w-[14%]`}>
                    Boot
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('text')} hidden sm:table-cell md:w-[20%]`}
                  >
                    PIDs
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClassForKind('text')} md:w-[28%]`}>
                    System
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody class={PLATFORM_TABLE_BODY_CLASS}>
                <For each={tableState.filtered()}>
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
                              <StatusDot
                                size="sm"
                                variant={serviceStatusVariant(status())}
                                title={titleCase(status())}
                              />
                              <div class="min-w-0 truncate font-medium text-base-content">
                                {formatServiceName(row.service.service)}
                              </div>
                            </div>
                          </TableCell>
                          <TableCell class={getPlatformTableCellClassForKind('badge')}>
                            <span class="inline-flex rounded-full border border-border px-2 py-0.5 text-[11px] font-medium text-base-content">
                              {titleCase(rawState())}
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
                          <TableRow
                            data-inline-detail-for={row.id}
                            data-truenas-service-detail-row={row.id}
                          >
                            <TableCell
                              id={detailRowId()}
                              colspan={5}
                              class="border-b border-border bg-surface-alt p-0"
                            >
                              <div
                                class="px-2 py-3 sm:px-4 sm:py-4"
                                onClick={(event) => event.stopPropagation()}
                              >
                                <ServiceDetailTable row={row} onClose={() => detail.close(row)} />
                              </div>
                            </TableCell>
                          </TableRow>
                        </Show>
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

export default TrueNASServicesTable;
