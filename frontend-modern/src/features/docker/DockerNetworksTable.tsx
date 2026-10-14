import { For, Show, createMemo, createSignal, type Component } from 'solid-js';
import { InlineDetailTableRow } from '@/components/shared/InlineDetailTableRow';
import { StatusDot } from '@/components/shared/StatusDot';
import { TableCell, TableHead, TableRow } from '@/components/shared/Table';
import {
  PLATFORM_HEALTH_FILTER_OPTIONS,
  PlatformResponsiveTableLabel,
  type PlatformTableFilterOption,
  PlatformTableEmptyState,
  PlatformTableToolbar,
  createPlatformTableFilterState,
  getPlatformTableCellClassForKind,
  getPlatformTableContainerLayout,
  getPlatformTableHeadClassForKind,
  type PlatformTableContainerLayout,
  PlatformTableShell,
  PlatformWindowedRows,
  withPlatformStatusCounts,
} from '@/features/platformPage/sharedPlatformPage';
import { useObservedElementWidth } from '@/hooks/useObservedElementWidth';
import {
  PlatformResourceDetailToggleButton,
  createPlatformResourceDetailState,
  getPlatformResourceDetailRowClass,
} from '@/features/platformPage/PlatformResourceDetailTableRow';
import { getSimpleStatusIndicator, type StatusIndicator } from '@/utils/status';
import {
  DockerResourceNameCell,
  dockerHostName,
  dockerJoinValues,
  dockerResourceName,
  dockerStateTextToneClass,
  dockerTextValue,
  type DockerNativeTableProps,
} from './DockerNativeTableShared';
import {
  buildDockerNetworkAttachmentRows,
  dockerContainerRunState,
  dockerResourceSearchHaystack,
  type DockerNetworkAttachmentRow,
  type DockerResourceStatusFilter,
} from './dockerPageModel';
import type { Resource } from '@/types/resource';

type DockerNetworksTableProps = DockerNativeTableProps & {
  relatedResources?: Resource[];
};

type AttachmentGroupKey = 'attention' | 'running' | 'other';

type AttachmentStatusFilter = 'all' | AttachmentGroupKey;

type DockerNetworkColumn = 'network' | 'attached' | 'attention' | 'subnet' | 'driver' | 'host';

const DOCKER_NETWORK_COLUMN_WIDTH_CLASS: Record<
  PlatformTableContainerLayout,
  Record<DockerNetworkColumn, string>
> = {
  // A phone row has no Subnets or Driver column (the expansion's Network
  // details list both), so the attached summary and its state get the room:
  // "3 need attention" whole beside its dot, the summary truncating instead.
  compact: {
    network: 'w-[30%]',
    attached: 'w-[30%]',
    attention: 'w-[40%]',
    subnet: 'w-0',
    driver: 'w-0',
    host: 'w-0',
  },
  basic: {
    network: 'w-[30%]',
    attached: 'w-[25%]',
    attention: 'w-[15%]',
    subnet: 'w-[15%]',
    driver: 'w-[15%]',
    host: 'w-0',
  },
  operational: {
    network: 'w-[25%]',
    attached: 'w-[30%]',
    attention: 'w-[15%]',
    subnet: 'w-[20%]',
    driver: 'w-[10%]',
    host: 'w-0',
  },
  expanded: {
    network: 'w-[23%]',
    attached: 'w-[32%]',
    attention: 'w-[13%]',
    subnet: 'w-[22%]',
    driver: 'w-[10%]',
    host: 'w-0',
  },
  full: {
    network: 'w-[20%]',
    attached: 'w-[27%]',
    attention: 'w-[13%]',
    subnet: 'w-[18%]',
    driver: 'w-[10%]',
    host: 'w-[12%]',
  },
};

type AttachmentGroup = {
  key: AttachmentGroupKey;
  label: string;
  description: string;
};

const ATTACHMENT_DETAIL_ROW_LIMIT = 24;

const ATTACHMENT_GROUPS: readonly AttachmentGroup[] = [
  { key: 'attention', label: 'Attention', description: 'Needs operator review' },
  { key: 'other', label: 'Other', description: 'Stopped, paused, or unknown' },
  { key: 'running', label: 'Running', description: 'No active issue reported' },
] as const;

// No static ariaLabel here: the shared chip naming (label plus live count)
// is the accessible name, and the titles carry the descriptions.
const ATTACHMENT_STATUS_FILTER_OPTIONS: PlatformTableFilterOption<AttachmentStatusFilter>[] = [
  { value: 'all', label: 'All', title: 'All attached containers' },
  {
    value: 'attention',
    label: 'Attention',
    title: 'Containers that need review',
  },
  {
    value: 'running',
    label: 'Running',
    title: 'Running containers, including any that need review',
  },
  {
    value: 'other',
    label: 'Other',
    title: 'Containers not known to be running: stopped, paused, restarting, or unknown',
  },
];

