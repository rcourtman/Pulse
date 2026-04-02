import { createMemo, Index, Show } from 'solid-js';

import { ComponentErrorBoundary } from '@/components/ErrorBoundary';
import { NodeGroupHeader } from '@/components/shared/NodeGroupHeader';
import { TableBody, TableCell, TableRow } from '@/components/shared/Table';
import { getAlertStyles } from '@/utils/alerts';
import { isNodeOnline } from '@/utils/status';
import { getCanonicalWorkloadId } from '@/utils/workloads';
import type { SummarySeriesGroupScope } from '@/components/shared/summaryCardInteraction';

import { GuestDrawer } from './GuestDrawer';
import { GuestRow } from './GuestRow';
import { buildWorkloadSummaryGroupScope } from './workloadSelectors';
import type { DashboardState } from './useDashboardState';

type WorkloadPanelProps = Pick<
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
  | 'handleTagClick'
  | 'activeSummaryWorkloadGroupScope'
  | 'activeSummaryWorkloadId'
  | 'focusedSummaryWorkloadGroupId'
  | 'mobileVisibleColumnIds'
  | 'nodeByInstance'
  | 'search'
  | 'selectedGuestId'
  | 'setFocusedWorkloadGroupScope'
  | 'setHoveredWorkloadGroupScope'
  | 'setHoveredWorkloadId'
  | 'setSelectedGuestId'
  | 'setTableBodyRef'
  | 'topSpacerHeight'
  | 'totalColumns'
  | 'visibleGroupKeys'
  | 'windowedGroupedGuests'
  | 'workloadIOEmphasis'
>;

export function WorkloadPanel(props: WorkloadPanelProps) {
  return (
    <TableBody ref={props.setTableBodyRef} class="divide-y divide-border">
      <Show when={props.groupedWindowing.isWindowed() && props.topSpacerHeight() > 0}>
        <TableRow aria-hidden="true">
          <TableCell
            colspan={props.totalColumns()}
            style={{ height: `${props.topSpacerHeight()}px`, padding: '0', border: '0' }}
          />
        </TableRow>
      </Show>
      <Index each={props.visibleGroupKeys()} fallback={<></>}>
        {(groupKey) => {
          const groupGuests = () => props.windowedGroupedGuests()[groupKey()] || [];
          const fullGroupGuests = () => props.groupedGuests()[groupKey()] || [];
          const node = () => props.nodeByInstance()[groupKey()];
          const groupSummaryScope = createMemo<SummarySeriesGroupScope | null>(() => {
            if (props.groupingMode() !== 'grouped') {
              return null;
            }
            return buildWorkloadSummaryGroupScope(
              groupKey(),
              fullGroupGuests(),
              props.getGroupLabel(groupKey(), fullGroupGuests()),
            );
          });
          const isSummaryGroupHighlighted = createMemo(
            () => props.activeSummaryWorkloadGroupScope()?.id === groupKey(),
          );
          const handleGroupHoverChange = (next: SummarySeriesGroupScope | null) => {
            props.setHoveredWorkloadGroupScope(next);
          };
          const handleGroupFocusToggle = () => {
            const scope = groupSummaryScope();
            props.setFocusedWorkloadGroupScope(
              scope && props.focusedSummaryWorkloadGroupId() === scope.id ? null : scope,
            );
          };

          return (
            <>
              <Show when={props.groupingMode() === 'grouped'}>
                <Show
                  when={node()}
                  fallback={
                    <TableRow
                      class="bg-surface-alt transition-colors duration-150 hover:bg-surface-hover"
                      data-summary-group-id={groupKey()}
                      data-summary-group-series-count={String(groupSummaryScope()?.seriesIds.length ?? 0)}
                      data-summary-row-active={isSummaryGroupHighlighted() ? 'true' : 'false'}
                      aria-pressed={props.focusedSummaryWorkloadGroupId() === groupSummaryScope()?.id}
                      onClick={handleGroupFocusToggle}
                      onMouseEnter={() => handleGroupHoverChange(groupSummaryScope())}
                      onMouseLeave={() => handleGroupHoverChange(null)}
                    >
                      <TableCell
                        colspan={props.totalColumns()}
                        class="py-0.5 pr-1.5 sm:pr-2 pl-2 sm:pl-3 text-[12px] sm:text-sm font-semibold text-base-content"
                      >
                        {(() => {
                          const label = props.getGroupLabel(groupKey(), fullGroupGuests());
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
                    trClass="transition-colors duration-150 hover:bg-surface-hover"
                    trProps={{
                      'data-summary-group-id': groupKey(),
                      'data-summary-group-series-count': String(
                        groupSummaryScope()?.seriesIds.length ?? 0,
                      ),
                      'data-summary-row-active': isSummaryGroupHighlighted() ? 'true' : 'false',
                      'aria-pressed': props.focusedSummaryWorkloadGroupId() === groupSummaryScope()?.id,
                      onClick: handleGroupFocusToggle,
                      onMouseEnter: () => handleGroupHoverChange(groupSummaryScope()),
                      onMouseLeave: () => handleGroupHoverChange(null),
                    }}
                  />
                </Show>
              </Show>
              <Index each={groupGuests()} fallback={<></>}>
                {(guest) => {
                  const guestId = createMemo(() => getCanonicalWorkloadId(guest()));
                  const metadata = () =>
                    props.guestMetadata()[guestId()] ||
                    props.guestMetadata()[`${guest().instance}:${guest().node}:${guest().vmid}`];
                  const parentNode = () => node() ?? props.guestParentNodeMap()[guestId()];
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
                        isSummaryHighlighted={props.activeSummaryWorkloadId() === guestId()}
                        ioEmphasis={props.workloadIOEmphasis()}
                        onHoverChange={props.setHoveredWorkloadId}
                      />
                      <Show when={props.selectedGuestId() === guestId()}>
                        <TableRow data-inline-detail-for={guestId()}>
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
      </Index>
      <Show when={props.groupedWindowing.isWindowed() && props.bottomSpacerHeight() > 0}>
        <TableRow aria-hidden="true">
          <TableCell
            colspan={props.totalColumns()}
            style={{ height: `${props.bottomSpacerHeight()}px`, padding: '0', border: '0' }}
          />
        </TableRow>
      </Show>
    </TableBody>
  );
}
