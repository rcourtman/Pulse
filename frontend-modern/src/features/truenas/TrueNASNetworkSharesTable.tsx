import { createMemo, type Component, type JSX } from 'solid-js';
import { TableCell, TableRow } from '@/components/shared/Table';
import { getSimpleStatusIndicator } from '@/utils/status';
import { asTrimmedString } from '@/utils/stringUtils';
import {
  PlatformWindowedRows,
  PlatformResponsiveTableLabel,
  PlatformSortableTableHead,
  createPlatformTableFilterState,
  createPlatformTableSortState,
  formatPlatformTableTitleCaseValue,
  getPlatformTableCellClassForKind,
  summarizePlatformTableValues,
  type PlatformTableFilterOption,
  type PlatformTableSortValue,
} from '@/features/platformPage/sharedPlatformPage';
import {
  TrueNASResourceDetailRow,
  TrueNASRowLead,
  TrueNASStateNoteDetails,
  TrueNASTableFrame,
  createTrueNASDetailState,
  type TrueNASStateNote,
} from './TrueNASTableFrame';
import {
  getPlatformInlineStateClass,
  getPlatformStatusTextToneClass,
  isPlatformAttentionVariant,
} from '@/features/platformPage/platformStatusTone';
import {
  createPlatformResourceLabelResolver,
  getPlatformResourceDetailRowClass,
} from '@/features/platformPage/PlatformResourceDetailTableRow';
import type { Resource, ResourceTrueNASShareMeta } from '@/types/resource';
import {
  filterTrueNASShares,
  getTrueNASResourceDisplayStatus,
  getTrueNASStaleNotice,
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

const formatProtocol = (share: ResourceTrueNASShareMeta | undefined): string =>
  asTrimmedString(share?.protocol)?.toUpperCase() || '-';

const formatAccess = (
  share: ResourceTrueNASShareMeta | undefined,
): { label: string; title: string } => {
  const flags = [
    share?.readOnly === true ? 'Read-only' : share?.readOnly === false ? 'Read/write' : undefined,
    share?.browsable === true ? 'Browsable' : share?.browsable === false ? 'Hidden' : undefined,
    share?.accessBasedEnumeration ? 'ABE' : undefined,
    share?.auditEnabled ? 'Audit' : undefined,
    share?.exposeSnapshots ? 'Snapshots' : undefined,
  ];
  return summarizePlatformTableValues(flags, { emptyText: '—', maxVisible: 3 });
};

const prefixedValues = (values: readonly string[] | undefined, prefix: string): string[] =>
  (values ?? [])
    .map((value) => asTrimmedString(value))
    .filter((value): value is string => Boolean(value))
    .map((value) => `${prefix}:${value}`);

const formatClients = (
  share: ResourceTrueNASShareMeta | undefined,
): { label: string; title: string } =>
  summarizePlatformTableValues(
    [
      ...prefixedValues(share?.networks, 'net'),
      ...prefixedValues(share?.hosts, 'host'),
      ...prefixedValues(share?.aliases, 'alias'),
    ],
    { emptyText: '—' },
  );

const formatSecurity = (
  share: ResourceTrueNASShareMeta | undefined,
): { label: string; title: string } => {
  const values = [
    ...(share?.security ?? []),
    ...[
      share?.mapRootUser ? `root:${share.mapRootUser}` : undefined,
      share?.mapAllUser ? `all:${share.mapAllUser}` : undefined,
    ],
  ];
  return summarizePlatformTableValues(values, { emptyText: '—' });
};

const shareName = (resource: Resource, share: ResourceTrueNASShareMeta | undefined): string =>
  asTrimmedString(share?.name) ||
  asTrimmedString(resource.displayName) ||
  asTrimmedString(resource.name) ||
  resource.id;

const shareReportedState = (
  resource: Resource,
  share: ResourceTrueNASShareMeta | undefined,
): string => {
  if (share?.locked) return 'Locked';
  if (share?.enabled === false || resource.status === 'offline' || resource.status === 'stopped') {
    return 'Disabled';
  }
  if (share?.enabled === true || resource.status === 'online' || resource.status === 'running') {
    return 'Active';
  }
  return 'Attention';
};

// Columns a user can sort by. Access and Clients summarize several flags or
// endpoints at once, so they carry no single scalar to order on.
const TRUENAS_SHARE_SORT_KEYS = ['share', 'protocol', 'path', 'state'] as const;

type TrueNASShareSortKey = (typeof TRUENAS_SHARE_SORT_KEYS)[number];

const getTrueNASShareSortValue = (
  resource: Resource,
  key: TrueNASShareSortKey,
): PlatformTableSortValue => {
  const share = shareMeta(resource);
  switch (key) {
    case 'share':
      return shareName(resource, share);
    case 'protocol': {
      const protocol = formatProtocol(share);
      return protocol === '-' ? null : protocol;
    }
    case 'path':
      return asTrimmedString(share?.path) || null;
    case 'state':
      return formatPlatformTableTitleCaseValue(mapTrueNASShareStatus(resource));
    default:
      key satisfies never;
      return null;
  }
};

export const TrueNASNetworkSharesTable: Component<{
  shares: Resource[];
  scope: Resource[];
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
  showToolbar?: boolean;
  /** The page's URL search, when it owns one; the table then reads and writes it. */
  search?: () => string;
  onSearchChange?: (value: string) => void;
}> = (props) => {
  const tableState = createPlatformTableFilterState({
    resources: () => props.shares,
    initialStatus: 'all' as TrueNASShareStatusFilter,
    filter: filterTrueNASShares,
    externalSearch: props.search,
    onExternalSearchChange: props.onSearchChange,
  });
  const drawer = createTrueNASDetailState('truenas-share-drawer');
  const resolveResourceLabel = createPlatformResourceLabelResolver(() => props.scope);
  const sort = createPlatformTableSortState({
    storageKey: 'truenasShares',
    sortKeys: TRUENAS_SHARE_SORT_KEYS,
  });
  const sortedRows = createMemo(() =>
    sort.sortRows(tableState.filtered(), getTrueNASShareSortValue),
  );

  return (
    <TrueNASTableFrame
      hasItems={props.shares.length > 0}
      tableState={tableState}
      common={props}
      searchPlaceholder="Search TrueNAS shares"
      statusOptions={TRUENAS_SHARE_STATUS_OPTIONS}
      rowNoun="shares"
      title="Network Shares"
      tableClass="min-w-full table-fixed text-xs md:min-w-[960px]"
      header={
        <>
          <PlatformSortableTableHead
            kind="name"
            sort={sort}
            sortKey="share"
            class="platform-table-mobile-w-30 md:w-[23%]"
          >
            Share
          </PlatformSortableTableHead>
          <PlatformSortableTableHead
            kind="text"
            sort={sort}
            sortKey="protocol"
            class="platform-table-mobile-w-15 md:w-[11%]"
          >
            <PlatformResponsiveTableLabel compact="Proto" full="Protocol" />
          </PlatformSortableTableHead>
          <PlatformSortableTableHead
            kind="text"
            sort={sort}
            sortKey="path"
            class="platform-table-mobile-w-25 md:w-[23%]"
          >
            Path
          </PlatformSortableTableHead>
          <PlatformSortableTableHead
            kind="text"
            sort={sort}
            class="platform-table-phone-hidden md:w-[16%]"
          >
            <PlatformResponsiveTableLabel compact="ACL" full="Access" />
          </PlatformSortableTableHead>
          <PlatformSortableTableHead
            kind="text"
            sort={sort}
            class="hidden lg:table-cell md:w-[13%]"
          >
            Clients
          </PlatformSortableTableHead>
          <PlatformSortableTableHead
            kind="badge"
            sort={sort}
            sortKey="state"
            class="platform-table-phone-hidden md:w-[14%]"
          >
            State
          </PlatformSortableTableHead>
        </>
      }
      body={
        <>
          <PlatformWindowedRows items={sortedRows} estimatedRowHeight={32}>
            {(resource) => {
              const share = () => shareMeta(resource);
              const name = () => shareName(resource, share());
              const displayStatus = () => getTrueNASResourceDisplayStatus(resource);
              const staleNotice = () => getTrueNASStaleNotice(resource);
              const indicator = () =>
                share()?.locked && !staleNotice()
                  ? { variant: 'warning' as const, label: 'Locked' }
                  : getSimpleStatusIndicator(displayStatus());
              const access = createMemo(() => formatAccess(share()));
              const clients = createMemo(() => formatClients(share()));
              const security = createMemo(() => formatSecurity(share()));
              const stateLabel = () => shareReportedState(resource, share());
              const showPhoneState = () =>
                Boolean(staleNotice()) || isPlatformAttentionVariant(indicator().variant);
              const phoneStateLabel = () => {
                const stale = staleNotice();
                if (!stale) return stateLabel();
                return share()?.locked ? `Locked · ${stale.badge}` : stale.badge;
              };
              const stateTitle = () =>
                staleNotice()?.detail ??
                (share()?.locked ? 'Encrypted dataset is locked' : undefined);
              const isExpanded = () => drawer.isExpanded(resource);
              return (
                <>
                  <TableRow
                    class={`${getPlatformResourceDetailRowClass(isExpanded())} text-[11px] sm:text-xs`}
                    data-truenas-share-row={resource.id}
                    onClick={() => drawer.toggle(resource)}
                  >
                    <TableCell class={getPlatformTableCellClassForKind('name')}>
                      <TrueNASRowLead
                        item={resource}
                        drawer={drawer}
                        label={name()}
                        variant={indicator().variant}
                        dotTitle={stateTitle() ?? indicator().label}
                      >
                        <div
                          class={
                            showPhoneState()
                              ? 'min-w-0 flex-1 sm:flex sm:items-center sm:gap-1.5'
                              : 'flex min-w-0 flex-1 items-center gap-1.5'
                          }
                        >
                          <div
                            class="min-w-0 truncate font-medium text-base-content"
                            title={[
                              name(),
                              share()?.dataset || share()?.path || resource.parentName || 'TrueNAS',
                              stateTitle(),
                            ]
                              .filter(Boolean)
                              .join(' · ')}
                          >
                            {name()}
                          </div>
                          {/* A phone hides State; an attention or stale share stacks
                                    its cue under the name so neither label clips. */}
                          <span
                            class={`platform-table-phone-only-inline text-[10px] leading-tight ${getPlatformInlineStateClass(showPhoneState())} ${getPlatformStatusTextToneClass(indicator().variant)}`}
                            title={stateTitle()}
                            data-truenas-share-state-phone={indicator().variant}
                          >
                            <span class="min-w-0 truncate">{phoneStateLabel()}</span>
                          </span>
                        </div>
                      </TrueNASRowLead>
                    </TableCell>
                    <TableCell class={getPlatformTableCellClassForKind('text')}>
                      <span class="font-mono text-[11px] text-base-content">
                        {formatProtocol(share())}
                      </span>
                    </TableCell>
                    <TableCell
                      class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                    >
                      <span class="block truncate font-mono text-[11px]" title={share()?.path}>
                        {share()?.path || '-'}
                      </span>
                    </TableCell>
                    <TableCell
                      class={`${getPlatformTableCellClassForKind('text')} platform-table-phone-hidden text-base-content`}
                      title={[access().title, security().title].filter(Boolean).join(' | ')}
                    >
                      <span class="block truncate">{access().label}</span>
                    </TableCell>
                    <TableCell
                      class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content lg:table-cell`}
                      title={clients().title}
                    >
                      <span class="block truncate">{clients().label}</span>
                    </TableCell>
                    <TableCell
                      class={`${getPlatformTableCellClassForKind('badge')} platform-table-phone-hidden`}
                    >
                      <TrueNASStateNoteDetails
                        note={
                          staleNotice()
                            ? ({
                                short: 'not updated',
                                full: staleNotice()!.detail,
                                tone: 'warning',
                              } satisfies TrueNASStateNote)
                            : null
                        }
                        title={stateTitle()}
                        label={stateLabel()}
                        labelClass={`shrink-0 text-[11px] font-medium ${
                          staleNotice() && !share()?.locked
                            ? 'text-base-content'
                            : getPlatformStatusTextToneClass(indicator().variant) ||
                              'text-base-content'
                        }`}
                        dataAttr="share"
                        shareVariant={indicator().variant}
                      />
                    </TableCell>
                  </TableRow>
                  <TrueNASResourceDetailRow
                    resource={resource}
                    indicator={indicator()}
                    drawer={drawer}
                    colSpan={6}
                    resolveResourceLabel={resolveResourceLabel}
                  />
                </>
              );
            }}
          </PlatformWindowedRows>
        </>
      }
    />
  );
};

export default TrueNASNetworkSharesTable;
