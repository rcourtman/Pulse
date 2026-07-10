import { For, Show, createMemo, type Component } from 'solid-js';
import { TableCell, TableRow } from '@/components/shared/Table';
import {
  PLATFORM_HEALTH_FILTER_OPTIONS,
  PlatformSortableTableHead,
  PlatformTableEmptyState,
  PlatformTableToolbar,
  createPlatformTableFilterState,
  createPlatformTableSortState,
  getPlatformTableCellClassForKind,
  PlatformTableShell,
  type PlatformTableSortValue,
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
  dockerResourceName,
  type DockerNativeTableProps,
} from './DockerNativeTableShared';
import { filterDockerResources, type DockerResourceStatusFilter } from './dockerPageModel';
import {
  getDockerImageOperationalPresentation,
  type DockerImageUpdateTone,
} from './dockerImagePresentation';
import type { Resource } from '@/types/resource';

const updateToneClass: Record<DockerImageUpdateTone, string> = {
  danger: 'bg-red-100 text-red-700 dark:bg-red-950/40 dark:text-red-300',
  warning: 'bg-amber-100 text-amber-700 dark:bg-amber-950/40 dark:text-amber-300',
  success: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-300',
  muted: 'bg-surface-hover text-muted',
};

const DOCKER_IMAGE_SORT_KEYS = ['image', 'host', 'usedBy', 'size', 'update'] as const;

type DockerImageSortKey = (typeof DOCKER_IMAGE_SORT_KEYS)[number];

export const DockerImagesTable: Component<
  DockerNativeTableProps & { relatedContainers?: Resource[] }
> = (props) => {
  const tableState = createPlatformTableFilterState({
    resources: () => props.resources,
    initialStatus: 'all' as DockerResourceStatusFilter,
    filter: filterDockerResources,
  });
  const drawer = createPlatformResourceDetailState({ idPrefix: 'docker-image-drawer' });
  const resolveResourceLabel = createPlatformResourceLabelResolver(() => props.resources);
  const sort = createPlatformTableSortState({
    storageKey: 'dockerImages',
    sortKeys: DOCKER_IMAGE_SORT_KEYS,
    descendingFirst: ['size'],
  });
  // Closure rather than a module-level accessor: the operational columns
  // (Used by / Update check) derive from props.relatedContainers.
  const getImageSortValue = (
    resource: Resource,
    key: DockerImageSortKey,
  ): PlatformTableSortValue => {
    switch (key) {
      case 'image':
        return dockerResourceName(resource);
      case 'host': {
        const host = dockerHostName(resource);
        return host === '—' ? null : host;
      }
      case 'usedBy': {
        const summary = getDockerImageOperationalPresentation(
          resource,
          props.relatedContainers ?? [],
        ).consumerSummary;
        return summary && summary !== '—' ? summary : null;
      }
      case 'size':
        return typeof resource.docker?.sizeBytes === 'number' && resource.docker.sizeBytes > 0
          ? resource.docker.sizeBytes
          : null;
      case 'update':
        return getDockerImageOperationalPresentation(resource, props.relatedContainers ?? [])
          .updateLabel;
      default:
        key satisfies never;
        return null;
    }
  };
  const sortedRows = createMemo(() => sort.sortRows(tableState.filtered(), getImageSortValue));

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
            searchPlaceholder="Search image, host, or update state"
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
            tableClass="min-w-full table-fixed text-xs md:min-w-[880px]"
            header={
              <>
                <PlatformSortableTableHead
                  kind="name"
                  sort={sort}
                  sortKey="image"
                  class="md:w-[30%]"
                >
                  Image
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  sortKey="host"
                  class="md:w-[18%]"
                >
                  Host
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  sortKey="usedBy"
                  class="md:w-[24%]"
                >
                  Used by
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="numeric-value"
                  sort={sort}
                  sortKey="size"
                  class="md:w-[12%]"
                >
                  Size
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="badge"
                  sort={sort}
                  sortKey="update"
                  class="md:w-[16%]"
                >
                  Update check
                </PlatformSortableTableHead>
              </>
            }
            body={
              <>
                <For each={sortedRows()}>
                  {(resource) => {
                    const operational = () =>
                      getDockerImageOperationalPresentation(
                        resource,
                        props.relatedContainers ?? [],
                      );
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
                            {dockerHostName(resource)}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                          >
                            <span
                              class="inline-block max-w-[20rem] truncate"
                              title={operational().consumerSummary}
                            >
                              {operational().consumerSummary}
                            </span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                          >
                            {dockerByteValue(resource.docker?.sizeBytes)}
                          </TableCell>
                          <TableCell class={getPlatformTableCellClassForKind('badge')}>
                            <span
                              class={`inline-flex rounded-full px-2 py-0.5 text-[10px] font-medium ${updateToneClass[operational().updateTone]}`}
                              title={operational().updateDetail}
                            >
                              {operational().updateLabel}
                            </span>
                          </TableCell>
                        </TableRow>
                        <PlatformResourceDetailTableRow
                          resource={resource}
                          open={isExpanded()}
                          detailRowId={detailRowId()}
                          colSpan={5}
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
