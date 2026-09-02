import { For, Show, createMemo, type Component } from 'solid-js';
import { InlineDetailTableRow } from '@/components/shared/InlineDetailTableRow';
import { StatusDot } from '@/components/shared/StatusDot';
import { TableCell, TableHead, TableRow } from '@/components/shared/Table';
import { getSimpleStatusIndicator } from '@/utils/status';
import { asTrimmedString } from '@/utils/stringUtils';
import {
  PLATFORM_HEALTH_FILTER_OPTIONS,
  PlatformTableNumberValue,
  PlatformTableToolbar,
  createPlatformTableFilterState,
  filterPlatformResources,
  formatPlatformTableIntegerValue,
  formatPlatformTableUptimeValue,
  getPlatformTableCellClassForKind,
  getPlatformTableContainerLayout,
  getPlatformTableHeadClassForKind,
  PlatformResponsiveTableLabel,
  type PlatformResourceStatusFilter,
  PlatformTableEmptyState,
  PlatformTableShell,
  PlatformWindowedRows,
  withPlatformStatusCounts,
} from '@/features/platformPage/sharedPlatformPage';
import { useObservedElementWidth } from '@/hooks/useObservedElementWidth';
import {
  createPlatformResourceDetailState,
  getPlatformResourceDetailRowInteractionProps,
  PlatformResourceDetailToggleButton,
} from '@/features/platformPage/PlatformResourceDetailTableRow';
import type { Resource } from '@/types/resource';
import { ProxmoxMailGatewayDrawer } from './ProxmoxMailGatewayDrawer';

export type MailGatewayPhoneColumn =
  'instance' | 'nodes' | 'uptime' | 'mail' | 'queue' | 'deferred';

export const MAIL_GATEWAY_PHONE_COLUMNS: readonly MailGatewayPhoneColumn[] = [
  'instance',
  'nodes',
  'uptime',
  'mail',
  'queue',
  'deferred',
];

export const MAIL_GATEWAY_NARROW_PHONE_COLUMNS: readonly MailGatewayPhoneColumn[] = [
  'instance',
  'uptime',
  'mail',
  'queue',
  'deferred',
];

export const MAIL_GATEWAY_PHONE_COLUMN_WIDTHS: Readonly<Record<MailGatewayPhoneColumn, number>> = {
  instance: 30,
  nodes: 12,
  uptime: 16,
  mail: 14,
  queue: 14,
  deferred: 14,
};

export const MAIL_GATEWAY_NARROW_PHONE_COLUMN_WIDTHS: Readonly<
  Record<MailGatewayPhoneColumn, number>
> = {
  instance: 40,
  nodes: 0,
  uptime: 15,
  mail: 15,
  queue: 15,
  deferred: 15,
};

// Proxmox Mail Gateway instances are mail-flow / quarantine appliances.
// The generic infrastructure table renders dashes for Disk I/O / Uptime
// / Temperature (PMG only exposes uptime, which we project now) and
// omits the queue / spam / virus / quarantine counts that are the
// operator columns. This bespoke table reuses canonical shared
// primitives and surfaces those PMG-native columns.

