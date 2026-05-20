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
import type { Resource, ResourceTrueNASShareMeta } from '@/types/resource';
import {
  filterTrueNASShares,
  mapTrueNASShareStatus,
  type TrueNASShareStatusFilter,
} from './truenasPageModel';

const TRUENAS_SHARE_STATUS_OPTIONS: PlatformTableFilterOption<TrueNASShareStatusFilter>[] = [
  { value: 'all', label: 'All' },
  { value: 'active', label: 'Active', tone: 'success' },
  { value: 'attention', label: 'Attention', tone: 'warning' },
  { value: 'disabled', label: 'Disabled', tone: 'danger' },
];

const shareMeta = (resource: Resource): ResourceTrueNASShareMeta | undefined =>
  resource.truenas?.share;

const titleCase = (value: string | undefined): string => {
  const normalized = asTrimmedString(value);
  if (!normalized) return 'Unknown';
  return normalized.charAt(0).toUpperCase() + normalized.slice(1).toLowerCase();
};

const formatProtocol = (share: ResourceTrueNASShareMeta | undefined): string =>
  asTrimmedString(share?.protocol)?.toUpperCase() || '-';

const compactList = (values: Array<string | undefined>): string[] =>
  values.map((value) => asTrimmedString(value)).filter((value): value is string => Boolean(value));

const summarizeValues = (
  values: string[],
  empty = '-',
  visibleCount = 2,
): { label: string; title: string } => {
  if (values.length === 0) return { label: empty, title: '' };
  const visible = values.slice(0, visibleCount);
  const suffix = values.length > visible.length ? ` +${values.length - visible.length}` : '';
  return { label: `${visible.join(', ')}${suffix}`, title: values.join(', ') };
};

const formatAccess = (
  share: ResourceTrueNASShareMeta | undefined,
): { label: string; title: string } => {
  const flags = compactList([
    share?.readOnly === true ? 'Read-only' : share?.readOnly === false ? 'Read/write' : undefined,
    share?.browsable === true ? 'Browsable' : share?.browsable === false ? 'Hidden' : undefined,
    share?.accessBasedEnumeration ? 'ABE' : undefined,
    share?.auditEnabled ? 'Audit' : undefined,
    share?.exposeSnapshots ? 'Snapshots' : undefined,
  ]);
  return summarizeValues(flags, '-', 3);
};

const formatClients = (
  share: ResourceTrueNASShareMeta | undefined,
): { label: string; title: string } => {
  const networks = compactList(share?.networks ?? []);
  const hosts = compactList(share?.hosts ?? []);
  const aliases = compactList(share?.aliases ?? []);
  const values = [
    ...networks.map((network) => `net:${network}`),
    ...hosts.map((host) => `host:${host}`),
    ...aliases.map((alias) => `alias:${alias}`),
  ];
  return summarizeValues(values);
};

const formatSecurity = (
  share: ResourceTrueNASShareMeta | undefined,
): { label: string; title: string } => {
  const values = [
    ...compactList(share?.security ?? []),
    ...compactList([
      share?.mapRootUser ? `root:${share.mapRootUser}` : undefined,
      share?.mapAllUser ? `all:${share.mapAllUser}` : undefined,
    ]),
  ];
  return summarizeValues(values);
};

const shareName = (resource: Resource, share: ResourceTrueNASShareMeta | undefined): string =>
  asTrimmedString(share?.name) ||
  asTrimmedString(resource.displayName) ||
  asTrimmedString(resource.name) ||
  resource.id;

