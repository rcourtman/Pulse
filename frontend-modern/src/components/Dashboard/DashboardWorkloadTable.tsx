import { createMemo, For, Index, Show } from 'solid-js';

import { ComponentErrorBoundary } from '@/components/ErrorBoundary';
import { Card } from '@/components/shared/Card';
import { NodeGroupHeader } from '@/components/shared/NodeGroupHeader';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/shared/Table';
import { getAlertStyles } from '@/utils/alerts';
import { isNodeOnline } from '@/utils/status';
import { getCanonicalWorkloadId } from '@/utils/workloads';

import { GuestDrawer } from './GuestDrawer';
import { GuestRow } from './GuestRow';
import type { DashboardState, WorkloadSortKey } from './useDashboardState';

type DashboardWorkloadTableProps = Pick<
  DashboardState,
  | 'activeAlerts'
  | 'alertsEnabled'
  | 'bottomSpacerHeight'
  | 'getGroupLabel'
  | 'groupedGuests'
  | 'groupedWindowing'
  | 'guestMetadata'
  | 'guestParentNodeMap'
  | 'groupingMode'
  | 'handleCustomUrlUpdate'
  | 'handleSort'
  | 'handleTagClick'
  | 'isMobile'
  | 'mobileVisibleColumnIds'
  | 'mobileVisibleColumns'
  | 'nodeByInstance'
  | 'search'
  | 'selectedGuestId'
  | 'setHoveredWorkloadId'
  | 'setSelectedGuestId'
  | 'setTableBodyRef'
  | 'setTableWrapperRef'
  | 'sortDirection'
  | 'sortKey'
  | 'topSpacerHeight'
  | 'totalColumns'
  | 'visibleColumns'
  | 'visibleGroupKeys'
  | 'windowedGroupedGuests'
  | 'workloadIOEmphasis'
>;

