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
import { asTrimmedString } from '@/utils/stringUtils';
import { formatRelativeTime } from '@/utils/format';
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

const textValue = (value: string | undefined): string => asTrimmedString(value) || '—';

const eventName = (resource: Resource): string =>
  asTrimmedString(resource.displayName) || asTrimmedString(resource.name) || resource.id;

const involvedObject = (resource: Resource): string =>
  textValue(
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

// Humanized for the cell ("2h ago"); the raw timestamp stays on the title.
const observedTime = (resource: Resource): string =>
  formatRelativeTime(observedTimestamp(resource), { compact: true, emptyText: '—' }) || '—';

const numberValue = (value: number | undefined): JSX.Element =>
  typeof value === 'number' ? <span class="tabular-nums">{value}</span> : <span>—</span>;

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
          <TableCard class={PLATFORM_TABLE_CARD_CLASS}>
            <TableCardHeader title={props.title ?? 'Events'} />
            <Table class="min-w-full table-fixed text-xs md:min-w-[1180px]">
              <TableHeader>
                <TableRow class={PLATFORM_TABLE_HEADER_ROW_CLASS}>
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
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('numeric-value')} md:w-[7%]`}
                  >
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
                </TableRow>
              </TableHeader>
              <TableBody class={PLATFORM_TABLE_BODY_CLASS}>
                <For each={tableState.filtered()}>
                  {(resource) => {
                    const indicator = () =>
                      mapKubernetesEventSeverity(resource.kubernetes?.eventType);
                    const name = () => eventName(resource);
                    const scope = () => kubernetesScopeLabel(resource);
                    const observed = () => observedTime(resource);
                    const message = () => textValue(resource.kubernetes?.message);
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
                          {textValue(resource.kubernetes?.eventType)}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                        >
                          <span
                            class="inline-block max-w-[12rem] truncate"
                            title={textValue(resource.kubernetes?.reason)}
                          >
                            {textValue(resource.kubernetes?.reason)}
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
                          {numberValue(resource.kubernetes?.count)}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                        >
                          <span
                            class="inline-block max-w-[12rem] truncate"
                            title={observedTimestamp(resource) || observed()}
                          >
                            {observed()}
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
              </TableBody>
            </Table>
          </TableCard>
        </Show>
      </div>
    </Show>
  );
};

export default KubernetesEventsTable;