export const TrueNASNetworkSharesTable: Component<{
  shares: Resource[];
  scope: Resource[];
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
  showToolbar?: boolean;
}> = (props) => {
  const tableState = createPlatformTableFilterState({
    resources: () => props.shares,
    initialStatus: 'all' as TrueNASShareStatusFilter,
    filter: filterTrueNASShares,
  });
  const drawer = createPlatformResourceDetailState({ idPrefix: 'truenas-share-drawer' });
  const resolveResourceLabel = createPlatformResourceLabelResolver(() => props.scope);

  return (
    <Show
      when={props.shares.length > 0}
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
            searchPlaceholder="Search TrueNAS shares"
            status={tableState.status()}
            onStatusChange={tableState.setStatus}
            statusOptions={TRUENAS_SHARE_STATUS_OPTIONS}
            visible={tableState.visible()}
            total={tableState.total()}
            rowNoun="shares"
          />
        </Show>

        <Show
          when={tableState.filtered().length > 0}
          fallback={
            <PlatformTableEmptyState
              icon={props.emptyIcon}
              title="No shares match current filters"
              description="Adjust the search or status filter to see more TrueNAS shares."
            />
          }
        >
          <TableCard class={PLATFORM_TABLE_CARD_CLASS}>
            <TableCardHeader title="Network Shares" />
            <Table class="min-w-full table-fixed text-xs md:min-w-[1180px]">
              <TableHeader>
                <TableRow class={PLATFORM_TABLE_HEADER_ROW_CLASS}>
                  <TableHead class={`${getPlatformTableHeadClassForKind('name')} md:w-[20%]`}>
                    Share
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClassForKind('text')} md:w-[8%]`}>
                    Protocol
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('text')} hidden sm:table-cell md:w-[15%]`}
                  >
                    Dataset
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[23%]`}
                  >
                    Path
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClassForKind('text')} md:w-[16%]`}>
                    Access
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('text')} hidden lg:table-cell md:w-[12%]`}
                  >
                    Clients
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClassForKind('badge')} md:w-[6%]`}>
                    State
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody class={PLATFORM_TABLE_BODY_CLASS}>
                <For each={tableState.filtered()}>
                  {(resource) => {
                    const share = () => shareMeta(resource);
                    const name = () => shareName(resource, share());
                    const indicator = () => getSimpleStatusIndicator(resource.status);
                    const access = createMemo(() => formatAccess(share()));
                    const clients = createMemo(() => formatClients(share()));
                    const security = createMemo(() => formatSecurity(share()));
                    const stateLabel = () => titleCase(mapTrueNASShareStatus(resource));
                    const detailRowId = () => drawer.detailRowId(resource);
                    const isExpanded = () => drawer.isExpanded(resource);
                    return (
                      <>
                        <TableRow
                          class={`${getPlatformResourceDetailRowClass(isExpanded())} text-[11px] sm:text-xs`}
                          aria-controls={isExpanded() ? detailRowId() : undefined}
                          aria-expanded={isExpanded() ? 'true' : 'false'}
                          data-truenas-share-row={resource.id}
                          onClick={() => drawer.toggle(resource)}
                          onKeyDown={drawer.handleActivationKey(resource)}
                          tabIndex={0}
                        >
                          <TableCell class={getPlatformTableCellClassForKind('name')}>
                            <div class="flex min-w-0 items-center gap-2">
                              <StatusDot
                                size="sm"
                                variant={indicator().variant}
                                title={indicator().label}
                              />
                              <div class="min-w-0">
                                <div class="truncate font-medium text-base-content" title={name()}>
                                  {name()}
                                </div>
                                <div class="truncate text-[10px] text-muted">
                                  {share()?.dataset ||
                                    share()?.path ||
                                    resource.parentName ||
                                    'TrueNAS'}
                                </div>
                              </div>
                            </div>
                          </TableCell>
                          <TableCell class={getPlatformTableCellClassForKind('text')}>
                            <span class="font-mono text-[11px] text-base-content">
                              {formatProtocol(share())}
                            </span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content sm:table-cell`}
                          >
                            <span class="block truncate" title={share()?.dataset}>
                              {share()?.dataset || '-'}
                            </span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                          >
                            <span
                              class="block truncate font-mono text-[11px]"
                              title={share()?.path}
                            >
                              {share()?.path || '-'}
                            </span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                            title={[access().title, security().title].filter(Boolean).join(' | ')}
                          >
                            <span class="block truncate">{access().label}</span>
                            <Show when={security().label !== '-'}>
                              <span class="block truncate text-[10px] text-muted">
                                {security().label}
                              </span>
                            </Show>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content lg:table-cell`}
                            title={clients().title}
                          >
                            <span class="block truncate">{clients().label}</span>
                          </TableCell>
                          <TableCell class={getPlatformTableCellClassForKind('badge')}>
                            <span class="text-[11px] font-medium text-base-content">
                              {stateLabel()}
                            </span>
                          </TableCell>
                        </TableRow>
                        <PlatformResourceDetailTableRow
                          resource={resource}
                          open={isExpanded()}
                          detailRowId={detailRowId()}
                          colSpan={7}
                          resolveResourceLabel={resolveResourceLabel}
                          onClose={() => drawer.close(resource)}
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

export default TrueNASNetworkSharesTable;
