import { For, Show, createMemo, type Component } from 'solid-js';
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
import {
  PLATFORM_HEALTH_FILTER_OPTIONS,
  PLATFORM_TABLE_BODY_CLASS,
  PLATFORM_TABLE_CARD_CLASS,
  PLATFORM_TABLE_HEADER_ROW_CLASS,
  PlatformTableEmptyState,
  PlatformTableToolbar,
  createPlatformTableFilterState,
  getPlatformTableCellClassForKind,
  getPlatformTableHeadClassForKind,
} from '@/features/platformPage/sharedPlatformPage';
import {
  createPlatformResourceDetailState,
  getPlatformResourceDetailRowClass,
} from '@/features/platformPage/PlatformResourceDetailTableRow';
import type { StatusIndicator } from '@/utils/status';
import {
  DockerResourceNameCell,
  dockerHostName,
  dockerJoinValues,
  dockerTextValue,
  type DockerNativeTableProps,
} from './DockerNativeTableShared';
import {
  buildDockerNetworkAttachmentRows,
  dockerResourceSearchHaystack,
  filterDockerResources,
  type DockerNetworkAttachmentRow,
  type DockerResourceStatusFilter,
} from './dockerPageModel';
import type { Resource } from '@/types/resource';

type DockerNetworksTableProps = DockerNativeTableProps & {
  relatedResources?: Resource[];
};

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
  const danger = rows.filter((row) => row.status.variant === 'danger').length;
  if (danger > 0) {
    return { variant: 'danger', label: `${danger} needs attention` };
  }
  const warning = rows.filter((row) => row.status.variant === 'warning').length;
  if (warning > 0) {
    return { variant: 'warning', label: `${warning} warning` };
  }
  const running = rows.filter((row) => row.status.variant === 'success').length;
  if (running === rows.length) {
    return { variant: 'success', label: `${running} running` };
  }
  return { variant: 'muted', label: attachmentCountLabel(rows.length) };
};

const networkDetailRows = (resource: Resource) => [
  ['Driver', dockerTextValue(resource.docker?.driver)],
  ['Scope', dockerTextValue(resource.docker?.scope)],
  ['Addressing', networkAddressing(resource)],
  ['Flags', networkFlags(resource)],
  ['Subnets', networkSubnets(resource)],
  ['Network ID', dockerTextValue(resource.docker?.networkId)],
];

const AttachmentDetail: Component<{ rows: readonly DockerNetworkAttachmentRow[] }> = (props) => (
  <section class="min-w-0">
    <div class="mb-2 flex items-center justify-between gap-2">
      <h3 class="text-xs font-semibold text-base-content">Attached containers</h3>
      <span class="text-[11px] text-muted">{attachmentCountLabel(props.rows.length)}</span>
    </div>
    <Show
      when={props.rows.length > 0}
      fallback={
        <div class="rounded border border-border bg-surface px-3 py-2 text-[11px] text-muted">
          No attached Docker containers reported on this network.
        </div>
      }
    >
      <div class="space-y-1.5">
        <For each={props.rows}>
          {(row) => (
            <div class="rounded border border-border bg-surface px-3 py-2">
              <div class="grid gap-2 text-[11px] md:grid-cols-[minmax(0,1.2fr)_7rem_9rem_minmax(0,1fr)] md:items-center">
                <div class="flex min-w-0 items-center gap-2">
                  <StatusDot size="sm" variant={row.status.variant} title={row.status.label} />
                  <span class="truncate font-semibold text-base-content" title={row.name}>
                    {row.name}
                  </span>
                </div>
                <span class="text-base-content">{row.status.label}</span>
                <span class="font-mono text-base-content" title={row.address}>
                  {row.address}
                </span>
                <span class="truncate font-mono text-muted" title={row.ports}>
                  {row.ports}
                </span>
              </div>
              <div class="mt-1 truncate text-[10px] text-muted" title={row.image}>
                {row.image}
              </div>
            </div>
          )}
        </For>
      </div>
    </Show>
  </section>
);

const NetworkConfigDetail: Component<{ resource: Resource }> = (props) => (
  <section class="min-w-0">
    <h3 class="mb-2 text-xs font-semibold text-base-content">Network details</h3>
    <div class="rounded border border-border bg-surface px-3 py-2">
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
    return filterDockerResources(resources, '', status).filter((resource) => {
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
      <div class="space-y-3">
        <Show when={props.showToolbar !== false}>
          <PlatformTableToolbar
            search={tableState.search}
            onSearchChange={tableState.setSearch}
            searchPlaceholder="Search networks"
            status={tableState.status()}
            onStatusChange={tableState.setStatus}
            statusOptions={PLATFORM_HEALTH_FILTER_OPTIONS}
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
          <TableCard class={PLATFORM_TABLE_CARD_CLASS}>
            <TableCardHeader title={props.title ?? 'Networks'} />
            <Table class="min-w-[850px] table-fixed text-xs md:min-w-[1320px]">
              <TableHeader>
                <TableRow class={PLATFORM_TABLE_HEADER_ROW_CLASS}>
                  <TableHead class={`${getPlatformTableHeadClassForKind('name')} w-[220px]`}>
                    Network
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClassForKind('text')} w-[360px]`}>
                    Attached workloads
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClassForKind('text')} w-[150px]`}>
                    Attention
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[300px]`}
                  >
                    Subnets
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[140px]`}
                  >
                    Driver
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[140px]`}
                  >
                    Host
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody class={PLATFORM_TABLE_BODY_CLASS}>
                <For each={tableState.filtered()}>
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
                          aria-controls={isExpanded() ? detailRowId() : undefined}
                          aria-expanded={isExpanded() ? 'true' : 'false'}
                          data-docker-network-row={resource.id}
                          onClick={() => drawer.toggle(resource)}
                          onKeyDown={drawer.handleActivationKey(resource)}
                          tabIndex={0}
                        >
                          <DockerResourceNameCell resource={resource} />
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
                            <span class="inline-flex max-w-full items-center gap-1.5 truncate">
                              <StatusDot
                                size="sm"
                                variant={attention().variant}
                                title={attention().label}
                                ariaHidden
                              />
                              <span class="truncate" title={attention().label}>
                                {attention().label}
                              </span>
                            </span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                          >
                            <span class="inline-block max-w-[18rem] truncate" title={subnets()}>
                              {subnets()}
                            </span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                          >
                            {dockerTextValue(resource.docker?.driver)}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                          >
                            {dockerHostName(resource)}
                          </TableCell>
                        </TableRow>
                        <Show when={isExpanded()}>
                          <TableRow data-docker-network-detail-row={resource.id}>
                            <TableCell
                              id={detailRowId()}
                              colspan={6}
                              class="border-b border-border bg-surface-alt p-0"
                            >
                              <div
                                class="grid gap-4 px-2 py-3 sm:px-4 sm:py-4 lg:grid-cols-[minmax(0,1fr)_minmax(16rem,24rem)]"
                                onClick={(event) => event.stopPropagation()}
                              >
                                <AttachmentDetail rows={attachments()} />
                                <NetworkConfigDetail resource={resource} />
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

export default DockerNetworksTable;
