import { Show, createMemo, type Component, type JSX } from 'solid-js';
import { filterChipStatusDot } from '@/components/shared/FilterBar';
import { InlineDetailTableRow } from '@/components/shared/InlineDetailTableRow';
import { TableCell, TableHead, TableRow } from '@/components/shared/Table';
import { asTrimmedString } from '@/utils/stringUtils';
import {
  PlatformWindowedRows,
  PlatformResponsiveTableLabel,
  PlatformTableNumberValue,
  PlatformTableRelativeTimeValue,
  createPlatformTableFilterState,
  formatPlatformTableTextValue,
  getPlatformTableCellClassForKind,
  getPlatformTableHeadClassForKind,
  type PlatformTableFilterOption,
  PlatformTableShell,
} from '@/features/platformPage/sharedPlatformPage';
import {
  createPlatformResourceDetailState,
  getPlatformResourceDetailRowClass,
} from '@/features/platformPage/PlatformResourceDetailTableRow';
import type { Resource } from '@/types/resource';
import {
  compareKubernetesEvents,
  filterKubernetesEvents,
  mapKubernetesEventSeverity,
  type KubernetesEventSeverityFilter,
  isSingleKubernetesCluster,
} from './kubernetesPageModel';
import {
  KubernetesEventDetailPanel,
  KubernetesRowLead,
  KubernetesTableFrame,
  kubernetesEventInvolvedObject,
  kubernetesEventName,
} from './KubernetesRecordDetail';
import { KubernetesScopeText } from './KubernetesScopeText';

const KUBERNETES_EVENT_SEVERITY_OPTIONS: PlatformTableFilterOption<KubernetesEventSeverityFilter>[] =
  [
    { value: 'all', label: 'All' },
    {
      value: 'warning',
      label: 'Warning',
      tone: 'warning',
      leading: filterChipStatusDot('bg-amber-500'),
    },
    {
      value: 'normal',
      label: 'Normal',
      leading: filterChipStatusDot('bg-slate-400'),
    },
  ];

const involvedObject = (resource: Resource): string =>
  formatPlatformTableTextValue(kubernetesEventInvolvedObject(resource));

const observedTimestamp = (resource: Resource): string =>
  asTrimmedString(
    resource.kubernetes?.eventTime ||
      resource.kubernetes?.firstSeen ||
      resource.kubernetes?.createdAt,
  ) || '';