export const ProxmoxMailGatewayTable: Component<{
  resources: Resource[];
  emptyTitle: string;
  emptyDescription: string;
}> = (props) => {
  const tableState = createPlatformTableFilterState({
    resources: () => props.resources,
    initialStatus: 'all' as PlatformResourceStatusFilter,
    filter: filterPlatformResources,
  });
  const detail = createPlatformResourceDetailState({ idPrefix: 'proxmox-mail-gateway-detail' });
  const observedWidth = useObservedElementWidth();
  const layout = createMemo(() =>
    getPlatformTableContainerLayout(observedWidth.width() ?? 1920, [520, 720, 960, 1200]),
  );
  const isNarrowPhone = createMemo(() => {
    const width = observedWidth.width();
    return typeof width === 'number' && width > 0 && width < 360;
  });
  // Uptime and mail-flow counters remain in the narrow projection; node count
  // moves into the existing instance detail expansion below 360px.
  const showNodes = createMemo(() => !isNarrowPhone());
  const showUptime = createMemo(() => true);
  const showOperational = createMemo(() => ['operational', 'expanded', 'full'].includes(layout()));
  const showVirus = createMemo(() => ['expanded', 'full'].includes(layout()));
  const showVersion = createMemo(() => layout() === 'full');
  const visibleColumnCount = createMemo(
    () =>
      4 +
      Number(showNodes()) +
      Number(showUptime()) +
      Number(showOperational()) * 2 +
      Number(showVirus()) +
      Number(showVersion()),
  );

  return (
    <Show
      when={props.resources.length > 0}
      fallback={
        <PlatformTableEmptyState title={props.emptyTitle} description={props.emptyDescription} />
      }
    >
      <div ref={observedWidth.setElement} class="space-y-3" data-proxmox-mail-layout={layout()}>
        <PlatformTableToolbar
          search={tableState.search}
          onSearchChange={tableState.setSearch}
          searchPlaceholder="Search Mail Gateways"
          searchSuggestions={tableState.searchSuggestions}
          status={tableState.status()}
          onStatusChange={tableState.setStatus}
          statusOptions={withPlatformStatusCounts(
            PLATFORM_HEALTH_FILTER_OPTIONS,
            tableState.countForStatus,
          )}
          visible={tableState.visible()}
          total={tableState.total()}
          rowNoun="instances"
        />

        <Show
          when={tableState.filtered().length > 0}
          fallback={
            <PlatformTableEmptyState
              title="No instances match current filters"
              description="Adjust the search or status filter to see more instances."
            />
          }
        >
          <PlatformTableShell
            tableClass="min-w-[0px] table-fixed text-xs"
            colgroup={
              <Show when={layout() === 'compact'}>
                <colgroup>
                  <For
                    each={
                      isNarrowPhone()
                        ? MAIL_GATEWAY_NARROW_PHONE_COLUMNS
                        : MAIL_GATEWAY_PHONE_COLUMNS
                    }
                  >
                    {(column) => (
                      <col
                        style={{
                          width: `${
                            (isNarrowPhone()
                              ? MAIL_GATEWAY_NARROW_PHONE_COLUMN_WIDTHS
                              : MAIL_GATEWAY_PHONE_COLUMN_WIDTHS)[column]
                          }%`,
                        }}
                        data-proxmox-mail-column={column}
                      />
                    )}
                  </For>
                </colgroup>
              </Show>
            }
            header={
              <>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('name')} platform-table-mobile-w-30 md:w-[18%]`}
                >
                  Instance
                </TableHead>
                <Show when={showVersion()}>
                  <TableHead class={getPlatformTableHeadClassForKind('text')}>Version</TableHead>
                </Show>
                <Show when={showNodes()}>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('numeric-value')} platform-table-mobile-w-15 md:w-[12%]`}
                  >
                    Nodes
                  </TableHead>
                </Show>
                <Show when={showUptime()}>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('numeric-value')} platform-table-mobile-w-15 md:w-[14%]`}
                  >
                    {layout() === 'compact' ? 'Age' : 'Uptime'}
                  </TableHead>
                </Show>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('numeric-value')} platform-table-mobile-w-15 md:w-[14%]`}
                >
                  <PlatformResponsiveTableLabel compact="In" full="Mail in" />
                </TableHead>
                <Show when={showOperational()}>
                  <TableHead class={getPlatformTableHeadClassForKind('numeric-value')}>
                    Spam
                  </TableHead>
                </Show>
                <Show when={showVirus()}>
                  <TableHead class={getPlatformTableHeadClassForKind('numeric-value')}>
                    Virus
                  </TableHead>
                </Show>
                <Show when={showOperational()}>
                  <TableHead class={getPlatformTableHeadClassForKind('numeric-value')}>
                    Quarantine
                  </TableHead>
                </Show>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('numeric-value')} platform-table-mobile-w-15 md:w-[14%]`}
                >
                  <PlatformResponsiveTableLabel compact="Q" full="Queue" />
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('numeric-value')} platform-table-mobile-w-10 md:w-[14%]`}
                >
                  <PlatformResponsiveTableLabel compact="Def" full="Deferred" />
                </TableHead>
              </>
            }
            body={
              <>
                <PlatformWindowedRows items={tableState.filtered} estimatedRowHeight={32}>
                  {(instance) => {
                    const pmg = () => instance.pmg;
                    const name = () => asTrimmedString(instance.name) || instance.id;
                    const version = () => asTrimmedString(pmg()?.version) || '—';
                    const indicator = () => getSimpleStatusIndicator(instance.status);
                    const isOpen = () => detail.isExpanded(instance);
                    const detailRowId = () => detail.detailRowId(instance);
                    return (
                      <>
                        <TableRow
                          {...getPlatformResourceDetailRowInteractionProps({
                            expanded: isOpen(),
                            onToggle: () => detail.toggle(instance),
                          })}
                        >
                          <TableCell class={getPlatformTableCellClassForKind('name')}>
                            <div class="flex items-center gap-2 min-w-0">
                              <PlatformResourceDetailToggleButton
                                expanded={isOpen()}
                                resourceLabel={name()}
                                controlsId={detailRowId()}
                                onToggle={() => detail.toggle(instance)}
                              />
                              <StatusDot
                                size="sm"
                                variant={indicator().variant}
                                title={instance.status || 'unknown'}
                                ariaHidden
                              />
                              <span class="font-semibold text-base-content truncate" title={name()}>
                                {name()}
                              </span>
                            </div>
                          </TableCell>
                          <Show when={showVersion()}>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('text')} text-base-content font-mono text-[11px]`}
                            >
                              {version()}
                            </TableCell>
                          </Show>
                          <Show when={showNodes()}>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                            >
                              <PlatformTableNumberValue
                                value={pmg()?.nodeCount}
                                format={formatPlatformTableIntegerValue}
                              />
                            </TableCell>
                          </Show>
                          <Show when={showUptime()}>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                            >
                              {formatPlatformTableUptimeValue(
                                instance.uptime ?? pmg()?.uptimeSeconds,
                              )}
                            </TableCell>
                          </Show>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                          >
                            <PlatformTableNumberValue
                              value={pmg()?.mailCountTotal}
                              format={formatPlatformTableIntegerValue}
                            />
                          </TableCell>
                          <Show when={showOperational()}>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                            >
                              <PlatformTableNumberValue
                                value={pmg()?.spamIn}
                                format={formatPlatformTableIntegerValue}
                              />
                            </TableCell>
                          </Show>
                          <Show when={showVirus()}>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                            >
                              <PlatformTableNumberValue
                                value={pmg()?.virusIn}
                                format={formatPlatformTableIntegerValue}
                              />
                            </TableCell>
                          </Show>
                          <Show when={showOperational()}>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                            >
                              <PlatformTableNumberValue
                                value={pmg()?.quarantine}
                                format={formatPlatformTableIntegerValue}
                              />
                            </TableCell>
                          </Show>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                          >
                            <PlatformTableNumberValue
                              value={pmg()?.queueTotal ?? pmg()?.queueActive}
                              format={formatPlatformTableIntegerValue}
                            />
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                          >
                            <PlatformTableNumberValue
                              value={pmg()?.queueDeferred}
                              format={formatPlatformTableIntegerValue}
                            />
                          </TableCell>
                        </TableRow>
                        <Show when={isOpen()}>
                          <InlineDetailTableRow
                            cellId={detailRowId()}
                            colspan={visibleColumnCount()}
                            contentClass="px-4 py-4"
                            data-inline-detail-for={instance.id}
                          >
                            <ProxmoxMailGatewayDrawer
                              instanceRow={instance}
                              onClose={() => detail.close(instance)}
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

export default ProxmoxMailGatewayTable;