export function DashboardWorkloadTable(props: DashboardWorkloadTableProps) {
  return (
    <ComponentErrorBoundary name="Guest Table">
      <Card padding="none" tone="card" class="mb-4 rounded-md">
        <div class="border-b border-border bg-surface-hover px-3 py-2 text-[11px] font-semibold uppercase tracking-wide text-muted">
          Workloads
        </div>
        <div class="overflow-x-auto">
          <Table
            wrapperRef={props.setTableWrapperRef}
            class="whitespace-nowrap min-w-[max-content]"
            style={{
              'table-layout': 'fixed',
              'min-width': props.isMobile() ? '100%' : 'max-content',
            }}
          >
            <TableHeader>
              <TableRow class="bg-surface-alt text-muted border-b border-border">
                <For each={props.mobileVisibleColumns()}>
                  {(col) => {
                    const isFirst = () => col.id === props.visibleColumns()[0]?.id;
                    const sortKeyForCol = col.sortKey as WorkloadSortKey | undefined;
                    const isSortable = !!sortKeyForCol;
                    const isSorted = () => sortKeyForCol && props.sortKey() === sortKeyForCol;

                    return (
                      <TableHead
                        class={`py-0.5 text-[11px] sm:text-xs font-medium uppercase tracking-wider whitespace-nowrap
 ${isFirst() ? 'pl-2 sm:pl-3 pr-1.5 sm:pr-2 text-left' : 'px-1.5 sm:px-2 text-center'}
 ${isSortable ? 'cursor-pointer hover:bg-surface-hover' : ''}`}
                        style={{
                          ...(['cpu', 'memory', 'disk'].includes(col.id)
                            ? { width: props.isMobile() ? '70px' : '140px' }
                            : ['netIo', 'diskIo'].includes(col.id)
                              ? { width: '170px' }
                              : props.isMobile() && col.id === 'name'
                                ? { width: '100%', 'min-width': '120px' }
                                : col.width
                                  ? { width: col.width }
                                  : {}),
                          'vertical-align': 'middle',
                        }}
                        onClick={() => isSortable && props.handleSort(sortKeyForCol!)}
                        title={col.icon ? col.label : undefined}
                      >
                        <div
                          class={`flex items-center gap-0.5 ${isFirst() ? 'justify-start' : 'justify-center'}`}
                          style={{ 'min-height': '14px' }}
                        >
                          {col.icon ? (
                            <span class="flex items-center">{col.icon}</span>
                          ) : (
                            col.label
                          )}
                          {isSorted() && (props.sortDirection() === 'asc' ? ' ▲' : ' ▼')}
                        </div>
                      </TableHead>
                    );
                  }}
                </For>
              </TableRow>
            </TableHeader>
            <TableBody ref={props.setTableBodyRef} class="divide-y divide-border">
              <Show when={props.groupedWindowing.isWindowed() && props.topSpacerHeight() > 0}>
                <TableRow aria-hidden="true">
                  <TableCell
                    colspan={props.totalColumns()}
                    style={{ height: `${props.topSpacerHeight()}px`, padding: '0', border: '0' }}
                  />
                </TableRow>
              </Show>
              <For each={props.visibleGroupKeys()} fallback={<></>}>
                {(groupKey) => {
                  const groupGuests = () => props.windowedGroupedGuests()[groupKey] || [];
                  const fullGroupGuests = () => props.groupedGuests()[groupKey] || [];
                  const node = () => props.nodeByInstance()[groupKey];

                  return (
                    <>
                      <Show when={props.groupingMode() === 'grouped'}>
                        <Show
                          when={node()}
                          fallback={
                            <TableRow class="bg-surface-alt">
                              <TableCell
                                colspan={props.totalColumns()}
                                class="py-0.5 pr-1.5 sm:pr-2 pl-2 sm:pl-3 text-[12px] sm:text-sm font-semibold text-base-content"
                              >
                                {(() => {
                                  const label = props.getGroupLabel(groupKey, fullGroupGuests());
                                  return (
                                    <div class="flex items-center gap-3">
                                      <span>{label.name}</span>
                                      <Show when={label.type}>
                                        <span class="inline-flex items-center rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300">
                                          {label.type}
                                        </span>
                                      </Show>
                                    </div>
                                  );
                                })()}
                              </TableCell>
                            </TableRow>
                          }
                        >
                          <NodeGroupHeader
                            node={node()!}
                            renderAs="tr"
                            colspan={props.totalColumns()}
                          />
                        </Show>
                      </Show>
                      <Index each={groupGuests()} fallback={<></>}>
                        {(guest) => {
                          const guestId = createMemo(() => getCanonicalWorkloadId(guest()));
                          const metadata = () =>
                            props.guestMetadata()[guestId()] ||
                            props.guestMetadata()[
                              `${guest().instance}:${guest().node}:${guest().vmid}`
                            ];
                          const parentNode = () =>
                            node() ?? props.guestParentNodeMap()[guestId()];
                          const parentNodeOnline = () => {
                            const pn = parentNode();
                            return pn ? isNodeOnline(pn) : true;
                          };

                          return (
                            <ComponentErrorBoundary name="GuestRow">
                              <GuestRow
                                guest={guest()}
                                alertStyles={getAlertStyles(
                                  guestId(),
                                  props.activeAlerts,
                                  props.alertsEnabled(),
                                )}
                                customUrl={metadata()?.customUrl}
                                onTagClick={props.handleTagClick}
                                activeSearch={props.search()}
                                parentNodeOnline={parentNodeOnline()}
                                onCustomUrlUpdate={props.handleCustomUrlUpdate}
                                isGroupedView={props.groupingMode() === 'grouped'}
                                visibleColumnIds={props.mobileVisibleColumnIds()}
                                onClick={() =>
                                  props.setSelectedGuestId(
                                    props.selectedGuestId() === guestId() ? null : guestId(),
                                  )
                                }
                                isExpanded={props.selectedGuestId() === guestId()}
                                ioEmphasis={props.workloadIOEmphasis()}
                                onHoverChange={props.setHoveredWorkloadId}
                              />
                              <Show when={props.selectedGuestId() === guestId()}>
                                <TableRow>
                                  <TableCell
                                    colspan={props.totalColumns()}
                                    class="p-0 border-b border-border bg-surface-alt"
                                  >
                                    <div
                                      class="px-2 sm:px-4 py-3 sm:py-4"
                                      onClick={(event) => event.stopPropagation()}
                                    >
                                      <GuestDrawer
                                        guest={guest()}
                                        onClose={() => props.setSelectedGuestId(null)}
                                        customUrl={metadata()?.customUrl}
                                        onCustomUrlChange={props.handleCustomUrlUpdate}
                                      />
                                    </div>
                                  </TableCell>
                                </TableRow>
                              </Show>
                            </ComponentErrorBoundary>
                          );
                        }}
                      </Index>
                    </>
                  );
                }}
              </For>
              <Show when={props.groupedWindowing.isWindowed() && props.bottomSpacerHeight() > 0}>
                <TableRow aria-hidden="true">
                  <TableCell
                    colspan={props.totalColumns()}
                    style={{ height: `${props.bottomSpacerHeight()}px`, padding: '0', border: '0' }}
                  />
                </TableRow>
              </Show>
            </TableBody>
          </Table>
        </div>
      </Card>
    </ComponentErrorBoundary>
  );
}
