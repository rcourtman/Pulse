import { Show, createMemo, type Component, type JSX } from 'solid-js';
import { StatusDot } from '@/components/shared/StatusDot';
import { TableCell, TableHead, TableRow } from '@/components/shared/Table';
import { asTrimmedString } from '@/utils/stringUtils';
import {
  PlatformWindowedRows,
  PLATFORM_HEALTH_FILTER_OPTIONS,
  PlatformResponsiveTableLabel,
  PlatformTableEmptyState,
  PlatformTableNumberValue,
  PlatformTableRelativeTimeValue,
  PlatformTableToolbar,
  createPlatformTableFilterState,
  formatPlatformTableTextValue,
  getPlatformTableCellClassForKind,
  getPlatformTableHeadClassForKind,
  PlatformTableShell,
  withPlatformStatusCounts,
} from '@/features/platformPage/sharedPlatformPage';
import {
  PlatformResourceDetailToggleButton,
  PlatformResourceDetailTableRow,
  createPlatformResourceDetailState,
  createPlatformResourceLabelResolver,
  getPlatformResourceDetailRowClass,
} from '@/features/platformPage/PlatformResourceDetailTableRow';
import type { Resource } from '@/types/resource';
import {
  compareKubernetesEvents,
  filterKubernetesResources,
  kubernetesScopeLabel,
  mapKubernetesEventSeverity,
  type KubernetesResourceStatusFilter,
} from './kubernetesPageModel';

const eventName = (resource: Resource): string =>
  asTrimmedString(resource.displayName) || asTrimmedString(resource.name) || resource.id;

const involvedObject = (resource: Resource): string =>
  formatPlatformTableTextValue(
    [resource.kubernetes?.involvedKind, resource.kubernetes?.involvedName]
      .filter(Boolean)
      .join('/'),
  );

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
  const tableState = createPlatformTableFilterState({
    resources: sortedEvents,
    initialStatus: 'all' as KubernetesResourceStatusFilter,
    filter: filterKubernetesResources,
  });
  const drawer = createPlatformResourceDetailState({ idPrefix: 'kubernetes-event-drawer' });
  const resolveResourceLabel = createPlatformResourceLabelResolver(() => props.resources);

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
            searchPlaceholder="Search events"
            searchSuggestions={tableState.searchSuggestions}
            status={tableState.status()}
            onStatusChange={tableState.setStatus}
            statusOptions={withPlatformStatusCounts(
              PLATFORM_HEALTH_FILTER_OPTIONS,
              tableState.countForStatus,
            )}
            visible={tableState.visible()}
            total={tableState.total()}
            rowNoun="events"
          />
        </Show>

        <Show
          when={tableState.filtered().length > 0}
          fallback={
            <PlatformTableEmptyState
              icon={props.emptyIcon}
              title="No events match current filters"
              description="Adjust the search or status filter to see more Kubernetes events."
            />
          }
        >
          <PlatformTableShell
            title={props.title ?? 'Events'}
            tableClass="min-w-full table-fixed text-xs md:min-w-[1180px]"
            header={
              <>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('name')} platform-table-mobile-w-30 md:w-[16%]`}
                >
                  Event
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('text')} hidden lg:table-cell md:w-[15%]`}
                >
                  Scope
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('text')} platform-table-phone-hidden md:w-[10%]`}
                >
                  Type
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('text')} platform-table-mobile-w-15 md:w-[14%]`}
                >
                  <PlatformResponsiveTableLabel compact="Why" full="Reason" />
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('text')} platform-table-mobile-w-20 md:w-[15%]`}
                >
                  Object
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('numeric-value')} platform-table-mobile-w-10 md:w-[7%] platform-table-narrow-hidden`}
                >
                  Count
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('numeric-value')} platform-table-mobile-w-10 md:w-[13%]`}
                >
                  <PlatformResponsiveTableLabel compact="Seen" full="Observed" />
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('text')} hidden sm:table-cell md:w-[10%]`}
                >
                  Message
                </TableHead>
              </>
            }
            body={
              <>
                <PlatformWindowedRows items={tableState.filtered} estimatedRowHeight={32}>
                  {(resource) => {
                    const indicator = () =>
                      mapKubernetesEventSeverity(resource.kubernetes?.eventType);
                    const name = () => eventName(resource);
                    const scope = () => kubernetesScopeLabel(resource);
                    const observed = () => observedTimestamp(resource);
                    const message = () =>
                      formatPlatformTableTextValue(resource.kubernetes?.message);
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
                            <div class="flex min-w-0 items-center gap-2">
                              <PlatformResourceDetailToggleButton
                                expanded={isExpanded()}
                                resourceLabel={name()}
                                controlsId={detailRowId()}
                                onToggle={() => drawer.toggle(resource)}
                              />
                              <StatusDot
                                size="sm"
                                variant={indicator().variant}
                                title={indicator().label}
                                ariaHidden
                              />
                              <span class="truncate font-semibold text-base-content" title={name()}>
                                {name()}
                              </span>
                            </div>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content lg:table-cell`}
                          >
                            <span class="inline-block max-w-[12rem] truncate" title={scope()}>
                              {scope()}
                            </span>
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
                              class="inline-block max-w-[12rem] truncate"
                              title={formatPlatformTableTextValue(resource.kubernetes?.reason)}
                            >
                              {formatPlatformTableTextValue(resource.kubernetes?.reason)}
                            </span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                          >
                            <span
                              class="inline-block max-w-[13rem] truncate"
                              title={involvedObject(resource)}
                            >
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
                            <span
                              class="inline-block max-w-[12rem] truncate"
                              title={observed() || '—'}
                            >
                              <PlatformTableRelativeTimeValue value={observed()} />
                            </span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content sm:table-cell`}
                          >
                            <span class="inline-block max-w-[16rem] truncate" title={message()}>
                              {message()}
                            </span>
                          </TableCell>
                        </TableRow>
                        <PlatformResourceDetailTableRow
                          resource={resource}
                          open={isExpanded()}
                          detailRowId={detailRowId()}
                          colSpan={8}
                          resolveResourceLabel={resolveResourceLabel}
                          onClose={() => drawer.close(resource)}
                        />
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

export default KubernetesEventsTable;
