import { For, Show, createMemo, type Component, type JSX } from 'solid-js';
import { StatusDot } from '@/components/shared/StatusDot';
import { TableCell, TableHead, TableRow } from '@/components/shared/Table';
import { asTrimmedString } from '@/utils/stringUtils';
import {
  PLATFORM_HEALTH_FILTER_OPTIONS,
  PlatformTableEmptyState,
  PlatformTableNumberValue,
  PlatformTableRelativeTimeValue,
  PlatformTableToolbar,
  createPlatformTableFilterState,
  formatPlatformTableTextValue,
  getPlatformTableCellClassForKind,
  getPlatformTableHeadClassForKind,
  PlatformTableShell,
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
            status={tableState.status()}
            onStatusChange={tableState.setStatus}
            statusOptions={PLATFORM_HEALTH_FILTER_OPTIONS}
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
                <TableHead class={`${getPlatformTableHeadClassForKind('name')} md:w-[16%]`}>
                  Event
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[15%]`}
                >
                  Scope
                </TableHead>
                <TableHead class={`${getPlatformTableHeadClassForKind('text')} md:w-[10%]`}>
                  Type
                </TableHead>
                <TableHead class={`${getPlatformTableHeadClassForKind('text')} md:w-[14%]`}>
                  Reason
                </TableHead>
                <TableHead class={`${getPlatformTableHeadClassForKind('text')} md:w-[15%]`}>
                  Object
                </TableHead>
                <TableHead class={`${getPlatformTableHeadClassForKind('numeric-value')} md:w-[7%]`}>
                  Count
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[13%]`}
                >
                  Observed
                </TableHead>
                <TableHead class={`${getPlatformTableHeadClassForKind('text')} md:w-[10%]`}>
                  Message
                </TableHead>
              </>
            }
            body={
              <>
                <For each={tableState.filtered()}>
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
                          aria-controls={isExpanded() ? detailRowId() : undefined}
                          aria-expanded={isExpanded() ? 'true' : 'false'}
                          data-kubernetes-event-row={resource.id}
                          onClick={() => drawer.toggle(resource)}
                          onKeyDown={drawer.handleActivationKey(resource)}
                          tabIndex={0}
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
                            class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                          >
                            <span class="inline-block max-w-[12rem] truncate" title={scope()}>
                              {scope()}
                            </span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
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
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                          >
                            <PlatformTableNumberValue value={resource.kubernetes?.count} />
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                          >
                            <span
                              class="inline-block max-w-[12rem] truncate"
                              title={observed() || '—'}
                            >
                              <PlatformTableRelativeTimeValue value={observed()} />
                            </span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
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
                </For>
              </>
            }
          />
        </Show>
      </div>
    </Show>
  );
};

export default KubernetesEventsTable;