const networkFlags = (resource: Resource): string =>
  dockerJoinValues(
    [
      resource.docker?.internal ? 'internal' : undefined,
      resource.docker?.attachable ? 'attachable' : undefined,
      resource.docker?.ingress ? 'ingress' : undefined,
      resource.docker?.configOnly ? 'config-only' : undefined,
    ],
    'standard',
  );

const networkAddressing = (resource: Resource): string =>
  dockerJoinValues(
    [
      resource.docker?.enableIpv4 ? 'IPv4' : undefined,
      resource.docker?.enableIpv6 ? 'IPv6' : undefined,
    ],
    '—',
  );

const networkSubnets = (resource: Resource): string =>
  dockerJoinValues(
    resource.docker?.subnets?.map((subnet) =>
      subnet.gateway ? `${subnet.subnet} via ${subnet.gateway}` : subnet.subnet,
    ),
  );

const attachmentCountLabel = (count: number): string =>
  `${count} attached ${count === 1 ? 'container' : 'containers'}`;

const attachmentPlainCountLabel = (count: number): string =>
  `${count} ${count === 1 ? 'container' : 'containers'}`;

const attachmentSummary = (rows: readonly DockerNetworkAttachmentRow[]): string => {
  if (rows.length === 0) return 'No containers';
  const labels = rows.slice(0, 3).map((row) => {
    if (row.address && row.address !== '—') return `${row.name} ${row.address}`;
    return row.name;
  });
  const suffix = rows.length > labels.length ? ` +${rows.length - labels.length}` : '';
  return `${attachmentCountLabel(rows.length)} · ${labels.join(', ')}${suffix}`;
};

const attachmentAttention = (rows: readonly DockerNetworkAttachmentRow[]): StatusIndicator => {
  if (rows.length === 0) return { variant: 'muted', label: 'Unused' };
  // Counts every row the Attention filter holds. The network itself is
  // degraded, not down, while something on it needs a look, so this reads
  // amber; each container's own dot in the expansion keeps its severity.
  const attention = rows.filter((row) => attachmentGroupKey(row) === 'attention');
  if (attention.length > 0) {
    return {
      variant: 'warning',
      label: `${attention.length} ${attention.length === 1 ? 'needs' : 'need'} attention`,
    };
  }
  const running = rows.filter((row) => row.status.variant === 'success').length;
  if (running === rows.length) {
    return { variant: 'success', label: `${running} running` };
  }
  return { variant: 'muted', label: attachmentCountLabel(rows.length) };
};

const NETWORK_DOT_RANK: Record<StatusIndicator['variant'], number> = {
  danger: 0,
  warning: 1,
  info: 2,
  muted: 3,
  success: 4,
};

// A network has no health of its own: Docker reports it or the host is lost.
// Its row dot says whether anything on it needs a look, unless the network's
// own status is worse (its host stopped reporting). The health filter reads
// the same verdict, so an amber row sits under Degraded, never Healthy.
const networkRowIndicator = (resource: Resource, attention: StatusIndicator): StatusIndicator => {
  const own = getSimpleStatusIndicator(resource.status);
  const attachedProblem = attention.variant === 'danger' || attention.variant === 'warning';
  return attachedProblem && NETWORK_DOT_RANK[attention.variant] < NETWORK_DOT_RANK[own.variant]
    ? attention
    : own;
};

const networkRowHealth = (indicator: StatusIndicator): DockerResourceStatusFilter | 'unknown' => {
  if (indicator.variant === 'success') return 'online';
  if (indicator.variant === 'warning') return 'degraded';
  if (indicator.variant === 'danger') return 'offline';
  return 'unknown';
};

const networkDetailRows = (resource: Resource) => [
  ['Driver', dockerTextValue(resource.docker?.driver)],
  ['Scope', dockerTextValue(resource.docker?.scope)],
  ['Addressing', networkAddressing(resource)],
  ['Flags', networkFlags(resource)],
  ['Subnets', networkSubnets(resource)],
  ['Network ID', dockerTextValue(resource.docker?.networkId)],
];

const attachmentGroupKey = (row: DockerNetworkAttachmentRow): AttachmentGroupKey => {
  if (row.status.variant === 'danger' || row.status.variant === 'warning') return 'attention';
  if (row.status.variant === 'success') return 'running';
  return 'other';
};