export const KubernetesEventsTable: Component<{
  resources: Resource[];
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
  title?: string;
  showToolbar?: boolean;
}> = (props) => {
  const sortedEvents = createMemo(() => [...props.resources].sort(compareKubernetesEvents));
  const singleCluster = createMemo(() => isSingleKubernetesCluster(props.resources));
  const tableState = createPlatformTableFilterState({
    resources: sortedEvents,
    initialStatus: 'all' as KubernetesEventSeverityFilter,
    filter: filterKubernetesEvents,
  });
  const drawer = createPlatformResourceDetailState({ idPrefix: 'kubernetes-event-drawer' });

  return (
    <KubernetesTableFrame
      hasResources={props.resources.length > 0}
      emptyIcon={props.emptyIcon}
      emptyTitle={props.emptyTitle}
      emptyDescription={props.emptyDescription}
      showToolbar={props.showToolbar}
      rowNoun="events"
      filteredEmptyNoun="Kubernetes events"
      statusOptions={KUBERNETES_EVENT_SEVERITY_OPTIONS}
      tableState={tableState}
    >
      <PlatformTableShell
        title={props.title ?? 'Events'}
        tableClass="min-w-full table-fixed text-xs md:min-w-[1180px]"
        header={
          <>
            <TableHead
              class={`${getPlatformTableHeadClassForKind('name')} platform-table-mobile-w-30 md:w-[17%]`}
            >
              Event
            </TableHead>
            <TableHead
              class={`${getPlatformTableHeadClassForKind('text')} hidden lg:table-cell md:w-[12%]`}
            >
              Scope
            </TableHead>
            <TableHead
              class={`${getPlatformTableHeadClassForKind('text')} platform-table-phone-hidden md:w-[7%]`}
            >
              Type
            </TableHead>
            <TableHead
              class={`${getPlatformTableHeadClassForKind('text')} platform-table-mobile-w-20 md:w-[11%]`}
            >
              <PlatformResponsiveTableLabel compact="Why" full="Reason" />
            </TableHead>
            <TableHead
              class={`${getPlatformTableHeadClassForKind('text')} platform-table-mobile-w-25 md:w-[12%]`}
            >
              Object
            </TableHead>
            <TableHead
              class={`${getPlatformTableHeadClassForKind('numeric-value')} platform-table-mobile-w-10 md:w-[5%] platform-table-narrow-hidden`}
            >
              Count
            </TableHead>
            <TableHead
              class={`${getPlatformTableHeadClassForKind('numeric-value')} platform-table-mobile-w-15 md:w-[9%]`}
            >
              <PlatformResponsiveTableLabel compact="Seen" full="Observed" />
            </TableHead>
            <TableHead
              class={`${getPlatformTableHeadClassForKind('text')} hidden sm:table-cell md:w-[27%]`}
            >
              Message
            </TableHead>
          </>
        }
        body={
          <>
            <PlatformWindowedRows items={tableState.filtered} estimatedRowHeight={32}>
              {(resource) => {
                const indicator = () => mapKubernetesEventSeverity(resource.kubernetes?.eventType);
                const name = () => kubernetesEventName(resource);
                const observed = () => observedTimestamp(resource);
                const message = () => formatPlatformTableTextValue(resource.kubernetes?.message);
                const detailRowId = () => drawer.detailRowId(resource);
                const isExpanded = () => drawer.isExpanded(resource);

                return (
                  <>
                    <TableRow
                      class={`${getPlatformResourceDetailRowClass(isExpanded())} text-[11px] sm:text-xs`}
                      data-kubernetes-event-row={resource.id}
                      onClick={() => drawer.toggle(resource)}
                    >
                      <TableCell class={getPlatformTableCellClassForKind('name')}>
                        <KubernetesRowLead
                          resource={resource}
                          drawer={drawer}
                          label={name()}
                          status={indicator()}
                        />
                      </TableCell>
                      <TableCell
                        class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content lg:table-cell`}
                      >
                        <KubernetesScopeText
                          resource={resource}
                          singleCluster={singleCluster()}
                          class="block truncate"
                        />
                      </TableCell>
                      <TableCell
                        class={`${getPlatformTableCellClassForKind('text')} platform-table-phone-hidden text-base-content`}
                      >
                        {formatPlatformTableTextValue(resource.kubernetes?.eventType)}
                      </TableCell>
                      <TableCell
                        class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                      >
                        <span
                          class="block truncate"
                          title={formatPlatformTableTextValue(resource.kubernetes?.reason)}
                        >
                          {formatPlatformTableTextValue(resource.kubernetes?.reason)}
                        </span>
                      </TableCell>
                      <TableCell
                        class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                      >
                        <span class="block truncate" title={involvedObject(resource)}>
                          {involvedObject(resource)}
                        </span>
                      </TableCell>
                      <TableCell
                        class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content platform-table-narrow-hidden`}
                      >
                        <PlatformTableNumberValue value={resource.kubernetes?.count} />
                      </TableCell>
                      <TableCell
                        class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                      >
                        <span class="block truncate" title={observed() || '—'}>
                          <PlatformTableRelativeTimeValue value={observed()} />
                        </span>
                      </TableCell>
                      <TableCell
                        class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content sm:table-cell`}
                      >
                        <span class="block truncate" title={message()}>
                          {message()}
                        </span>
                      </TableCell>
                    </TableRow>
                    <Show when={isExpanded()}>
                      <InlineDetailTableRow
                        cellId={detailRowId()}
                        colspan={8}
                        data-inline-kubernetes-event-detail-for={resource.id}
                      >
                        <KubernetesEventDetailPanel
                          resource={resource}
                          status={indicator()}
                          onClose={() => drawer.close(resource)}
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
    </KubernetesTableFrame>
  );
};

export default KubernetesEventsTable;
