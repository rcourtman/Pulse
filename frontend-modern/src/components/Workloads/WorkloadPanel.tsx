import { createMemo, For, Show } from 'solid-js';

import { ComponentErrorBoundary } from '@/components/ErrorBoundary';
import {
  GROUPED_TABLE_ROW_BADGE_CLASS,
  getGroupedTableRowCellClass,
  getGroupedTableRowClass,
} from '@/components/shared/groupedTableRowPresentation';
import { InlineDetailTableRow } from '@/components/shared/InlineDetailTableRow';
import { NodeGroupHeader } from '@/components/shared/NodeGroupHeader';
import { buildSummaryDisclosureControlsId } from '@/components/shared/summaryInteractionA11y';
import { TableBody, TableCell, TableRow } from '@/components/shared/Table';
import { getAlertsForResource, getAlertStyles } from '@/utils/alerts';
import { guestOverrideIdCandidates } from '@/features/alerts/guestOverrideIdentity';
import { isNodeOnline } from '@/utils/status';
import { getCanonicalWorkloadId, getWorkloadMetadataIdCandidates } from '@/utils/workloads';

import { GuestDrawer } from './GuestDrawer';
import { GuestRow } from './GuestRow';
import { getWorkloadGuestMetadataRecord } from './workloadGuestMetadataRecord';
import type { WorkloadsState } from './useWorkloadsState';

type WorkloadPanelProps = Pick<
  WorkloadsState,
  | 'activeAlerts'
  | 'alertsEnabled'
  | 'bottomSpacerHeight'
  | 'getGroupLabel'
  | 'groupedGuests'
  | 'groupedWindowing'
  | 'groupLabelBadges'
  | 'guestMetadata'
  | 'guestParentNodeMap'
  | 'groupingMode'
  | 'handleCustomUrlUpdate'
  | 'handleTagClick'
  | 'activeSummaryWorkloadId'
  | 'nestedWorkloadContextByGuestId'
  | 'nodeByInstance'
  | 'search'
  | 'selectedGuestId'
  | 'setHoveredWorkloadId'
  | 'setSelectedGuestId'
  | 'setTableBodyRef'
  | 'topSpacerHeight'
  | 'totalColumns'
  | 'visibleGroupKeys'
  | 'windowedGroupedGuests'
  | 'workloadIOEmphasis'
  | 'workloadMetricDisplayMode'
  | 'workloadMetricHoverMode'
  | 'workloadMemoryDisplayBasis'
  | 'workloadMetricHistory'
  | 'workloadTableLayoutMode'
  | 'workloadTableVisibleColumnIds'
  | 'workloadColumnWidths'
>;