// Attention is Pulse's verdict on the row; Running and Other are Docker's run
// state. A container that is unhealthy or restarting too often while it runs
// is both running and worth a look, so it matches the Running filter as well
// as Attention. The sections below stay one per row: Attention first.
const attachmentMatchesStatus = (
  row: DockerNetworkAttachmentRow,
  status: AttachmentStatusFilter,
): boolean => {
  if (status === 'all') return true;
  if (status === 'attention') return attachmentGroupKey(row) === 'attention';
  const running = dockerContainerRunState(row.resource) === 'running';
  return status === 'running' ? running : !running;
};

const filterAttachmentRows = (
  rows: DockerNetworkAttachmentRow[],
  search: string,
  status: AttachmentStatusFilter,
): DockerNetworkAttachmentRow[] => {
  const needle = search.trim().toLowerCase();
  return rows.filter((row) => {
    if (!attachmentMatchesStatus(row, status)) return false;
    if (!needle) return true;
    return row.searchText.includes(needle);
  });
};

const AttachmentRowCard: Component<{ row: DockerNetworkAttachmentRow }> = (props) => (
  <div class="rounded-sm border border-border bg-surface px-3 py-2">
    <div class="grid gap-2 text-[11px] md:grid-cols-[minmax(0,1.2fr)_7rem_9rem_minmax(0,1fr)] md:items-center">
      <div class="flex min-w-0 items-center gap-2">
        <StatusDot size="sm" variant={props.row.status.variant} title={props.row.status.label} />
        <span class="truncate font-semibold text-base-content" title={props.row.name}>
          {props.row.name}
        </span>
      </div>
      <span class="text-base-content">{props.row.status.label}</span>
      <span class="font-mono text-base-content" title={props.row.address}>
        {props.row.address}
      </span>
      <span class="truncate font-mono text-muted" title={props.row.ports}>
        {props.row.ports}
      </span>
    </div>
    <div class="mt-1 truncate text-[10px] text-muted" title={props.row.image}>
      {props.row.image}
    </div>
  </div>
);

const AttachmentDetail: Component<{ rows: readonly DockerNetworkAttachmentRow[] }> = (props) => {
  const [showAll, setShowAll] = createSignal(false);
  const tableState = createPlatformTableFilterState<
    DockerNetworkAttachmentRow,
    AttachmentStatusFilter
  >({
    resources: () => [...props.rows],
    initialStatus: 'all',
    filter: filterAttachmentRows,
  });
  const visibleRows = createMemo(() =>
    showAll() ? tableState.filtered() : tableState.filtered().slice(0, ATTACHMENT_DETAIL_ROW_LIMIT),
  );
  const groupedRows = createMemo(() =>
    ATTACHMENT_GROUPS.map((group) => ({
      ...group,
      rows: visibleRows().filter((row) => attachmentGroupKey(row) === group.key),
    })).filter((group) => group.rows.length > 0),
  );
  const hiddenRowCount = createMemo(() =>
    Math.max(tableState.filtered().length - visibleRows().length, 0),
  );

  const setSearch = (value: string) => {
    tableState.setSearch(value);
    setShowAll(false);
  };
  const setStatus = (value: AttachmentStatusFilter) => {
    tableState.setStatus(value);
    setShowAll(false);
  };

  return (
    <section class="min-w-0">
      <div class="mb-2 flex items-center justify-between gap-2">
        <h3 class="text-xs font-semibold text-base-content">Attached containers</h3>
        <span class="text-[11px] text-muted">{attachmentCountLabel(tableState.total())}</span>
      </div>
      <Show
        when={props.rows.length > 0}
        fallback={
          <div class="rounded-sm border border-border bg-surface px-3 py-2 text-[11px] text-muted">
            No attached Docker containers reported on this network.
          </div>
        }
      >
        <div class="mb-3">
          <PlatformTableToolbar
            search={tableState.search}
            onSearchChange={setSearch}
            searchPlaceholder="Search attached containers"
            searchSuggestions={tableState.searchSuggestions}
            status={tableState.status()}
            onStatusChange={setStatus}
            statusOptions={withPlatformStatusCounts(
              ATTACHMENT_STATUS_FILTER_OPTIONS,
              tableState.countForStatus,
            )}
            visible={tableState.visible()}
            total={tableState.total()}
            rowNoun="containers"
            hasActiveFilters={tableState.hasActiveFilters()}
            onResetFilters={() => {
              tableState.resetFilters();
              setShowAll(false);
            }}
          />
        </div>

        <Show
          when={tableState.filtered().length > 0}
          fallback={
            <div class="rounded-sm border border-border bg-surface px-3 py-2 text-[11px] text-muted">
              No attached containers match current filters.
            </div>
          }
        >
          <div class="space-y-3">
            <For each={groupedRows()}>
              {(group) => (
                <section class="space-y-1.5">
                  <div class="flex items-center justify-between gap-2 text-[11px]">
                    <div class="flex min-w-0 items-center gap-2">
                      <span class="font-semibold text-base-content">{group.label}</span>
                      <span class="truncate text-muted">{group.description}</span>
                    </div>
                    <span class="text-muted">{attachmentPlainCountLabel(group.rows.length)}</span>
                  </div>
                  <div class="space-y-1.5">
                    <For each={group.rows}>{(row) => <AttachmentRowCard row={row} />}</For>
                  </div>
                </section>
              )}
            </For>
            <Show when={hiddenRowCount() > 0}>
              <button
                type="button"
                class="w-full rounded-sm border border-border bg-surface px-3 py-2 text-xs font-medium text-base-content hover:bg-surface-hover"
                onClick={() => setShowAll(true)}
              >
                Show all {attachmentPlainCountLabel(tableState.filtered().length)}
              </button>
            </Show>
            <Show when={showAll() && tableState.filtered().length > ATTACHMENT_DETAIL_ROW_LIMIT}>
              <button
                type="button"
                class="w-full rounded-sm border border-border bg-surface px-3 py-2 text-xs font-medium text-base-content hover:bg-surface-hover"
                onClick={() => setShowAll(false)}
              >
                Show first {ATTACHMENT_DETAIL_ROW_LIMIT}
              </button>
            </Show>
          </div>
        </Show>
      </Show>
    </section>
  );
};

