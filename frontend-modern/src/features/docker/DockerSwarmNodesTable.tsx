import { For, Show, createMemo, type Component } from 'solid-js';
import { TableCell, TableRow } from '@/components/shared/Table';
import { asTrimmedString } from '@/utils/stringUtils';
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
  DockerResourceNameCell,
  dockerByteValue,
  dockerCpuValue,
  dockerResourceName,
  dockerTextValue,
  type DockerNativeTableProps,
} from './DockerNativeTableShared';
import {
  compareDockerSwarmNodes,
  filterDockerResources,
  mapDockerSwarmNodeStatus,
  type DockerResourceStatusFilter,
} from './dockerPageModel';
import type { Resource } from '@/types/resource';

const DOCKER_SWARM_NODE_SORT_KEYS = [
  'node',
  'role',
  'availability',
  'reachability',
  'engine',
  'cpus',
  'memory',
  'address',
] as const;

type DockerSwarmNodeSortKey = (typeof DOCKER_SWARM_NODE_SORT_KEYS)[number];

const getDockerSwarmNodeSortValue = (
  resource: Resource,
  key: DockerSwarmNodeSortKey,
): PlatformTableSortValue => {
  switch (key) {
    case 'node':
      return dockerResourceName(resource);
    case 'role':
      return asTrimmedString(resource.docker?.nodeRole) || null;
    case 'availability':
      return asTrimmedString(resource.docker?.availability) || null;
    case 'reachability':
      return (
        asTrimmedString(
          resource.docker?.leader ? 'leader' : resource.docker?.managerReachability,
        ) || null
      );
    case 'engine':
      return (
        asTrimmedString(resource.docker?.engineVersion || resource.docker?.runtimeVersion) || null
      );
    case 'cpus':
      return typeof resource.docker?.nanoCpus === 'number' && resource.docker.nanoCpus > 0
        ? resource.docker.nanoCpus
        : null;
    case 'memory':
      return typeof resource.docker?.memoryBytes === 'number' && resource.docker.memoryBytes > 0
        ? resource.docker.memoryBytes
        : null;
    case 'address':
      return asTrimmedString(resource.docker?.address || resource.docker?.managerAddress) || null;
    default:
      key satisfies never;
      return null;
  }
};

export const DockerSwarmNodesTable: Component<DockerNativeTableProps> = (props) => {
  const tableState = createPlatformTableFilterState({
    resources: () => props.resources,
    initialStatus: 'all' as DockerResourceStatusFilter,
    filter: filterDockerResources,
  });
  const sort = createPlatformTableSortState({
    storageKey: 'dockerSwarmNodes',
    sortKeys: DOCKER_SWARM_NODE_SORT_KEYS,
    descendingFirst: ['cpus', 'memory'],
  });
  const sortedRows = createMemo(() =>
    sort.sortRows(
      [...tableState.filtered()].sort(compareDockerSwarmNodes),
      getDockerSwarmNodeSortValue,
    ),
  );

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
            searchPlaceholder="Search Swarm nodes"
            status={tableState.status()}
            onStatusChange={tableState.setStatus}
            statusOptions={PLATFORM_HEALTH_FILTER_OPTIONS}
            visible={tableState.visible()}
            total={tableState.total()}
            rowNoun="nodes"
          />
        </Show>

        <Show
          when={tableState.filtered().length > 0}
          fallback={
            <PlatformTableEmptyState
              icon={props.emptyIcon}
              title="No Swarm nodes match current filters"
              description="Adjust the search or status filter to see more Docker Swarm nodes."
            />
          }
        >
          <PlatformTableShell
            title={props.title ?? 'Swarm Nodes'}
            tableClass="min-w-full table-fixed text-xs md:min-w-[1120px]"
            header={
              <>
                <PlatformSortableTableHead
                  kind="name"
                  sort={sort}
                  sortKey="node"
                  class="md:w-[20%]"
                >
                  Node
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  sortKey="role"
                  class="md:w-[10%]"
                >
                  Role
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  sortKey="availability"
                  class="md:w-[12%]"
                >
                  Availability
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  sortKey="reachability"
                  class="hidden md:table-cell md:w-[14%]"
                >
                  Reachability
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  sortKey="engine"
                  class="hidden md:table-cell md:w-[16%]"
                >
                  Engine
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="numeric-value"
                  sort={sort}
                  sortKey="cpus"
                  class="hidden md:table-cell md:w-[8%]"
                >
                  CPUs
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="numeric-value"
                  sort={sort}
                  sortKey="memory"
                  class="md:w-[10%]"
                >
                  Memory
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  sortKey="address"
                  class="hidden md:table-cell md:w-[10%]"
                >
                  Address
                </PlatformSortableTableHead>
              </>
            }
            body={
              <>
                <For each={sortedRows()}>
                  {(resource) => {
                    const managerReachability = () =>
                      dockerTextValue(
                        resource.docker?.leader ? 'leader' : resource.docker?.managerReachability,
                      );

                    return (
                      <TableRow
                        class="text-[11px] sm:text-xs"
                        data-docker-swarm-node-row={resource.id}
                      >
                        <DockerResourceNameCell
                          resource={resource}
                          indicator={mapDockerSwarmNodeStatus(resource)}
                        />
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                        >
                          {dockerTextValue(resource.docker?.nodeRole)}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                        >
                          {dockerTextValue(resource.docker?.availability)}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                        >
                          {managerReachability()}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                        >
                          {dockerTextValue(
                            resource.docker?.engineVersion || resource.docker?.runtimeVersion,
                          )}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('numeric-value')} hidden text-base-content md:table-cell`}
                        >
                          {dockerCpuValue(resource.docker?.nanoCpus)}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                        >
                          {dockerByteValue(resource.docker?.memoryBytes)}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                        >
                          {dockerTextValue(
                            resource.docker?.address || resource.docker?.managerAddress,
                          )}
                        </TableCell>
                      </TableRow>
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

export default DockerSwarmNodesTable;
