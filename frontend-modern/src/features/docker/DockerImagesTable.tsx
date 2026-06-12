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
  dockerHostName,
  dockerJoinValues,
  dockerResourceName,
  dockerNumberValue,
  type DockerNativeTableProps,
} from './DockerNativeTableShared';
import { filterDockerResources, type DockerResourceStatusFilter } from './dockerPageModel';

export const DockerImagesTable: Component<DockerNativeTableProps> = (props) => {
  const tableState = createPlatformTableFilterState({
    resources: () => props.resources,
    initialStatus: 'all' as DockerResourceStatusFilter,
    filter: filterDockerResources,
  });
  const drawer = createPlatformResourceDetailState({ idPrefix: 'docker-image-drawer' });
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
            searchPlaceholder="Search images"
            status={tableState.status()}
            onStatusChange={tableState.setStatus}
            statusOptions={PLATFORM_HEALTH_FILTER_OPTIONS}
            visible={tableState.visible()}
            total={tableState.total()}
            rowNoun="images"
          />
        </Show>

        <Show
          when={tableState.filtered().length > 0}
          fallback={
            <PlatformTableEmptyState
              icon={props.emptyIcon}
              title="No images match current filters"
              description="Adjust the search or status filter to see more Docker images."
            />
          }
        >
          <PlatformTableShell
            title={props.title ?? 'Images'}
            tableClass="min-w-full table-fixed text-xs md:min-w-[1120px]"
            header={
              <>
                <TableHead class={`${getPlatformTableHeadClassForKind('name')} md:w-[24%]`}>
                  Image
                </TableHead>
                <TableHead class={`${getPlatformTableHeadClassForKind('text')} md:w-[22%]`}>
                  Tags
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[24%]`}
                >
                  Digests
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('numeric-value')} md:w-[10%]`}
                >
                  Size
                </TableHead>
                <TableHead class={`${getPlatformTableHeadClassForKind('numeric-value')} md:w-[8%]`}>
                  In Use
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[12%]`}
                >
                  Host
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
                          data-docker-image-row={resource.id}
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
                            <span
                              class="inline-block max-w-[18rem] truncate"
                              title={dockerJoinValues(resource.docker?.repoTags)}
                            >
                              {dockerJoinValues(resource.docker?.repoTags)}
                            </span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                          >
                            <span
                              class="inline-block max-w-[22rem] truncate"
                              title={dockerJoinValues(resource.docker?.repoDigests)}
                            >
                              {dockerJoinValues(resource.docker?.repoDigests)}
                            </span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                          >
                            {dockerByteValue(resource.docker?.sizeBytes)}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                          >
                            {dockerNumberValue(resource.docker?.imageContainers)}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                          >
                            {dockerHostName(resource)}
                          </TableCell>
                        </TableRow>
                        <PlatformResourceDetailTableRow
                          resource={resource}
                          open={isExpanded()}
                          detailRowId={detailRowId()}
                          colSpan={6}
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

export default DockerImagesTable;