const NetworkConfigDetail: Component<{ resource: Resource }> = (props) => (
  <section class="min-w-0">
    <h3 class="mb-2 text-xs font-semibold text-base-content">Network details</h3>
    <div class="rounded-sm border border-border bg-surface px-3 py-2">
      <For each={networkDetailRows(props.resource)}>
        {([label, value]) => (
          <div class="grid grid-cols-[6rem_minmax(0,1fr)] gap-3 py-1 text-[11px]">
            <span class="text-muted">{label}</span>
            <span class="truncate text-base-content" title={value}>
              {value}
            </span>
          </div>
        )}
      </For>
    </div>
  </section>
);

export const DockerNetworksTable: Component<DockerNetworksTableProps> = (props) => {
  const allResources = () => props.relatedResources ?? props.resources;
  const attachmentsByNetwork = createMemo(() => {
    const rows = new Map<string, DockerNetworkAttachmentRow[]>();
    for (const network of props.resources) {
      rows.set(network.id, buildDockerNetworkAttachmentRows(network, allResources()));
    }
    return rows;
  });
  const networkAttachments = (resource: Resource): readonly DockerNetworkAttachmentRow[] =>
    attachmentsByNetwork().get(resource.id) ?? [];

  const filterNetworks = (
    resources: Resource[],
    search: string,
    status: DockerResourceStatusFilter,
  ): Resource[] => {
    const needle = search.trim().toLowerCase();
    return resources.filter((resource) => {
      if (status !== 'all') {
        const attention = attachmentAttention(networkAttachments(resource));
        if (networkRowHealth(networkRowIndicator(resource, attention)) !== status) return false;
      }
      if (!needle) return true;
      if (dockerResourceSearchHaystack(resource).includes(needle)) return true;
      return networkAttachments(resource).some((row) => row.searchText.includes(needle));
    });
  };

  const tableState = createPlatformTableFilterState({
    resources: () => props.resources,
    initialStatus: 'all' as DockerResourceStatusFilter,
    filter: filterNetworks,
  });
  const drawer = createPlatformResourceDetailState({ idPrefix: 'docker-network-drawer' });
  const observedWidth = useObservedElementWidth();
  const layout = createMemo(() =>
    getPlatformTableContainerLayout(observedWidth.width() ?? 1920, [480, 720, 960, 1200]),
  );
  const showSubnet = createMemo(() => layout() !== 'compact');
  const showDriver = createMemo(() => layout() !== 'compact');
  const showHost = createMemo(() => layout() === 'full');
  const columnWidthClass = (column: DockerNetworkColumn) =>
    DOCKER_NETWORK_COLUMN_WIDTH_CLASS[layout()][column];
  const visibleColumnCount = createMemo(
    () => 3 + Number(showSubnet()) + Number(showDriver()) + Number(showHost()),
  );

  return (
    <Show
      when={props.resources.length > 0}
      fallback={
        <PlatformTableEmptyState
          icon={props.emptyIcon}
          title={props.emptyTitle}
          description={props.emptyDescription}
        />
      }
    >
      <div ref={observedWidth.setElement} class="space-y-3" data-docker-networks-layout={layout()}>
        <Show when={props.showToolbar !== false}>
          <PlatformTableToolbar
            search={tableState.search}
            onSearchChange={tableState.setSearch}
            searchPlaceholder="Search networks"
            searchSuggestions={tableState.searchSuggestions}
            status={tableState.status()}
            onStatusChange={tableState.setStatus}
            statusOptions={withPlatformStatusCounts(
              PLATFORM_HEALTH_FILTER_OPTIONS,
              tableState.countForStatus,
            )}
            visible={tableState.visible()}
            total={tableState.total()}
            rowNoun="networks"
          />
        </Show>

        <Show
          when={tableState.filtered().length > 0}
          fallback={
            <PlatformTableEmptyState
              icon={props.emptyIcon}
              title="No networks match current filters"
              description="Adjust the search or status filter to see more Docker networks."
            />
          }
        >
          <PlatformTableShell
            title={props.title ?? 'Networks'}
            tableClass="min-w-0 table-fixed text-xs"
            header={
              <>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('name')} platform-table-mobile-w-30 ${columnWidthClass('network')}`}
                >
                  Network
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('text')} ${columnWidthClass('attached')}`}
                >
                  <PlatformResponsiveTableLabel compact="Attached" full="Attached workloads" />
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('text')} ${columnWidthClass('attention')}`}
                >
                  State
                </TableHead>
                <Show when={showSubnet()}>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('text')} ${columnWidthClass('subnet')}`}
                  >
                    Subnets
                  </TableHead>
                </Show>
                <Show when={showDriver()}>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('text')} ${columnWidthClass('driver')}`}
                  >
                    Driver
                  </TableHead>
                </Show>
                <Show when={showHost()}>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('text')} ${columnWidthClass('host')}`}
                  >
                    Host
                  </TableHead>
                </Show>
              </>
            }
            body={
              <>
                <PlatformWindowedRows items={tableState.filtered} estimatedRowHeight={32}>
                  {(resource) => {
                    const detailRowId = () => drawer.detailRowId(resource);
                    const isExpanded = () => drawer.isExpanded(resource);
                    const attachments = () => networkAttachments(resource);
                    const attention = () => attachmentAttention(attachments());
                    const subnets = () => networkSubnets(resource);

                    return (
                      <>
                        <TableRow
                          class={`${getPlatformResourceDetailRowClass(isExpanded())} text-[11px] sm:text-xs`}
                          data-docker-network-row={resource.id}
                          onClick={() => drawer.toggle(resource)}
                        >
                          <DockerResourceNameCell
                            resource={resource}
                            indicator={networkRowIndicator(resource, attention())}
                            detailToggle={
                              <PlatformResourceDetailToggleButton
                                expanded={isExpanded()}
                                resourceLabel={dockerResourceName(resource)}
                                controlsId={detailRowId()}
                                onToggle={() => drawer.toggle(resource)}
                              />
                            }
                          />
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                          >
                            <span
                              class="block max-w-full truncate"
                              title={attachmentSummary(attachments())}
                            >
                              {attachmentSummary(attachments())}
                            </span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                          >
                            <span
                              class={`block max-w-full truncate ${dockerStateTextToneClass(
                                attention().variant,
                              )}`}
                              title={attention().label}
                            >
                              {attention().label}
                            </span>
                          </TableCell>
                          <Show when={showSubnet()}>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                            >
                              <span class="block truncate" title={subnets()}>
                                {subnets()}
                              </span>
                            </TableCell>
                          </Show>
                          <Show when={showDriver()}>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                            >
                              {dockerTextValue(resource.docker?.driver)}
                            </TableCell>
                          </Show>
                          <Show when={showHost()}>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                            >
                              {dockerHostName(resource)}
                            </TableCell>
                          </Show>
                        </TableRow>
                        <Show when={isExpanded()}>
                          <InlineDetailTableRow
                            cellId={detailRowId()}
                            colspan={visibleColumnCount()}
                            contentClass="grid gap-4 px-2 py-3 sm:px-4 sm:py-4 lg:grid-cols-[minmax(0,1fr)_minmax(16rem,24rem)]"
                            data-docker-network-detail-row={resource.id}
                          >
                            <AttachmentDetail rows={attachments()} />
                            <NetworkConfigDetail resource={resource} />
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

export default DockerNetworksTable;
