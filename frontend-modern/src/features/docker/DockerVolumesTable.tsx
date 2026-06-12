import { For, Show, type Component } from 'solid-js';
import { TableCell, TableHead, TableRow } from '@/components/shared/Table';
import {
  PLATFORM_HEALTH_FILTER_OPTIONS,
  PlatformTableEmptyState,
  PlatformTableToolbar,
  createPlatformTableFilterState,
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
import {
  DockerResourceNameCell,
  dockerByteValue,
  dockerNumberValue,
  dockerResourceName,
  dockerTextValue,
  type DockerNativeTableProps,
} from './DockerNativeTableShared';
import { filterDockerResources, type DockerResourceStatusFilter } from './dockerPageModel';
import { formatRelativeTime } from '@/utils/format';

const parseCreatedAt = (value: string | undefined): number | null => {
  if (!value) return null;
  const parsed = Date.parse(value);
  return Number.isFinite(parsed) ? parsed : null;
};

const createdRelative = (value: string | undefined): string => {
  const parsed = parseCreatedAt(value);
  return parsed === null ? dockerTextValue(value) : formatRelativeTime(parsed);
};

const createdAbsolute = (value: string | undefined): string => {
  const parsed = parseCreatedAt(value);
  return parsed === null ? dockerTextValue(value) : new Date(parsed).toLocaleString();
};

export const DockerVolumesTable: Component<DockerNativeTableProps> = (props) => {
  const tableState = createPlatformTableFilterState({
    resources: () => props.resources,
    initialStatus: 'all' as DockerResourceStatusFilter,
    filter: filterDockerResources,
    externalSearch: props.externalSearch,
    externalStatus: props.externalStatus,
  });
  const drawer = createPlatformResourceDetailState({ idPrefix: 'docker-volume-drawer' });
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
            searchPlaceholder="Search volumes"
            status={tableState.status()}
            onStatusChange={tableState.setStatus}
            statusOptions={PLATFORM_HEALTH_FILTER_OPTIONS}
            visible={tableState.visible()}
            total={tableState.total()}
            rowNoun="volumes"
            hasActiveFilters={tableState.hasActiveFilters()}
            onResetFilters={tableState.resetFilters}
          />
        </Show>

        <Show
          when={tableState.filtered().length > 0}
          fallback={
            <PlatformTableEmptyState
              icon={props.emptyIcon}
              title="No volumes match current filters"
              description="Adjust the search or status filter to see more Docker volumes."
            />
          }
        >
          <PlatformTableShell
            title={props.title ?? 'Volumes'}
            tableClass="min-w-full table-fixed text-xs md:min-w-[1120px]"
            header={
              <>
                <TableHead class={`${getPlatformTableHeadClassForKind('name')} md:w-[22%]`}>
                  Volume
                </TableHead>
                <TableHead class={`${getPlatformTableHeadClassForKind('text')} md:w-[12%]`}>
                  Driver
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[10%]`}
                >
                  Scope
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('numeric-value')} md:w-[10%]`}
                >
                  Size
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('numeric-value')} hidden md:table-cell md:w-[8%]`}
                >
                  Refs
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[14%]`}
                >
                  Created
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[24%]`}
                >
                  Mountpoint
                </TableHead>
              </>
            }
            body={
              <>
                <For each={tableState.filtered()}>
                  {(resource) => {
                    const detailRowId = () => drawer.detailRowId(resource);
                    const isExpanded = () => drawer.isExpanded(resource);
                    return (
                      <>
                        <TableRow
                          class={`${getPlatformResourceDetailRowClass(isExpanded())} text-[11px] sm:text-xs`}
                          aria-controls={isExpanded() ? detailRowId() : undefined}
                          aria-expanded={isExpanded() ? 'true' : 'false'}
                          data-docker-volume-row={resource.id}
                          onClick={() => drawer.toggle(resource)}
                          onKeyDown={drawer.handleActivationKey(resource)}
                          tabIndex={0}
                        >
                          <DockerResourceNameCell
                            resource={resource}
                            detailToggle={
                              <PlatformResourceDetailToggleButton
                                expanded={isExpanded()}
                                resourceLabel={dockerResourceName(resource)}
                                controlsId={detailRowId()}
                                onToggle={() => drawer.toggle(resource)}
                              />
                            }
                          />
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                          >
                            {dockerTextValue(resource.docker?.driver)}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                          >
                            {dockerTextValue(resource.docker?.scope)}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                          >
                            {dockerByteValue(resource.docker?.sizeBytes)}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} hidden text-base-content md:table-cell`}
                          >
                            {dockerNumberValue(resource.docker?.refCount)}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                          >
                            <span
                              class="inline-block max-w-[12rem] truncate"
                              title={createdAbsolute(resource.docker?.createdAt)}
                            >
                              {createdRelative(resource.docker?.createdAt)}
                            </span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                          >
                            <span
                              class="inline-block max-w-[22rem] truncate"
                              title={dockerTextValue(resource.docker?.mountpoint)}
                            >
                              {dockerTextValue(resource.docker?.mountpoint)}
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
              </>
            }
          />
        </Show>
      </div>
    </Show>
  );
};

export default DockerVolumesTable;