export function WorkloadPanel(props: WorkloadPanelProps) {
  return (
    <TableBody ref={props.setTableBodyRef} class="divide-y divide-border">
      <Show when={props.groupedWindowing.isWindowed() && props.topSpacerHeight() > 0}>
        <TableRow aria-hidden="true" class="h-0 border-0!">
          <TableCell colspan={props.totalColumns()} class="h-0 p-0! border-0! leading-0">
            <svg
              aria-hidden="true"
              width="1"
              height={String(props.topSpacerHeight())}
              class="block w-px pointer-events-none"
            />
          </TableCell>
        </TableRow>
      </Show>
      <For each={props.visibleGroupKeys()} fallback={<></>}>
        {(groupKey) => {
          const groupGuests = () => props.windowedGroupedGuests()[groupKey] || [];
          const fullGroupGuests = () => props.groupedGuests()[groupKey] || [];
          // Build the id lookup from the full group, not the windowed slice:
          // it only rebuilds when the group's data changes, so a runway top-up
          // leaves every mounted row's guest() dependency untouched instead of
          // revalidating ~140 rows' memo chains per scroll event.
          const groupGuestById = createMemo(
            () => new Map(fullGroupGuests().map((guest) => [getCanonicalWorkloadId(guest), guest])),
          );
          const groupGuestIds = createMemo(() => groupGuests().map(getCanonicalWorkloadId));
          const node = () => props.nodeByInstance()[groupKey];

          return (
            <>
              {/* Group rows are identity-only dividers: host details open from the
                  platform page's own hosts table. data-summary-group-id keeps a click
                  on a divider from clearing an open guest drawer. */}
              <Show
                when={
                  props.groupingMode() === 'grouped' && groupGuests()[0] === fullGroupGuests()[0]
                }
              >
                <Show
                  when={node()}
                  fallback={
                    <TableRow class={getGroupedTableRowClass()} data-summary-group-id={groupKey}>
                      <TableCell
                        colspan={props.totalColumns()}
                        class={getGroupedTableRowCellClass()}
                      >
                        {(() => {
                          const label = props.getGroupLabel(groupKey, fullGroupGuests());
                          const badges = props.groupLabelBadges();
                          const badge = badges[groupKey] ?? badges[groupKey.toLowerCase()];
                          const badgeLabel = badge?.label || label.type;
                          const badgeClass = badge?.classes || GROUPED_TABLE_ROW_BADGE_CLASS;
                          return (
                            <div class="flex items-center gap-3">
                              <span>{label.name}</span>
                              <Show when={badgeLabel}>
                                <span class={badgeClass} title={badge?.title ?? badgeLabel}>
                                  {badgeLabel}
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
                    trClass="select-none duration-150 [&>td>div]:flex-nowrap"
                    trProps={{ 'data-summary-group-id': groupKey }}
                  />
                </Show>
              </Show>
              <For each={groupGuestIds()} fallback={<></>}>
                {(keyedGuestId) => {
                  const guest = () => groupGuestById().get(keyedGuestId)!;
                  const guestId = () => keyedGuestId;
                  const metadataIdCandidates = createMemo(() =>
                    getWorkloadMetadataIdCandidates(guest()),
                  );
                  const detailControlsId = createMemo(() =>
                    buildSummaryDisclosureControlsId(guestId()),
                  );
                  const metadata = () =>
                    getWorkloadGuestMetadataRecord(guest(), props.guestMetadata());
                  const parentNode = () => node() ?? props.guestParentNodeMap()[guestId()];
                  const parentNodeOnline = () => {
                    const pn = parentNode();
                    return pn ? isNodeOnline(pn) : true;
                  };
                  const nestedWorkloadContext = () =>
                    props.nestedWorkloadContextByGuestId()[guestId()];

                  return (
                    <ComponentErrorBoundary name="GuestRow">
                      <GuestRow
                        guest={guest()}
                        alertStyles={getAlertStyles(
                          [guestId(), ...(guest().alertResourceIds ?? [])],
                          props.activeAlerts,
                          props.alertsEnabled(),
                        )}
                        customUrl={metadata()?.customUrl}
                        onTagClick={props.handleTagClick}
                        activeSearch={props.search()}
                        parentNodeOnline={parentNodeOnline()}
                        parentMemoryTotal={parentNode()?.memory?.total}
                        parentNodeName={parentNode()?.name}
                        onCustomUrlUpdate={props.handleCustomUrlUpdate}
                        isGroupedView={props.groupingMode() === 'grouped'}
                        visibleColumnIds={props.workloadTableVisibleColumnIds()}
                        columnWidths={props.workloadColumnWidths()}
                        workloadTableLayoutMode={props.workloadTableLayoutMode()}
                        onClick={() =>
                          props.setSelectedGuestId(
                            props.selectedGuestId() === guestId() ? null : guestId(),
                          )
                        }
                        isExpanded={props.selectedGuestId() === guestId()}
                        isSummaryHighlighted={props.activeSummaryWorkloadId() === guestId()}
                        ioEmphasis={props.workloadIOEmphasis()}
                        metricDisplayMode={props.workloadMetricDisplayMode()}
                        metricHoverMode={props.workloadMetricHoverMode()}
                        memoryDisplayBasis={props.workloadMemoryDisplayBasis()}
                        metricHistory={props.workloadMetricHistory}
                        nestedWorkloadContext={nestedWorkloadContext()}
                        onHoverChange={props.setHoveredWorkloadId}
                      />
                      <Show when={props.selectedGuestId() === guestId()}>
                        <InlineDetailTableRow
                          cellId={detailControlsId()}
                          colspan={props.totalColumns()}
                          data-inline-detail-for={guestId()}
                        >
                          <GuestDrawer
                            guest={guest()}
                            onClose={() => props.setSelectedGuestId(null)}
                            metadataId={metadataIdCandidates()[0] || guestId()}
                            customUrl={metadata()?.customUrl}
                            nestedWorkloadContext={nestedWorkloadContext()}
                            onCustomUrlChange={props.handleCustomUrlUpdate}
                            parentNodeOnline={parentNodeOnline()}
                            parentMemoryTotal={parentNode()?.memory?.total}
                            memoryDisplayBasis={props.workloadMemoryDisplayBasis()}
                            alerts={getAlertsForResource(
                              [
                                guestId(),
                                ...(guest().alertResourceIds ?? []),
                                ...guestOverrideIdCandidates(guest()),
                              ],
                              props.activeAlerts,
                              props.alertsEnabled(),
                            )}
                          />
                        </InlineDetailTableRow>
                      </Show>
                    </ComponentErrorBoundary>
                  );
                }}
              </For>
            </>
          );
        }}
      </For>
      <Show when={props.groupedWindowing.isWindowed() && props.bottomSpacerHeight() > 0}>
        <TableRow aria-hidden="true" class="h-0 border-0!">
          <TableCell colspan={props.totalColumns()} class="h-0 p-0! border-0! leading-0">
            <svg
              aria-hidden="true"
              width="1"
              height={String(props.bottomSpacerHeight())}
              class="block w-px pointer-events-none"
            />
          </TableCell>
        </TableRow>
      </Show>
    </TableBody>
  );
}
